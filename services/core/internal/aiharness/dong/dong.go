// Package dong holds closed enums: a fixed, ordered set of string values
// whose Parse refuses anything outside the set. Every enum the model fills in
// (router labels, intents, retrieval sources, verifier verdicts) is one of
// these, so a value the model invents is a structural error, never a guessed
// meaning (docs/architecture/03-ai-engine-hop-dong.md, «Luật không
// heuristic»). Parse is exact: no case folding, no trimming, no aliases.
package dong

import (
	"errors"
	"fmt"
)

// ErrLa is the error every refused value wraps.
var ErrLa = errors.New("dong: value outside the closed set")

// Tap is a closed set of values of T, in declaration order.
type Tap[T ~string] struct {
	ten string
	thu []T
	co  map[T]bool
}

// Moi builds a closed set named ten. It panics on an empty set, an empty
// value or a duplicate: those are programming errors in a package var.
func Moi[T ~string](ten string, v ...T) Tap[T] {
	if len(v) == 0 {
		panic("dong: empty set " + ten)
	}
	co := make(map[T]bool, len(v))
	for _, x := range v {
		if x == "" || co[x] {
			panic(fmt.Sprintf("dong: set %s has an empty or duplicate value %q", ten, x))
		}
		co[x] = true
	}
	return Tap[T]{ten: ten, thu: append([]T(nil), v...), co: co}
}

// Ten is the set's name, used in errors and schema descriptions.
func (t Tap[T]) Ten() string { return t.ten }

// Co reports whether v is in the set.
func (t Tap[T]) Co(v T) bool { return t.co[v] }

// Parse returns s as a T when it is in the set, else an error wrapping ErrLa.
func (t Tap[T]) Parse(s string) (T, error) {
	if t.co[T(s)] {
		return T(s), nil
	}
	return "", fmt.Errorf("%w: %s has no %q", ErrLa, t.ten, s)
}

// ParseAll parses every value; one unknown value refuses the whole list, and
// so does a duplicate (a closed list is a set).
func (t Tap[T]) ParseAll(ss []string) ([]T, error) {
	out := make([]T, 0, len(ss))
	seen := make(map[T]bool, len(ss))
	for _, s := range ss {
		v, err := t.Parse(s)
		if err != nil {
			return nil, err
		}
		if seen[v] {
			return nil, fmt.Errorf("%w: %s repeats %q", ErrLa, t.ten, s)
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, nil
}

// Values is the set in declaration order, as strings (a schema's enum).
func (t Tap[T]) Values() []string {
	out := make([]string, len(t.thu))
	for i, v := range t.thu {
		out[i] = string(v)
	}
	return out
}

// Len is the size of the set.
func (t Tap[T]) Len() int { return len(t.thu) }
