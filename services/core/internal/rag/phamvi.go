package rag

import (
	"context"
	"sort"

	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/repo"
)

// A catalogue holds two kinds of destination side by side: the curated ones
// (d-da-lat, d-tphcm, ...: a city with a tight box around the part people
// mean) and one per province that `rudi-ingest migrate` seeds (d-tinh-N,
// ingest.ProvinceBoxes), which is where every ingested place is filed
// (ingest.Apply). A place in Đà Lạt may therefore sit under d-da-lat or
// under d-tinh-68. Everything here relates the two by geography only -- the
// province ids ingest owns, the boxes and centres in the destinations table,
// a place's own coordinates -- never by reading a name.

var tinhIDs = func() map[string]bool {
	out := make(map[string]bool, len(ingest.ProvinceBoxes))
	for _, p := range ingest.ProvinceBoxes {
		out[ingest.ProvinceDestinationID(p.Code)] = true
	}
	return out
}()

// LaTinh reports whether a destination id is a province destination seeded
// by the ingest (ingest.ProvinceDestinationID).
func LaTinh(id string) bool { return tinhIDs[id] }

// Hop is a bounding box, edges inclusive.
type Hop struct{ Nam, Tay, Bac, Dong float64 }

func (h Hop) chua(lat, lng float64) bool {
	return h.Nam <= lat && lat <= h.Bac && h.Tay <= lng && lng <= h.Dong
}

func (h Hop) cat(o Hop) bool {
	return h.Nam <= o.Bac && o.Nam <= h.Bac && h.Tay <= o.Dong && o.Tay <= h.Dong
}

func hopCua(d DiemDen) Hop { return Hop{Nam: d.Nam, Tay: d.Tay, Bac: d.Bac, Dong: d.Dong} }

// TinhChua is the province destination a point lies in: among the province
// destinations whose box holds it, the one whose centre is nearest (ties by
// id); "" when no province box holds it. Province boxes overlap -- Khánh
// Hòa's reaches Đà Lạt because it holds Trường Sa (ingest.ProvinceBox) -- so
// the box alone cannot answer, and the nearest centre settles it.
func TinhChua(dests []DiemDen, lat, lng float64) string {
	best, bestD := "", 0.0
	for _, d := range dests {
		if !LaTinh(d.ID) || !hopCua(d).chua(lat, lng) {
			continue
		}
		dl, dg := d.Lat-lat, d.Lng-lng
		dist := dl*dl + dg*dg
		if best == "" || dist < bestD || (dist == bestD && d.ID < best) {
			best, bestD = d.ID, dist
		}
	}
	return best
}

// TinhCua is the province destination a curated destination lies in
// (TinhChua of its centre); "" for a province destination itself or when
// no province destination holds it.
func TinhCua(dests []DiemDen, id string) string {
	if LaTinh(id) {
		return ""
	}
	for _, d := range dests {
		if d.ID == id {
			return TinhChua(dests, d.Lat, d.Lng)
		}
	}
	return ""
}

// PhamVi is where the places of one destination are: every place filed
// under one of Tron, and the places filed under one of Tinh whose
// coordinates lie in Hop. The zero value is every destination.
type PhamVi struct {
	Tron []string
	Tinh []string
	Hop  *Hop
}

// PhamViCua is the scope of destination id among dests:
//   - a curated destination: its own places, and the places of every province
//     destination whose box meets its box that lie inside its box (Đà Lạt
//     takes the d-tinh-68 rows inside Đà Lạt's box, not the rest of Lâm
//     Đồng);
//   - a province destination: its own places and every place of each curated
//     destination lying in it (TinhCua), so «Lâm Đồng» keeps Đà Lạt;
//   - an id not in dests: that id alone, which matches nothing unknown.
//
// "" is the zero PhamVi: no destination filter.
func PhamViCua(dests []DiemDen, id string) PhamVi {
	if id == "" {
		return PhamVi{}
	}
	pv := PhamVi{Tron: []string{id}}
	var self *DiemDen
	for i := range dests {
		if dests[i].ID == id {
			self = &dests[i]
		}
	}
	if self == nil {
		return pv
	}
	if LaTinh(id) {
		for _, d := range dests {
			if !LaTinh(d.ID) && TinhCua(dests, d.ID) == id {
				pv.Tron = append(pv.Tron, d.ID)
			}
		}
		sort.Strings(pv.Tron[1:])
		return pv
	}
	h := hopCua(*self)
	for _, d := range dests {
		if LaTinh(d.ID) && hopCua(d).cat(h) {
			pv.Tinh = append(pv.Tinh, d.ID)
		}
	}
	sort.Strings(pv.Tinh)
	if len(pv.Tinh) > 0 {
		pv.Hop = &h
	}
	return pv
}

// Chua reports whether a place filed under diemDen, at (lat, lng) when
// coToaDo, is in the scope. A place with no coordinates is never taken from
// a province by the box.
func (pv PhamVi) Chua(diemDen string, lat, lng float64, coToaDo bool) bool {
	if len(pv.Tron) == 0 {
		return true
	}
	for _, id := range pv.Tron {
		if id == diemDen {
			return true
		}
	}
	if pv.Hop == nil || !coToaDo {
		return false
	}
	for _, id := range pv.Tinh {
		if id == diemDen {
			return pv.Hop.chua(lat, lng)
		}
	}
	return false
}

// hopSQL is the box as the SQL filter's float8[4] (south, west, north,
// east), nil for none.
func (pv PhamVi) hopSQL() []float64 {
	if pv.Hop == nil {
		return nil
	}
	return []float64{pv.Hop.Nam, pv.Hop.Tay, pv.Hop.Bac, pv.Hop.Dong}
}

// phamViTai reads the destinations and scopes id among them; "" reads
// nothing.
func phamViTai(ctx context.Context, q Querier, id string) (PhamVi, error) {
	if id == "" {
		return PhamVi{}, nil
	}
	rows, err := repo.Repository{Q: q}.ListDestinations(ctx)
	if err != nil {
		return PhamVi{}, err
	}
	return PhamViCua(DiemDenTuRepo(rows), id), nil
}
