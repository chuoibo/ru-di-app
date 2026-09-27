package napkho

import (
	"errors"
	"slices"
	"testing"

	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/vectordb"
)

func cfg(t *testing.T) nap.CauHinh {
	t.Helper()
	c, err := nap.MacDinh()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The ingestion's committed configuration and vectordb's schema agree, and
// any drift is refused before a collection is made.
func TestKiemKhop(t *testing.T) {
	c := cfg(t)
	if err := KiemKhop(c); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*nap.CauHinh){
		"schema": func(c *nap.CauHinh) { c.LuocDo = "rd.v1" },
		"dims":   func(c *nap.CauHinh) { c.Dense.Dims = 3072 },
		"rrf":    func(c *nap.CauHinh) { c.Hop.RRFK = 61 },
	} {
		m := c
		mut(&m)
		if err := KiemKhop(m); !errors.Is(err, ErrLuocDo) {
			t.Errorf("%s drift accepted: %v", name, err)
		}
	}
}

func TestTenChiTrongSoDo(t *testing.T) {
	for _, ok := range []string{"rd_places__v3", "rd_manual__v12", "rd_places", "rd_manual", "rd_eval__0123456789ab"} {
		if _, _, err := kho(ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"rd_place__v1", "nep_memories", "nep_memories__v1", "rd_places__v0", "rd_eval__x", "places"} {
		if _, _, err := kho(bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if k, v, _ := kho("rd_manual__v7"); k != vectordb.KhoHuongDan || v != 7 {
		t.Fatalf("%s %d", k, v)
	}
}

// probes are hard-filter sets over the golden destinations: each allergen,
// diet, budget, slot and window, and combinations.
func probes() []nap.Loc {
	var out []nap.Loc
	for _, d := range []string{"", "d-da-lat", "d-tphcm", "d-hoi-an"} {
		for _, a := range tuvung.DiUng.IDs() {
			out = append(out, nap.Loc{DiemDen: d, DiUng: []string{a}})
		}
		for _, k := range tuvung.AnKieng.IDs() {
			out = append(out, nap.Loc{DiemDen: d, AnKieng: []string{k}})
		}
		for _, b := range []int64{0, 30_000, 100_000, 500_000} {
			out = append(out, nap.Loc{DiemDen: d, NganSach: &b})
		}
		for _, s := range []int16{14, 24, 39, 47, 100, 300} {
			out = append(out, nap.Loc{DiemDen: d, O: &s})
		}
		out = append(out, nap.Loc{DiemDen: d, Khung: []int16{36, 37, 38, 39}}, nap.Loc{DiemDen: d, Khung: []int16{335, 0}})
		b := int64(150_000)
		s := int16(38)
		out = append(out, nap.Loc{DiemDen: d, DiUng: []string{"tom"}, AnKieng: []string{"chay"}, NganSach: &b, O: &s})
	}
	return out
}

// One rule, three readings: nap's Go filter (Khop, what KhoNho and the gate
// check), vectordb's Go filter (Dat, what the fake and the Postgres re-check
// use) on the row as this adapter writes it, and -- in the live tier --
// Milvus's expression. The first two must agree on every golden row under
// every probe, unknown allergens, prices and hours included.
func TestKhopVaDatDongY(t *testing.T) {
	v, err := nap.DocVang(rag.VangDiaDiem())
	if err != nil {
		t.Fatal(err)
	}
	c := cfg(t)
	var rows []nap.Hang
	unknown := map[string]int{}
	for i, q := range v.Quan {
		h, bo := nap.DungHoSo(v.Hang(i, q))
		if bo {
			continue
		}
		for _, r := range nap.DoanQuan(h, q.NhanTay(), c.Chunker[nap.CorpusQuan]) {
			rows = append(rows, r)
			if !r.DiUngRo {
				unknown["di_ung"]++
			}
			if !r.GiaRo {
				unknown["gia"]++
			}
			if !r.GioRo {
				unknown["gio"]++
			}
		}
	}
	if unknown["di_ung"] == 0 || unknown["gia"] == 0 || unknown["gio"] == 0 {
		t.Fatalf("the golden rows hold no unknown attribute to check: %v", unknown)
	}
	checked := 0
	for _, l := range probes() {
		lc := LocCung(l)
		for _, r := range rows {
			a := l.Khop(r)
			b, _ := lc.Dat(ThuocTinh(r))
			if a != b {
				t.Fatalf("row %s under %+v: nap %v, vectordb %v", r.ChunkID, l, a, b)
			}
			checked++
		}
	}
	t.Logf("%d row × filter pairs agree; unknown rows %v", checked, unknown)
}

func TestThuocTinhChuaRoLaLoaiTru(t *testing.T) {
	r := nap.Hang{DiemDen: "d", DiUng: []string{"tom"}, DiUngRo: false, GiaMin: 5, GiaRo: false, MoO: []int16{3}, GioRo: false}
	tt := ThuocTinh(r)
	if !slices.Equal(tt.DiUng, []string{vectordb.KhongRo}) || tt.GiaMinVND != vectordb.GiaKhongRo || len(tt.OSlots) != 0 {
		t.Fatalf("unknowns written as knowns: %+v", tt)
	}
	r.DiUngRo, r.GiaRo, r.GioRo = true, true, true
	tt = ThuocTinh(r)
	if !slices.Equal(tt.DiUng, []string{"tom"}) || tt.GiaMinVND != 5 || !slices.Equal(tt.OSlots, []int16{3}) {
		t.Fatalf("knowns lost: %+v", tt)
	}
}
