// Package valhalla is the pure half of services/api/app/journey/routing.py:
// everything the adapter does to a routing answer once it has one.
//
// The call itself is not here and must not be. `_post` opens a socket, and a
// coordinate is the one thing this product refuses to hand anybody but the
// configured private service, so the request belongs to the layer that is
// allowed to do IO. What is left over is pure and is what this package holds:
// the cost sanity check `_number`, the polyline6 decoder `_decode_shape`, and
// the shaping of `sources_to_targets` and `trip.legs` into the costs
// app/domain/journey.py schedules. Reading a base url out of the environment
// and validating it is configuration, also outside; nothing here needs it.
//
// A decoded body arrives as json.loads hands it to Python, which is what
// internal/pyjson decodes: nil, bool, *big.Int, float64, string, []any and
// Object. The domain may not import pyjson, so the caller maps it across.
//
// Every failure of the shaping is one error. That is not a simplification:
// routing.py catches KeyError, TypeError, ValueError and AttributeError around
// the whole comprehension and answers RoutingUnavailable, so a missing key, a
// list where a dict belongs and a cost out of range are already the same
// answer on the wire. The one distinction Python keeps is the one kept here:
// a cell with a null time or distance is a missing road, not a failure.
//
// testdata/python_valhalla*.json is rendered by
// scripts/render_domain_w7_goldens.py from the real module in the parity API
// image, and oracle_test.go replays every case.
package valhalla

import (
	"errors"
	"math"
	"math/big"

	"mobile/services/core/internal/domain/journey"
)

// The three failures this package answers with. ErrInvalidCost and
// ErrInvalidShape are the ValueErrors `_number` and `_decode_shape` raise;
// ErrRoutingUnavailable is RoutingUnavailable, which is what the shaping turns
// every one of them into, because routing.py catches KeyError, TypeError,
// ValueError and AttributeError around the whole comprehension. A caller
// outside this package sees only the last one -- no detail of a failed routing
// request may reach an API error or a log line -- but the two ValueErrors stay
// separate here so the oracle can tell that each one fired where Python's did.
var (
	ErrInvalidCost        = errors.New("invalid_cost")
	ErrInvalidShape       = errors.New("invalid_shape")
	ErrRoutingUnavailable = errors.New("routing_unavailable")
)

// modes is MODES: the transport a client may ask for, and what Valhalla calls
// the costing model behind it.
var modes = map[string]string{"motorbike": "motor_scooter", "car": "auto", "walk": "pedestrian"}

// Costing is MODES[mode], and whether the mode is one of the three.
func Costing(mode string) (string, bool) {
	costing, ok := modes[mode]
	return costing, ok
}

// Modes is MODES' keys, sorted, for a caller that needs to list them.
func Modes() []string { return []string{"car", "motorbike", "walk"} }

// MaxResponseBytes is MAX_RESPONSE_BYTES, the ceiling `_post` reads to. It
// lives here beside the rest of the adapter's constants; the reading is not.
const MaxResponseBytes = 8 * 1024 * 1024

// costCeiling is the 1_000_000_000 `_number` refuses a cost above -- the same
// number app/domain/journey.py scores a missing road at, so no accepted road
// can ever look as bad as no road.
const costCeiling = 1_000_000_000

// Object is a decoded JSON object. Only key lookup is read here, so the
// insertion order json.loads keeps is not carried.
type Object map[string]any

// Num is what `_number` returns: the value itself, still an int or still a
// float. The difference is not cosmetic. `round(_number(x) * 1000)` on an int
// is exact integer arithmetic, and on a float it is binary multiplication
// followed by round-half-to-even, so 0.0035 kilometres is 4 metres (the double
// nearest 3.5 is a hair above) while 0.0045 is 4 metres too (the double
// nearest 4.5 is exact, and 4 is the even one).
type Num struct {
	IsInt bool
	Int   int64
	Float float64
}

// Number is `_number`: a cost must be a real number, not a bool, not negative,
// not above the ceiling and not a NaN or an infinity.
//
// An int outside the ceiling is refused by the range test before
// math.isfinite() could be asked to convert it to a float, so the accepted
// ints are exactly 0..1_000_000_000 and always fit an int64.
func Number(value any) (Num, error) {
	switch v := value.(type) {
	case *big.Int:
		if v == nil {
			return Num{}, invalidCost()
		}
		if v.Sign() < 0 || v.Cmp(big.NewInt(costCeiling)) > 0 {
			return Num{}, invalidCost()
		}
		return Num{IsInt: true, Int: v.Int64()}, nil
	case float64:
		if v > costCeiling || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return Num{}, invalidCost()
		}
		return Num{Float: v}, nil
	}
	// bool lands here with every other type: `isinstance(value, bool)` is the
	// first clause of the same refusal.
	return Num{}, invalidCost()
}

func invalidCost() error { return ErrInvalidCost }

// Seconds is math.ceil(n): an int is already whole, a float rounds up.
func (n Num) Seconds() int64 {
	if n.IsInt {
		return n.Int
	}
	return int64(math.Ceil(n.Float))
}

// Meters is round(n * 1000): kilometres as metres, with Python's
// round-half-to-even on the float path and exact arithmetic on the int one.
func (n Num) Meters() int64 {
	if n.IsInt {
		return n.Int * 1000
	}
	return int64(math.RoundToEven(n.Float * 1000))
}

// Leg is one leg of a route, in the key order the preview's segments carry it:
// distance_meters, duration_seconds, geometry, source.
type Leg struct {
	DistanceMeters  int64
	DurationSeconds int64
	// Geometry is GeoJSON order: each pair is longitude then latitude.
	Geometry [][2]float64
	Source   string
}

// DecodeShape is `_decode_shape`: Valhalla's polyline6 as GeoJSON-order pairs.
//
// The cursor walks code points, not bytes, because Python indexes a str that
// way; a multi-byte character is one step and its ord() is far above the
// 63..126 the format uses, so it fails the byte range test rather than being
// read as several.
func DecodeShape(encoded string) ([][2]float64, error) {
	runes := []rune(encoded)
	cursor := 0
	var lat, lng int64
	var result [][2]float64
	for cursor < len(runes) {
		var coordinates [2]int64
		for part := 0; part < 2; part++ {
			var bits, shift int64
			for {
				if cursor >= len(runes) || shift > 30 {
					return nil, ErrInvalidShape
				}
				b := int64(runes[cursor]) - 63
				cursor++
				if b < 0 || b > 63 {
					return nil, ErrInvalidShape
				}
				bits |= (b & 31) << shift
				shift += 5
				if b < 32 {
					break
				}
			}
			if bits&1 != 0 {
				coordinates[part] = ^(bits >> 1)
			} else {
				coordinates[part] = bits >> 1
			}
		}
		lat += coordinates[0]
		lng += coordinates[1]
		if abs64(lat) > 90_000_000 || abs64(lng) > 180_000_000 {
			return nil, ErrInvalidShape
		}
		result = append(result, [2]float64{float64(lng) / 1_000_000, float64(lat) / 1_000_000})
	}
	if len(result) < 2 {
		return nil, ErrInvalidShape
	}
	return result, nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ShapeMatrix is the shaping half of ValhallaProvider.matrix: the decoded body
// of a sources_to_targets answer, for a square of n points, as the cost matrix
// journey.SuggestOrder searches. A cell whose time or distance is null is a
// road that does not exist.
func ShapeMatrix(response any, points int) (journey.Matrix, error) {
	body, ok := response.(Object)
	if !ok {
		return nil, ErrRoutingUnavailable
	}
	rows, ok := body["sources_to_targets"].([]any)
	if !ok {
		return nil, ErrRoutingUnavailable
	}
	if len(rows) != points {
		return nil, ErrRoutingUnavailable
	}
	for _, raw := range rows {
		row, ok := raw.([]any)
		if !ok || len(row) != points {
			return nil, ErrRoutingUnavailable
		}
	}
	out := make(journey.Matrix, 0, len(rows))
	for _, raw := range rows {
		row := raw.([]any)
		cells := make([]*journey.Cost, 0, len(row))
		for _, item := range row {
			cell, ok := item.(Object)
			if !ok {
				return nil, ErrRoutingUnavailable
			}
			// `c.get("time") is None or c.get("distance") is None`: a missing
			// key and an explicit null are the same missing road.
			if cell["time"] == nil || cell["distance"] == nil {
				cells = append(cells, nil)
				continue
			}
			seconds, err := Number(cell["time"])
			if err != nil {
				return nil, ErrRoutingUnavailable
			}
			meters, err := Number(cell["distance"])
			if err != nil {
				return nil, ErrRoutingUnavailable
			}
			cells = append(cells, &journey.Cost{Seconds: seconds.Seconds(), Meters: meters.Meters()})
		}
		out = append(out, cells)
	}
	return out, nil
}

// ShapeRoute is the shaping half of ValhallaProvider.route: the decoded body of
// a route answer, for a trip through points, as its legs. Fewer than two points
// is no request and no legs, which is the one early return of the Python.
func ShapeRoute(response any, points int) ([]Leg, error) {
	if points < 2 {
		return []Leg{}, nil
	}
	body, ok := response.(Object)
	if !ok {
		return nil, ErrRoutingUnavailable
	}
	trip, ok := body["trip"].(Object)
	if !ok {
		return nil, ErrRoutingUnavailable
	}
	legs, ok := trip["legs"].([]any)
	if !ok {
		return nil, ErrRoutingUnavailable
	}
	status, found := trip["status"]
	if !found {
		return nil, ErrRoutingUnavailable
	}
	if !isZero(status) || len(legs) != points-1 || !isKilometres(trip) {
		return nil, ErrRoutingUnavailable
	}
	out := make([]Leg, 0, len(legs))
	for _, raw := range legs {
		leg, ok := raw.(Object)
		if !ok {
			return nil, ErrRoutingUnavailable
		}
		summary, ok := leg["summary"].(Object)
		if !ok {
			return nil, ErrRoutingUnavailable
		}
		length, found := summary["length"]
		if !found {
			return nil, ErrRoutingUnavailable
		}
		meters, err := Number(length)
		if err != nil {
			return nil, ErrRoutingUnavailable
		}
		duration, found := summary["time"]
		if !found {
			return nil, ErrRoutingUnavailable
		}
		seconds, err := Number(duration)
		if err != nil {
			return nil, ErrRoutingUnavailable
		}
		shape, ok := leg["shape"].(string)
		if !ok {
			return nil, ErrRoutingUnavailable
		}
		geometry, err := DecodeShape(shape)
		if err != nil {
			return nil, ErrRoutingUnavailable
		}
		out = append(out, Leg{
			DistanceMeters:  meters.Meters(),
			DurationSeconds: seconds.Seconds(),
			Geometry:        geometry,
			Source:          "valhalla",
		})
	}
	return out, nil
}

// isZero is `trip["status"] != 0` read the way Python reads it: numeric
// equality across the numeric tower, so a status of 0.0 passes, and False
// passes too because a bool IS an int of that value.
func isZero(value any) bool {
	switch v := value.(type) {
	case bool:
		return !v
	case *big.Int:
		return v != nil && v.Sign() == 0
	case float64:
		return v == 0
	}
	return false
}

// isKilometres is `trip.get("units", "kilometers") != "kilometers"`: a trip
// that does not say what its numbers mean is taken at the units the request
// asked for, and one that says anything else is refused.
func isKilometres(trip Object) bool {
	units, found := trip["units"]
	if !found {
		return true
	}
	text, ok := units.(string)
	return ok && text == "kilometers"
}
