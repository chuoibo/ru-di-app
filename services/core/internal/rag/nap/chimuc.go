package nap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/adk/model"

	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/repo"
)

// The indexer (S0 → S9 for one document at a time): it drains rag_dirty in
// order of first change, applies each change to every live collection of
// the corpus built with this configuration (active and candidates alike:
// the dual-write that lets a candidate be promoted with no gap), rewrites
// the hard-filter attributes of the place's rows in every live collection
// built with ANOTHER configuration (their vectors this pipeline cannot
// rebuild, but an allergen the enrichment found must reach the index that
// may be serving; if that rewrite fails the place is deleted there, fail
// closed), and deletes a removed or tombstoned place from every live
// collection whatever its configuration (so neither a rollback nor a
// promotion brings it back).
// It runs as a periodic task (a 60 s safety net) and on every message of
// the outbox lane 'rag' (the trigger's wake-up); both take the same lock.

// MaxMotLuot bounds the documents one pass takes.
const MaxMotLuot = 200

// MaxThuLai is how many failed passes a document gets before it leaves
// rag_dirty for the dead-letter table (`core rag v-dlq retry` brings it back).
const MaxThuLai = 3

// ChiMuc is the indexer. Model is the enrichment model (nil: none; a
// changed place then keeps only its stale enrichment's allergens, and its
// allergen certainty is lost until `core rag v-enrich` runs). TranGoi is the
// enrichment calls one pass may make.
type ChiMuc struct {
	Nap     Nap
	Model   model.LLM
	TranGoi int
	Logger  *slog.Logger
}

// BaoCaoChiMuc is one pass, counts only.
type BaoCaoChiMuc struct {
	Lay      int `json:"lay"`
	Ghi      int `json:"ghi"`
	Xoa      int `json:"xoa"`
	KhongDoi int `json:"khong_doi"`
	LamGiau  int `json:"lam_giau"`
	Hong     int `json:"hong"`
	VaoDLQ   int `json:"vao_dlq"`
	// KhacCauHinh counts rows of other-configuration collections whose
	// attributes were rewritten; XoaKhacCauHinh the documents deleted from
	// one because the rewrite could not be made.
	KhacCauHinh    int `json:"thuoc_tinh_khac_cau_hinh"`
	XoaKhacCauHinh int `json:"xoa_khac_cau_hinh"`
}

type dirty struct {
	id  string
	lan int64
	xoa bool
}

// MotLuot runs one pass inside tx (the periodic task's transaction, which
// holds its lock). A document that fails is counted in the dead-letter
// table and stays dirty until MaxThuLai; the pass itself fails only on a
// Postgres error.
func (c ChiMuc) MotLuot(ctx context.Context, tx pgx.Tx) (BaoCaoChiMuc, error) {
	var b BaoCaoChiMuc
	rows, err := tx.Query(ctx, `SELECT doc_id, lan, xoa FROM rag_dirty WHERE corpus='place' ORDER BY noticed_at, doc_id LIMIT $1`, MaxMotLuot)
	if err != nil {
		return b, err
	}
	var ds []dirty
	for rows.Next() {
		var d dirty
		if err := rows.Scan(&d.id, &d.lan, &d.xoa); err != nil {
			rows.Close()
			return b, err
		}
		ds = append(ds, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return b, err
	}
	b.Lay = len(ds)
	if len(ds) == 0 {
		return b, nil
	}
	song, err := PhienBanSong(ctx, tx, CorpusQuan)
	if err != nil {
		return b, err
	}
	var cungCauHinh, khacCauHinh []PhienBan
	for _, p := range song {
		if p.VanTay == c.Nap.Cfg.VanTay() && p.DenseModel == c.Nap.Dense.Model() {
			cungCauHinh = append(cungCauHinh, p)
		} else {
			khacCauHinh = append(khacCauHinh, p)
		}
	}
	ids := make([]string, len(ds))
	for i, d := range ds {
		ids[i] = d.id
	}
	places, err := repo.Repository{Q: tx}.PlacesByID(ctx, ids)
	if err != nil {
		return b, err
	}
	byID := map[string]repo.Place{}
	for _, p := range places {
		byID[p.ID] = p
	}
	bia, err := biaTheoLyDo(ctx, tx)
	if err != nil {
		return b, err
	}
	enr, err := DocLamGiau(ctx, tx, ids)
	if err != nil {
		return b, err
	}
	// Enrich what changed, within the pass's ceiling.
	if c.Model != nil && c.TranGoi > 0 {
		var need []HoSoQuan
		for _, d := range ds {
			if p, ok := byID[d.id]; ok && !d.xoa {
				if h, bo := DungHoSo(p); !bo {
					if lg := enr[d.id]; lg == nil || lg.NguonHash != h.NguonHash {
						need = append(need, h)
					}
				}
			}
		}
		if len(need) > 0 {
			done, hong, _ := ChayLamGiau(ctx, c.Model, c.Nap.Cfg, c.TranGoi, need)
			if err := GhiLamGiau(ctx, tx, done); err != nil {
				return b, err
			}
			for i := range done {
				lg := done[i]
				enr[lg.PlaceID] = &lg
			}
			b.LamGiau = len(done)
			for id, e := range hong {
				if err := GhiDLQ(ctx, tx, CorpusQuan, id, ChangLamGiau, e); err != nil {
					return b, err
				}
			}
		}
	}
	chunker := c.Nap.Cfg.Chunker[CorpusQuan]
	var ok []dirty
	for _, d := range ds {
		p, exists := byID[d.id]
		reason, tomb := bia[d.id]
		var h HoSoQuan
		bo := false
		if exists {
			h, bo = DungHoSo(p)
		}
		gone := !exists || bo || (tomb && reason != "unsafe" && reason != "source_deleted")
		if gone {
			if err := c.xoaMoiNoi(ctx, song, d.id); err != nil {
				if e := c.hong(ctx, tx, d, ChangXoa, err, &b); e != nil {
					return b, e
				}
				continue
			}
			if !exists {
				if _, err := tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) VALUES('place',$1,'source_deleted') ON CONFLICT (corpus,doc_id) DO NOTHING`, d.id); err != nil {
					return b, err
				}
			} else if bo {
				if _, err := tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) VALUES('place',$1,'unsafe') ON CONFLICT (corpus,doc_id) DO NOTHING`, d.id); err != nil {
					return b, err
				}
			}
			b.Xoa++
			ok = append(ok, d)
			continue
		}
		if tomb {
			// Back and safe: an automatic tombstone lifts.
			if _, err := tx.Exec(ctx, `DELETE FROM rag_tombstones WHERE corpus='place' AND doc_id=$1 AND reason IN ('unsafe','source_deleted')`, d.id); err != nil {
				return b, err
			}
		}
		rows := DoanQuan(h, ApDung(enr[d.id], h.NguonHash), chunker)
		if _, err := c.Nap.Vector(ctx, tx, rows); err != nil {
			if e := c.hong(ctx, tx, d, ChangNhung, err, &b); e != nil {
				return b, e
			}
			continue
		}
		wrote, err := c.ghiMoiNoi(ctx, cungCauHinh, d.id, rows)
		if err != nil {
			if e := c.hong(ctx, tx, d, ChangGhi, err, &b); e != nil {
				return b, e
			}
			continue
		}
		if err := c.thuocTinhNoiKhac(ctx, khacCauHinh, d.id, rows, &b); err != nil {
			if e := c.hong(ctx, tx, d, ChangGhi, err, &b); e != nil {
				return b, e
			}
			continue
		}
		if wrote {
			b.Ghi++
		} else {
			b.KhongDoi++
		}
		ok = append(ok, d)
	}
	if len(ok) > 0 {
		ids, lans := make([]string, len(ok)), make([]int64, len(ok))
		for i, d := range ok {
			ids[i], lans[i] = d.id, d.lan
		}
		// Only rows no change reached while this pass worked.
		if _, err := tx.Exec(ctx, `DELETE FROM rag_dirty d USING unnest($1::text[], $2::bigint[]) AS s(id, lan)
			WHERE d.corpus='place' AND d.doc_id=s.id AND d.lan=s.lan`, ids, lans); err != nil {
			return b, err
		}
		if err := XoaDLQ(ctx, tx, CorpusQuan, ids); err != nil {
			return b, err
		}
	}
	return b, nil
}

func (c ChiMuc) hong(ctx context.Context, tx pgx.Tx, d dirty, chang string, err error, b *BaoCaoChiMuc) error {
	b.Hong++
	if c.Logger != nil {
		c.Logger.Warn("rag indexer: a document failed", "stage", chang, "code", MaLoi(err))
	}
	if e := GhiDLQ(ctx, tx, CorpusQuan, d.id, chang, err); e != nil {
		return e
	}
	var n int
	if e := tx.QueryRow(ctx, `SELECT so_lan FROM rag_ingest_dlq WHERE corpus='place' AND doc_id=$1 AND chang=$2`, d.id, chang).Scan(&n); e != nil {
		return e
	}
	if n >= MaxThuLai {
		b.VaoDLQ++
		_, e := tx.Exec(ctx, `DELETE FROM rag_dirty WHERE corpus='place' AND doc_id=$1`, d.id)
		return e
	}
	return nil
}

// thuocTinhNoiKhac carries a document's current hard-filter attributes into
// every live collection of another configuration: a partial rewrite of its
// rows there. A rewrite that fails, or a document with no chunk left to
// take attributes from, deletes the document from that collection instead:
// a stale allergen list must never stay servable.
func (c ChiMuc) thuocTinhNoiKhac(ctx context.Context, ps []PhienBan, docID string, rows []Hang, b *BaoCaoChiMuc) error {
	for _, p := range ps {
		if len(rows) > 0 {
			n, err := c.Nap.Kho.CapNhatThuocTinh(ctx, p.Collection, docID, rows[0])
			if err == nil {
				b.KhacCauHinh += n
				continue
			}
		}
		var ids []string
		for _, f := range FacetsQuan {
			ids = append(ids, ChunkID(docID, f, p.Chunker))
		}
		if err := c.Nap.Kho.XoaID(ctx, p.Collection, ids); err != nil {
			return fmt.Errorf("%w: %v", errMilvus, err)
		}
		b.XoaKhacCauHinh++
	}
	return nil
}

// xoaMoiNoi deletes every facet of a document from every live collection.
func (c ChiMuc) xoaMoiNoi(ctx context.Context, song []PhienBan, docID string) error {
	for _, p := range song {
		var ids []string
		for _, f := range FacetsQuan {
			ids = append(ids, ChunkID(docID, f, p.Chunker))
		}
		if err := c.Nap.Kho.XoaID(ctx, p.Collection, ids); err != nil {
			return fmt.Errorf("%w: %v", errMilvus, err)
		}
	}
	return nil
}

// ghiMoiNoi upserts a document's rows into every live collection of this
// configuration and deletes the facets it no longer has. wrote is false when
// every collection already held exactly these rows.
func (c ChiMuc) ghiMoiNoi(ctx context.Context, ps []PhienBan, docID string, rows []Hang) (bool, error) {
	have := map[string]bool{}
	for _, r := range rows {
		have[r.Facet] = true
	}
	wrote := false
	for _, p := range ps {
		var stale []string
		for _, f := range FacetsQuan {
			if !have[f] {
				stale = append(stale, ChunkID(docID, f, p.Chunker))
			}
		}
		if len(stale) > 0 {
			if err := c.Nap.Kho.XoaID(ctx, p.Collection, stale); err != nil {
				return wrote, fmt.Errorf("%w: %v", errMilvus, err)
			}
		}
		if len(rows) > 0 {
			if err := c.Nap.Kho.Upsert(ctx, p.Collection, rows); err != nil {
				return wrote, fmt.Errorf("%w: %v", errMilvus, err)
			}
			wrote = true
		}
	}
	return wrote, nil
}

// DinhKy is the indexer as a periodic task: one pass a minute under the
// task's lock, the safety net under the lane.
func (c ChiMuc) DinhKy() jobs.DinhKy {
	return jobs.DinhKy{Ten: "rag.nap.chi_muc", Nhip: time.Minute, Chay: func(ctx context.Context, tx pgx.Tx) error {
		_, err := c.MotLuot(ctx, tx)
		return err
	}}
}

// XuLyTin is the lane 'rag' handler: one pass now, under the same lock as
// the periodic task. A pass another process holds is not queued (it is
// doing the same work). The message carries nothing but an id: the rows
// say what changed.
func (c ChiMuc) XuLyTin(pool *pgxpool.Pool) func(ctx context.Context, queue string, m jobs.Message) error {
	return func(ctx context.Context, queue string, _ jobs.Message) error {
		if queue != "rag" {
			return fmt.Errorf("nap: lane %q is not the indexer's", queue)
		}
		_, err := jobs.MotLuot(ctx, pool, c.DinhKy())
		return err
	}
}

// TrangThai is `core rag v-status`: per corpus, ids, states, counts and
// ages. No content, no person.
type TrangThai struct {
	Corpus       Corpus   `json:"corpus"`
	Active       int64    `json:"active,omitempty"`
	Collection   string   `json:"collection,omitempty"`
	AliasTro     string   `json:"alias_tro,omitempty"`
	AliasKhop    bool     `json:"alias_khop"`
	DemMilvus    int64    `json:"dem_milvus"`
	Chunks       int      `json:"chunks"`
	Dirty        int      `json:"dirty"`
	DirtyCuNhatS float64  `json:"dirty_cu_nhat_giay"`
	DLQ          int      `json:"dlq"`
	ChoDuyet     int      `json:"cho_duyet"`
	ThieuLamGiau int      `json:"thieu_lam_giau"`
	PhienBanSong int      `json:"phien_ban_song"`
	NhanhSuyGiam []string `json:"nhanh_suy_giam"`
	Loi          string   `json:"loi,omitempty"`
}

// DocTrangThai reads the status of every corpus. A Milvus that does not
// answer is reported (loi), not fatal: the status must work when the index
// does not.
func (n Nap) DocTrangThai(ctx context.Context, q Querier) ([]TrangThai, error) {
	var out []TrangThai
	for _, c := range Corpora {
		t := TrangThai{Corpus: c}
		if n.Cfg.Thua.CheDo == ThuaBM25 {
			t.NhanhSuyGiam = append(t.NhanhSuyGiam, "milco_tat")
		}
		if p, err := PhienBanActive(ctx, q, c); err == nil {
			t.Active, t.Collection, t.Chunks = p.ID, p.Collection, p.Chunks
			if got, err := n.Kho.MoTaAlias(ctx, Alias(c)); err != nil {
				t.Loi = "milvus"
			} else {
				t.AliasTro, t.AliasKhop = got, got == p.Collection
				if cnt, err := n.Kho.Dem(ctx, p.Collection); err == nil {
					t.DemMilvus = cnt
				} else {
					t.Loi = "milvus"
				}
			}
		} else if !errors.Is(err, ErrKhongActive) {
			return nil, err
		} else {
			t.NhanhSuyGiam = append(t.NhanhSuyGiam, "khong_active")
		}
		if err := q.QueryRow(ctx, `SELECT count(*), COALESCE(EXTRACT(EPOCH FROM clock_timestamp() - min(noticed_at)), 0)
			FROM rag_dirty WHERE corpus=$1`, string(c)).Scan(&t.Dirty, &t.DirtyCuNhatS); err != nil {
			return nil, err
		}
		if err := q.QueryRow(ctx, `SELECT count(*) FROM rag_ingest_dlq WHERE corpus=$1`, string(c)).Scan(&t.DLQ); err != nil {
			return nil, err
		}
		if err := q.QueryRow(ctx, `SELECT count(*) FROM rag_vector_versions WHERE corpus=$1 AND dropped_at IS NULL AND state<>'failed'`, string(c)).Scan(&t.PhienBanSong); err != nil {
			return nil, err
		}
		if c == CorpusQuan {
			if err := q.QueryRow(ctx, `SELECT count(*) FROM place_enrichments WHERE extractor=$1 AND can_duyet AND review='auto'`, Extractor).Scan(&t.ChoDuyet); err != nil {
				return nil, err
			}
			if err := q.QueryRow(ctx, `SELECT count(*) FROM places p WHERE NOT EXISTS (SELECT 1 FROM place_enrichments e
				WHERE e.place_id=p.id AND e.extractor=$1 AND e.review<>'rejected')`, Extractor).Scan(&t.ThieuLamGiau); err != nil {
				return nil, err
			}
		}
		t.DirtyCuNhatS = float64(int64(t.DirtyCuNhatS*10)) / 10
		sort.Strings(t.NhanhSuyGiam)
		out = append(out, t)
	}
	return out, nil
}
