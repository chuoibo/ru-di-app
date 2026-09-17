// Package suggestion is app.domain.suggestion: history digest and card grounding.
package suggestion

import (
	"sort"
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/tree"
)

const (
	Kind            = "outing_suggestion"
	MaxStops        = 4
	MaxText         = 240
	MaxRecentTitles = 3
)

var verdicts = map[string]bool{"hop": true, "tam": true, "khong-hop": true}

// Error is SuggestionError.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func refuse(code string) error { return &Error{Code: code} }

func malformed() error { return refuse("suggestion_card_malformed") }

// Trip is one recap row as summarise_history reads it.
type Trip struct {
	Title         string
	SplitTotalVND int64
	Headcount     int64
}

// History is summarise_history's dict.
type History struct {
	OutingCount     int
	SplitTotalVND   int64
	AvgPerPersonVND *int64
	TopCategories   []string
	RecentTitles    []string
}

// SummariseHistory is summarise_history.
func SummariseHistory(trips []Trip, categories []string) (History, error) {
	var total int64
	var people int64
	titles := []string{}
	for _, trip := range trips {
		if money.Violation(money.VND(trip.SplitTotalVND), false, false) != "" {
			return History{}, refuse("suggestion_history_not_integer_dong")
		}
		if trip.Headcount < 1 {
			return History{}, refuse("suggestion_history_headcount_not_integer")
		}
		total += trip.SplitTotalVND
		people += trip.Headcount
		title := strings.TrimSpace(trip.Title)
		if title != "" {
			if utf8.RuneCountInString(title) > MaxText {
				title = string([]rune(title)[:MaxText])
			}
			titles = append(titles, title)
		}
	}
	counts := map[string]int{}
	order := []string{}
	for _, category := range categories {
		if _, seen := counts[category]; !seen {
			order = append(order, category)
		}
		counts[category]++
	}
	sort.Slice(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return order[i] < order[j]
	})
	out := History{
		OutingCount: len(trips), SplitTotalVND: total, TopCategories: order, RecentTitles: titles,
	}
	if len(out.TopCategories) == 0 {
		out.TopCategories = []string{}
	}
	if len(titles) > MaxRecentTitles {
		out.RecentTitles = titles[:MaxRecentTitles]
	}
	if people != 0 {
		avg := total / people
		out.AvgPerPersonVND = &avg
	}
	return out, nil
}

func bounded(payload *tree.OrderedMap, key string) (string, error) {
	value, ok := payload.Get(key)
	text, is := value.(tree.String)
	if !ok || !is {
		return "", malformed()
	}
	s := string(text)
	if utf8.RuneCountInString(s) > MaxText {
		s = string([]rune(s)[:MaxText])
	}
	return s, nil
}

func reasonOf(value tree.Value) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if _, ok := value.(tree.Null); ok {
		return nil, nil
	}
	text, ok := value.(tree.String)
	if !ok {
		return nil, malformed()
	}
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > MaxText {
		trimmed = string([]rune(trimmed)[:MaxText])
	}
	return &trimmed, nil
}

func verdictOf(value tree.Value) *string {
	text, ok := value.(tree.String)
	if !ok || !verdicts[string(text)] {
		return nil
	}
	s := string(text)
	return &s
}

func clonePlace(place *tree.OrderedMap) *tree.OrderedMap {
	return place.Clone()
}

// Ground is ground_suggestion.
func Ground(raw tree.Value, places []*tree.OrderedMap) (*tree.OrderedMap, error) {
	obj, ok := raw.(*tree.OrderedMap)
	if !ok {
		return nil, malformed()
	}
	kindValue, found := obj.Get("kind")
	if !found {
		return nil, malformed()
	}
	kind, ok := kindValue.(tree.String)
	if !ok {
		return nil, malformed()
	}
	if string(kind) != Kind {
		return nil, refuse("suggestion_card_kind_unknown")
	}
	payloadValue, ok := obj.Get("payload")
	payload, okPayload := payloadValue.(*tree.OrderedMap)
	if !ok || !okPayload {
		return nil, malformed()
	}
	title, err := bounded(payload, "title")
	if err != nil {
		return nil, err
	}
	when, err := bounded(payload, "when_text")
	if err != nil {
		return nil, err
	}
	rawStops, ok := payload.Get("stops")
	list, okList := rawStops.(tree.List)
	if !ok || !okList {
		return nil, malformed()
	}
	type entry struct {
		id, time, note  string
		reason, verdict *string
	}
	entries := make([]entry, 0, len(list))
	for _, item := range list {
		row, ok := item.(*tree.OrderedMap)
		if !ok {
			return nil, malformed()
		}
		idValue, _ := row.Get("place_id")
		id, ok := idValue.(tree.String)
		if !ok {
			return nil, malformed()
		}
		timeText, err := bounded(row, "time_text")
		if err != nil {
			return nil, err
		}
		note, err := bounded(row, "note")
		if err != nil {
			return nil, err
		}
		reasonValue, _ := row.Get("reason")
		reason, err := reasonOf(reasonValue)
		if err != nil {
			return nil, err
		}
		verdictValue, _ := row.Get("verdict")
		verdict := verdictOf(verdictValue)
		if reason == nil || verdict == nil {
			reason, verdict = nil, nil
		}
		entries = append(entries, entry{id: string(id), time: timeText, note: note, reason: reason, verdict: verdict})
	}
	catalogue := map[string]*tree.OrderedMap{}
	for _, place := range places {
		if place == nil {
			continue
		}
		id, ok := place.Get("id")
		text, okID := id.(tree.String)
		if !ok || !okID {
			continue
		}
		catalogue[string(text)] = place
	}
	for _, e := range entries {
		if _, found := catalogue[e.id]; !found {
			return nil, refuse("suggestion_place_not_in_catalogue")
		}
	}
	stops := tree.List{}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.id] {
			continue
		}
		seen[e.id] = true
		stop := tree.NewOrderedMap()
		stop.Set("time_text", tree.String(e.time))
		stop.Set("note", tree.String(e.note))
		if e.reason == nil {
			stop.Set("reason", tree.Null{})
		} else {
			stop.Set("reason", tree.String(*e.reason))
		}
		if e.verdict == nil {
			stop.Set("verdict", tree.Null{})
		} else {
			stop.Set("verdict", tree.String(*e.verdict))
		}
		stop.Set("place", clonePlace(catalogue[e.id]))
		stops = append(stops, stop)
		if len(stops) == MaxStops {
			break
		}
	}
	if len(stops) == 0 {
		return nil, refuse("suggestion_card_empty")
	}
	body := tree.NewOrderedMap()
	body.Set("title", tree.String(title))
	body.Set("when_text", tree.String(when))
	body.Set("stops", stops)
	out := tree.NewOrderedMap()
	out.Set("kind", tree.String(Kind))
	out.Set("payload", body)
	return out, nil
}
