package config

import (
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{EnvPythonUpstream: "http://api:8000"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "0.0.0.0:8000" || cfg.LivenessListen != "127.0.0.1:8001" {
		t.Fatalf("defaults = %q %q", cfg.Listen, cfg.LivenessListen)
	}
	if got := cfg.PythonUpstream.String(); got != "http://api:8000" {
		t.Fatalf("upstream = %q", got)
	}
}

func TestLoadTrailingSlashUpstreamIsNormalised(t *testing.T) {
	cfg, err := Load(env(map[string]string{EnvPythonUpstream: " http://127.0.0.1:9000/ "}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.PythonUpstream.String(); got != "http://127.0.0.1:9000" {
		t.Fatalf("upstream = %q", got)
	}
}

func TestLoadRefusals(t *testing.T) {
	cases := map[string]struct {
		values map[string]string
		want   string
	}{
		"missing upstream": {map[string]string{}, "is required"},
		"blank upstream":   {map[string]string{EnvPythonUpstream: "   "}, "is required"},
		"ftp scheme":       {map[string]string{EnvPythonUpstream: "ftp://api:8000"}, "http or https"},
		"no host":          {map[string]string{EnvPythonUpstream: "http://"}, "no host"},
		"path":             {map[string]string{EnvPythonUpstream: "http://api:8000/v1"}, "bare origin"},
		"query":            {map[string]string{EnvPythonUpstream: "http://api:8000?x=1"}, "bare origin"},
		"credentials":      {map[string]string{EnvPythonUpstream: "http://u:p@api:8000"}, "bare origin"},
		"bad listen": {map[string]string{
			EnvPythonUpstream: "http://api:8000", EnvListen: "8000",
		}, "not host:port"},
		"bad port": {map[string]string{
			EnvPythonUpstream: "http://api:8000", EnvListen: "0.0.0.0:99999",
		}, "invalid port"},
		"same listeners": {map[string]string{
			EnvPythonUpstream: "http://api:8000", EnvListen: "127.0.0.1:8001",
		}, "must differ"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(env(tc.values))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}
