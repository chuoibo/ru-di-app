//go:build postgres

package janitor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/testdb"
)

// ownTables is a pool on a schema of its own holding empty copies of the
// tables Purge sweeps. Purge deletes by age across the whole table, and this
// test's clock is years ahead, so run on the shared public schema it deleted
// every live session of whatever package ran beside it (401s in community
// and diary tests of go_postgres_tier.sh, seen 2026-10-01).
func ownTables(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := fmt.Sprintf("janitor_test_%d", time.Now().UnixNano())
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	for _, table := range []string{"people", "account_sessions", "otp_challenges", "idempotency_keys"} {
		if _, err := base.Exec(ctx, fmt.Sprintf("CREATE TABLE %s.%s (LIKE public.%s INCLUDING ALL)", ident, table, table)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestPurgeRemovesOnlyWhatHasExpired: old rows go, recent and live rows stay.
func TestPurgeRemovesOnlyWhatHasExpired(t *testing.T) {
	ctx := context.Background()
	pool := ownTables(t)
	now := time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)
	old, recent := now.Add(-60*24*time.Hour), now.Add(-time.Hour)
	person := "7a111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	clean := func() {
		exec(`DELETE FROM account_sessions WHERE person_id = $1`, person)
		exec(`DELETE FROM otp_challenges WHERE phone_digest IN ('\x01'::bytea, '\x02'::bytea)`)
		exec(`DELETE FROM idempotency_keys WHERE scope = 'janitor-test'`)
		exec(`DELETE FROM people WHERE id = $1`, person)
	}
	clean()
	t.Cleanup(clean)
	exec(`INSERT INTO people (id, display_name) VALUES ($1, 'Janitor test')`, person)
	exec(`INSERT INTO otp_challenges (id, phone_digest, code_digest, created_at, expires_at, attempts)
	      VALUES (gen_random_uuid(), '\x01'::bytea, '\x00'::bytea, $1::timestamptz, $1::timestamptz + interval '5 minutes', 0),
	             (gen_random_uuid(), '\x02'::bytea, '\x00'::bytea, $2::timestamptz, $2::timestamptz + interval '5 minutes', 0)`, old, recent)
	exec(`INSERT INTO account_sessions (id, person_id, token_digest, issued_via, created_at, expires_at)
	      VALUES (gen_random_uuid(), $1, decode(md5('a'), 'hex') || decode(md5('b'), 'hex'), 'otp', $2::timestamptz, $2::timestamptz + interval '1 day'),
	             (gen_random_uuid(), $1, decode(md5('c'), 'hex') || decode(md5('d'), 'hex'), 'otp', $2, $3)`,
		person, old, now.Add(24*time.Hour))
	key := func(name string, created time.Time, done bool) {
		if done {
			exec(`INSERT INTO idempotency_keys (id, scope, idempotency_key, request_fingerprint, created_at,
			        completed_at, response_status, response_body, response_media_type)
			      VALUES (gen_random_uuid(), 'janitor-test', $1, 'f', $2, $2, 201, '\x7b7d'::bytea, 'application/json')`,
				name, created)
			return
		}
		exec(`INSERT INTO idempotency_keys (id, scope, idempotency_key, request_fingerprint, created_at)
		      VALUES (gen_random_uuid(), 'janitor-test', $1, 'f', $2)`, name, created)
	}
	key("done-old", old, true)
	key("done-new", recent, true)
	key("claim-old", old, false)
	key("claim-new", recent, false)

	report, err := Purge(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	count := func(sql string, args ...any) int {
		var n int
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM otp_challenges WHERE phone_digest IN ('\x01'::bytea, '\x02'::bytea)`); n != 1 {
		t.Errorf("otp rows left = %d, want the recent one", n)
	}
	if n := count(`SELECT count(*) FROM account_sessions WHERE person_id = $1`, person); n != 1 {
		t.Errorf("sessions left = %d, want the live one", n)
	}
	var keys []string
	rows, _ := pool.Query(ctx, `SELECT idempotency_key FROM idempotency_keys WHERE scope='janitor-test' ORDER BY 1`)
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		keys = append(keys, k)
	}
	rows.Close()
	if len(keys) != 2 || keys[0] != "claim-new" || keys[1] != "done-new" {
		t.Errorf("idempotency keys left = %v, want [claim-new done-new]", keys)
	}
	if report.OTPChallenges < 1 || report.Sessions < 1 || report.IdempotencyKeys < 2 {
		t.Errorf("report = %+v", report)
	}
}

// TestReapUnlinksOnlyUnreferencedQueuedFiles: a queued key no row names is
// unlinked and dequeued; one a photo row still names is dequeued and kept.
func TestReapUnlinksOnlyUnreferencedQueuedFiles(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if err := ingest.Migrate(ctx, pool); err != nil { // owns pending_object_deletes
		t.Fatal(err)
	}
	const gone, kept, place = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "reap-test-place"
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	clean := func() {
		exec(`DELETE FROM pending_object_deletes WHERE storage_key IN ($1, $2)`, gone, kept)
		exec(`DELETE FROM place_photos WHERE place_id = $1`, place)
		exec(`DELETE FROM places WHERE id = $1`, place)
		exec(`DELETE FROM destinations WHERE id = 'd-reap-test'`)
	}
	clean()
	t.Cleanup(clean)
	exec(`INSERT INTO destinations (id, name, lat, lng, bbox_south, bbox_west, bbox_north, bbox_east)
	      VALUES ('d-reap-test', 'Reap', 10, 106, 9, 105, 11, 107)`)
	exec(`INSERT INTO places (id, destination_id, name, category, source) VALUES ($1, 'd-reap-test', 'Reap', 'cafe', 'seed')`, place)
	exec(`INSERT INTO place_photos (id, place_id, storage_key, content_type, byte_size, width, height,
	        author, license, source_url, sort_order)
	      VALUES (gen_random_uuid(), $1, $2, 'image/jpeg', 1, 1, 1, 'A', 'CC BY 4.0', 'https://example.test/r', 0)`, place, kept)
	exec(`INSERT INTO pending_object_deletes (storage_key, reason) VALUES ($1, 'test'), ($2, 'test')`, gone, kept)

	store := fakeStore{gone: true, kept: true}
	report, err := ReapObjects(ctx, pool, store)
	if err != nil {
		t.Fatal(err)
	}
	if report.Deleted != 1 || report.Kept != 1 || store[gone] || !store[kept] {
		t.Fatalf("report %+v, store %v: want the unreferenced file gone and the named one kept", report, store)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pending_object_deletes WHERE storage_key IN ($1, $2)`, gone, kept).Scan(&left); err != nil || left != 0 {
		t.Fatalf("queue rows left = %d, %v", left, err)
	}
}
