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

// Each BM25 leg alone: the folded field finds marked text from a query
// typed without diacritics; the marked field does not (it keeps marks), and
// finds it from the marked query.
func TestHaiTruongBM25Milvus(t *testing.T) {
	kho := khoTest(t)
	ctx := context.Background()
	n, _ := naptest.Nap(t, kho)
	rows := []nap.Hang{{ChunkID: "a", DocID: "a", Facet: nap.FacetHoSo, Text: "Quán Cà Phê ở Đà Lạt", ContentHash: "h1"},
		{ChunkID: "b", DocID: "b", Facet: nap.FacetHoSo, Text: "Tiệm sửa xe gần chợ", ContentHash: "h2"}}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		t.Fatal(err)
	}
	if err := kho.TaoCollection(ctx, "rd_places__v1", nap.LuocDoTu(n.Cfg, nap.CorpusQuan)); err != nil {
		t.Fatal(err)
	}
	if err := kho.Upsert(ctx, "rd_places__v1", rows); err != nil {
		t.Fatal(err)
	}
	tim := func(q string, w nap.TrongSo) []nap.Trung {
		hits, err := kho.TimLai(ctx, "rd_places__v1", nap.TruyVan{Chu: q, TrongSo: w, K: 5})
		if err != nil {
			t.Fatal(err)
		}
		return hits
	}
	// A fresh upsert becomes searchable a moment later even at Strong
	// consistency (vectordb's live tests wait the same way): wait for the
	// marked query on the marked leg, then ask each question once.
	for i := 0; len(tim("Quán Cà Phê", nap.TrongSo{BM25: 1})) == 0; i++ {
		if i == 100 {
			t.Fatal("the rows never became searchable")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if h := tim("quan ca phe o da lat", nap.TrongSo{BM25KhongDau: 1}); len(h) != 1 || h[0].DocID != "a" {
		t.Fatalf("folded leg, unmarked query: %+v", h)
	}
	if h := tim("quan ca phe o da lat", nap.TrongSo{BM25: 1}); len(h) != 0 {
		t.Fatalf("the marked leg matched an unmarked query: it folds: %+v", h)
	}
	if h := tim("Quán Cà Phê", nap.TrongSo{BM25: 1}); len(h) != 1 || h[0].DocID != "a" {
		t.Fatalf("marked leg, marked query: %+v", h)
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
			for _, r := range nap.DoanQuan(h, q.NhanTay(), n.Cfg.Chunker[nap.CorpusQuan]) {
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
	t.Logf("milvus no-diacritics slice: bo_dau %s; without the folded leg %s", kq.BoDau, kq.BoDauKhongGap)
	for g, s := range kq.Nhom {
		t.Logf("  %-14s %s", g, s)
	}
	if kq.Tong.ViPham != 0 || !k.Dat {
		t.Fatalf("gate on Milvus: %v", k.LyDo)
	}
	if kq.BoDau.Recall <= kq.BoDauKhongGap.Recall {
		t.Fatalf("the folded BM25 field adds nothing on Milvus: %s vs %s", kq.BoDau, kq.BoDauKhongGap)
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
