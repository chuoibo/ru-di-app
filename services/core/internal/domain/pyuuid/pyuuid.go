// Package pyuuid is `str(uuid.UUID(text))` of CPython 3.12, for ids that
// arrive inside larger strings. It is a copy of internal/cursors' parser,
// which keeps its own copy under the cursors oracle; photoref's oracle checks
// this one against the real app.domain.photo_ref.
package pyuuid

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Parse is uuid.UUID(text) followed by str(): "urn:" and "uuid:" removed
// everywhere, braces trimmed from both ends, hyphens removed, 32 code points,
// then int(text, 16) in [0, 2**128).
func Parse(text string) (string, bool) {
	s := strings.ReplaceAll(text, "urn:", "")
	s = strings.ReplaceAll(s, "uuid:", "")
	s = strings.Trim(s, "{}")
	s = strings.ReplaceAll(s, "-", "")
	if utf8.RuneCountInString(s) != 32 {
		return "", false
	}
	hi, lo, ok := parseHex128(decimalAndSpaceToASCII(s))
	if !ok {
		return "", false
	}
	const digits = "0123456789abcdef"
	var hex [32]byte
	for i := 31; i >= 0; i-- {
		hex[i] = digits[lo&0xF]
		lo = lo>>4 | hi<<60
		hi >>= 4
	}
	h := string(hex[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], true
}

// decimalAndSpaceToASCII is _PyUnicode_TransformDecimalAndSpaceToASCII: an
// ASCII string is returned unchanged; otherwise characters below U+007F are
// kept, whitespace becomes a space, a decimal digit becomes its ASCII digit,
// and the first other character becomes "?" and ends the text.
func decimalAndSpaceToASCII(s string) cstr {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return cstr(s)
	}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x7F:
			out = append(out, byte(r))
		case isPySpace(r):
			out = append(out, ' ')
		default:
			value := decimalValue(r)
			if value < 0 {
				return cstr(append(out, '?'))
			}
			out = append(out, byte('0'+value))
		}
	}
	return cstr(out)
}

// parseHex128 is PyLong_FromString(text, &end, 16) with PyLong_FromUnicodeObject's
// demand that it consumed the whole buffer, limited to values below 2**128
// (the caller has at most 32 digits). A negative non-zero value fails, as
// uuid refuses it.
func parseHex128(s cstr) (hi, lo uint64, ok bool) {
	p := 0
	for s.at(p) != 0 && isCSpace(s.at(p)) {
		p++
	}
	negative := false
	switch s.at(p) {
	case '+':
		p++
	case '-':
		p++
		negative = true
	}
	if s.at(p) == '0' && (s.at(p+1) == 'x' || s.at(p+1) == 'X') {
		p += 2
		if s.at(p) == '_' {
			p++
		}
	}
	start := p
	if s.at(start) == '_' {
		return 0, 0, false
	}
	var previous byte
	for {
		c := s.at(p)
		if value := hexValue(c); value < 16 {
			hi = hi<<4 | lo>>60
			lo = lo<<4 | uint64(value)
		} else if c == '_' {
			if previous == '_' {
				return 0, 0, false
			}
		} else {
			break
		}
		previous = c
		p++
	}
	if previous == '_' || p == start {
		return 0, 0, false
	}
	for s.at(p) != 0 && isCSpace(s.at(p)) {
		p++
	}
	if s.at(p) != 0 || p != len(s) {
		return 0, 0, false
	}
	if negative && (hi|lo) != 0 {
		return 0, 0, false
	}
	return hi, lo, true
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return 37
}

// isCSpace is Py_ISSPACE: the C locale's whitespace.
func isCSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

// isPySpace is str.isspace() for one code point. oracle_test.go checks it
// against CPython over every code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// decimalValue is Py_UNICODE_TODECIMAL: the digit a Unicode Nd character
// stands for, or -1. Nd characters come in runs of ten from zero, and Go 1.23
// and CPython 3.12 both read Unicode 15.0; oracle_test.go checks every code
// point.
func decimalValue(r rune) int {
	for _, span := range unicode.Nd.R16 {
		if lo, hi := rune(span.Lo), rune(span.Hi); r >= lo && r <= hi {
			return int(r-lo) % 10
		}
	}
	for _, span := range unicode.Nd.R32 {
		if lo, hi := rune(span.Lo), rune(span.Hi); r >= lo && r <= hi {
			return int(r-lo) % 10
		}
	}
	return -1
}

// cstr is the NUL-terminated UTF-8 buffer CPython's parsers walk: reading at
// or past the end yields NUL, and an embedded NUL stops them the same way.
type cstr string

func (s cstr) at(i int) byte {
	if i >= 0 && i < len(s) {
		return s[i]
	}
	return 0
}
