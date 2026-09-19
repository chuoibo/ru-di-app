package album

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_album*.json is rendered by scripts/render_domain_wai_goldens.py
// from the real app.domain.album in the parity API image.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "AlbumError", e.Code, true
	}
	return "", "", false
}

func TestAlbumMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "album", replay, refusal)
}

func TestAlbumConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "album")
	if got, _ := oracletest.Int64(c["MAX_PHOTOS"]); got != MaxPhotos {
		t.Errorf("MAX_PHOTOS: Python %v, Go %d", c["MAX_PHOTOS"], MaxPhotos)
	}
	if got, _ := oracletest.Int64(c["MAX_PLACES"]); got != MaxPlaces {
		t.Errorf("MAX_PLACES: Python %v, Go %d", c["MAX_PLACES"], MaxPlaces)
	}
	if got, _ := oracletest.Int64(c["MAX_HIGHLIGHTS"]); got != MaxHighlights {
		t.Errorf("MAX_HIGHLIGHTS: Python %v, Go %d", c["MAX_HIGHLIGHTS"], MaxHighlights)
	}
	if got, _ := oracletest.Int64(c["MIN_HIGHLIGHT_REACTIONS"]); got != MinHighlightReactions {
		t.Errorf("MIN_HIGHLIGHT_REACTIONS: Python %v, Go %d", c["MIN_HIGHLIGHT_REACTIONS"], MinHighlightReactions)
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "period_label":
		starts, okS := args["starts_on"].(oracletest.PyDate)
		ends, okE := args["ends_on"].(oracletest.PyDate)
		if !okS || !okE {
			return nil, &Error{"album_dates_malformed"}
		}
		start, err := oracletest.TimeOfDate(starts)
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		end, err := oracletest.TimeOfDate(ends)
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return PeriodLabel(start, end)
	case "build_album":
		outing, err := outingOf(args["outing"])
		if err != nil {
			return nil, err
		}
		memories, err := memoriesOf(args["memories"])
		if err != nil {
			return nil, err
		}
		built, err := Build(outing, memories)
		if err != nil {
			return nil, err
		}
		return builtOK(built), nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func outingOf(raw any) (Outing, error) {
	row, ok := raw.(map[string]any)
	if !ok {
		return Outing{}, &Error{"album_outing_malformed"}
	}
	starts, okS := row["starts_on"].(oracletest.PyDate)
	ends, okE := row["ends_on"].(oracletest.PyDate)
	if !okS || !okE {
		return Outing{}, &Error{"album_dates_malformed"}
	}
	start, err := oracletest.TimeOfDate(starts)
	if err != nil {
		return Outing{}, oracletest.Decode(err)
	}
	end, err := oracletest.TimeOfDate(ends)
	if err != nil {
		return Outing{}, oracletest.Decode(err)
	}
	title, _ := row["title"].(string)
	head, _ := row["headcount"].(int64)
	split, _ := row["split_total_vnd"].(int64)
	expenses, _ := row["expense_count"].(int64)
	return Outing{
		Title: title, StartsOn: start, EndsOn: end,
		Headcount: head, SplitTotalVND: split, ExpenseCount: expenses,
	}, nil
}

func memoriesOf(raw any) ([]Memory, error) {
	items, err := oracletest.List(raw)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	out := make([]Memory, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, &Error{"album_memory_malformed"}
		}
		m := Memory{}
		if id, ok := row["id"].(string); ok {
			m.ID = id
		}
		if kind, ok := row["kind"].(string); ok {
			m.Kind = kind
		}
		url, err := oracletest.OptionalString(row["image_url"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		m.ImageURL = url
		caption, err := oracletest.OptionalString(row["caption"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		m.Caption = caption
		placeID, err := oracletest.OptionalString(row["place_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		m.PlaceID = placeID
		placeName, err := oracletest.OptionalString(row["place_name"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		m.PlaceName = placeName
		if stamp, ok := row["created_at"].(oracletest.PyInstant); ok {
			at, err := oracletest.TimeOfStamp(stamp)
			if err != nil {
				return nil, oracletest.Decode(err)
			}
			m.CreatedAt = at
		}
		if n, ok := row["reaction_count"].(int64); ok {
			m.ReactionCount = n
		}
		if n, ok := row["comment_count"].(int64); ok {
			m.CommentCount = n
		}
		out = append(out, m)
	}
	return out, nil
}

func builtOK(b Built) map[string]any {
	photos := make([]any, 0, len(b.Photos))
	for _, p := range b.Photos {
		photos = append(photos, photoOK(p))
	}
	places := make([]any, 0, len(b.Places))
	for _, p := range b.Places {
		places = append(places, map[string]any{"place_id": p.PlaceID, "place_name": ptr(p.PlaceName)})
	}
	highlights := make([]any, 0, len(b.Highlights))
	for _, p := range b.Highlights {
		highlights = append(highlights, photoOK(p))
	}
	return map[string]any{
		"title": b.Title, "period_label": b.PeriodLabel,
		"starts_on": oracletest.DateOf(b.StartsOn), "ends_on": oracletest.DateOf(b.EndsOn),
		"photos": photos, "photo_count": int64(b.PhotoCount),
		"places": places, "place_count": int64(b.PlaceCount),
		"checkin_count": int64(b.CheckinCount), "highlights": highlights,
		"split_total_vnd": b.SplitTotalVND, "expense_count": b.ExpenseCount, "headcount": b.Headcount,
	}
}

func photoOK(p Photo) map[string]any {
	return map[string]any{
		"memory_id": p.MemoryID, "image_url": p.ImageURL, "caption": ptr(p.Caption),
		"created_at":     oracletest.StampOf(p.CreatedAt),
		"reaction_count": p.ReactionCount, "comment_count": p.CommentCount,
	}
}

func ptr(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
