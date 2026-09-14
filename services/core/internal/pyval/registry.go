package pyval

import (
	"fmt"
	"math/big"
	"sort"

	"mobile/services/core/internal/pyjson"
)

// ValidatorFunc is the Go port of one Python validator function named in the
// IR (a field_validator, a model_validator, or a pydantic internal). It
// returns the value validation continues with. Returning *Error records a
// validation error at the current location; any other error aborts the
// request the way an unexpected exception would.
type ValidatorFunc func(call *Call, v Value) (Value, error)

// Call describes one invocation.
type Call struct {
	// Name is the qualified Python name, e.g. app.api.schemas._require_timezone.
	Name string
	// Mode is "after", "before", "plain" or "wrap".
	Mode string
	// Keywords are functools.partial keywords the IR recorded.
	Keywords map[string]pyjson.Value
	// FieldName is set for with-info field validators.
	FieldName string
	// Data holds the fields of the enclosing model validated so far
	// (ValidationInfo.data), for with-info validators.
	Data map[string]Value
	// Handler runs the wrapped schema; set only in "wrap" mode.
	Handler func(Value) (Value, error)
}

// Registry maps IR validator names to Go ports.
type Registry struct {
	funcs map[string]ValidatorFunc
}

// NewRegistry returns a registry holding pydantic's own internal validators
// and the ports of the app validators on routes Go serves.
func NewRegistry() *Registry {
	r := &Registry{funcs: map[string]ValidatorFunc{}}
	for name, typ := range map[string]string{
		"greater_than_validator":          "greater_than",
		"greater_than_or_equal_validator": "greater_than_equal",
		"less_than_validator":             "less_than",
		"less_than_or_equal_validator":    "less_than_equal",
	} {
		r.Register("pydantic._internal._validators."+name, compareValidator(typ))
	}
	registerServedValidators(r)
	return r
}

// Register installs f under name, replacing any earlier registration.
func (r *Registry) Register(name string, f ValidatorFunc) {
	r.funcs[name] = f
}

// Names lists the registered names, sorted.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.funcs))
	for k := range r.funcs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) lookup(name string) (ValidatorFunc, bool) {
	if r == nil {
		return nil, false
	}
	f, ok := r.funcs[name]
	return f, ok
}

// compareValidator ports pydantic._internal._validators.<cmp>_validator:
// `if not (x <op> bound): raise PydanticKnownError(type, {key: bound})`.
func compareValidator(typ string) ValidatorFunc {
	key := boundKey[typ]
	return func(call *Call, v Value) (Value, error) {
		b, ok := call.Keywords[key]
		if !ok {
			return nil, fmt.Errorf("pyval: %s called without %s", call.Name, key)
		}
		c, comparable := comparePy(v, b)
		if !comparable {
			return nil, fmt.Errorf("pyval: unable to apply constraint %q", key)
		}
		pass := false
		switch typ {
		case "greater_than":
			pass = c > 0
		case "greater_than_equal":
			pass = c >= 0
		case "less_than":
			pass = c < 0
		case "less_than_equal":
			pass = c <= 0
		}
		if !pass {
			return nil, KnownBoundError(typ, b)
		}
		return v, nil
	}
}

// comparePy orders two Python numbers exactly. NaN compares as unordered:
// every comparison with it is false, which callers see as "not passing"
// because c is then 2.
func comparePy(a, b any) (int, bool) {
	fa, okA := bigFloatOf(a)
	fb, okB := bigFloatOf(b)
	if !okA || !okB {
		return 0, false
	}
	if fa == nil || fb == nil {
		return 2, true
	}
	return fa.Cmp(fb), true
}

func bigFloatOf(v any) (*big.Float, bool) {
	switch x := v.(type) {
	case pyjson.Int:
		return new(big.Float).SetInt(x.Big()), true
	case pyjson.Bool:
		if x {
			return big.NewFloat(1), true
		}
		return big.NewFloat(0), true
	case pyjson.Float:
		f := float64(x)
		if f != f {
			return nil, true
		}
		return new(big.Float).SetFloat64(f), true
	}
	return nil, false
}
