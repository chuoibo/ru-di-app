//go:build postgres

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A process that runs the AI jobs refuses to start on what would only fail
// every job later: half a model configuration, a base URL that is not
// loopback, or a database without the engine's metrics schema; a keyless one
// starts and refuses at the routes. These drive
// the real startup paths -- `serve` with its worker in the process, and
// `work` -- against the tier's database.

// dropAIMetrics removes the engine's schema from a migrated test schema and
// proves the name no longer resolves anywhere on the search path.
func dropAIMetrics(t *testing.T, databaseURL string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DROP TABLE ai_turn_metrics; DROP TABLE aiharness_schema_migrations`); err != nil {
		t.Fatal(err)
	}
	var gone bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('ai_turn_metrics') IS NULL AND to_regclass('aiharness_schema_migrations') IS NULL`).Scan(&gone); err != nil || !gone {
		t.Fatalf("the engine schema still resolves on the search path (%v): the case would test nothing", err)
	}
}

func TestServeRefusesGoEngineItCannotRun(t *testing.T) {
	for _, c := range []struct {
		name      string
		env       map[string]string
		noMetrics bool
		want      string
	}{
		{name: "agy without its key", env: map[string]string{"AGY_PROXY_URL": "http://127.0.0.1:9"}, want: "AGY_PROXY_KEY"},
		{name: "base URL not loopback", env: map[string]string{"GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "https://example.com"}, want: "loopback"},
		{name: "no metrics schema", env: map[string]string{"GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9"}, noMetrics: true, want: "needs its schema"},
	} {
		t.Run(c.name, func(t *testing.T) {
			databaseURL := chatSchemaURL(t)
			migrateChatInto(t, databaseURL)
			if c.noMetrics {
				dropAIMetrics(t, databaseURL)
			}
			python := newPythonStub(t)
			listen, live := freeAddresses(t)
			env := map[string]string{
				"MOBILE_CORE_LISTEN":          listen,
				"MOBILE_CORE_LIVENESS_LISTEN": live,
				"MOBILE_PYTHON_UPSTREAM":      python.URL,
				"MOBILE_DATABASE_URL":         databaseURL,
			}
			for k, v := range c.env {
				env[k] = v
			}
			logs := &lockedBuffer{}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if code := serveUntil(ctx, func(k string) string { return env[k] }, logs); code != 1 {
				t.Fatalf("exit %d; log:\n%s", code, logs.String())
			}
			out := logs.String()
			if !strings.Contains(out, "refusing to start") || !strings.Contains(out, c.want) {
				t.Errorf("refusal does not name %q:\n%s", c.want, out)
			}
			if strings.Contains(out, "synthetic-key") {
				t.Errorf("the key reached the log:\n%s", out)
			}
		})
	}
}

// Identity beside the refusals: with a key, a loopback base URL and the
// schema, `serve` runs the AI worker on the Go engine and says so. No
// connection to the model is opened at startup.
func TestServeRunsGoEngineWhenReady(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	python := newPythonStub(t)
	core := startCore(t, map[string]string{
		"MOBILE_PYTHON_UPSTREAM": python.URL,
		"MOBILE_DATABASE_URL":    databaseURL,
		"GEMINI_API_KEY":         "synthetic-key",
		"MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9",
	})
	logs := core.logs.String()
	if !strings.Contains(logs, "AI engine runs in this process") || strings.Contains(logs, "synthetic-key") {
		t.Errorf("startup log:\n%s", logs)
	}
	if code := core.stop(); code != 0 {
		t.Errorf("core stopped with %d", code)
	}
}

// `core work` refuses the same database without the metrics schema, after
// the chat schema check passed and the engine was built.
func TestWorkRefusesGoEngineWithoutMetricsSchema(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	dropAIMetrics(t, databaseURL)
	env := map[string]string{
		"MOBILE_DATABASE_URL":    databaseURL,
		"GEMINI_API_KEY":         "synthetic-key",
		"MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9",
	}
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code := workUntil(ctx, func(k string) string { return env[k] }, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "needs its schema") || strings.Contains(stderr.String(), "synthetic-key") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}

// A keyless stack still serves: `serve` starts, says it has no model, and
// runs no engine (its AI routes refuse as without a key, ADR-0052).
func TestServeStartsKeyless(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	python := newPythonStub(t)
	core := startCore(t, map[string]string{
		"MOBILE_PYTHON_UPSTREAM": python.URL,
		"MOBILE_DATABASE_URL":    databaseURL,
	})
	logs := core.logs.String()
	if !strings.Contains(logs, "no model configured") || strings.Contains(logs, "AI engine runs in this process") {
		t.Errorf("startup log:\n%s", logs)
	}
	if code := core.stop(); code != 0 {
		t.Errorf("core stopped with %d", code)
	}
}
