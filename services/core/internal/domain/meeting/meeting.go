// Package meeting is app/places/meeting.py: F45's fair meeting point, ranked
// by the worst journey anybody makes, over areas that name no person.
package meeting

import (
	"math"
	"slices"
	"strconv"

	"mobile/services/core/internal/domain/areas"
)

const (
	// MinOriginAreas is MIN_ORIGIN_AREAS.
	MinOriginAreas = 2
	// MaxOriginAreas is MAX_ORIGIN_AREAS.
	MaxOriginAreas = 12
)

// Place is the part of a place_rows() dict rank_meeting_points reads.
// Address is nil where the row's address is None.
type Place struct {
	ID       string
	Name     string
	Category string
	Address  *string
	Lat      float64
	Lng      float64
}

// Fairness is the fairness dict: each value is Python's round(x, 2).
type Fairness struct {
	WorstKm  float64
	TotalKm  float64
	SpreadKm float64
}

// Leg is one travel entry: the origin's area_summary plus its rounded km.
type Leg struct {
	ID    string
	Label string
	Lat   float64
	Lng   float64
	Km    float64
}

// Candidate is one ranked row, without the internal sort key.
type Candidate struct {
	PlaceID   string
	PlaceName string
	Category  string
	Address   *string
	Lat       float64
	Lng       float64
	Fairness  Fairness
	Travel    []Leg
}

type ranked struct {
	candidate  Candidate
	worst, sum float64
	placeID    string
}

// RankMeetingPoints is rank_meeting_points(origins, places, limit=limit).
//
// Origins keep their order and duplicates, and a place is skipped when there
// are no origins. Rows sort stably on the unrounded (worst, sum, place id),
// and limit slices like Python's ranked[:limit], negative values included.
// The error is the ValueError haversine_km raised, for the first place and
// origin in input order that triggers one.
//
// Coordinates are finite, as the places table's CHECK constraints guarantee.
// With a NaN or infinite coordinate a distance can be NaN, and CPython's sort
// then orders rows in a way this stable sort does not promise to reproduce.
func RankMeetingPoints(origins []areas.Area, places []Place, limit int) ([]Candidate, error) {
	rows := make([]ranked, 0, len(places))
	for _, place := range places {
		legs := make([]float64, len(origins))
		for i, area := range origins {
			km, err := areas.HaversineKm(area.Lat, area.Lng, place.Lat, place.Lng)
			if err != nil {
				return nil, err
			}
			legs[i] = km
		}
		if len(legs) == 0 {
			continue
		}
		worst, least := pyMax(legs), pyMin(legs)
		total := pySum(legs)
		travel := make([]Leg, len(origins))
		for i, area := range origins {
			travel[i] = Leg{ID: area.ID, Label: area.Label, Lat: area.Lat, Lng: area.Lng, Km: pyRound2(legs[i])}
		}
		rows = append(rows, ranked{
			candidate: Candidate{
				PlaceID:   place.ID,
				PlaceName: place.Name,
				Category:  place.Category,
				Address:   place.Address,
				Lat:       place.Lat,
				Lng:       place.Lng,
				Fairness: Fairness{
					WorstKm:  pyRound2(worst),
					TotalKm:  pyRound2(total),
					SpreadKm: pyRound2(worst - least),
				},
				Travel: travel,
			},
			worst:   worst,
			sum:     total,
			placeID: place.ID,
		})
	}
	slices.SortStableFunc(rows, compareRanked)
	end := limit
	if end < 0 {
		end += len(rows)
		if end < 0 {
			end = 0
		}
	}
	if end > len(rows) {
		end = len(rows)
	}
	out := make([]Candidate, end)
	for i := range out {
		out[i] = rows[i].candidate
	}
	return out, nil
}

// compareRanked is tuple comparison of (worst, sum, place_id) for non-NaN
// floats: the first unequal element decides.
func compareRanked(a, b ranked) int {
	for _, pair := range [2][2]float64{{a.worst, b.worst}, {a.sum, b.sum}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.placeID < b.placeID:
		return -1
	case a.placeID > b.placeID:
		return 1
	}
	return 0
}

// pyMax is max(values) for a non-empty list: the first item no later item
// is greater than.
func pyMax(values []float64) float64 {
	best := values[0]
	for _, v := range values[1:] {
		if v > best {
			best = v
		}
	}
	return best
}

// pyMin is min(values) for a non-empty list.
func pyMin(values []float64) float64 {
	best := values[0]
	for _, v := range values[1:] {
		if v < best {
			best = v
		}
	}
	return best
}

// pySum is CPython 3.12's sum(values) for a non-empty list of floats: the
// int start 0 plus the first item, then Neumaier compensated summation over
// the rest, the compensation added once at the end when finite and non-zero.
func pySum(values []float64) float64 {
	total := 0.0 + values[0]
	compensation := 0.0
	for _, x := range values[1:] {
		t := total + x
		if math.Abs(total) >= math.Abs(x) {
			compensation += (total - t) + x
		} else {
			compensation += (x - t) + total
		}
		total = t
	}
	if compensation != 0 && !math.IsInf(compensation, 0) && !math.IsNaN(compensation) {
		total += compensation
	}
	return total
}

// pyRound2 is round(x, 2): the double nearest to x's exact binary value
// rounded half-even to two decimals, which strconv's exact decimal
// conversion also computes. NaN and infinities round to themselves.
func pyRound2(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 2, 64), 64)
	if err != nil {
		panic("meeting: formatted float does not parse: " + err.Error())
	}
	return rounded
}
