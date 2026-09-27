package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// The Go engine takes its reranker from MOBILE_RERANK_*: unset starts
// without one (and says so), a refused configuration stops the start, and
// the token never reaches the log.
func TestNepEngineReranker(t *testing.T) {
	base := map[string]string{"GEMINI_API_KEY": "synthetic-key", "MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:1"}
	chay := func(extra map[string]string) (string, error) {
		env := map[string]string{}
		for k, v := range base {
			env[k] = v
		}
		for k, v := range extra {
			env[k] = v
		}
		var log bytes.Buffer
		_, err := nepEngine(context.Background(), func(k string) string { return env[k] }, slog.New(slog.NewTextHandler(&log, nil)), nil, nepMem{})
		return log.String(), err
	}
	log, err := chay(nil)
	if err != nil || !strings.Contains(log, "reranker not configured") {
		t.Fatalf("unset: %v\n%s", err, log)
	}
	tok := strings.Repeat("t", 40)
	log, err = chay(map[string]string{"MOBILE_RERANK_URL": "https://reranker.gpu.internal:8443", "MOBILE_RERANK_TOKEN": tok})
	if err != nil || !strings.Contains(log, "model=Qwen3-Reranker-4B") || !strings.Contains(log, "timeout=3s") || strings.Contains(log, tok) {
		t.Fatalf("configured: %v\n%s", err, log)
	}
	for _, bad := range []map[string]string{
		{"MOBILE_RERANK_URL": "http://10.0.0.5:8000"},
		{"MOBILE_RERANK_URL": "http://127.0.0.1:18081", "MOBILE_RERANK_TIMEOUT": "forever"},
		{"MOBILE_RERANK_URL": "http://127.0.0.1:18081", "MOBILE_RERANK_TOKEN": "short"},
	} {
		if _, err := chay(bad); err == nil || !strings.Contains(err.Error(), EnvAIEngineNep) {
			t.Errorf("%v accepted: %v", bad, err)
		}
	}
}
