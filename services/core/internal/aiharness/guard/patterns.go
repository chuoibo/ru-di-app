// Package guard is the engine's output guard: STRUCTURAL checks on a text
// before a byte of it leaves the engine -- the canary marker, a quoted
// clause of the instruction, and the data formats of privacy (an email, a
// phone number, a bank account or card number; DinhDang runs only the
// formats). It reads no meaning from words. The money law, the injection
// word patterns and the phrase rules about claimed actions that used to
// live here were removed by the owner's rule of 2026-09-25: the router
// (hieu) decides money and injection, the verifier (kiemchung) judges
// claimed actions and money in an answer.
//
// Text is compared after normalisation: NFKC (fullwidth and mathematical
// letters to plain), Cyrillic and Greek look-alikes to Latin, then
// promptsafety.Fold (no marks, đ as d, lower case), so a quoted clause of
// the instruction is found however it is typed.
package guard

import (
	"strings"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/promptsafety"
)

// homoglyph maps look-alike letters to the Latin letter they imitate.
var homoglyph = strings.NewReplacer(
	// Cyrillic
	"а", "a", "е", "e", "о", "o", "р", "p", "с", "c", "у", "y", "х", "x", "і", "i", "ј", "j", "ѕ", "s", "һ", "h", "ԁ", "d", "ԛ", "q", "ԝ", "w", "к", "k", "м", "m", "н", "h", "т", "t", "в", "b",
	"А", "A", "В", "B", "Е", "E", "К", "K", "М", "M", "Н", "H", "О", "O", "Р", "P", "С", "C", "Т", "T", "Х", "X", "У", "Y", "І", "I", "Ј", "J", "Ѕ", "S",
	// Greek
	"α", "a", "ε", "e", "ι", "i", "κ", "k", "ν", "v", "ο", "o", "ρ", "p", "τ", "t", "υ", "u", "χ", "x",
	"Α", "A", "Β", "B", "Ε", "E", "Ζ", "Z", "Η", "H", "Ι", "I", "Κ", "K", "Μ", "M", "Ν", "N", "Ο", "O", "Ρ", "P", "Τ", "T", "Υ", "Y", "Χ", "X",
)

// Gap is the normalised form the echo check compares.
func Gap(s string) string {
	return promptsafety.Fold(homoglyph.Replace(norm.NFKC.String(s)))
}
