package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
)

// Place is PlaceRecord, field for field.
//
// JSONB columns keep PostgreSQL's text: psycopg hands Python json.loads of
// exactly these bytes, so key order and number spelling (1 is an int, 1.0 a
// float) are the ones Python sees. A nil json.RawMessage is Python's None,
// which a JSONB SQL NULL and a JSON `null` both become.
type Place struct {
	ID            string
	DestinationID string
	Name          string
	Category      string
	Kinds         []string
	Address       *string
	Lat           float64
	Lng           float64
	Rating        *float64
	RatingCount   *int64
	PriceMinVND   *int64
	PriceMaxVND   *int64
	OpenHours     *string
	OpenNow       *bool
	TravelMinutes *int64
	DistanceKM    *float64
	PhotoCount    int64
	Traits        []string
	GroupFit      json.RawMessage
	Flag          *string
	Description   *string
	Reviews       json.RawMessage
	Source        string
	SourceRef     *string
	License       *string
	Activities    json.RawMessage
}

// PlaceFilter is list_places' keyword arguments; nil is "not passed".
type PlaceFilter struct {
	DestinationID *string
	Category      *string
}

// ErrUnrepresentable marks a stored value the Python record would carry but
// this Go record's type cannot (a JSONB list element that is not a string).
var ErrUnrepresentable = errors.New("repo: stored value has no representation in the Go record")

// ListPlaces is list_places: every place matching the filters, ORDER BY id
// under the column's collation. `select(Place)` loads every mapped column,
// created_at and updated_at included, although PlaceRecord drops them.
func (r Repository) ListPlaces(ctx context.Context, filter PlaceFilter) ([]Place, error) {
	var where []string
	var args []any
	if filter.DestinationID != nil {
		args = append(args, *filter.DestinationID)
		where = append(where, "places.destination_id = $"+strconv.Itoa(len(args))+"::VARCHAR")
	}
	if filter.Category != nil {
		args = append(args, *filter.Category)
		where = append(where, "places.category = $"+strconv.Itoa(len(args))+"::VARCHAR")
	}
	sql := `SELECT places.id, places.destination_id, places.name, places.category, places.kinds,
	               places.address, places.lat, places.lng, places.rating, places.rating_count,
	               places.price_min_vnd, places.price_max_vnd, places.open_hours, places.open_now,
	               places.travel_minutes, places.distance_km, places.photo_count, places.traits,
	               places.group_fit, places.activities, places.flag, places.description,
	               places.reviews, places.source, places.source_ref, places.license,
	               places.created_at, places.updated_at
	          FROM places`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += " ORDER BY places.id"
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Place{}
	for rows.Next() {
		var p Place
		var kinds, traits, groupFit, activities, reviews []byte
		var created, updated any
		if err := rows.Scan(&p.ID, &p.DestinationID, &p.Name, &p.Category, &kinds,
			&p.Address, &p.Lat, &p.Lng, &p.Rating, &p.RatingCount,
			&p.PriceMinVND, &p.PriceMaxVND, &p.OpenHours, &p.OpenNow,
			&p.TravelMinutes, &p.DistanceKM, &p.PhotoCount, &traits,
			&groupFit, &activities, &p.Flag, &p.Description,
			&reviews, &p.Source, &p.SourceRef, &p.License,
			&created, &updated); err != nil {
			return nil, err
		}
		if p.Kinds, err = pythonListOrEmpty(kinds); err != nil {
			return nil, err
		}
		if p.Traits, err = pythonListOrEmpty(traits); err != nil {
			return nil, err
		}
		p.GroupFit = jsonOrNone(groupFit)
		p.Activities = jsonOrNone(activities)
		p.Reviews = jsonOrNone(reviews)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// jsonOrNone is what json.loads leaves of a JSONB value: nil for SQL NULL and
// for JSON null, the bytes otherwise.
func jsonOrNone(raw []byte) json.RawMessage {
	if raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	return json.RawMessage(raw)
}

// pythonListOrEmpty is `list(value or [])` over json.loads(raw), the shape
// _place_record gives kinds and traits. Python's truthiness and list() decide:
// null, false, 0, "", [] and {} are empty; a string becomes its code points; an
// object becomes its keys in stored order; a non-zero number or true raises
// TypeError. A list element that is not a string is ErrUnrepresentable.
func pythonListOrEmpty(raw []byte) ([]string, error) {
	out := []string{}
	if raw == nil {
		return out, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case nil:
		return out, nil
	case bool:
		if !value {
			return out, nil
		}
		return nil, ErrPythonTypeError
	case json.Number:
		if jsonNumberIsZero(string(value)) {
			return out, nil
		}
		return nil, ErrPythonTypeError
	case string:
		for _, r := range value {
			out = append(out, string(r))
		}
		return out, nil
	case json.Delim:
		for decoder.More() {
			element, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			text, isText := element.(string)
			if value == '{' {
				out = append(out, text)
				var skip json.RawMessage
				if err := decoder.Decode(&skip); err != nil {
					return nil, err
				}
				continue
			}
			if !isText {
				return nil, ErrUnrepresentable
			}
			out = append(out, text)
		}
		return out, nil
	}
	return nil, ErrUnrepresentable
}

// jsonNumberIsZero follows json.loads: a literal with a fraction or an
// exponent is a float (and float("1e-400") underflows to 0.0), anything else
// an int.
func jsonNumberIsZero(literal string) bool {
	if strings.ContainsAny(literal, ".eE") {
		f, _ := strconv.ParseFloat(literal, 64)
		return f == 0
	}
	n, ok := new(big.Int).SetString(literal, 10)
	return ok && n.Sign() == 0
}
