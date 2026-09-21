package pyval

import (
	"strconv"
	"strings"
)

// parseUUID is uuid::Uuid::parse_str from the uuid crate that pydantic-core
// 2.46.5 links. On failure it returns the Display text of the crate's error,
// which pydantic-core puts in ctx.error of a uuid_parsing error.
//
// Accepted spellings: 32 hex digits; 8-4-4-4-12 with hyphens; that form in
// braces; and that form after a lowercase "urn:uuid:". Hex digits may be
// either case. Measured in the parity image, confirming the crate source.
func parseUUID(s string) (UUID, string) {
	var u UUID
	b := []byte(s)
	ok := false
	switch n := len(b); {
	case n == 32:
		ok = parseSimpleUUID(b, &u)
	case n == 36:
		ok = parseHyphenatedUUID(b, &u)
	case n == 38 && b[0] == '{' && b[37] == '}':
		ok = parseHyphenatedUUID(b[1:37], &u)
	case n == 45 && strings.HasPrefix(s, "urn:uuid:"):
		ok = parseHyphenatedUUID(b[9:], &u)
	}
	if ok {
		return u, ""
	}
	return u, uuidErrorText(s)
}

func parseSimpleUUID(b []byte, u *UUID) bool {
	for i := 0; i < 16; i++ {
		hi, lo := b[2*i], b[2*i+1]
		if !isHexDigit(hi) || !isHexDigit(lo) {
			return false
		}
		u[i] = unhex(hi)<<4 | unhex(lo)
	}
	return true
}

func parseHyphenatedUUID(b []byte, u *UUID) bool {
	if len(b) != 36 || b[8] != '-' || b[13] != '-' || b[18] != '-' || b[23] != '-' {
		return false
	}
	hexOnly := make([]byte, 0, 32)
	for i, c := range b {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		hexOnly = append(hexOnly, c)
	}
	return parseSimpleUUID(hexOnly, u)
}

// uuidErrorText is InvalidUuid::into_err followed by the error's Display.
func uuidErrorText(input string) string {
	uuidStr, offset, simple := input, 0, true
	switch {
	case len(input) >= 2 && input[0] == '{' && input[len(input)-1] == '}':
		uuidStr, offset, simple = input[1:len(input)-1], 1, false
	case strings.HasPrefix(input, "urn:uuid:"):
		uuidStr, offset, simple = input[len("urn:uuid:"):], len("urn:uuid:"), false
	}
	hyphens := 0
	var bounds [4]int
	for i := 0; i < len(uuidStr); {
		r, size := nextCodePoint(uuidStr, i)
		switch {
		case r == '-':
			if hyphens < 4 {
				bounds[hyphens] = i
			}
			hyphens++
		case r >= 0x80 || !isHexDigit(byte(r)):
			// Every code point before this one was ASCII, so the byte index
			// the crate reports equals the code point index.
			return "invalid character: found `" + string(r) + "` at " + strconv.Itoa(i+offset+1)
		}
		i += size
	}
	if hyphens == 0 && simple {
		return "invalid length: expected length 32 for simple format, found " + strconv.Itoa(len(input))
	}
	if hyphens != 4 {
		return "invalid group count: expected 5, found " + strconv.Itoa(hyphens+1)
	}
	blockStarts := [5]int{0, 9, 14, 19, 24}
	expected := [5]int{8, 4, 4, 4, 12}
	for g := 0; g < 4; g++ {
		if bounds[g] != blockStarts[g+1]-1 {
			return groupLengthText(g, expected[g], bounds[g]-blockStarts[g])
		}
	}
	return groupLengthText(4, expected[4], len(input)-blockStarts[4])
}

func groupLengthText(group, expected, found int) string {
	return "invalid group length in group " + strconv.Itoa(group) + ": expected " +
		strconv.Itoa(expected) + ", found " + strconv.Itoa(found)
}
