package auth

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

var errNotUUID = errors.New("not a UUID")

// ParsePythonUUID parses s the way Python's uuid.UUID(s) does and returns the
// canonical lowercase form. s is treated as latin-1: one byte, one character,
// which is how Starlette decodes header values.
//
// uuid.UUID removes "urn:" and then "uuid:" anywhere in the text, strips any
// run of braces from both ends, deletes every hyphen, requires exactly 32
// characters, and hands those to int(..., 16).
func ParsePythonUUID(s string) (string, error) {
	s = strings.ReplaceAll(s, "urn:", "")
	s = strings.ReplaceAll(s, "uuid:", "")
	s = strings.Trim(s, "{}")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return "", errNotUUID
	}
	value, ok := pythonHexInt(s)
	if !ok {
		return "", errNotUUID
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	if value.Sign() < 0 || value.Cmp(limit) >= 0 {
		return "", errNotUUID
	}
	hex := fmt.Sprintf("%032x", value)
	return hex[:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:], nil
}

// pythonHexInt is CPython's int(s, 16) for latin-1 text: surrounding
// whitespace, an optional sign, an optional 0x prefix (followed by at most one
// underscore), then hex digits with single underscores only between digits.
//
// int() does not strip what str.strip() strips. It maps non-ASCII whitespace
// (NEL, NBSP) to a space and then skips only C-locale isspace characters, so
// \x1c-\x1f, which str.isspace() calls whitespace, make the text invalid here.
func pythonHexInt(s string) (*big.Int, bool) {
	start, end := 0, len(s)
	for start < end && isIntSpace(s[start]) {
		start++
	}
	for end > start && isIntSpace(s[end-1]) {
		end--
	}
	s = s[start:end]
	negative := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		negative = s[0] == '-'
		s = s[1:]
	}
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
		if s != "" && s[0] == '_' {
			s = s[1:]
		}
	}
	if s == "" || s[0] == '_' || s[len(s)-1] == '_' {
		return nil, false
	}
	var digits strings.Builder
	previousUnderscore := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
			if previousUnderscore {
				return nil, false
			}
			previousUnderscore = true
		case isHexDigit(c):
			digits.WriteByte(c)
			previousUnderscore = false
		default:
			return nil, false
		}
	}
	value, ok := new(big.Int).SetString(digits.String(), 16)
	if !ok {
		return nil, false
	}
	if negative {
		value.Neg(value)
	}
	return value, true
}

// isIntSpace is the whitespace int() skips around its digits, in latin-1.
func isIntSpace(b byte) bool {
	switch b {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xa0:
		return true
	}
	return false
}

func isHexDigit(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// pyStripLatin1 is str.strip() over latin-1 text: Python's whitespace in that
// range includes \x1c-\x1f, NEL (0x85) and NBSP (0xa0).
func pyStripLatin1(s string) string {
	start, end := 0, len(s)
	for start < end && isPythonSpace(s[start]) {
		start++
	}
	for end > start && isPythonSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isPythonSpace(b byte) bool {
	switch b {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return false
}
