// Package tree is the JSON-like value grounding functions read and write.
//
// It exists so internal/domain never imports internal/pyjson (the Go spelling
// of the Python domain boundary). Callers map this model across at the route
// and oracle edges; insertion order is kept because Python 3.7+ dicts do.
package tree

// Value is Null, Bool, Int, Float, String, List or *OrderedMap. A nil Value
// is treated as Null.
type Value interface {
	isValue()
}

// Null is Python None.
type Null struct{}

// Bool is Python bool.
type Bool bool

// Int is a JSON integer that fits in int64. Grounding never needs bigger.
type Int int64

// Float is Python float (IEEE 754 binary64).
type Float float64

// String is Python str.
type String string

// List is Python list.
type List []Value

func (Null) isValue()        {}
func (Bool) isValue()        {}
func (Int) isValue()         {}
func (Float) isValue()       {}
func (String) isValue()      {}
func (List) isValue()        {}
func (*OrderedMap) isValue() {}

// NewInt returns the Int for v.
func NewInt(v int64) Int { return Int(v) }

// OrderedMap is a Python dict with str keys: iteration follows insertion
// order, and overwriting an existing key keeps the key's original position.
type OrderedMap struct {
	keys  []string
	vals  []Value
	index map[string]int
}

// NewOrderedMap returns an empty map.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{index: map[string]int{}}
}

// Set stores v under k. A new key goes last; an existing key keeps its place.
func (m *OrderedMap) Set(k string, v Value) {
	if m.index == nil {
		m.index = map[string]int{}
	}
	if i, ok := m.index[k]; ok {
		m.vals[i] = v
		return
	}
	m.index[k] = len(m.keys)
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
}

// Get returns the value stored under k.
func (m *OrderedMap) Get(k string) (Value, bool) {
	if m == nil {
		return nil, false
	}
	i, ok := m.index[k]
	if !ok {
		return nil, false
	}
	return m.vals[i], true
}

// Len returns the number of keys.
func (m *OrderedMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// Keys returns the keys in insertion order. The slice is a copy.
func (m *OrderedMap) Keys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.keys...)
}

// Clone copies keys and values in insertion order. A nil map becomes empty.
func (m *OrderedMap) Clone() *OrderedMap {
	out := NewOrderedMap()
	if m == nil {
		return out
	}
	for i, k := range m.keys {
		out.Set(k, m.vals[i])
	}
	return out
}
