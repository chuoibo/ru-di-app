//go:build milvus

package napkho

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/testmilvus"
	"mobile/services/core/internal/vectordb"
)

// The ingestion half of the Milvus tier (scripts/go_milvus_tier.sh): nap
// over this adapter on a real Milvus. Every test works under a fresh name
// prefix (testmilvus.Ket), dropped at the end, so no two tests share a
// collection or an alias.

func khoTest(t *testing.T) *Kho { t.Helper(); return &Kho{M: testmilvus.Ket(t)} }

func TestUpsertLapLaiMotHangMilvus(t *testing.T) {
	naptest.UpsertLapLai(t, khoTest(t), "rd_places__v1")
}

// The one BM25 field (rd.v4) folds diacritics: marked text is found from a
// query typed without marks and from the marked query alike; unrelated text
// is not.
func TestBM25GapDauMilvus(t *testing.T) {
	kho := khoTest(t)
	ctx := context.Background()
	n, _ := naptest.Nap(t, kho)
	rows := []nap.Hang{{ChunkID: "a", DocID: "a", Text: "Quán Cà Phê ở Đà Lạt", ContentHash: "h1"},
		{ChunkID: "b", DocID: "b", Text: "Tiệm sửa xe gần chợ", ContentHash: "h2"}}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		t.Fatal(err)
	}
	if err := kho.TaoCollection(ctx, "rd_places__v1", nap.LuocDoTu(n.Cfg, nap.CorpusQuan)); err != nil {
		t.Fatal(err)
	}
	if err := kho.Upsert(ctx, "rd_places__v1", rows); err != nil {
		t.Fatal(err)
	}
	tim := func(q string) []nap.Trung {
		hits, err := kho.TimLai(ctx, "rd_places__v1", nap.TruyVan{Chu: q, TrongSo: nap.TrongSo{BM25: 1}, K: 5})
		if err != nil {
			t.Fatal(err)
		}
		return hits
	}
	// A fresh upsert becomes searchable a moment later even at Strong
	// consistency (vectordb's live tests wait the same way).
	for i := 0; len(tim("Quán Cà Phê")) == 0; i++ {
		if i == 100 {
			t.Fatal("the rows never became searchable")
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, q := range []string{"quan ca phe o da lat", "Quán Cà Phê", "QUÁN CÀ PHÊ ĐÀ LẠT"} {
		if h := tim(q); len(h) != 1 || h[0].DocID != "a" {
			t.Fatalf("%q: %+v", q, h)
		}
	}
}

// Filter parity: every hard-filter probe over the golden rows -- unknown
// allergens, prices and hours included -- admits exactly the same chunks in
// Milvus as nap's Go reading of the same filter (and so as KhoNho).
func TestLocMilvusKhopGo(t *testing.T) {
	kho := khoTest(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	n, _ := naptest.Nap(t, kho)
	var rows []nap.Hang
	unknown := 0
	for i, q := range v.Quan {
		h, bo := nap.DungHoSo(v.Hang(i, q))
		if !bo {
			for _, r := range doanThu(t, h, q.NhanTay(), n.Cfg.Chunker[nap.CorpusQuan]) {
				if !r.DiUngRo || !r.GiaRo || !r.GioRo {
					unknown++
				}
				rows = append(rows, r)
			}
		}
	}
	if unknown == 0 {
		t.Fatal("no row with an unknown attribute: a fail-open filter could not show")
	}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		t.Fatal(err)
	}
	const ten = "rd_places__v1"
	if err := kho.TaoCollection(ctx, ten, nap.LuocDoTu(n.Cfg, nap.CorpusQuan)); err != nil {
		t.Fatal(err)
	}
	if err := nap.GhiHang(ctx, kho, ten, rows); err != nil {
		t.Fatal(err)
	}
	mem := nap.NewKhoNho()
	_ = mem.TaoCollection(ctx, ten, nap.LuocDoTu(n.Cfg, nap.CorpusQuan))
	_ = mem.Upsert(ctx, ten, rows)
	truth := map[string]nap.Hang{}
	for _, r := range rows {
		truth[r.ChunkID] = r
	}
	probes, vi, err := nap.ThamDoLoc(ctx, kho, ten, truth, []string{"d-da-lat", "d-tphcm", "d-hoi-an"})
	if err != nil || vi != 0 || probes < 200 {
		t.Fatalf("probes %d, violations %d, %v", probes, vi, err)
	}
	var locs []nap.Loc
	for _, q := range v.TruyVan {
		locs = append(locs, q.RangBuoc.Loc())
	}
	locs = append(locs, probesLoc()...)
	diff := 0
	for _, l := range locs {
		a, err := kho.LocHang(ctx, ten, l)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := mem.LocHang(ctx, ten, l)
		if ids(a) != ids(b) {
			diff++
		}
	}
	if diff != 0 {
		t.Fatalf("%d of %d filters admit different rows in Milvus and in Go", diff, len(locs))
	}
	t.Logf("%d probes, 0 violations; %d filters admit identical rows; %d rows with an unknown attribute", probes, len(locs), unknown)
}

func probesLoc() []nap.Loc { return probes() }

func ids(ks []nap.KhoaHang) string {
	var out []string
	for _, k := range ks {
		out = append(out, k.ChunkID)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// The golden set on Milvus itself: dense HNSW + both BM25 functions,
// weighted RRF in Go, template-parameter filters. violation@10 must be 0 and
// the gate's thresholds must hold; the numbers are logged, not pinned (HNSW
// is approximate; the unit tier pins the exact in-memory numbers).
func TestHybridLocCungKhongViPham(t *testing.T) {
	kho := khoTest(t)
	n, enc := naptest.Nap(t, kho)
	v := naptest.Vang(t)
	start := time.Now()
	kq, err := n.DanhGiaVang(context.Background(), enc, v, nap.TenEval(n.Cfg))
	if err != nil {
		t.Fatal(err)
	}
	k := nap.KetQuaCong{Nguong: n.Cfg.Cong, CanVang: true, Vang: &kq, ThamDo: 1, DoiSoat: nap.DoiSoat{Dat: true}}
	nap.KiemCong(&k, nil)
	t.Logf("milvus golden: tong %s; gap %.4f; chi_dense %s; chi_thua %s; %s", kq.Tong, kq.GapDau, kq.ChiDense, kq.ChiThua, time.Since(start).Round(time.Millisecond))
	t.Logf("milvus no-diacritics slice: bo_dau %s; dense alone %s", kq.BoDau, kq.BoDauChiDense)
	for g, s := range kq.Nhom {
		t.Logf("  %-14s %s", g, s)
	}
	if kq.Tong.ViPham != 0 || !k.Dat {
		t.Fatalf("gate on Milvus: %v", k.LyDo)
	}
	if kq.BoDau.Recall <= kq.BoDauChiDense.Recall {
		t.Fatalf("the folded BM25 field adds nothing on Milvus: %s vs %s", kq.BoDau, kq.BoDauChiDense)
	}
}

// The lifecycle on Milvus: build → reconcile → gate → promote by alias →
// change capture → indexer → second version → rollback by alias → removed
// places stay removed → the reconciler repairs a moved alias.
func TestBuildDoiSoatPromoteRollbackQuaAlias(t *testing.T) {
	naptest.VongDoi(t, naptest.Pool(t), khoTest(t))
}

func TestTombstoneKhongSongLaiSauRollback(t *testing.T) {
	naptest.TombstoneQuaRollback(t, naptest.Pool(t), khoTest(t))
}

func TestDoiSoatChanPromoteMilvus(t *testing.T) {
	naptest.DoiSoatChanPromote(t, naptest.Pool(t), khoTest(t))
}

// An attribute change reaches a collection of another configuration: the
// indexer rewrites the place's rows there in place (F7).
func TestThuocTinhDenBanKhacCauHinhMilvus(t *testing.T) {
	naptest.ThuocTinhKhacCauHinh(t, naptest.Pool(t), khoTest(t))
}

// A physical collection may never carry an alias's name: vectordb refuses
// to create one (Milvus resolves a name as a collection before it tries it
// as an alias, so the alias would silently stop moving).
func TestTenTrungAliasBiTuChoi(t *testing.T) {
	kho := khoTest(t)
	ctx := context.Background()
	n, _ := naptest.Nap(t, kho)
	err := kho.TaoCollection(ctx, nap.Alias(nap.CorpusQuan), nap.LuocDoTu(n.Cfg, nap.CorpusQuan))
	if !errors.Is(err, vectordb.ErrBongAlias) {
		t.Fatalf("an alias-named collection was created: %v", err)
	}
}

// The indexer's two batched calls on a real Milvus: KhoaTheoDoc reads the
// keys and the stored fingerprint of exactly the documents asked; a
// partial update rewrites attributes, categories and fingerprint and leaves
// text, hash and vector where they were (the row is still found by its
// text); a batch naming a key Milvus does not hold is refused whole or
// writes nothing for it -- never a row without a vector.
func TestKhoaVaCapNhatMotPhanMilvus(t *testing.T) {
	kho := khoTest(t)
	ctx := context.Background()
	n, _ := naptest.Nap(t, kho)
	const ten = "rd_places__v1"
	gia := func(lo int64) (int64, int64, bool) { return lo, lo + 20000, true }
	rows := []nap.Hang{
		{ChunkID: "a", DocID: "a", Text: "Quán bún bò bên hồ", ContentHash: "ha", DiemDen: "d-da-lat", DiUngRo: true, DiUng: []string{}},
		{ChunkID: "b", DocID: "b", Text: "Tiệm bánh căn sáng sớm", ContentHash: "hb", DiemDen: "d-da-lat", DiUngRo: true, DiUng: []string{"tom"}},
		{ChunkID: "c", DocID: "c", Text: "Cà phê view đồi thông", ContentHash: "hc", DiemDen: "d-da-lat"},
	}
	rows[0].GiaMin, rows[0].GiaMax, rows[0].GiaRo = gia(40000)
	rows[0].HienThi = map[string]string{"ten": "Bún Bò Hồ", "gia": "40.000–60.000 đ/người"}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		t.Fatal(err)
	}
	if err := kho.TaoCollection(ctx, ten, nap.LuocDoTu(n.Cfg, nap.CorpusQuan)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kho.XoaCollection(context.Background(), ten) })
	if err := kho.Upsert(ctx, ten, rows); err != nil {
		t.Fatal(err)
	}
	ks, err := kho.KhoaTheoDoc(ctx, ten, []string{"a", "b", "khong-co"})
	if err != nil || len(ks) != 2 || ks[0].DocID != "a" || ks[1].DocID != "b" {
		t.Fatalf("keys of a, b and a missing id: %+v %v", ks, err)
	}
	if ks[0].ContentHash != "ha" || ks[0].Dau != nap.DauThuocTinh(rows[0]) || ks[1].Dau != nap.DauThuocTinh(rows[1]) {
		t.Fatalf("stored keys: %+v", ks)
	}

	moi := rows[0]
	moi.GiaMin, moi.GiaMax, moi.GiaRo = gia(90000)
	moi.DiUng, moi.DanhMuc = []string{"dau_phong"}, []string{"an_vat"}
	moi.HienThi = map[string]string{"ten": "Bún Bò Hồ", "gia": "90.000–110.000 đ/người"}
	moi.Text, moi.Dense = "", nil // a partial update must not need them
	if got, err := kho.CapNhatThuocTinhLo(ctx, ten, []nap.Hang{moi}); err != nil || got != 1 {
		t.Fatalf("partial update: %d %v", got, err)
	}
	ks, _ = kho.KhoaTheoDoc(ctx, ten, []string{"a"})
	if len(ks) != 1 || ks[0].ContentHash != "ha" || ks[0].Dau != nap.DauThuocTinh(moi) {
		t.Fatalf("after the partial update: %+v", ks)
	}
	tt, err := kho.M.DocThuocTinh(ctx, kho.ten(ten), "a")
	if err != nil || tt["a"].GiaMinVND != 90000 || len(tt["a"].DanhMuc) != 1 || tt["a"].DanhMuc[0] != "an_vat" ||
		len(tt["a"].DiUng) != 1 || tt["a"].DiUng[0] != "dau_phong" {
		t.Fatalf("stored attributes: %+v %v", tt["a"], err)
	}
	for i := 0; ; i++ {
		hits, err := kho.TimLai(ctx, ten, nap.TruyVan{Chu: "bun bo ben ho", Dense: rows[0].Dense, TrongSo: nap.TrongSo{Dense: 1, BM25: 1}, K: 3})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) > 0 && hits[0].DocID == "a" {
			break
		}
		if i == 100 {
			t.Fatalf("the partially updated row lost its text or vector: %+v", hits)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// A search hands back the evidence fields, as the partial update left
	// them (ADR-0051: the answer is built from these).
	hits, err := kho.M.Tim(ctx, vectordb.YeuCauTim{Ten: kho.ten(ten), Kho: vectordb.KhoDiaDiem, K: 3,
		Thua: &vectordb.ThuaTruyVan{Text: "bún bò bên hồ"}})
	if err != nil || len(hits) == 0 || hits[0].DocID != "a" || hits[0].HienThi["gia"] != "90.000–110.000 đ/người" {
		t.Fatalf("evidence from the index: %+v %v", hits, err)
	}
	for _, h := range hits {
		if h.DocID == "b" && h.HienThi != nil {
			t.Fatalf("a row written without fields came back with some: %+v", h)
		}
	}

	ma := rows[1]
	ma.DiUng = []string{"tom", "cua"}
	ghost := nap.Hang{ChunkID: "khong-co", DocID: "khong-co", DiemDen: "d-da-lat"}
	// Milvus 3.0.2 refuses the whole batch ("cannot insert a new entity:
	// missing required field dense"): the indexer then writes row by row.
	if _, err = kho.CapNhatThuocTinhLo(ctx, ten, []nap.Hang{ma, ghost}); err == nil {
		t.Fatal("a partial update naming a missing key was accepted")
	}
	if c, e := kho.Dem(ctx, ten); e != nil || c != 3 {
		t.Fatalf("a partial update of a missing key left %d rows (%v)", c, e)
	}
	if ks, _ = kho.KhoaTheoDoc(ctx, ten, []string{"b"}); len(ks) != 1 || ks[0].Dau != nap.DauThuocTinh(rows[1]) {
		t.Fatalf("the refused batch wrote part of itself: %+v", ks)
	}
}
