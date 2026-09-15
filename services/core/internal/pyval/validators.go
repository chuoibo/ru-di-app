package pyval

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"strings"

	"mobile/services/core/internal/pyjson"
)

// validator is one pydantic-core validator in Python mode
// (TypeAdapter.validate_python with from_attributes=True, as FastAPI calls
// it). Inputs are what json.loads or Starlette produced: pyjson values,
// Bytes, and pyjson.String for path, query and header values.
type validator interface {
	validate(st *state, in any) (Value, *vErr)
	name() string
}

// Exactness levels of pydantic-core, used by smart unions.
const (
	lax = iota
	strictMatch
	exact
)

type state struct {
	exactness      int
	fieldsSetCount int // -1 when no model set it
	data           map[string]Value
	fieldName      string
}

func newState() *state { return &state{exactness: exact, fieldsSetCount: -1} }

func (st *state) floor(level int) {
	if level < st.exactness {
		st.exactness = level
	}
}

type unsupportedValidator struct{}

func (unsupportedValidator) validate(*state, any) (Value, *vErr) {
	return nil, fatal(errors.New("pyval: unsupported schema reached"))
}
func (unsupportedValidator) name() string { return "unsupported" }

// ---- str ----

type strValidator struct {
	strict      bool
	min, max    int
	pattern     *regexp.Regexp
	patternText string
	constrained bool
}

func (v *strValidator) name() string {
	if v.constrained {
		return "constrained-str"
	}
	return "str"
}

func (v *strValidator) validate(st *state, in any) (Value, *vErr) {
	s, ok := in.(pyjson.String)
	if !ok {
		return nil, errStringType()
	}
	if !v.constrained {
		return s, nil
	}
	text := string(s)
	if hasSurrogate(text) {
		return nil, errStringUnicode()
	}
	n := -1
	if v.min >= 0 || v.max >= 0 {
		n = codePoints(text)
	}
	if v.min >= 0 && n < v.min {
		return nil, errStringTooShort(v.min)
	}
	if v.max >= 0 && n > v.max {
		return nil, errStringTooLong(v.max)
	}
	if v.pattern != nil && !v.pattern.MatchString(text) {
		return nil, errPattern(v.patternText)
	}
	return s, nil
}

// ---- int ----

type numBound struct {
	key string
	b   bound
	raw pyjson.Value
}

var boundType = map[string]string{
	"multiple_of": "multiple_of",
	"le":          "less_than_equal",
	"lt":          "less_than",
	"ge":          "greater_than_equal",
	"gt":          "greater_than",
}

type intValidator struct {
	strict bool
	bounds []numBound
}

func (v *intValidator) name() string {
	if len(v.bounds) > 0 {
		return "constrained-int"
	}
	return "int"
}

func (v *intValidator) validate(st *state, in any) (Value, *vErr) {
	var i pyjson.Int
	switch x := in.(type) {
	case pyjson.Int:
		i = x
	case pyjson.Bool:
		if v.strict {
			return nil, errSimple("int_type")
		}
		st.floor(lax)
		if x {
			i = pyjson.NewInt(1)
		} else {
			i = pyjson.NewInt(0)
		}
	case pyjson.Float:
		if v.strict {
			return nil, errSimple("int_type")
		}
		st.floor(lax)
		f := float64(x)
		switch {
		case math.IsNaN(f) || math.IsInf(f, 0):
			return nil, errSimple("finite_number")
		case f != math.Trunc(f):
			return nil, errSimple("int_from_float")
		case !(f > math.MinInt64 && f < math.MaxInt64):
			return nil, errSimple("int_parsing_size")
		}
		i = pyjson.NewInt(int64(f))
	case pyjson.String:
		if v.strict {
			return nil, errSimple("int_type")
		}
		st.floor(lax)
		if hasSurrogate(string(x)) {
			return nil, errStringUnicode()
		}
		parsed, errType := strAsInt(string(x))
		if errType != "" {
			return nil, errSimple(errType)
		}
		i = parsed
	default:
		return nil, errSimple("int_type")
	}
	for _, b := range v.bounds {
		failed := false
		switch b.key {
		case "multiple_of":
			if b.b.isFloat || b.b.i.Sign() == 0 {
				failed = false
			} else {
				failed = new(big.Int).Rem(i.Big(), b.b.i).Sign() != 0
			}
		case "le":
			failed = cmpIntBound(i, b.b) > 0
		case "lt":
			failed = cmpIntBound(i, b.b) >= 0
		case "ge":
			failed = cmpIntBound(i, b.b) < 0
		case "gt":
			failed = cmpIntBound(i, b.b) <= 0
		}
		if failed {
			return nil, errBound(boundType[b.key], b.raw)
		}
	}
	return i, nil
}

// ---- float ----

type floatValidator struct {
	strict      bool
	allowInfNan bool
	bounds      []numBound
}

func (v *floatValidator) name() string {
	if len(v.bounds) > 0 || !v.allowInfNan {
		return "constrained-float"
	}
	return "float"
}

func (v *floatValidator) validate(st *state, in any) (Value, *vErr) {
	var f float64
	switch x := in.(type) {
	case pyjson.Float:
		f = float64(x)
	case pyjson.Int:
		st.floor(strictMatch)
		conv, ok := intToFloat(x)
		if !ok {
			// float(int) raises OverflowError; measured as float_type.
			return nil, errSimple("float_type")
		}
		f = conv
	case pyjson.Bool:
		if v.strict {
			return nil, errSimple("float_type")
		}
		st.floor(lax)
		if x {
			f = 1
		}
	case pyjson.String:
		if v.strict {
			return nil, errSimple("float_type")
		}
		st.floor(lax)
		if hasSurrogate(string(x)) {
			return nil, errStringUnicode()
		}
		parsed, ok := strAsFloat(string(x))
		if !ok {
			return nil, errSimple("float_parsing")
		}
		f = parsed
	default:
		return nil, errSimple("float_type")
	}
	if !v.allowInfNan && (math.IsNaN(f) || math.IsInf(f, 0)) {
		return nil, errSimple("finite_number")
	}
	for _, b := range v.bounds {
		bf := b.b.f
		failed := false
		switch b.key {
		case "multiple_of":
			rem := math.Mod(f, bf)
			threshold := math.Abs(f) / 1e9
			failed = math.Abs(rem) > threshold && math.Abs(rem-bf) > threshold
		case "le":
			failed = !(f <= bf)
		case "lt":
			failed = !(f < bf)
		case "ge":
			failed = !(f >= bf)
		case "gt":
			failed = !(f > bf)
		}
		if failed {
			return nil, errBound(boundType[b.key], pyjson.Float(bf))
		}
	}
	return pyjson.Float(f), nil
}

// ---- bool ----

type boolValidator struct{ strict bool }

func (v *boolValidator) name() string { return "bool" }

func (v *boolValidator) validate(st *state, in any) (Value, *vErr) {
	if b, ok := in.(pyjson.Bool); ok {
		return b, nil
	}
	if v.strict {
		return nil, errSimple("bool_type")
	}
	// validate_bool: an int that fits i64 goes through int_as_bool; any other
	// number is tried as float_as_int first, and a float that is not an
	// integral i64 (0.5, NaN, 1e20, or an int beyond i64) is bool_type.
	switch x := in.(type) {
	case pyjson.Int:
		st.floor(lax)
		n, ok := x.Int64()
		if !ok {
			return nil, errSimple("bool_type")
		}
		return intAsBool(n)
	case pyjson.Float:
		st.floor(lax)
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || !(f > math.MinInt64 && f < math.MaxInt64) {
			return nil, errSimple("bool_type")
		}
		return intAsBool(int64(f))
	case pyjson.String:
		st.floor(lax)
		if hasSurrogate(string(x)) {
			return nil, errStringUnicode()
		}
		// str_as_bool compares with eq_ignore_ascii_case: only A-Z fold, so
		// U+017F LATIN SMALL LETTER LONG S never reads as "s".
		switch s := asciiLower(string(x)); s {
		case "0", "f", "n", "no", "off", "false":
			return pyjson.Bool(false), nil
		case "1", "t", "y", "on", "yes", "true":
			return pyjson.Bool(true), nil
		}
		return nil, errSimple("bool_parsing")
	}
	return nil, errSimple("bool_type")
}

func intAsBool(n int64) (Value, *vErr) {
	switch n {
	case 0:
		return pyjson.Bool(false), nil
	case 1:
		return pyjson.Bool(true), nil
	}
	return nil, errSimple("bool_parsing")
}

// ---- uuid ----

type uuidValidator struct{ strict bool }

func (v *uuidValidator) name() string { return "uuid" }

func (v *uuidValidator) validate(st *state, in any) (Value, *vErr) {
	s, ok := in.(pyjson.String)
	if !ok {
		return nil, errSimple("uuid_type")
	}
	if v.strict {
		return nil, fail("is_instance_of", "Input should be an instance of UUID", ctxOf("class", pyjson.String("UUID")))
	}
	st.floor(lax)
	if hasSurrogate(string(s)) {
		return nil, errStringUnicode()
	}
	u, text := parseUUID(string(s))
	if text != "" {
		return nil, errUUIDParsing(text)
	}
	return u, nil
}

// ---- literal ----

type literalValidator struct {
	expected []pyjson.Value
	repr     string
}

func (v *literalValidator) name() string {
	return "literal[" + strings.ReplaceAll(v.repr, " or ", ",") + "]"
}

// validate follows LiteralLookup::validate: exact bool, exact int, exact str
// (whose as_cow refuses surrogates), then a Python dict lookup, where 1, 1.0
// and True are the same key.
func (v *literalValidator) validate(st *state, in any) (Value, *vErr) {
	if b, ok := in.(pyjson.Bool); ok {
		for _, e := range v.expected {
			if eb, ok := e.(pyjson.Bool); ok && eb == b {
				return e, nil
			}
		}
	}
	if i, ok := in.(pyjson.Int); ok {
		for _, e := range v.expected {
			if ei, ok := e.(pyjson.Int); ok && ei.Big().Cmp(i.Big()) == 0 {
				return e, nil
			}
		}
	}
	if s, ok := in.(pyjson.String); ok {
		if hasSurrogate(string(s)) && v.hasString() {
			return nil, errStringUnicode()
		}
		for _, e := range v.expected {
			if es, ok := e.(pyjson.String); ok && es == s {
				return e, nil
			}
		}
		return nil, errLiteral(v.repr)
	}
	for _, e := range v.expected {
		switch e.(type) {
		case pyjson.Null:
			if isNone(in) {
				return e, nil
			}
		case pyjson.Int, pyjson.Bool:
			if c, ok := comparePy(in, e); ok && c == 0 {
				st.floor(lax)
				return e, nil
			}
		}
	}
	return nil, errLiteral(v.repr)
}

func (v *literalValidator) hasString() bool {
	for _, e := range v.expected {
		if _, ok := e.(pyjson.String); ok {
			return true
		}
	}
	return false
}

// ---- list ----

type listValidator struct {
	item     validator
	min, max int
}

func (v *listValidator) name() string {
	if v.item == nil {
		return "list[any]"
	}
	return "list[" + v.item.name() + "]"
}

func (v *listValidator) validate(st *state, in any) (Value, *vErr) {
	items, ok := in.(pyjson.List)
	if !ok {
		return nil, errSimple("list_type")
	}
	out := make(List, 0, len(items))
	var lines []lineError
	count := 0
	for idx, item := range items {
		var got Value = item
		var e *vErr
		if v.item != nil {
			got, e = v.item.validate(st, item)
			if e != nil && e.fatal != nil {
				return nil, e
			}
		}
		// MaxLengthCheck::incr runs for successes and failures alike, and
		// its error replaces every item error collected so far.
		count++
		if v.max >= 0 && count > v.max {
			return nil, errTooLong("List", v.max, len(items))
		}
		if e != nil {
			lines = append(lines, e.withOuter(idx).lines...)
			continue
		}
		out = append(out, got)
	}
	if len(lines) > 0 {
		return nil, &vErr{lines: lines}
	}
	if v.min >= 0 && len(out) < v.min {
		return nil, errTooShort("List", v.min, len(out))
	}
	return out, nil
}

// ---- dict ----

type dictValidator struct {
	keys, values validator
	min, max     int
}

func (v *dictValidator) name() string {
	k, val := "any", "any"
	if v.keys != nil {
		k = v.keys.name()
	}
	if v.values != nil {
		val = v.values.name()
	}
	return "dict[" + k + "," + val + "]"
}

func (v *dictValidator) validate(st *state, in any) (Value, *vErr) {
	m, ok := in.(*pyjson.OrderedMap)
	if !ok {
		return nil, errSimple("dict_type")
	}
	out := &Dict{}
	var lines []lineError
	for k, raw := range m.All() {
		var key Value = pyjson.String(k)
		keyOK, valOK := true, true
		if v.keys != nil {
			got, e := v.keys.validate(st, pyjson.String(k))
			if e != nil {
				if e.fatal != nil {
					return nil, e
				}
				lines = append(lines, e.withOuter("[key]").withOuter(k).lines...)
				keyOK = false
			} else {
				key = got
			}
		}
		var val Value = raw
		if v.values != nil {
			got, e := v.values.validate(st, raw)
			if e != nil {
				if e.fatal != nil {
					return nil, e
				}
				lines = append(lines, e.withOuter(k).lines...)
				valOK = false
			} else {
				val = got
			}
		}
		if keyOK && valOK {
			out.set(key, val)
		}
	}
	if len(lines) > 0 {
		return nil, &vErr{lines: lines}
	}
	if v.max >= 0 && out.Len() > v.max {
		return nil, errTooLong("Dictionary", v.max, out.Len())
	}
	if v.min >= 0 && out.Len() < v.min {
		return nil, errTooShort("Dictionary", v.min, out.Len())
	}
	return out, nil
}

// ---- nullable, default, any, none, ref ----

type nullableValidator struct{ inner validator }

func (v *nullableValidator) name() string { return "nullable[" + v.inner.name() + "]" }

func (v *nullableValidator) validate(st *state, in any) (Value, *vErr) {
	if isNone(in) {
		return pyjson.Null{}, nil
	}
	return v.inner.validate(st, in)
}

type defaultValidator struct {
	inner   validator
	value   Value
	factory func() Value
}

func (v *defaultValidator) name() string { return "default[" + v.inner.name() + "]" }

func (v *defaultValidator) validate(st *state, in any) (Value, *vErr) {
	return v.inner.validate(st, in)
}

func (v *defaultValidator) defaultValue() Value {
	if v.factory != nil {
		return v.factory()
	}
	return copyValue(v.value)
}

type anyValidator struct{}

func (anyValidator) name() string { return "any" }

func (anyValidator) validate(_ *state, in any) (Value, *vErr) {
	if in == nil {
		return pyjson.Null{}, nil
	}
	return in, nil
}

type noneValidator struct{}

func (noneValidator) name() string { return "none" }

func (noneValidator) validate(_ *state, in any) (Value, *vErr) {
	if isNone(in) {
		return pyjson.Null{}, nil
	}
	return nil, errSimple("none_required")
}

type refValidator struct {
	name_ string
	inner validator
}

func (v *refValidator) name() string {
	if v.inner == nil {
		return "..."
	}
	return v.inner.name()
}

func (v *refValidator) validate(st *state, in any) (Value, *vErr) {
	return v.inner.validate(st, in)
}

// ---- model ----

type modelValidator struct {
	class  string
	name_  string
	fields *modelFieldsValidator
}

func (v *modelValidator) name() string { return v.name_ }

// validate follows ModelValidator with from_attributes=True: a dict goes to
// the fields; builtins (str, bytes, list, int, None...) have no attributes to
// read and fail with model_attributes_type.
func (v *modelValidator) validate(st *state, in any) (Value, *vErr) {
	m, ok := asDict(in)
	if !ok {
		return nil, errSimple("model_attributes_type")
	}
	st.floor(lax)
	return v.fields.validateDict(st, m)
}

// dictSource is a Python dict a model validates: what json.loads produced,
// or the dict FastAPI builds from a form, whose values may be UploadFiles.
type dictSource interface {
	lookup(key string) (any, bool)
	keyList() []string
}

type jsonDict struct{ m *pyjson.OrderedMap }

func (j jsonDict) lookup(key string) (any, bool) {
	v, ok := j.m.Get(key)
	return v, ok
}

func (j jsonDict) keyList() []string { return j.m.Keys() }

func asDict(in any) (dictSource, bool) {
	switch x := in.(type) {
	case *pyjson.OrderedMap:
		return jsonDict{x}, true
	case *formDict:
		return x, true
	}
	return nil, false
}

type modelField struct {
	name string
	// alias is the field's alias when the model declares one.
	alias string
	v     validator
	def   *defaultValidator
}

type modelFieldsValidator struct {
	class  string
	fields []modelField
	extra  string
}

func (v *modelFieldsValidator) name() string { return "model-fields" }

func (v *modelFieldsValidator) validate(st *state, in any) (Value, *vErr) {
	m, ok := asDict(in)
	if !ok {
		return nil, errSimple("model_attributes_type")
	}
	return v.validateDict(st, m)
}

// validateDict is ModelFieldsValidator::validate over a dict: fields in
// declaration order (value error, or default, or missing), then, unless
// extra is ignored, every input key no field used, in input order.
func (v *modelFieldsValidator) validateDict(st *state, m dictSource) (Value, *vErr) {
	out := &Model{Class: v.class}
	data := map[string]Value{}
	var lines []lineError
	for _, f := range v.fields {
		raw, present := m.lookup(f.name)
		if present {
			prevData, prevField := st.data, st.fieldName
			st.data, st.fieldName = data, f.name
			got, e := f.v.validate(st, raw)
			st.data, st.fieldName = prevData, prevField
			if e != nil {
				if e.fatal != nil {
					return nil, e
				}
				lines = append(lines, e.withOuter(f.name).lines...)
				continue
			}
			data[f.name] = got
			out.Fields = append(out.Fields, Field{Name: f.name, Value: got})
			out.FieldsSet = append(out.FieldsSet, f.name)
			continue
		}
		if f.def != nil {
			dv := f.def.defaultValue()
			data[f.name] = dv
			out.Fields = append(out.Fields, Field{Name: f.name, Value: dv})
			continue
		}
		lines = append(lines, errMissing().withOuter(f.name).lines...)
	}
	if v.extra != "ignore" {
		for _, k := range m.keyList() {
			if hasSurrogate(k) {
				// `either_str.as_cow()?` returns at once, dropping the rest.
				return nil, errStringUnicode()
			}
			if v.isField(k) {
				continue
			}
			lines = append(lines, errExtraForbidden().withOuter(k).lines...)
		}
	}
	if len(lines) > 0 {
		return nil, &vErr{lines: lines}
	}
	st.fieldsSetCount = len(out.FieldsSet)
	return out, nil
}

func (v *modelFieldsValidator) isField(k string) bool {
	for _, f := range v.fields {
		if f.name == k {
			return true
		}
	}
	return false
}

// ---- union (smart mode) ----

type unionChoice struct {
	label string
	v     validator
}

type unionValidator struct{ choices []unionChoice }

func (v *unionValidator) name() string {
	names := make([]string, len(v.choices))
	for i, c := range v.choices {
		names[i] = c.v.name()
	}
	return "union[" + strings.Join(names, ",") + "]"
}

// validate is UnionValidator::validate_smart: an exact success without a
// model fields-set count wins at once; otherwise the success with the most
// fields set, then the best exactness, earliest first; with no success the
// errors of every choice, each under its label.
func (v *unionValidator) validate(st *state, in any) (Value, *vErr) {
	var best Value
	bestFound, bestExact, bestCount := false, -1, -1
	var lines []lineError
	for _, c := range v.choices {
		sub := &state{exactness: exact, fieldsSetCount: -1, data: st.data, fieldName: st.fieldName}
		got, e := c.v.validate(sub, in)
		if e == nil {
			if sub.exactness == exact && sub.fieldsSetCount < 0 {
				return got, nil
			}
			better := !bestFound
			if bestFound {
				if bestCount >= 0 && sub.fieldsSetCount >= 0 && bestCount != sub.fieldsSetCount {
					better = bestCount < sub.fieldsSetCount
				} else {
					better = bestExact < sub.exactness
				}
			}
			if better {
				best, bestFound, bestExact, bestCount = got, true, sub.exactness, sub.fieldsSetCount
			}
			continue
		}
		if e.fatal != nil {
			return nil, e
		}
		if !bestFound {
			label := c.label
			if label == "" {
				label = c.v.name()
			}
			lines = append(lines, e.withOuter(label).lines...)
		}
	}
	if bestFound {
		st.floor(bestExact)
		return best, nil
	}
	return nil, &vErr{lines: lines}
}

// ---- function validators ----

type funcValidator struct {
	mode      string
	name_     string
	fieldName string
	keywords  map[string]pyjson.Value
	fn        ValidatorFunc
	inner     validator
}

func (v *funcValidator) name() string {
	short := v.name_[strings.LastIndexByte(v.name_, '.')+1:]
	if v.inner == nil {
		return "function-" + v.mode + "[" + short + "()]"
	}
	return "function-" + v.mode + "[" + short + "(), " + v.inner.name() + "]"
}

func (v *funcValidator) call(st *state, in any) (Value, *vErr) {
	c := &Call{Name: v.name_, Mode: v.mode, Keywords: v.keywords, FieldName: v.fieldName, Data: st.data}
	if v.mode == "wrap" {
		c.Handler = func(x Value) (Value, error) {
			got, e := v.inner.validate(st, x)
			if e != nil {
				if e.fatal != nil {
					return nil, e.fatal
				}
				return nil, &wrappedLines{e}
			}
			return got, nil
		}
	}
	out, err := v.fn(c, in)
	if err == nil {
		return out, nil
	}
	var pe *Error
	var wl *wrappedLines
	switch {
	case errors.As(err, &pe):
		return nil, fail(pe.Type, pe.Msg, pe.Ctx)
	case errors.As(err, &wl):
		return nil, wl.e
	}
	return nil, fatal(err)
}

// wrappedLines carries the line errors of a wrap handler through a
// ValidatorFunc that returned them unchanged.
type wrappedLines struct{ e *vErr }

func (w *wrappedLines) Error() string { return "pyval: validation failed inside a wrap validator" }

func (v *funcValidator) validate(st *state, in any) (Value, *vErr) {
	switch v.mode {
	case "after":
		got, e := v.inner.validate(st, in)
		if e != nil {
			return nil, e
		}
		return v.call(st, got)
	case "before":
		got, e := v.call(st, in)
		if e != nil {
			return nil, e
		}
		return v.inner.validate(st, got)
	}
	return v.call(st, in)
}
