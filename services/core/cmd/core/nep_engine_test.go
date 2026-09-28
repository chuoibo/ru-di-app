package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNepEngineFlag(t *testing.T) {
	for raw, want := range map[string]bool{"": false, "brain": false, "go": true} {
		got, err := nepEngineGo(raw)
		if err != nil || got != want {
			t.Errorf("%q: %v %v", raw, got, err)
		}
	}
	for _, bad := range []string{"Go", "python", "1", " go"} {
		if _, err := nepEngineGo(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// A worker told to run Nếp on the Go engine with no usable model refuses
// before it opens the database: every Nếp job would otherwise fail.
func TestWorkRefusesGoEngineWithoutModel(t *testing.T) {
	t.Setenv("MOBILE_BRAIN_URL", "http://127.0.0.1:1")
	t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-internal-test-only")
	for _, env := range []map[string]string{
		{EnvAIEngineNep: "go"},
		// A key but the real host: a test binary never builds that client.
		{EnvAIEngineNep: "go", "GEMINI_API_KEY": "synthetic-key"},
		{EnvAIEngineNep: "go", "GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "https://example.com"},
		{EnvAIEngineNep: "maybe"},
	} {
		var stderr bytes.Buffer
		code := workUntil(context.Background(), func(k string) string { return env[k] }, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), EnvAIEngineNep) {
			t.Fatalf("%v: exit %d: %s", env, code, stderr.String())
		}
		if strings.Contains(stderr.String(), "synthetic-key") {
			t.Fatalf("the key reached the log: %s", stderr.String())
		}
	}
}
