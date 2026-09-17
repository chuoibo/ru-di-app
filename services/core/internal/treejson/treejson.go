// Package treejson maps pyjson values to the domain tree model and back.
package treejson

import (
	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/pyjson"
)

// To maps a pyjson value into domain/tree.
func To(v pyjson.Value) tree.Value {
	switch x := v.(type) {
	case nil:
		return tree.Null{}
	case pyjson.Null:
		return tree.Null{}
	case pyjson.Bool:
		return tree.Bool(x)
	case pyjson.Int:
		if n, ok := x.Int64(); ok {
			return tree.NewInt(n)
		}
		return tree.String(x.String())
	case pyjson.Float:
		return tree.Float(x)
	case pyjson.String:
		return tree.String(x)
	case pyjson.List:
		out := make(tree.List, len(x))
		for i, item := range x {
			out[i] = To(item)
		}
		return out
	case *pyjson.OrderedMap:
		return MapTo(x)
	default:
		return tree.Null{}
	}
}

// From maps a domain/tree value into pyjson.
func From(v tree.Value) pyjson.Value {
	switch x := v.(type) {
	case nil:
		return pyjson.Null{}
	case tree.Null:
		return pyjson.Null{}
	case tree.Bool:
		return pyjson.Bool(x)
	case tree.Int:
		return pyjson.NewInt(int64(x))
	case tree.Float:
		return pyjson.Float(x)
	case tree.String:
		return pyjson.String(x)
	case tree.List:
		out := make(pyjson.List, len(x))
		for i, item := range x {
			out[i] = From(item)
		}
		return out
	case *tree.OrderedMap:
		return MapFrom(x)
	default:
		return pyjson.Null{}
	}
}

// MapTo maps a pyjson object. Nil stays nil.
func MapTo(m *pyjson.OrderedMap) *tree.OrderedMap {
	if m == nil {
		return nil
	}
	out := tree.NewOrderedMap()
	for _, k := range m.Keys() {
		v, _ := m.Get(k)
		out.Set(k, To(v))
	}
	return out
}

// MapFrom maps a domain object. Nil stays nil.
func MapFrom(m *tree.OrderedMap) *pyjson.OrderedMap {
	if m == nil {
		return nil
	}
	out := pyjson.NewOrderedMap()
	for _, k := range m.Keys() {
		v, _ := m.Get(k)
		out.Set(k, From(v))
	}
	return out
}

// MapsTo maps a list of objects.
func MapsTo(ms []*pyjson.OrderedMap) []*tree.OrderedMap {
	out := make([]*tree.OrderedMap, len(ms))
	for i, m := range ms {
		out[i] = MapTo(m)
	}
	return out
}

// MapsFrom maps a list of domain objects.
func MapsFrom(ms []*tree.OrderedMap) []*pyjson.OrderedMap {
	out := make([]*pyjson.OrderedMap, len(ms))
	for i, m := range ms {
		out[i] = MapFrom(m)
	}
	return out
}
