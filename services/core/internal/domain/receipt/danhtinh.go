package receipt

import (
	"strings"
	"unicode"
)

// identityFragments are the substrings of a key that name a person. A model
// reading money off a screenshot or a message has no channel for identity:
// who paid comes from the session and the message's author, never from it.
var identityFragments = []string{
	"paidby", "payer", "person", "people", "sharedby", "sharedwith", "participant",
	"attendee", "author", "advancer", "beneficiar", "member", "recipient",
	"splitwith", "splitbetween", "whopaid",
}

// LooksLikeIdentityKey is _looks_like_identity_key of screenshot.py and
// chat_expense.py: a key outside the contract that, case-folded and stripped
// to [a-z0-9], contains a person-shaped fragment.
func LooksLikeIdentityKey(key string, contract map[string]bool) bool {
	if contract[key] {
		return false
	}
	compact := compactFold(key)
	for _, f := range identityFragments {
		if strings.Contains(compact, f) {
			return true
		}
	}
	return false
}

// PyStrip is str.strip(), for the packages that read beside this one.
func PyStrip(s string) string { return pyStrip(s) }

// fullFold are the characters whose full case folding (str.casefold())
// yields ASCII letters that unicode.ToLower does not: only these can change
// what survives compaction to [a-z0-9].
var fullFold = map[rune]string{
	'ß': "ss", 'ẞ': "ss", 'ŉ': "\u02bcn", 'ǰ': "j\u030c", 'ẖ': "h\u0331", 'ẗ': "t\u0308",
	'ẘ': "w\u030a", 'ẙ': "y\u030a", 'ẚ': "a\u02be", 'ﬀ': "ff", 'ﬁ': "fi", 'ﬂ': "fl",
	'ﬃ': "ffi", 'ﬄ': "ffl", 'ﬅ': "st", 'ﬆ': "st", 'ſ': "s",
}

// compactFold is re.sub(r"[^a-z0-9]", "", key.casefold()).
func compactFold(key string) string {
	var b strings.Builder
	keep := func(r rune) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	for _, r := range key {
		if f, ok := fullFold[r]; ok {
			for _, x := range f {
				keep(x)
			}
			continue
		}
		keep(unicode.ToLower(r))
	}
	return b.String()
}
