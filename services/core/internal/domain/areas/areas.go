// Package areas is app/places/areas.py's catalogue: the districts a group may
// name as where it hangs out, with the centroid every meeting distance is
// measured from. Like Python's, the centroids are approximate demo points, not
// survey coordinates.
package areas

import "slices"

// Area is one entry of AREAS.
type Area struct {
	ID    string
	Label string
	Lat   float64
	Lng   float64
}

// catalogue is AREAS in declaration order, which is also the order GET /areas
// answers in. The coordinates are copied as Python writes them, so each parses
// to the same float64.
var catalogue = [...]Area{
	{ID: "da-lat", Label: "Đà Lạt", Lat: 11.9429, Lng: 108.4428},
	{ID: "hcm-quan-1", Label: "Quận 1, TP.HCM", Lat: 10.7769, Lng: 106.7009},
	{ID: "hcm-quan-3", Label: "Quận 3, TP.HCM", Lat: 10.7840, Lng: 106.6870},
	{ID: "hcm-quan-4", Label: "Quận 4, TP.HCM", Lat: 10.7590, Lng: 106.7050},
	{ID: "hcm-phu-nhuan", Label: "Phú Nhuận, TP.HCM", Lat: 10.7990, Lng: 106.6800},
	{ID: "hcm-quan-7", Label: "Quận 7, TP.HCM", Lat: 10.7340, Lng: 106.7220},
	{ID: "hcm-binh-thanh", Label: "Bình Thạnh, TP.HCM", Lat: 10.8040, Lng: 106.7100},
	{ID: "hcm-thu-duc", Label: "Thủ Đức, TP.HCM", Lat: 10.8500, Lng: 106.7550},
}

// All returns AREAS in declaration order.
func All() []Area { return slices.Clone(catalogue[:]) }

// Find is find_area: the area with exactly this id.
func Find(id string) (Area, bool) {
	for _, area := range catalogue {
		if area.ID == id {
			return area, true
		}
	}
	return Area{}, false
}
