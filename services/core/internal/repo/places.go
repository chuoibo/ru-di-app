package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
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
	// Nil together or not at all. A quarter of the fed catalogue has no
	// coordinates and never will; scanning NULL into a float64 is what
	// turned every read of such a row into a 500.
	Lat           *float64
	Lng           *float64
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
	// How the point was arrived at. A rooftop match and a province centroid
	// are both "has coordinates"; only one of them may be drawn.
	GeoPrecision *string
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
	return r.listPlaces(ctx, filter, false)
}

// ListPlaceCards is ListPlaces without the three columns no list card carries:
// `description`, `reviews` and `activities` come back nil.
//
// Those three are most of a fed row's bytes -- on TP.HCM's 5,235 rows they are
// ~14 MB of the ~15 MB a full read pulls -- while the query itself runs in
// ~35 ms. Reading them for a list is what made GET /places take 12 s. A caller
// that needs them for a few rows reads those rows with PlacesByIDs.
func (r Repository) ListPlaceCards(ctx context.Context, filter PlaceFilter) ([]Place, error) {
	return r.listPlaces(ctx, filter, true)
}

// PlacesByIDs reads every mapped column of the given places, in no particular
// order; ids that do not exist are absent.
func (r Repository) PlacesByIDs(ctx context.Context, ids []string) ([]Place, error) {
	out := []Place{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.Q.Query(ctx, placeSelect(false)+` WHERE places.id = ANY($1::VARCHAR[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPlace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// placeSelect is the column list scanPlace reads, in its order. slim swaps the
// three heavy columns for typed NULLs so the scan is unchanged.
func placeSelect(slim bool) string {
	activities, description, reviews := "places.activities", "places.description", "places.reviews"
	if slim {
		activities, description, reviews = "NULL::jsonb", "NULL::text", "NULL::jsonb"
	}
	return `SELECT places.id, places.destination_id, places.name, places.category, places.kinds,
	               places.address, places.lat, places.lng, places.rating, places.rating_count,
	               places.price_min_vnd, places.price_max_vnd, places.open_hours, places.open_now,
	               places.travel_minutes, places.distance_km, places.photo_count, places.traits,
	               places.group_fit, ` + activities + `, places.flag, ` + description + `,
	               ` + reviews + `, places.source, places.source_ref, places.license,
	               places.province_code, places.geo_precision, places.geo_evidence,
	               places.source_updated_at, places.source_kind, places.confidence,
	               places.evidence_posts, places.status, places.superseded_by,
	               places.created_at, places.updated_at
	          FROM places`
}

func (r Repository) listPlaces(ctx context.Context, filter PlaceFilter, slim bool) ([]Place, error) {
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
	sql := placeSelect(slim)
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
		p, err := scanPlace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// PlacesByID reads the places with these ids, in id order; an id with no row
// is simply absent. Not a Python port: the retrieval index (internal/rag)
// hydrates its ranked candidates from live rows with it, so a hit is always
// checked against the catalogue as it is now.
func (r Repository) PlacesByID(ctx context.Context, ids []string) ([]Place, error) {
	out := []Place{}
	if len(ids) == 0 {
		return out, nil
	}
	// placeSelect, not a column list of its own: scanPlace reads every mapped
	// column, so a second list drifts the moment the table gains one.
	rows, err := r.Q.Query(ctx, placeSelect(false)+` WHERE places.id = ANY($1::text[]) ORDER BY places.id COLLATE "C"`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPlace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// scanPlace reads one row of every mapped column in declaration order and
// builds the record the way _place_record does.
func scanPlace(row pgx.Row) (Place, error) {
	var p Place
	var kinds, traits, groupFit, activities, reviews []byte
	var created, updated any
	// Mapped columns the record does not carry: read because select(Place)
	// reads every mapped column, and the oracle compares the statement.
	var provinceCode, geoEvidence, sourceUpdated, sourceKind, confidence, evidencePosts, status, supersededBy any
	if err := row.Scan(&p.ID, &p.DestinationID, &p.Name, &p.Category, &kinds,
		&p.Address, &p.Lat, &p.Lng, &p.Rating, &p.RatingCount,
		&p.PriceMinVND, &p.PriceMaxVND, &p.OpenHours, &p.OpenNow,
		&p.TravelMinutes, &p.DistanceKM, &p.PhotoCount, &traits,
		&groupFit, &activities, &p.Flag, &p.Description,
		&reviews, &p.Source, &p.SourceRef, &p.License,
		&provinceCode, &p.GeoPrecision, &geoEvidence, &sourceUpdated, &sourceKind,
		&confidence, &evidencePosts, &status, &supersededBy,
		&created, &updated); err != nil {
		return Place{}, err
	}
	var err error
	if p.Kinds, err = pythonListOrEmpty(kinds); err != nil {
		return Place{}, err
	}
	if p.Traits, err = pythonListOrEmpty(traits); err != nil {
		return Place{}, err
	}
	p.GroupFit = jsonOrNone(groupFit)
	p.Activities = jsonOrNone(activities)
	p.Reviews = jsonOrNone(reviews)
	return p, nil
}

// GetPlace is get_place: `session.get(Place, id)`, every mapped column
// labelled table_column, nil when there is none, then the same record
// building as ListPlaces (so a stored kinds of `true` is ErrPythonTypeError
// here too). ApiService.place_row reaches it on a fresh service, whose
// catalogue cache is still empty.
//
// SQLAlchemy note: a second session.get of the same id in one session answers
// from the identity map without a statement; Go reads again.
func (r Repository) GetPlace(ctx context.Context, placeID string) (*Place, error) {
	p, err := scanPlace(r.Q.QueryRow(ctx,
		`SELECT places.id AS places_id, places.destination_id AS places_destination_id, places.name AS places_name,
		        places.category AS places_category, places.kinds AS places_kinds, places.address AS places_address,
		        places.lat AS places_lat, places.lng AS places_lng, places.rating AS places_rating,
		        places.rating_count AS places_rating_count, places.price_min_vnd AS places_price_min_vnd,
		        places.price_max_vnd AS places_price_max_vnd, places.open_hours AS places_open_hours,
		        places.open_now AS places_open_now, places.travel_minutes AS places_travel_minutes,
		        places.distance_km AS places_distance_km, places.photo_count AS places_photo_count,
		        places.traits AS places_traits, places.group_fit AS places_group_fit,
		        places.activities AS places_activities, places.flag AS places_flag,
		        places.description AS places_description, places.reviews AS places_reviews,
		        places.source AS places_source, places.source_ref AS places_source_ref,
		        places.license AS places_license, places.province_code AS places_province_code,
		        places.geo_precision AS places_geo_precision, places.geo_evidence AS places_geo_evidence,
		        places.source_updated_at AS places_source_updated_at, places.source_kind AS places_source_kind,
		        places.confidence AS places_confidence, places.evidence_posts AS places_evidence_posts,
		        places.status AS places_status, places.superseded_by AS places_superseded_by,
		        places.created_at AS places_created_at,
		        places.updated_at AS places_updated_at
		   FROM places
		  WHERE places.id = $1::VARCHAR`, placeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
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
