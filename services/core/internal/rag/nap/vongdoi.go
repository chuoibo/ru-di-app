package nap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/jobs"
)

// khoaAlias serialises every change of which collection an alias serves:
// promote, rollback and the reconciler.
const khoaAlias = `SELECT pg_advisory_xact_lock(hashtext('rag_nap_alias'))`

// kiemTenTrung refuses to touch aliases while a physical collection carries
// an alias's name: Milvus would resolve the name to that collection and
// silently ignore every alias swap (research sdlc-production §B1).
func (n Nap) kiemTenTrung(ctx context.Context) error {
	cols, err := n.Kho.DanhSachCollection(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", errMilvus, err)
	}
	if bad := KiemKhongTrungAlias(cols); len(bad) > 0 {
		return fmt.Errorf("%w: %v", ErrTenTrung, bad)
	}
	return nil
}

// datAlias points alias at ten and reads it back.
func (n Nap) datAlias(ctx context.Context, alias, ten string) error {
	if err := n.Kho.DatAlias(ctx, alias, ten); err != nil {
		return fmt.Errorf("%w: %v", errMilvus, err)
	}
	got, err := n.Kho.MoTaAlias(ctx, alias)
	if err != nil {
		return fmt.Errorf("%w: %v", errMilvus, err)
	}
	if got != ten {
		return fmt.Errorf("%w: %s → %q, want %s", ErrAlias, alias, got, ten)
	}
	return nil
}

// Promote serves an evaluated version: under the alias lock, the version
// must be `evaluated` with a passing gate that still holds -- its verdict
// recorded under the configuration fingerprint the pipeline runs now (the
// same the version was built with) and, for places, measured on the golden
// file whose sha256 is vangSha -- so a verdict from before a configuration
// or golden change can never serve. The alias moves to its collection and
// is read back (DescribeAlias); then, in the same transaction, the old
// active version is retired and becomes the new one's parent. If the
// transaction fails after the swap, the reconciler moves the alias back to
// what Postgres says: Postgres wins.
func (n Nap) Promote(ctx context.Context, db CSDL, id int64, vangSha string) (PhienBan, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return PhienBan{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, khoaAlias); err != nil {
		return PhienBan{}, err
	}
	p, err := DocPhienBan(ctx, tx, id)
	if err != nil {
		return p, err
	}
	if p.State != "evaluated" {
		return p, ErrTrangThai
	}
	var k KetQuaCong
	if err = json.Unmarshal(p.Eval, &k); err != nil || !k.Dat {
		return p, ErrCong
	}
	if k.VanTay != n.Cfg.VanTay() || p.VanTay != n.Cfg.VanTay() {
		return p, fmt.Errorf("%w: evaluated under configuration %s, built under %s, running %s", ErrCong, k.VanTay, p.VanTay, n.Cfg.VanTay())
	}
	if p.Corpus == CorpusQuan && (k.Vang == nil || k.Vang.Sha == "" || k.Vang.Sha != vangSha) {
		return p, fmt.Errorf("%w: the verdict was not measured on the current golden set", ErrCong)
	}
	if err = n.kiemTenTrung(ctx); err != nil {
		return p, err
	}
	if err = n.datAlias(ctx, Alias(p.Corpus), p.Collection); err != nil {
		return p, err
	}
	var old *int64
	if err = tx.QueryRow(ctx, `UPDATE rag_vector_versions SET state='retired', retired_at=clock_timestamp()
		WHERE corpus=$1 AND state='active' RETURNING id`, string(p.Corpus)).Scan(&old); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_vector_versions SET state='active', promoted_at=clock_timestamp(),
		parent_id=COALESCE($2, parent_id) WHERE id=$1`, id, old); err != nil {
		return p, err
	}
	p.State, p.ParentID = "active", old
	return p, tx.Commit(ctx)
}

// Rollback serves the active version's parent again: its collection must
// still exist (retired, not dropped). No rebuild: one alias swap, read
// back, one transaction. Tombstones belong to no version and were deleted
// from every live collection when they were made, so a removed place does
// not come back.
func (n Nap) Rollback(ctx context.Context, db CSDL, c Corpus) (from, to int64, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, khoaAlias); err != nil {
		return 0, 0, err
	}
	cur, err := PhienBanActive(ctx, tx, c)
	if err != nil {
		return 0, 0, err
	}
	if cur.ParentID == nil {
		return 0, 0, ErrKhongCoCha
	}
	par, err := DocPhienBan(ctx, tx, *cur.ParentID)
	if err != nil {
		return 0, 0, err
	}
	var dropped bool
	if err = tx.QueryRow(ctx, `SELECT dropped_at IS NOT NULL FROM rag_vector_versions WHERE id=$1`, par.ID).Scan(&dropped); err != nil {
		return 0, 0, err
	}
	if par.State != "retired" || dropped {
		return 0, 0, fmt.Errorf("%w: parent %d is %s (dropped %v)", ErrKhongCoCha, par.ID, par.State, dropped)
	}
	if err = n.kiemTenTrung(ctx); err != nil {
		return 0, 0, err
	}
	if err = n.datAlias(ctx, Alias(c), par.Collection); err != nil {
		return 0, 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_vector_versions SET state='retired', retired_at=clock_timestamp() WHERE id=$1`, cur.ID); err != nil {
		return 0, 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_vector_versions SET state='active', promoted_at=clock_timestamp(), retired_at=NULL WHERE id=$1`, par.ID); err != nil {
		return 0, 0, err
	}
	return cur.ID, par.ID, tx.Commit(ctx)
}

// BaoCaoDoiChieu is one reconciler pass, counts only.
type BaoCaoDoiChieu struct {
	SuaAlias int `json:"sua_alias"`
	XoaBan   int `json:"xoa_ban"`
}

// DoiChieu is the reconciler: for each corpus with an active version, the
// serving alias must point at its collection; a mismatch (a crash between
// the swap and the commit, a hand edit) is put back to what Postgres says
// and logged. Then retired versions beyond cfg.GiuBan (count and age) have
// their collections dropped; the version row stays, marked dropped.
func (n Nap) DoiChieu(ctx context.Context, tx pgx.Tx, logger *slog.Logger) (BaoCaoDoiChieu, error) {
	var b BaoCaoDoiChieu
	if _, err := tx.Exec(ctx, khoaAlias); err != nil {
		return b, err
	}
	for _, c := range Corpora {
		p, err := PhienBanActive(ctx, tx, c)
		if errors.Is(err, ErrKhongActive) {
			continue
		}
		if err != nil {
			return b, err
		}
		got, err := n.Kho.MoTaAlias(ctx, Alias(c))
		if err != nil {
			return b, fmt.Errorf("%w: %v", errMilvus, err)
		}
		if got != p.Collection {
			if err := n.datAlias(ctx, Alias(c), p.Collection); err != nil {
				return b, err
			}
			b.SuaAlias++
			if logger != nil {
				logger.Warn("rag alias reconciled to Postgres", "corpus", string(c), "version", p.ID)
			}
		}
		rows, err := tx.Query(ctx, `SELECT id, milvus_collection, retired_at FROM rag_vector_versions
			WHERE corpus=$1 AND state='retired' AND dropped_at IS NULL ORDER BY retired_at DESC, id DESC`, string(c))
		if err != nil {
			return b, err
		}
		type ban struct {
			id  int64
			ten string
			at  time.Time
		}
		var olds []ban
		for rows.Next() {
			var x ban
			if err := rows.Scan(&x.id, &x.ten, &x.at); err != nil {
				rows.Close()
				return b, err
			}
			olds = append(olds, x)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return b, err
		}
		cutoff := time.Now().Add(-time.Duration(n.Cfg.GiuBan.Ngay) * 24 * time.Hour)
		for i, x := range olds {
			// The newest retired version is the rollback target: kept
			// whatever its age.
			if i == 0 || (i < n.Cfg.GiuBan.ToiDa && x.at.After(cutoff)) {
				continue
			}
			if err := n.Kho.XoaCollection(ctx, x.ten); err != nil {
				return b, fmt.Errorf("%w: %v", errMilvus, err)
			}
			if _, err := tx.Exec(ctx, `UPDATE rag_vector_versions SET dropped_at=clock_timestamp() WHERE id=$1`, x.id); err != nil {
				return b, err
			}
			b.XoaBan++
		}
	}
	return b, nil
}

// DinhKyDoiChieu is the reconciler as a periodic task.
func (n Nap) DinhKyDoiChieu(logger *slog.Logger) jobs.DinhKy {
	return jobs.DinhKy{Ten: "rag.nap.doi_chieu", Nhip: time.Minute, Chay: func(ctx context.Context, tx pgx.Tx) error {
		_, err := n.DoiChieu(ctx, tx, logger)
		return err
	}}
}

// KetQuaTuDong is `core rag v-build --auto`.
type KetQuaTuDong struct {
	Dung     BaoCaoDung `json:"dung"`
	Cong     KetQuaCong `json:"cong"`
	TyLeDoi  float64    `json:"ty_le_doi"`
	TuDong   bool       `json:"tu_dong"`
	LyDoTay  string     `json:"ly_do_tay,omitempty"`
	PhienBan int64      `json:"phien_ban"`
}

// DungTuDong builds, evaluates and — only when the gate passes, at most
// cfg.TuDong.TyLeDoiToiDa of the documents changed, and the parent was built
// with the same configuration fingerprint and dense model — promotes. Any
// other change (a new model, a new chunker, a new sparse leg, a large
// change) stops at `evaluated` for a person to promote.
func (n Nap) DungTuDong(ctx context.Context, db CSDL, c Corpus, q NhungCauHoi, golden TapVang) (KetQuaTuDong, error) {
	var kq KetQuaTuDong
	rep, err := n.Dung(ctx, db, c)
	kq.Dung, kq.PhienBan = rep, rep.PhienBan
	if err != nil {
		return kq, err
	}
	if kq.Cong, err = n.DanhGia(ctx, db, rep.PhienBan, q, golden); err != nil {
		return kq, err
	}
	if !kq.Cong.Dat {
		return kq, ErrCong
	}
	if rep.Docs > 0 {
		kq.TyLeDoi = float64(rep.DocDoi) / float64(rep.Docs)
	}
	cur, err := PhienBanActive(ctx, db, c)
	switch {
	case errors.Is(err, ErrKhongActive):
		kq.LyDoTay = "chua_co_active"
	case err != nil:
		return kq, err
	case cur.VanTay != n.Cfg.VanTay() || cur.DenseModel != n.Dense.Model() || cur.SparseRev != n.Cfg.SparseRev() || cur.Chunker != n.Cfg.Chunker[c]:
		kq.LyDoTay = "doi_cau_hinh"
	case kq.TyLeDoi > n.Cfg.TuDong.TyLeDoiToiDa:
		kq.LyDoTay = "doi_nhieu"
	}
	if kq.LyDoTay != "" {
		return kq, nil
	}
	if _, err = n.Promote(ctx, db, rep.PhienBan, golden.Sha); err != nil {
		return kq, err
	}
	kq.TuDong = true
	return kq, nil
}
