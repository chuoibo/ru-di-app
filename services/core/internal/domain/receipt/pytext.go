package receipt

import (
	"strings"
	"unicode"
)

// Python's str methods and `re` classes differ from Go's at the edges, and
// the reader's output is read by exactly those rules (money law 1: an amount
// is parsed or refused, never approximated). These helpers are the Python
// definitions, not Go's.

// pySpace is str.isspace(): the characters Python's `\s`, str.split() and
// str.strip() treat as whitespace. Go's unicode.IsSpace misses U+001C..U+001F.
func pySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, ' ', 0x85, 0xa0, 0x1680,
		0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// pyStrip is str.strip().
func pyStrip(s string) string { return strings.TrimFunc(s, pySpace) }

// pyDigit is `\d` in a str pattern: a Unicode decimal digit (Nd).
func pyDigit(r rune) bool { return unicode.Is(unicode.Nd, r) }

// digitValue is int() of one Nd character: its offset in its run of ten.
// Every Nd range in Unicode is made of complete runs that start at zero.
func digitValue(r rune) int {
	start := r
	for pyDigit(start - 1) {
		start--
	}
	return int(r-start) % 10
}

// allDigits reports whether s is `\d+`.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !pyDigit(r) {
			return false
		}
	}
	return true
}

// collapseSpace is re.sub(r"\s+", " ", s).
func collapseSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if pySpace(r) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// dropSpace is re.sub(r"\s+", "", s).
func dropSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if pySpace(r) {
			return -1
		}
		return r
	}, s)
}
