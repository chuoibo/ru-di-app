package routing

import (
	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/valhalla"
	"mobile/services/core/internal/pyjson"
)

// asAny maps a pyjson value into the shape valhalla.Shape* reads: objects
// as valhalla.Object, ints as *big.Int. An int64 would fail Number.
func asAny(value pyjson.Value) any {
	switch v := value.(type) {
	case nil, pyjson.Null:
		return nil
	case pyjson.Bool:
		return bool(v)
	case pyjson.Int:
		return v.Big()
	case pyjson.Float:
		return float64(v)
	case pyjson.String:
		return string(v)
	case pyjson.List:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = asAny(item)
		}
		return out
	case *pyjson.OrderedMap:
		obj := make(valhalla.Object, v.Len())
		for k, item := range v.All() {
			obj[k] = asAny(item)
		}
		return obj
	default:
		return nil
	}
}

func jsonNumber(value any) pyjson.Value {
	switch n := value.(type) {
	case int64:
		return pyjson.NewInt(n)
	case float64:
		return pyjson.Float(n)
	case int:
		return pyjson.NewInt(int64(n))
	default:
		return pyjson.Null{}
	}
}

func pointObjects(points []itinerary.Point) pyjson.List {
	out := make(pyjson.List, len(points))
	for i, point := range points {
		row := pyjson.NewOrderedMap()
		row.Set("lat", jsonNumber(point.Lat))
		row.Set("lon", jsonNumber(point.Lng))
		out[i] = row
	}
	return out
}
