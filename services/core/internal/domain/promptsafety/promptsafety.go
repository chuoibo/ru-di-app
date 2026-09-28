// Package promptsafety is app.places.prompt_safety: which catalogue rows a
// model may see.
package promptsafety

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/tree"
)

const (
	maxName    = 120
	maxAddress = 200
	maxItem    = 60
)

var instruction = regexp.MustCompile(
	`bo qua .{0,20}huong dan|quen .{0,20}huong dan|lam theo .{0,20}sau day|` +
		`tra loi .{0,20}hop|ignore .{0,30}(instruction|prompt|above|previous)|` +
		`disregard .{0,30}(instruction|prompt|above|previous)|system prompt|` +
		`you are (now|a) |</?(system|assistant|user)>|` + "```",
)

func fold(text string) string {
	nfd := norm.NFD.String(text)
	var b strings.Builder
	for _, r := range nfd {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch r {
		case 'đ', 'Đ':
			b.WriteByte('d')
		default:
			b.WriteRune(r)
		}
	}
	collapsed := whitespace.ReplaceAllString(b.String(), " ")
	return strings.ToLower(collapsed)
}

// whitespace is the pattern fold collapses, compiled once instead of on every
// call; the same expression, so the same result (the oracle goldens hold it).
var whitespace = regexp.MustCompile(`\s+`)

// Fold is the normalisation every pattern here reads: NFD with the marks
// dropped, đ as d, whitespace collapsed, lower case. Exported for the AI
// engine's guard, unchanged: this package is an oracle port.
func Fold(text string) string { return fold(text) }

// LooksLikeInstruction is the instruction catalogue alone, on folded text:
// what field_is_safe refuses a field for, without the length and control
// character checks. The AI engine's guard runs it on every untrusted source.
func LooksLikeInstruction(text string) bool {
	return instruction.FindString(fold(text)) != ""
}

func fieldSafe(value tree.Value, maxChars int) bool {
	if value == nil {
		return true
	}
	if _, ok := value.(tree.Null); ok {
		return true
	}
	text, ok := value.(tree.String)
	if !ok {
		return true
	}
	s := string(text)
	if utf8.RuneCountInString(s) > maxChars {
		return false
	}
	for _, r := range s {
		if unicode.Is(unicode.Cc, r) {
			return false
		}
	}
	return instruction.FindString(fold(s)) == ""
}

// TextSafe is field_is_safe for one piece of free text that is not a catalogue
// row: the same length bound, control-character refusal and instruction
// catalogue a place name gets. Chat assist uses it on members' display names,
// which people type themselves and which now reach the model.
func TextSafe(s string, maxChars int) bool {
	return fieldSafe(tree.String(s), maxChars)
}

func listSafe(value tree.Value) bool {
	if value == nil {
		return true
	}
	if _, ok := value.(tree.Null); ok {
		return true
	}
	list, ok := value.(tree.List)
	if !ok || len(list) > 20 {
		return false
	}
	for _, item := range list {
		if !fieldSafe(item, maxItem) {
			return false
		}
	}
	return true
}

// Safe is place_is_safe_for_prompt.
func Safe(place *tree.OrderedMap) bool {
	if place == nil {
		return false
	}
	name, _ := place.Get("name")
	address, _ := place.Get("address")
	hours, _ := place.Get("open_hours")
	if !fieldSafe(name, maxName) || !fieldSafe(address, maxAddress) || !fieldSafe(hours, maxItem) {
		return false
	}
	kinds, _ := place.Get("kinds")
	traits, _ := place.Get("traits")
	return listSafe(kinds) && listSafe(traits)
}

// Filter is safe_places: the rows a prompt may quote, in arrival order.
func Filter(places []*tree.OrderedMap) []*tree.OrderedMap {
	out := []*tree.OrderedMap{}
	for _, place := range places {
		if Safe(place) {
			out = append(out, place)
		}
	}
	return out
}
