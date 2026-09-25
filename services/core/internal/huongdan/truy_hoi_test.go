package huongdan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/promptsafety"
)

// The golden set: natural questions a person types into Nếp, each with the
// section(s) that answer it and the screen they are standing on. Written
// before any ranking code existed; the questions are not edited to fit the
// ranker (see ghi_chu in the file).
const duongVang = "testdata/truy-hoi-so-tay.json"

type cauVang struct {
	Hoi  string   `json:"hoi"`
	Man  string   `json:"man"`
	Dung []string `json:"dung"`
	Go   string   `json:"go"`
}

func docVang(t testing.TB) []cauVang {
	t.Helper()
	raw, err := os.ReadFile(duongVang)
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

// ketQuaVang is what one run of the golden set measured.
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

func TestBoVangHopLe(t *testing.T) {
	cau := docVang(t)
	if len(cau) < 75 || len(cau) > 100 {
		t.Fatalf("bộ vàng có %d câu, cần khoảng 80", len(cau))
	}
	khongDau := 0
	seen := map[string]bool{}
	for _, c := range cau {
		if seen[c.Hoi] {
			t.Errorf("câu trùng: %q", c.Hoi)
		}
		seen[c.Hoi] = true
		if soTay.chuanMan(c.Man) != c.Man {
			t.Errorf("%q: màn %q không có trong _rut.json", c.Hoi, c.Man)
		}
		if len(c.Dung) == 0 {
			t.Errorf("%q: không có mục đúng", c.Hoi)
		}
		for _, id := range c.Dung {
			if _, ok := soTay.theoID[id]; !ok {
				t.Errorf("%q: mục %q không có trong sổ tay", c.Hoi, id)
			}
		}
		// The declared typing style must be what the text is: a question
		// «without diacritics» has none, one «with» has some.
		coDau := promptsafety.Fold(c.Hoi) != strings.ToLower(c.Hoi)
		switch c.Go {
		case "co_dau":
			if !coDau {
				t.Errorf("%q khai co_dau mà không có dấu", c.Hoi)
			}
		case "khong_dau", "teen":
			if coDau {
				t.Errorf("%q khai %s mà có dấu", c.Hoi, c.Go)
			}
			khongDau++
		default:
			t.Errorf("%q: go %q lạ", c.Hoi, c.Go)
		}
	}
	if 2*khongDau < len(cau) {
		t.Errorf("chỉ %d/%d câu gõ không dấu hoặc teencode, cần ít nhất một nửa", khongDau, len(cau))
	}
}

// The measured quality of Tim on the golden set, pinned to four decimals: a
// ranking change that moves one question by one rank moves one of these, and
// then the change has to say so here. recall@5 must also stay at or above the
// bar of design 05 §3.
//
// Measured when pinned (91 questions, 49 without diacritics or in teencode):
// three questions miss. «checkin o dau» and «log out o dau» share no term with
// the manual («check-in», «đăng xuất»): no lexical ranker bridges that, the
// vector stage of slice 16 is where it can. «thêm quán này vào buổi đi chơi
// của nhóm» on places/[id] finds one of its two sections first; the other,
// chon-keo/chon-keo-cho-dia-diem (the screen «Thêm vào kèo» opens), is not in
// the top 10 with or without pinning: it shares only «thêm vào» with the
// question, and «buổi đi», «đi chơi», «nhóm» weigh more elsewhere.
const (
	recall5Vang  = "0.9725"
	mrrVang      = "0.8560"
	nguongRecall = 0.90
)

func TestTruyHoiVang(t *testing.T) {
	r := doVang(soTay, docVang(t))
	for _, x := range r.truot {
		t.Log("miss:", x)
	}
	if got := fmt.Sprintf("%.4f", r.recall5); got != recall5Vang {
		t.Errorf("recall@5 = %s, pinned %s", got, recall5Vang)
	}
	if got := fmt.Sprintf("%.4f", r.mrr); got != mrrVang {
		t.Errorf("MRR = %s, pinned %s", got, mrrVang)
	}
	if r.recall5 < nguongRecall {
		t.Errorf("recall@5 %.4f is under the bar %.2f", r.recall5, nguongRecall)
	}
}
