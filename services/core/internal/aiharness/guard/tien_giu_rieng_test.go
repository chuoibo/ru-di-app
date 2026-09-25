package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/preprocess"
)

// The labelled corpora of the money law. Each file is pinned by hash: a
// sentence edited to fit the law turns its test red before it can turn
// anything green. Neither is a held-out number any more -- the law's author
// read both while writing it. The held-out number is the reviewer's, on the
// sealed corpus (tien_niem_phong.json) that never enters this repository and
// that the law's author never opened; ADR-0037 proposal §4 makes that number
// a condition of the flag.
type boTien struct {
	file, sha256 string
	tien, khong  int
	// bat is how many money sentences the law refuses, batNham how many
	// others it refuses by mistake: measured, and a change to either is a
	// reviewed edit here.
	bat, batNham int
	// lot and nham name every sentence behind the gap, so a trade of one
	// miss for another cannot hide behind an unchanged count.
	lot, nham []string
}

var (
	// Written 2026-09-25 by someone who never opened this package, before
	// slice 6's first review round; the author of that round's law read it.
	boGiuRieng = boTien{
		file:   "testdata/tien_giu_rieng.json",
		sha256: "b97e37295f49f043c17113bd75fea157bc54aa7f55b5c76faf9015f6d353097b",
		tien:   157, khong: 104, bat: 155, batNham: 0,
		// Both ask a price per head with no bill, no split and no debt in
		// sight. Review round 2 (N1) and the v2 corpora label that reading
		// place search («buffet ở đây mỗi người bao nhiêu», «vé vào cổng bao
		// nhiêu tiền một người», «Bao nhiêu tiền một người cho set lẩu đó»),
		// and a false refusal costs more than a miss (tien.go): the law
		// follows them, and this corpus's label is outvoted.
		lot: []string{"chia: mỗi người bao nhiêu", "viet_tat: bn tiền 1 ng"},
	}
	// The DEV half of the v2 corpora, split by seed 20260925 from one pool
	// with the sealed half (same groups, no shared sentence even after
	// folding). Opened while writing the law.
	boDevV2 = boTien{
		file:   "testdata/tien_dev_v2.json",
		sha256: "399f94b5989dd0e2c2a9806b3b7f448df54942348272e02edf56d5612832deba",
		tien:   151, khong: 135, bat: 151, batNham: 0,
	}
)

type cauBo struct {
	Cau  string `json:"cau"`
	Nhom string `json:"nhom"`
	LyDo string `json:"ly_do"`
}

type tepBo struct {
	GhiChu    string  `json:"ghi_chu"`
	Tien      []cauBo `json:"tien"`
	KhongTien []cauBo `json:"khong_tien"`
}

func (b boTien) doc(t *testing.T) tepBo {
	t.Helper()
	raw, err := os.ReadFile(b.file)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != b.sha256 {
		t.Fatalf("%s đã bị sửa (sha256 %s): corpus không được sửa cho vừa luật", b.file, got)
	}
	var f tepBo
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatal(err)
	}
	if len(f.Tien) != b.tien || len(f.KhongTien) != b.khong {
		t.Fatalf("%s có %d câu tiền, %d câu không tiền", b.file, len(f.Tien), len(f.KhongTien))
	}
	for _, c := range f.KhongTien {
		if strings.TrimSpace(c.LyDo) == "" {
			t.Fatalf("%s: câu không tiền thiếu lý do: %q", b.file, c.Cau)
		}
	}
	return f
}

// laTienNhuEngine judges a sentence the way the engine does: preprocess
// first, then the law.
func laTienNhuEngine(s string) bool { return LaTien(preprocess.LamSach(s).Chu) }

// do measures the law on one corpus, per group, and holds it to its pins.
func (b boTien) do(t *testing.T) {
	f := b.doc(t)
	type dem struct{ bat, tong int }
	nhom := map[string]*dem{}
	cong := func(k string, trung bool) {
		d := nhom[k]
		if d == nil {
			d = &dem{}
			nhom[k] = d
		}
		d.tong++
		if trung {
			d.bat++
		}
	}
	var lot, nham []string
	for _, c := range f.Tien {
		ok := laTienNhuEngine(c.Cau)
		cong("tien/"+c.Nhom, ok)
		if !ok {
			lot = append(lot, c.Nhom+": "+c.Cau)
		}
	}
	for _, c := range f.KhongTien {
		ok := laTienNhuEngine(c.Cau)
		cong("khong_tien/"+c.Nhom, ok)
		if ok {
			nham = append(nham, c.Nhom+": "+c.Cau)
		}
	}
	keys := make([]string, 0, len(nhom))
	for k := range nhom {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%-36s bắt %3d/%3d", k, nhom[k].bat, nhom[k].tong)
	}
	bat, batNham := len(f.Tien)-len(lot), len(nham)
	t.Logf("%s: tiền bắt %d/%d (recall %.4f); không tiền bắt nhầm %d/%d (%.4f)",
		b.file, bat, len(f.Tien), float64(bat)/float64(len(f.Tien)), batNham, len(f.KhongTien), float64(batNham)/float64(len(f.KhongTien)))
	if bat != b.bat || batNham != b.batNham || !slices.Equal(lot, b.lot) || !slices.Equal(nham, b.nham) {
		t.Errorf("luật tiền trên %s: bắt %d/%d (ghim %d), bắt nhầm %d/%d (ghim %d)\nlọt:\n  %s\nbắt nhầm:\n  %s",
			b.file, bat, len(f.Tien), b.bat, batNham, len(f.KhongTien), b.batNham,
			strings.Join(lot, "\n  "), strings.Join(nham, "\n  "))
	}
}

func TestLuatTienGiuRieng(t *testing.T) { boGiuRieng.do(t) }

func TestLuatTienDevV2(t *testing.T) { boDevV2.do(t) }

// No rule is dead weight: each one refuses at least one money sentence of
// the labelled sets on its own, and none refuses a look-alike. The log shows
// which rule refuses first, per rule.
func TestMoiLuatTienDeuCoCau(t *testing.T) {
	var tien, khong []string
	for _, b := range []boTien{boGiuRieng, boDevV2} {
		f := b.doc(t)
		for _, c := range f.Tien {
			tien = append(tien, c.Cau)
		}
		for _, c := range f.KhongTien {
			khong = append(khong, c.Cau)
		}
	}
	tien = append(append(append(tien, cauTien...), cauTienReview2...), cauTienTuViet...)
	khong = append(append(append(khong, khongPhaiTien...), khongTienReview2...), khongTienTuViet...)
	// Read each sentence once; every rule then runs on the same reading.
	doc := func(ss []string) []string {
		out := make([]string, len(ss))
		for i, s := range ss {
			out[i] = docTien(preprocess.LamSach(s).Chu)
		}
		return out
	}
	gTien, gKhong := doc(tien), doc(khong)
	dau := map[string]int{}
	for _, s := range tien {
		if ten, ok := luatTienKhop(preprocess.LamSach(s).Chu); ok {
			dau[ten]++
		}
	}
	for _, l := range luatTiens {
		n := 0
		for _, g := range gTien {
			if g != "" && l.re.MatchString(g) {
				n++
			}
		}
		t.Logf("%-14s khớp %3d câu tiền, đứng đầu ở %3d", l.ten, n, dau[l.ten])
		if n == 0 {
			t.Errorf("luật %s không bắt câu tiền nào", l.ten)
		}
		for i, g := range gKhong {
			if g != "" && l.re.MatchString(g) {
				t.Errorf("luật %s bắt nhầm %q", l.ten, khong[i])
			}
		}
	}
}

// Each place reading is load-bearing: turned off, it lets through a
// false refusal it exists to stop.
func TestCachDocNoiChon(t *testing.T) {
	for _, c := range []struct{ cau, cho string }{
		{"gửi mình quán dưới 100k", "qqgia"},
		{"chuyển kèo sang quán 200k", "qqviec"},
		{"Đưa mẹ đi ăn tầm 500k", "qqviec"},
		{"Quán nào cho chuyển khoản, mình không mang tiền mặt", "qqthuoctinh"},
		{"Chỗ cắm trại có phải đóng tiền vào cổng không", "qqphi"},
		{"Tiền vé vào cổng mỗi người bao nhiêu", "qqgia"},
		{"Mỗi người trả khoảng bao nhiêu ở quán nướng đó", "qquoc"},
		{"ck 250 cho Nam", "qqso"},
		{"thu 7 quan nao 100k", "thungay"},
	} {
		g := chuTien(preprocess.LamSach(c.cau).Chu)
		if !strings.Contains(g, c.cho) {
			t.Errorf("%q đọc thành %q, thiếu %s", c.cau, g, c.cho)
		}
	}
}
