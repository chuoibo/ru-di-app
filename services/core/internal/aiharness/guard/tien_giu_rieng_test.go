package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/preprocess"
)

// The held-out corpus of the money law. It was written by someone who never
// opened this package or any money regex, with labels taken from the design
// alone (its `ghi_chu` says which lines, and what was left out on purpose).
// The file is pinned by hash: a sentence edited to fit the law turns this
// test red before it can turn anything green.
const (
	giuRiengFile   = "testdata/tien_giu_rieng.json"
	giuRiengSHA256 = "b97e37295f49f043c17113bd75fea157bc54aa7f55b5c76faf9015f6d353097b"
	// What the law reaches on it. Recall must stay 157/157 and false
	// positives 0/104; a change to either number is a reviewed edit here.
	giuRiengTien      = 157
	giuRiengKhongTien = 104
	giuRiengBat       = 157
	giuRiengBatNham   = 0
)

type cauGiuRieng struct {
	Cau  string `json:"cau"`
	Nhom string `json:"nhom"`
	LyDo string `json:"ly_do"`
}

type boGiuRieng struct {
	GhiChu    string        `json:"ghi_chu"`
	Tien      []cauGiuRieng `json:"tien"`
	KhongTien []cauGiuRieng `json:"khong_tien"`
}

func docGiuRieng(t *testing.T) boGiuRieng {
	t.Helper()
	raw, err := os.ReadFile(giuRiengFile)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != giuRiengSHA256 {
		t.Fatalf("%s đã bị sửa (sha256 %s): corpus giữ riêng không được sửa cho vừa luật", giuRiengFile, got)
	}
	var b boGiuRieng
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Tien) != giuRiengTien || len(b.KhongTien) != giuRiengKhongTien {
		t.Fatalf("corpus có %d câu tiền, %d câu không tiền", len(b.Tien), len(b.KhongTien))
	}
	for _, c := range b.KhongTien {
		if strings.TrimSpace(c.LyDo) == "" {
			t.Fatalf("câu không tiền thiếu lý do: %q", c.Cau)
		}
	}
	return b
}

// laTienNhuEngine judges a sentence the way the engine does: preprocess
// first, then the law.
func laTienNhuEngine(s string) bool { return LaTien(preprocess.LamSach(s).Chu) }

// No rule is dead weight: each one refuses at least one money sentence of
// the two corpora on its own, and none refuses a look-alike. The log shows
// which rule refuses first, per rule.
func TestMoiLuatTienDeuCoCau(t *testing.T) {
	b := docGiuRieng(t)
	var tien, khong []string
	for _, c := range b.Tien {
		tien = append(tien, c.Cau)
	}
	tien = append(tien, cauTien...)
	for _, c := range b.KhongTien {
		khong = append(khong, c.Cau)
	}
	khong = append(khong, khongPhaiTien...)
	dau := map[string]int{}
	for _, s := range tien {
		if ten, ok := luatTienKhop(preprocess.LamSach(s).Chu); ok {
			dau[ten]++
		}
	}
	for _, l := range luatTiens {
		n := 0
		for _, s := range tien {
			if l.re.MatchString(chuTien(preprocess.LamSach(s).Chu)) {
				n++
			}
		}
		t.Logf("%-14s khớp %3d câu tiền, đứng đầu ở %3d", l.ten, n, dau[l.ten])
		if n == 0 {
			t.Errorf("luật %s không bắt câu tiền nào", l.ten)
		}
		for _, s := range khong {
			g := chuTien(preprocess.LamSach(s).Chu)
			// A question about what a word means passes before any rule.
			if nghiaTu.MatchString(g) && !strings.Contains(g, "qqtien") {
				continue
			}
			if l.re.MatchString(g) {
				t.Errorf("luật %s bắt nhầm %q", l.ten, s)
			}
		}
	}
}

func TestLuatTienGiuRieng(t *testing.T) {
	b := docGiuRieng(t)
	type dem struct{ bat, tong int }
	nhom := map[string]*dem{}
	var lot, nham []string
	bat := 0
	for _, c := range b.Tien {
		d := nhom["tien/"+c.Nhom]
		if d == nil {
			d = &dem{}
			nhom["tien/"+c.Nhom] = d
		}
		d.tong++
		if laTienNhuEngine(c.Cau) {
			bat++
			d.bat++
		} else {
			lot = append(lot, c.Nhom+": "+c.Cau)
		}
	}
	batNham := 0
	for _, c := range b.KhongTien {
		d := nhom["khong_tien/"+c.Nhom]
		if d == nil {
			d = &dem{}
			nhom["khong_tien/"+c.Nhom] = d
		}
		d.tong++
		if laTienNhuEngine(c.Cau) {
			batNham++
			d.bat++
			nham = append(nham, c.Nhom+": "+c.Cau)
		}
	}
	keys := make([]string, 0, len(nhom))
	for k := range nhom {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%-26s bắt %3d/%3d", k, nhom[k].bat, nhom[k].tong)
	}
	t.Logf("tiền: bắt %d/%d; không tiền: bắt nhầm %d/%d", bat, len(b.Tien), batNham, len(b.KhongTien))
	if bat != giuRiengBat || batNham != giuRiengBatNham {
		t.Errorf("luật tiền trên corpus giữ riêng: bắt %d/%d (ghim %d), bắt nhầm %d/%d (ghim %d)\nlọt:\n  %s\nbắt nhầm:\n  %s",
			bat, len(b.Tien), giuRiengBat, batNham, len(b.KhongTien), giuRiengBatNham,
			strings.Join(lot, "\n  "), strings.Join(nham, "\n  "))
	}
}
