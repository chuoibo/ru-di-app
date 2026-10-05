// Package conversation is the last few human turns of a room, as the
// contextual suggestion hands them to the model.
//
// Owner decision 2026-10-05: the twelve latest text messages go in whole, each
// one labelled with who said it ("Minh: …"). Python's summarise_conversation,
// which clipped every line at 200 characters and dropped the speaker, was
// deleted by ADR-0052; this package no longer mirrors it.
package conversation

import (
	"strconv"
	"strings"
)

const (
	Kind     = "text"
	MaxLines = 12
	MinLines = 2
)

// Digest is what the contextual prompt reads.
type Digest struct {
	RecentLines  []string
	MessageCount int
	SpeakerCount int
	MemberCount  int
}

// Message is one stored row as the digest reads it. Speaker is the author's
// display name as a model may read it, or "" when the caller has none it may
// show (no safe name, a former member, no author at all).
type Message struct {
	Kind     string
	Body     *string
	AuthorID *string
	Speaker  string
}

// Summarise keeps the MaxLines newest non-empty text messages, oldest first,
// each as "Speaker: body" with the body whole. messages arrive newest-first.
// A turn without a usable name is labelled "Bạn N", numbered in the order the
// unnamed people first speak, so the model can still tell them apart without
// ever seeing an account id. Turns without an author share one label.
func Summarise(messages []Message, memberCount int) Digest {
	type turn struct {
		key, speaker, body string
	}
	turns := []turn{}
	speakers := map[string]bool{}
	for _, message := range messages {
		if len(turns) == MaxLines {
			break
		}
		if message.Kind != Kind || message.Body == nil {
			continue
		}
		body := strings.TrimSpace(*message.Body)
		if body == "" {
			continue
		}
		key := ""
		if message.AuthorID != nil {
			key = *message.AuthorID
			speakers[key] = true
		}
		turns = append(turns, turn{key: key, speaker: speakerLabel(message.Speaker), body: oneLine(body)})
	}
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
	unnamed := map[string]string{}
	lines := make([]string, 0, len(turns))
	for _, t := range turns {
		label := t.speaker
		if label == "" {
			if t.key == "" {
				label = "Ai đó"
			} else if known, ok := unnamed[t.key]; ok {
				label = known
			} else {
				label = "Bạn " + strconv.Itoa(len(unnamed)+1)
				unnamed[t.key] = label
			}
		}
		lines = append(lines, label+": "+t.body)
	}
	return Digest{
		RecentLines: lines, MessageCount: len(lines), SpeakerCount: len(speakers), MemberCount: memberCount,
	}
}

// lineBreaks are every character a model could read as the start of a new
// line: a body must not be able to open a turn of its own ("ok\nMinh: …")
// and so put words in someone else's mouth.
var lineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\v", " ", "\f", " ",
	"\u0085", " ", "\u2028", " ", "\u2029", " ")

func oneLine(body string) string { return lineBreaks.Replace(body) }

// speakerLabel is the name a line starts with, or "" when the name could pass
// for a speaker boundary itself ("Minh: hi An") and the turn must fall back
// to a neutral label.
func speakerLabel(name string) string {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, ":：") || oneLine(name) != name {
		return ""
	}
	return name
}

// Has is whether there is enough conversation to suggest from.
func Has(digest Digest) bool { return digest.MessageCount >= MinLines }
