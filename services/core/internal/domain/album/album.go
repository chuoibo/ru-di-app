// Package album is app.domain.album: one way of reading rows that already
// exist. No table, no blob, no second URL.
package album

import (
	"fmt"
	"strings"
	"time"
)

const (
	MaxPhotos             = 60
	MaxPlaces             = 20
	MaxHighlights         = 6
	MinHighlightReactions = 1
)

// Error is AlbumError.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

// Outing is the recap row the builder reads.
type Outing struct {
	Title         string
	StartsOn      time.Time
	EndsOn        time.Time
	Headcount     int64
	SplitTotalVND int64
	ExpenseCount  int64
}

// Memory is one wall row the album may list.
type Memory struct {
	ID            string
	Kind          string
	ImageURL      *string
	Caption       *string
	PlaceID       *string
	PlaceName     *string
	CreatedAt     time.Time
	ReactionCount int64
	CommentCount  int64
}

// Photo is one album photograph.
type Photo struct {
	MemoryID      string
	ImageURL      string
	Caption       *string
	CreatedAt     time.Time
	ReactionCount int64
	CommentCount  int64
}

// Place is one named stop.
type Place struct {
	PlaceID   string
	PlaceName *string
}

// Built is build_album's dict.
type Built struct {
	Title         string
	PeriodLabel   string
	StartsOn      time.Time
	EndsOn        time.Time
	Photos        []Photo
	PhotoCount    int
	Places        []Place
	PlaceCount    int
	CheckinCount  int
	Highlights    []Photo
	SplitTotalVND int64
	ExpenseCount  int64
	Headcount     int64
}

func text(value *string, limit int) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	runes := []rune(trimmed)
	if len(runes) > limit {
		trimmed = string(runes[:limit])
	}
	return &trimmed
}

func clip(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return trimmed
}

// PeriodLabel is period_label: `2026`, or `2025–2026`.
func PeriodLabel(startsOn, endsOn time.Time) (string, error) {
	if startsOn.IsZero() || endsOn.IsZero() {
		return "", &Error{"album_dates_malformed"}
	}
	if endsOn.Year() != startsOn.Year() {
		return fmt.Sprintf("%d–%d", startsOn.Year(), endsOn.Year()), nil
	}
	return fmt.Sprintf("%d", startsOn.Year()), nil
}

// Build is build_album.
func Build(outing Outing, memories []Memory) (Built, error) {
	label, err := PeriodLabel(outing.StartsOn, outing.EndsOn)
	if err != nil {
		return Built{}, err
	}
	title := clip(outing.Title, 240)
	if title == "" {
		title = "Chuyến đi"
	}
	var photos []Photo
	var places []Place
	seen := map[string]bool{}
	checkins := 0
	type ranked struct {
		hearts int64
		photo  Photo
	}
	var reacted []ranked
	for _, memory := range memories {
		switch memory.Kind {
		case "photo":
			if memory.ImageURL == nil || *memory.ImageURL == "" {
				continue
			}
			entry := Photo{
				MemoryID: memory.ID, ImageURL: *memory.ImageURL, Caption: text(memory.Caption, 240),
				CreatedAt: memory.CreatedAt, ReactionCount: memory.ReactionCount, CommentCount: memory.CommentCount,
			}
			photos = append(photos, entry)
			if entry.ReactionCount >= MinHighlightReactions {
				reacted = append(reacted, ranked{hearts: entry.ReactionCount, photo: entry})
			}
		case "checkin":
			checkins++
			if memory.PlaceID != nil && *memory.PlaceID != "" && !seen[*memory.PlaceID] {
				seen[*memory.PlaceID] = true
				places = append(places, Place{PlaceID: *memory.PlaceID, PlaceName: text(memory.PlaceName, 240)})
			}
		}
	}
	for i := 0; i < len(reacted); i++ {
		for j := i + 1; j < len(reacted); j++ {
			if reacted[j].hearts > reacted[i].hearts ||
				(reacted[j].hearts == reacted[i].hearts && reacted[j].photo.CreatedAt.After(reacted[i].photo.CreatedAt)) {
				reacted[i], reacted[j] = reacted[j], reacted[i]
			}
		}
	}
	highlights := make([]Photo, 0, MaxHighlights)
	for i, row := range reacted {
		if i == MaxHighlights {
			break
		}
		highlights = append(highlights, row.photo)
	}
	photoCount := len(photos)
	if len(photos) > MaxPhotos {
		photos = photos[:MaxPhotos]
	}
	placeCount := len(places)
	if len(places) > MaxPlaces {
		places = places[:MaxPlaces]
	}
	return Built{
		Title: title, PeriodLabel: label, StartsOn: outing.StartsOn, EndsOn: outing.EndsOn,
		Photos: photos, PhotoCount: photoCount, Places: places, PlaceCount: placeCount,
		CheckinCount: checkins, Highlights: highlights,
		SplitTotalVND: outing.SplitTotalVND, ExpenseCount: outing.ExpenseCount, Headcount: outing.Headcount,
	}, nil
}
