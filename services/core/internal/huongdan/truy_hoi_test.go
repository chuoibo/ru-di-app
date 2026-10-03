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
//
//   - duongHaiNguoi (P4, 13 questions, 2026-09-28): the owner's two classes
//     of chat. Every group and every ordinary two-person chat has Rủ Đi AI
//     and the same tools; only a two-person chat where both turned on «Một
//     đôi» adds «Tờ giấy». Questions about the AI and the tools in a
//     two-person chat and about Tờ giấy / Một đôi, asked from the chat or the
//     Messages screen; the right sections are those of the two-class manual.
//     Written, hashed and measured on the old one-class text before it was
//     rewritten.
const (
	duongVang    = "testdata/truy-hoi-so-tay.json"
	duongManKhac = "testdata/truy-hoi-man-khac.json"
	duongTruyVan = "testdata/truy-hoi-truy-van-model.json"
	// duongHaiNguoi is the P4 set of two-person chat questions.
	duongHaiNguoi = "testdata/truy-hoi-hai-nguoi.json"
)

var bamBoVang = map[string]string{
	duongVang:     "1f4768f93fd533b238052e5c1325e79f707449b4dcc5ddc8b9f2cc4338788f4e",
	duongManKhac:  "f3287aedd0498261459133db8a2ba182b0d356257e0f032c4d0656d6c241dfc2",
	duongTruyVan:  "2b90cdaa1c4574fbf7110967a73df15cd5b7bbae08cea0f4b30b08c4b7620119",
	duongHaiNguoi: "b8ccd398bf8183415d36c58ce01110685c7792b53a4b72c08d9f93690c428f87",
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
	haiNguoi := docBoVang(t, duongHaiNguoi)
	if len(haiNguoi) < 12 {
		t.Fatalf("bộ hai người có %d câu, cần ít nhất 12", len(haiNguoi))
	}
	kiemBoVang(t, duongHaiNguoi, haiNguoi)
	seen := map[string]bool{}
	for _, c := range append(append(append([]cauVang{}, cau...), khac...), haiNguoi...) {
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
// Re-pinned at the merge of main's memory book (2026-09-28): the ranker did
// not change, three labels the manual quotes did. Main renamed «Rủ một người
// đi chơi» to «Hẹn người thương», «Hai ô ràng buộc» to «Những điều cần
// tránh» and the Explore placeholder to «Một món thèm, một nơi muốn ghé…», so
// tao-moi.md, to-giay.md and kham-pha.md quote the new labels and keep the old
// words beside them in plain text (what a person still types). duongVang and
// duongTruyVan did not move. duongManKhac: recall@5 0.9348 → 0.9565 (co_dau
// 0.8667 → 0.9333), MRR 0.7658 → 0.7665 (co_dau 0.7556 → 0.7578); «moi nguoi
// ay di choi rieng» now hits, two miss.
//
// duongHaiNguoi added (P4, 2026-09-28), pinned on the manual as it was when
// the set was written, the one-class text that says a two-person chat only
// asks and has «Tờ giấy» in its tray: recall@5 0.7692, MRR 0.4635 (co_dau
// 0.8333 / 0.5333, khong_dau 0.5000 / 0.4062, teen 1.0000 / 0.4000); three
// miss. It is the baseline the two-class rewrite is graded against.
//
// Re-pinned at the two-class rewrite (P4, 2026-09-28): the ranker did not
// change, the manual did. chat-nhom.md, tin-nhan.md and to-giay.md now say a
// two-person chat has Rủ Đi AI and every tool a group has, and that «Tờ
// giấy» (pinned row, tray button) is there only when both turned on «Một
// đôi», reached otherwise from the settings row. Folding makes «đôi» the
// same term as «đổi», so every section that gains «Một đôi» lowers the
// weight of «đổi» everywhere; it is quoted in one indexed section
// (to-giay/cai-dat-so, which already held «đổi») and in the overviews.
// Before → after (recall@5 / MRR):
//
//	duongHaiNguoi   0.7692 / 0.4635 → 1.0000 / 0.6859
//	  co_dau        0.8333 / 0.5333 → 1.0000 / 0.6806
//	  khong_dau     0.5000 / 0.4062 → 1.0000 / 0.8750
//	  teen          1.0000 / 0.4000 → 1.0000 / 0.4444
//	duongVang       0.9505 / 0.9127 → 0.9505 / 0.9179
//	  co_dau        0.9405 / 0.8859 → 0.9405 / 0.8978
//	  khong_dau     1.0000 / 1.0000 → 1.0000 / 1.0000
//	  teen          0.8333 / 0.7369 → 0.8333 / 0.7354
//	duongManKhac    0.9565 / 0.7665 → 0.9565 / 0.7661
//	  co_dau        0.9333 / 0.7578 → 0.9333 / 0.7578
//	  khong_dau     1.0000 / 0.8873 → 1.0000 / 0.8873
//	  teen          0.9286 / 0.6293 → 0.9286 / 0.6280
//	duongTruyVan    1.0000 / 0.8942 → 1.0000 / 0.8942
//
// Recall@5 did not fall on any group; every MRR is within 0.01 of before
// (ADR-0047 §9). duongHaiNguoi has no miss left; «ko bik bo fieu o dau» and
// «bo fieu o dau v» went from rank 7 to 8, «đóng sổ hai người» from 2 to 1.
//
// The numbers of the ranking of 5c3a3c1 on the same sets, for the record:
// duongVang 0.9725 / 0.8560 (teen 0.8333 / 0.6694), duongManKhac 0.8514 /
// 0.3526 (teen 0.8214 / 0.2905).
// UI/UX upgrade B4 (2026-10-02): tai-chinh.md's step no longer points at a
// «chi theo nhóm» section (the finance screen has none), and the screen labels
// in _rut.json follow the money screens' new copy. Recall@5 unchanged on every
// set and group; duongVang MRR 0.9179 → 0.9177, [co_dau] 0.8978 → 0.8972.
// Rewording the step without «ở» moved «bo fieu o dau v» out of the top 5:
// that question sits one token's weight from the edge.
// UI/UX upgrade B6 (2026-10-02): the settings button of a two-person chat is
// named «Cài đặt cuộc trò chuyện» (it said «Cài đặt nhóm», QA UI-128), and
// to-giay.md quotes the new name. That is the very phrase of the hai-nguoi
// question «mở tờ giấy từ cài đặt cuộc trò chuyện», which moves up: duongHaiNguoi
// MRR 0.6859 → 0.7051, [co_dau] 0.6806 → 0.7222. Recall@5 and every other set
// unchanged. Adding «Chốt» to chat-nhom.md's labels moved four sets (duongTruyVan
// MRR 0.8942 → 0.8923): the poll's close button keeps the words the manual
// already quotes instead.
// Five-column tab strip (2026-10-02, merged onto B4–B7): the ranker did not
// change, the manual did. Cộng đồng became Khám phá's second section, so
// kham-pha.md and cong-dong.md say so in their overviews and in the existing
// steps (no new section: a first try added three, and «ở đầu màn» folds to the
// «o dau» of «ở đâu» questions; recall@5 fell 0.9505 → 0.9396). Recall@5 did
// not move on any group; MRR against main before the merge:
//
//	duongVang       0.9177 → 0.9179   teen 0.7354 → 0.7369
//	duongManKhac    0.7661 → 0.7665   teen 0.6280 → 0.6293
//	duongTruyVan    0.8942 (unchanged), duongHaiNguoi unchanged
//
// Production, no demo story (2026-10-03): the ranker did not change, the
// manual did. ca-nhan.md described the demo's profile editor («Bio», «Xong»),
// which is gone; it now names the real one (`HoSoSong`: «Tên», «Giới thiệu»,
// «Thành phố», «Lưu hồ sơ»). Recall@5 did not move on any group. MRR:
//
//	duongVang       0.9179 → 0.9124   khong_dau 1.0000 → 0.9865
//	duongTruyVan    0.8942 → 0.8878
//	duongManKhac    0.7665 → 0.7647   khong_dau 0.8873 → 0.8922, teen 0.6293 → 0.6173
//
// Two groups fall by more than 0.01 (khong_dau 0.0135, teen 0.0120). Every
// accurate wording tried moved them the same way; only manuals naming labels
// the app does not have («Xong», «Bio») kept the old numbers, and a manual
// that sends a person to a button that is not there is the worse trade.
// «log out o dau» is back among duongVang's misses.
var vangGhim = map[string]map[string][2]string{
	duongVang: {
		"":          {"0.9505", "0.9124"},
		"co_dau":    {"0.9405", "0.8972"},
		"khong_dau": {"1.0000", "0.9865"},
		"teen":      {"0.8333", "0.7369"},
	},
	duongTruyVan: {
		"": {"1.0000", "0.8878"},
	},
	duongManKhac: {
		"":          {"0.9565", "0.7647"},
		"co_dau":    {"0.9333", "0.7578"},
		"khong_dau": {"1.0000", "0.8922"},
		"teen":      {"0.9286", "0.6173"},
	},
	duongHaiNguoi: {
		"":          {"1.0000", "0.7051"},
		"co_dau":    {"1.0000", "0.7222"},
		"khong_dau": {"1.0000", "0.8750"},
		"teen":      {"1.0000", "0.4444"},
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
