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

// tuDem are the syllables, already folded the way rag/xephang folds, that
// carry no task on their own: the words a question is built from rather than
// the words it is about («làm sao», «ở đâu», «thì», «được không»), plus the
// teencode spellings of the same («ko», «dc», «j», «z»).
//
// They change only one thing: whether a passage counts as matching at all. A
// passage must share at least one syllable with the question that is not in
// this list. (Only syllables are compared: a passage holding a pair of
// syllables holds each syllable too, so a pair with a content half never
// decides anything its content syllable does not.) Ranking still sees every
// term; BM25 already gives a word that is everywhere almost no weight.
// Without this rule «bấm» and «thì» make every passage of the current screen
// «match», and pinning would bury the one passage that answers behind all of
// them.
//
// A folded syllable can stand for several words («de» is «để» and «đề»). The
// list keeps a syllable only when the content words that fold to it are still
// found through a neighbour («đề nghị» → «nghi»).
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

// teen maps teencode and chat abbreviations, folded, to the folded words they
// stand for. Written from common usage, not from the misses of the golden
// sets (the order is in the commit that added it); no translations: «add»,
// «mem», «gr», «checkin», «log out» are English, and bridging them is a
// reviewed synonym table's job or the vector stage's (slice 16).
//
// A question syllable is rewritten only when the manual does not use it (see
// chuanHoi), so a real word of the app is never taken for teencode: «o» stays
// «ở», and «k» would stay if a manual ever wrote a lone «k».
var teen = map[string]string{
	"ko": "khong", "k": "khong", "kh": "khong", "hok": "khong", "hong": "khong", "hk": "khong", "kg": "khong",
	"dc": "duoc", "dk": "duoc",
	"j": "gi",
	"z": "vay", "v": "vay",
	"r":   "roi",
	"vs":  "voi",
	"ntn": "nhu the nao",
	"bik": "biet", "bit": "biet", "bjk": "biet",
	"mk": "minh", "mik": "minh",
	"mn": "moi nguoi", "mng": "moi nguoi",
	"ny":  "nguoi yeu",
	"sdt": "so dien thoai", "dt": "dien thoai",
	"tn":  "tin nhan",
	"bh":  "bay gio",
	"cx":  "cung",
	"trc": "truoc",
	"ae":  "anh em",
	"cf":  "ca phe",
	"lm":  "lam",
}

// tyLeGhim decides which sections of the screen the person stands on go
// first: those whose score is at least this share of the best matching score.
// Pinning every section that matched at all put a section sharing one common
// word («nhóm», «xem») ahead of the answer on another screen: on questions
// asked from a screen that holds no answer (testdata/truy-hoi-man-khac.json)
// MRR was 0.3526. One half was fixed before it was measured, as the one
// alternative tried (order and numbers in the commit that set it).
const tyLeGhim = 0.5

// chuanHoi rewrites the teencode syllables of a folded question into the
// words they stand for. A syllable the manual uses is left alone. One the
// manual does not use is looked up in teen; failing that, the two spelling
// habits of teencode are undone when the result is a syllable the manual
// uses: «f» for «ph» («fieu» → «phieu») and «w» for «qu» («wan» → «quan»).
func (s *SoTay) chuanHoi(amTiet []string) []string {
	out := make([]string, 0, len(amTiet))
	for _, a := range amTiet {
		if s.tuVung[a] {
			out = append(out, a)
			continue
		}
		if t, ok := teen[a]; ok {
			out = append(out, strings.Fields(t)...)
			continue
		}
		switch {
		case strings.HasPrefix(a, "f") && s.tuVung["ph"+a[1:]]:
			a = "ph" + a[1:]
		case strings.HasPrefix(a, "w") && s.tuVung["qu"+a[1:]]:
			a = "qu" + a[1:]
		}
		out = append(out, a)
	}
	return out
}

// thuatNoiDung are the syllables of a question that decide whether a passage
// matches: each distinct one not in tuDem.
func thuatNoiDung(amTiet []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range amTiet {
		if seen[a] || tuDem[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
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
	amTiet := s.chuanHoi(xephang.AmTiet(catCau(h.Cau)))
	noiDung := thuatNoiDung(amTiet)
	if len(noiDung) == 0 {
		return nil
	}
	man := s.chuanMan(h.Man)
	var ghim, con []int
	cao := -1.0
	for _, kq := range s.chiMuc.Tim(strings.Join(amTiet, " "), s.chiMuc.Len()) {
		i := s.theoID[kq.ID]
		if !s.khop(i, noiDung) {
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
