package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestWorkerDBConns(t *testing.T) {
	if n, err := workerDBConns(""); err != nil || n != 10 {
		t.Fatalf("default: %d %v", n, err)
	}
	if n, err := workerDBConns("24"); err != nil || n != 24 {
		t.Fatalf("24: %d %v", n, err)
	}
	for _, bad := range []string{"1", "51", "ten", " 4"} {
		if _, err := workerDBConns(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// A rate with nowhere to keep its count is refused, not silently dropped.
func TestModelLimiterFromEnv(t *testing.T) {
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	if l, err := modelLimiter(env(nil)); l != nil || err != nil {
		t.Fatalf("unset: %v %v", l, err)
	}
	if _, err := modelLimiter(env(map[string]string{"MOBILE_MODEL_RPM": "600"})); err == nil || !strings.Contains(err.Error(), EnvRedisURL) {
		t.Fatalf("no Redis: %v", err)
	}
	if _, err := modelLimiter(env(map[string]string{"MOBILE_MODEL_RPM": "lots", EnvRedisURL: "redis://127.0.0.1:1/0"})); err == nil {
		t.Fatal("a non-number rate accepted")
	}
	if l, err := modelLimiter(env(map[string]string{"MOBILE_MODEL_RPM": "600", EnvRedisURL: "redis://127.0.0.1:1/0"})); l == nil || err != nil {
		t.Fatalf("set: %v %v", l, err)
	}
}

// `core work` refuses a queue list or a broker URL it cannot use before it
// opens anything, and never repeats the URL (it carries a password).
func TestWorkRefusesBadQueueConfig(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{EnvWorkerQueues: "ai.nep,memory"}, EnvWorkerQueues},
		{map[string]string{EnvWorkerDBConns: "1"}, EnvWorkerDBConns},
		{map[string]string{EnvAMQPURL: "http://tier:s3cret-value@127.0.0.1:5672/"}, "MOBILE_AMQP_URL"},
	} {
		var stderr bytes.Buffer
		code := workUntil(context.Background(), func(k string) string { return c.env[k] }, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), c.want) || strings.Contains(stderr.String(), "s3cret") {
			t.Errorf("%v: exit %d: %s", c.env, code, stderr.String())
		}
	}
}
