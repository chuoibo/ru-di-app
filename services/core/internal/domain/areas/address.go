package areas

import (
	"strings"
	"unicode"
)

// namedHCM is _NAMED_HCM, checked in this order: the first needle present
// wins, whatever its position in the address.
var namedHCM = [...]struct{ needle, id string }{
	{"phú nhuận", "hcm-phu-nhuan"},
	{"bình thạnh", "hcm-binh-thanh"},
	{"thủ đức", "hcm-thu-duc"},
}

// AreaOfAddress is area_of_address: the area a catalogue address names.
//
// The address must be valid UTF-8, as every Python str read from Postgres
// text is. Matching works on code points, as Python's does.
func AreaOfAddress(address string) (Area, bool) {
	if address == "" {
		return Area{}, false
	}
	lowered := pyLower(address)
	if strings.Contains(lowered, "đà lạt") {
		return Find("da-lat")
	}
	if strings.Contains(lowered, "hcm") || strings.Contains(lowered, "hồ chí minh") {
		if digits, ok := searchHCMDistrict(address); ok {
			return Find("hcm-quan-" + digits)
		}
		for _, named := range namedHCM {
			if strings.Contains(lowered, named.needle) {
				return Find(named.id)
			}
		}
	}
	return Area{}, false
}

// pyLower is str.lower() for the purpose of the substring tests above.
//
// Python applies full case mapping: U+0130 becomes "i" followed by U+0307,
// which Go's simple mapping would shorten to "i" and so let "MİNH" contain
// "minh". Every other code point maps to its simple lowercase, except that
// Python turns a word-final capital sigma into U+03C2 rather than U+03C3.
// Neither sigma occurs in any needle, so that difference cannot change a
// containment answer and is not reproduced.
func pyLower(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == 0x130 {
			b.WriteString("i̇")
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// searchHCMDistrict is _HCM_DISTRICT.search(address), returning group(1).
//
// The pattern is Qu[âậ]n\s+(\d+)\b under re.IGNORECASE with str semantics.
// Greedy matching needs no backtracking here: giving back a space makes \d
// face a space, and giving back a digit puts \b between two word characters.
// So a start position matches exactly when the maximal digit run after the
// maximal space run is followed by the end or by a non-word character.
func searchHCMDistrict(address string) (string, bool) {
	runes := []rune(address)
	n := len(runes)
	for start := 0; start+4 <= n; start++ {
		if !caseFoldsTo(runes[start], 'q') || !caseFoldsTo(runes[start+1], 'u') ||
			!(caseFoldsTo(runes[start+2], 'â') || caseFoldsTo(runes[start+2], 'ậ')) ||
			!caseFoldsTo(runes[start+3], 'n') {
			continue
		}
		spaces := start + 4
		for spaces < n && isPySpace(runes[spaces]) {
			spaces++
		}
		if spaces == start+4 {
			continue
		}
		end := spaces
		for end < n && isPyDecimal(runes[end]) {
			end++
		}
		if end == spaces {
			continue
		}
		if end == n || !isPyWord(runes[end]) {
			return string(runes[spaces:end]), true
		}
	}
	return "", false
}

// caseFoldsTo is sre's LITERAL_UNI_IGNORE and IN_UNI_IGNORE test for a
// lowercase pattern character: the subject's lowercase equals it.
func caseFoldsTo(r, lower rune) bool { return unicode.ToLower(r) == lower }

// isPyDecimal is \d for a str pattern: Py_UNICODE_ISDECIMAL, category Nd.
func isPyDecimal(r rune) bool { return unicode.Is(unicode.Nd, r) }

// isPySpace is \s for a str pattern: Py_UNICODE_ISSPACE. Unlike
// unicode.IsSpace it includes the separators U+001C to U+001F.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d, r >= 0x1c && r <= 0x20:
		return true
	case r == 0x85, r == 0xa0, r == 0x1680, r == 0x2028, r == 0x2029,
		r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	}
	return false
}

// isPyWord is the word-character test behind \b: Py_UNICODE_ISALNUM or '_'.
func isPyWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}
