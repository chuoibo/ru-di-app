//go:build postgres

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
)

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// `serve` and `work` both refuse a database below the versions this binary
// needs of either schema: the job table's (chatassist version 5) and the
// outbox's (jobs version 1). The two binaries share the database, so either
// gap would turn every question into an error behind a healthy process.
func TestServeAndWorkRefuseBelowBothSchemaVersions(t *testing.T) {
	for _, c := range []struct{ name, drop string }{
		{"chatassist version 5", `DELETE FROM chat_ai_schema_migrations WHERE version=5`},
		{"job outbox version 1", `DELETE FROM job_schema_migrations WHERE version=1`},
	} {
		t.Run(c.name, func(t *testing.T) {
			databaseURL := chatSchemaURL(t)
			migrateChatInto(t, databaseURL)
			pool, err := pgxpool.New(context.Background(), databaseURL)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(context.Background(), c.drop); err != nil {
				t.Fatal(err)
			}
			pool.Close()
			python := newPythonStub(t)
			listen, live := freeAddresses(t)
			env := map[string]string{
				"MOBILE_CORE_LISTEN":          listen,
				"MOBILE_CORE_LIVENESS_LISTEN": live,
				"MOBILE_PYTHON_UPSTREAM":      python.URL,
				"MOBILE_DATABASE_URL":         databaseURL,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			logs := &lockedBuffer{}
			if code := serveUntil(ctx, func(k string) string { return env[k] }, logs); code != 1 || !strings.Contains(logs.String(), "migrate-chat") {
				t.Fatalf("serve: exit %d; log:\n%s", code, logs.String())
			}
			t.Setenv("MOBILE_BRAIN_URL", "http://127.0.0.1:1")
			t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-internal-test-only")
			var stderr bytes.Buffer
			if code := workUntil(ctx, func(k string) string { return env[k] }, &stderr); code != 1 || !strings.Contains(stderr.String(), "migrate-chat") {
				t.Fatalf("work: exit %d: %s", code, stderr.String())
			}
		})
	}
}

// The fifteen-minute bound on shared plaintext holds with the workers scaled
// to zero (review of slice 10, finding 1): `serve` with MOBILE_INPROC_WORKER=0
// runs no job, yet a queued question whose sharing window closed loses its
// words and fails as sharing_expired within one sweep period (5 s) of the
// process starting. Mutant MA (the sweep under the inproc branch) is red here.
func TestServeSweepsWithNoWorkerUp(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var job string
	if err = pool.QueryRow(ctx, `WITH p AS (INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Synthetic caller') RETURNING id)
		INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,share_expires_at,status)
		SELECT gen_random_uuid(),'me',NULL,p.id,NULL,$1,gen_random_uuid(),$1,'hoi','synthetic question',clock_timestamp()-interval '1 second','queued' FROM p
		RETURNING id::text`, make([]byte, 32)).Scan(&job); err != nil {
		t.Fatal(err)
	}
	python := newPythonStub(t)
	began := time.Now()
	core := startCore(t, map[string]string{
		"MOBILE_PYTHON_UPSTREAM": python.URL,
		"MOBILE_DATABASE_URL":    databaseURL,
		EnvInprocWorker:          "0",
	})
	deadline := began.Add(6500 * time.Millisecond)
	for {
		var gone bool
		var status string
		var code *string
		if err = pool.QueryRow(ctx, `SELECT prompt IS NULL, status, code FROM chat_ai_invocations WHERE id=$1`, job).Scan(&gone, &status, &code); err != nil {
			t.Fatal(err)
		}
		if gone && status == "failed" && code != nil && *code == "sharing_expired" {
			t.Logf("swept %v after start, no worker up", time.Since(began).Round(time.Millisecond))
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after %v with no worker up: prompt gone=%v status=%s code=%v; log:\n%s", time.Since(began).Round(time.Millisecond), gone, status, code, core.logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if logs := core.logs.String(); !strings.Contains(logs, `"runs_here":false`) {
		t.Errorf("serve ran the jobs itself:\n%s", logs)
	}
	if code := core.stop(); code != 0 {
		t.Errorf("core stopped with %d", code)
	}
}

// `serve` running the jobs itself stops the way `core work` does: the job in
// flight is released back to the queue -- queued, the attempt given back --
// and serve returns only after, so the release never meets a closed pool
// (review of slice 10, finding 10).
func TestServeReleasesItsJobOnStop(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	entered := make(chan struct{}, 1)
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/capabilities") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"plan":{"available":true}}`)
			return
		}
		// The body read to the end: only then does the server watch the
		// connection, and see the worker hang up.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer model.Close()
	t.Setenv("MOBILE_BRAIN_URL", model.URL)
	t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-internal-test-only")
	token := "synthetic-token-release-on-stop"
	var job string
	if err = pool.QueryRow(ctx, `WITH p AS (INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Synthetic caller') RETURNING id),
		s AS (INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) SELECT gen_random_uuid(),p.id,$1,'genesis',clock_timestamp()+interval '1 hour' FROM p)
		INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,boi_canh,share_expires_at,status)
		SELECT gen_random_uuid(),'me',NULL,p.id,NULL,$1,gen_random_uuid(),$1,'hoi','synthetic question','{}',clock_timestamp()+interval '15 minutes','queued' FROM p
		RETURNING id::text`, auth.TokenDigest(token)).Scan(&job); err != nil {
		t.Fatal(err)
	}
	python := newPythonStub(t)
	core := startCore(t, map[string]string{"MOBILE_PYTHON_UPSTREAM": python.URL, "MOBILE_DATABASE_URL": databaseURL})
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		var status string
		var code *string
		_ = pool.QueryRow(ctx, `SELECT status, code FROM chat_ai_invocations WHERE id=$1`, job).Scan(&status, &code)
		t.Fatalf("the in-process worker never ran the job (status=%s code=%s); log:\n%s", status, deref(code), core.logs.String())
	}
	if code := core.stop(); code != 0 {
		t.Fatalf("core stopped with %d", code)
	}
	var status string
	var attempts int
	var leased bool
	if err = pool.QueryRow(ctx, `SELECT status, attempts, lease_id IS NOT NULL FROM chat_ai_invocations WHERE id=$1`, job).Scan(&status, &attempts, &leased); err != nil {
		t.Fatal(err)
	}
	if status != "queued" || attempts != 0 || leased {
		t.Fatalf("after serve returned: status=%s attempts=%d leased=%v, want the job released", status, attempts, leased)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// Identity beside the refusals: on a migrated database with no broker
// configured, `work` starts (poll only), answers its liveness port, runs its
// periodic tasks, and stops cleanly when told to.
func TestWorkStartsPollOnlyAndStops(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	t.Setenv("MOBILE_BRAIN_URL", "http://127.0.0.1:1")
	t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-internal-test-only")
	_, live := freeAddresses(t)
	env := map[string]string{"MOBILE_DATABASE_URL": databaseURL, "MOBILE_CORE_LIVENESS_LISTEN": live, EnvWorkerQueues: "ai.nep"}
	ctx, cancel := context.WithCancel(context.Background())
	var stderr safeBuffer
	done := make(chan int, 1)
	go func() { done <- workUntil(ctx, func(k string) string { return env[k] }, &stderr) }()
	deadline := time.Now().Add(10 * time.Second)
	for !live200(live) {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("work never answered liveness: %s", stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("work did not stop")
	}
	out := stderr.String()
	for _, want := range []string{`"msg":"AI worker started"`, `"queues":"ai.nep"`, `"broker":false`, `"msg":"AI worker stopped"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %s:\n%s", want, out)
		}
	}
}
