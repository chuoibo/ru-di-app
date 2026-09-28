package tuvung

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// The held-out corpus (testdata/giu_rieng.json) was written by someone who
// read neither this package, nor internal/rag, nor any allergy pattern: only
// §2.2, §4 and §5 of design 04. Its labels are a careful reader's, in its own
// words («tôm», «động vật có vỏ», «thuần chay»). The numbers below are what
// this package measures on it; they are pinned apart from the fitted goldens
// of tuvung_golden.json, and every miss is listed by id. The corpus file is
// never edited to fit: its sha256 is pinned.

const giuRiengSHA256 = "cc062cdea602fcb40c032f736cda596030c644439de62a81b7eebaaaa017c614"

type giuRieng struct {
	NguoiHoi []struct {
		ID              string   `json:"id"`
		Cau             string   `json:"cau"`
		DiUng           []string `json:"di_ung"`
		AnKieng         []string `json:"an_kieng"`
		Loai            string   `json:"loai"`
		MoHo            bool     `json:"mo_ho"`
		DiUngNguoiKhac  []string `json:"di_ung_nguoi_khac"`
		AnKiengNguoiKha []string `json:"an_kieng_nguoi_khac"`
	} `json:"nguoi_hoi"`
	Quan []struct {
		ID     string   `json:"id"`
		Chu    string   `json:"chu"`
		Chua   []string `json:"chua"`
		PhucVu []string `json:"phuc_vu"`
		MoHo   bool     `json:"mo_ho"`
	} `json:"quan"`
}

// nhanGiuRieng maps the corpus's labels onto the closed ids. The mapping is
// the corpus's own quy_uoc block: «động vật có vỏ» is sò, hàu, nghêu, ốc,
// hến (oc_so); «gluten» is lúa mì (lua_mi).
var nhanGiuRieng = map[string]string{
	"tôm": "tom", "cua": "cua", "cá": "ca", "mực": "muc", "động vật có vỏ": "oc_so", "hải sản": "hai_san",
	"đậu phộng": "dau_phong", "các loại hạt": "hat_cay", "sữa": "sua", "trứng": "trung", "mè": "me",
	"gluten": "lua_mi", "đậu nành": "dau_nanh",
	"chay": "chay", "thuần chay": "thuan_chay", "halal": "halal",
}

func docGiuRieng(t *testing.T) giuRieng {
	t.Helper()
	raw, err := os.ReadFile("testdata/giu_rieng.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != giuRiengSHA256 {
		t.Fatalf("the held-out corpus changed: sha256 %s, pinned %s", got, giuRiengSHA256)
	}
	var g giuRieng
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func maNhan(t *testing.T, labels []string) []string {
	t.Helper()
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		id, ok := nhanGiuRieng[l]
		if !ok {
			t.Fatalf("label %q is not in the corpus's closed sets", l)
		}
		out = append(out, id)
	}
	return out
}

func tapCo(ids []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func thieu(want, got map[string]bool) []string {
	var out []string
	for id := range want {
		if !got[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// soGiuRieng is everything measured on the corpus, misses by id.
type soGiuRieng struct {
	// Asker allergens. A row is recalled when the family closure of its
	// label is inside the closure of what DiUngNguoiHoi read: then the hard
	// filter hides every place the label would hide.
	coNhan, duNhan int
	lotNhan        []string
	// gia_mao rows (look-alikes, no allergen): any allergen read is false.
	giaMao, giaMaoSai int
	giaMaoSaiID       []string
	// Allergens read beyond the label (and beyond another person's
	// allergens, which design 04 §5.1 reads on purpose) on every other row.
	thua   int
	thuaID []string
	// Rows naming someone else's allergy, and how many of those the scan
	// reads (it should: the filter serves the whole outing).
	nguoiKhac, nguoiKhacDoc int
	// Asker diets: rows whose diets are read exactly (reported), and the
	// labelled diets missed, by id (gated: the diet filter then shows
	// places that do not serve the diet).
	kiengDung, kiengCo int
	kiengLech          []string
	kiengLot           []string
	// Places, diet: a diet read that the label does not give is a wrong
	// «serves diet X»; the target is none. Diets the label gives and the
	// reading misses are reported.
	quanKiengSai   int
	quanKiengSaiID []string
	quanKiengCo    int
	quanKiengLot   []string
	// Places, allergens: each labelled allergen must hide the place from
	// someone allergic to it (its closure meets the tags).
	quanDiUngCo  int
	quanDiUngLot []string
	// Allergen tags the label does not give: the safe direction, counted.
	quanDiUngThua int
}

func (s soGiuRieng) String() string {
	return fmt.Sprintf("nguoi_hoi: recall %d/%d, gia_mao sai %d/%d, thua %d, nguoi_khac doc %d/%d, an_kieng dung %d/%d\n"+
		"quan: an_kieng sai %d, an_kieng lot %d/%d, di_ung lot %d/%d, di_ung thua %d",
		s.duNhan, s.coNhan, s.giaMaoSai, s.giaMao, s.thua, s.nguoiKhacDoc, s.nguoiKhac, s.kiengDung, s.kiengCo,
		s.quanKiengSai, len(s.quanKiengLot), s.quanKiengCo, len(s.quanDiUngLot), s.quanDiUngCo, s.quanDiUngThua)
}

func doGiuRieng(t *testing.T, g giuRieng) soGiuRieng {
	t.Helper()
	var s soGiuRieng
	for _, r := range g.NguoiHoi {
		got := DiUngNguoiHoi(r.Cau)
		gotClosed := tapCo(MoRongDiUng(got))
		label := maNhan(t, r.DiUng)
		other := maNhan(t, r.DiUngNguoiKhac)
		mo := ""
		if r.MoHo {
			mo = "~"
		}
		if len(label) > 0 {
			s.coNhan++
			if miss := thieu(tapCo(MoRongDiUng(label)), gotClosed); len(miss) == 0 {
				s.duNhan++
			} else {
				s.lotNhan = append(s.lotNhan, fmt.Sprintf("%s%s:%s", mo, r.ID, strings.Join(miss, "+")))
			}
		}
		if r.Loai == "gia_mao" {
			s.giaMao++
			if len(got) > 0 {
				s.giaMaoSai++
				s.giaMaoSaiID = append(s.giaMaoSaiID, fmt.Sprintf("%s:%s", r.ID, strings.Join(got, "+")))
			}
		} else if extra := thieu(gotClosed, tapCo(MoRongDiUng(append(label, other...)))); len(extra) > 0 {
			s.thua++
			s.thuaID = append(s.thuaID, fmt.Sprintf("%s%s:%s", mo, r.ID, strings.Join(extra, "+")))
		}
		if len(other) > 0 {
			s.nguoiKhac++
			if len(thieu(tapCo(MoRongDiUng(other)), gotClosed)) == 0 {
				s.nguoiKhacDoc++
			}
		}
		diets := DoiKieng(maNhan(t, r.AnKieng))
		gotDiets := docAnKiengNguoiHoi(r.Cau)
		if len(diets) > 0 || len(gotDiets) > 0 {
			s.kiengCo++
			if strings.Join(diets, ",") == strings.Join(gotDiets, ",") {
				s.kiengDung++
			} else {
				s.kiengLech = append(s.kiengLech, fmt.Sprintf("%s%s:%v→%v", mo, r.ID, diets, gotDiets))
			}
			if miss := thieu(tapCo(diets), tapCo(gotDiets)); len(miss) > 0 {
				s.kiengLot = append(s.kiengLot, fmt.Sprintf("%s%s:%s", mo, r.ID, strings.Join(miss, "+")))
			}
		}
	}
	for _, q := range g.Quan {
		mo := ""
		if q.MoHo {
			mo = "~"
		}
		label := tapCo(DoiKieng(maNhan(t, q.PhucVu)))
		got := tapCo(docAnKiengQuan(q.Chu))
		for _, wrong := range thieu(got, label) {
			s.quanKiengSai++
			s.quanKiengSaiID = append(s.quanKiengSaiID, fmt.Sprintf("%s%s:%s", mo, q.ID, wrong))
		}
		if len(label) > 0 {
			s.quanKiengCo++
			if miss := thieu(label, got); len(miss) > 0 {
				s.quanKiengLot = append(s.quanKiengLot, fmt.Sprintf("%s%s:%s", mo, q.ID, strings.Join(miss, "+")))
			}
		}
		tags := tapCo(DiUngQuan(q.Chu))
		chua := maNhan(t, q.Chua)
		if len(chua) > 0 {
			s.quanDiUngCo++
		}
		var lot []string
		for _, a := range chua {
			if !anNhan(a, tags) {
				lot = append(lot, a)
			}
		}
		if len(lot) > 0 {
			s.quanDiUngLot = append(s.quanDiUngLot, fmt.Sprintf("%s%s:%s", mo, q.ID, strings.Join(lot, "+")))
		}
		if len(thieu(tags, tapCo(MoRongDiUng(chua)))) > 0 {
			s.quanDiUngThua++
		}
	}
	return s
}

// anNhan reports whether a place tagged tags is hidden from someone
// allergic to a, its label: a «hải sản» label only by the seafood tag or by
// every seafood child (a place tagged only «tôm» is still shown to a
// crab-allergic asker); any other by itself or its family head.
func anNhan(a string, tags map[string]bool) bool {
	if a == "hai_san" {
		if tags["hai_san"] {
			return true
		}
		for child, head := range hoDiUng {
			if head == "hai_san" && !tags[child] {
				return false
			}
		}
		return true
	}
	for _, c := range MoRongDiUng([]string{a}) {
		if tags[c] {
			return true
		}
	}
	return false
}

func TestAnNhanHaiSanCanDuHo(t *testing.T) {
	for _, c := range []struct {
		label string
		tags  []string
		want  bool
	}{
		{"hai_san", []string{"hai_san"}, true},
		{"hai_san", []string{"tom", "cua", "muc", "oc_so", "ca"}, true},
		{"hai_san", []string{"tom"}, false},
		{"hai_san", []string{"tom", "cua", "muc", "oc_so"}, false},
		{"cua", []string{"hai_san"}, true},
		{"cua", []string{"tom"}, false},
	} {
		if got := anNhan(c.label, tapCo(c.tags)); got != c.want {
			t.Errorf("label %s, tags %v: hidden %v, want %v", c.label, c.tags, got, c.want)
		}
	}
}

// The readers the corpus measures: the asker's diets are read as DocCau
// reads them; a place text is read as one field a place declares itself
// with (its name, a kind, a trait), the only fields the index takes diets
// from.
func docAnKiengNguoiHoi(text string) []string { return DoiKieng(AnKieng.QuetKhongPhuDinh(text)) }
func docAnKiengQuan(text string) []string     { return AnKiengQuan(text) }

// Gated on the corpus only in the unsafe direction, miss for miss: every
// asker allergen read (family-closed), every asker diet read, no place
// text read as serving a diet it does not, no labelled place allergen left
// untagged (a «hải sản» label needs the seafood tag or every child). Everything
// else is reported, not gated -- look-alikes read, allergens read beyond
// the label, diets missed -- because reading too much only hides places:
// the asker reader is the safety net unioned with Understand (design 04
// §5.1), and a gate on over-reading is what pushed round 1 into dropping
// stated allergens to make «look-alikes 0/60» pass. A leading «~» marks a
// row the corpus author labelled a best guess (mo_ho).
//
// Measured blind on 5c3a3c1, the same corpus gave: asker recall 60/166,
// look-alikes read 2/60 (h240, h249), 4 extra, others' allergens 4/6, asker
// diets 31/38; places: 15 wrong diets, 8/24 diets missed, 22/61 allergens
// missed, 18 extra. It has been read by every fixer since, so the numbers
// below are fitted, not blind; the blind measure of this reader is the
// sealed half of di_ung_heldout_v2, which only the reviewer opens.
var ghimGiuRieng = struct {
	lotNhan, kiengLot, quanKiengSai, quanDiUngLot []string
}{}

func TestGiuRieng(t *testing.T) {
	g := docGiuRieng(t)
	if len(g.NguoiHoi) != 277 || len(g.Quan) != 130 {
		t.Fatalf("%d asker rows, %d place rows; want 277 and 130", len(g.NguoiHoi), len(g.Quan))
	}
	s := doGiuRieng(t, g)
	t.Logf("%s", s)
	t.Logf("reported, not gated: look-alikes read %v; beyond the label %v; asker diets off %v; place diets missed %v",
		s.giaMaoSaiID, s.thuaID, s.kiengLech, s.quanKiengLot)
	w := ghimGiuRieng
	for _, c := range []struct {
		ten       string
		got, want []string
	}{
		{"asker allergens missed", s.lotNhan, w.lotNhan},
		{"asker diets missed", s.kiengLot, w.kiengLot},
		{"places read as serving a diet they do not", s.quanKiengSaiID, w.quanKiengSai},
		{"place allergens left untagged", s.quanDiUngLot, w.quanDiUngLot},
	} {
		if strings.Join(c.got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s:\n got  %q\n want %q", c.ten, c.got, c.want)
		}
	}
}
