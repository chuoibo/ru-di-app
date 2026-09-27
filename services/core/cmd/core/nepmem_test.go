package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aictx"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

var khoa32 = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

// Half a configuration is refused before anything opens; nothing set is
// memory off, and the engine keeps its in-process defaults.
func TestNepMemCauHinh(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	for name, c := range map[string]struct {
		env  map[string]string
		want string
	}{
		"url without key":    {map[string]string{EnvAIInferURL: "http://127.0.0.1:8090"}, EnvNepMemoryKey},
		"bad key":            {map[string]string{EnvNepMemoryKey: "short"}, "base64 of 32 bytes"},
		"short token":        {map[string]string{EnvNepMemoryKey: khoa32, EnvAIInferURL: "http://127.0.0.1:8090", EnvAIInferToken: "x"}, "token"},
		"bad threshold":      {map[string]string{EnvNepMemoryKey: khoa32, EnvAIInferURL: "http://127.0.0.1:8090", EnvAIInferToken: strings.Repeat("t", 32), EnvNepMemoryThreshold: "cao"}, EnvNepMemoryThreshold},
		"aictx key alone":    {map[string]string{aictx.EnvKhoa: khoa32}, aictx.EnvRedisURL},
		"aictx url, bad key": {map[string]string{aictx.EnvRedisURL: "redis://127.0.0.1:1/0", aictx.EnvKhoa: "x"}, aictx.EnvKhoa},
		"aictx url, no key":  {map[string]string{aictx.EnvRedisURL: "redis://127.0.0.1:1/0"}, aictx.EnvKhoa},
	} {
		if _, err := openNepMem(ctx, envOf(c.env), logger, nil); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error naming %s", name, err, c.want)
		}
	}
	m, err := openNepMem(ctx, envOf(nil), logger, nil)
	if err != nil || m.kho != nil || m.ngan != nil || len(m.engineOptions()) != 0 || len(m.periodic()) != 0 {
		t.Fatalf("nothing set: %+v %v", m, err)
	}
}

// The redis-ai instance is checked when the store is built: one that
// cannot be read (here: nothing listening) refuses in prod, warns in dev
// and is then used, independently of long-term memory being off.
func TestNepMemRedisAiKhoiDong(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	env := map[string]string{aictx.EnvRedisURL: "redis://127.0.0.1:1/0", aictx.EnvKhoa: khoa32}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	if _, err := openNepMem(ctx, envOf(env), logger, pool); err == nil || !strings.Contains(err.Error(), "redis-ai") {
		t.Fatalf("prod with an unreadable redis-ai: %v", err)
	}
	env["MOBILE_AUTH_MODE"] = "dev"
	m, err := openNepMem(ctx, envOf(env), logger, pool)
	if err != nil || m.ngan == nil || m.kho != nil || len(m.engineOptions()) != 1 {
		t.Fatalf("dev: %+v %v", m, err)
	}
	m.close()
	if !strings.Contains(logs.String(), "accepted in dev only") {
		t.Fatalf("no warning in dev: %s", logs.String())
	}
}
