// Package socialmap is app/places/social_map.py: what a group's own
// check-ins add up to on the F43 map and the F44 heatmap, and nothing more.
//
// The input types carry only the fields the Python functions read. A
// check-in's author, time and caption cannot reach an answer because they
// cannot reach these functions at all.
package socialmap

import (
	"slices"

	"mobile/services/core/internal/domain/areas"
)

// PrivateCheckinFields is PRIVATE_CHECKIN_FIELDS.
func PrivateCheckinFields() []string {
	return []string{"author_id", "created_at", "caption"}
}

// Checkin is the row _scan_checkins builds from a memory record. Nil is
// Python's None.
type Checkin struct {
	PlaceID   *string
	PlaceName *string
	Lat       *float64
	Lng       *float64
}

// VisitedPlace is one visited_layer row.
type VisitedPlace struct {
	PlaceID    string
	PlaceName  string
	Lat        float64
	Lng        float64
	VisitCount int
}

// VisitedLayer is visited_layer: one row per place id, keeping the name and
// coordinate of the first check-in seen for it, ordered by count descending
// then place id.
func VisitedLayer(checkins []Checkin) []VisitedPlace {
	index := map[string]int{}
	var rows []VisitedPlace
	for _, row := range checkins {
		if row.PlaceID == nil || *row.PlaceID == "" || row.Lat == nil || row.Lng == nil {
			continue
		}
		placeID := *row.PlaceID
		if i, ok := index[placeID]; ok {
			rows[i].VisitCount++
			continue
		}
		name := placeID
		if row.PlaceName != nil && *row.PlaceName != "" {
			name = *row.PlaceName
		}
		index[placeID] = len(rows)
		rows = append(rows, VisitedPlace{
			PlaceID: placeID, PlaceName: name, Lat: *row.Lat, Lng: *row.Lng, VisitCount: 1,
		})
	}
	slices.SortStableFunc(rows, func(a, b VisitedPlace) int {
		return countThenID(a.VisitCount, b.VisitCount, a.PlaceID, b.PlaceID)
	})
	return rows
}

// Place is the part of a place_rows() dict trending_layer reads.
type Place struct {
	ID          string
	Name        string
	Lat         float64
	Lng         float64
	Rating      *float64
	RatingCount *int64
	Flag        *string
}

// TrendingPlace is one trending_layer row.
type TrendingPlace struct {
	PlaceID     string
	PlaceName   string
	Lat         float64
	Lng         float64
	Rating      *float64
	RatingCount *int64
}

// TrendingLayer is trending_layer: the places flagged exactly "hot", in a
// stable sort by id.
func TrendingLayer(places []Place) []TrendingPlace {
	sorted := slices.Clone(places)
	slices.SortStableFunc(sorted, func(a, b Place) int { return compareStrings(a.ID, b.ID) })
	var rows []TrendingPlace
	for _, place := range sorted {
		if place.Flag == nil || *place.Flag != "hot" {
			continue
		}
		rows = append(rows, TrendingPlace{
			PlaceID: place.ID, PlaceName: place.Name, Lat: place.Lat, Lng: place.Lng,
			Rating: place.Rating, RatingCount: place.RatingCount,
		})
	}
	return rows
}

// HeatmapRow is one heatmap_rows row: an area_summary plus its counts.
type HeatmapRow struct {
	ID           string
	Label        string
	Lat          float64
	Lng          float64
	VisitCount   int
	SharePercent int
}

// resolve is _resolve: the nearest area of every row with both coordinates.
// It stops at the first ValueError, as the Python list comprehension does.
func resolve(checkins []Checkin) (resolved []areas.Area, found []bool, err error) {
	for _, row := range checkins {
		if row.Lat == nil || row.Lng == nil {
			continue
		}
		area, ok, err := areas.NearestArea(*row.Lat, *row.Lng)
		if err != nil {
			return nil, nil, err
		}
		resolved = append(resolved, area)
		found = append(found, ok)
	}
	return resolved, found, nil
}

// HeatmapRows is heatmap_rows: check-ins bucketed by district, ordered by
// count descending then area id, each with the floor percentage of the rows
// that resolved.
func HeatmapRows(checkins []Checkin) ([]HeatmapRow, error) {
	resolved, found, err := resolve(checkins)
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	var rows []HeatmapRow
	total := 0
	for i, area := range resolved {
		if !found[i] {
			continue
		}
		total++
		if j, ok := index[area.ID]; ok {
			rows[j].VisitCount++
			continue
		}
		index[area.ID] = len(rows)
		rows = append(rows, HeatmapRow{ID: area.ID, Label: area.Label, Lat: area.Lat, Lng: area.Lng, VisitCount: 1})
	}
	slices.SortStableFunc(rows, func(a, b HeatmapRow) int {
		return countThenID(a.VisitCount, b.VisitCount, a.ID, b.ID)
	})
	for i := range rows {
		if total > 0 {
			rows[i].SharePercent = rows[i].VisitCount * 100 / total
		}
	}
	return rows, nil
}

// UnknownAreaCount is unknown_area_count: rows with coordinates that fall in
// no area.
func UnknownAreaCount(checkins []Checkin) (int, error) {
	_, found, err := resolve(checkins)
	if err != nil {
		return 0, err
	}
	unknown := 0
	for _, ok := range found {
		if !ok {
			unknown++
		}
	}
	return unknown, nil
}

// countThenID orders by (-count, id).
func countThenID(countA, countB int, idA, idB string) int {
	if countA != countB {
		if countA > countB {
			return -1
		}
		return 1
	}
	return compareStrings(idA, idB)
}

// compareStrings orders by code point, which for valid UTF-8 is byte order.
func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
