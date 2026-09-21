package pyval

import (
	"strconv"

	"mobile/services/core/internal/pyjson"
)

// lineError is one pydantic-core ValLineError. loc runs outermost first and
// holds string and int items.
type lineError struct {
	typ string
	msg string
	ctx *pyjson.OrderedMap
	loc []any
}

// vErr is a failed validation: line errors, or a fatal error that Python
// would have let propagate as an exception.
type vErr struct {
	lines []lineError
	fatal error
}

func fail(typ, msg string, ctx *pyjson.OrderedMap) *vErr {
	return &vErr{lines: []lineError{{typ: typ, msg: msg, ctx: ctx}}}
}

func fatal(err error) *vErr { return &vErr{fatal: err} }

// withOuter prepends one location item to every line, as
// ValLineError.with_outer_location does while an error bubbles up.
func (e *vErr) withOuter(item any) *vErr {
	for i := range e.lines {
		e.lines[i].loc = append([]any{item}, e.lines[i].loc...)
	}
	return e
}

func ctxOf(kv ...any) *pyjson.OrderedMap {
	m := pyjson.NewOrderedMap()
	for i := 0; i+1 < len(kv); i += 2 {
		m.Set(kv[i].(string), kv[i+1].(pyjson.Value))
	}
	return m
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Error messages of pydantic-core 2.46.5 (src/errors/types.rs), measured in
// the parity image.

func errMissing() *vErr { return fail("missing", "Field required", nil) }

func errExtraForbidden() *vErr {
	return fail("extra_forbidden", "Extra inputs are not permitted", nil)
}

func errStringType() *vErr { return fail("string_type", "Input should be a valid string", nil) }

func errStringUnicode() *vErr {
	return fail("string_unicode", "Input should be a valid string, unable to parse raw data as a unicode string", nil)
}

func errStringTooShort(n int) *vErr {
	return fail("string_too_short", "String should have at least "+strconv.Itoa(n)+" character"+plural(n),
		ctxOf("min_length", pyjson.NewInt(int64(n))))
}

func errStringTooLong(n int) *vErr {
	return fail("string_too_long", "String should have at most "+strconv.Itoa(n)+" character"+plural(n),
		ctxOf("max_length", pyjson.NewInt(int64(n))))
}

func errPattern(pattern string) *vErr {
	return fail("string_pattern_mismatch", "String should match pattern '"+pattern+"'",
		ctxOf("pattern", pyjson.String(pattern)))
}

func errTooShort(fieldType string, min, actual int) *vErr {
	return fail("too_short", fieldType+" should have at least "+strconv.Itoa(min)+" item"+plural(min)+
		" after validation, not "+strconv.Itoa(actual),
		ctxOf("field_type", pyjson.String(fieldType), "min_length", pyjson.NewInt(int64(min)),
			"actual_length", pyjson.NewInt(int64(actual))))
}

func errTooLong(fieldType string, max, actual int) *vErr {
	return fail("too_long", fieldType+" should have at most "+strconv.Itoa(max)+" item"+plural(max)+
		" after validation, not "+strconv.Itoa(actual),
		ctxOf("field_type", pyjson.String(fieldType), "max_length", pyjson.NewInt(int64(max)),
			"actual_length", pyjson.NewInt(int64(actual))))
}

var simpleMessages = map[string]string{
	"int_type":              "Input should be a valid integer",
	"int_parsing":           "Input should be a valid integer, unable to parse string as an integer",
	"int_from_float":        "Input should be a valid integer, got a number with a fractional part",
	"int_parsing_size":      "Unable to parse input string as an integer, exceeded maximum size",
	"finite_number":         "Input should be a finite number",
	"float_type":            "Input should be a valid number",
	"float_parsing":         "Input should be a valid number, unable to parse string as a number",
	"bool_type":             "Input should be a valid boolean",
	"bool_parsing":          "Input should be a valid boolean, unable to interpret input",
	"list_type":             "Input should be a valid list",
	"dict_type":             "Input should be a valid dictionary",
	"uuid_type":             "UUID input should be a string, bytes or UUID object",
	"none_required":         "Input should be None",
	"model_attributes_type": "Input should be a valid dictionary or object to extract fields from",
}

func errSimple(typ string) *vErr {
	msg, ok := simpleMessages[typ]
	if !ok {
		panic("pyval: no message for " + typ)
	}
	return fail(typ, msg, nil)
}

func errUUIDParsing(text string) *vErr {
	return fail("uuid_parsing", "Input should be a valid UUID, "+text, ctxOf("error", pyjson.String(text)))
}

func errLiteral(expected string) *vErr {
	return fail("literal_error", "Input should be "+expected, ctxOf("expected", pyjson.String(expected)))
}

// errBound is greater_than / greater_than_equal / less_than /
// less_than_equal / multiple_of with its bound in ctx.
func errBound(typ string, v pyjson.Value) *vErr {
	return fail(typ, boundMessage(typ, displayNumber(v)), ctxOf(boundKey[typ], v))
}

var boundKey = map[string]string{
	"greater_than":       "gt",
	"greater_than_equal": "ge",
	"less_than":          "lt",
	"less_than_equal":    "le",
	"multiple_of":        "multiple_of",
}

func boundMessage(typ, shown string) string {
	switch typ {
	case "greater_than":
		return "Input should be greater than " + shown
	case "greater_than_equal":
		return "Input should be greater than or equal to " + shown
	case "less_than":
		return "Input should be less than " + shown
	case "less_than_equal":
		return "Input should be less than or equal to " + shown
	}
	return "Input should be a multiple of " + shown
}

// displayNumber renders a ctx number the way pydantic-core formats it into
// a message: Rust Display, so 90.0 reads "90".
func displayNumber(v pyjson.Value) string {
	switch x := v.(type) {
	case pyjson.Int:
		return x.String()
	case pyjson.Float:
		return rustDisplayFloat(float64(x))
	case pyjson.String:
		return string(x)
	}
	return "?"
}

// Error is a validation failure raised by a registered validator function:
// the Go image of ValueError, AssertionError, PydanticCustomError or
// PydanticKnownError inside a pydantic validator.
type Error struct {
	Type string
	Msg  string
	Ctx  *pyjson.OrderedMap
}

func (e *Error) Error() string { return e.Type + ": " + e.Msg }

// ValueError is `raise ValueError(msg)`. FastAPI's jsonable_encoder turns the
// exception object in ctx.error into {} (vars() of an exception), measured.
func ValueError(msg string) *Error {
	return &Error{Type: "value_error", Msg: "Value error, " + msg, Ctx: ctxOf("error", pyjson.NewOrderedMap())}
}

// AssertionError is a failed `assert cond, msg`.
func AssertionError(msg string) *Error {
	return &Error{Type: "assertion_error", Msg: "Assertion failed, " + msg, Ctx: ctxOf("error", pyjson.NewOrderedMap())}
}

// KnownBoundError is PydanticKnownError for greater_than, greater_than_equal,
// less_than, less_than_equal or multiple_of with bound v.
func KnownBoundError(typ string, v pyjson.Value) *Error {
	line := errBound(typ, v).lines[0]
	return &Error{Type: line.typ, Msg: line.msg, Ctx: line.ctx}
}
