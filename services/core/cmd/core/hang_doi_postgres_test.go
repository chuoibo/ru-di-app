//go:build postgres

package main

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
