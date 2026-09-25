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
		if got := byName[c.TuVung].Quet(c.Chu); !same(got, c.Ra) {
			t.Errorf("%s.Quet(%q) = %q, want %q", c.TuVung, c.Chu, got, c.Ra)
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
