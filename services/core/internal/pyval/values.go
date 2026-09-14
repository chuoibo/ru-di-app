package pyval

import (
	"mobile/services/core/internal/pyjson"
)

// Value is what validation hands a handler: the Go image of the Python
// object pydantic produced. It is one of
//
//	pyjson.Null, pyjson.Bool, pyjson.Int, pyjson.Float, pyjson.String
//	List, *Dict, UUID, Date, DateTime, *Model, Bytes
//
// plus, under an `any` schema, the raw pyjson.List or *pyjson.OrderedMap
// that json.loads produced.
type Value any

// List is a validated Python list.
type List []Value

// Bytes is Python bytes: a request body FastAPI did not decode as JSON.
type Bytes []byte

// UUID is uuid.UUID.
type UUID [16]byte

// String formats u as str(uuid.UUID).
func (u UUID) String() string { return pyjson.UUID(u) }

// Model is a validated pydantic model instance.
type Model struct {
	// Class is the model's qualified name, e.g. app.api.schemas.ReportCreateRequest.
	Class string
	// Fields holds every declared field in declaration order, defaults included.
	Fields []Field
	// FieldsSet names the fields the input provided (model_fields_set), in
	// declaration order.
	FieldsSet []string
}

// Field is one model attribute.
type Field struct {
	Name  string
	Value Value
}

// Get returns the attribute called name.
func (m *Model) Get(name string) (Value, bool) {
	for _, f := range m.Fields {
		if f.Name == name {
			return f.Value, true
		}
	}
	return nil, false
}

// Dict is a validated Python dict. Keys compare as Python would compare the
// validated keys, so two inputs that validate to the same UUID collapse into
// one entry that keeps the first position and the last value.
type Dict struct {
	keys  []Value
	vals  []Value
	index map[string]int
}

// Len returns the number of entries.
func (d *Dict) Len() int { return len(d.keys) }

// Entry returns the i-th key and value in insertion order.
func (d *Dict) Entry(i int) (Value, Value) { return d.keys[i], d.vals[i] }

func (d *Dict) set(k, v Value) {
	if d.index == nil {
		d.index = map[string]int{}
	}
	id := hashKey(k)
	if i, ok := d.index[id]; ok {
		d.vals[i] = v
		return
	}
	d.index[id] = len(d.keys)
	d.keys = append(d.keys, k)
	d.vals = append(d.vals, v)
}

// hashKey is an identity for Python dict key equality over the key kinds a
// validated dict can hold. 1, 1.0 and True are one key, as in Python.
func hashKey(k Value) string {
	switch x := k.(type) {
	case pyjson.String:
		return "s" + string(x)
	case UUID:
		return "u" + string(x[:])
	case Date:
		return "d" + x.ISOFormat()
	case DateTime:
		// Aware datetimes compare by instant; naive ones by wall clock and
		// never equal an aware one.
		if x.Aware {
			return "t" + x.Time().UTC().Format("2006-01-02T15:04:05.000000")
		}
		return "w" + x.ISOFormat()
	case pyjson.Bool:
		if x {
			return "n1"
		}
		return "n0"
	case pyjson.Int:
		return "n" + x.String()
	case pyjson.Float:
		if i, ok := floatAsExactInt(float64(x)); ok {
			return "n" + i.String()
		}
		return "f" + pyjson.FloatRepr(float64(x))
	case pyjson.Null, nil:
		return "z"
	}
	// Lists, dicts and models are unhashable in Python and never keys.
	return "?"
}

func isNone(v any) bool {
	switch v.(type) {
	case nil, pyjson.Null:
		return true
	}
	return false
}
