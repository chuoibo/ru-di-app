package huongdan

import (
	"context"
	"strings"

	"mobile/services/core/internal/rag/xephang"
)

// MaxRuneCau is how much of a question Tim reads: the first MaxRuneCau runes,
// the rest ignored. It is the bound the Nếp handler already puts on a question
// (maxHoiNep in chatassist), so no question that reaches Tim through Nếp is
// cut; it is here so that Tim itself cannot be made to fold and rank a
// megabyte (a 1 MiB question took 191 ms before this, against the 50 ms
// bound of design 05 §3).
const MaxRuneCau = 2000

// catCau returns the first MaxRuneCau runes of cau. An invalid byte counts as
// one rune, as range over a string reads it.
func catCau(cau string) string {
	n := 0
	for i := range cau {
		if n == MaxRuneCau {
			return cau[:i]
		}
		n++
	}
	return cau
}

// tyLeGhim decides which sections of the screen the person stands on go
// first: those whose score is at least this share of the best matching score.
// Pinning every section that matched at all put a section sharing one common
// word («nhóm», «xem») ahead of the answer on another screen: on questions
// asked from a screen that holds no answer (testdata/truy-hoi-man-khac.json)
// MRR was 0.3526. One half was fixed before it was measured, as the one
// alternative tried (order and numbers in the commit that set it).
const tyLeGhim = 0.5

// tim ranks the manual's passages for h: BM25 over every syllable of the
// query (folded the way rag/xephang folds: lower case, no diacritics), with
// no word list deciding anything. The query is the one the MODEL wrote (the
// router's truy_van, or the search_app_manual argument): teencode,
// abbreviations and English are the model's to rewrite into the manual's
// words, never a Go table's (docs/architecture/03-ai-engine-hop-dong.md §8).
// A passage is a candidate when it shares any term with the query; the
// ranking alone decides its place, and the sections of the screen the
// person stands on go first when their score is at least tyLeGhim of the
// best (a structural pin by screen id, not by word).
func (s *SoTay) tim(ctx context.Context, h Hoi) []Doan {
	if ctx.Err() != nil {
		return nil
	}
	k := h.K
	if k <= 0 {
		k = KMacDinh
	}
	amTiet := xephang.AmTiet(catCau(h.Cau))
	if len(amTiet) == 0 {
		return nil
	}
	man := s.chuanMan(h.Man)
	var ghim, con []int
	cao := -1.0
	for _, kq := range s.chiMuc.Tim(strings.Join(amTiet, " "), s.chiMuc.Len()) {
		i := s.theoID[kq.ID]
		if kq.Diem <= 0 {
			continue
		}
		if cao < 0 {
			cao = kq.Diem
		}
		if man != "" && s.doan[i].Man == man && kq.Diem >= tyLeGhim*cao {
			ghim = append(ghim, i)
		} else {
			con = append(con, i)
		}
	}
	thuTu := append(ghim, con...)
	if len(thuTu) > k {
		thuTu = thuTu[:k]
	}
	out := make([]Doan, len(thuTu))
	for j, i := range thuTu {
		out[j] = s.doan[i].sao()
	}
	return out
}

func (s *SoTay) theoMan(man string) []Doan {
	t, ok := s.trangCua[s.chuanMan(man)]
	if !ok {
		return nil
	}
	out := make([]Doan, len(t.doan))
	for i, d := range t.doan {
		out[i] = d.sao()
	}
	return out
}

// sao copies a section so a caller that edits a slice cannot edit the manual
// every other goroutine reads.
func (d Doan) sao() Doan {
	d.Buoc = append([]string(nil), d.Buoc...)
	d.Nhan = append([]string(nil), d.Nhan...)
	return d
}
