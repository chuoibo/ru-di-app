//go:build postgres

package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

// schemaPool is the tier's database with the outbox installed in a schema of
// the test's own.
func schemaPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := fmt.Sprintf("jobs_pg_%d", time.Now().UnixNano())
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if ok, err := SchemaCurrent(ctx, pool); err != nil || !ok {
		t.Fatalf("SchemaCurrent after Migrate: %v %v", ok, err)
	}
	return pool
}

// Two workers ticking the same task at once: one pass runs, the other is
// skipped rather than queued, and the lock is free again after the pass --
// it ended with the pass's transaction, so no pooled connection holds it.
func TestMotLuotRunsOnePassAtATime(t *testing.T) {
	pool := schemaPool(t)
	ctx := context.Background()
	var inside, ran atomic.Int64
	gate := make(chan struct{})
	d := DinhKy{Ten: fmt.Sprintf("test.one_at_a_time_%d", time.Now().UnixNano()), Nhip: time.Second, Chay: func(context.Context, pgx.Tx) error {
		if inside.Add(1) > 1 {
			t.Error("two passes at once")
		}
		<-gate
		inside.Add(-1)
		ran.Add(1)
		return nil
	}}
	held := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `WITH k AS (SELECT hashtextextended('jobs:'||$1,0) AS v)
			SELECT count(*) FROM pg_locks, k WHERE locktype='advisory' AND objsubid=1 AND granted
			AND classid=((k.v >> 32) & x'ffffffff'::bigint)::oid AND objid=(k.v & x'ffffffff'::bigint)::oid`, d.Ten).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := MotLuot(ctx, pool, d)
			if err != nil {
				t.Error(err)
			}
			results <- got
		}()
	}
	// Let one hold the lock, then the other find it taken.
	deadline := time.Now().Add(5 * time.Second)
	for inside.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	// The lock table sees the pass in progress: the check below can be red.
	if n := held(); n != 1 {
		t.Fatalf("lock table shows %d holders during the pass, want 1", n)
	}
	close(gate)
	wg.Wait()
	close(results)
	var yes int
	for r := range results {
		if r {
			yes++
		}
	}
	if yes != 1 || ran.Load() != 1 {
		t.Fatalf("passes ran %d (reported %d), want exactly one", ran.Load(), yes)
	}
	// Released: the next pass runs.
	gate = make(chan struct{})
	close(gate)
	if got, err := MotLuot(ctx, pool, d); err != nil || !got {
		t.Fatalf("the lock was not released: %v %v", got, err)
	}
	// "The next pass ran" alone proves nothing: an advisory lock is reentrant
	// on its backend, and the pool may hand the same connection back. Ask the
	// lock table.
	if n := held(); n != 0 {
		t.Fatalf("advisory lock still held by %d backend(s) after the pass", n)
	}
	// A failing pass reports its error and releases the lock too.
	boom := DinhKy{Ten: d.Ten, Nhip: time.Second, Chay: func(context.Context, pgx.Tx) error { return errors.New("boom") }}
	if got, err := MotLuot(ctx, pool, boom); !got || err == nil {
		t.Fatalf("failing pass: %v %v", got, err)
	}
	if n := held(); n != 0 {
		t.Fatalf("advisory lock still held by %d backend(s) after a failing pass", n)
	}
	if got, err := MotLuot(ctx, pool, d); err != nil || !got {
		t.Fatalf("lock kept after a failing pass: %v %v", got, err)
	}
}

// Every periodic task firing at once on a pool of two, one connection of
// which is busy elsewhere (a job, a LISTEN): each pass runs on the very
// connection that holds its lock, so they queue for the one free connection
// and all finish, instead of each holding one and waiting for a second
// (review of slice 10: at two connections the sweep failed after 5 s and the
// outbox cleanup after 60 s).
func TestDinhKyChayTrenKetNoiGiuKhoa(t *testing.T) {
	base := schemaPool(t)
	cfg := base.Config().Copy()
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	busy, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Release()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stamp := time.Now().UnixNano()
	var ds []DinhKy
	for i := range 4 {
		ds = append(ds, DinhKy{Ten: fmt.Sprintf("test.small_pool_%d_%d", stamp, i), Nhip: time.Minute, Chay: func(ctx context.Context, tx pgx.Tx) error {
			// The lock is this transaction's: the pass sees it on its own
			// backend.
			var mine int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND granted AND pid=pg_backend_pid()`).Scan(&mine); err != nil {
				return err
			}
			if mine != 1 {
				return fmt.Errorf("the pass runs on a backend holding %d advisory locks, want its own one", mine)
			}
			_, err := Don(ctx, tx)
			return err
		}})
	}
	began := time.Now()
	var wg sync.WaitGroup
	errs := make(chan error, len(ds))
	for _, d := range ds {
		wg.Add(1)
		go func(d DinhKy) {
			defer wg.Done()
			ran, err := MotLuot(ctx, pool, d)
			if err == nil && !ran {
				err = fmt.Errorf("%s: skipped with nobody else holding its lock", d.Ten)
			}
			errs <- err
		}(d)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a pass failed on a pool of two with one connection busy, after %v: %v", time.Since(began).Round(time.Millisecond), err)
		}
	}
	t.Logf("4 passes at once, pool of 2, 1 connection busy: all done in %v", time.Since(began).Round(time.Millisecond))
}

// Don removes what nobody will publish or read again, and nothing else.
func TestDonKeepsDueAndRecentRows(t *testing.T) {
	pool := schemaPool(t)
	ctx := context.Background()
	for _, q := range []string{
		`SELECT jobs_them('ai.nep','0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd1',1,clock_timestamp(),clock_timestamp()-interval '1 second')`, // expired: goes
		`SELECT jobs_them('ai.nep','0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd2',1,clock_timestamp(),clock_timestamp()+interval '1 minute')`, // due: stays
		`SELECT jobs_them('ai.nep','0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd3',1,clock_timestamp(),NULL)`,                                  // published long ago: goes
		`SELECT jobs_them('ai.nep','0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd4',1,clock_timestamp(),NULL)`,                                  // published just now: stays
		`UPDATE job_outbox SET published_at=clock_timestamp()-interval '2 hours' WHERE ref_id='0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd3'`,
		`UPDATE job_outbox SET published_at=clock_timestamp() WHERE ref_id='0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd4'`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := Don(ctx, pool); err != nil || n != 2 {
		t.Fatalf("Don removed %d %v, want 2", n, err)
	}
	var left []string
	rows, _ := pool.Query(ctx, `SELECT right(ref_id::text,1) FROM job_outbox ORDER BY 1`)
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		left = append(left, s)
	}
	rows.Close()
	if fmt.Sprint(left) != "[2 4]" {
		t.Fatalf("left %v", left)
	}
}
