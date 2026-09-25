package tuvung

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/promptsafety"
)

// Breaks between two syllables, weakest first.
const (
	ngatKhong   uint8 = iota // a blank, a hyphen, a quote: the same clause
	ngatHaiCham              // ':' -- «Món chay: không có» is one statement
	ngatVe                   // , ( ) – — …: a new clause
	ngatCau                  // . ! ? ; a line break: a new sentence
)

// cau is a text cut for reading: the folded syllables AmTiet returns, the
// same syllables as typed (lower case, marks kept) and the strongest break
// that precedes each. Folding merges «cá» and «cả», «trứng» and «trung»;
// the raw forms tell them apart where the writer typed the marks.
type cau struct {
	s    []string
	raw  []string
	ngat []uint8
	// coDau: some letter of the text carries a Vietnamese mark (or is đ), so
	// a syllable typed without one was meant without one.
	coDau bool
}

func doNgat(r rune) uint8 {
	switch r {
	case '.', '!', '?', ';', '\n', '\r':
		return ngatCau
	case ',', '(', ')', '[', ']', '{', '}', '–', '—', '…', '•', '·', '|':
		return ngatVe
	case ':':
		return ngatHaiCham
	}
	return ngatKhong
}

// catCau cuts text. Its syllables are AmTiet(text), one for one; if an odd
// input ever made the two disagree, the raw forms fall back to the folded
// ones and the breaks to none, which only makes the readers below stricter.
func catCau(text string) cau {
	want := AmTiet(text)
	var c cau
	var tok []rune
	pending := ngatKhong
	flush := func() {
		if len(tok) == 0 {
			return
		}
		raw := string(tok)
		tok = tok[:0]
		folded := promptsafety.Fold(raw)
		if folded == "" {
			return
		}
		c.s = append(c.s, folded)
		c.raw = append(c.raw, strings.ToLower(norm.NFC.String(raw)))
		c.ngat = append(c.ngat, pending)
		pending = ngatKhong
	}
	for _, r := range norm.NFC.String(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) {
			tok = append(tok, r)
			continue
		}
		flush()
		if b := doNgat(r); b > pending {
			pending = b
		}
	}
	flush()
	if len(c.s) != len(want) {
		return cau{s: want, raw: append([]string(nil), want...), ngat: make([]uint8, len(want))}
	}
	for i := range want {
		if c.s[i] != want[i] {
			return cau{s: want, raw: append([]string(nil), want...), ngat: make([]uint8, len(want))}
		}
	}
	c.coDau = !KhongDau(text) && hasLetter(text)
	return c
}

func hasLetter(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// coDauTieng reports whether a raw syllable carries a Vietnamese mark or đ.
func coDauTieng(raw string) bool { return promptsafety.Fold(raw) != raw }

// ngatTruoc reports whether a break of at least b separates syllable i from
// syllable i-1.
func (c cau) ngatTruoc(i int, b uint8) bool { return i > 0 && i < len(c.ngat) && c.ngat[i] >= b }
