package nap

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"mobile/services/core/internal/huongdan"
	"mobile/services/core/internal/repo"
)

// Nap is the pipeline's dependencies. Kho is the vector store, Dense the
// document encoder.
type Nap struct {
	Kho   KhoVector
	Dense NhungTaiLieu
	Cfg   CauHinh
}

// BaoCaoDung is what one build did, in ids, states and counts only.
type BaoCaoDung struct {
	PhienBan   int64  `json:"phien_ban"`
	Corpus     Corpus `json:"corpus"`
	Collection string `json:"collection"`
	Docs       int    `json:"docs"`
	Chunks     int    `json:"chunks"`
	// BoKhongAnToan: rows SafeDeep dropped (tombstoned unsafe). CachLy:
	// fields it quarantined. Bia: tombstoned places left out.
	BoKhongAnToan int `json:"bo_khong_an_toan"`
	// QuaDai: places left out because their text is longer than a row
	// holds (MaxByteVanBan; nothing is ever cut).
	QuaDai int `json:"qua_dai,omitempty"`
	CachLy int `json:"cach_ly"`
	Bia    int `json:"bia"`
	// Trung: duplicates dedupe left out.
	Trung int `json:"trung"`
	// ThieuLamGiau: places with no current usable enrichment, LEFT OUT of
	// the version (owner 2026-09-30: build without waiting; vnlocal writes
	// the enrichment within minutes and the indexer adds the place then).
	// LamGiauCu: of them, those with only a stale one; ChenLenh: held back
	// by the injection label. No place without a current enrichment is ever
	// in an index.
	ThieuLamGiau int `json:"thieu_lam_giau"`
	LamGiauCu    int `json:"lam_giau_cu"`
	ChenLenh     int `json:"chen_lenh"`
	// NhungMoi: chunks the encoder was asked for (the rest came from the
	// cache); SoGoiNhung: provider requests.
	NhungMoi   int   `json:"nhung_moi"`
	SoGoiNhung int64 `json:"so_goi_nhung"`
	// DocDoi: documents whose chunks differ from the parent version's.
	DocDoi int `json:"doc_doi"`
}

// TaiLieuQuan is one place ready to index.
type TaiLieuQuan struct {
	HoSo HoSoQuan
	TT   ThuocTinh
	Rows []Hang
}

// ChuanBiQuan runs S1-S5 and S7 over the live catalogue: every place through
// SafeDeep, tombstoned ones left out, its stored enrichment applied, its
// chunks built. No model and no vector yet, except the sentence vectors
// semantic chunking asks for on a facet longer than NguongDoan.
// CanLamGiau lists the safe, untombstoned places without a current usable
// enrichment: exactly the ones ChuanBiQuan counts as ThieuLamGiau, which the
// build gate refuses. It builds no chunk, so no chunking outcome can hide a
// place from the enrichment (a long facet split under a stub encoder once
// did, and those places were never enriched while the gate kept counting them).
func CanLamGiau(ctx context.Context, q Querier) ([]HoSoQuan, error) {
	places, err := repo.Repository{Q: q}.ListPlaces(ctx, repo.PlaceFilter{})
	if err != nil {
		return nil, err
	}
	bia, err := biaTheoLyDo(ctx, q)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(places))
	for i, p := range places {
		ids[i] = p.ID
	}
	enr, err := DocLamGiau(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	var out []HoSoQuan
	for _, p := range places {
		if r, ok := bia[p.ID]; ok && r != "unsafe" && r != "source_deleted" {
			continue
		}
		h, bo := DungHoSo(p)
		if bo {
			continue
		}
		if !ApDung(enr[p.ID], h.NguonHash).Co {
			out = append(out, h)
		}
	}
	return out, nil
}

func (n Nap) ChuanBiQuan(ctx context.Context, q Querier, rep *BaoCaoDung) (docs []TaiLieuQuan, unsafe []string, err error) {
	places, err := repo.Repository{Q: q}.ListPlaces(ctx, repo.PlaceFilter{})
	if err != nil {
		return nil, nil, err
	}
	bia, err := biaTheoLyDo(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, len(places))
	for i, p := range places {
		ids[i] = p.ID
	}
	enr, err := DocLamGiau(ctx, q, ids)
	if err != nil {
		return nil, nil, err
	}
	dm, err := DocDanhMuc(ctx, q, ids)
	if err != nil {
		return nil, nil, err
	}
	chunker := n.Cfg.Chunker[CorpusQuan]
	for _, p := range places {
		if r, ok := bia[p.ID]; ok && r != "unsafe" && r != "source_deleted" {
			rep.Bia++
			continue
		}
		h, bo := DungHoSo(p)
		if bo {
			unsafe = append(unsafe, p.ID)
			continue
		}
		rep.CachLy += h.CachLy
		t := ApDung(enr[p.ID], h.NguonHash)
		t.DanhMuc = dm[p.ID]
		if t.CachLy {
			rep.ChenLenh++
		}
		if !t.Co {
			rep.ThieuLamGiau++
			if t.Cu {
				rep.LamGiauCu++
			}
			continue
		}
		rows, err := DoanQuan(h, t, chunker)
		if errors.Is(err, ErrQuaDai) {
			// Never cut to fit: the place stays out of this version, counted.
			rep.QuaDai++
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		docs = append(docs, TaiLieuQuan{HoSo: h, TT: t, Rows: rows})
	}
	rep.BoKhongAnToan = len(unsafe)
	return docs, unsafe, nil
}

// biaTheoLyDo reads the place tombstones, id → reason.
func biaTheoLyDo(ctx context.Context, q Querier) (map[string]string, error) {
	rows, err := q.Query(ctx, `SELECT doc_id, reason FROM rag_tombstones WHERE corpus='place'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, r string
		if err := rows.Scan(&id, &r); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

// Vector runs S8 on rows: dense from the cache or the encoder. It returns
// how many rows needed the encoder. The sparse leg is Milvus's BM25 over
// the text: nothing to compute here.
func (n Nap) Vector(ctx context.Context, cache CSDL, rows []Hang) (int, error) {
	var bn BoNhoNhung
	if cache != nil {
		bn = BoNhoPG{Q: cache}
	}
	moi, err := NhungHang(ctx, n.Dense, bn, n.Cfg, rows)
	if err != nil {
		return 0, err
	}
	rev := n.Cfg.SparseRev()
	for i := range rows {
		rows[i].SparseRev = rev
	}
	return moi, nil
}

// LocTrung runs S6 over prepared places whose rows carry vectors, and
// returns the places to index and how many duplicates left.
func (n Nap) LocTrung(docs []TaiLieuQuan) ([]TaiLieuQuan, map[string]string) {
	cands := make([]UngVienTrung, 0, len(docs))
	for _, d := range docs {
		var dense []float32
		if len(d.Rows) > 0 {
			dense = d.Rows[0].Dense
		}
		cands = append(cands, UngVienTrung{ID: d.HoSo.ID, DiemDen: d.HoSo.DiemDen, Lat: d.HoSo.Lat, Lng: d.HoSo.Lng, CoToaDo: d.HoSo.CoToaDo,
			Nguon: d.HoSo.Nguon, Giau: giau(d.HoSo), Dense: dense})
	}
	dup := TimTrung(cands, n.Cfg.Trung.CosineToiThieu, n.Cfg.Trung.KhoangCachM)
	var out []TaiLieuQuan
	for _, d := range docs {
		if _, gone := dup[d.HoSo.ID]; !gone {
			out = append(out, d)
		}
	}
	return out, dup
}

// KyVongQuan is what a place version must hold: S1-S7 over the live
// catalogue, vectors from the cache or the encoder, and dedupe. The build
// writes it; the gate recomputes it (from the cache: no encoder call for an
// unchanged chunk) and compares, so the truth is never read back from the
// collection under test.
func (n Nap) KyVongQuan(ctx context.Context, db CSDL, rep *BaoCaoDung) (kept []TaiLieuQuan, unsafe []string, dup map[string]string, moi int, err error) {
	docs, unsafe, err := n.ChuanBiQuan(ctx, db, rep)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	var all []Hang
	for _, d := range docs {
		all = append(all, d.Rows...)
	}
	if moi, err = n.Vector(ctx, db, all); err != nil {
		return nil, nil, nil, 0, err
	}
	// Vector filled the flat copy; hand the vectors back to each document
	// before dedupe reads them.
	i := 0
	for di := range docs {
		for ri := range docs[di].Rows {
			docs[di].Rows[ri] = all[i]
			i++
		}
	}
	kept, dup = n.LocTrung(docs)
	return kept, unsafe, dup, moi, nil
}

// ghiLo is how many rows one upsert carries.
const ghiLo = 256

// GhiHang upserts rows into a collection in batches.
func GhiHang(ctx context.Context, kho KhoVector, ten string, rows []Hang) error {
	for i := 0; i < len(rows); i += ghiLo {
		if err := kho.Upsert(ctx, ten, rows[i:min(i+ghiLo, len(rows))]); err != nil {
			return fmt.Errorf("%w: %v", errMilvus, err)
		}
	}
	return nil
}

// DoiSoat is S10: the collection holds exactly the expected chunks with the
// expected content, no more, no fewer, counted at strong consistency.
type DoiSoat struct {
	KyVong int  `json:"ky_vong"`
	Dem    int  `json:"dem"`
	Thieu  int  `json:"thieu"`
	Thua   int  `json:"thua"`
	Lech   int  `json:"lech"`
	Dat    bool `json:"dat"`
}

// DoiSoatHang compares a collection with the rows it should hold.
func DoiSoatHang(ctx context.Context, kho KhoVector, ten string, rows []Hang) (DoiSoat, error) {
	want := map[string]string{}
	for _, r := range rows {
		want[r.ChunkID] = r.ContentHash
	}
	got, err := kho.LietKe(ctx, ten)
	if err != nil {
		return DoiSoat{}, err
	}
	n, err := kho.Dem(ctx, ten)
	if err != nil {
		return DoiSoat{}, err
	}
	d := DoiSoat{KyVong: len(want), Dem: int(n)}
	seen := map[string]bool{}
	for _, k := range got {
		seen[k.ChunkID] = true
		h, ok := want[k.ChunkID]
		switch {
		case !ok:
			d.Thua++
		case h != k.ContentHash:
			d.Lech++
		}
	}
	for id := range want {
		if !seen[id] {
			d.Thieu++
		}
	}
	d.Dat = d.Thieu == 0 && d.Thua == 0 && d.Lech == 0 && d.Dem == d.KyVong
	return d, nil
}

// digest is sha256 over the sorted (chunk id, content hash) pairs.
func digest(rows []Hang) []byte {
	pairs := make([]string, len(rows))
	for i, r := range rows {
		pairs[i] = r.ChunkID + ":" + r.ContentHash
	}
	sort.Strings(pairs)
	h := sha256.New()
	for _, p := range pairs {
		h.Write([]byte(p + "\n"))
	}
	return h.Sum(nil)
}

// docDoi counts documents whose chunk set differs between rows and a
// collection's listing (added, removed or changed).
func docDoi(rows []Hang, old []KhoaHang) int {
	a, b := map[string][]string{}, map[string][]string{}
	for _, r := range rows {
		a[r.DocID] = append(a[r.DocID], r.ChunkID+":"+r.ContentHash)
	}
	for _, k := range old {
		b[k.DocID] = append(b[k.DocID], k.ChunkID+":"+k.ContentHash)
	}
	n := 0
	for id, xs := range a {
		ys := b[id]
		sort.Strings(xs)
		sort.Strings(ys)
		if fmt.Sprint(xs) != fmt.Sprint(ys) {
			n++
		}
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			n++
		}
	}
	return n
}

// Dung builds a new version of corpus c: a new physical collection, filled
// from Postgres, reconciled, and left `built` (never served until promoted).
// A failure leaves the version `failed` and its collection dropped.
func (n Nap) Dung(ctx context.Context, db CSDL, c Corpus) (BaoCaoDung, error) {
	rep := BaoCaoDung{Corpus: c}
	if err := n.Cfg.Kiem(); err != nil {
		return rep, err
	}
	var parent *int64
	if p, err := PhienBanActive(ctx, db, c); err == nil {
		parent = &p.ID
	} else if !errors.Is(err, ErrKhongActive) {
		return rep, err
	}
	var prompt any
	if c == CorpusQuan {
		prompt = PromptVersion()
	}
	if err := db.QueryRow(ctx, `INSERT INTO rag_vector_versions(corpus, state, dense_model, dense_dims, sparse_mode, sparse_rev, chunker,
		config_fingerprint, prompt_version, parent_id) VALUES($1,'building',$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, milvus_collection`,
		string(c), n.Dense.Model(), n.Cfg.Dense.Dims, string(ThuaBM25), n.Cfg.SparseRev(), n.Cfg.Chunker[c],
		n.Cfg.VanTay(), prompt, parent).Scan(&rep.PhienBan, &rep.Collection); err != nil {
		return rep, err
	}
	fail := func(err error) (BaoCaoDung, error) {
		_ = n.Kho.XoaCollection(context.WithoutCancel(ctx), rep.Collection)
		_, _ = db.Exec(context.WithoutCancel(ctx), `UPDATE rag_vector_versions SET state='failed', dropped_at=clock_timestamp() WHERE id=$1`, rep.PhienBan)
		return rep, err
	}
	if err := KiemTenCollection(rep.Collection); err != nil {
		return fail(err)
	}
	var rows []Hang
	var unsafe, safe, trung, giu []string
	switch c {
	case CorpusQuan:
		kept, bad, dup, moi, err := n.KyVongQuan(ctx, db, &rep)
		if err != nil {
			return fail(err)
		}
		unsafe, rep.NhungMoi, rep.Trung = bad, moi, len(dup)
		for d, g := range dup {
			trung, giu = append(trung, d), append(giu, g)
		}
		for _, d := range kept {
			rows = append(rows, d.Rows...)
			safe = append(safe, d.HoSo.ID)
		}
		rep.Docs = len(kept)
	case CorpusSoTay:
		rows = DoanSoTay(huongdan.TatCa(), n.Cfg.Chunker[CorpusSoTay])
		var err error
		if rep.NhungMoi, err = n.Vector(ctx, db, rows); err != nil {
			return fail(err)
		}
		docs := map[string]bool{}
		for _, r := range rows {
			docs[r.DocID] = true
		}
		rep.Docs = len(docs)
	}
	rep.Chunks = len(rows)
	rep.SoGoiNhung = n.Dense.SoGoi()
	if err := n.Kho.TaoCollection(ctx, rep.Collection, LuocDoTu(n.Cfg, c)); err != nil {
		return fail(fmt.Errorf("%w: %v", errMilvus, err))
	}
	if err := GhiHang(ctx, n.Kho, rep.Collection, rows); err != nil {
		return fail(err)
	}
	ds, err := DoiSoatHang(ctx, n.Kho, rep.Collection, rows)
	if err != nil {
		return fail(err)
	}
	if !ds.Dat {
		return fail(fmt.Errorf("%w: %+v", ErrDoiSoat, ds))
	}
	if parent != nil {
		if p, err := DocPhienBan(ctx, db, *parent); err == nil {
			if old, err := n.Kho.LietKe(ctx, p.Collection); err == nil {
				rep.DocDoi = docDoi(rows, old)
			} else {
				rep.DocDoi = rep.Docs
			}
		}
	} else {
		rep.DocDoi = rep.Docs
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback(ctx)
	if len(unsafe) > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO rag_tombstones(corpus,doc_id,reason) SELECT 'place', unnest($1::text[]), 'unsafe' ON CONFLICT (corpus,doc_id) DO NOTHING`, unsafe); err != nil {
			return fail(err)
		}
	}
	if len(safe) > 0 {
		if _, err = tx.Exec(ctx, `DELETE FROM rag_tombstones WHERE corpus='place' AND reason='unsafe' AND doc_id = ANY($1::text[])`, safe); err != nil {
			return fail(err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE rag_vector_versions SET state='built', docs=$2, chunks=$3, changed_docs=$4, source_digest=$5
		WHERE id=$1 AND state='building'`, rep.PhienBan, rep.Docs, rep.Chunks, rep.DocDoi, digest(rows)); err != nil {
		return fail(err)
	}
	if len(trung) > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO rag_trung(phien_ban, doc_id, giu) SELECT $1, unnest($2::text[]), unnest($3::text[])`,
			rep.PhienBan, trung, giu); err != nil {
			return fail(err)
		}
	}
	if c == CorpusQuan {
		// The collection holds the snapshot read when the build began, and
		// the indexer skipped it while it was building: every place, and
		// every document the snapshot held, is checked again in the
		// background now that the indexer writes this version too. A row
		// that still matches costs a read, not a write.
		if _, err = tx.Exec(ctx, `SELECT rag_danh_dau('place',
			ARRAY(SELECT id FROM places UNION SELECT unnest($1::text[])), NULL, (-1)::smallint)`, safe); err != nil {
			return fail(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fail(err)
	}
	return rep, nil
}
