package rag

import (
	"sort"

	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/tuvung"
)

// rangBuoc is a YeuCau's hard filters, prepared once: the allergen set closed
// over families, the diets with what they imply.
type rangBuoc struct {
	diemDen  string
	diUng    map[string]bool
	anKieng  []string
	nganSach *int64
	luc      *int
	khung    *[2]int
}

func rangBuocCua(y YeuCau) rangBuoc {
	r := rangBuoc{diemDen: y.DiemDen, diUng: map[string]bool{}, anKieng: tuvung.AnKieng.LocHopLe(y.AnKieng),
		nganSach: y.NganSach, luc: y.Luc, khung: y.Khung}
	for _, id := range tuvung.MoRongDiUng(y.DiUng) {
		r.diUng[id] = true
	}
	return r
}

// diUngSQL is the closed allergen set as the SQL filter's array.
func (r rangBuoc) diUngSQL() []string {
	out := make([]string, 0, len(r.diUng))
	for id := range r.diUng {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// thamSo are the time and budget filters as SQL parameters, nil for "not
// asked".
func (r rangBuoc) thamSo() (luc, khung, ngan any) {
	if r.luc != nil {
		luc = *r.luc
	}
	if r.khung != nil {
		khung = giomo.KhungSQL(r.khung[0], r.khung[1])
	}
	if r.nganSach != nil {
		ngan = *r.nganSach
	}
	return luc, khung, ngan
}

// dat checks one live profile against the hard filters, the second check of
// design 04 §4a. It reports whether the place may be shown and, when it may,
// which of its facts are unknown. The index's SQL filters are the first
// check; this one runs on the live row every hit is hydrated from, so a stale
// index can lose a place but never show one that breaks a filter.
func (r rangBuoc) dat(h HoSo) (bool, []string) {
	if r.diemDen != "" && h.DiemDen != r.diemDen {
		return false, nil
	}
	for _, a := range h.DiUng {
		if r.diUng[a] {
			return false, nil
		}
	}
	have := map[string]bool{}
	for _, d := range h.AnKieng {
		have[d] = true
	}
	for _, d := range r.anKieng {
		if !have[d] {
			return false, nil
		}
	}
	var co []string
	if r.luc != nil || r.khung != nil {
		switch {
		case h.Lich == nil:
			co = append(co, CoGioChuaRo)
		case r.luc != nil && !h.Lich.MoLuc(*r.luc):
			return false, nil
		case r.khung != nil && !h.Lich.MoTrong(r.khung[0], r.khung[1]):
			return false, nil
		}
	}
	if r.nganSach != nil {
		switch {
		case h.GiaMin == nil:
			co = append(co, CoGiaChuaRo)
		case *h.GiaMin > *r.nganSach:
			return false, nil
		}
	}
	return true, co
}

// xepChuaRo orders hits so every place with an unknown comes after every
// place without, keeping each group's order; and when the asker named a time,
// drops the places whose hours are unknown once at least three places are
// known to be open (design 04 §4a).
func (r rangBuoc) xepChuaRo(hits []Hit) []Hit {
	var ro, chuaRo []Hit
	for _, h := range hits {
		if len(h.Co) == 0 {
			ro = append(ro, h)
		} else {
			chuaRo = append(chuaRo, h)
		}
	}
	if r.luc != nil || r.khung != nil {
		biet := 0
		for _, h := range hits {
			if !coCo(h.Co, CoGioChuaRo) {
				biet++
			}
		}
		if biet >= gioToiThieu {
			kept := chuaRo[:0]
			for _, h := range chuaRo {
				if !coCo(h.Co, CoGioChuaRo) {
					kept = append(kept, h)
				}
			}
			chuaRo = kept
		}
	}
	return append(ro, chuaRo...)
}

func coCo(co []string, c string) bool {
	for _, x := range co {
		if x == c {
			return true
		}
	}
	return false
}

// coHits are the query-level flags the returned hits raise.
func coHits(hits []Hit) []string {
	var out []string
	for _, c := range []string{CoGioChuaRo, CoGiaChuaRo} {
		for _, h := range hits {
			if coCo(h.Co, c) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}
