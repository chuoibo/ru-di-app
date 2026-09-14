package pyjson

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"
)

// Loads is CPython 3.12 json.loads(data) for a bytes argument, which is what
// both the idempotency fingerprint and Starlette's Request.json() call.
//
// The bytes are decoded first, in the encoding json.detect_encoding picks
// (UTF-8, UTF-8 with BOM, UTF-16 or UTF-32), with the 'surrogatepass' error
// handler; failure is *UnicodeDecodeError. The text is then scanned like the
// C accelerator: NaN, Infinity and -Infinity are accepted, a duplicate key
// takes the last value but keeps the first position, ints are exact (beyond
// IntMaxStrDigits digits is *ValueError), floats are float64, and malformed
// JSON is *DecodeError carrying CPython's msg and pos. Nesting deeper than
// MaxDepth is *RecursionError.
//
// Lone surrogates, from \uD800-style escapes or from surrogate bytes passed
// through, are kept as surrogate code points in generalized UTF-8.
func Loads(data []byte) (Value, error) {
	doc, err := decodeText(data)
	if err != nil {
		return nil, err
	}
	d := decoder{s: doc}
	idx := d.skipSpace(0)
	v, end, err := d.scanOnce(idx)
	if err != nil {
		return nil, err
	}
	end = d.skipSpace(end)
	if end != len(doc) {
		return nil, d.fail("Extra data", end)
	}
	return v, nil
}

// decodeText is bytes.decode(json.detect_encoding(b), "surrogatepass").
func decodeText(b []byte) ([]rune, error) {
	switch {
	case bytes.HasPrefix(b, []byte{0, 0, 0xFE, 0xFF}):
		return decodeUTF32(b[4:], 4, true, "utf-32")
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE, 0, 0}):
		return decodeUTF32(b[4:], 4, false, "utf-32")
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return decodeUTF16(b[2:], 2, true, "utf-16")
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return decodeUTF16(b[2:], 2, false, "utf-16")
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return decodeUTF8(b[3:], 3, "utf-8-sig")
	}
	if len(b) >= 4 {
		if b[0] == 0 {
			if b[1] != 0 {
				return decodeUTF16(b, 0, true, "utf-16-be")
			}
			return decodeUTF32(b, 0, true, "utf-32-be")
		}
		if b[1] == 0 {
			if b[2] != 0 || b[3] != 0 {
				return decodeUTF16(b, 0, false, "utf-16-le")
			}
			return decodeUTF32(b, 0, false, "utf-32-le")
		}
	} else if len(b) == 2 {
		if b[0] == 0 {
			return decodeUTF16(b, 0, true, "utf-16-be")
		}
		if b[1] == 0 {
			return decodeUTF16(b, 0, false, "utf-16-le")
		}
	}
	return decodeUTF8(b, 0, "utf-8")
}

func decodeUTF8(b []byte, base int, enc string) ([]rune, error) {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		if c := b[i]; c < utf8.RuneSelf {
			out = append(out, rune(c))
			i++
			continue
		}
		r, size := decodeCodePoint(string(b[i:min(len(b), i+utf8.UTFMax)]))
		if size == 0 {
			return nil, &UnicodeDecodeError{Encoding: enc, Offset: base + i, Reason: "invalid utf-8"}
		}
		out = append(out, r)
		i += size
	}
	return out, nil
}

func decodeUTF16(b []byte, base int, bigEndian bool, enc string) ([]rune, error) {
	if len(b)%2 != 0 {
		return nil, &UnicodeDecodeError{Encoding: enc, Offset: base + len(b) - 1, Reason: "truncated data"}
	}
	unit := func(i int) rune {
		if bigEndian {
			return rune(b[i])<<8 | rune(b[i+1])
		}
		return rune(b[i+1])<<8 | rune(b[i])
	}
	out := make([]rune, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		u := unit(i)
		if u >= 0xD800 && u <= 0xDBFF && i+3 < len(b) {
			if u2 := unit(i + 2); u2 >= 0xDC00 && u2 <= 0xDFFF {
				out = append(out, 0x10000+(u-0xD800)<<10+(u2-0xDC00))
				i += 2
				continue
			}
		}
		// A lone surrogate passes through ('surrogatepass').
		out = append(out, u)
	}
	return out, nil
}

func decodeUTF32(b []byte, base int, bigEndian bool, enc string) ([]rune, error) {
	if len(b)%4 != 0 {
		return nil, &UnicodeDecodeError{Encoding: enc, Offset: base + len(b) - len(b)%4, Reason: "truncated data"}
	}
	out := make([]rune, 0, len(b)/4)
	for i := 0; i < len(b); i += 4 {
		var u uint32
		if bigEndian {
			u = uint32(b[i])<<24 | uint32(b[i+1])<<16 | uint32(b[i+2])<<8 | uint32(b[i+3])
		} else {
			u = uint32(b[i+3])<<24 | uint32(b[i+2])<<16 | uint32(b[i+1])<<8 | uint32(b[i])
		}
		if u > 0x10FFFF {
			return nil, &UnicodeDecodeError{Encoding: enc, Offset: base + i, Reason: "code point not in range(0x110000)"}
		}
		out = append(out, rune(u))
	}
	return out, nil
}

// decoder follows Modules/_json.c (scan_once_unicode and friends) index for
// index, so error positions agree with CPython.
type decoder struct {
	s     []rune
	depth int
}

func (d *decoder) fail(msg string, pos int) error {
	line, lastNL := 1, -1
	for i := 0; i < pos && i < len(d.s); i++ {
		if d.s[i] == '\n' {
			line++
			lastNL = i
		}
	}
	return &DecodeError{Msg: msg, Pos: pos, Line: line, Col: pos - lastNL}
}

func isSpace(c rune) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isDigit(c rune) bool { return c >= '0' && c <= '9' }

func (d *decoder) skipSpace(idx int) int {
	for idx < len(d.s) && isSpace(d.s[idx]) {
		idx++
	}
	return idx
}

// hasWord reports whether word starts at idx, with the same bounds check as
// the C scanner (idx + len(word) - 1 < length).
func (d *decoder) hasWord(idx int, word string) bool {
	if idx+len(word)-1 >= len(d.s) {
		return false
	}
	for i := 0; i < len(word); i++ {
		if d.s[idx+i] != rune(word[i]) {
			return false
		}
	}
	return true
}

// pythonNaN is float('nan') in CPython: the quiet NaN with an empty payload.
var pythonNaN = math.Float64frombits(0x7FF8_0000_0000_0000)

func (d *decoder) scanOnce(idx int) (Value, int, error) {
	if idx < 0 || idx >= len(d.s) {
		return nil, 0, d.fail("Expecting value", idx)
	}
	switch d.s[idx] {
	case '"':
		str, end, err := d.scanString(idx + 1)
		if err != nil {
			return nil, 0, err
		}
		return String(str), end, nil
	case '{':
		d.depth++
		if d.depth > MaxDepth {
			return nil, 0, &RecursionError{Msg: "maximum recursion depth exceeded while decoding a JSON object from a unicode string"}
		}
		v, end, err := d.parseObject(idx + 1)
		d.depth--
		return v, end, err
	case '[':
		d.depth++
		if d.depth > MaxDepth {
			return nil, 0, &RecursionError{Msg: "maximum recursion depth exceeded while decoding a JSON array from a unicode string"}
		}
		v, end, err := d.parseArray(idx + 1)
		d.depth--
		return v, end, err
	case 'n':
		if d.hasWord(idx, "null") {
			return Null{}, idx + 4, nil
		}
	case 't':
		if d.hasWord(idx, "true") {
			return Bool(true), idx + 4, nil
		}
	case 'f':
		if d.hasWord(idx, "false") {
			return Bool(false), idx + 5, nil
		}
	case 'N':
		if d.hasWord(idx, "NaN") {
			return Float(pythonNaN), idx + 3, nil
		}
	case 'I':
		if d.hasWord(idx, "Infinity") {
			return Float(math.Inf(1)), idx + 8, nil
		}
	case '-':
		if d.hasWord(idx, "-Infinity") {
			return Float(math.Inf(-1)), idx + 9, nil
		}
	}
	return d.matchNumber(idx)
}

func (d *decoder) matchNumber(start int) (Value, int, error) {
	s := d.s
	endIdx := len(s) - 1
	idx := start
	if s[idx] == '-' {
		idx++
		if idx > endIdx {
			return nil, 0, d.fail("Expecting value", start)
		}
	}
	switch {
	case s[idx] >= '1' && s[idx] <= '9':
		idx++
		for idx <= endIdx && isDigit(s[idx]) {
			idx++
		}
	case s[idx] == '0':
		idx++
	default:
		return nil, 0, d.fail("Expecting value", start)
	}
	isFloat := false
	if idx < endIdx && s[idx] == '.' && isDigit(s[idx+1]) {
		isFloat = true
		idx += 2
		for idx <= endIdx && isDigit(s[idx]) {
			idx++
		}
	}
	if idx < endIdx && (s[idx] == 'e' || s[idx] == 'E') {
		eStart := idx
		idx++
		if idx < endIdx && (s[idx] == '-' || s[idx] == '+') {
			idx++
		}
		for idx <= endIdx && isDigit(s[idx]) {
			idx++
		}
		if isDigit(s[idx-1]) {
			isFloat = true
		} else {
			idx = eStart
		}
	}
	text := make([]byte, idx-start)
	for i := range text {
		text[i] = byte(s[start+i])
	}
	if isFloat {
		// ParseFloat reports overflow as ±Inf with ErrRange, which is what
		// CPython's float() returns without an error.
		f, err := strconv.ParseFloat(string(text), 64)
		if err != nil && !isRangeError(err) {
			return nil, 0, fmt.Errorf("pyjson: unexpected float syntax %q: %w", text, err)
		}
		return Float(f), idx, nil
	}
	digits := len(text)
	if text[0] == '-' {
		digits--
	}
	if digits > IntMaxStrDigits {
		return nil, 0, &ValueError{Msg: fmt.Sprintf("Exceeds the limit (%d digits) for integer string conversion: value has %d digits; use sys.set_int_max_str_digits() to increase the limit", IntMaxStrDigits, digits)}
	}
	n, ok := ParseInt(string(text))
	if !ok {
		return nil, 0, fmt.Errorf("pyjson: unexpected int syntax %q", text)
	}
	return n, idx, nil
}

func isRangeError(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

func hexValue(c rune) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// scanString is scanstring_unicode with strict=True; end is the index just
// after the opening quote.
func (d *decoder) scanString(end int) (string, int, error) {
	s := d.s
	n := len(s)
	begin := end - 1
	var out []byte
	for {
		var c rune
		next := end
		for ; next < n; next++ {
			c = s[next]
			if c == '"' || c == '\\' {
				break
			}
			if c <= 0x1f {
				return "", 0, d.fail("Invalid control character at", next)
			}
		}
		if next >= n {
			return "", 0, d.fail("Unterminated string starting at", begin)
		}
		for _, r := range s[end:next] {
			out = appendCodePoint(out, r)
		}
		next++
		if c == '"' {
			end = next
			break
		}
		if next == n {
			return "", 0, d.fail("Unterminated string starting at", begin)
		}
		c = s[next]
		if c != 'u' {
			end = next + 1
			switch c {
			case '"', '\\', '/':
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			default:
				c = 0
			}
			if c == 0 {
				return "", 0, d.fail("Invalid \\escape", end-2)
			}
		} else {
			c = 0
			next++
			end = next + 4
			if end >= n {
				return "", 0, d.fail("Invalid \\uXXXX escape", next-1)
			}
			for ; next < end; next++ {
				h, ok := hexValue(s[next])
				if !ok {
					return "", 0, d.fail("Invalid \\uXXXX escape", end-5)
				}
				c = c<<4 | h
			}
			if c >= 0xD800 && c <= 0xDBFF && end+6 < n && s[next] == '\\' && s[next+1] == 'u' {
				next += 2
				end += 6
				var c2 rune
				for ; next < end; next++ {
					h, ok := hexValue(s[next])
					if !ok {
						return "", 0, d.fail("Invalid \\uXXXX escape", end-5)
					}
					c2 = c2<<4 | h
				}
				if c2 >= 0xDC00 && c2 <= 0xDFFF {
					c = 0x10000 + (c-0xD800)<<10 + (c2 - 0xDC00)
				} else {
					end -= 6
				}
			}
		}
		out = appendCodePoint(out, c)
	}
	return string(out), end, nil
}

func (d *decoder) parseObject(idx int) (Value, int, error) {
	s := d.s
	endIdx := len(s) - 1
	m := NewOrderedMap()
	idx = d.skipSpace(idx)
	if idx > endIdx || s[idx] != '}' {
		for {
			if idx > endIdx || s[idx] != '"' {
				return nil, 0, d.fail("Expecting property name enclosed in double quotes", idx)
			}
			key, next, err := d.scanString(idx + 1)
			if err != nil {
				return nil, 0, err
			}
			idx = d.skipSpace(next)
			if idx > endIdx || s[idx] != ':' {
				return nil, 0, d.fail("Expecting ':' delimiter", idx)
			}
			idx = d.skipSpace(idx + 1)
			val, next, err := d.scanOnce(idx)
			if err != nil {
				return nil, 0, err
			}
			m.Set(key, val)
			idx = d.skipSpace(next)
			if idx <= endIdx && s[idx] == '}' {
				break
			}
			if idx > endIdx || s[idx] != ',' {
				return nil, 0, d.fail("Expecting ',' delimiter", idx)
			}
			idx = d.skipSpace(idx + 1)
		}
	}
	return m, idx + 1, nil
}

func (d *decoder) parseArray(idx int) (Value, int, error) {
	s := d.s
	endIdx := len(s) - 1
	list := List{}
	idx = d.skipSpace(idx)
	if idx > endIdx || s[idx] != ']' {
		for {
			val, next, err := d.scanOnce(idx)
			if err != nil {
				return nil, 0, err
			}
			list = append(list, val)
			idx = d.skipSpace(next)
			if idx <= endIdx && s[idx] == ']' {
				break
			}
			if idx > endIdx || s[idx] != ',' {
				return nil, 0, d.fail("Expecting ',' delimiter", idx)
			}
			idx = d.skipSpace(idx + 1)
		}
	}
	return list, idx + 1, nil
}
