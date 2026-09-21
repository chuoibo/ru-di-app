package taste

import (
	"strings"
	"unicode"
)

// casefold is CPython's str.casefold(): the full case folding of
// CaseFolding.txt (statuses C and F), falling back to the full lowercase
// mapping. Go's unicode package only offers the simple mappings, which differ
// at 298 code points under Unicode 15.0 (Python 3.12 and Go 1.23 both ship
// it); oracle_test.go checks this function against CPython's answer for every
// code point.
func casefold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		foldRune(&b, r)
	}
	return b.String()
}

// Casefold is casefold for the other domain packages that compare text as
// CPython's str.casefold() does (internal/domain/expense).
func Casefold(s string) string { return casefold(s) }

func foldRune(b *strings.Builder, r rune) {
	switch {
	case r >= 0x13A0 && r <= 0x13F5:
		// Cherokee capitals are the fold target; ToLower would lower them.
		b.WriteRune(r)
	case r >= 0x13F8 && r <= 0x13FD:
		b.WriteRune(r - 8)
	case r >= 0xAB70 && r <= 0xABBF:
		b.WriteRune(r - 0xAB70 + 0x13A0)
	case r >= 0x1F80 && r <= 0x1FAF:
		// Greek vowels with ypogegrammeni or prosgegrammeni: the base vowel
		// with its breathing and accent, then a separate iota.
		base := [3]rune{0x1F00, 0x1F20, 0x1F60}[(r-0x1F80)/0x10]
		b.WriteRune(base + (r-0x1F80)%8)
		b.WriteRune(0x03B9)
	default:
		if folded, ok := foldSpecial[r]; ok {
			b.WriteString(folded)
			return
		}
		b.WriteRune(unicode.ToLower(r))
	}
}

// foldSpecial holds the remaining code points where str.casefold() is not
// unicode.ToLower.
var foldSpecial = map[rune]string{
	0x00B5: "μ",
	0x00DF: "ss",
	0x0130: "i̇",
	0x0149: "ʼn",
	0x017F: "s",
	0x01F0: "ǰ",
	0x0345: "ι",
	0x0390: "ΐ",
	0x03B0: "ΰ",
	0x03C2: "σ",
	0x03D0: "β",
	0x03D1: "θ",
	0x03D5: "φ",
	0x03D6: "π",
	0x03F0: "κ",
	0x03F1: "ρ",
	0x03F5: "ε",
	0x0587: "եւ",
	0x1C80: "в",
	0x1C81: "д",
	0x1C82: "о",
	0x1C83: "с",
	0x1C84: "т",
	0x1C85: "т",
	0x1C86: "ъ",
	0x1C87: "ѣ",
	0x1C88: "ꙋ",
	0x1E96: "ẖ",
	0x1E97: "ẗ",
	0x1E98: "ẘ",
	0x1E99: "ẙ",
	0x1E9A: "aʾ",
	0x1E9B: "ṡ",
	0x1E9E: "ss",
	0x1F50: "ὐ",
	0x1F52: "ὒ",
	0x1F54: "ὔ",
	0x1F56: "ὖ",
	0x1FB2: "ὰι",
	0x1FB3: "αι",
	0x1FB4: "άι",
	0x1FB6: "ᾶ",
	0x1FB7: "ᾶι",
	0x1FBC: "αι",
	0x1FBE: "ι",
	0x1FC2: "ὴι",
	0x1FC3: "ηι",
	0x1FC4: "ήι",
	0x1FC6: "ῆ",
	0x1FC7: "ῆι",
	0x1FCC: "ηι",
	0x1FD2: "ῒ",
	0x1FD3: "ΐ",
	0x1FD6: "ῖ",
	0x1FD7: "ῗ",
	0x1FE2: "ῢ",
	0x1FE3: "ΰ",
	0x1FE4: "ῤ",
	0x1FE6: "ῦ",
	0x1FE7: "ῧ",
	0x1FF2: "ὼι",
	0x1FF3: "ωι",
	0x1FF4: "ώι",
	0x1FF6: "ῶ",
	0x1FF7: "ῶι",
	0x1FFC: "ωι",
	0xFB00: "ff",
	0xFB01: "fi",
	0xFB02: "fl",
	0xFB03: "ffi",
	0xFB04: "ffl",
	0xFB05: "st",
	0xFB06: "st",
	0xFB13: "մն",
	0xFB14: "մե",
	0xFB15: "մի",
	0xFB16: "վն",
	0xFB17: "մխ",
}

// isPySpace is CPython's str.isspace() for one code point: bidirectional
// class WS, B or S, or category Zs. Unlike unicode.IsSpace it includes
// U+001C..U+001F. oracle_test.go checks it against every code point.
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

// pyStrip is str.strip() with no argument.
func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }
