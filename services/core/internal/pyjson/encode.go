package pyjson

import (
	"bytes"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"
)

// Compact returns the body Starlette's JSONResponse renders:
//
//	json.dumps(v, ensure_ascii=False, allow_nan=False, indent=None,
//	           separators=(",", ":")).encode("utf-8")
//
// NaN and infinities fail with *ValueError; surrogate code points fail with
// *UnicodeEncodeError, as the final .encode does.
func Compact(v Value) ([]byte, error) {
	e := encoder{itemSep: ",", keySep: ":"}
	return e.run(v)
}

// Dumps returns json.dumps(v) with every default: separators ", " and ": ",
// ensure_ascii=True, allow_nan=True. The output is ASCII.
func Dumps(v Value) ([]byte, error) {
	e := encoder{itemSep: ", ", keySep: ": ", ensureASCII: true, allowNaN: true}
	return e.run(v)
}

// Canonical returns the idempotency fingerprint body:
//
//	json.dumps(v, sort_keys=True, separators=(",", ":"),
//	           ensure_ascii=False).encode("utf-8")
//
// NaN and infinities are written as NaN, Infinity and -Infinity. Surrogate
// code points fail with *UnicodeEncodeError, as the final .encode does.
func Canonical(v Value) ([]byte, error) {
	e := encoder{itemSep: ",", keySep: ":", allowNaN: true, sortKeys: true}
	return e.run(v)
}

// FloatRepr returns Python's repr(f): the shortest digits that round-trip,
// scientific notation when the decimal exponent is below -4 or at least 16,
// and "nan", "inf", "-inf" for the non-finite values.
func FloatRepr(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	return string(appendFloatRepr(nil, f))
}

type encoder struct {
	buf         []byte
	itemSep     string
	keySep      string
	ensureASCII bool
	allowNaN    bool
	sortKeys    bool
	depth       int
	surrogates  bool
}

func (e *encoder) run(v Value) ([]byte, error) {
	if err := e.value(v); err != nil {
		return nil, err
	}
	if e.surrogates && !e.ensureASCII {
		return nil, surrogateError(e.buf)
	}
	return e.buf, nil
}

func (e *encoder) value(v Value) error {
	switch x := v.(type) {
	case nil, Null:
		e.buf = append(e.buf, "null"...)
	case Bool:
		if x {
			e.buf = append(e.buf, "true"...)
		} else {
			e.buf = append(e.buf, "false"...)
		}
	case Int:
		return e.int(x)
	case Float:
		return e.float(float64(x))
	case String:
		return e.string(string(x))
	case List:
		if err := e.enter(); err != nil {
			return err
		}
		e.buf = append(e.buf, '[')
		for i, item := range x {
			if i > 0 {
				e.buf = append(e.buf, e.itemSep...)
			}
			if err := e.value(item); err != nil {
				return err
			}
		}
		e.buf = append(e.buf, ']')
		e.depth--
	case *OrderedMap:
		return e.object(x)
	default:
		panic("pyjson: unsupported Value implementation")
	}
	return nil
}

func (e *encoder) enter() error {
	e.depth++
	if e.depth > MaxDepth {
		return &RecursionError{Msg: "maximum recursion depth exceeded while encoding a JSON object"}
	}
	return nil
}

func (e *encoder) object(m *OrderedMap) error {
	if err := e.enter(); err != nil {
		return err
	}
	e.buf = append(e.buf, '{')
	order := make([]int, m.Len())
	for i := range order {
		order[i] = i
	}
	if e.sortKeys {
		// Byte order of generalized UTF-8 is code point order, which is how
		// Python compares str.
		sort.Slice(order, func(a, b int) bool { return m.keys[order[a]] < m.keys[order[b]] })
	}
	for n, i := range order {
		if n > 0 {
			e.buf = append(e.buf, e.itemSep...)
		}
		if err := e.string(m.keys[i]); err != nil {
			return err
		}
		e.buf = append(e.buf, e.keySep...)
		if err := e.value(m.vals[i]); err != nil {
			return err
		}
	}
	e.buf = append(e.buf, '}')
	e.depth--
	return nil
}

func (e *encoder) int(i Int) error {
	if i.big == nil {
		e.buf = strconv.AppendInt(e.buf, i.small, 10)
		return nil
	}
	text := i.big.String()
	digits := len(text)
	if text[0] == '-' {
		digits--
	}
	if digits > IntMaxStrDigits {
		return &ValueError{Msg: "Exceeds the limit (4300 digits) for integer string conversion; use sys.set_int_max_str_digits() to increase the limit"}
	}
	e.buf = append(e.buf, text...)
	return nil
}

func (e *encoder) float(f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if !e.allowNaN {
			return &ValueError{Msg: "Out of range float values are not JSON compliant: " + FloatRepr(f)}
		}
		switch {
		case math.IsNaN(f):
			e.buf = append(e.buf, "NaN"...)
		case f > 0:
			e.buf = append(e.buf, "Infinity"...)
		default:
			e.buf = append(e.buf, "-Infinity"...)
		}
		return nil
	}
	e.buf = appendFloatRepr(e.buf, f)
	return nil
}

// appendFloatRepr lays out Go's shortest round-trip digits the way CPython's
// format_float_short does for repr (mode 'r', Py_DTSF_ADD_DOT_0).
func appendFloatRepr(dst []byte, f float64) []byte {
	var tmp [32]byte
	s := strconv.AppendFloat(tmp[:0], f, 'e', -1, 64) // [-]d[.ddd]e±dd
	if s[0] == '-' {
		dst = append(dst, '-')
		s = s[1:]
	}
	epos := bytes.IndexByte(s, 'e')
	exp, _ := strconv.Atoi(string(s[epos+1:]))
	var digitBuf [24]byte
	digits := append(digitBuf[:0], s[0])
	if epos > 1 {
		digits = append(digits, s[2:epos]...)
	}
	decpt := exp + 1
	nd := len(digits)
	if decpt <= -4 || decpt > 16 {
		dst = append(dst, digits[0])
		if nd > 1 {
			dst = append(dst, '.')
			dst = append(dst, digits[1:]...)
		}
		dst = append(dst, 'e')
		if exp < 0 {
			dst = append(dst, '-')
			exp = -exp
		} else {
			dst = append(dst, '+')
		}
		if exp < 10 {
			dst = append(dst, '0')
		}
		return strconv.AppendInt(dst, int64(exp), 10)
	}
	switch {
	case decpt <= 0:
		dst = append(dst, '0', '.')
		for i := 0; i < -decpt; i++ {
			dst = append(dst, '0')
		}
		dst = append(dst, digits...)
	case decpt >= nd:
		dst = append(dst, digits...)
		for i := nd; i < decpt; i++ {
			dst = append(dst, '0')
		}
		dst = append(dst, '.', '0')
	default:
		dst = append(dst, digits[:decpt]...)
		dst = append(dst, '.')
		dst = append(dst, digits[decpt:]...)
	}
	return dst
}

const hexDigits = "0123456789abcdef"

func (e *encoder) string(s string) error {
	e.buf = append(e.buf, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && !(c == 0x7f && e.ensureASCII) {
				i++
				continue
			}
			e.buf = append(e.buf, s[start:i]...)
			e.buf = appendASCIIEscape(e.buf, c)
			i++
			start = i
			continue
		}
		r, size := decodeCodePoint(s[i:])
		if size == 0 {
			return &InvalidStringError{Offset: i}
		}
		if isSurrogate(r) {
			e.surrogates = true
		}
		if e.ensureASCII {
			e.buf = append(e.buf, s[start:i]...)
			if r >= 0x10000 {
				r -= 0x10000
				e.buf = appendUnicodeEscape(e.buf, 0xD800|(r>>10))
				e.buf = appendUnicodeEscape(e.buf, 0xDC00|(r&0x3FF))
			} else {
				e.buf = appendUnicodeEscape(e.buf, r)
			}
			start = i + size
		}
		i += size
	}
	e.buf = append(e.buf, s[start:]...)
	e.buf = append(e.buf, '"')
	return nil
}

func appendASCIIEscape(dst []byte, c byte) []byte {
	switch c {
	case '"':
		return append(dst, '\\', '"')
	case '\\':
		return append(dst, '\\', '\\')
	case '\b':
		return append(dst, '\\', 'b')
	case '\f':
		return append(dst, '\\', 'f')
	case '\n':
		return append(dst, '\\', 'n')
	case '\r':
		return append(dst, '\\', 'r')
	case '\t':
		return append(dst, '\\', 't')
	}
	return appendUnicodeEscape(dst, rune(c))
}

func appendUnicodeEscape(dst []byte, r rune) []byte {
	return append(dst, '\\', 'u',
		hexDigits[r>>12&0xF], hexDigits[r>>8&0xF], hexDigits[r>>4&0xF], hexDigits[r&0xF])
}

func isSurrogate(r rune) bool { return r >= 0xD800 && r <= 0xDFFF }

// decodeCodePoint decodes one generalized UTF-8 code point. size is 0 when
// s does not start with one.
func decodeCodePoint(s string) (rune, int) {
	r, size := utf8.DecodeRuneInString(s)
	if size > 1 || (size == 1 && r != utf8.RuneError) {
		return r, size
	}
	if len(s) >= 3 && s[0] == 0xED && s[1] >= 0xA0 && s[1] <= 0xBF && s[2]&0xC0 == 0x80 {
		return 0xD000 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F), 3
	}
	return 0, 0
}

// appendCodePoint encodes r as generalized UTF-8.
func appendCodePoint(dst []byte, r rune) []byte {
	if isSurrogate(r) {
		return append(dst, 0xED, byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
	}
	return utf8.AppendRune(dst, r)
}

// surrogateError locates the first run of surrogates in an encoded document,
// counting code points like CPython's UTF-8 encoder does.
func surrogateError(doc []byte) error {
	s := string(doc)
	pos := 0
	for i := 0; i < len(s); pos++ {
		r, size := decodeCodePoint(s[i:])
		i += size
		if !isSurrogate(r) {
			continue
		}
		end := pos + 1
		for i < len(s) {
			r2, size2 := decodeCodePoint(s[i:])
			if !isSurrogate(r2) {
				break
			}
			i += size2
			end++
		}
		return &UnicodeEncodeError{Start: pos, End: end, First: r}
	}
	return &UnicodeEncodeError{}
}
