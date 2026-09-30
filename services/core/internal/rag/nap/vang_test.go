package nap

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"testing"

	"mobile/services/core/internal/rag"
)

func docVangTest(t testing.TB) TapVang {
	t.Helper()
	v, err := DocVang(rag.VangDiaDiem())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func napNho(t testing.TB) (Nap, StubDense) {
	t.Helper()
	cfg, err := MacDinh()
	if err != nil {
		t.Fatal(err)
	}
	enc := StubDense{N: cfg.Dense.Dims}
	return Nap{Kho: NewKhoNho(), Dense: enc, Cfg: cfg}, enc
}

// vangGhim are the golden set's numbers with the stub encoder, the
// in-memory store and the committed configuration: deterministic, pinned to
// four decimals. A change to the chunker, the enrichment attributes, the
// filter, the fusion or the golden file moves one of them, and the diff
// says which group moved.
//
// d08 («bánh mì ở Hội An, dị ứng gluten») used to rank ha-cao-lau-gieng-co,
// which the golden lists in phai_loai while its hand label said no gluten:
// the golden disagreed with itself. Cao lầu noodles are wheat, so the hand
// label now says lua_mi (rag/testdata, rag's own pins unchanged).
//
// The fusion weights are cauhinh.json «hop» (dense 1, BM25 with diacritics
// 0.1, BM25 folded 1): with the stub encoder a heavier marked leg lifts the
// marked twin of each khong_dau question and opens the no-diacritics gap
// past its 0.05 threshold (weight 1: gap 0.1319); 0.1 keeps the gain on
// the marked questions and a gap of 0.0369. Tuned on the stub, so the
// numbers move when a real encoder replaces it.
//
// bo_dau is every question folded to no diacritics; bo_dau_chi_dense the
// same with the dense leg alone: the difference is the recall the folded
// BM25 leg buys.
//
// rd.v4 (one row per place, 3072 dims, 2026-09-30) with the stub encoder:
// recall@10 0.9333 -> 0.8800 against rd.v3's three facets. These are pins of the stub,
// not the gate: the same golden set on Milvus with gemini-embedding-2 reads
// recall@10 0.9467, nDCG@10 0.9120, MRR@10 0.9033, violation 0 (measured
// 2026-09-30 at 1536 dims), and v-eval holds the unchanged thresholds on
// whatever a build embeds.
var vangGhim = map[string]string{
	"bay_injection":    "n=7 co_lien_quan=2 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"bo_dau":           "n=143 co_lien_quan=75 recall@10=0.8800 ndcg@10=0.7858 mrr@10=0.7575 violation@10=0.0000 so_vi_pham=0",
	"bo_dau_chi_dense": "n=143 co_lien_quan=75 recall@10=0.8067 ndcg@10=0.7452 mrr@10=0.7249 violation@10=0.0000 so_vi_pham=0",
	"chi_dense":        "n=143 co_lien_quan=75 recall@10=0.8067 ndcg@10=0.7452 mrr@10=0.7249 violation@10=0.0000 so_vi_pham=0",
	"chi_thua":         "n=143 co_lien_quan=75 recall@10=0.8867 ndcg@10=0.8256 mrr@10=0.8049 violation@10=0.0000 so_vi_pham=0",
	"di_ung":           "n=86 co_lien_quan=24 recall@10=0.6667 ndcg@10=0.4654 mrr@10=0.4066 violation@10=0.0000 so_vi_pham=0",
	"khi_chat":         "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=0.8603 mrr@10=0.8350 violation@10=0.0000 so_vi_pham=0",
	"khong_dau":        "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=0.9950 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"khong_dau_gap":    "0.0000",
	"lien_diem_den":    "n=8 co_lien_quan=8 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"rang_buoc":        "n=10 co_lien_quan=9 recall@10=0.8889 ndcg@10=0.8422 mrr@10=0.8000 violation@10=0.0000 so_vi_pham=0",
	"ten_rieng":        "n=12 co_lien_quan=12 recall@10=1.0000 ndcg@10=0.9692 mrr@10=0.9583 violation@10=0.0000 so_vi_pham=0",
	"tong":             "n=143 co_lien_quan=75 recall@10=0.8800 ndcg@10=0.7858 mrr@10=0.7575 violation@10=0.0000 so_vi_pham=0",
}

func TestVangGhimVaCong(t *testing.T) {
	n, enc := napNho(t)
	v := docVangTest(t)
	kq, err := n.DanhGiaVang(context.Background(), enc, v, TenEval(n.Cfg))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range kq.Nhom {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%-14s %s", k, kq.Nhom[k])
	}
	t.Logf("%-14s %s", "tong", kq.Tong)
	t.Logf("khong_dau_gap=%.4f", kq.GapDau)
	t.Logf("%-14s %s", "chi_dense", kq.ChiDense)
	t.Logf("%-14s %s", "chi_thua", kq.ChiThua)
	t.Logf("%-14s %s", "bo_dau", kq.BoDau)
	t.Logf("%-14s %s", "bo_dau_dense", kq.BoDauChiDense)
	got := map[string]string{"tong": kq.Tong.String(), "chi_dense": kq.ChiDense.String(), "chi_thua": kq.ChiThua.String(),
		"khong_dau_gap": fmt.Sprintf("%.4f", kq.GapDau),
		"bo_dau":        kq.BoDau.String(), "bo_dau_chi_dense": kq.BoDauChiDense.String()}
	// The folded BM25 leg must buy recall on the no-diacritics slice.
	if kq.BoDau.Recall <= kq.BoDauChiDense.Recall {
		t.Errorf("the folded BM25 leg adds nothing on questions without diacritics: %.4f vs %.4f", kq.BoDau.Recall, kq.BoDauChiDense.Recall)
	}
	for k, s := range kq.Nhom {
		got[k] = s.String()
	}
	for k, want := range vangGhim {
		if os.Getenv("NAP_VANG_IN") == "1" {
			fmt.Printf("PIN %s=%s\n", k, got[k])
			continue
		}
		if want != "" && got[k] != want {
			t.Errorf("%s: got %q, pinned %q", k, got[k], want)
		}
	}
	// With the stub encoder the gate's relevance thresholds are not the
	// subject (see vangGhim); a violation always is.
	k := KetQuaCong{Nguong: n.Cfg.Cong, CanVang: true, Vang: &kq, ThamDo: 1, DoiSoat: DoiSoat{Dat: true}}
	KiemCong(&k, nil)
	if kq.Tong.ViPham != 0 || slices.Contains(k.LyDo, "violation_10") {
		t.Errorf("the golden set breaks a hard filter with the stub encoder: %v", k.LyDo)
	}
}
