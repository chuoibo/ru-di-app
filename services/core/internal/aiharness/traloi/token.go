package traloi

import (
	"strings"
	"unicode"
)

// The answer's prose has two pieces of MARKUP, both fixed by the answer
// format the instruction and schema define, both read by delimiter only:
//
//   - [[p:<alias>]] names a place; the prose never writes a place's name.
//     Go replaces the token with the name from the turn's ledger.
//   - «…» encloses a button label copied from the manual; «» is used for
//     nothing else.
//
// Reading these delimiters is parsing a format we defined, like reading
// JSON: it never guesses what a word means.

const (
	moToken   = "[["
	dongToken = "]]"
	tienTo    = "p:"
	moNhan    = "«"
	dongNhan  = "»"
)

// doan is one piece of a sentence: text, or a place token's alias.
type doan struct {
	chu    string
	biDanh string
	token  bool
}

// phanTich is a sentence split into pieces.
type phanTich struct {
	doan []doan
	// biDanh are the place tokens' aliases, in order, repeats kept.
	biDanh []string
	// hong counts broken tokens (unclosed, not a place, empty alias),
	// removed from the text.
	hong int
	// nhan are the «…» labels, in order.
	nhan []string
}

// tachCau splits a sentence into text and place tokens, and collects its
// button labels.
func tachCau(s string) phanTich {
	var p phanTich
	var chu strings.Builder
	flush := func() {
		if chu.Len() > 0 {
			p.doan = append(p.doan, doan{chu: chu.String()})
			chu.Reset()
		}
	}
	for s != "" {
		i := strings.Index(s, moToken)
		if i < 0 {
			chu.WriteString(s)
			break
		}
		chu.WriteString(s[:i])
		s = s[i+len(moToken):]
		j := strings.Index(s, dongToken)
		if j < 0 {
			// Unclosed: drop the rest of its word.
			p.hong++
			k := strings.IndexFunc(s, unicode.IsSpace)
			if k < 0 {
				s = ""
			} else {
				s = s[k:]
			}
			continue
		}
		trong := s[:j]
		s = s[j+len(dongToken):]
		a, ok := strings.CutPrefix(trong, tienTo)
		if !ok || a == "" || strings.ContainsFunc(a, func(r rune) bool { return unicode.IsSpace(r) || r == '[' || r == ']' }) {
			p.hong++
			continue
		}
		flush()
		p.doan = append(p.doan, doan{biDanh: a, token: true})
		p.biDanh = append(p.biDanh, a)
	}
	flush()
	p.nhan = nhanTrong(p.chuMoHinh())
	return p
}

// chuMoHinh is the text the model wrote around its tokens (tokens left
// out): what the privacy format check and the label markup read.
func (p phanTich) chuMoHinh() string {
	var b strings.Builder
	for _, d := range p.doan {
		if !d.token {
			b.WriteString(d.chu)
		} else {
			b.WriteString(" ")
		}
	}
	return b.String()
}

// nhanTrong returns every «…» span of s, trimmed; an unclosed « is text.
func nhanTrong(s string) []string {
	var out []string
	for {
		i := strings.Index(s, moNhan)
		if i < 0 {
			return out
		}
		s = s[i+len(moNhan):]
		j := strings.Index(s, dongNhan)
		if j < 0 {
			return out
		}
		out = append(out, strings.TrimSpace(s[:j]))
		s = s[j+len(dongNhan):]
	}
}

// ghep renders the sentence, each token replaced by ten(alias); ok false
// counts the token as naming no evidence of the turn.
func (p phanTich) ghep(ten func(alias string) (string, bool)) (string, int) {
	var b strings.Builder
	la := 0
	for _, d := range p.doan {
		if !d.token {
			b.WriteString(d.chu)
			continue
		}
		t, ok := ten(d.biDanh)
		if !ok {
			la++
		}
		b.WriteString(t)
	}
	return strings.Join(strings.Fields(b.String()), " "), la
}
