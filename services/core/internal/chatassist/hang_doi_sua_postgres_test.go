//go:build postgres

package chatassist

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/jobs"
)

// The second round of slice 10 (review of 849664a): version 5 installs
// without rewriting the job table and without queueing behind a long
// transaction; the sweep ends a job whose last attempt's worker died; and a
// job whose terminal write failed goes back to the queue at once.

// phienBan4 is a database as the binary before slice 10 left it: the outbox
// installed (migrate-chat installs it first), the chat AI schema at version
// 4, and n finished Nếp jobs in the table.
func phienBan4(t *testing.T, n int) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool := taoSchema(t)
	if err := jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := migrateDen(ctx, pool, 4); err != nil {
		t.Fatal(err)
	}
	person := newID()
	if _, err := pool.Exec(ctx, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic caller')`, person); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,share_expires_at,status)
		SELECT gen_random_uuid(),'me',NULL,$1,NULL,$2,gen_random_uuid(),$2,'hoi',NULL,clock_timestamp()+interval '15 minutes','succeeded' FROM generate_series(1,$3)`,
		person, make([]byte, 32), n); err != nil {
		t.Fatal(err)
	}
	return pool
}

// Version 5 adds its columns without rewriting the table: the table keeps its
// file (a volatile default gave it a new one -- the rewrite ran under ACCESS
// EXCLUSIVE on the table every create, claim and heartbeat writes), and every
// row already there reads the one default stored in the catalogue.
func TestPhienBan5KhongGhiLaiBang(t *testing.T) {
	pool := phienBan4(t, 5000)
	ctx := context.Background()
	var before, after uint32
	if err := pool.QueryRow(ctx, `SELECT pg_relation_filenode('chat_ai_invocations')`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	took := time.Since(began)
	if err := pool.QueryRow(ctx, `SELECT pg_relation_filenode('chat_ai_invocations')`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("version 5 rewrote chat_ai_invocations: file %d -> %d", before, after)
	}
	var rows, dues int
	var clean bool
	if err := pool.QueryRow(ctx, `SELECT count(*), count(DISTINCT available_at), bool_and(enqueue_seq=0 AND model_calls=0 AND first_token_at IS NULL) FROM chat_ai_invocations`).Scan(&rows, &dues, &clean); err != nil {
		t.Fatal(err)
	}
	if rows != 5000 || dues != 1 || !clean {
		t.Fatalf("after version 5: %d rows, %d distinct available_at (want 1: the stored default), defaults clean=%v", rows, dues, clean)
	}
	if ok, err := SchemaCurrent(ctx, pool); err != nil || !ok {
		t.Fatalf("SchemaCurrent=%v %v", ok, err)
	}
	t.Logf("version 5 on 5000 rows: table file unchanged, %v", took.Round(time.Millisecond))
}

// Version 5 does not queue behind a long transaction: its ALTER waits five
// seconds for the lock, then fails, and the migration can be run again.
// Queued, it would have blocked every statement on the table behind it.
func TestPhienBan5KhongXepHangSauGiaoDichDai(t *testing.T) {
	pool := phienBan4(t, 1)
	ctx := context.Background()
	reader, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err = reader.QueryRow(ctx, `SELECT count(*) FROM chat_ai_invocations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	began := time.Now()
	err = Migrate(bounded, pool)
	took := time.Since(began)
	_ = reader.Rollback(ctx)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" || took > 8*time.Second {
		t.Fatalf("migration behind an open reader: %v after %v, want lock_not_available (55P03) within about 5 s", err, took.Round(time.Millisecond))
	}
	if ok, err := SchemaCurrent(ctx, pool); err != nil || ok {
		t.Fatalf("a failed version 5 left SchemaCurrent=%v %v", ok, err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatalf("run again once the reader ended: %v", err)
	}
	if ok, err := SchemaCurrent(ctx, pool); err != nil || !ok {
		t.Fatalf("SchemaCurrent=%v %v", ok, err)
	}
	t.Logf("gave up on the lock after %v", took.Round(time.Millisecond))
}

// The sweep's attempts>=3 branch (review of slice 10, mutant MD): a job whose
// third attempt's worker died has no attempt left, so no claim will ever take
// it; once its lease lapses the sweep fails it as worker_interrupted, now,
// not as sharing_expired fifteen minutes later. A job with attempts left and
// no content out is left for the next claim.
func TestSweepKetThucJobHetLuotThu(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	ids := f.chenNep(t, 3, func(i int) string { return fmt.Sprintf("lần thử cuối %d", i) })
	for _, id := range ids {
		if _, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe); !ok || err != nil {
			t.Fatalf("claim: %v %v", ok, err)
		}
	}
	hetLuot, conLuot, dangChay := ids[0], ids[1], ids[2]
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET attempts=3 WHERE id=ANY($1)`, []string{hetLuot, dangChay}); err != nil {
		t.Fatal(err)
	}
	// The third attempt of dangChay is still running under a live lease: the
	// sweep leaves it to its worker.
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=ANY($1)`, []string{hetLuot, conLuot}); err != nil {
		t.Fatal(err)
	}
	if err := f.handler.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	var code *string
	var leased bool
	if err := f.pool.QueryRow(ctx, `SELECT status, code, lease_id IS NOT NULL FROM chat_ai_invocations WHERE id=$1`, hetLuot).Scan(&status, &code, &leased); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code == nil || *code != "worker_interrupted" || leased {
		t.Fatalf("no attempt left, lease lapsed: status=%s code=%v leased=%v, want failed/worker_interrupted", status, code, leased)
	}
	if status, attempts, _, _ := f.trangThai(t, conLuot); status != "running" || attempts != 1 {
		t.Fatalf("a job with attempts left was touched by the sweep: %s attempts=%d", status, attempts)
	}
	if status, _, _, _ := f.trangThai(t, dangChay); status != "running" {
		t.Fatalf("the sweep ended a last attempt whose lease is still live: %s", status)
	}
	if j, ok, err := f.handler.claimNext(ctx, 0); !ok || err != nil || j.id != conLuot {
		t.Fatalf("the job with attempts left was not claimable again: %q %v %v", j.id, ok, err)
	}
}

// A job whose terminal write failed (the database, not the job) goes back to
// the queue at once -- queued, a new enqueue_seq and its outbox row, the
// attempt spent -- instead of waiting a whole lease with nobody renewing it
// (review of slice 10, finding 9). The table is locked while the answer is
// written, so the write times out; the lock is let go only once the worker's
// hand-back is waiting on it.
func TestLoiDBSauClaimThiTraLaiNgay(t *testing.T) {
	f := setup(t, nil)
	stub := llm.NewStub(llm.Buoc{Text: "Đi dạo hồ nhé.", Cho: 400 * time.Millisecond}, llm.Buoc{Text: "Đi dạo hồ nhé."})
	f.nepTrenEngine(t, stub)
	f.handler.WithWorker(fastWorker())
	ctx := context.Background()
	id := f.chenNep(t, 1, func(int) string { return "ghi hỏng" })[0]
	j, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
	if !ok || err != nil {
		t.Fatalf("claim: %v %v", ok, err)
	}
	done := make(chan error, 1)
	go func() { done <- f.handler.runJob(ctx, j) }()
	deadline := time.Now().Add(5 * time.Second)
	for stub.SoGoi() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the model never got the question")
		}
		time.Sleep(5 * time.Millisecond)
	}
	lock, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(ctx)
	if _, err = lock.Exec(ctx, `LOCK TABLE chat_ai_invocations IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query=$1`, traLaiSQL).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the worker never tried to hand the job back after its terminal write failed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err = lock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err == nil {
		t.Fatal("runJob reported success for a write that failed")
	}
	var leased bool
	if err = f.pool.QueryRow(ctx, `SELECT lease_id IS NOT NULL FROM chat_ai_invocations WHERE id=$1`, id).Scan(&leased); err != nil {
		t.Fatal(err)
	}
	status, attempts, seq, _ := f.trangThai(t, id)
	if status != "queued" || leased || attempts != 1 || seq != 2 || len(f.outbox(t, id)) != 2 {
		t.Fatalf("after the failed write: status=%s leased=%v attempts=%d seq=%d outbox=%d, want queued, no lease, the attempt spent, a second entry", status, leased, attempts, seq, len(f.outbox(t, id)))
	}
	// The failed attempt's message names seq 1 and claims nothing; the new
	// entry runs as attempt 2.
	if _, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe); ok || err != nil {
		t.Fatalf("the old message claimed the job: %v %v", ok, err)
	}
	if ok, err := f.handler.ClaimByID(ctx, id, 2); !ok || err != nil {
		t.Fatalf("the new entry: %v %v", ok, err)
	}
	if status, attempts, _, _ := f.trangThai(t, id); status != "succeeded" || attempts != 2 {
		t.Fatalf("second attempt: %s attempts=%d", status, attempts)
	}
}

// The two branches of the hand-back TestLoiDBSauClaimThiTraLaiNgay does not
// reach (review of slice 10 round 3, finding 3): the terminal write fails on
// the job's last attempt, or on a job whose content already went out. Such a
// job cannot run again, so the hand-back ends its lease now and the next
// sweep fails it as worker_interrupted: not queued with no attempt left,
// where no claim can take it and it ends sharing_expired fifteen minutes on
// (mutant N2), not queued for a second worker to start over after content
// went out (N1), and not left to wait out its lease (N3).
//
// The heartbeat stops before the hand-back. A renewal held up by the same
// fault as the write used to land after the lease was ended and give the job
// a whole lease back: 5 runs in 6 in the review. Here that is made certain
// for the order the fix forbids: the heartbeat's pool is one connection,
// which the test takes before it lets the lock go and gives back only once
// the hand-back committed. A heartbeat still running then renews the lease,
// and the sweep finds it live.
func TestTraLaiHetLuotHoacDaCoNoiDung(t *testing.T) {
	for _, c := range []struct{ name, set string }{
		{"lần thử cuối", "attempts=3"},
		{"đã có nội dung", "first_token_at=clock_timestamp()"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t, nil)
			stub := llm.NewStub(llm.Buoc{Text: "Đi dạo hồ nhé.", Cho: 400 * time.Millisecond})
			f.nepTrenEngine(t, stub)
			ctx := context.Background()
			cfg := fastWorker()
			cfg.Lease = 60 * time.Second
			cfg.Heartbeat = 50 * time.Millisecond
			nhipCfg := f.pool.Config().Copy()
			nhipCfg.MaxConns = 1
			nhip, err := pgxpool.NewWithConfig(ctx, nhipCfg)
			if err != nil {
				t.Fatal(err)
			}
			defer nhip.Close()
			f.handler.WithWorker(cfg).WithNhipPool(nhip)
			id := f.chenNep(t, 1, func(int) string { return "ghi hỏng" })[0]
			j, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
			if !ok || err != nil {
				t.Fatalf("claim: %v %v", ok, err)
			}
			if _, err = f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET `+c.set+` WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
			_, attemptsBefore, _, _ := f.trangThai(t, id)
			done := make(chan error, 1)
			go func() { done <- f.handler.runJob(ctx, j) }()
			deadline := time.Now().Add(5 * time.Second)
			for stub.SoGoi() == 0 {
				if time.Now().After(deadline) {
					t.Fatal("the model never got the question")
				}
				time.Sleep(5 * time.Millisecond)
			}
			lock, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(ctx)
			if _, err = lock.Exec(ctx, `LOCK TABLE chat_ai_invocations IN ACCESS EXCLUSIVE MODE`); err != nil {
				t.Fatal(err)
			}
			deadline = time.Now().Add(15 * time.Second)
			for {
				var waiting int
				if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query=$1`, traLaiSQL).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the worker never tried to hand the job back after its terminal write failed")
				}
				time.Sleep(20 * time.Millisecond)
			}
			acquire, stop := context.WithTimeout(ctx, 5*time.Second)
			beat, err := nhip.Acquire(acquire)
			stop()
			if err != nil {
				t.Fatal(err)
			}
			if err = lock.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			lapsed := func() bool {
				var ended bool
				if err := f.pool.QueryRow(ctx, `SELECT COALESCE(lease_until<=clock_timestamp(),true) FROM chat_ai_invocations WHERE id=$1`, id).Scan(&ended); err != nil {
					t.Fatal(err)
				}
				return ended
			}
			deadline = time.Now().Add(3 * time.Second)
			for !lapsed() && time.Now().Before(deadline) {
				time.Sleep(5 * time.Millisecond)
			}
			beat.Release()
			select {
			case err = <-done:
				if err == nil {
					t.Fatal("runJob reported success for a write that failed")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("runJob never returned")
			}
			if !lapsed() {
				t.Fatal("the job's lease is live after the hand-back: ended and renewed again, or never ended")
			}
			if err = f.handler.Sweep(ctx); err != nil {
				t.Fatal(err)
			}
			var status string
			var code *string
			var leased bool
			if err = f.pool.QueryRow(ctx, `SELECT status, code, lease_id IS NOT NULL FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &code, &leased); err != nil {
				t.Fatal(err)
			}
			if status != "failed" || code == nil || *code != "worker_interrupted" || leased {
				t.Fatalf("after the hand-back and one sweep: status=%s code=%v leased=%v, want failed/worker_interrupted", status, code, leased)
			}
			if _, attempts, seq, _ := f.trangThai(t, id); attempts != attemptsBefore || seq != 1 || len(f.outbox(t, id)) != 1 {
				t.Fatalf("attempts=%d (was %d) seq=%d outbox=%d: the job went back to the queue", attempts, attemptsBefore, seq, len(f.outbox(t, id)))
			}
		})
	}
}

// stop waits for at most the renewal in flight, and no renewal begins after
// it was called. Found replaying the review's PP1 probe on the first version
// of this fix: 1 run in 6 never handed the job back within 15 s. select picks
// at random between ready cases, so a stop that came while a renewal was held
// up met the tick that came meanwhile and, half the time, began one more
// renewal: a chain of them, 2 s each behind a stalled database, with the
// hand-back waiting on it. Twenty rounds of: a renewal held up in the
// heartbeat's pool of one connection, stop called, the connection let go.
// Each round the pool may hand out one connection after stop, never two
// (without the check, two in about half the rounds).
func TestHeartbeatKhongGiaHanSauKhiDung(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	cfg := fastWorker()
	cfg.Heartbeat = time.Millisecond
	nhipCfg := f.pool.Config().Copy()
	nhipCfg.MaxConns = 1
	nhip, err := pgxpool.NewWithConfig(ctx, nhipCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer nhip.Close()
	f.handler.WithWorker(cfg).WithNhipPool(nhip)
	id := f.chenNep(t, 1, func(int) string { return "nhịp" })[0]
	j, ok, err := f.handler.claimTin(ctx, id, 1, scopeMe)
	if !ok || err != nil {
		t.Fatalf("claim: %v %v", ok, err)
	}
	var after []int64
	for range 20 {
		held, err := nhip.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		stop := f.handler.heartbeat(ctx, j, func() { t.Error("heartbeat lost a lease it holds") })
		time.Sleep(20 * time.Millisecond)
		before := nhip.Stat().AcquireCount()
		stopped := make(chan struct{})
		go func() { stop(); close(stopped) }()
		time.Sleep(10 * time.Millisecond)
		held.Release()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("stop never returned")
		}
		after = append(after, nhip.Stat().AcquireCount()-before)
	}
	for _, n := range after {
		if n > 1 {
			t.Fatalf("connections handed to renewals after stop, per round: %v -- a renewal began after stop", after)
		}
	}
}
