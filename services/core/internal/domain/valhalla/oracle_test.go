package valhalla

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_valhalla*.json is rendered by
// scripts/render_domain_w7_goldens.py by calling the real `_number`,
// `_decode_shape` and the shaping halves of ValhallaProvider.matrix and
// .route in the parity API image, with `_post` already holding the answer.
// Every case is replayed here.

// functions is every ported piece, in the order of the script's
// VALHALLA_FUNCTIONS.
var functions = []string{"number", "decode_shape", "shape_matrix", "shape_route"}

// refusalClasses is every exception class the committed corpus must show.
var refusalClasses = []string{"ValueError", "RoutingUnavailable"}

func refusal(err error) (class, code string, ok bool) {
	switch {
	case errors.Is(err, ErrInvalidCost):
		return "ValueError", "invalid_cost", true
	case errors.Is(err, ErrInvalidShape):
		return "ValueError", "invalid_shape", true
	case errors.Is(err, ErrRoutingUnavailable):
		return "RoutingUnavailable", "routing_unavailable", true
	}
	return "", "", false
}

// --- the tagged body encoding -----------------------------------------------

// body expands the script's tagged JSON: nil, bool and str are themselves, an
// int is int64 or BigInt, a float is {"float": repr}, a list is a list, and an
// object is {"dict": [[key, value], ...]}.
func body(value any) (any, error) {
	switch v := value.(type) {
	case nil, bool, string:
		return v, nil
	case int64:
		return big.NewInt(v), nil
	case oracletest.BigInt:
		return oracletest.Integer(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			decoded, err := body(item)
			if err != nil {
				return nil, err
			}
			out = append(out, decoded)
		}
		return out, nil
	case map[string]any:
		if text, ok := v["float"]; ok && len(v) == 1 {
			return asFloat(text)
		}
		entries, ok := v["dict"]
		if !ok || len(v) != 1 {
			return nil, fmt.Errorf("%v is not a tagged body", v)
		}
		items, err := oracletest.List(entries)
		if err != nil {
			return nil, err
		}
		out := make(Object, len(items))
		for _, item := range items {
			pair, err := oracletest.List(item)
			if err != nil || len(pair) != 2 {
				return nil, fmt.Errorf("%v is not a key and a value", item)
			}
			key, err := oracletest.Str(pair[0])
			if err != nil {
				return nil, err
			}
			decoded, err := body(pair[1])
			if err != nil {
				return nil, err
			}
			out[key] = decoded
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected body %T", value)
}

// asFloat reads the repr the script wrote, which round-trips exactly.
func asFloat(value any) (float64, error) {
	text, err := oracletest.Str(value)
	if err != nil {
		return 0, err
	}
	// Python's repr is "inf", "-inf", "nan" or a decimal spelling, all of
	// which ParseFloat reads back to the same double.
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a float repr", text)
	}
	return parsed, nil
}

// wireFloat writes a float the way the script's enc_float does.
func wireFloat(value float64) any {
	return map[string]any{"float": pyjson.FloatRepr(value)}
}

func wireGeometry(points [][2]float64) []any {
	out := make([]any, 0, len(points))
	for _, point := range points {
		out = append(out, []any{wireFloat(point[0]), wireFloat(point[1])})
	}
	return out
}

func wireMatrix(matrix journey.Matrix) []any {
	rows := make([]any, 0, len(matrix))
	for _, row := range matrix {
		cells := make([]any, 0, len(row))
		for _, cost := range row {
			if cost == nil {
				cells = append(cells, nil)
				continue
			}
			cells = append(cells, []any{cost.Seconds, cost.Meters})
		}
		rows = append(rows, cells)
	}
	return rows
}

func wireLegs(legs []Leg) []any {
	out := make([]any, 0, len(legs))
	for _, item := range legs {
		out = append(out, map[string]any{
			"distance_meters":  item.DistanceMeters,
			"duration_seconds": item.DurationSeconds,
			"geometry":         wireGeometry(item.Geometry),
			"source":           item.Source,
		})
	}
	return out
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "number":
		value, err := body(args["value"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		number, err := Number(value)
		if err != nil {
			return nil, err
		}
		if number.IsInt {
			return number.Int, nil
		}
		return wireFloat(number.Float), nil
	case "decode_shape":
		encoded, err := oracletest.Str(args["encoded"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		points, err := DecodeShape(encoded)
		if err != nil {
			return nil, err
		}
		return wireGeometry(points), nil
	case "shape_matrix", "shape_route":
		raw, err := body(args["body"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		points, err := oracletest.Int64(args["points"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		if c.Fn == "shape_matrix" {
			matrix, err := ShapeMatrix(raw, int(points))
			if err != nil {
				return nil, err
			}
			return wireMatrix(matrix), nil
		}
		legs, err := ShapeRoute(raw, int(points))
		if err != nil {
			return nil, err
		}
		return wireLegs(legs), nil
	}
	return nil, oracletest.Decode(fmt.Errorf("no replay for %s", c.Fn))
}

func checkValhalla(t *testing.T, files []oracletest.File, committed bool) {
	t.Helper()
	report := oracletest.Agree(t, files, "valhalla", replay, refusal)
	if !committed {
		return
	}
	for _, name := range functions {
		if report.ByFn[name] == nil {
			t.Errorf("no case calls %s", name)
		}
	}
	for _, class := range refusalClasses {
		found := false
		for _, file := range files {
			for _, c := range file.Cases {
				if body, ok := c.Result["raised"].(map[string]any); ok {
					if kind, _ := body["type"].(string); kind == class {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("no committed case raises %s", class)
		}
	}
}

func TestValhallaMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_valhalla*.json")
	constants := oracletest.Constants(t, files, "valhalla")
	modes, err := oracletest.Row(constants["modes"], "motorbike", "car", "walk")
	if err != nil {
		t.Fatal(err)
	}
	for mode, want := range modes {
		costing, known := Costing(mode)
		if !known || costing != want {
			t.Errorf("mode %q: Python %v, Go %q (known %v)", mode, want, costing, known)
		}
	}
	if len(modes) != len(Modes()) {
		t.Errorf("Python knows %d modes, Go %d", len(modes), len(Modes()))
	}
	for name, want := range map[string]int64{
		"max_response_bytes": MaxResponseBytes, "cost_ceiling": costCeiling,
	} {
		got, err := oracletest.Int64(constants[name])
		if err != nil || got != want {
			t.Errorf("%s: Python %v, Go %d (%v)", name, constants[name], want, err)
		}
	}
	checkValhalla(t, files, true)
}
