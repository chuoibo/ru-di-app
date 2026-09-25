// Package companion is app.domain.companion: AI card grounding. The speaking
// cadence (PlanTurn) left with the automatic companion (ADR-0036 §2.1).
package companion

import (
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/domain/tree"
)

const (
	MaxPlaces = 5
	MaxStops  = 6
	MaxText   = 600
)

// Error is CompanionError.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func refuse(code string) error { return &Error{Code: code} }

func malformed() error { return refuse("companion_card_malformed") }

func boundedText(payload *tree.OrderedMap, key string) (string, error) {
	value, ok := payload.Get(key)
	if !ok {
		return "", malformed()
	}
	text, ok := value.(tree.String)
	if !ok {
		return "", malformed()
	}
	s := string(text)
	if utf8.RuneCountInString(s) > MaxText {
		s = string([]rune(s)[:MaxText])
	}
	return s, nil
}

func optionalText(payload *tree.OrderedMap, key string) (string, error) {
	if _, ok := payload.Get(key); !ok {
		return "", nil
	}
	return boundedText(payload, key)
}

func catalogueByID(places []*tree.OrderedMap) map[string]*tree.OrderedMap {
	out := map[string]*tree.OrderedMap{}
	for _, place := range places {
		if place == nil {
			continue
		}
		id, ok := place.Get("id")
		if !ok {
			continue
		}
		text, ok := id.(tree.String)
		if !ok {
			continue
		}
		out[string(text)] = place
	}
	return out
}

func clonePlace(place *tree.OrderedMap) *tree.OrderedMap {
	return place.Clone()
}

func groundPlaces(payload *tree.OrderedMap, catalogue map[string]*tree.OrderedMap) (*tree.OrderedMap, error) {
	intro, err := optionalText(payload, "intro")
	if err != nil {
		return nil, err
	}
	rawIDs, ok := payload.Get("place_ids")
	list, okList := rawIDs.(tree.List)
	if !ok || !okList {
		return nil, malformed()
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(tree.String)
		if !ok {
			return nil, malformed()
		}
		ids = append(ids, string(text))
	}
	for _, id := range ids {
		if _, found := catalogue[id]; !found {
			return nil, refuse("companion_place_not_in_catalogue")
		}
	}
	unique := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	end := len(unique)
	if end > MaxPlaces {
		end = MaxPlaces
	}
	if end == 0 {
		return nil, refuse("companion_card_empty")
	}
	places := tree.List{}
	for _, id := range unique[:end] {
		places = append(places, clonePlace(catalogue[id]))
	}
	out := tree.NewOrderedMap()
	body := tree.NewOrderedMap()
	body.Set("intro", tree.String(intro))
	body.Set("places", places)
	if omitted := len(unique) - end; omitted > 0 {
		body.Set("omitted_place_count", tree.NewInt(int64(omitted)))
	}
	out.Set("kind", tree.String("places"))
	out.Set("payload", body)
	return out, nil
}

func groundItinerary(payload *tree.OrderedMap, catalogue map[string]*tree.OrderedMap) (*tree.OrderedMap, error) {
	title, err := optionalText(payload, "title")
	if err != nil {
		return nil, err
	}
	rawStops, ok := payload.Get("stops")
	list, okList := rawStops.(tree.List)
	if !ok || !okList {
		return nil, malformed()
	}
	type stop struct{ id, time, note string }
	stops := make([]stop, 0, len(list))
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
		timeText, err := boundedText(row, "time_text")
		if err != nil {
			return nil, err
		}
		note, err := boundedText(row, "note")
		if err != nil {
			return nil, err
		}
		stops = append(stops, stop{id: string(id), time: timeText, note: note})
	}
	for _, s := range stops {
		if _, found := catalogue[s.id]; !found {
			return nil, refuse("companion_place_not_in_catalogue")
		}
	}
	if len(stops) == 0 {
		return nil, refuse("companion_card_empty")
	}
	end := len(stops)
	if end > MaxStops {
		end = MaxStops
	}
	shown := tree.List{}
	for _, s := range stops[:end] {
		entry := tree.NewOrderedMap()
		entry.Set("time_text", tree.String(s.time))
		entry.Set("note", tree.String(s.note))
		entry.Set("place", clonePlace(catalogue[s.id]))
		shown = append(shown, entry)
	}
	body := tree.NewOrderedMap()
	body.Set("title", tree.String(title))
	body.Set("stops", shown)
	if omitted := len(stops) - end; omitted > 0 {
		body.Set("omitted_stop_count", tree.NewInt(int64(omitted)))
	}
	out := tree.NewOrderedMap()
	out.Set("kind", tree.String("itinerary"))
	out.Set("payload", body)
	return out, nil
}

// GroundCard is ground_card.
func GroundCard(raw tree.Value, places []*tree.OrderedMap) (*tree.OrderedMap, error) {
	obj, ok := raw.(*tree.OrderedMap)
	if !ok {
		return nil, malformed()
	}
	if _, found := obj.Get("kind"); !found {
		return nil, malformed()
	}
	payloadValue, ok := obj.Get("payload")
	payload, okPayload := payloadValue.(*tree.OrderedMap)
	if !ok || !okPayload {
		return nil, malformed()
	}
	kindValue, _ := obj.Get("kind")
	kind, ok := kindValue.(tree.String)
	if !ok {
		return nil, malformed()
	}
	switch string(kind) {
	case "text":
		text, err := boundedText(payload, "text")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(text) == "" {
			return nil, refuse("companion_card_empty")
		}
		body := tree.NewOrderedMap()
		body.Set("text", tree.String(text))
		out := tree.NewOrderedMap()
		out.Set("kind", tree.String("text"))
		out.Set("payload", body)
		return out, nil
	case "places":
		return groundPlaces(payload, catalogueByID(places))
	case "itinerary":
		return groundItinerary(payload, catalogueByID(places))
	default:
		return nil, refuse("companion_card_kind_unknown")
	}
}
