package oracletest

import (
	"fmt"

	"mobile/services/core/internal/pyjson"
)

// AsPyJSON turns a decoded oracle value into the pyjson model grounding
// functions consume (dicts become OrderedMap, preserving key order from the
// JSON decoder's map — golden cases use small dicts whose order the tests
// also pin by rebuilding from the same keys).
func AsPyJSON(v any) (pyjson.Value, error) {
	switch x := v.(type) {
	case nil:
		return pyjson.Null{}, nil
	case bool:
		return pyjson.Bool(x), nil
	case int64:
		return pyjson.NewInt(x), nil
	case float64:
		return pyjson.Float(x), nil
	case string:
		return pyjson.String(x), nil
	case []any:
		out := make(pyjson.List, len(x))
		for i, item := range x {
			value, err := AsPyJSON(item)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		return out, nil
	case map[string]any:
		out := pyjson.NewOrderedMap()
		for _, key := range sortedKeys(x) {
			value, err := AsPyJSON(x[key])
			if err != nil {
				return nil, err
			}
			out.Set(key, value)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("oracletest: no pyjson form for %T", v)
	}
}

// FromPyJSON is the inverse of AsPyJSON for comparing a grounded card to
// Python's dict.
func FromPyJSON(v pyjson.Value) any {
	switch x := v.(type) {
	case nil, pyjson.Null:
		return nil
	case pyjson.Bool:
		return bool(x)
	case pyjson.Int:
		if n, ok := x.Int64(); ok {
			return n
		}
		return x.String()
	case pyjson.Float:
		return float64(x)
	case pyjson.String:
		return string(x)
	case pyjson.List:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = FromPyJSON(item)
		}
		return out
	case *pyjson.OrderedMap:
		out := map[string]any{}
		if x == nil {
			return out
		}
		for key, value := range x.All() {
			out[key] = FromPyJSON(value)
		}
		return out
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	// Insertion order is gone once the golden has been through encoding/json.
	// Grounding rebuilds a whitelist, so comparison is by key set + values.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// OrderedMap is AsPyJSON for a dict, or a decode error.
func OrderedMap(value any) (*pyjson.OrderedMap, error) {
	decoded, err := AsPyJSON(value)
	if err != nil {
		return nil, err
	}
	m, ok := decoded.(*pyjson.OrderedMap)
	if !ok {
		return nil, fmt.Errorf("oracletest: want a dict, got %T", value)
	}
	return m, nil
}

// OrderedMaps is AsPyJSON for a list of dicts. A non-dict item is a decode error.
func OrderedMaps(value any) ([]*pyjson.OrderedMap, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("oracletest: want a list, got %T", value)
	}
	out := make([]*pyjson.OrderedMap, 0, len(items))
	for i, item := range items {
		m, err := OrderedMap(item)
		if err != nil {
			return nil, fmt.Errorf("oracletest: item %d: %w", i, err)
		}
		out = append(out, m)
	}
	return out, nil
}
