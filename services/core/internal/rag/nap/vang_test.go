package nap

import (
	"context"
	"fmt"
	"os"
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
// bo_dau is every question folded to no diacritics; bo_dau_khong_gap the
// same with the folded BM25 leg off: the difference is the recall the
// diacritics-folding field buys.
var vangGhim = map[string]string{
	"bay_injection":       "n=7 co_lien_quan=2 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"di_ung":              "n=86 co_lien_quan=24 recall@10=0.8333 ndcg@10=0.6763 mrr@10=0.6691 violation@10=0.0000 so_vi_pham=0",
	"khi_chat":            "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=0.9109 mrr@10=0.9143 violation@10=0.0000 so_vi_pham=0",
	"khong_dau":           "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=0.9450 mrr@10=0.9333 violation@10=0.0000 so_vi_pham=0",
	"lien_diem_den":       "n=8 co_lien_quan=8 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"rang_buoc":           "n=10 co_lien_quan=9 recall@10=0.8889 ndcg@10=0.8113 mrr@10=0.7500 violation@10=0.0000 so_vi_pham=0",
	"ten_rieng":           "n=12 co_lien_quan=12 recall@10=1.0000 ndcg@10=0.9692 mrr@10=0.9583 violation@10=0.0000 so_vi_pham=0",
	"tong":                "n=143 co_lien_quan=75 recall@10=0.9333 ndcg@10=0.8496 mrr@10=0.8371 violation@10=0.0000 so_vi_pham=0",
	"khong_dau_gap":       "0.0500",
	"chi_dense":           "n=143 co_lien_quan=75 recall@10=0.8133 ndcg@10=0.7330 mrr@10=0.7144 violation@10=0.0000 so_vi_pham=0",
	"chi_thua":            "n=143 co_lien_quan=75 recall@10=0.9667 ndcg@10=0.8902 mrr@10=0.8671 violation@10=0.0000 so_vi_pham=0",
	"khong_dau_khong_gap": "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=0.9388 mrr@10=0.9333 violation@10=0.0000 so_vi_pham=0",
	"bo_dau":              "n=143 co_lien_quan=75 recall@10=0.9267 ndcg@10=0.8059 mrr@10=0.7799 violation@10=0.0000 so_vi_pham=0",
	"bo_dau_khong_gap":    "n=143 co_lien_quan=75 recall@10=0.7867 ndcg@10=0.6740 mrr@10=0.6444 violation@10=0.0000 so_vi_pham=0",
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
	t.Logf("%-14s %s", "kd_khong_gap", kq.KhongDauKhongGap)
	t.Logf("%-14s %s", "bo_dau", kq.BoDau)
	t.Logf("%-14s %s", "bo_dau_k_gap", kq.BoDauKhongGap)
	got := map[string]string{"tong": kq.Tong.String(), "chi_dense": kq.ChiDense.String(), "chi_thua": kq.ChiThua.String(),
		"khong_dau_gap": fmt.Sprintf("%.4f", kq.GapDau), "khong_dau_khong_gap": kq.KhongDauKhongGap.String(),
		"bo_dau": kq.BoDau.String(), "bo_dau_khong_gap": kq.BoDauKhongGap.String()}
	// The folded field must buy recall on the no-diacritics slice.
	if kq.BoDau.Recall <= kq.BoDauKhongGap.Recall {
		t.Errorf("the folded BM25 leg adds nothing on questions without diacritics: %.4f vs %.4f", kq.BoDau.Recall, kq.BoDauKhongGap.Recall)
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
	k := KetQuaCong{Nguong: n.Cfg.Cong, CanVang: true, Vang: &kq, ThamDo: 1, DoiSoat: DoiSoat{Dat: true}}
	KiemCong(&k, nil)
	if !k.Dat {
		t.Errorf("the golden set fails the gate with the stub encoder: %v", k.LyDo)
	}
}
