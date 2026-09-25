package chatassist

import (
	"strings"
	"testing"
	"time"
)

func TestWorkerConfigFromEnv(t *testing.T) {
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	cfg, err := WorkerConfigFromEnv(env(nil))
	if err != nil || cfg != DefaultWorkerConfig() {
		t.Fatalf("unset: %+v %v", cfg, err)
	}
	cfg, err = WorkerConfigFromEnv(env(map[string]string{EnvWorkers: "8", EnvLeaseSeconds: "30"}))
	if err != nil || cfg.Workers != 8 || cfg.Lease != 30*time.Second {
		t.Fatalf("set: %+v %v", cfg, err)
	}
	if cfg.Heartbeat != 5*time.Second {
		t.Fatalf("heartbeat %v, want 5s under a 30s lease", cfg.Heartbeat)
	}
	cfg, _ = WorkerConfigFromEnv(env(map[string]string{EnvLeaseSeconds: "15"}))
	if cfg.Heartbeat != 5*time.Second || cfg.Heartbeat*3 > cfg.Lease {
		t.Fatalf("three beats must fit in a lease: %+v", cfg)
	}
	for _, bad := range []map[string]string{
		{EnvWorkers: "0"}, {EnvWorkers: "65"}, {EnvWorkers: "two"}, {EnvWorkers: " 2"},
		{EnvLeaseSeconds: "14"}, {EnvLeaseSeconds: "301"}, {EnvLeaseSeconds: "1m"},
	} {
		if _, err := WorkerConfigFromEnv(env(bad)); err == nil || !strings.Contains(err.Error(), "must be") {
			t.Errorf("%v: accepted (%v)", bad, err)
		}
	}
}
