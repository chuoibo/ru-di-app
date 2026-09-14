package config

import (
	"strings"
	"testing"
)

// The cases were checked against resolve_auth_mode in the pinned API image.
var authModeCases = map[string]string{
	"": "prod", "prod": "prod", "dev": "dev", "Dev ": "dev", " PROD\n": "prod",
	"\x1cdev\x1f": "dev", "\u00a0dev\u3000": "dev", "\u2028prod\u0085": "prod",
	"prd": "", "dev prod": "", "de v": "", "\u200bdev": "",
}

func TestAuthModeResolvesLikePython(t *testing.T) {
	for raw, want := range authModeCases {
		got, err := resolveAuthMode(raw)
		if want == "" {
			if err == nil || !strings.Contains(err.Error(), "refusing to guess") {
				t.Fatalf("%q: got %q %v, want a refusal", raw, got, err)
			}
			continue
		}
		if err != nil || got != want {
			t.Fatalf("%q: got %q %v, want %q", raw, got, err, want)
		}
	}
}

func TestLoadRefusesToStartOnAnUnknownAuthMode(t *testing.T) {
	env := map[string]string{EnvPythonUpstream: "http://api:8000"}
	getenv := func(name string) string { return env[name] }
	cfg, err := Load(getenv)
	if err != nil || cfg.AuthMode != "prod" {
		t.Fatalf("default: %+v %v", cfg, err)
	}
	env[EnvAuthMode] = "prd"
	if _, err := Load(getenv); err == nil {
		t.Fatal("an unknown auth mode started")
	}
}
