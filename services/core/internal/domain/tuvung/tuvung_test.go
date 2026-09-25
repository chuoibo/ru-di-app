package tuvung

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/catalog"
	"mobile/services/core/internal/domain/promptsafety"
)

type golden struct {
	Quet []struct {
		TuVung string   `json:"tu_vung"`
		Chu    string   `json:"chu"`
		Ra     []string `json:"ra"`
	} `json:"quet"`
	KhongPhuDinh []struct {
		TuVung string   `json:"tu_vung"`
		Chu    string   `json:"chu"`
		Ra     []string `json:"ra"`
	} `json:"khong_phu_dinh"`
	AnKiengQuan []struct {
		Chu string   `json:"chu"`
		Ra  []string `json:"ra"`
	} `json:"an_kieng_quan"`
	NguoiHoi []struct {
		Chu string   `json:"chu"`
		Ra  []string `json:"ra"`
	} `json:"nguoi_hoi"`
	MoRong []struct {
		Vao []string `json:"vao"`
		Ra  []string `json:"ra"`
	} `json:"mo_rong_di_ung"`
	KhongDau []struct {
		Chu string `json:"chu"`
		Ra  bool   `json:"ra"`
	} `json:"khong_dau"`
}

func load(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/tuvung_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

var byName = map[string]*TuVung{"di_ung": DiUng, "an_kieng": AnKieng, "loai_cho": LoaiCho, "khi_chat": KhiChat}

func same(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func TestGoldenQuet(t *testing.T) {
	g := load(t)
	if len(g.Quet) < 25 || len(g.KhongPhuDinh) < 4 {
		t.Fatalf("golden too small: %d, %d", len(g.Quet), len(g.KhongPhuDinh))
	}
	for _, c := range g.Quet {
		got := byName[c.TuVung].Quet(c.Chu)
		if c.TuVung == "di_ung" {
			// A place's allergens: the phrases plus the one-syllable words
			// typed with their own marks.
			got = DiUngQuan(c.Chu)
		}
		if !same(got, c.Ra) {
			t.Errorf("%s.Quet(%q) = %q, want %q", c.TuVung, c.Chu, got, c.Ra)
		}
	}
	if len(g.AnKiengQuan) < 20 {
		t.Fatalf("only %d an_kieng_quan cases", len(g.AnKiengQuan))
	}
	for _, c := range g.AnKiengQuan {
		if got := AnKiengQuan(c.Chu); !same(got, c.Ra) {
			t.Errorf("AnKiengQuan(%q) = %q, want %q", c.Chu, got, c.Ra)
		}
	}
	for _, c := range g.KhongPhuDinh {
		if got := byName[c.TuVung].QuetKhongPhuDinh(c.Chu); !same(got, c.Ra) {
			t.Errorf("%s.QuetKhongPhuDinh(%q) = %q, want %q", c.TuVung, c.Chu, got, c.Ra)
		}
	}
}

func TestGoldenNguoiHoiMoRongKhongDau(t *testing.T) {
	g := load(t)
	for _, c := range g.NguoiHoi {
		if got := DiUngNguoiHoi(c.Chu); !same(got, c.Ra) {
			t.Errorf("DiUngNguoiHoi(%q) = %q, want %q", c.Chu, got, c.Ra)
		}
	}
	for _, c := range g.MoRong {
		if got := MoRongDiUng(c.Vao); !same(got, c.Ra) {
			t.Errorf("MoRongDiUng(%q) = %q, want %q", c.Vao, got, c.Ra)
		}
	}
	for _, c := range g.KhongDau {
		if got := KhongDau(c.Chu); got != c.Ra {
			t.Errorf("KhongDau(%q) = %v, want %v", c.Chu, got, c.Ra)
		}
	}
}

// Each phrase finds itself, written with or without its marks: a phrase the
// matcher cannot see is a word the vocabulary silently ignores.
func TestEveryPhraseFindsItself(t *testing.T) {
	n := 0
	for name, v := range byName {
		for _, m := range v.Muc() {
			for _, c := range m.Cum {
				for _, text := range []string{c, promptsafety.Fold(c), "trước " + strings.ToUpper(c) + " sau"} {
					found := false
					for _, id := range v.Quet(text) {
						found = found || id == m.ID
					}
					if !found {
						t.Errorf("%s: %q does not find %s", name, text, m.ID)
					}
					n++
				}
			}
		}
	}
	if n < 400 {
		t.Fatalf("only %d phrase checks", n)
	}
}

// Syllables that fold onto an everyday word may only appear inside a longer
// phrase. Each entry is the collision that bans it.
var dongAm = map[string]string{
	"cua": "của", "ca": "cà, ca", "muc": "mức, mục", "so": "số, sợ", "hau": "hậu, hầu", "hen": "hẹn",
	"trung": "trung tâm", "sua": "sửa", "pho": "phố", "lau": "lâu", "vung": "vùng", "me": "mẹ, mê",
	"lac": "lạc đường", "dem": "đem", "nhau": "nhau", "chay": "chạy, cháy", "ghe": "ghé, ghế", "bo": "bò",
	"re": "rẽ", "sang": "sáng", "toi": "tôi, tối", "kem": "kèm", "hat": "hát", "dau": "đâu, đau",
	// «quẩy» (to party) was in soi_dong until the golden fixture's own
	// check caught «sẽ quay lại» tagging a plain noodle shop as lively.
	"quay": "quay lại",
}

func TestNoSingleSyllableCollision(t *testing.T) {
	for name, v := range byName {
		for _, m := range v.Muc() {
			for _, c := range m.Cum {
				s := AmTiet(c)
				if len(s) == 1 {
					if why, bad := dongAm[s[0]]; bad {
						t.Errorf("%s/%s: phrase %q folds onto %s; use it only inside a longer phrase", name, m.ID, c, why)
					}
				}
			}
		}
	}
}

// loai_cho is the product's four categories, in their order, and nothing else.
func TestLoaiChoIsTheCatalogue(t *testing.T) {
	var want []string
	for _, c := range catalog.Categories {
		want = append(want, c.ID)
	}
	if !reflect.DeepEqual(LoaiCho.IDs(), want) {
		t.Fatalf("loai_cho %v, catalogue %v", LoaiCho.IDs(), want)
	}
	for _, id := range want {
		if LoaiCho.Nhan(id) == "" {
			t.Errorf("no label for %s", id)
		}
	}
}

func TestLocHopLeDropsInventedValues(t *testing.T) {
	if got := KhiChat.LocHopLe([]string{"lam_viec", "bịa", "yen_tinh", "yen_tinh"}); !reflect.DeepEqual(got, []string{"yen_tinh", "lam_viec"}) {
		t.Fatalf("LocHopLe = %q", got)
	}
	if AnKieng.Co("keto") || !AnKieng.Co("halal") {
		t.Fatal("Co is wrong")
	}
	if got := DoiKieng([]string{"thuan_chay"}); !reflect.DeepEqual(got, []string{"chay", "thuan_chay"}) {
		t.Fatalf("DoiKieng = %q", got)
	}
}

func TestLaTuDung(t *testing.T) {
	for _, s := range []string{"quan", "nao", "o", "30", "2026"} {
		if !LaTuDung(s) {
			t.Errorf("%q should be a stop word", s)
		}
	}
	for _, s := range []string{"lau", "cafe", "may", "300k", "da", "lat", ""} {
		if LaTuDung(s) {
			t.Errorf("%q should not be a stop word", s)
		}
	}
}

func TestAmTietIsFoldThenSyllables(t *testing.T) {
	got := AmTiet("Cà phê, ĐÀ LẠT! 24/7")
	want := []string{"ca", "phe", "da", "lat", "24", "7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AmTiet = %q, want %q", got, want)
	}
}

// catCau keeps AmTiet's syllables one for one, with the raw forms and the
// breaks beside them.
func TestCatCauMatchesAmTiet(t *testing.T) {
	for _, text := range []string{"Cà phê, ĐÀ LẠT! 24/7", "Món chay: không có.", "dị ứng cả tôm lẫn cua", "Non-halal; x́y", "", "!!!"} {
		c := catCau(text)
		if !reflect.DeepEqual(c.s, AmTiet(text)) && !(len(c.s) == 0 && len(AmTiet(text)) == 0) {
			t.Errorf("%q: %q, AmTiet %q", text, c.s, AmTiet(text))
		}
		if len(c.raw) != len(c.s) || len(c.ngat) != len(c.s) {
			t.Errorf("%q: lengths %d %d %d", text, len(c.s), len(c.raw), len(c.ngat))
		}
	}
	c := catCau("Món chay: không có. Cả tôm, cua")
	if !reflect.DeepEqual(c.raw, []string{"món", "chay", "không", "có", "cả", "tôm", "cua"}) ||
		!reflect.DeepEqual(c.ngat, []uint8{0, 0, ngatHaiCham, 0, ngatCau, 0, ngatVe}) || !c.coDau {
		t.Fatalf("%+v", c)
	}
}

// Every one-syllable allergen word is read on a place typed with its own
// marks, and never typed as the everyday word it folds onto; after a trigger
// it is read typed either way.
func TestMotAmChiDocDungDau(t *testing.T) {
	dongAmCua := map[string][]string{
		"cua": {"của", "cửa"}, "ghe": {"ghé", "ghế"}, "ca": {"cà", "cả", "ca"}, "muc": {"mức", "mục"}, "so": {"số", "sợ"},
		"hau": {"hậu", "hầu"}, "hen": {"hẹn"}, "tep": {}, "ruoc": {"rước"}, "mam": {"mâm", "mầm"}, "trung": {"trung", "trúng"},
		"sua": {"sửa", "sứa"}, "me": {"mẹ", "mê", "me"}, "vung": {"vùng"}, "lac": {"lác"}, "hat": {"hát"}, "chao": {"cháo", "chào"},
		"hs": {},
	}
	for key, d := range dauMotAm {
		others, ok := dongAmCua[key]
		if !ok {
			t.Errorf("%s: no collisions listed for this test", key)
		}
		if d.quan != nil {
			if got := DiUngQuan("Quán có " + d.co + " ngon"); !reflect.DeepEqual(got, DiUng.LocHopLe(d.quan)) {
				t.Errorf("place «%s» read %v, want %v", d.co, got, d.quan)
			}
		} else if got := DiUngQuan("Quán có " + d.co + " ngon"); len(got) != 0 {
			t.Errorf("place «%s» read %v; it is never read on a place", d.co, got)
		}
		for _, o := range others {
			if got := DiUngQuan("Quán có " + o + " ngon"); len(got) != 0 {
				t.Errorf("place «%s» (not «%s») read %v", o, d.co, got)
			}
		}
		want := DiUng.LocHopLe(d.hoi)
		for _, text := range []string{"Mình dị ứng " + d.co, promptsafety.Fold("Mình dị ứng " + d.co)} {
			if got := DiUngNguoiHoi(text); !reflect.DeepEqual(got, want) {
				t.Errorf("asker %q read %v, want %v", text, got, want)
			}
		}
		for _, o := range others {
			if o == key {
				continue // a bare form is the allergen after a trigger
			}
			if got := DiUngNguoiHoi("Mình dị ứng " + o); len(got) != 0 {
				t.Errorf("asker «dị ứng %s» read %v", o, got)
			}
		}
	}
}

// The place diet list is AnKieng without «ăn chay», plus what only a place
// says: one list, so the two never drift.
func TestAnKiengQuanTuAnKieng(t *testing.T) {
	quan := map[string]map[string]bool{}
	for _, m := range anKiengQuan.Muc() {
		quan[m.ID] = map[string]bool{}
		for _, c := range m.Cum {
			quan[m.ID][c] = true
		}
	}
	for _, m := range AnKieng.Muc() {
		for _, c := range m.Cum {
			if quan[m.ID][c] == (c == "ăn chay") {
				t.Errorf("%s/%q: in the place list = %v", m.ID, c, quan[m.ID][c])
			}
		}
	}
	if !reflect.DeepEqual(anKiengQuan.IDs(), AnKieng.IDs()) {
		t.Fatalf("ids %v, %v", anKiengQuan.IDs(), AnKieng.IDs())
	}
}
