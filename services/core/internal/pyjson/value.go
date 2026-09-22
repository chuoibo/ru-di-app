// Package pyjson reproduces, byte for byte, the JSON that the Python API
// (CPython 3.12 json module, Starlette JSONResponse, pydantic 2 JSON mode)
// writes and reads, so a Go handler can answer a route in exactly the bytes
// FastAPI would have produced.
//
// The value model keeps the Python distinctions that encoding/json erases:
// int and float are different types (1 vs 1.0), ints are arbitrary precision,
// objects keep insertion order with dict overwrite semantics, and strings may
// carry lone surrogate code points.
//
// # Strings
//
// A String holds "generalized UTF-8": every code point is encoded on its own
// with the usual UTF-8 bit layout, and surrogate code points U+D800..U+DFFF
// use the 3-byte form (ED A0 80 .. ED BF BF). A Python str containing a lone
// surrogate, or a high and a low surrogate as two separate code points, is
// therefore representable exactly, and byte order of two such strings equals
// Python's code point order. Ordinary Go strings (valid UTF-8) are already in
// this form.
package pyjson

import (
	"iter"
	"math/big"
	"strconv"
)

// Value is one of Null, Bool, Int, Float, String, List or *OrderedMap.
// A nil Value is treated as Null by the encoders.
type Value interface {
	isValue()
}

// Null is Python None.
type Null struct{}

// IsNull reports whether v is absent or Python None.
//
// Read this together with the note on Value above: a nil Value encodes as Null,
// but nothing that comes back from `Loads` is ever a nil Value. A decoded null
// is `Null{}`, a struct, so the interface holding it is NOT nil and `v == nil`
// is false. Guarding a decoded document with `== nil` therefore lets every JSON
// null through.
//
// That is not a rare shape: with no model key configured the brain answers
// `{"card": null}`, so the guard meant to stop there is the one that does not.
func IsNull(v Value) bool {
	if v == nil {
		return true
	}
	_, null := v.(Null)
	return null
}

// Bool is Python bool.
type Bool bool

// Float is Python float (IEEE 754 binary64).
type Float float64

// String is Python str in generalized UTF-8 (see the package doc).
type String string

// List is Python list.
type List []Value

func (Null) isValue()   {}
func (Bool) isValue()   {}
func (Int) isValue()    {}
func (Float) isValue()  {}
func (String) isValue() {}
func (List) isValue()   {}

// Int is Python int: arbitrary precision, with an int64 fast path.
// The zero value is 0.
type Int struct {
	small int64
	big   *big.Int // nil when small holds the value
}

// NewInt returns the Int for v.
func NewInt(v int64) Int { return Int{small: v} }

// NewBigInt returns the Int for a copy of v. A nil v is 0.
func NewBigInt(v *big.Int) Int {
	if v == nil {
		return Int{}
	}
	if v.IsInt64() {
		return Int{small: v.Int64()}
	}
	return Int{big: new(big.Int).Set(v)}
}

// ParseInt parses an optionally signed base-10 integer with no other
// characters. It applies no digit limit; the encoders apply Python's.
func ParseInt(s string) (Int, bool) {
	if s == "" || s[0] == '+' {
		return Int{}, false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return Int{small: n}, true
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && !(i == 0 && c == '-' && len(s) > 1) {
			return Int{}, false
		}
	}
	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return Int{}, false
	}
	return NewBigInt(b), true
}

// Int64 returns the value and whether it fits in an int64.
func (i Int) Int64() (int64, bool) {
	if i.big == nil {
		return i.small, true
	}
	return 0, false
}

// Big returns the value as a new big.Int.
func (i Int) Big() *big.Int {
	if i.big == nil {
		return big.NewInt(i.small)
	}
	return new(big.Int).Set(i.big)
}

// String returns the base-10 text of the value (Python repr, no digit limit).
func (i Int) String() string {
	if i.big == nil {
		return strconv.FormatInt(i.small, 10)
	}
	return i.big.String()
}

// OrderedMap is a Python dict with str keys: iteration follows insertion
// order, and overwriting an existing key keeps the key's original position.
// A nil *OrderedMap behaves as an empty map for reads and encodes as {}.
type OrderedMap struct {
	keys  []string
	vals  []Value
	index map[string]int
}

func (*OrderedMap) isValue() {}

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

// Keys returns the keys in iteration order. The slice is a copy.
func (m *OrderedMap) Keys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.keys...)
}

// Len returns the number of keys.
func (m *OrderedMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// All iterates over key/value pairs in iteration order.
func (m *OrderedMap) All() iter.Seq2[string, Value] {
	return func(yield func(string, Value) bool) {
		if m == nil {
			return
		}
		for i, k := range m.keys {
			if !yield(k, m.vals[i]) {
				return
			}
		}
	}
}
