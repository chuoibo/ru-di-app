package huongdan

import (
	"context"
	"strings"

	"mobile/services/core/internal/rag/xephang"
)

// tuDem are the syllables, already folded the way rag/xephang folds, that
// carry no task on their own: the words a question is built from rather than
// the words it is about («làm sao», «ở đâu», «thì», «được không»), plus the
// teencode spellings of the same («ko», «dc», «j», «z»).
//
// They change only one thing: whether a passage counts as matching at all. A
// passage must share at least one term with the question that is not made of
// these alone (a syllable not in the list, or a syllable pair with at least
// one half not in it). Ranking still sees every term; BM25 already gives a
// word that is everywhere almost no weight. Without this rule «bấm» and
// «thì» make every passage of the current screen «match», and pinning would
// bury the one passage that answers behind all of them.
//
// A folded syllable can stand for several words («de» is «để» and «đề»). The
// list keeps a syllable only when the content words that fold to it are still
// found through a pair («đề nghị» → «de_nghi») or a neighbour.
var tuDem = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range []string{
		// Question scaffolding.
		"lam", "sao", "the", "nao", "thi", "o", "dau", "cho", "de", "muon", "kieu", "nhu",
		"gi", "khi", "duoc", "khong", "co", "roi", "vay", "nay", "cua", "la", "ma",
		"voi", "va", "hay", "hoac", "mot", "nhung", "cac", "bam", "minh", "ban", "toi",
		"a", "oi", "nhi", "nhe", "ne", "ha", "ah", "ak",
		// Teencode for the same words.
		"ko", "k", "kh", "hok", "hong", "hk", "dc", "j", "z", "v", "vs", "r", "ntn",
		"mk", "mik", "bik", "e",
	} {
		m[t] = true
	}
	return m
}()

// thuatNoiDung are the terms of q that decide whether a passage matches: every
// rag/xephang term except a stop syllable and a pair of two stop syllables.
func thuatNoiDung(q string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range xephang.Thuat(q) {
		if seen[t] {
			continue
		}
		seen[t] = true
		if a, b, laCap := strings.Cut(t, "_"); laCap {
			if tuDem[a] && tuDem[b] {
				continue
			}
		} else if tuDem[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *SoTay) khop(i int, noiDung []string) bool {
	for _, t := range noiDung {
		if s.thuat[i][t] {
			return true
		}
	}
	return false
}

func (s *SoTay) tim(ctx context.Context, h Hoi) []Doan {
	if ctx.Err() != nil {
		return nil
	}
	k := h.K
	if k <= 0 {
		k = KMacDinh
	}
	noiDung := thuatNoiDung(h.Cau)
	if len(noiDung) == 0 {
		return nil
	}
	man := s.chuanMan(h.Man)
	var ghim, con []int
	for _, kq := range s.chiMuc.Tim(h.Cau, s.chiMuc.Len()) {
		i := s.theoID[kq.ID]
		if !s.khop(i, noiDung) {
			continue
		}
		if man != "" && s.doan[i].Man == man {
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
