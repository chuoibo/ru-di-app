package router

import (
	"strings"
	"unicode/utf8"
)

// SplitTarget splits an origin-form request-target the way httptools'
// parse_url does: the path ends at the first "?" or "#", the query runs from
// that "?" to the next "#", and the fragment is dropped.
func SplitTarget(target string) (rawPath, rawQuery string) {
	end := strings.IndexAny(target, "?#")
	if end < 0 {
		return target, ""
	}
	rawPath = target[:end]
	if target[end] == '#' {
		return rawPath, ""
	}
	rawQuery = target[end+1:]
	if i := strings.IndexByte(rawQuery, '#'); i >= 0 {
		rawQuery = rawQuery[:i]
	}
	return rawPath, rawQuery
}

// ScopePath is scope["path"] as uvicorn's httptools protocol builds it from a
// raw path: urllib.parse.unquote, applied only when the path contains "%".
func ScopePath(rawPath string) string {
	if !strings.Contains(rawPath, "%") {
		return rawPath
	}
	return decodeUTF8Replace(unquoteToBytes(rawPath))
}

// targetBytesOK reports whether llhttp accepts every byte of a target:
// printable ASCII only, no space, no DEL.
func targetBytesOK(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// unquoteToBytes mirrors urllib.parse.unquote_to_bytes: "%" followed by two
// hex digits is a byte, anything else after "%" stays literal.
func unquoteToBytes(s string) []byte {
	bits := strings.Split(s, "%")
	out := make([]byte, 0, len(s))
	out = append(out, bits[0]...)
	for _, item := range bits[1:] {
		if len(item) >= 2 && isHex(item[0]) && isHex(item[1]) {
			out = append(out, unhex(item[0])<<4|unhex(item[1]))
			out = append(out, item[2:]...)
			continue
		}
		out = append(out, '%')
		out = append(out, item...)
	}
	return out
}

// decodeUTF8Replace mirrors bytes.decode("utf-8", "replace") in CPython, which
// emits one U+FFFD per maximal subpart of an ill-formed sequence (Unicode
// Table 3-7). Go's decoder emits one per byte instead, so it cannot be used.
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
		n := 1
		for ; n <= need; n++ {
			if i+n >= len(b) {
				break
			}
			low, high := byte(0x80), byte(0xbf)
			if n == 1 {
				low, high = lo, hi
			}
			if b[i+n] < low || b[i+n] > high {
				break
			}
		}
		if n > need {
			out.Write(b[i : i+need+1])
			i += need + 1
			continue
		}
		out.WriteRune(utf8.RuneError)
		i += n
	}
	return out.String()
}

// sequenceShape returns how many continuation bytes a lead byte needs and the
// allowed range of the first one; need is 0 for a byte that cannot lead.
func sequenceShape(lead byte) (need int, lo, hi byte) {
	switch {
	case lead >= 0xc2 && lead <= 0xdf:
		return 1, 0x80, 0xbf
	case lead == 0xe0:
		return 2, 0xa0, 0xbf
	case lead >= 0xe1 && lead <= 0xec, lead == 0xee, lead == 0xef:
		return 2, 0x80, 0xbf
	case lead == 0xed:
		return 2, 0x80, 0x9f
	case lead == 0xf0:
		return 3, 0x90, 0xbf
	case lead >= 0xf1 && lead <= 0xf3:
		return 3, 0x80, 0xbf
	case lead == 0xf4:
		return 3, 0x80, 0x8f
	}
	return 0, 0, 0
}

// quoteSafe is RedirectResponse's safe set; letters, digits and "_.-~" are
// always safe in urllib.parse.quote.
const quoteSafe = ":/%#?=@[]!$&'()*+,;"

// quote mirrors urllib.parse.quote(s, safe=quoteSafe) on a UTF-8 string.
func quote(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var out strings.Builder
	out.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 && (isAlnum(c) || strings.IndexByte("_.-~"+quoteSafe, c) >= 0) {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hexDigits[c>>4])
		out.WriteByte(hexDigits[c&0x0f])
	}
	return out.String()
}

// latin1 reinterprets each byte as the code point of the same value.
func latin1(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		out.WriteRune(rune(s[i]))
	}
	return out.String()
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}
