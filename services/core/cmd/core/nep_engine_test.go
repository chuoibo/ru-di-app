package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A worker with no usable model refuses before it opens the database: every
// job would otherwise fail. The key never reaches the log.
func TestWorkRefusesWithoutAUsableModel(t *testing.T) {
	for env, want := range map[*map[string]string]string{
		{}: "no model configured",
		// A URL without its token.
		{"AGY_PROXY_URL": "http://127.0.0.1:1"}: "AGY_PROXY_KEY",
		// A key but the real host: a test binary never builds that client.
		{"GEMINI_API_KEY": "synthetic-key"}:                                                  "loopback",
		{"GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "https://example.com"}: "loopback",
	} {
		var stderr bytes.Buffer
		code := workUntil(context.Background(), func(k string) string { return (*env)[k] }, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), want) {
			t.Fatalf("%v: exit %d: %s", *env, code, stderr.String())
		}
		if strings.Contains(stderr.String(), "synthetic-key") {
			t.Fatalf("the key reached the log: %s", stderr.String())
		}
	}
}
