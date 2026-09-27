// Package preprocess is the engine's deterministic first stage: it cleans the
// text a person wrote, STRUCTURALLY, before any model sees it. No I/O, no
// model, and no reading of meaning: dates and times are the router's
// (hieu writes ISO values the engine lays on the calendar), and the keyword
// date reader that used to write the server's time facts from the words is
// gone (the owner's rule of 2026-09-25).
//
// Cleaning is NFC, the assistant's @mentions removed wherever they stand, the
// invisible characters removed (zero-width, bidi controls, tag characters --
// every Unicode format character, and control characters other than
// whitespace), and whitespace collapsed. The invisible characters are counted:
// they are how an instruction hides from a reader and from a pattern alike
// («i​gnore»), and the guard reads the cleaned text.
package preprocess

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/chatintent"
)

// KetQua is one cleaned text.
type KetQua struct {
	Chu string
	// KyTuAn counts the invisible characters removed.
	KyTuAn int
	// KhongDau: the text has letters and none of them carries a Vietnamese
	// diacritic («toi nay di dau»): a flag for the metrics row and the eval
	// only, never an input to any decision.
	KhongDau bool
}

var mention = func() *regexp.Regexp {
	var alts []string
	for _, m := range chatintent.Mentions() {
		parts := strings.Fields(m)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		alts = append(alts, strings.Join(parts, `\s*`))
	}
	return regexp.MustCompile(`(?i)(?:` + strings.Join(alts, "|") + `)`)
}()

func invisible(r rune) bool {
	if unicode.Is(unicode.Cf, r) {
		return true
	}
	return unicode.Is(unicode.Cc, r) && !unicode.IsSpace(r)
}

// LamSach cleans one piece of text a person (or a device) supplied.
func LamSach(s string) KetQua {
	var out KetQua
	var b strings.Builder
	for _, r := range norm.NFC.String(s) {
		if invisible(r) {
			out.KyTuAn++
			continue
		}
		b.WriteRune(r)
	}
	// NFC again: removing a character between a letter and its combining
	// mark can leave a sequence the first pass did not compose.
	text := norm.NFC.String(b.String())
	text = mention.ReplaceAllString(text, " ")
	out.Chu = strings.Join(strings.Fields(text), " ")
	coChu, coDau := false, false
	for _, r := range out.Chu {
		if unicode.IsLetter(r) {
			coChu = true
			if r > unicode.MaxASCII {
				coDau = true
			}
		}
	}
	out.KhongDau = coChu && !coDau
	return out
}
