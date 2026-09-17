// Package conversation is app.domain.conversation: the last few human turns.
package conversation

import "strings"

const (
	Kind     = "text"
	MaxLines = 12
	MaxLine  = 200
	MinLines = 2
)

// Digest is summarise_conversation's dict.
type Digest struct {
	RecentLines  []string
	MessageCount int
	SpeakerCount int
	MemberCount  int
}

// Message is one stored row as the digest reads it.
type Message struct {
	Kind     string
	Body     *string
	AuthorID *string
}

// Summarise is summarise_conversation. messages arrive newest-first.
func Summarise(messages []Message, memberCount int) Digest {
	lines := []string{}
	speakers := map[string]bool{}
	for _, message := range messages {
		if message.Kind != Kind || message.Body == nil {
			continue
		}
		body := strings.TrimSpace(*message.Body)
		if body == "" {
			continue
		}
		if len(lines) < MaxLines {
			runes := []rune(body)
			if len(runes) > MaxLine {
				body = string(runes[:MaxLine])
			}
			lines = append(lines, body)
			if message.AuthorID != nil {
				speakers[*message.AuthorID] = true
			}
		}
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return Digest{
		RecentLines: lines, MessageCount: len(lines), SpeakerCount: len(speakers), MemberCount: memberCount,
	}
}

// Has is has_conversation.
func Has(digest Digest) bool { return digest.MessageCount >= MinLines }
