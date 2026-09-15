package repo

// Bookmarks of catalogue places (M2): list_saved_places, save_place and
// unsave_place of SqlAlchemyApiRepository. place_id is plain text with no
// foreign key, so the catalogue is the service's business, not this table's.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SavedPlace is SavedPlaceRecord.
type SavedPlace struct {
	ID        string
	PersonID  string
	PlaceID   string
	CreatedAt time.Time
}

// savedPlaceSavepoint is the name SQLAlchemy gives a session's first begin_nested.
const savedPlaceSavepoint = "sa_savepoint_1"

// ErrSavedPlaceVanished is save_place's `assert winner is not None`: the
// INSERT failed on an integrity rule and no bookmark is there to have won,
// which is what a person with no people row looks like (the foreign key).
var ErrSavedPlaceVanished = errors.New("repo: saved place missing after an integrity error")

const savedPlaceColumns = `saved_places.id, saved_places.person_id, saved_places.place_id, saved_places.created_at`

func scanSavedPlace(row pgx.Row) (*SavedPlace, error) {
	var s SavedPlace
	err := row.Scan(&s.ID, &s.PersonID, &s.PlaceID, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.CreatedAt = s.CreatedAt.UTC()
	return &s, nil
}

// ListSavedPlaces is list_saved_places: newest bookmark first, id breaking ties.
func (r Repository) ListSavedPlaces(ctx context.Context, personID string) ([]SavedPlace, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+savedPlaceColumns+`
		   FROM saved_places
		  WHERE saved_places.person_id = $1::UUID
		  ORDER BY saved_places.created_at DESC, saved_places.id`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedPlace{}
	for rows.Next() {
		s, err := scanSavedPlace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// savedPlace is the `session.scalar` both writers start with (no LIMIT, no
// lock; uq_saved_places_person_place keeps it to one row).
func (r Repository) savedPlace(ctx context.Context, personID, placeID string) (*SavedPlace, error) {
	return scanSavedPlace(r.Q.QueryRow(ctx,
		`SELECT `+savedPlaceColumns+`
		   FROM saved_places
		  WHERE saved_places.person_id = $1::UUID AND saved_places.place_id = $2::VARCHAR`, personID, placeID))
}

// SavePlace is save_place. Returns the bookmark and whether this call made it.
//
// Statements, in Python's order:
//  1. the existing bookmark; when there is one it is the answer, created false;
//  2. SAVEPOINT, one INSERT with a client-side uuid4 and the caller's clock
//     (no RETURNING), RELEASE SAVEPOINT; created true;
//  3. when the INSERT fails: ROLLBACK TO SAVEPOINT. An IntegrityError of any
//     kind reads the bookmark again as «the race's winner», created false, and
//     ErrSavedPlaceVanished when there is none; any other error is returned.
func (r Repository) SavePlace(ctx context.Context, personID, placeID string, now time.Time) (SavedPlace, bool, error) {
	existing, err := r.savedPlace(ctx, personID, placeID)
	if err != nil {
		return SavedPlace{}, false, err
	}
	if existing != nil {
		return *existing, false, nil
	}
	id, err := newUUID()
	if err != nil {
		return SavedPlace{}, false, err
	}
	row := SavedPlace{ID: id, PersonID: personID, PlaceID: placeID, CreatedAt: pythonInstant(now)}
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+savedPlaceSavepoint); err != nil {
		return SavedPlace{}, false, err
	}
	if _, insertErr := r.Q.Exec(ctx,
		`INSERT INTO saved_places (id, person_id, place_id, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::VARCHAR, $4::TIMESTAMP WITH TIME ZONE)`,
		row.ID, row.PersonID, row.PlaceID, row.CreatedAt); insertErr != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+savedPlaceSavepoint); err != nil {
			return SavedPlace{}, false, err
		}
		if integrityViolation(insertErr) == nil {
			return SavedPlace{}, false, insertErr
		}
		winner, err := r.savedPlace(ctx, personID, placeID)
		if err != nil {
			return SavedPlace{}, false, err
		}
		if winner == nil {
			return SavedPlace{}, false, ErrSavedPlaceVanished
		}
		return *winner, false, nil
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+savedPlaceSavepoint); err != nil {
		return SavedPlace{}, false, err
	}
	return row, true, nil
}

// UnsavePlace is unsave_place: the bookmark, false when there is none, else
// the flush of `session.delete(row)` (DELETE by primary key) and true. A
// DELETE that matches nothing (a concurrent unsave) is only SQLAlchemy's
// warning, not an error, so it is none here either.
func (r Repository) UnsavePlace(ctx context.Context, personID, placeID string) (bool, error) {
	row, err := r.savedPlace(ctx, personID, placeID)
	if err != nil || row == nil {
		return false, err
	}
	if _, err := r.Q.Exec(ctx, `DELETE FROM saved_places WHERE saved_places.id = $1::UUID`, row.ID); err != nil {
		return false, err
	}
	return true, nil
}
