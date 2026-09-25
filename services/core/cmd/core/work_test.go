package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestInprocWorkerFlag(t *testing.T) {
	for raw, want := range map[string]bool{"": true, "1": true, "0": false} {
		got, err := inprocWorker(raw)
		if err != nil || got != want {
			t.Errorf("%q: %v %v", raw, got, err)
		}
	}
	for _, bad := range []string{"true", "yes", "2", " 1"} {
		if _, err := inprocWorker(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// A worker with nothing to call must not start: it would claim every job only
// to fail it, and the failures would read as the model being down.
func TestWorkRefusesWithoutModelService(t *testing.T) {
	t.Setenv("MOBILE_BRAIN_URL", "")
	t.Setenv("MOBILE_PYTHON_UPSTREAM", "")
	t.Setenv("MOBILE_INTERNAL_TOKEN", "")
	var stderr bytes.Buffer
	code := workUntil(context.Background(), func(string) string { return "" }, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "no model service configured") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}

func TestWorkRefusesABadWorkerCount(t *testing.T) {
	var stderr bytes.Buffer
	getenv := func(k string) string {
		if k == "MOBILE_AI_WORKERS" {
			return "zero"
		}
		return ""
	}
	if code := workUntil(context.Background(), getenv, &stderr); code != 1 || !strings.Contains(stderr.String(), "MOBILE_AI_WORKERS") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}
