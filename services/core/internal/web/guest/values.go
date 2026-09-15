package guest

import (
	"fmt"
	"math/big"
)

// The view builders take and return Python values, because the Python
// builders are functions over plain dicts and what they raise depends on the
// runtime type of each value. A value is one of:
//
//	nil             None
//	bool            bool
//	int64, int      int inside int64
//	*big.Int        int of any size
//	string          str
//	[]any           list or tuple
//	map[string]any  dict with str keys
//
// Anything else is refused with an *UnsupportedError rather than guessed at.

// PyError is an exception Python lets escape from a builder or a template:
// KeyError, TypeError, ValueError or UndefinedError. Routes answer it as a
// 500, as the API does.
type PyError struct {
	Class   string
	Message string
}

func (e *PyError) Error() string { return e.Class + ": " + e.Message }

// UnsupportedError is a value or construct this port does not model. No input
// the Python service builds reaches it.
type UnsupportedError struct{ Reason string }

func (e *UnsupportedError) Error() string { return "guest: unsupported: " + e.Reason }

// keyError is KeyError(key); its str() is the repr of a str key without
// quotes of its own, which every key read here is.
func keyError(key string) error { return &PyError{Class: "KeyError", Message: "'" + key + "'"} }

func typeError(format string, args ...any) error {
	return &PyError{Class: "TypeError", Message: fmt.Sprintf(format, args...)}
}

// pyInt returns v as an exact int when it is a Python int (bool included).
func pyInt(v any) (*big.Int, bool) {
	switch n := v.(type) {
	case bool:
		if n {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	case int:
		return big.NewInt(int64(n)), true
	case int64:
		return big.NewInt(n), true
	case *big.Int:
		return n, n != nil
	}
	return nil, false
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case int, int64, *big.Int:
		return "int"
	case string:
		return "str"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	}
	return fmt.Sprintf("%T", v)
}

func checkSupported(v any) error {
	switch v.(type) {
	case nil, bool, int, int64, *big.Int, string, []any, map[string]any:
		return nil
	}
	return &UnsupportedError{Reason: fmt.Sprintf("value of Go type %T", v)}
}

// truthy is bool(v).
func truthy(v any) (bool, error) {
	switch x := v.(type) {
	case nil:
		return false, nil
	case bool:
		return x, nil
	case string:
		return x != "", nil
	case []any:
		return len(x) > 0, nil
	case map[string]any:
		return len(x) > 0, nil
	}
	if n, ok := pyInt(v); ok {
		return n.Sign() != 0, nil
	}
	return false, checkSupported(v)
}

// less is a < b for the operand types the builders meet: ints (bool
// included) with ints, str with str. Every other pairing of None, str and int
// raises TypeError in Python; containers are not modelled.
func less(a, b any) (bool, error) {
	for _, v := range []any{a, b} {
		if err := checkSupported(v); err != nil {
			return false, err
		}
		switch v.(type) {
		case []any, map[string]any:
			return false, &UnsupportedError{Reason: "ordering a container"}
		}
	}
	x, aInt := pyInt(a)
	y, bInt := pyInt(b)
	if aInt && bInt {
		return x.Cmp(y) < 0, nil
	}
	s, aStr := a.(string)
	t, bStr := b.(string)
	if aStr && bStr {
		// Byte order of UTF-8 is code point order, which is how str compares.
		return s < t, nil
	}
	return false, typeError("'<' not supported between instances of '%s' and '%s'", typeName(a), typeName(b))
}

// item is obj[key] with a str key.
func item(obj any, key string) (any, error) {
	m, ok := obj.(map[string]any)
	if !ok {
		if err := checkSupported(obj); err != nil {
			return nil, err
		}
		return nil, typeError("'%s' object is not subscriptable by str", typeName(obj))
	}
	v, found := m[key]
	if !found {
		return nil, keyError(key)
	}
	return v, checkSupported(v)
}

// get is dict.get(key, fallback) on a dict.
func get(m map[string]any, key string, fallback any) (any, error) {
	if v, found := m[key]; found {
		return v, checkSupported(v)
	}
	return fallback, nil
}

// iterate lists what `for x in v` yields when every x is then subscripted by
// a str: a list's items; a str's characters or a dict's keys, each of which
// is a str that the subscript refuses, so only whether there are any matters.
func iterate(v any) ([]any, error) {
	switch x := v.(type) {
	case []any:
		return x, nil
	case string:
		if x == "" {
			return nil, nil
		}
		return []any{"c"}, nil
	case map[string]any:
		if len(x) == 0 {
			return nil, nil
		}
		return []any{"key"}, nil
	}
	if err := checkSupported(v); err != nil {
		return nil, err
	}
	return nil, typeError("'%s' object is not iterable", typeName(v))
}

// equalStr is x == s for a str s: only an equal str is equal.
func equalStr(x any, s string) bool {
	t, ok := x.(string)
	return ok && t == s
}
