package routes

import (
	"encoding/json"
	"fmt"
	"math"

	"mobile/services/core/internal/domain/scoring"
	"mobile/services/core/internal/pyjson"
)

// groupFit reads a catalogue row's JSONB group_fit the way
// app.places.scoring.group_size_fit does. A falsy value (SQL NULL, null, {},
// [], "", 0, false) states no capacity. An object's min_people and max_people
// are compared with an integer party size, so a float bound is equivalent to
// its ceiling (min) or floor (max). Where Python would raise on indexing or
// comparing, the error is returned for the caller to raise only when the party
// size is known: Python checks the size before it touches the bounds.
func groupFit(raw json.RawMessage) (*scoring.GroupFit, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	value, err := pyjson.Loads(raw)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case *pyjson.OrderedMap:
		if v.Len() == 0 {
			return nil, nil
		}
		low, err := bound(v, "min_people", math.Ceil)
		if err != nil {
			return nil, err
		}
		high, err := bound(v, "max_people", math.Floor)
		if err != nil {
			return nil, err
		}
		return &scoring.GroupFit{MinPeople: low, MaxPeople: high}, nil
	case pyjson.List:
		if len(v) == 0 {
			return nil, nil
		}
	case pyjson.String:
		if v == "" {
			return nil, nil
		}
	case pyjson.Bool:
		if !bool(v) {
			return nil, nil
		}
	case pyjson.Int:
		if n, ok := v.Int64(); ok && n == 0 {
			return nil, nil
		}
	case pyjson.Float:
		if v == 0 {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("routes: group_fit %s is not subscriptable by key", raw)
}

// bound reads one capacity bound as the integer it compares like.
func bound(fit *pyjson.OrderedMap, key string, round func(float64) float64) (int64, error) {
	value, ok := fit.Get(key)
	if !ok {
		return 0, fmt.Errorf("routes: group_fit has no %s", key)
	}
	switch v := value.(type) {
	case pyjson.Int:
		if n, ok := v.Int64(); ok {
			return n, nil
		}
		if v.Big().Sign() > 0 {
			return math.MaxInt64, nil
		}
		return math.MinInt64, nil
	case pyjson.Bool:
		if bool(v) {
			return 1, nil
		}
		return 0, nil
	case pyjson.Float:
		f := round(float64(v))
		switch {
		case math.IsNaN(f):
			// Every comparison with NaN is False, so nothing fits.
			if key == "min_people" {
				return math.MaxInt64, nil
			}
			return math.MinInt64, nil
		case f >= math.MaxInt64:
			return math.MaxInt64, nil
		case f <= math.MinInt64:
			return math.MinInt64, nil
		}
		return int64(f), nil
	}
	return 0, fmt.Errorf("routes: group_fit %s is %T, which does not compare with an int", key, value)
}
