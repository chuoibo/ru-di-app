package aieval

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/adk/model"

	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
)

var capNhatHieu = flag.Bool("cap-nhat-hieu", false, "rewrite the converted router sets")

const thuMucHieu = "testdata/hieu"

// The converted sets are exactly the conversion of the pinned corpora: a
// hand edit of either side, or a corpus that changed, is red here.
func TestBoHieuChuyenKhopNguon(t *testing.T) {
	for _, c := range []struct {
		nguon, ra string
		chuyen    func([]byte, string, string, int) (BoHieu, error)
		ten       string
		so        int
	}{
		{"tien_niem_phong_v2.json", "tien_v2.json", ChuyenTien, "tien-v2", 286},
		{"tien_niem_phong_v3.json", "tien_v3.json", ChuyenTien, "tien-v3", 505},
		{"di_ung_niem_phong_v2.json", "di_ung_v2.json", ChuyenDiUng, "di-ung-v2", 205},
		{"di_ung_niem_phong_v3.json", "di_ung_v3.json", ChuyenDiUng, "di-ung-v3", 339},
	} {
		raw, err := os.ReadFile(filepath.Join(thuMucHieu, "nguon", c.nguon))
		if err != nil {
			t.Fatal(err)
		}
		b, err := c.chuyen(raw, c.ten, "nguon/"+c.nguon, 1)
		if err != nil {
			t.Fatalf("%s: %v", c.nguon, err)
		}
		if len(b.Ca) != c.so {
			t.Fatalf("%s: %d cases, want %d", c.nguon, len(b.Ca), c.so)
		}
		got, err := MaHoaBoHieu(b)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(thuMucHieu, c.ra)
		if *capNhatHieu {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run with -cap-nhat-hieu once)", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s is not the conversion of %s", c.ra, c.nguon)
		}
		if _, err := DocBoHieu(want); err != nil {
			t.Fatalf("%s: %v", c.ra, err)
		}
	}
}

func stubTu(c CaHieu) model.LLM { return StubHieu(c) }

// T1 of the router: every intent of both bots, money, injection, the
// question back, one repair and the fixed fallback, each through the real
// router over its scripted output, every request held to the invariants.
func TestBoHieuT1(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(thuMucHieu, "t1-hieu.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := DocBoHieu(raw)
	if err != nil {
		t.Fatal(err)
	}
	stubs := map[string]*llm.Stub{}
	moHinh := func(c CaHieu) model.LLM {
		s := stubTu(c).(*llm.Stub)
		stubs[c.ID] = s
		return s
	}
	var ra []KetQuaHieu
	tk := ChayBoHieu(context.Background(), hieu.Moi(), b, moHinh, func(r KetQuaHieu) { ra = append(ra, r) })
	for _, r := range ra {
		if !r.Dat {
			t.Errorf("%s: %+v", r.ID, r.Truot)
		}
	}
	if tk.Dat != tk.SoCa || tk.CaVuotTran != 0 || tk.MaxGoi != 2 {
		t.Fatalf("%+v", tk)
	}
	// Coverage: every intent of each bot, and each policy branch.
	phu := map[string]bool{}
	for _, c := range b.Ca {
		for _, y := range c.KyVong.YDinh {
			phu[string(c.Bot)+"/"+y] = true
		}
		if q := c.KyVong.QuyetDinh; q != nil {
			phu[string(c.Bot)+"/tu_choi_tien"] = phu[string(c.Bot)+"/tu_choi_tien"] || q.TuChoiTien
			phu["han_che"] = phu["han_che"] || q.HanChe
			phu["hoi_lai"] = phu["hoi_lai"] || q.HoiLai
			phu["nhap_tien"] = phu["nhap_tien"] || q.NhapTien
		}
		phu["loi_hieu"] = phu["loi_hieu"] || c.KyVong.LoiHieu
		phu["sua"] = phu["sua"] || (c.KyVong.SoGoi == 2 && !c.KyVong.LoiHieu)
		for _, req := range stubs[c.ID].YeuCau() {
			for _, tr := range KiemYeuCauHieu(req, c) {
				t.Errorf("%s: %s: %s", c.ID, tr.Kiem, tr.ChiTiet)
			}
		}
	}
	for _, bot := range []obs.Bot{obs.BotNep, obs.BotNhom} {
		yd, _ := hieu.YDinhCua(bot)
		for _, y := range yd.Values() {
			if !phu[string(bot)+"/"+y] {
				t.Errorf("no T1 case for %s/%s", bot, y)
			}
		}
		if !phu[string(bot)+"/tu_choi_tien"] {
			t.Errorf("no money refusal case for %s", bot)
		}
	}
	for _, k := range []string{"han_che", "hoi_lai", "nhap_tien", "loi_hieu", "sua"} {
		if !phu[k] {
			t.Errorf("no T1 case for %s", k)
		}
	}
}

// Canary: a wrong expectation is red at the check it names, and nowhere
// else; the identity (the case as written) is green.
func TestBoHieuT1Canary(t *testing.T) {
	raw, _ := os.ReadFile(filepath.Join(thuMucHieu, "t1-hieu.json"))
	b, err := DocBoHieu(raw)
	if err != nil {
		t.Fatal(err)
	}
	var ca CaHieu
	for _, c := range b.Ca {
		if c.ID == "nep-find-places" {
			ca = c
		}
	}
	chay := func(c CaHieu) []TruotHieu {
		var out []TruotHieu
		ChayBoHieu(context.Background(), hieu.Moi(), BoHieu{Bo: "x", Luc: b.Luc, Ca: []CaHieu{c}}, stubTu,
			func(r KetQuaHieu) { out = r.Truot })
		return out
	}
	if tr := chay(ca); len(tr) != 0 {
		t.Fatalf("identity red: %+v", tr)
	}
	for kiem, sua := range map[string]func(*KyVongHieu){
		"tien":         func(k *KyVongHieu) { k.Tien = []string{"money_action"} },
		"y_dinh":       func(k *KyVongHieu) { k.YDinh = []string{"plan_help"} },
		"di_ung_thieu": func(k *KyVongHieu) { k.DiUng = []string{"tom", "cua"} },
		"di_ung_thua":  func(k *KyVongHieu) { k.DiUng = nil },
		"quyet_dinh":   func(k *KyVongHieu) { k.QuyetDinh = &hieu.QuyetDinh{HanChe: true} },
		"so_goi":       func(k *KyVongHieu) { k.SoGoi = 2 },
	} {
		c := ca
		sua(&c.KyVong)
		tr := chay(c)
		if len(tr) != 1 || tr[0].Kiem != kiem {
			t.Errorf("%s: red at %+v", kiem, tr)
		}
	}
}

// The scorer on the converted sets: a perfect router passes every case and
// its confusion counts equal the labels; a router that never refuses money
// misses exactly the money cases; one that reads no allergen misses every
// case with a required one.
func TestChamBoHieuChuyen(t *testing.T) {
	for _, ten := range []string{"tien_v3.json", "di_ung_v3.json"} {
		raw, err := os.ReadFile(filepath.Join(thuMucHieu, ten))
		if err != nil {
			t.Fatal(err)
		}
		b, err := DocBoHieu(raw)
		if err != nil {
			t.Fatal(err)
		}
		hoanHao := func(c CaHieu) model.LLM {
			tien := "none"
			if len(c.KyVong.Tien) > 0 {
				tien = c.KyVong.Tien[0]
			}
			slots, _ := json.Marshal(map[string][]string{"di_ung": c.KyVong.DiUng, "an_kieng": c.KyVong.AnKieng})
			return llm.NewStub(llm.Buoc{Text: `{"nhan_guard":"sach","tien":"` + tien + `","y_dinh":["find_places"],"huong":"tra_loi_thang","slots":` +
				strings.ReplaceAll(string(slots), "null", "[]") + `,"can_truy_hoi":[],"truy_van":[],"can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"cao"}`})
		}
		tk := ChayBoHieu(context.Background(), hieu.Moi(), b, hoanHao, nil)
		if tk.Dat != tk.SoCa || tk.LoiHieu != 0 || tk.Tien.FP+tk.Tien.FN != 0 {
			t.Fatalf("%s perfect router: %+v", ten, tk)
		}
		mu := func(CaHieu) model.LLM {
			return llm.NewStub(llm.Buoc{Text: `{"nhan_guard":"sach","tien":"none","y_dinh":["find_places"],"huong":"tra_loi_thang","slots":{},"can_truy_hoi":[],"truy_van":[],"can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"cao"}`})
		}
		tk2 := ChayBoHieu(context.Background(), hieu.Moi(), b, mu, nil)
		coTien, coDiUng := 0, 0
		for _, c := range b.Ca {
			if len(c.KyVong.Tien) > 0 && c.KyVong.Tien[0] != "none" {
				coTien++
			}
			if len(c.KyVong.DiUng) > 0 {
				coDiUng++
			}
		}
		if tk2.Tien.FN != coTien || tk2.Tien.TP != 0 || tk2.DiUngThieu != coDiUng {
			t.Fatalf("%s blind router: %+v (money %d, allergy %d)", ten, tk2, coTien, coDiUng)
		}
		if strings.HasPrefix(ten, "tien") && (tk.Tien.TP != coTien || coTien != 260) {
			t.Fatalf("%s: TP %d of %d", ten, tk.Tien.TP, coTien)
		}
	}
}

// Canary of the T1 request invariants: a real router request is green, and
// each damage is red at the check that names it.
func TestKiemYeuCauHieuCanary(t *testing.T) {
	c := CaHieu{ID: "x", Bot: obs.BotNep, Cau: "tối nay đi đâu"}
	c.Ra = []json.RawMessage{json.RawMessage(`{"nhan_guard":"sach","tien":"none","y_dinh":["smalltalk"],"huong":"tra_loi_thang","slots":{},"can_truy_hoi":[],"truy_van":[],"can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"cao"}`)}
	s := StubHieu(c)
	ChayBoHieu(context.Background(), hieu.Moi(), BoHieu{Bo: "x", Luc: LucBoHieu, Ca: []CaHieu{c}}, func(CaHieu) model.LLM { return s }, nil)
	req := string(s.YeuCau()[0])
	if tr := KiemYeuCauHieu([]byte(req), c); len(tr) != 0 {
		t.Fatalf("identity red: %+v", tr)
	}
	for kiem, hong := range map[string]func(string) string{
		"dong_bay_gio":     func(r string) string { return strings.Replace(r, "Bây giờ: ", "Bay gio: ", 1) },
		"cau_hoi_danh_dau": func(r string) string { return strings.Replace(r, "tốiˆnayˆđiˆđâu", "tối nay đi đâu", 1) },
		"luoc_do": func(r string) string {
			return strings.Replace(r, `"responseMimeType": "application/json"`, `"responseMimeType": "text/plain"`, 1)
		},
	} {
		h := hong(req)
		if h == req {
			t.Fatalf("%s: damage did not apply", kiem)
		}
		tr := KiemYeuCauHieu([]byte(h), c)
		if len(tr) != 1 || tr[0].Kiem != kiem {
			t.Errorf("%s: red at %+v", kiem, tr)
		}
	}
}

// Family coverage of the allergen scorer, the corpus's own rule.
func TestDocDuBaoTrum(t *testing.T) {
	bao := map[string][]string{"hai_san": {"tom", "cua"}}
	for name, c := range map[string]struct {
		req, pred, ok []string
		thieu, thua   int
	}{
		"exact":                  {[]string{"tom"}, []string{"tom"}, nil, 0, 0},
		"head covers member":     {[]string{"tom"}, []string{"hai_san"}, nil, 0, 0},
		"member not head":        {[]string{"hai_san"}, []string{"tom"}, nil, 1, 1},
		"missed":                 {[]string{"tom", "cua"}, []string{"tom"}, nil, 1, 0},
		"extra refused":          {[]string{"tom"}, []string{"tom", "sua"}, nil, 0, 1},
		"extra accepted":         {[]string{"tom"}, []string{"tom", "sua"}, []string{"sua"}, 0, 0},
		"nothing asked, nothing": {nil, nil, nil, 0, 0},
	} {
		thieu, thua := docDu(c.req, c.pred, c.ok, bao)
		if len(thieu) != c.thieu || len(thua) != c.thua {
			t.Errorf("%s: missed %v extra %v", name, thieu, thua)
		}
	}
}
