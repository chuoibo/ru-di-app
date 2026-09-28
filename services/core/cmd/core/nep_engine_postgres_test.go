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

// A process that runs Nếp's jobs on the Go engine refuses to start on what
// would only fail every Nếp job later: no key, a base URL that is not
// loopback, or a database without the engine's metrics schema. These drive
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
		{name: "no key", env: map[string]string{}, want: "GEMINI_API_KEY"},
		{name: "base URL not loopback", env: map[string]string{"GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "https://example.com"}, want: "loopback"},
		{name: "no metrics schema", env: map[string]string{"GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9"}, noMetrics: true, want: "needs the AI engine schema"},
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
				EnvAIEngineNep:                "go",
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
			if !strings.Contains(out, "refusing to start") || !strings.Contains(out, EnvAIEngineNep) || !strings.Contains(out, c.want) {
				t.Errorf("refusal does not name %s and %q:\n%s", EnvAIEngineNep, c.want, out)
			}
			if strings.Contains(out, "synthetic-key") {
				t.Errorf("the key reached the log:\n%s", out)
			}
		})
	}
}

// Identity beside the refusals: with a key, a loopback base URL and the
// schema, `serve` runs Nep's worker on the Go engine and says so. No
// connection to the model is opened at startup.
func TestServeRunsGoEngineWhenReady(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	python := newPythonStub(t)
	core := startCore(t, map[string]string{
		"MOBILE_PYTHON_UPSTREAM": python.URL,
		"MOBILE_DATABASE_URL":    databaseURL,
		EnvAIEngineNep:           "go",
		"GEMINI_API_KEY":         "synthetic-key",
		"MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9",
	})
	logs := core.logs.String()
	if !strings.Contains(logs, `"engine":"go"`) || !strings.Contains(logs, `"runs_here":true`) || strings.Contains(logs, "synthetic-key") {
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
	// The brain client reads the process environment, as in work_test.go.
	t.Setenv("MOBILE_BRAIN_URL", "http://127.0.0.1:1")
	t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-internal-test-only")
	env := map[string]string{
		"MOBILE_DATABASE_URL":    databaseURL,
		EnvAIEngineNep:           "go",
		"GEMINI_API_KEY":         "synthetic-key",
		"MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9",
	}
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code := workUntil(ctx, func(k string) string { return env[k] }, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "needs the AI engine schema") || strings.Contains(stderr.String(), "synthetic-key") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}
