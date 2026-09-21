// Package reel is app.domain.reel: attach server-owned memory facts to AI picks.
package reel

import (
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/domain/tree"
)

const (
	MaxPicks = 6
	MaxTitle = 120
	MaxNote  = 240
)

// Error is ReelError.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func refuse(code string) error { return &Error{Code: code} }

func incomplete() error { return refuse("incomplete_pick") }

func bounded(value tree.Value, limit int) (string, error) {
	text, ok := value.(tree.String)
	if !ok {
		return "", incomplete()
	}
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return "", incomplete()
	}
	if utf8.RuneCountInString(trimmed) > limit {
		trimmed = string([]rune(trimmed)[:limit])
	}
	return trimmed, nil
}

// Memory is one offered row.
type Memory struct {
	ID            string
	ImageURL      *string
	Caption       *string
	PlaceName     *string
	CreatedAt     string
	ReactionCount int64
	CommentCount  int64
}

func textOrNull(value *string) tree.Value {
	if value == nil {
		return tree.Null{}
	}
	return tree.String(*value)
}

// Ground is ground_reel.
func Ground(raw tree.Value, memories []Memory) (*tree.OrderedMap, error) {
	obj, ok := raw.(*tree.OrderedMap)
	if !ok {
		return nil, incomplete()
	}
	rawPicks, ok := obj.Get("picks")
	list, okList := rawPicks.(tree.List)
	if !ok || !okList {
		return nil, incomplete()
	}
	byID := map[string]Memory{}
	for _, memory := range memories {
		byID[memory.ID] = memory
	}
	type pick struct {
		id  string
		row *tree.OrderedMap
	}
	entries := make([]pick, 0, len(list))
	for _, item := range list {
		row, ok := item.(*tree.OrderedMap)
		if !ok {
			return nil, incomplete()
		}
		idValue, _ := row.Get("memory_id")
		id, ok := idValue.(tree.String)
		if !ok || string(id) == "" {
			return nil, incomplete()
		}
		entries = append(entries, pick{id: string(id), row: row})
	}
	for _, e := range entries {
		if _, found := byID[e.id]; !found {
			return nil, refuse("unknown_memory")
		}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.id] {
			return nil, refuse("duplicate_memory")
		}
		seen[e.id] = true
	}
	if len(entries) > MaxPicks {
		return nil, refuse("too_many_picks")
	}
	if len(entries) == 0 {
		return nil, refuse("empty_reel")
	}
	titleValue, _ := obj.Get("title")
	title, err := bounded(titleValue, MaxTitle)
	if err != nil {
		return nil, err
	}
	picks := tree.List{}
	for _, e := range entries {
		noteValue, _ := e.row.Get("note")
		note, err := bounded(noteValue, MaxNote)
		if err != nil {
			return nil, err
		}
		memory := byID[e.id]
		entry := tree.NewOrderedMap()
		entry.Set("memory_id", tree.String(memory.ID))
		entry.Set("image_url", textOrNull(memory.ImageURL))
		entry.Set("caption", textOrNull(memory.Caption))
		entry.Set("place_name", textOrNull(memory.PlaceName))
		entry.Set("created_at", tree.String(memory.CreatedAt))
		entry.Set("reaction_count", tree.NewInt(memory.ReactionCount))
		entry.Set("comment_count", tree.NewInt(memory.CommentCount))
		entry.Set("note", tree.String(note))
		picks = append(picks, entry)
	}
	out := tree.NewOrderedMap()
	out.Set("title", tree.String(title))
	out.Set("picks", picks)
	return out, nil
}
