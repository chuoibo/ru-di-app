package nap

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// LoVao is one document of a batch, keyed by its content hash.
type LoVao struct {
	Khoa string
	TaiLieu
}

// KetQuaLo is a batch job read back: still running, done with vectors (key →
// vector; LoiDong lines the provider failed), or failed.
type KetQuaLo struct {
	Xong, Hong bool
	Vecs       map[string][]float32
	LoiDong    int
	Loi        string
}

// NhungLo is the dense encoder's batch door (the Gemini Batch API behind
// aiharness/nhung.Lo, wired by cmd/core): same model and dims as the online
// door, and a vector from it is the online door's vector for the same text.
type NhungLo interface {
	Model() string
	Dims() int
	GuiLo(ctx context.Context, ten string, docs []LoVao) (job string, err error)
	XemLo(ctx context.Context, job string) (KetQuaLo, error)
}

// BaoCaoLo is what one batch run did, in counts and states only.
type BaoCaoLo struct {
	// Can: distinct content hashes of the rows; DaCo: of them already in the
	// cache; Gui: submitted in a new job (0 when an open job was resumed).
	Can, DaCo, Gui int
	Job            string `json:"job,omitempty"`
	// SoJob: batch jobs polled to their end in this run (a run sends the
	// missing documents in jobs of at most toiDa each).
	SoJob     int     `json:"so_job"`
	TrangThai string  `json:"trang_thai"`
	Ghi       int     `json:"ghi"`
	LoiDong   int     `json:"loi_dong"`
	Giay      float64 `json:"giay"`
}

// ErrLoHong: the provider failed the job; nothing was written.
var ErrLoHong = errors.New("nap: the batch embedding job failed")

// NhungQuaLo fills the dense cache for rows through the batch door: the
// hashes the cache lacks go out in one job, the run polls every cho until
// the job ends or ctx does, and a finished job's vectors are checked
// (KiemVector) and written to the cache under the configuration's task. A
// job already running for this model, dims and task is polled instead of a
// new one submitted, so a run cut short costs nothing twice. The build that
// follows (Vector / NhungHang) then finds every vector in the cache. cho 0
// or less polls once and returns ("dang_chay" while the job runs).
func (n Nap) NhungQuaLo(ctx context.Context, db CSDL, lo NhungLo, rows []Hang, cho time.Duration, toiDa int) (BaoCaoLo, error) {
	started := time.Now()
	var tong BaoCaoLo
	for lan := 0; ; lan++ {
		r, err := n.motLuotLo(ctx, db, lo, rows, cho, toiDa)
		if lan == 0 {
			tong.Can, tong.DaCo = r.Can, r.DaCo
		}
		tong.Gui += r.Gui
		tong.Ghi += r.Ghi
		tong.LoiDong += r.LoiDong
		if r.Job != "" {
			tong.Job = r.Job
			tong.SoJob++
		}
		tong.TrangThai = r.TrangThai
		tong.Giay = time.Since(started).Seconds()
		// A finished job may leave documents for the next one (toiDa);
		// anything else (nothing left, still running at the deadline, an
		// error) ends the run.
		if err != nil || r.TrangThai != "xong" {
			if lan > 0 && r.TrangThai == "khong_can" {
				tong.TrangThai = "xong"
			}
			return tong, err
		}
	}
}

// motLuotLo polls the open job, or submits one of at most toiDa documents
// (0: all) the cache lacks, and waits for it (or the deadline).
func (n Nap) motLuotLo(ctx context.Context, db CSDL, lo NhungLo, rows []Hang, cho time.Duration, toiDa int) (BaoCaoLo, error) {
	started := time.Now()
	var rep BaoCaoLo
	if lo.Model() != n.Cfg.Dense.Model || lo.Dims() != n.Cfg.Dense.Dims {
		return rep, fmt.Errorf("%w: the batch door serves %s/%d, the configuration asks %s/%d",
			ErrCauHinh, lo.Model(), lo.Dims(), n.Cfg.Dense.Model, n.Cfg.Dense.Dims)
	}
	task := TaskTaiLieu(n.Cfg)
	cache := BoNhoPG{Q: db}

	job, err := jobDangChay(ctx, db, lo.Model(), lo.Dims(), task)
	if err != nil {
		return rep, err
	}
	byHash := map[string]Hang{}
	for _, r := range rows {
		byHash[r.ContentHash] = r
	}
	hashes := make([]string, 0, len(byHash))
	for h := range byHash {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)
	rep.Can = len(hashes)
	if len(hashes) > 0 {
		have, err := cache.LayNhung(ctx, lo.Model(), lo.Dims(), task, hashes)
		if err != nil {
			return rep, err
		}
		rep.DaCo = len(have)
		if job == "" {
			var docs []LoVao
			for _, h := range hashes {
				if _, ok := have[h]; !ok {
					r := byHash[h]
					docs = append(docs, LoVao{Khoa: h, TaiLieu: TaiLieu{TieuDe: r.TieuDe, Chu: r.Text}})
				}
			}
			if len(docs) == 0 {
				rep.TrangThai = "khong_can"
				rep.Giay = time.Since(started).Seconds()
				return rep, nil
			}
			if toiDa > 0 && len(docs) > toiDa {
				docs = docs[:toiDa]
			}
			ten := fmt.Sprintf("rudi-%s-%s", n.Cfg.VanTay(), time.Now().UTC().Format("20060102T150405"))
			if job, err = lo.GuiLo(ctx, ten, docs); err != nil {
				return rep, err
			}
			if _, err := db.Exec(ctx, `INSERT INTO rag_embed_batches(job, model, dims, task, so_dong, trang_thai)
				VALUES($1,$2,$3,$4,$5,'dang_chay')`, job, lo.Model(), lo.Dims(), task, len(docs)); err != nil {
				return rep, err
			}
			rep.Gui = len(docs)
		}
	}
	if job == "" {
		rep.TrangThai = "khong_can"
		return rep, nil
	}
	rep.Job = job
	for {
		kq, err := lo.XemLo(ctx, job)
		if err != nil {
			return rep, err
		}
		switch {
		case kq.Hong:
			rep.TrangThai = "hong"
			if _, err := db.Exec(ctx, `UPDATE rag_embed_batches SET trang_thai='hong', xong_at=now() WHERE job=$1`, job); err != nil {
				return rep, err
			}
			rep.Giay = time.Since(started).Seconds()
			return rep, fmt.Errorf("%w: %s", ErrLoHong, kq.Loi)
		case kq.Xong:
			fresh := map[string][]float32{}
			for h, v := range kq.Vecs {
				ok, err := KiemVector(v, n.Cfg.Dense.Dims)
				if err != nil {
					rep.LoiDong++
					continue
				}
				fresh[h] = ok
			}
			rep.LoiDong += kq.LoiDong
			if err := ghiKetQuaLo(ctx, db, cache, lo, task, job, fresh, rep.LoiDong); err != nil {
				return rep, err
			}
			rep.Ghi, rep.TrangThai = len(fresh), "xong"
			rep.Giay = time.Since(started).Seconds()
			return rep, nil
		}
		if cho <= 0 {
			// One look (the indexer's turn): the next turn polls again.
			rep.TrangThai = "dang_chay"
			rep.Giay = time.Since(started).Seconds()
			return rep, nil
		}
		select {
		case <-ctx.Done():
			// The job keeps running at the provider; the next run polls it.
			rep.TrangThai = "dang_chay"
			rep.Giay = time.Since(started).Seconds()
			return rep, nil
		case <-time.After(cho):
		}
	}
}

func jobDangChay(ctx context.Context, db CSDL, model string, dims int, task string) (string, error) {
	var job string
	err := db.QueryRow(ctx, `SELECT job FROM rag_embed_batches WHERE model=$1 AND dims=$2 AND task=$3 AND trang_thai='dang_chay'`,
		model, dims, task).Scan(&job)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return job, err
}

// ghiKetQuaLo writes a finished job's vectors and closes its row in one
// transaction.
func ghiKetQuaLo(ctx context.Context, db CSDL, _ BoNhoPG, lo NhungLo, task, job string, fresh map[string][]float32, loi int) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if len(fresh) > 0 {
		if err := (BoNhoPG{Q: tx}).GhiNhung(ctx, lo.Model(), lo.Dims(), task, fresh); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE rag_embed_batches SET trang_thai='xong', so_ghi=$2, loi_dong=$3, xong_at=now() WHERE job=$1`,
		job, len(fresh), loi); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
