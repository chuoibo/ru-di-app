// Package chatintent is app.domain.chat_intent: what a chat message asks
// the companion to do. Pure: text in, a small struct out, no I/O.
package chatintent

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/taste"
)

// Intent names.
const (
	Plan     = "plan"
	ChiaBill = "chia_bill"
	Vote     = "vote"
	Mention  = "mention"
)

var commands = []struct {
	token  string
	intent string
}{
	{"/plan", Plan},
	{"/chia-bill", ChiaBill},
	{"/chiabill", ChiaBill},
	{"/vote", Vote},
	{"/binh-chon", Vote},
}

var mentions = []string{"@rủ đi", "@rudi", "@ru di"}

const (
	maxQuestion = 300
	maxOption   = 200
	minOptions  = 2
	maxOptions  = 20
)

// Parsed is ParsedIntent.
type Parsed struct {
	Intent string
	Args   string
}

// VoteSpec is VoteSpec.
type VoteSpec struct {
	Question string
	Options  []string
}

func nfc(text string) string { return norm.NFC.String(text) }

func isPySpace(r rune) bool {
	return unicode.IsSpace(r) || r == 0x1C || r == 0x1D || r == 0x1E || r == 0x1F
}

// Parse is parse_intent: the command or mention in body, or nil.
func Parse(body string) *Parsed {
	text := strings.TrimSpace(nfc(body))
	if text == "" {
		return nil
	}
	folded := taste.Casefold(text)
	for _, command := range commands {
		if folded == command.token {
			return &Parsed{Intent: command.intent, Args: ""}
		}
		if strings.HasPrefix(folded, command.token) {
			rest := folded[len(command.token):]
			r, _ := utf8.DecodeRuneInString(rest)
			if isPySpace(r) {
				return &Parsed{Intent: command.intent, Args: strings.TrimSpace(text[len(command.token):])}
			}
		}
	}
	for _, mention := range mentions {
		if strings.Contains(folded, mention) {
			return &Parsed{Intent: Mention, Args: text}
		}
	}
	return nil
}

// ParseVote is parse_vote: `Câu hỏi? A | B | C` or nil.
func ParseVote(args string) *VoteSpec {
	text := strings.TrimSpace(nfc(args))
	if !strings.Contains(text, "|") {
		return nil
	}
	var question, rest string
	if strings.Contains(text, "?") {
		head, tail, _ := strings.Cut(text, "?")
		question, rest = strings.TrimSpace(head+"?"), tail
	} else {
		question, rest, _ = strings.Cut(text, "|")
		question = strings.TrimSpace(question)
	}
	var options []string
	seen := map[string]bool{}
	for _, part := range strings.Split(rest, "|") {
		label := strings.Join(strings.Fields(part), " ")
		if label == "" {
			continue
		}
		if utf8.RuneCountInString(label) > maxOption {
			return nil
		}
		key := taste.Casefold(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		options = append(options, label)
	}
	bare := strings.TrimSpace(strings.TrimRight(question, "?"))
	if bare == "" || utf8.RuneCountInString(question) > maxQuestion {
		return nil
	}
	if len(options) < minOptions || len(options) > maxOptions {
		return nil
	}
	return &VoteSpec{Question: question, Options: options}
}
