//go:build postgres

package idem

// The store against real PostgreSQL, because a map cannot refuse an insert.
// Ports tests/postgres/test_idempotency_postgres.py's store-level claims:
// one winner per key across connections, a byte-for-byte replay, conflicts,
// scope isolation, release, the legacy upgrade, and the unique index itself.
// Run through scripts/go_postgres_tier.sh -- ./internal/db/ ./internal/idem/

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

const (
	pgScope            = "scope-alpha"
	pgOtherScope       = "scope-beta"
	pgFingerprint      = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pgOtherFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func freshKey(t *testing.T) string {
	t.Helper()
	id, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("key-%x", id.Bytes)
}

func mustReserve(t *testing.T, store *PostgresStore, scope, key, fingerprint, legacy string) Outcome {
	t.Helper()
	outcome, err := store.Reserve(context.Background(), scope, key, fingerprint, legacy)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func storedFingerprint(t *testing.T, pool *pgxpool.Pool, scope, key string) string {
	t.Helper()
	var fingerprint string
	err := pool.QueryRow(context.Background(),
		"SELECT request_fingerprint FROM idempotency_keys WHERE scope = $1 AND idempotency_key = $2",
		scope, key).Scan(&fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func TestPostgresStoreReservesAFreshKeyOnce(t *testing.T) {
	pool := testdb.Pool(t)
	store := NewPostgresStore(pool)
	key := freshKey(t)
	if got := mustReserve(t, store, pgScope, key, pgFingerprint, ""); got.Kind != Reserved {
		t.Fatalf("first reservation: %v", got.Kind)
	}
	if got := mustReserve(t, store, pgScope, key, pgFingerprint, ""); got.Kind != InFlight {
		t.Fatalf("second reservation: %v", got.Kind)
	}
	if got := mustReserve(t, store, pgOtherScope, key, pgFingerprint, ""); got.Kind != Reserved {
		t.Fatalf("same key under another scope: %v", got.Kind)
	}
	if got := mustReserve(t, store, pgScope, key, pgOtherFingerprint, ""); got.Kind != Conflict {
		t.Fatalf("different fingerprint: %v", got.Kind)
	}
}

func TestPostgresStoreReplaysTheCommittedAnswerByteForByte(t *testing.T) {
	pool := testdb.Pool(t)
	store := NewPostgresStore(pool)
	ctx := context.Background()
	mediaType := "text/x-é; charset=utf-8"
	empty := ""
	cases := []StoredResponse{
		{Status: 201, Body: []byte("{\"id\": \"kept\"}\x00\xff"), MediaType: &mediaType},
		{Status: 200, Body: []byte{}, MediaType: nil},
		{Status: 299, Body: nil, MediaType: &empty},
	}
	for _, stored := range cases {
		key := freshKey(t)
		mustReserve(t, store, pgScope, key, pgFingerprint, "")
		if err := store.Complete(ctx, pgScope, key, stored); err != nil {
			t.Fatal(err)
		}
		got := mustReserve(t, store, pgScope, key, pgFingerprint, "")
		if got.Kind != Replay || got.Response.Status != stored.Status ||
			!bytes.Equal(got.Response.Body, stored.Body) || got.Response.Body == nil ||
			(got.Response.MediaType == nil) != (stored.MediaType == nil) ||
			(stored.MediaType != nil && *got.Response.MediaType != *stored.MediaType) {
			t.Fatalf("replay %+v, stored %+v", got, stored)
		}
		var bodyIsNull, completed bool
		err := pool.QueryRow(ctx, `SELECT response_body IS NULL, completed_at IS NOT NULL
FROM idempotency_keys WHERE scope = $1 AND idempotency_key = $2`, pgScope, key).Scan(&bodyIsNull, &completed)
		if err != nil {
			t.Fatal(err)
		}
		if bodyIsNull || !completed {
			t.Fatalf("row: body null %v, completed %v (Python stores b\"\", never NULL)", bodyIsNull, completed)
		}
	}
}

func TestPostgresStoreReleaseFreesTheKey(t *testing.T) {
	pool := testdb.Pool(t)
	store := NewPostgresStore(pool)
	key := freshKey(t)
	mustReserve(t, store, pgScope, key, pgFingerprint, "")
	if err := store.Release(context.Background(), pgScope, key); err != nil {
		t.Fatal(err)
	}
	if got := mustReserve(t, store, pgScope, key, pgOtherFingerprint, ""); got.Kind != Reserved {
		t.Fatalf("after release: %v", got.Kind)
	}
}

func TestPostgresStoreAdoptsALegacyFingerprintInPlace(t *testing.T) {
	pool := testdb.Pool(t)
	store := NewPostgresStore(pool)
	ctx := context.Background()

	// Completed row holding the raw digest: replayed, and healed.
	key := freshKey(t)
	mustReserve(t, store, pgScope, key, pgOtherFingerprint, "")
	if err := store.Complete(ctx, pgScope, key, StoredResponse{Status: 201, Body: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	if got := mustReserve(t, store, pgScope, key, pgFingerprint, pgOtherFingerprint); got.Kind != Replay {
		t.Fatalf("legacy row: %v", got.Kind)
	}
	if got := storedFingerprint(t, pool, pgScope, key); got != pgFingerprint {
		t.Fatalf("fingerprint not adopted: %s", got)
	}

	// In-flight row: still refused as in flight, but healed all the same.
	key = freshKey(t)
	mustReserve(t, store, pgScope, key, pgOtherFingerprint, "")
	if got := mustReserve(t, store, pgScope, key, pgFingerprint, pgOtherFingerprint); got.Kind != InFlight {
		t.Fatalf("legacy in-flight row: %v", got.Kind)
	}
	if got := storedFingerprint(t, pool, pgScope, key); got != pgFingerprint {
		t.Fatalf("in-flight fingerprint not adopted: %s", got)
	}

	// A legacy digest that does not match is no licence to replay.
	key = freshKey(t)
	mustReserve(t, store, pgScope, key, pgOtherFingerprint, "")
	if got := mustReserve(t, store, pgScope, key, pgFingerprint, "cccc"); got.Kind != Conflict {
		t.Fatalf("unrelated legacy digest: %v", got.Kind)
	}
	if got := storedFingerprint(t, pool, pgScope, key); got != pgOtherFingerprint {
		t.Fatalf("a refused row was rewritten: %s", got)
	}
}

func TestTheDatabaseItselfRefusesADuplicateRow(t *testing.T) {
	pool := testdb.Pool(t)
	key := freshKey(t)
	insert := `INSERT INTO idempotency_keys (id, scope, idempotency_key, request_fingerprint)
VALUES (gen_random_uuid(), $1, $2, $3)`
	if _, err := pool.Exec(context.Background(), insert, pgScope, key, pgFingerprint); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(context.Background(), insert, pgScope, key, pgFingerprint)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "uq_idempotency_keys_scope_key" {
		t.Fatalf("duplicate insert: %v", err)
	}
}

func TestConcurrentReservationsProduceExactlyOneWinner(t *testing.T) {
	pool := testdb.Pool(t)
	store := NewPostgresStore(pool)
	key := freshKey(t)
	const racers = 12
	outcomes := make([]Kind, racers)
	errs := make([]error, racers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range racers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			outcome, err := store.Reserve(context.Background(), pgScope, key, pgFingerprint, "")
			outcomes[i], errs[i] = outcome.Kind, err
		}(i)
	}
	close(start)
	wg.Wait()
	winners := 0
	for i := range racers {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		switch outcomes[i] {
		case Reserved:
			winners++
		case InFlight:
		default:
			t.Fatalf("racer %d: %v", i, outcomes[i])
		}
	}
	if winners != 1 {
		t.Fatalf("%d winners", winners)
	}
}

// completionWatcher reads the row on its own connection the moment the answer
// starts: only committed work is visible there, as it is to a second press.
type completionWatcher struct {
	http.ResponseWriter
	pool      *pgxpool.Pool
	key       string
	completed *bool
}

func (w *completionWatcher) WriteHeader(code int) {
	var done bool
	err := w.pool.QueryRow(context.Background(),
		"SELECT completed_at IS NOT NULL FROM idempotency_keys WHERE idempotency_key = $1", w.key).Scan(&done)
	*w.completed = err == nil && done
	w.ResponseWriter.WriteHeader(code)
}

func TestTheAnswerIsReleasedOnlyAfterTheCompletionHasCommitted(t *testing.T) {
	pool := testdb.Pool(t)
	key := freshKey(t)
	middleware := New(NewPostgresStore(pool))(http.HandlerFunc(answer201))
	var completed bool
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/expenses", strings.NewReader(`{}`))
	r.Header.Set(HeaderName, key)
	r.Header.Set("X-Actor-ID", pgScope)
	r.Header.Set("Content-Type", "application/json")
	middleware.ServeHTTP(&completionWatcher{ResponseWriter: rec, pool: pool, key: key, completed: &completed}, r)
	if rec.Code != http.StatusCreated || !completed {
		t.Fatalf("code %d; the caller held the answer while the key read as unfinished: %v", rec.Code, !completed)
	}

	// Another spelling of the same JSON: canonicalised, so it replays.
	second := httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/expenses", strings.NewReader(`{ }`))
	r.Header.Set(HeaderName, key)
	r.Header.Set("X-Actor-ID", pgScope)
	r.Header.Set("Content-Type", "application/json")
	middleware.ServeHTTP(second, r)
	if second.Code != http.StatusCreated || second.Header().Get(ReplayHeaderName) != "true" ||
		second.Body.String() != `{"id": "first"}` {
		t.Fatalf("replay %d %v %q", second.Code, second.Header(), second.Body.String())
	}
}
