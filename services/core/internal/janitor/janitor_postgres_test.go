//go:build postgres

package janitor

import (
	"context"
	"testing"
	"time"

	"mobile/services/core/internal/testdb"
)

// TestPurgeRemovesOnlyWhatHasExpired: old rows go, recent and live rows stay.
func TestPurgeRemovesOnlyWhatHasExpired(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
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
