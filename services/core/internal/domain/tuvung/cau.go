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
// same syllables as typed (lower case, marks kept), the strongest break
// that precedes each and the characters between each and the next. Folding
// merges «cá» and «cả», «trứng» and «trung»; the raw forms tell them apart
// where the writer typed the marks.
type cau struct {
	s    []string
	raw  []string
	ngat []uint8
	// sau[i] is what stands between syllable i and the next one (or the
	// end of the text): «Quán chay (tạm ngưng)» has « (» after «chay».
	sau []string
	// coDau: some letter of the text carries a Vietnamese mark (or is đ), so
	// a syllable typed without one was meant without one. A place row sets
	// it for all its fields at once (CoDau).
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
	var tok, gap []rune
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
		if len(c.s) > 0 {
			c.sau[len(c.s)-1] = string(gap)
		}
		// «...» is a pause («dị ứng... ừm... sò điệp»), like «…», not the
		// end of a sentence.
		if g := string(gap); pending == ngatCau && strings.Count(g, ".") >= 2 && !strings.ContainsAny(g, "!?;\n\r") {
			pending = ngatVe
		}
		gap = gap[:0]
		c.s = append(c.s, folded)
		c.raw = append(c.raw, strings.ToLower(norm.NFC.String(raw)))
		c.ngat = append(c.ngat, pending)
		c.sau = append(c.sau, "")
		pending = ngatKhong
	}
	for _, r := range norm.NFC.String(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) {
			tok = append(tok, r)
			continue
		}
		flush()
		gap = append(gap, r)
		if b := doNgat(r); b > pending {
			pending = b
		}
	}
	flush()
	if len(c.s) > 0 {
		c.sau[len(c.s)-1] = string(gap)
	}
	if len(c.s) != len(want) {
		return cau{s: want, raw: append([]string(nil), want...), ngat: make([]uint8, len(want)), sau: make([]string, len(want))}
	}
	for i := range want {
		if c.s[i] != want[i] {
			return cau{s: want, raw: append([]string(nil), want...), ngat: make([]uint8, len(want)), sau: make([]string, len(want))}
		}
	}
	c.coDau = CoDau(text)
	return c
}

// CoDau reports whether any of texts has a letter that carries a
// Vietnamese mark or is đ. A place row is read with the answer for all its
// texts together: a bare «cua» among marked words was typed without marks
// on purpose, even when its own field is one word long.
func CoDau(texts ...string) bool {
	for _, text := range texts {
		if !KhongDau(text) && hasLetter(text) {
			return true
		}
	}
	return false
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

// boThanh drops the five tone marks of a raw syllable and keeps the rest
// (ă â ê ô ơ ư đ): «sửa», «sứa» and «sữa» all give «sưa», «mẻ» and «mè»
// give «me», but «của» gives «cua» and «cửa» gives «cưa».
func boThanh(raw string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(raw) {
		switch r {
		case '̀', '́', '̃', '̉', '̣':
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}

// ngatTruoc reports whether a break of at least b separates syllable i from
// syllable i-1.
func (c cau) ngatTruoc(i int, b uint8) bool { return i > 0 && i < len(c.ngat) && c.ngat[i] >= b }

// gachSau reports whether syllable i is followed by an opening parenthesis,
// a dash (a spaced hyphen, an en or em dash) or a question mark: what comes
// next qualifies or questions it, as in «Quán chay (tạm ngưng)», «Món chay
// – sắp có», «Vegetarian? No.». A hyphen inside a word («vegetarian-
// friendly») is not a dash.
func (c cau) gachSau(i int) bool {
	if i < 0 || i >= len(c.sau) {
		return false
	}
	gap := c.sau[i]
	if strings.ContainsAny(gap, "(–—?") {
		return true
	}
	return strings.Contains(gap, "-") && strings.TrimSpace(gap) != gap
}
