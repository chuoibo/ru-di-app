package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mobile/services/core/internal/chatassist"
)

func TestWorkerDBConns(t *testing.T) {
	if n, err := workerDBConns("", 7); err != nil || n != 10 {
		t.Fatalf("default: %d %v", n, err)
	}
	if n, err := workerDBConns("", 21); err != nil || n != 21 {
		t.Fatalf("default above ten workers: %d %v, want the floor", n, err)
	}
	if n, err := workerDBConns("24", 7); err != nil || n != 24 {
		t.Fatalf("24: %d %v", n, err)
	}
	if n, err := workerDBConns("7", 7); err != nil || n != 7 {
		t.Fatalf("at the floor: %d %v", n, err)
	}
	for _, bad := range []string{"0", "51", "ten", " 4", "6"} {
		if _, err := workerDBConns(bad, 7); err == nil || !strings.Contains(err.Error(), EnvWorkerDBConns) {
			t.Errorf("%q accepted: %v", bad, err)
		}
	}
	if _, err := workerDBConns("", 69); err == nil || !strings.Contains(err.Error(), "MOBILE_AI_WORKERS") {
		t.Errorf("a floor past the ceiling: %v", err)
	}
}

// The floor is every slot, every periodic task and the relay's flush: the
// pool sizes the review of slice 10 found starving (two connections with a
// broker) are refused at startup, with the numbers in the message.
func TestWorkerDBFloor(t *testing.T) {
	tasks := len(workPeriodic(chatassist.New(nil, nil)))
	if tasks != 4 {
		t.Fatalf("work runs %d periodic tasks; the floor below assumes 4", tasks)
	}
	if got := workerDBFloor(2, tasks, true); got != 7 {
		t.Fatalf("defaults with a broker: %d, want 2 slots + 4 tasks + 1 relay", got)
	}
	if got := workerDBFloor(2, tasks, false); got != 6 {
		t.Fatalf("poll only: %d", got)
	}
	var stderr bytes.Buffer
	env := map[string]string{EnvWorkerDBConns: "2", EnvAMQPURL: "amqp://guest:synthetic@127.0.0.1:1/"}
	if code := workUntil(context.Background(), func(k string) string { return env[k] }, &stderr); code != 1 || !strings.Contains(stderr.String(), "at least 7") || strings.Contains(stderr.String(), "synthetic@") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}

// `serve` with MOBILE_INPROC_WORKER=0 runs no job, and still sweeps: the
// fifteen-minute bound on shared plaintext must not depend on a worker
// being up (review of slice 10, mutant MA moved the sweep under inproc).
func TestServePeriodicSweepsWithoutWorkers(t *testing.T) {
	names := func(inproc bool) string {
		var out []string
		for _, d := range servePeriodic(chatassist.New(nil, nil), inproc) {
			out = append(out, d.Ten)
		}
		return strings.Join(out, ",")
	}
	off, err := inprocWorker("0")
	if err != nil || off {
		t.Fatalf("MOBILE_INPROC_WORKER=0: %v %v", off, err)
	}
	if got := names(off); got != "chatassist.sweep,jobs.don_outbox" {
		t.Fatalf("serve without workers runs %s", got)
	}
	if got := names(true); got != "chatassist.sweep,jobs.don_outbox,aiharness.xoa_so_do,rag.xoa_nhat_ky" {
		t.Fatalf("serve with workers runs %s", got)
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
