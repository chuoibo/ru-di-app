package pyval

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Strings in this package are generalized UTF-8 (see package pyjson): a
// surrogate code point is the 3-byte sequence ED A0..BF xx.

// nextCodePoint decodes one code point of generalized UTF-8 at s[i:].
func nextCodePoint(s string, i int) (rune, int) {
	c := s[i]
	if c < utf8.RuneSelf {
		return rune(c), 1
	}
	if c == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2]&0xC0 == 0x80 {
		return 0xD000 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F), 3
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	return r, size
}

// codePoints counts code points, which is what Python len() and Rust
// str.chars().count() count.
func codePoints(s string) int {
	n := 0
	for i := 0; i < len(s); {
		_, size := nextCodePoint(s, i)
		i += size
		n++
	}
	return n
}

// hasSurrogate reports whether s holds a surrogate code point, which makes
// PyUnicode_AsUTF8 (and so every pydantic-core `as_cow`) fail.
func hasSurrogate(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == 0xED && s[i+1] >= 0xA0 && s[i+1] <= 0xBF {
			return true
		}
	}
	return false
}

// latin1 decodes raw bytes as ISO-8859-1, which is how Starlette turns
// header names, header values and the query string into str.
func latin1(raw string) string {
	for i := 0; i < len(raw); i++ {
		if raw[i] >= utf8.RuneSelf {
			var b strings.Builder
			b.Grow(len(raw) + 8)
			for j := 0; j < len(raw); j++ {
				b.WriteRune(rune(raw[j]))
			}
			return b.String()
		}
	}
	return raw
}

// isPySpace is str.isspace() for one code point.
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

// isRustSpace is char::is_whitespace (Unicode White_Space): Python's set
// without U+001C..U+001F.
func isRustSpace(r rune) bool {
	return isPySpace(r) && !(r >= 0x1C && r <= 0x1F)
}

func trimFunc(s string, space func(rune) bool) string {
	start := 0
	for start < len(s) {
		r, size := nextCodePoint(s, start)
		if !space(r) {
			break
		}
		start += size
	}
	end := len(s)
	for end > start {
		i := end - 1
		for i > start && s[i]&0xC0 == 0x80 {
			i--
		}
		r, _ := nextCodePoint(s, i)
		if !space(r) {
			break
		}
		end = i
	}
	return s[start:end]
}

// pyStrip is str.strip().
func pyStrip(s string) string { return trimFunc(s, isPySpace) }

// rustTrim is Rust's str::trim().
func rustTrim(s string) string { return trimFunc(s, isRustSpace) }

// asciiLower lowers A-Z only.
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// pyStrRepr is repr(str) in CPython 3.12.
func pyStrRepr(s string) string {
	quote := byte('\'')
	if strings.IndexByte(s, '\'') >= 0 && strings.IndexByte(s, '"') < 0 {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for i := 0; i < len(s); {
		r, size := nextCodePoint(s, i)
		i += size
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7F:
			b.WriteString(`\x`)
			b.WriteString(hex2(int(r)))
		case r < 0x7F:
			b.WriteRune(r)
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteString(`\u`)
			b.WriteString(strconv.FormatInt(int64(r), 16))
		case r == ' ' || unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xFF:
			b.WriteString(`\x`)
			b.WriteString(hex2(int(r)))
		case r <= 0xFFFF:
			b.WriteString(`\u`)
			b.WriteString(padHex(int(r), 4))
		default:
			b.WriteString(`\U`)
			b.WriteString(padHex(int(r), 8))
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func hex2(v int) string { return padHex(v, 2) }

func padHex(v, width int) string {
	h := strconv.FormatInt(int64(v), 16)
	for len(h) < width {
		h = "0" + h
	}
	return h
}

// decodeUTF8Replace is bytes.decode("utf-8", "replace"): one U+FFFD per
// maximal subpart of an ill-formed sequence (Unicode Table 3-7). The same
// algorithm lives unexported in internal/httpapi/router.
func decodeUTF8Replace(b []byte) string {
	var out strings.Builder
	out.Grow(len(b))
	for i := 0; i < len(b); {
		lead := b[i]
		if lead < 0x80 {
			out.WriteByte(lead)
			i++
			continue
		}
		need, lo, hi := sequenceShape(lead)
		if need == 0 {
			out.WriteRune(utf8.RuneError)
			i++
			continue
		}
		j := i + 1
		ok := true
		for k := 0; k < need; k++ {
			if j >= len(b) {
				ok = false
				break
			}
			c := b[j]
			low, high := byte(0x80), byte(0xBF)
			if k == 0 {
				low, high = lo, hi
			}
			if c < low || c > high {
				ok = false
				break
			}
			j++
		}
		if ok {
			out.Write(b[i:j])
		} else {
			out.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return out.String()
}

// sequenceShape returns how many continuation bytes a lead byte needs and
// the allowed range of the first one.
func sequenceShape(lead byte) (need int, lo, hi byte) {
	switch {
	case lead >= 0xC2 && lead <= 0xDF:
		return 1, 0x80, 0xBF
	case lead == 0xE0:
		return 2, 0xA0, 0xBF
	case lead >= 0xE1 && lead <= 0xEC, lead == 0xEE, lead == 0xEF:
		return 2, 0x80, 0xBF
	case lead == 0xED:
		return 2, 0x80, 0x9F
	case lead == 0xF0:
		return 3, 0x90, 0xBF
	case lead >= 0xF1 && lead <= 0xF3:
		return 3, 0x80, 0xBF
	case lead == 0xF4:
		return 3, 0x80, 0x8F
	}
	return 0, 0, 0
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	}
	return c - 'A' + 10
}

// pyUnquote is urllib.parse.unquote(s, "utf-8", "replace") for a str whose
// code points are latin-1: ASCII runs are percent-decoded as UTF-8, other
// code points pass through.
func pyUnquote(s string) string {
	if strings.IndexByte(s, '%') < 0 {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] >= utf8.RuneSelf {
			r, size := nextCodePoint(s, i)
			out.WriteRune(r)
			i += size
			continue
		}
		j := i
		for j < len(s) && s[j] < utf8.RuneSelf {
			j++
		}
		out.WriteString(decodeUTF8Replace(unquoteToBytes(s[i:j])))
		i = j
	}
	return out.String()
}

func unquoteToBytes(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2]) {
			out = append(out, unhex(s[i+1])<<4|unhex(s[i+2]))
			i += 2
			continue
		}
		out = append(out, s[i])
	}
	return out
}

// parseQSL is urllib.parse.parse_qsl(qs, keep_blank_values=True) over the
// latin-1 decoding of the raw query string, which is what Starlette's
// QueryParams holds.
func parseQSL(rawQuery string) [][2]string {
	qs := latin1(rawQuery)
	if qs == "" {
		return nil
	}
	var out [][2]string
	for _, nameValue := range strings.Split(qs, "&") {
		if nameValue == "" {
			continue
		}
		name, value, _ := strings.Cut(nameValue, "=")
		name = pyUnquote(strings.ReplaceAll(name, "+", " "))
		value = pyUnquote(strings.ReplaceAll(value, "+", " "))
		out = append(out, [2]string{name, value})
	}
	return out
}

// contentTypeIsJSON is the check get_request_handler makes on a non-empty
// content-type header value, through email.message.Message:
// `_splitparam(value)[0].strip().lower()`, text/plain unless it holds exactly
// one "/", then main type "application" and subtype "json" or "*+json".
func contentTypeIsJSON(value string) bool {
	head, _, _ := strings.Cut(latin1(value), ";")
	ctype := asciiLower(pyStrip(head))
	if strings.Count(ctype, "/") != 1 {
		return false
	}
	main, sub, _ := strings.Cut(ctype, "/")
	return main == "application" && (sub == "json" || strings.HasSuffix(sub, "+json"))
}
