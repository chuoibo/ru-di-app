//go:build postgres

package ingest

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/testdb"
)

const probeDestination = `
	INSERT INTO destinations (id, name, lat, lng,
	  bbox_south, bbox_west, bbox_north, bbox_east)
	VALUES ('d-probe', 'Probe', 10.0, 106.0, 9.0, 105.0, 11.0, 107.0)
	ON CONFLICT (id) DO NOTHING`

// migrated runs the migration once and hands back a transaction that rolls
// itself back, so no test leaves a row behind for the next one.
func migrated(t *testing.T) (pgx.Tx, context.Context) {
	t.Helper()
	ctx := context.Background()
	if err := Migrate(ctx, testdb.Pool(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tx := testdb.Tx(t)
	if _, err := tx.Exec(ctx, probeDestination); err != nil {
		t.Fatalf("destination: %v", err)
	}
	return tx, ctx
}

// mustReject asserts the database refuses a statement, and that it refuses it
// for the named constraint rather than for some unrelated reason. A test that
// only asserts "an error happened" still passes when the row was rejected
// because a column name was misspelled.
func mustReject(t *testing.T, tx pgx.Tx, constraint, sql string) {
	t.Helper()
	_, err := tx.Exec(context.Background(), sql)
	if err == nil {
		t.Fatalf("expected %s to refuse the row", constraint)
	}
	if !strings.Contains(err.Error(), constraint) {
		t.Fatalf("expected %s, got: %v", constraint, err)
	}
}

// TestMigrateIsIdempotent runs the migration twice against a schema Alembic has
// already migrated. A second run must be a no-op rather than an error: the
// deployment command is allowed to be called again, and each package that needs
// these tables calls it during setup.
func TestMigrateIsIdempotent(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, table := range []string{
		"ingest_batch",
		"ingest_place_raw",
		"ingest_reject",
		"place_source_post",
		"place_override",
		"admin_province",
		"pending_object_deletes",
	} {
		var present bool
		if err := pool.QueryRow(ctx,
			`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&present); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if !present {
			t.Errorf("%s was not created", table)
		}
	}
}

// TestCatalogueRefusesAHalfTruth covers the rules this change exists for. Each
// one guards a statement the app would otherwise make to a person without
// having the facts behind it.
func TestCatalogueRefusesAHalfTruth(t *testing.T) {
	t.Run("a point must say how it was arrived at", func(t *testing.T) {
		tx, _ := migrated(t)
		mustReject(t, tx, "place_point_states_its_precision", `
			INSERT INTO places (id, destination_id, name, category, source,
			  source_ref, lat, lng)
			VALUES ('p-1', 'd-probe', 'Probe', 'cafe', 'vnlocal', 'plc_1',
			  10.5, 106.5)`)
	})

	t.Run("half a point is a broken row, not a partial answer", func(t *testing.T) {
		tx, _ := migrated(t)
		mustReject(t, tx, "place_point_is_whole", `
			INSERT INTO places (id, destination_id, name, category, source,
			  source_ref, lat, geo_precision)
			VALUES ('p-2', 'd-probe', 'Probe', 'cafe', 'vnlocal', 'plc_2',
			  10.5, 'rooftop')`)
	})

	t.Run("a merged row must name its successor", func(t *testing.T) {
		tx, _ := migrated(t)
		mustReject(t, tx, "place_superseded_names_successor", `
			INSERT INTO places (id, destination_id, name, category, source,
			  source_ref, status)
			VALUES ('p-3', 'd-probe', 'Probe', 'cafe', 'vnlocal', 'plc_3',
			  'superseded')`)
	})

	t.Run("a feed row must point back at what produced it", func(t *testing.T) {
		tx, _ := migrated(t)
		mustReject(t, tx, "place_vnlocal_row_cites_its_source", `
			INSERT INTO places (id, destination_id, name, category, source)
			VALUES ('p-4', 'd-probe', 'Probe', 'cafe', 'vnlocal')`)
	})

	t.Run("an invented precision is refused", func(t *testing.T) {
		tx, _ := migrated(t)
		mustReject(t, tx, "place_geo_precision_known", `
			INSERT INTO places (id, destination_id, name, category, source,
			  source_ref, lat, lng, geo_precision)
			VALUES ('p-6', 'd-probe', 'Probe', 'cafe', 'vnlocal', 'plc_6',
			  10.5, 106.5, 'probably')`)
	})

	t.Run("a place with no coordinates at all is allowed", func(t *testing.T) {
		tx, ctx := migrated(t)
		if _, err := tx.Exec(ctx, `
			INSERT INTO places (id, destination_id, name, category, source,
			  source_ref)
			VALUES ('p-5', 'd-probe', 'Probe', 'cafe', 'vnlocal', 'plc_5')`,
		); err != nil {
			t.Fatalf("a place without coordinates must be storable: %v", err)
		}
		var status string
		if err := tx.QueryRow(ctx,
			`SELECT status FROM places WHERE id='p-5'`).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "active" {
			t.Errorf("status defaulted to %q, want active", status)
		}
	})
}

// TestCorrectionsSurviveAReload is the reason place_override exists: a reload
// must not be able to quietly undo a person's edit, so corrections are stored
// beside the fed row rather than written into it.
func TestCorrectionsSurviveAReload(t *testing.T) {
	tx, ctx := migrated(t)
	if _, err := tx.Exec(ctx, `
		INSERT INTO places (id, destination_id, name, category, source, source_ref)
		VALUES ('p-ov', 'd-probe', 'Fed name', 'cafe', 'vnlocal', 'plc_ov')`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO place_override (place_id, field, value, actor, reason)
		VALUES ('p-ov', 'name', '"Corrected name"'::jsonb, 'operator',
		  'model mangled the name')`); err != nil {
		t.Fatalf("override: %v", err)
	}
	// The feed writes the base row again, as a second delivery would.
	if _, err := tx.Exec(ctx,
		`UPDATE places SET name='Fed name again' WHERE id='p-ov'`); err != nil {
		t.Fatal(err)
	}
	var value string
	if err := tx.QueryRow(ctx,
		`SELECT value #>> '{}' FROM place_override
		 WHERE place_id='p-ov' AND field='name'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "Corrected name" {
		t.Errorf("the correction was lost: got %q", value)
	}

	// A correction with no stated reason is not a correction, it is a change
	// nobody can review later.
	mustReject(t, tx, "place_override_reason_check", `
		INSERT INTO place_override (place_id, field, value, actor, reason)
		VALUES ('p-ov', 'address', '"x"'::jsonb, 'operator', 'typo')`)
}

// TestOnePostCanDescribeSeveralPlaces locks the shape of the identity table.
// A "five best places" video is one post about five places, so a unique
// constraint on the post alone would have silently dropped four of them.
//
// The post id here is invented rather than realistic: a real one is nineteen
// digits, and real platform identifiers do not belong in the repository. What
// this test measures is the key, which is text and has no format rule.
func TestOnePostCanDescribeSeveralPlaces(t *testing.T) {
	tx, ctx := migrated(t)
	if _, err := tx.Exec(ctx, `
		INSERT INTO places (id, destination_id, name, category, source, source_ref)
		VALUES ('p-a', 'd-probe', 'A', 'cafe', 'vnlocal', 'plc_a'),
		       ('p-b', 'd-probe', 'B', 'cafe', 'vnlocal', 'plc_b')`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO place_source_post (platform, post_id, place_id)
		VALUES ('tiktok', 'post-five-places', 'p-a'),
		       ('tiktok', 'post-five-places', 'p-b')`); err != nil {
		t.Fatalf("one post must be allowed to describe two places: %v", err)
	}

	// The same pairing twice is still one fact.
	if _, err := tx.Exec(ctx, `
		INSERT INTO place_source_post (platform, post_id, place_id)
		VALUES ('tiktok', 'post-five-places', 'p-a')`); err == nil {
		t.Error("the same post/place pairing was accepted twice")
	}
}

// TestSeedProvincesIsIdempotent writes the province list twice. The second run
// must change nothing: the seed is called on every deployment, and a lookup
// table that grows a duplicate row on each one is a lookup table that starts
// returning two answers.
func TestSeedProvincesIsIdempotent(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := SeedProvinces(ctx, pool); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if _, err := SeedProvinces(ctx, pool); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM admin_province`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != len(Provinces) {
		t.Fatalf("admin_province holds %d rows, want %d", rows, len(Provinces))
	}
	if rows != 34 {
		t.Errorf("the 2025 reorganisation has 34 provinces, found %d", rows)
	}
	// The codes are the official ones, not a sequence somebody invented: Ho Chi
	// Minh City is 79 and Ha Noi is 1. Sharing this numbering with the feed is
	// what makes its province_code join.
	var name string
	if err := pool.QueryRow(ctx,
		`SELECT name FROM admin_province WHERE code = 79`).Scan(&name); err != nil {
		t.Fatalf("code 79 must exist: %v", err)
	}
	if !strings.Contains(name, "Hồ Chí Minh") {
		t.Errorf("code 79 is %q, expected Ho Chi Minh City", name)
	}
}

// TestProvinceDestinationsDoNotDisturbTheCuratedOnes. The catalogue already
// holds fifteen destinations somebody wrote descriptions for, and one of them
// is Ho Chi Minh City. A province row that claimed the same id would replace
// it, so province ids are prefixed rather than slugged from the name.
func TestProvinceDestinationsDoNotDisturbTheCuratedOnes(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tx := testdb.Tx(t)
	if _, err := tx.Exec(ctx, `
		INSERT INTO destinations (id, name, province, lat, lng,
		  bbox_south, bbox_west, bbox_north, bbox_east, blurb, sort_order)
		VALUES ('d-tphcm', 'TP. Hồ Chí Minh', 'TP. Hồ Chí Minh',
		  10.7769, 106.7009, 10.72, 106.65, 10.83, 106.75,
		  'Một câu ai đó viết tay', 20)
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	for _, box := range ProvinceBoxes {
		if id := ProvinceDestinationID(box.Code); id == "d-tphcm" {
			t.Fatalf("province %d claims the curated id %s", box.Code, id)
		}
	}

	// Seeding runs on the pool, outside this transaction, so it is done last
	// and cleaned up by id.
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM destinations WHERE id LIKE 'd-tinh-%'`)
	})
	if _, err := SeedProvinceDestinations(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := SeedProvinceDestinations(ctx, pool); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM destinations WHERE id LIKE 'd-tinh-%'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != len(ProvinceBoxes) {
		t.Errorf("%d province destinations, want %d", rows, len(ProvinceBoxes))
	}
	// Bac Ninh has no boundary in the extract and is written down as missing
	// rather than quietly absent: a place there cannot be projected yet, and
	// the next person needs to know it is the boundary that is missing.
	if _, noted := ProvincesWithoutBox[24]; !noted {
		t.Error("a province without a box must say so")
	}
}
