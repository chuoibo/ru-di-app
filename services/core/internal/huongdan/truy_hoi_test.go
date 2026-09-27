package huongdan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/promptsafety"
)

// Two golden sets: natural questions a person types into Nếp, each with the
// section(s) that answer it and the screen they are standing on.
//
//   - duongVang (slice 13, 91 questions): written before any ranking code
//     existed. In 77 of them the person stands on a screen that holds an
//     answer, so it mostly measures on-screen questions.
//   - duongManKhac (review of slice 13, 46 questions): every question asked
//     from a screen that holds no answer. Written and hashed before the
//     ranking changes of the commit that added it (the rune cap, the teencode
//     table, the pinning share), measured on the old ranking right after.
//
// Neither file is edited to fit the ranker: each is pinned to the sha256 it
// had when written (TestBoVangKhongSua). A new question goes into a new file
// committed before the change it grades.
//
//   - duongTruyVan (review no-heuristics, 26 queries): every teencode
//     question of the two sets above, rewritten as the router is told to
//     write truy_van (full words, diacritics, the app's words). Since no Go
//     table reads teencode any more, the text that reaches Tim on the
//     engine path is the MODEL's query; a person stood in for the router
//     here, wrote them before measuring, and a T3 run with the real router
//     replaces this set.
const (
	duongVang    = "testdata/truy-hoi-so-tay.json"
	duongManKhac = "testdata/truy-hoi-man-khac.json"
	duongTruyVan = "testdata/truy-hoi-truy-van-model.json"
)

var bamBoVang = map[string]string{
	duongVang:    "1f4768f93fd533b238052e5c1325e79f707449b4dcc5ddc8b9f2cc4338788f4e",
	duongManKhac: "f3287aedd0498261459133db8a2ba182b0d356257e0f032c4d0656d6c241dfc2",
	duongTruyVan: "2b90cdaa1c4574fbf7110967a73df15cd5b7bbae08cea0f4b30b08c4b7620119",
}

// docTruyVan reads duongTruyVan as golden questions (go "co_dau").
func docTruyVan(t testing.TB) []cauVang {
	t.Helper()
	raw, err := os.ReadFile(duongTruyVan)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		GhiChu string `json:"ghi_chu"`
		Cau    []struct {
			Hoi  string   `json:"hoi"`
			Goc  string   `json:"goc"`
			Man  string   `json:"man"`
			Dung []string `json:"dung"`
		} `json:"cau"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatal(err)
	}
	var out []cauVang
	for _, c := range f.Cau {
		out = append(out, cauVang{Hoi: c.Hoi, Man: c.Man, Dung: c.Dung, Go: "co_dau"})
	}
	return out
}

type cauVang struct {
	Hoi  string   `json:"hoi"`
	Man  string   `json:"man"`
	Dung []string `json:"dung"`
	Go   string   `json:"go"`
}

func docBoVang(t testing.TB, duong string) []cauVang {
	t.Helper()
	if duong == duongTruyVan {
		return docTruyVan(t)
	}
	raw, err := os.ReadFile(duong)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		GhiChu string    `json:"ghi_chu"`
		Cau    []cauVang `json:"cau"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatal(err)
	}
	return f.Cau
}

func docVang(t testing.TB) []cauVang { return docBoVang(t, duongVang) }

// ketQuaVang is what one run of a golden set (or one group of it) measured.
type ketQuaVang struct {
	recall5 float64 // mean over questions of |dung ∩ top 5| / |dung|
	mrr     float64 // mean of 1/rank of the first right section in the top 10
	truot   []string
}

func doVang(s *SoTay, cau []cauVang) ketQuaVang {
	var r ketQuaVang
	for _, c := range cau {
		top := s.tim(context.Background(), Hoi{Cau: c.Hoi, Man: c.Man, K: 10})
		dung := map[string]bool{}
		for _, id := range c.Dung {
			dung[id] = true
		}
		thay := 0
		for i, d := range top {
			if i < 5 && dung[d.ID] {
				thay++
			}
		}
		r.recall5 += float64(thay) / float64(len(c.Dung))
		hang := 0
		for i, d := range top {
			if dung[d.ID] {
				hang = i + 1
				break
			}
		}
		if hang > 0 {
			r.mrr += 1 / float64(hang)
		}
		if thay < len(c.Dung) {
			var ids []string
			for i, d := range top {
				if i < 5 {
					ids = append(ids, d.ID)
				}
			}
			r.truot = append(r.truot, fmt.Sprintf("%q (%s) want %v got %v", c.Hoi, c.Man, c.Dung, ids))
		}
	}
	r.recall5 /= float64(len(cau))
	r.mrr /= float64(len(cau))
	return r
}

// nhom is the questions of one typing style («» for all of them).
func nhom(cau []cauVang, g string) []cauVang {
	var out []cauVang
	for _, c := range cau {
		if g == "" || c.Go == g {
			out = append(out, c)
		}
	}
	return out
}

func kiemBoVang(t *testing.T, duong string, cau []cauVang) {
	t.Helper()
	khongDau := 0
	for _, c := range cau {
		if soTay.chuanMan(c.Man) != c.Man {
			t.Errorf("%s %q: màn %q không có trong _rut.json", duong, c.Hoi, c.Man)
		}
		if len(c.Dung) == 0 {
			t.Errorf("%s %q: không có mục đúng", duong, c.Hoi)
		}
		for _, id := range c.Dung {
			if _, ok := soTay.theoID[id]; !ok {
				t.Errorf("%s %q: mục %q không có trong sổ tay", duong, c.Hoi, id)
			}
		}
		// The declared typing style must be what the text is: a question
		// «without diacritics» has none, one «with» has some.
		coDau := promptsafety.Fold(c.Hoi) != strings.ToLower(c.Hoi)
		switch c.Go {
		case "co_dau":
			if !coDau {
				t.Errorf("%s %q khai co_dau mà không có dấu", duong, c.Hoi)
			}
		case "khong_dau", "teen":
			if coDau {
				t.Errorf("%s %q khai %s mà có dấu", duong, c.Hoi, c.Go)
			}
			khongDau++
		default:
			t.Errorf("%s %q: go %q lạ", duong, c.Hoi, c.Go)
		}
	}
	if 2*khongDau < len(cau) {
		t.Errorf("%s: chỉ %d/%d câu gõ không dấu hoặc teencode, cần ít nhất một nửa", duong, khongDau, len(cau))
	}
}

func TestBoVangHopLe(t *testing.T) {
	cau := docVang(t)
	if len(cau) < 75 || len(cau) > 100 {
		t.Fatalf("bộ vàng có %d câu, cần khoảng 80", len(cau))
	}
	kiemBoVang(t, duongVang, cau)

	khac := docBoVang(t, duongManKhac)
	if len(khac) < 30 {
		t.Fatalf("bộ màn khác có %d câu, cần ít nhất 30", len(khac))
	}
	kiemBoVang(t, duongManKhac, khac)
	for g, toiThieu := range map[string]int{"co_dau": 10, "khong_dau": 10, "teen": 10} {
		if n := len(nhom(khac, g)); n < toiThieu {
			t.Errorf("bộ màn khác: nhóm %s có %d câu, cần ít nhất %d", g, n, toiThieu)
		}
	}
	// What this set is for: the screen the person stands on holds no answer.
	for _, c := range khac {
		for _, id := range c.Dung {
			if soTay.doan[soTay.theoID[id]].Man == c.Man {
				t.Errorf("%q: màn %s có mục đúng %s, không phải màn khác", c.Hoi, c.Man, id)
			}
		}
	}
	seen := map[string]bool{}
	for _, c := range append(append([]cauVang{}, cau...), khac...) {
		if seen[c.Hoi] {
			t.Errorf("câu trùng: %q", c.Hoi)
		}
		seen[c.Hoi] = true
	}
}

// Each golden file is the bytes it had when written; a question edited to
// fit the ranker turns this red.
func TestBoVangKhongSua(t *testing.T) {
	for duong, want := range bamBoVang {
		raw, err := os.ReadFile(duong)
		if err != nil {
			t.Fatal(err)
		}
		tong := sha256.Sum256(raw)
		if got := hex.EncodeToString(tong[:]); got != want {
			t.Errorf("%s: sha256 %s, written as %s", duong, got, want)
		}
	}
}

// The measured quality of Tim, per set and per typing style, pinned to four
// decimals: a ranking change that moves one question by one rank moves one of
// these, and the change then has to say so here. Only the whole of each set
// is held to the bar of design 05 §3 (recall@5 ≥ 0.90).
//
// Measured when pinned (review no-heuristics, 2026-09-25: the teencode
// table and the stop-word gate are gone, every query term ranks). Recall@5
// did not move on any group of the two person-typed sets; MRR fell where
// raw teencode reaches Tim (duongVang teen 0.7917 → 0.7354, duongManKhac
// teen 0.7143 → 0.6458, the wholes 0.9211 → 0.9136 and 0.7880 → 0.7672,
// duongVang co_dau 0.8886 → 0.8883). Raw teencode no longer reaches Tim on
// the engine path: the model's rewrite does, and duongTruyVan, those same
// questions as the router writes them, holds recall@5 1.0000 and MRR
// 0.8942. On duongVang five questions miss: «checkin o dau» and «log out o
// dau» share no term with the manual in raw form; «thêm quán này vào buổi
// đi chơi của nhóm» finds one of its two sections; «mình lỡ vote nhầm, đổi
// lại được không» and «xem lại các buổi đã đi» are the price of pinning by
// share (tyLeGhim). On duongManKhac four miss.
//
// Re-pinned at the merge of origin/main into the AI v2 branch (2026-09-27):
// the ranker did not change, the manual did. Main redrew «Kèo mới», «Nhóm
// mới» and the itinerary modes («Hành trình» is now «Bản đồ», «Giờ» is the
// dial «Giờ chặng»), so keo.md, tao-keo.md and tin-nhan.md were rewritten to
// the labels the app now shows and every section's terms moved. duongVang:
// recall@5 unchanged on every group, MRR 0.9136 → 0.9127 (co_dau 0.8883 →
// 0.8859, teen 0.7354 → 0.7369); «checkin o dau» now hits and «ko bik bo
// fieu o dau» misses, still five misses. duongManKhac: recall@5 0.9130 →
// 0.9348 (co_dau 0.8000 → 0.8667), MRR 0.7672 → 0.7658 (co_dau 0.7444 →
// 0.7556, teen 0.6458 → 0.6293); three miss. duongTruyVan did not move.
//
// The numbers of the ranking of 5c3a3c1 on the same sets, for the record:
// duongVang 0.9725 / 0.8560 (teen 0.8333 / 0.6694), duongManKhac 0.8514 /
// 0.3526 (teen 0.8214 / 0.2905).
var vangGhim = map[string]map[string][2]string{
	duongVang: {
		"":          {"0.9505", "0.9127"},
		"co_dau":    {"0.9405", "0.8859"},
		"khong_dau": {"1.0000", "1.0000"},
		"teen":      {"0.8333", "0.7369"},
	},
	duongTruyVan: {
		"": {"1.0000", "0.8942"},
	},
	duongManKhac: {
		"":          {"0.9348", "0.7658"},
		"co_dau":    {"0.8667", "0.7556"},
		"khong_dau": {"1.0000", "0.8873"},
		"teen":      {"0.9286", "0.6293"},
	},
}

const nguongRecall = 0.90

func TestTruyHoiVang(t *testing.T) {
	for duong, theoNhom := range vangGhim {
		cau := docBoVang(t, duong)
		for g, want := range theoNhom {
			r := doVang(soTay, nhom(cau, g))
			if g == "" {
				for _, x := range r.truot {
					t.Logf("%s miss: %s", duong, x)
				}
				if r.recall5 < nguongRecall {
					t.Errorf("%s: recall@5 %.4f is under the bar %.2f", duong, r.recall5, nguongRecall)
				}
			}
			if got := fmt.Sprintf("%.4f", r.recall5); got != want[0] {
				t.Errorf("%s [%s]: recall@5 = %s, pinned %s", duong, g, got, want[0])
			}
			if got := fmt.Sprintf("%.4f", r.mrr); got != want[1] {
				t.Errorf("%s [%s]: MRR = %s, pinned %s", duong, g, got, want[1])
			}
		}
	}
}

// Design 04 §8.3: typing without diacritics may cost at most 0.05 of
// recall@5. Read one way: recall of the group with diacritics minus recall
// of the group without must be ≤ 0.05. The two groups are different
// questions, so the group without marks scoring HIGHER (it does, on both
// sets) says nothing about folding; the direct measure is the second half:
// every question with diacritics, typed without them, ranks exactly the same.
func TestKhoangCachDau(t *testing.T) {
	for _, duong := range []string{duongVang, duongManKhac} {
		cau := docBoVang(t, duong)
		co, khong := doVang(soTay, nhom(cau, "co_dau")), doVang(soTay, nhom(cau, "khong_dau"))
		gap := co.recall5 - khong.recall5
		t.Logf("%s: co_dau %.4f, khong_dau %.4f, gap %+.4f", duong, co.recall5, khong.recall5, gap)
		if gap > 0.05 {
			t.Errorf("%s: without diacritics loses %.4f of recall@5, bound 0.05", duong, gap)
		}
		for _, c := range nhom(cau, "co_dau") {
			a := idCua(Tim(context.Background(), Hoi{Cau: c.Hoi, Man: c.Man, K: 10}))
			b := idCua(Tim(context.Background(), Hoi{Cau: promptsafety.Fold(c.Hoi), Man: c.Man, K: 10}))
			if !reflect.DeepEqual(a, b) {
				t.Errorf("%q: %v, without diacritics %v", c.Hoi, a, b)
			}
		}
	}
}
