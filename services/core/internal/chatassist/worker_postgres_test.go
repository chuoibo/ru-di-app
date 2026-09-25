//go:build postgres

package chatassist

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fastWorker renews every 100ms on the shortest lease the config allows, so a
// test sees several beats inside a second.
func fastWorker() WorkerConfig {
	cfg := DefaultWorkerConfig()
	cfg.Lease = 15 * time.Second
	cfg.Heartbeat = 100 * time.Millisecond
	return cfg
}

func leaseUntil(t *testing.T, f fixture, id string) time.Time {
	t.Helper()
	var until time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT lease_until FROM chat_ai_invocations WHERE id=$1`, id).Scan(&until); err != nil {
		t.Fatal(err)
	}
	return until
}

// A running job's lease moves forward while the model is still thinking, so a
// lease shorter than the model call no longer lets a second worker take it.
func TestHeartbeatRenewsTheLeaseWhileTheJobRuns(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	letGo := func() { once.Do(func() { close(release) }) }
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic slow answer"}})
	})
	// Registered after setup, so it runs before the fake model server closes:
	// a failed assertion must not leave the handler blocked forever.
	t.Cleanup(letGo)
	f.handler.WithWorker(fastWorker())
	job := f.create(t)
	done := make(chan error, 1)
	go func() { _, err := f.handler.ProcessOne(context.Background()); done <- err }()
	<-entered
	first := leaseUntil(t, f, job.ID)
	time.Sleep(600 * time.Millisecond)
	if later := leaseUntil(t, f, job.ID); !later.After(first) {
		t.Fatalf("lease did not move: %v then %v", first, later)
	}
	letGo()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status); err != nil || status != "succeeded" {
		t.Fatalf("status %q %v", status, err)
	}
}

// Cancelling a job stops the model call within a beat instead of letting it run
// to its sixty-second timeout, and nothing is published.
func TestCancelStopsARunningJobWithinABeat(t *testing.T) {
	entered := make(chan struct{})
	aborted := make(chan struct{})
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		// A real model service reads the request before it thinks; Go's
		// server only notices a vanished client once the body is consumed.
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		select {
		case <-r.Context().Done():
			close(aborted)
		case <-time.After(20 * time.Second):
		}
	})
	f.handler.WithWorker(fastWorker())
	job := f.create(t)
	done := make(chan error, 1)
	start := time.Now()
	go func() { _, err := f.handler.ProcessOne(context.Background()); done <- err }()
	<-entered
	requireCode(t, f.request("POST", f.route()+"/"+job.ID+"/cancel", f.token, nil), 200)
	select {
	case <-aborted:
	case <-time.After(5 * time.Second):
		t.Fatal("the model call was not cancelled")
	}
	<-done
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("cancel took %v", took)
	}
	var cards int
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&cards); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if cards != 0 || status != "cancelled" {
		t.Fatalf("cards=%d status=%s", cards, status)
	}
}

// Many consumers told to run the same job (a broker redelivers) run it once.
func TestClaimByIDRunsANamedJobExactlyOnce(t *testing.T) {
	var calls atomic.Int64
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic once"}})
	})
	older := f.create(t)
	named := f.create(t)
	var wg sync.WaitGroup
	var ran atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := f.handler.ClaimByID(context.Background(), named.ID)
			if err != nil {
				t.Error(err)
			}
			if ok {
				ran.Add(1)
			}
		}()
	}
	wg.Wait()
	if ran.Load() != 1 || calls.Load() != 1 {
		t.Fatalf("ran=%d model calls=%d, want 1 and 1", ran.Load(), calls.Load())
	}
	var olderStatus, namedStatus string
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `SELECT status FROM chat_ai_invocations WHERE id=$1`, older.ID).Scan(&olderStatus); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT status FROM chat_ai_invocations WHERE id=$1`, named.ID).Scan(&namedStatus); err != nil {
		t.Fatal(err)
	}
	if olderStatus != "queued" || namedStatus != "succeeded" {
		t.Fatalf("older=%s named=%s: ClaimByID must take only the job it names", olderStatus, namedStatus)
	}
	if ok, err := f.handler.ClaimByID(ctx, named.ID); ok || err != nil {
		t.Fatalf("a finished job was claimed again: %v %v", ok, err)
	}
}
