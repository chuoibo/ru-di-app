package nap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/repo"
)

// The indexer (S0 → S9): it drains rag_dirty, most urgent first (a
// tombstone or a review before a source change, a background re-check
// last), then in order of first change, and brings every live collection
// to what Postgres says now, writing only what differs:
//
//   - a removed, tombstoned or unenriched place leaves every live collection
//     whatever its configuration (so neither a rollback nor a promotion
//     brings it back);
//   - in every live collection built with THIS configuration (active and
//     candidates alike: the dual-write that lets a candidate be promoted with
//     no gap) a row whose content hash and attribute fingerprint
//     (DauThuocTinh) both match is left alone, one whose hash matches gets a
//     partial update of its attributes, one whose text changed is rewritten
//     whole once its vector is at hand -- and until then its attributes are
//     rewritten anyway: an allergen or a closure never waits for an
//     embedding;
//   - in every live collection built with ANOTHER configuration (whose
//     vectors this pipeline cannot rebuild) the attributes are rewritten, or
//     the place deleted there if that fails (fail closed).
//
// Vectors come from the cache, then from the online door within a budget
// (NguongOnline per pass, TranOnlineGio per hour); a bulk change waits for
// the batch door instead (LuotLo). A version still being built is skipped:
// its build re-marks every place when it ends (Dung).
//
// The rag-indexer runs a pass whenever rag_dirty is notified (rag_danh_dau),
// at least every few seconds while there is a backlog, and on the lane
// 'rag' where there is a broker; every pass takes the same lock.

// MaxMotLuot bounds the documents one pass takes.
const MaxMotLuot = 200

// MaxThuLai is how many failed passes a document gets before it leaves
// rag_dirty for the dead-letter table (`core rag v-dlq retry` brings it back).
// A document that does leaves every live collection too: what failed may be
// the write that carried a new allergen.
const MaxThuLai = 3

// NguongOnline is the most documents one pass embeds online. A pass that
// finds more without a vector is a bulk change: it waits for the batch door
// (half the price) when there is one.
const NguongOnline = 50

// TranOnlineGio is the most documents the indexer embeds online in any
// hour (HanMuc); the rest wait for the batch door, or for the budget.
const TranOnlineGio = 300

// ChoLoToiDa is how long a document waiting for the batch door sleeps before
// a pass looks at it again on its own; the batch turn that brings its vector
// wakes it sooner.
const ChoLoToiDa = 6 * time.Hour

// choTran is the wait of a document over the hour's online budget when
// there is no batch door.
const choTran = 5 * time.Minute

// ToiDaLo is the most documents one batch job carries: a job of the whole
// catalogue (~25k chunks) was refused 429 RESOURCE_EXHAUSTED at creation
// (enqueued-token quota), 5,000 was accepted (probe 2026-09-29).
const ToiDaLo = 4000

// choLoi is the backoff after a document's n-th failed pass: 15 s, 60 s.
func choLoi(n int) time.Duration {
	d := 15 * time.Second
	for i := 1; i < n; i++ {
		d *= 4
	}
	return d
}

// ChiMuc is the indexer. Model is the enrichment model (nil: none; a
// changed place then keeps only its stale enrichment's allergens, and its
// allergen certainty is lost until `core rag v-enrich` runs). TranGoi is the
// enrichment calls one pass may make.
type ChiMuc struct {
	Nap     Nap
	Model   model.LLM
	TranGoi int
	Logger  *slog.Logger
	// Pool writes what must outlive a pass that rolls back: the vectors it
	// paid for. nil: the pass's own transaction (tests).
	Pool CSDL
	// Lo is the batch door (nil: none; a bulk change is then embedded
	// online within the budget, NguongOnline at a time).
	Lo NhungLo
	// HanMuc is the online budget and the backoff of a failing online door
	// (nil: no budget, a fixed backoff; tests).
	HanMuc *HanMuc
	// GiamSat receives the loop's heartbeat (nil: none).
	GiamSat *GiamSat
}

// BaoCaoChiMuc is one pass, counts only.
type BaoCaoChiMuc struct {
	Lay int `json:"lay"`
	// Ghi: documents rewritten whole (new text, its vector at hand).
	Ghi int `json:"ghi"`
	Xoa int `json:"xoa"`
	// ChoLamGiau: places kept out of the index until their enrichment
	// arrives (no current one).
	ChoLamGiau int `json:"cho_lam_giau"`
	// KhongDoi: documents every collection already held exactly; MotPhan:
	// documents whose text was unchanged and whose attributes were
	// rewritten in place.
	KhongDoi int `json:"khong_doi"`
	MotPhan  int `json:"mot_phan"`
	// ChoNhung: documents whose new text waits for its vector (their
	// attributes already rewritten where a row exists); NhungOnline: vectors
	// paid for online in this pass.
	ChoNhung    int `json:"cho_nhung"`
	NhungOnline int `json:"nhung_online"`
	LamGiau     int `json:"lam_giau"`
	Hong        int `json:"hong"`
	VaoDLQ      int `json:"vao_dlq"`
	// KhacCauHinh counts rows of other-configuration collections whose
	// attributes were rewritten; XoaKhacCauHinh the documents deleted from
	// one because the rewrite could not be made.
	KhacCauHinh    int `json:"thuoc_tinh_khac_cau_hinh"`
	XoaKhacCauHinh int `json:"xoa_khac_cau_hinh"`
}

type dirty struct {
	id       string
	lan      int64
	xoa      bool
	choNhung bool
}

// viec is one document to write, and what happened to it.
type viec struct {
	d   dirty
	row Hang
	// canVec: some collection lacks this text; coVec: its vector is at hand.
	canVec, coVec bool
	ghi, motPhan  bool
	// cho > 0 defers the document (nhung: until the batch door brings its
	// vector).
	cho   time.Duration
	nhung bool
	loi   error
}

// HanMuc is the indexer's online embedding budget (a sliding hour) and the
// backoff of an online door that fails. Safe for concurrent use; one per
// process.
type HanMuc struct {
	Gio int

	mu   sync.Mutex
	dung []mocDung
	loi  int
}

type mocDung struct {
	at time.Time
	n  int
}

// NewHanMuc is a budget of gio documents an hour.
func NewHanMuc(gio int) *HanMuc { return &HanMuc{Gio: gio} }

// Con is what is left of the hour's budget.
func (h *HanMuc) Con(now time.Time) int {
	if h == nil {
		return int(^uint(0) >> 1)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	keep := h.dung[:0]
	used := 0
	for _, m := range h.dung {
		if now.Sub(m.at) < time.Hour {
			keep = append(keep, m)
			used += m.n
		}
	}
	h.dung = keep
	return max(h.Gio-used, 0)
}

// Tieu spends n and clears the failure streak.
func (h *HanMuc) Tieu(now time.Time, n int) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dung = append(h.dung, mocDung{at: now, n: n})
	h.loi = 0
}

// Loi records a failed call to the online door and returns how long the
// documents it was for wait: 30 s, doubling to 10 min. A door that is down
// shows as freshness lag, never as dead letters.
func (h *HanMuc) Loi() time.Duration {
	if h == nil {
		return 30 * time.Second
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.loi++
	d := 30 * time.Second
	for i := 1; i < h.loi && d < 10*time.Minute; i++ {
		d *= 2
	}
	return min(d, 10*time.Minute)
}

func (c ChiMuc) pool(tx pgx.Tx) CSDL {
	if c.Pool != nil {
		return c.Pool
	}
	return tx
}

// MotLuot runs one pass inside tx (the periodic task's transaction, which
// holds its lock). A document that fails is counted in the dead-letter
// table and deferred (choLoi) until MaxThuLai; the pass itself fails on a
// Postgres error, or when Milvus does not answer the first read (then
// nothing is counted against any document).
func (c ChiMuc) MotLuot(ctx context.Context, tx pgx.Tx) (BaoCaoChiMuc, error) {
	var b BaoCaoChiMuc
	// The pass's own tombstone and enrichment writes concern the documents
	// it is indexing: they must not mark them again (schema_nap_4.sql).
	if _, err := tx.Exec(ctx, `SELECT set_config('rag.chi_muc','on',true)`); err != nil {
		return b, err
	}
	var docAt time.Time
	rows, err := tx.Query(ctx, `SELECT doc_id, lan, xoa, cho_nhung, clock_timestamp() FROM rag_dirty
		WHERE corpus='place' AND (cho_den IS NULL OR cho_den <= clock_timestamp())
		ORDER BY uu_tien DESC, noticed_at, doc_id LIMIT $1`, MaxMotLuot)
	if err != nil {
		return b, err
	}
	var ds []dirty
	for rows.Next() {
		var d dirty
		if err := rows.Scan(&d.id, &d.lan, &d.xoa, &d.choNhung, &docAt); err != nil {
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
	tatCa, err := PhienBanSong(ctx, tx, CorpusQuan)
	if err != nil {
		return b, err
	}
	var song, cungCauHinh, khacCauHinh []PhienBan
	for _, p := range tatCa {
		if p.State == "building" {
			// Its collection may not exist yet, and its build writes a
			// snapshot over anything written now: the build re-marks every
			// place when it ends.
			continue
		}
		song = append(song, p)
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
	dm, err := DocDanhMuc(ctx, tx, ids)
	if err != nil {
		return b, err
	}
	// Enrich what changed, within the pass's ceiling.
	if c.Model != nil && c.TranGoi > 0 {
		var need []HoSoQuan
		for _, d := range ds {
			if p, ok := byID[d.id]; ok && !d.xoa {
				if h, bo := DungHoSo(p); !bo {
					if lg := enr[d.id]; lg == nil || (!lg.Ngoai && lg.NguonHash != h.NguonHash) {
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
	var ra []dirty // to delete everywhere
	choLamGiau := map[string]bool{}
	var viet []viec
	for _, d := range ds {
		p, exists := byID[d.id]
		reason, tomb := bia[d.id]
		var h HoSoQuan
		bo := false
		if exists {
			h, bo = DungHoSo(p)
		}
		if !exists || bo || (tomb && reason != "unsafe" && reason != "source_deleted") {
			if !exists {
				if _, err := tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) VALUES('place',$1,'source_deleted') ON CONFLICT (corpus,doc_id) DO NOTHING`, d.id); err != nil {
					return b, err
				}
			} else if bo {
				if _, err := tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) VALUES('place',$1,'unsafe') ON CONFLICT (corpus,doc_id) DO NOTHING`, d.id); err != nil {
					return b, err
				}
			}
			ra = append(ra, d)
			continue
		}
		if tomb {
			// Back and safe: an automatic tombstone lifts.
			if _, err := tx.Exec(ctx, `DELETE FROM rag_tombstones WHERE corpus='place' AND doc_id=$1 AND reason IN ('unsafe','source_deleted')`, d.id); err != nil {
				return b, err
			}
		}
		t := ApDung(enr[d.id], h.NguonHash)
		t.DanhMuc = dm[d.id]
		if !t.Co {
			// No current enrichment: out of every live collection until
			// vnlocal's arrives (ingest then marks the place dirty), the
			// rule a build follows (ChuanBiQuan).
			choLamGiau[d.id] = true
			ra = append(ra, d)
			continue
		}
		rs, err := DoanQuan(h, t, chunker)
		if err == nil && len(rs) == 0 {
			// No text: no row (a build leaves it out the same way).
			ra = append(ra, d)
			continue
		}
		if err == nil && (len(rs) != 1 || rs[0].ChunkID != d.id) {
			err = fmt.Errorf("%w: %d rows for one place", ErrCauHinh, len(rs))
		}
		if err != nil {
			if e := c.hong(ctx, tx, song, d, ChangNhung, err, &b); e != nil {
				return b, e
			}
			continue
		}
		viet = append(viet, viec{d: d, row: rs[0]})
	}

	// Deletes: one call per live collection.
	loiXoa, err := c.xoaLo(ctx, song, ra)
	if err != nil {
		return b, err
	}
	for _, d := range ra {
		if e := loiXoa[d.id]; e != nil {
			if err := c.hong(ctx, tx, song, d, ChangXoa, e, &b); err != nil {
				return b, err
			}
			continue
		}
		if choLamGiau[d.id] {
			b.ChoLamGiau++
		} else {
			b.Xoa++
		}
		ok = append(ok, d)
	}

	if len(viet) > 0 {
		if err := c.vietLo(ctx, tx, cungCauHinh, khacCauHinh, viet, &b); err != nil {
			return b, err
		}
	}
	var cho []viec
	for i := range viet {
		v := &viet[i]
		switch {
		case v.loi != nil:
			if err := c.hong(ctx, tx, song, v.d, ChangGhi, v.loi, &b); err != nil {
				return b, err
			}
			continue
		case v.cho > 0:
			cho = append(cho, *v)
			if v.canVec && !v.coVec {
				b.ChoNhung++
			}
			continue
		case v.ghi:
			b.Ghi++
		case v.motPhan:
			b.MotPhan++
		default:
			b.KhongDoi++
		}
		ok = append(ok, v.d)
	}

	if len(ok) > 0 {
		ids, lans := make([]string, len(ok)), make([]int64, len(ok))
		for i, d := range ok {
			ids[i], lans[i] = d.id, d.lan
		}
		// Only rows no change reached while this pass worked. A row one did
		// reach stays, and its oldest unindexed change is no older than this
		// pass's read.
		if _, err := tx.Exec(ctx, `DELETE FROM rag_dirty d USING unnest($1::text[], $2::bigint[]) AS s(id, lan)
			WHERE d.corpus='place' AND d.doc_id=s.id AND d.lan=s.lan`, ids, lans); err != nil {
			return b, err
		}
		if _, err := tx.Exec(ctx, `UPDATE rag_dirty d SET noticed_at = GREATEST(d.noticed_at, $3)
			FROM unnest($1::text[], $2::bigint[]) AS s(id, lan)
			WHERE d.corpus='place' AND d.doc_id=s.id AND d.lan<>s.lan`, ids, lans, docAt); err != nil {
			return b, err
		}
		if err := XoaDLQ(ctx, tx, CorpusQuan, ids); err != nil {
			return b, err
		}
	}
	if len(cho) > 0 {
		ids, lans := make([]string, len(cho)), make([]int64, len(cho))
		giay, nhung := make([]float64, len(cho)), make([]bool, len(cho))
		for i, v := range cho {
			ids[i], lans[i], giay[i], nhung[i] = v.d.id, v.d.lan, v.cho.Seconds(), v.nhung
		}
		// A row a change reached meanwhile is not deferred: the change
		// may be the one that unblocks it.
		// Its attributes are current as of this pass's read: what waits is
		// its vector (cho_tu dates the wait).
		if _, err := tx.Exec(ctx, `UPDATE rag_dirty d SET cho_den = clock_timestamp() + s.giay * interval '1 second',
				cho_nhung = d.cho_nhung OR s.nhung, cho_tu = COALESCE(d.cho_tu, $5),
				noticed_at = GREATEST(d.noticed_at, $5)
			FROM unnest($1::text[], $2::bigint[], $3::float8[], $4::bool[]) AS s(id, lan, giay, nhung)
			WHERE d.corpus='place' AND d.doc_id=s.id AND d.lan=s.lan`, ids, lans, giay, nhung, docAt); err != nil {
			return b, err
		}
	}
	return b, nil
}

// vietLo brings the documents viet to every live collection: it reads what
// each same-configuration collection holds (one call each; a Milvus that
// does not answer fails the pass), finds the vectors the new texts need,
// and writes each collection in at most two calls, a whole-row upsert and
// a partial update, retried row by row when a batch is refused. Outcomes
// are left on each viec.
func (c ChiMuc) vietLo(ctx context.Context, tx pgx.Tx, cung, khac []PhienBan, viet []viec, b *BaoCaoChiMuc) error {
	ids := make([]string, len(viet))
	for i := range viet {
		ids[i] = viet[i].d.id
	}
	model := c.Nap.Dense.Model()
	luu := make([]map[string]KhoaHang, len(cung))
	for j, p := range cung {
		ks, err := c.Nap.Kho.KhoaTheoDoc(ctx, p.Collection, ids)
		if err != nil {
			return fmt.Errorf("%w: %v", errMilvus, err)
		}
		luu[j] = make(map[string]KhoaHang, len(ks))
		for _, k := range ks {
			luu[j][k.ChunkID] = k
		}
	}
	// A place the version's dedupe left out stays out of it.
	trung, err := docTrung(ctx, tx, cung, ids)
	if err != nil {
		return err
	}
	for j, p := range cung {
		var thua []*viec
		for i := range viet {
			v := &viet[i]
			if _, ok := luu[j][v.row.ChunkID]; ok && trung[p.ID][v.d.id] {
				thua = append(thua, v)
			}
		}
		c.ghiTungLo(ctx, thua, func(rows []Hang) error {
			ids := make([]string, len(rows))
			for i, r := range rows {
				ids[i] = r.ChunkID
			}
			return c.Nap.Kho.XoaID(ctx, p.Collection, ids)
		}, func(*viec) {})
	}
	rev := c.Nap.Cfg.SparseRev()
	var can []*viec
	for i := range viet {
		v := &viet[i]
		v.row.DenseModel, v.row.SparseRev = model, rev
		for j, p := range cung {
			if trung[p.ID][v.d.id] {
				continue
			}
			k, ok := luu[j][v.row.ChunkID]
			if !ok || k.ContentHash != v.row.ContentHash || k.DenseModel != model {
				v.canVec = true
			}
		}
		if v.canVec {
			can = append(can, v)
		}
	}
	if err := c.vectorLo(ctx, tx, can, b); err != nil {
		return err
	}
	for j, p := range cung {
		var whole, part []*viec
		for i := range viet {
			v := &viet[i]
			if trung[p.ID][v.d.id] {
				continue
			}
			k, ok := luu[j][v.row.ChunkID]
			switch {
			case ok && k.ContentHash == v.row.ContentHash && k.DenseModel == model:
				if k.Dau != DauThuocTinh(v.row) {
					part = append(part, v)
				}
			case v.coVec:
				whole = append(whole, v)
			case ok:
				// New text, no vector yet: the attributes go now.
				part = append(part, v)
			}
			// A new place with no vector yet has no row to update: it
			// enters with its vector.
		}
		c.ghiTungLo(ctx, whole, func(rows []Hang) error {
			return c.Nap.Kho.Upsert(ctx, p.Collection, rows)
		}, func(v *viec) { v.ghi = true })
		c.ghiTungLo(ctx, part, func(rows []Hang) error {
			_, err := c.Nap.Kho.CapNhatThuocTinhLo(ctx, p.Collection, rows)
			return err
		}, func(v *viec) { v.motPhan = true })
	}
	for i := range viet {
		v := &viet[i]
		if err := c.thuocTinhNoiKhac(ctx, khac, v.d.id, []Hang{v.row}, b); err != nil && v.loi == nil {
			v.loi = err
		}
	}
	return nil
}

// docTrung is, per version of ps, which of the documents ids its build's
// dedupe left out.
func docTrung(ctx context.Context, q Querier, ps []PhienBan, ids []string) (map[int64]map[string]bool, error) {
	out := map[int64]map[string]bool{}
	if len(ps) == 0 || len(ids) == 0 {
		return out, nil
	}
	vs := make([]int64, len(ps))
	for i, p := range ps {
		vs[i] = p.ID
	}
	rows, err := q.Query(ctx, `SELECT phien_ban, doc_id FROM rag_trung WHERE phien_ban = ANY($1::bigint[]) AND doc_id = ANY($2::text[])`, vs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v int64
		var d string
		if err := rows.Scan(&v, &d); err != nil {
			return nil, err
		}
		if out[v] == nil {
			out[v] = map[string]bool{}
		}
		out[v][d] = true
	}
	return out, rows.Err()
}

// ghiTungLo writes vs in one call, or one by one when the batch is refused,
// so a bad row fails only its own document.
func (c ChiMuc) ghiTungLo(_ context.Context, vs []*viec, ghi func([]Hang) error, xong func(*viec)) {
	if len(vs) == 0 {
		return
	}
	rows := make([]Hang, len(vs))
	for i, v := range vs {
		rows[i] = v.row
	}
	if ghi(rows) == nil {
		for _, v := range vs {
			xong(v)
		}
		return
	}
	for _, v := range vs {
		if err := ghi([]Hang{v.row}); err != nil {
			if v.loi == nil {
				v.loi = fmt.Errorf("%w: %v", errMilvus, err)
			}
			continue
		}
		xong(v)
	}
}

// vectorLo finds the vectors of the documents whose text some collection
// lacks: the cache first; then, for what is still missing, the online door
// when it is few enough (NguongOnline) and the hour's budget allows, the
// batch door otherwise (the documents wait, cho_nhung). A document already
// waiting for the batch door is never embedded online: its vector is
// ordered. A failing online door defers its documents (HanMuc.Loi) and
// never counts against them.
func (c ChiMuc) vectorLo(ctx context.Context, tx pgx.Tx, vs []*viec, b *BaoCaoChiMuc) error {
	if len(vs) == 0 {
		return nil
	}
	enc := c.Nap.Dense
	if enc.Dims() != c.Nap.Cfg.Dense.Dims {
		return fmt.Errorf("%w: encoder has %d dims, configuration %d", ErrCauHinh, enc.Dims(), c.Nap.Cfg.Dense.Dims)
	}
	task := TaskTaiLieu(c.Nap.Cfg)
	hashes := make([]string, 0, len(vs))
	for _, v := range vs {
		hashes = append(hashes, v.row.ContentHash)
	}
	sort.Strings(hashes)
	have, err := (BoNhoPG{Q: tx}).LayNhung(ctx, enc.Model(), enc.Dims(), task, hashes)
	if err != nil {
		return err
	}
	var online []*viec
	for _, v := range vs {
		if vec, ok := have[v.row.ContentHash]; ok && len(vec) == enc.Dims() {
			v.row.Dense, v.coVec = vec, true
			continue
		}
		if v.d.choNhung && c.Lo != nil {
			v.cho, v.nhung = ChoLoToiDa, true
			continue
		}
		online = append(online, v)
	}
	now := time.Now()
	con := c.HanMuc.Con(now)
	n := len(online)
	switch {
	case n == 0:
		return nil
	case n <= NguongOnline && n <= con:
	case c.Lo != nil:
		for _, v := range online {
			v.cho, v.nhung = ChoLoToiDa, true
		}
		return nil
	default:
		k := min(NguongOnline, con)
		for _, v := range online[k:] {
			v.cho = choTran
		}
		online = online[:k]
	}
	if len(online) == 0 {
		return nil
	}
	rows := make([]Hang, len(online))
	for i, v := range online {
		rows[i] = v.row
	}
	moi, err := NhungHang(ctx, enc, BoNhoPG{Q: c.pool(tx)}, c.Nap.Cfg, rows)
	if err != nil {
		wait := c.HanMuc.Loi()
		if c.Logger != nil {
			c.Logger.Warn("rag indexer: the online embedding failed; documents deferred",
				"docs", len(online), "retry_s", wait.Seconds(), "code", MaLoi(err), "error", rutGon(err))
		}
		for _, v := range online {
			v.cho = wait
		}
		return nil
	}
	c.HanMuc.Tieu(now, moi)
	b.NhungOnline += moi
	for i, v := range online {
		v.row.Dense, v.coVec = rows[i].Dense, true
	}
	return nil
}

// rutGon is an error's text, cut short for a log line.
func rutGon(err error) string {
	s := err.Error()
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// xoaLo deletes the documents ra from every live collection, one call per
// collection, then one by one in a collection that refuses the batch. It
// returns the documents that could not be deleted somewhere.
func (c ChiMuc) xoaLo(ctx context.Context, song []PhienBan, ra []dirty) (map[string]error, error) {
	loi := map[string]error{}
	if len(ra) == 0 {
		return loi, nil
	}
	var ids []string
	for _, d := range ra {
		ids = append(ids, MoiIDQuan(d.id)...)
	}
	for _, p := range song {
		if c.Nap.Kho.XoaID(ctx, p.Collection, ids) == nil {
			continue
		}
		for _, d := range ra {
			if err := c.Nap.Kho.XoaID(ctx, p.Collection, MoiIDQuan(d.id)); err != nil && loi[d.id] == nil {
				loi[d.id] = fmt.Errorf("%w: %v", errMilvus, err)
			}
		}
	}
	return loi, nil
}

// hong counts a document's failure. Before MaxThuLai it is deferred
// (choLoi); at MaxThuLai it leaves rag_dirty for the dead-letter table and
// every live collection (fail closed: the failure may have been the write
// that carried a new allergen). Either only if no change reached it while
// the pass worked: a new change may be the fix.
func (c ChiMuc) hong(ctx context.Context, tx pgx.Tx, song []PhienBan, d dirty, chang string, err error, b *BaoCaoChiMuc) error {
	b.Hong++
	if c.Logger != nil {
		c.Logger.Warn("rag indexer: a document failed", "stage", chang, "code", MaLoi(err), "doc", d.id, "error", rutGon(err))
	}
	if e := GhiDLQ(ctx, tx, CorpusQuan, d.id, chang, err); e != nil {
		return e
	}
	var n int
	if e := tx.QueryRow(ctx, `SELECT so_lan FROM rag_ingest_dlq WHERE corpus='place' AND doc_id=$1 AND chang=$2`, d.id, chang).Scan(&n); e != nil {
		return e
	}
	if n >= MaxThuLai {
		if e := c.xoaMoiNoi(ctx, song, d.id); e != nil {
			return e
		}
		b.VaoDLQ++
		_, e := tx.Exec(ctx, `DELETE FROM rag_dirty WHERE corpus='place' AND doc_id=$1 AND lan=$2`, d.id, d.lan)
		return e
	}
	_, e := tx.Exec(ctx, `UPDATE rag_dirty SET cho_den = clock_timestamp() + $3 * interval '1 second'
		WHERE corpus='place' AND doc_id=$1 AND lan=$2`, d.id, d.lan, choLoi(n).Seconds())
	return e
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
		if err := c.Nap.Kho.XoaID(ctx, p.Collection, MoiIDQuan(docID)); err != nil {
			return fmt.Errorf("%w: %v", errMilvus, err)
		}
		b.XoaKhacCauHinh++
	}
	return nil
}

// xoaMoiNoi deletes a document's row from every live collection.
func (c ChiMuc) xoaMoiNoi(ctx context.Context, song []PhienBan, docID string) error {
	for _, p := range song {
		if err := c.Nap.Kho.XoaID(ctx, p.Collection, MoiIDQuan(docID)); err != nil {
			return fmt.Errorf("%w: %v", errMilvus, err)
		}
	}
	return nil
}

// khoaLo is the batch door's lock: one turn at a time across processes.
const khoaLo = `hashtextextended('rag.nap.nhung_lo',0)`

// LuotLo is the batch door's turn (the rag-indexer runs one every
// ChuKyLo). It polls the open job once; or, when documents wait for the
// door and no job is open, submits one for every place text the cache lacks
// (all of them, ToiDaLo at most: a bulk change is one job, not one per
// pass). When a job ends with its vectors written, or nothing is missing
// any more, the documents that waited are woken. It works on pool, not
// inside a pass's transaction: a job's row must be committed the moment the
// provider accepts it, or a rollback would forget a job already paid for.
func (c ChiMuc) LuotLo(ctx context.Context, pool *pgxpool.Pool) (BaoCaoLo, error) {
	var rep BaoCaoLo
	if c.Lo == nil {
		return rep, nil
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return rep, err
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+khoaLo+`)`).Scan(&got); err != nil {
		return rep, err
	}
	if !got {
		rep.TrangThai = "ban"
		return rep, nil
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(`+khoaLo+`)`); err != nil {
			// A connection that cannot unlock goes back closed, its lock
			// with it.
			_ = conn.Conn().Close(context.WithoutCancel(ctx))
		}
	}()
	job, err := jobDangChay(ctx, pool, c.Lo.Model(), c.Lo.Dims(), TaskTaiLieu(c.Nap.Cfg))
	if err != nil {
		return rep, err
	}
	var cho int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty WHERE corpus='place' AND cho_nhung`).Scan(&cho); err != nil {
		return rep, err
	}
	if job == "" && cho == 0 {
		rep.TrangThai = "khong_can"
		return rep, nil
	}
	var rows []Hang
	if job == "" {
		var dr BaoCaoDung
		docs, _, err := c.Nap.ChuanBiQuan(ctx, pool, &dr)
		if err != nil {
			return rep, err
		}
		for _, d := range docs {
			rows = append(rows, d.Rows...)
		}
	}
	rep, err = c.Nap.NhungQuaLo(ctx, pool, c.Lo, rows, 0, ToiDaLo)
	if err != nil {
		return rep, err
	}
	if rep.TrangThai == "xong" || rep.TrangThai == "khong_can" {
		if _, err := pool.Exec(ctx, `WITH w AS (UPDATE rag_dirty SET cho_den = NULL WHERE corpus='place' AND cho_nhung RETURNING 1)
			SELECT pg_notify('rag_dirty','place') WHERE EXISTS (SELECT 1 FROM w)`); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// ChuKyLo is how often the rag-indexer takes the batch door's turn.
const ChuKyLo = 2 * time.Minute

// Luot runs one pass under the indexer's lock (DinhKy's): ran is false when
// another process holds it.
func (c ChiMuc) Luot(ctx context.Context, pool *pgxpool.Pool) (ran bool, rep BaoCaoChiMuc, err error) {
	d := c.DinhKy()
	d.Chay = func(ctx context.Context, tx pgx.Tx) error {
		var err error
		rep, err = c.MotLuot(ctx, tx)
		return err
	}
	ran, err = jobs.MotLuot(ctx, pool, d)
	return ran, rep, err
}

// Nghe is the rag-indexer's loop: a pass on every notification of
// rag_dirty (rag_danh_dau; a burst gathered for 2 s), at least every 20 s,
// and again at once while a pass takes a full MaxMotLuot. A pass whose lock
// another process holds is asked again after a second.
func (c ChiMuc) Nghe(pool *pgxpool.Pool, logger *slog.Logger) jobs.Nghe {
	return jobs.Nghe{Pool: pool, Kenh: "rag_dirty", ToiDa: 20 * time.Second, Gop: 2 * time.Second, Logger: logger,
		Chay: func(ctx context.Context) (bool, error) {
			c.GiamSat.Nhip(time.Now())
			ran, rep, err := c.Luot(ctx, pool)
			if err != nil {
				return false, err
			}
			if !ran {
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
				}
				return true, nil
			}
			if rep.Lay > 0 && logger != nil {
				logger.Info("rag indexer pass", "report", rep)
			}
			return rep.Lay >= MaxMotLuot, nil
		}}
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
	// DoTuoi is the place index's freshness (the SLO's numbers).
	DoTuoi *DoTuoi `json:"do_tuoi,omitempty"`
}

// DocTrangThai reads the status of every corpus. A Milvus that does not
// answer is reported (loi), not fatal: the status must work when the index
// does not.
func (n Nap) DocTrangThai(ctx context.Context, q Querier) ([]TrangThai, error) {
	var out []TrangThai
	for _, c := range Corpora {
		t := TrangThai{Corpus: c}
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
			// An enrichment counts from either source: RuDi's own
			// (place_enrichments) or vnlocal's (place_lam_giau, when ingest
			// has installed it).
			var coNgoai bool
			if err := q.QueryRow(ctx, `SELECT to_regclass('place_lam_giau') IS NOT NULL`).Scan(&coNgoai); err != nil {
				return nil, err
			}
			ngoai := ""
			if coNgoai {
				ngoai = ` AND NOT EXISTS (SELECT 1 FROM place_lam_giau l WHERE l.place_id=p.id)`
			}
			if err := q.QueryRow(ctx, `SELECT count(*) FROM places p WHERE NOT EXISTS (SELECT 1 FROM place_enrichments e
				WHERE e.place_id=p.id AND e.extractor=$1 AND e.review<>'rejected')`+ngoai, Extractor).Scan(&t.ThieuLamGiau); err != nil {
				return nil, err
			}
			dt, err := DocDoTuoi(ctx, q)
			if err != nil {
				return nil, err
			}
			t.DoTuoi = &dt
		}
		t.DirtyCuNhatS = float64(int64(t.DirtyCuNhatS*10)) / 10
		sort.Strings(t.NhanhSuyGiam)
		out = append(out, t)
	}
	return out, nil
}
