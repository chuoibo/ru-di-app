package rag

import (
	"strings"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/rag/xephang"
	"mobile/services/core/internal/repo"
)

// Querier is a pool or a transaction.
type Querier = repo.Querier

// YeuCau is one retrieval: the checked slots of a question plus the asker's
// own words. Nothing a model wrote reaches it as free text (design 04 §5.1).
type YeuCau struct {
	// DiemDen is the resolved destination; "" searches every destination.
	// The engine asks back instead of passing ""; the public search, which
	// cannot ask, passes it when the words name none.
	DiemDen string
	// Cau is the asker's own words (the routing copy). Only the lexical
	// lists read it, folded.
	Cau string
	// Bo are spans a slot already took (ResolveDestination's Cum), as folded
	// syllables: they are dropped from the lexical terms.
	Bo [][]string

	// Hard filters. Never relaxed, never widened by a retry.
	//
	// DiUng are the allergens the asker named (tuvung.DiUng ids); a place
	// whose own words mention any of them, or any of their family, is out.
	DiUng []string
	// AnKieng are the diets a place must declare in its name, kinds or traits.
	AnKieng []string
	// NganSach is the ceiling per person in whole đồng: a place whose lowest
	// price is above it is out; one with no price stays, flagged and last.
	NganSach *int64
	// Luc is a minute of the week (giomo) the place must be open at; Khung is
	// a window [tu, den) it must be open at some point of. A place with no
	// readable hours stays, flagged and last, and only while fewer than
	// three places are known to be open.
	Luc   *int
	Khung *[2]int

	// Soft preferences: their own ranked list, never a filter.
	LoaiCho []string
	KhiChat []string

	// K is how many places to return; 0 is 20.
	K int
}

// Hit is one place Retrieve returns, best first.
type Hit struct {
	ID string
	// Diem is the RRF score, an integer so a golden is stable to the unit.
	Diem int64
	// Co are this place's unknowns: gio_chua_ro, gia_chua_ro.
	Co []string
}

// KetQua is Retrieve's answer.
type KetQua struct {
	Quan []Hit
	// PhienBan is the index version that answered; 0 when live rows did.
	PhienBan int64
	// Degraded: no active version (or the index failed), answered from live
	// `places` rows of the destination with the same hard filters.
	Degraded bool
	// NLoc is how many places passed the hard filters; NUngVien how many
	// ranked candidates were checked a second time against live rows.
	NLoc, NUngVien int
	// Co are the query-level flags of rag_query_log: khong_dau, gio_chua_ro,
	// gia_chua_ro.
	Co []string
}

func (y YeuCau) k() int {
	if y.K <= 0 {
		return 20
	}
	return y.K
}

// Flags a hit or a query may carry.
const (
	CoKhongDau    = "khong_dau"
	CoGioChuaRo   = "gio_chua_ro"
	CoGiaChuaRo   = "gia_chua_ro"
	CoNhieuDiem   = "nhan_nhieu_diem_den"
	gioToiThieu   = 3
	danhSachToiDa = 50
)

// DocCau reads what a question's words alone can say, with no model: the one
// destination they name (never a default; see ResolveDestination), the
// allergens named beside a trigger (tuvung.DiUngNguoiHoi), the diets, the
// categories and the atmospheres. It is what the public search uses, and the
// deterministic half of what the engine's preprocess hands Retrieve beside
// Understand's slots.
func DocCau(cau string, dests []DiemDen) (YeuCau, DiemDenGiai) {
	dd := ResolveDestination(dests, GoiY{Cau: cau})
	y := YeuCau{
		DiemDen: dd.ID,
		Cau:     cau,
		Bo:      dd.Cum,
		DiUng:   tuvung.DiUngNguoiHoi(cau),
		AnKieng: tuvung.AnKieng.QuetKhongPhuDinh(cau),
		LoaiCho: tuvung.LoaiCho.Quet(cau),
		KhiChat: tuvung.KhiChat.Quet(cau),
	}
	return y, dd
}

// gapTruyVan is the one place a question is folded for retrieval: every
// lexical list reads its output, never the raw words.
func gapTruyVan(cau string) (syllables []string, folded string) {
	return xephang.AmTiet(cau), promptsafety.Fold(cau)
}

// thuatTruyVan is the lexical terms of a question: xephang's terms (folded
// syllables and neighbouring pairs), without stop syllables, without the
// spans a slot took, and without any pair one of whose halves was dropped.
// Order follows xephang.Thuat; each term once.
func thuatTruyVan(y YeuCau) []string {
	s, _ := gapTruyVan(y.Cau)
	keep := make([]bool, len(s))
	for i, w := range s {
		keep[i] = !tuvung.LaTuDung(w)
	}
	for _, span := range y.Bo {
		for _, at := range tuvung.ViTri(s, span) {
			for j := range span {
				keep[at+j] = false
			}
		}
	}
	var out []string
	seen := map[string]bool{}
	add := func(t string) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for i, w := range s {
		if keep[i] {
			add(w)
		}
	}
	for i := 1; i < len(s); i++ {
		if keep[i-1] && keep[i] {
			add(s[i-1] + "_" + s[i])
		}
	}
	return out
}

// tsQuery renders terms as a to_tsquery('simple', …) OR-query. A pair's '_'
// becomes NoiCap, the spelling rag_chunks.tsv indexes it under. Terms hold
// only letters, digits and '_' (tuvung.AmTiet), so quoting each is enough.
func tsQuery(terms []string) string {
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.ReplaceAll(t, "_", NoiCap)
		if t == "" || strings.ContainsAny(t, `'\`) {
			continue
		}
		parts = append(parts, "'"+t+"'")
	}
	return strings.Join(parts, " | ")
}

// coTruyVan are the query-level flags the words alone decide.
func coTruyVan(y YeuCau) []string {
	if tuvung.KhongDau(y.Cau) {
		return []string{CoKhongDau}
	}
	return nil
}
