package nap

import (
	"context"
	"slices"
	"testing"
)

// boLoc wraps a store and drops one hard filter from every search: the
// canary a gate that reads violation must catch.
type boLoc struct {
	KhoVector
	bo func(*Loc)
}

func (b boLoc) TimLai(ctx context.Context, ten string, tv TruyVan) ([]Trung, error) {
	b.bo(&tv.Loc)
	return b.KhoVector.TimLai(ctx, ten, tv)
}

func (b boLoc) LocHang(ctx context.Context, ten string, l Loc) ([]KhoaHang, error) {
	b.bo(&l)
	return b.KhoVector.LocHang(ctx, ten, l)
}

// Canary: the same golden run with the allergen filter (or the destination)
// dropped goes red at violation@10, in the di_ung group for the allergen
// filter, and the gate refuses it for that violation; with nothing dropped
// (identity) it stays at zero and no violation is cited. The stub encoder's
// relevance is not the gate's subject here (rd.v4 with the stub reads
// recall@10 0.88; the real encoder on Milvus, 0.9467 -- v-eval measures that).
func TestVangCanaryBoLoc(t *testing.T) {
	for _, c := range []struct {
		name string
		bo   func(*Loc)
		red  bool
	}{
		{"identity", func(*Loc) {}, false},
		{"bo_di_ung", func(l *Loc) { l.DiUng = nil }, true},
		{"bo_diem_den", func(l *Loc) { l.DiemDen = "" }, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			n, enc := napNho(t)
			n.Kho = boLoc{KhoVector: n.Kho, bo: c.bo}
			kq, err := n.DanhGiaVang(context.Background(), enc, docVangTest(t), TenEval(n.Cfg))
			if err != nil {
				t.Fatal(err)
			}
			k := KetQuaCong{Nguong: n.Cfg.Cong, CanVang: true, Vang: &kq, ThamDo: 1, DoiSoat: DoiSoat{Dat: true}}
			KiemCong(&k, nil)
			t.Logf("%s: tong %s; gate %v %v", c.name, kq.Tong, k.Dat, k.LyDo)
			citesViolation := slices.Contains(k.LyDo, "violation_10")
			if red := kq.Tong.ViPham > 0; red != c.red || citesViolation != c.red || (c.red && k.Dat) {
				t.Fatalf("violations %d, gate %v %v; want red=%v", kq.Tong.ViPham, k.Dat, k.LyDo, c.red)
			}
			if c.name == "bo_di_ung" && kq.Nhom["di_ung"].ViPham == 0 {
				t.Fatal("dropping the allergen filter left the di_ung group clean")
			}
		})
	}
}

// Canary on the probes: a store that ignores the allergen filter admits
// rows the truth refuses, and ThamDoLoc counts them.
func TestThamDoCanary(t *testing.T) {
	n, _ := napNho(t)
	ctx := context.Background()
	v := docVangTest(t)
	var rows []Hang
	for i, qv := range v.Quan[:60] {
		h, _ := DungHoSo(v.Hang(i, qv))
		rows = append(rows, doanThu(t, h, qv.NhanTay(), n.Cfg.Chunker[CorpusQuan])...)
	}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		t.Fatal(err)
	}
	_ = n.Kho.TaoCollection(ctx, "rd_place__v1", LuocDoTu(n.Cfg, CorpusQuan))
	if err := GhiHang(ctx, n.Kho, "rd_place__v1", rows); err != nil {
		t.Fatal(err)
	}
	truth := map[string]Hang{}
	for _, r := range rows {
		truth[r.ChunkID] = r
	}
	dests := []string{"d-da-lat", "d-tphcm", "d-hoi-an"}
	p, vi, err := ThamDoLoc(ctx, n.Kho, "rd_place__v1", truth, dests)
	if err != nil || vi != 0 || p < 3*70 {
		t.Fatalf("identity: probes %d violations %d err %v", p, vi, err)
	}
	bad := boLoc{KhoVector: n.Kho, bo: func(l *Loc) { l.DiUng = nil }}
	if _, vi, _ = ThamDoLoc(ctx, bad, "rd_place__v1", truth, dests); vi == 0 {
		t.Fatal("a store ignoring the allergen filter passed the probes")
	}
	t.Logf("probes %d; allergen filter dropped: %d violations", p, vi)
}
