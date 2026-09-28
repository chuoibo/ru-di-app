package traloi

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/tools"
)

// The prose path (a direct answer, the fast path's explain_screen, the agent
// loop) writes free prose rather than the structured answer of Chay. These
// two helpers prepare it for the same checks, reading delimiters only:
// place tokens are the markup of token.go, and sentences end where the
// punctuation ends them. Neither guesses what a word means.

// GhepVanXuoi renders the place tokens of prose: each [[p:alias]] becomes
// the name from the turn's ledger, an alias the ledger does not know
// becomes «một chỗ» (counted in la), a broken token is removed (counted in
// hong).
func GhepVanXuoi(s string, sc *tools.SoCai) (chu string, la, hong int) {
	ten := func(a string) (string, bool) {
		id, ok := sc.TuBiDanh(a)
		if !ok {
			return cau.MotCho, false
		}
		bc, _, _ := sc.Lay(id)
		if t := strings.TrimSpace(bc.Truong[TruongTen]); t != "" {
			return t, true
		}
		return cau.MotCho, true
	}
	var dong []string
	for _, d := range strings.Split(s, "\n") {
		p := tachCau(d)
		c, n := p.ghep(ten)
		la += n
		hong += p.hong
		dong = append(dong, c)
	}
	return strings.TrimSpace(strings.Join(dong, "\n")), la, hong
}

// TachCauVanXuoi splits prose into the numbered sentences the verifier
// judges: at a line break, and after a run of . ! ? … followed by a space.
// A dot between two digits («150.000») is not an end. Past
// kiemchung.MaxMenhDe sentences, the rest joins the last one, so every word
// is still judged.
func TachCauVanXuoi(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if t := strings.Join(strings.Fields(b.String()), " "); t != "" {
			out = append(out, t)
		}
		b.Reset()
	}
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		i += n
		if r == '\n' {
			flush()
			continue
		}
		b.WriteRune(r)
		if !laCuoiCau(r) {
			continue
		}
		// Take the whole run of end marks, then end only before a space.
		for i < len(s) {
			r2, n2 := utf8.DecodeRuneInString(s[i:])
			if !laCuoiCau(r2) {
				break
			}
			b.WriteRune(r2)
			i += n2
		}
		if i >= len(s) {
			break
		}
		if r2, _ := utf8.DecodeRuneInString(s[i:]); unicode.IsSpace(r2) {
			flush()
		}
	}
	flush()
	if len(out) > kiemchung.MaxMenhDe {
		du := strings.Join(out[kiemchung.MaxMenhDe-1:], " ")
		out = append(out[:kiemchung.MaxMenhDe-1], du)
	}
	return out
}

func laCuoiCau(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '…' }
