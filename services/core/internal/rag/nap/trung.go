package nap

import (
	"math"
	"sort"
	"unicode/utf8"
)

// Dedupe (S6). Two places are one when, in the same destination, they lie
// within cfg.Trung.KhoangCachM metres of each other AND their profile
// embeddings are at least cfg.Trung.CosineToiThieu apart in cosine. No
// string rule compares names: the embedding space and the map decide
// (owner decision, 2026-09-25). A duplicate leaves the index; `places` is
// never touched (only the Python seed/import writes it).

// UngVienTrung is one place as dedupe reads it.
type UngVienTrung struct {
	ID      string
	DiemDen string
	Lat     float64
	Lng     float64
	// CoToaDo false: no coordinates, never merged by distance.
	CoToaDo bool
	Nguon   string
	// Giau is how much the place says (runes of its profile and reviews):
	// the richer row wins among equals.
	Giau  int
	Dense []float32
}

// HaversineM is the great-circle distance in metres.
func HaversineM(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371008.8
	rad := math.Pi / 180
	dlat := (lat2 - lat1) * rad
	dlng := (lng2 - lng1) * rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dlng/2)*math.Sin(dlng/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

func uuTienNguon(n string) int {
	switch n {
	case "curated":
		return 3
	case "seed":
		return 2
	case "osm":
		return 1
	}
	return 0
}

// TimTrung returns duplicate id → canonical id. Components are closed
// transitively; the canonical place of a component is curated before seed
// before osm, then the richer, then the smaller id.
func TimTrung(ds []UngVienTrung, cosMin, metMax float64) map[string]string {
	parent := make([]int, len(ds))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	byDest := map[string][]int{}
	for i, d := range ds {
		byDest[d.DiemDen] = append(byDest[d.DiemDen], i)
	}
	for _, idx := range byDest {
		for a := 0; a < len(idx); a++ {
			for b := a + 1; b < len(idx); b++ {
				x, y := ds[idx[a]], ds[idx[b]]
				if !x.CoToaDo || !y.CoToaDo || HaversineM(x.Lat, x.Lng, y.Lat, y.Lng) > metMax {
					continue
				}
				if len(x.Dense) == 0 || len(x.Dense) != len(y.Dense) || cosine(x.Dense, y.Dense) < cosMin {
					continue
				}
				ra, rb := find(idx[a]), find(idx[b])
				if ra != rb {
					parent[ra] = rb
				}
			}
		}
	}
	groups := map[int][]int{}
	for i := range ds {
		groups[find(i)] = append(groups[find(i)], i)
	}
	out := map[string]string{}
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		sort.Slice(g, func(i, j int) bool {
			a, b := ds[g[i]], ds[g[j]]
			if uuTienNguon(a.Nguon) != uuTienNguon(b.Nguon) {
				return uuTienNguon(a.Nguon) > uuTienNguon(b.Nguon)
			}
			if a.Giau != b.Giau {
				return a.Giau > b.Giau
			}
			return a.ID < b.ID
		})
		for _, i := range g[1:] {
			out[ds[i].ID] = ds[g[0]].ID
		}
	}
	return out
}

func giau(h HoSoQuan) int { return utf8.RuneCountInString(h.HoSo) + utf8.RuneCountInString(h.DanhGia) }
