//go:build postgres

package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// devCatalogue lays down the shape a development database has after a real
// delivery was applied on top of the seed: a curated destination holding a
// seed place (with a photo, a bookmark, a stop and a memory pointing at it),
// and a province destination holding a fed place.
func devCatalogue(t *testing.T, tx pgx.Tx, ctx context.Context) (person string) {
	t.Helper()
	person = "5eed5eed-dead-4bee-8fed-cafebabef00d"
	for _, sql := range []string{
		`INSERT INTO destinations (id, name, lat, lng,
		   bbox_south, bbox_west, bbox_north, bbox_east)
		 VALUES ('d-tinh-79', 'Thành phố Hồ Chí Minh', 10.8, 106.7, 10.3, 106.3, 11.2, 107.1)
		 ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO places (id, destination_id, name, category, source,
		   lat, lng, geo_precision)
		 VALUES ('seed-1', 'd-probe', 'Quán Bịa', 'cafe', 'seed', 10.5, 106.5, 'rooftop')`,
		`INSERT INTO places (id, destination_id, name, category, source, source_ref)
		 VALUES ('vnl-1', 'd-tinh-79', 'Quán Thật', 'cafe', 'vnlocal', 'plc_1')`,
		`INSERT INTO place_photos (id, place_id, storage_key, content_type,
		   byte_size, width, height, author, license, source_url, sort_order)
		 VALUES (gen_random_uuid(), 'seed-1', '0123456789abcdef0123456789abcdef',
		   'image/jpeg', 10, 1, 1, 'A', 'CC BY 4.0', 'https://example.test/a', 0)`,
		`INSERT INTO people (id, display_name) VALUES ('` + person + `', 'Synthetic purge person')`,
		`INSERT INTO saved_places (person_id, place_id) VALUES ('` + person + `', 'seed-1')`,
		`INSERT INTO saved_places (person_id, place_id) VALUES ('` + person + `', 'vnl-1')`,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("fixture: %v\n%s", err, sql)
		}
	}
	return person
}

func count(t *testing.T, tx pgx.Tx, ctx context.Context, sql string) int64 {
	t.Helper()
	var n int64
	if err := tx.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

// TestPurgeRefusesToEmptyTheCatalogue. Removing the seed before a delivery was
// applied leaves a catalogue with nothing in it, which is a broken app rather
// than a clean one.
func TestPurgeRefusesToEmptyTheCatalogue(t *testing.T) {
	tx, ctx := migrated(t)
	if _, err := tx.Exec(ctx, `
		INSERT INTO places (id, destination_id, name, category, source)
		VALUES ('seed-only', 'd-probe', 'Quán Bịa', 'cafe', 'seed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := PurgeDevCatalogue(ctx, tx); !errors.Is(err, ErrNoFedCatalogue) {
		t.Fatalf("err = %v, want ErrNoFedCatalogue", err)
	}
	if n := count(t, tx, ctx, `SELECT count(*) FROM places WHERE id = 'seed-only'`); n != 1 {
		t.Error("a refused purge still deleted the seed")
	}
}

// TestPurgeRemovesOnlyTheInventedCatalogue. The fed row, its province
// destination and the person's bookmark to it survive; everything that only
// existed because of the seed goes, and the photo's object is queued rather
// than lost.
func TestPurgeRemovesOnlyTheInventedCatalogue(t *testing.T) {
	tx, ctx := migrated(t)
	person := devCatalogue(t, tx, ctx)

	report, err := PurgeDevCatalogue(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if report.SeedPlaces != 1 || report.Photos != 1 || report.QueuedObjects != 1 ||
		report.SavedPlaces != 1 || report.Destinations < 1 {
		t.Errorf("report = %+v", report)
	}

	for _, check := range []struct {
		sql  string
		want int64
	}{
		{`SELECT count(*) FROM places WHERE source = 'seed'`, 0},
		{`SELECT count(*) FROM places WHERE id = 'vnl-1'`, 1},
		{`SELECT count(*) FROM destinations WHERE id = 'd-probe'`, 0},
		{`SELECT count(*) FROM destinations WHERE id = 'd-tinh-79'`, 1},
		{`SELECT count(*) FROM place_photos WHERE place_id = 'seed-1'`, 0},
		{`SELECT count(*) FROM pending_object_deletes
		  WHERE storage_key = '0123456789abcdef0123456789abcdef'
		    AND reason = 'dev_catalogue_purge'`, 1},
		{`SELECT count(*) FROM saved_places WHERE person_id = '` + person + `' AND place_id = 'vnl-1'`, 1},
		{`SELECT count(*) FROM saved_places WHERE place_id = 'seed-1'`, 0},
	} {
		if got := count(t, tx, ctx, check.sql); got != check.want {
			t.Errorf("%d, want %d:\n%s", got, check.want, check.sql)
		}
	}

	// Running it again is a no-op, not an error: the operator may re-run it.
	again, err := PurgeDevCatalogue(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if again != (PurgeReport{}) {
		t.Errorf("second purge changed rows: %+v", again)
	}
}

// TestPurgeWillNotOrphanARealRow. A place that is not seed under a curated
// destination (an OSM import) would lose its destination; the purge names it
// and stops instead of choosing where it should go.
func TestPurgeWillNotOrphanARealRow(t *testing.T) {
	tx, ctx := migrated(t)
	devCatalogue(t, tx, ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO places (id, destination_id, name, category, source)
		VALUES ('osm-1', 'd-probe', 'Chợ Thật', 'vui-choi', 'osm')`); err != nil {
		t.Fatal(err)
	}
	_, err := PurgeDevCatalogue(ctx, tx)
	if err == nil || !strings.Contains(err.Error(), "d-probe: 1 osm") {
		t.Fatalf("err = %v, want a refusal naming d-probe", err)
	}
}
