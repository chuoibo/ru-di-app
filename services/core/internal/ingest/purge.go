package ingest

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PurgeReport says what removing the development catalogue did, row by row.
type PurgeReport struct {
	SeedPlaces    int64
	Photos        int64
	QueuedObjects int64
	SavedPlaces   int64
	OutingStops   int64
	Memories      int64
	Destinations  int64
}

// ErrNoFedCatalogue refuses a purge that would leave the catalogue empty.
var ErrNoFedCatalogue = fmt.Errorf("no fed (vnlocal) place is active yet: " +
	"load and apply a delivery before removing the development catalogue")

// PurgeDevCatalogue removes the invented catalogue -- the seed places and the
// curated destinations that only they filled -- from a database that now holds
// the fed one.
//
// It runs inside the caller's transaction, so the command can show exactly
// what would change and roll back, and a test can do the same.
//
// What it touches, and why each is handled the way it is:
//   - seed places and their photos are deleted; the photos' objects are queued
//     in `pending_object_deletes` rather than unlinked here, because a file
//     removed inside a transaction that then rolls back is gone anyway
//   - a bookmark to a seed place is deleted: it points at somewhere that does
//     not exist, and there is nothing else to keep
//   - an outing stop or a memory that names a seed place keeps its row and its
//     `place_name`, and loses only the link: what the group wrote down is
//     theirs, the catalogue row was ours
//   - a curated destination is deleted once nothing points at it; the province
//     destinations (`d-tinh-*`) are the ones the fed rows use
//
// It refuses rather than guesses in two cases: when no fed row is active (the
// catalogue would be left empty), and when a place that is not seed still sits
// under a curated destination (an OSM import, say) -- deleting its destination
// would orphan a real row, and moving it is a decision, not a cleanup.
func PurgeDevCatalogue(ctx context.Context, tx pgx.Tx) (PurgeReport, error) {
	var report PurgeReport

	var fed int64
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM places
		WHERE source = 'vnlocal' AND status = 'active'`).Scan(&fed); err != nil {
		return report, err
	}
	if fed == 0 {
		return report, ErrNoFedCatalogue
	}

	blocking, err := placesUnderCuratedDestinations(ctx, tx)
	if err != nil {
		return report, err
	}
	if len(blocking) > 0 {
		return report, fmt.Errorf("places that are not seed sit under a curated "+
			"destination; move or remove them first: %s", strings.Join(blocking, ", "))
	}

	steps := []struct {
		count *int64
		sql   string
	}{
		{&report.QueuedObjects, `
			INSERT INTO pending_object_deletes (storage_key, reason)
			SELECT pp.storage_key, 'dev_catalogue_purge'
			FROM place_photos pp JOIN places p ON p.id = pp.place_id
			WHERE p.source = 'seed' AND pp.storage_key ~ '^[0-9a-f]{32}$'
			ON CONFLICT (storage_key) DO NOTHING`},
		{&report.Photos, `
			DELETE FROM place_photos
			WHERE place_id IN (SELECT id FROM places WHERE source = 'seed')`},
		{&report.SavedPlaces, `
			DELETE FROM saved_places
			WHERE place_id IN (SELECT id FROM places WHERE source = 'seed')`},
		{&report.OutingStops, `
			UPDATE outing_stops SET place_id = NULL
			WHERE place_id IN (SELECT id FROM places WHERE source = 'seed')`},
		{&report.Memories, `
			UPDATE memories SET place_id = NULL
			WHERE place_id IN (SELECT id FROM places WHERE source = 'seed')`},
		{nil, `
			UPDATE places SET superseded_by = NULL, status = 'active'
			WHERE superseded_by IN (SELECT id FROM places WHERE source = 'seed')`},
		{&report.SeedPlaces, `DELETE FROM places WHERE source = 'seed'`},
		{&report.Destinations, `
			DELETE FROM destinations d
			WHERE d.id NOT LIKE 'd-tinh-%'
			  AND NOT EXISTS (SELECT 1 FROM places p WHERE p.destination_id = d.id)`},
	}
	for _, step := range steps {
		tag, err := tx.Exec(ctx, step.sql)
		if err != nil {
			return report, err
		}
		if step.count != nil {
			*step.count = tag.RowsAffected()
		}
	}
	return report, nil
}

// placesUnderCuratedDestinations lists "destination: n" for every curated
// destination that still holds a place which is not seed.
func placesUnderCuratedDestinations(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT destination_id, source, count(*) FROM places
		WHERE source <> 'seed' AND destination_id NOT LIKE 'd-tinh-%'
		GROUP BY destination_id, source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var destination, source string
		var n int64
		if err := rows.Scan(&destination, &source, &n); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%s: %d %s", destination, n, source))
	}
	sort.Strings(out)
	return out, rows.Err()
}
