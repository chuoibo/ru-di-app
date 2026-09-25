// Package preprocess is the engine's deterministic first stage: it cleans the
// text a person wrote before any guard reads it or any model sees it, and
// writes the server's own facts about time. No I/O, no model.
//
// Cleaning is NFC, the assistant's @mentions removed wherever they stand, the
// invisible characters removed (zero-width, bidi controls, tag characters --
// every Unicode format character, and control characters other than
// whitespace), and whitespace collapsed. The invisible characters are counted:
// they are how an instruction hides from a reader and from a pattern alike
// («i​gnore»), and the guard reads the cleaned text.
package preprocess

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/chatintent"
	"mobile/services/core/internal/domain/thoigian"
)

// KetQua is one cleaned text.
type KetQua struct {
	Chu string
	// KyTuAn counts the invisible characters removed.
	KyTuAn int
	// KhongDau: the text has letters and none of them carries a Vietnamese
	// diacritic («toi nay di dau»), a signal for routing and the eval only.
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

// DongMayChu is the server's facts about time for one question: the «now»
// line from the instant the question was stored with (never the worker's
// clock), then one line per date or time the question mentions, resolved on
// Vietnam's wall clock. It also returns how many of those read two ways.
func DongMayChu(cauHoi string, luc time.Time) ([]string, int) {
	lines := []string{thoigian.DongBayGio(luc)}
	moHo := 0
	moc := thoigian.Giai(cauHoi, luc)
	if len(moc) > 0 {
		lines = append(lines, "Ngày giờ trong câu hỏi, máy chủ tính theo giờ Việt Nam:")
	}
	for _, m := range moc {
		if m.MoHo {
			moHo++
		}
		lines = append(lines, fmt.Sprintf("- «%s» là %s", m.Cum, m.MoTa()))
	}
	return lines, moHo
}
