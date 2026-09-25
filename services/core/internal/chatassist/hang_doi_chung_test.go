//go:build postgres || broker

package chatassist

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/adk/model"

	"mobile/services/core/internal/aiharness"
	aimetrics "mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/auth"
)

// Helpers the queue's PostgreSQL and broker tiers share (slice 10).

// dongOutbox is one row of job_outbox, as the tests read it.
type dongOutbox struct {
	queue     string
	seq       int64
	due       time.Time
	expires   *time.Time
	published bool
}

// outbox lists the job's outbox rows by enqueue.
func (f fixture) outbox(t *testing.T, id string) []dongOutbox {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT queue, enqueue_seq, available_at, expires_at, published_at IS NOT NULL FROM job_outbox WHERE ref_id=$1 ORDER BY enqueue_seq`, id)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (dongOutbox, error) {
		var d dongOutbox
		return d, r.Scan(&d.queue, &d.seq, &d.due, &d.expires, &d.published)
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// chenNep inserts n of the caller's Nếp questions straight into the table,
// exactly as nepCreate does, past the per-person rate limit the tests are not
// about. The enqueue trigger fires on these INSERTs as on the route's.
func (f fixture) chenNep(t *testing.T, n int, prompt func(i int) string) []string {
	t.Helper()
	ids := make([]string, n)
	batch := &pgx.Batch{}
	digest := auth.TokenDigest(f.token)
	for i := range ids {
		ids[i] = newID()
		batch.Queue(`INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,boi_canh,share_expires_at,status) VALUES($1,'me',NULL,$2,NULL,$3,$4,$5,'hoi',$6,NULL,clock_timestamp()+interval '15 minutes','queued')`,
			ids[i], f.person, digest, newID(), make([]byte, 32), prompt(i))
	}
	if err := f.pool.SendBatch(context.Background(), batch).Close(); err != nil {
		t.Fatal(err)
	}
	return ids
}

// nepTrenEngine runs the fixture's Nếp jobs on the Go engine over m, a model
// the test controls -- a stub, never the network -- with the metrics schema
// installed. Extra engine options come after the test's own.
func (f fixture) nepTrenEngine(t *testing.T, m model.LLM, opts ...aiharness.Option) {
	t.Helper()
	if err := aimetrics.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	base := []aiharness.Option{aiharness.WithModel(m), aiharness.WithLogger(slog.New(slog.NewJSONHandler(io.Discard, nil))),
		aiharness.WithMaKiem("hangdoi7canary"), aiharness.WithRetryWait(func(int) time.Duration { return 0 })}
	engine, err := aiharness.New(append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	f.handler.WithNepEngine(engine)
}

// trangThai reads a job's status, attempts, enqueue_seq and model_calls.
func (f fixture) trangThai(t *testing.T, id string) (status string, attempts int, seq int64, calls int) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), `SELECT status, attempts, enqueue_seq, model_calls FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &attempts, &seq, &calls); err != nil {
		t.Fatal(err)
	}
	return
}
