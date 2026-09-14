// Package config reads the environment the core binary runs with and refuses
// to start on a value it cannot honour, the same way services/api refuses at
// import time instead of failing on the first request.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Variables read by the core binary itself. Variables that services/api reads
// are added here only when a Go package starts honouring them.
const (
	EnvListen         = "MOBILE_CORE_LISTEN"
	EnvLivenessListen = "MOBILE_CORE_LIVENESS_LISTEN"
	EnvPythonUpstream = "MOBILE_PYTHON_UPSTREAM"
	EnvForcePython    = "MOBILE_FORCE_PYTHON"
)

const (
	defaultListen = "0.0.0.0:8000"
	// Liveness stays on loopback: it answers "is this process serving", which
	// is not part of the public API and must not appear next to its routes.
	defaultLivenessListen = "127.0.0.1:8001"
)

// Config is the validated startup configuration.
type Config struct {
	Listen         string
	LivenessListen string
	PythonUpstream *url.URL
	// ForcePython is the raw MOBILE_FORCE_PYTHON value. It is validated
	// against the ownership manifest, which this package does not know about.
	ForcePython string
}

// Load reads and validates every variable. getenv is os.Getenv in production.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Listen:         orDefault(getenv(EnvListen), defaultListen),
		LivenessListen: orDefault(getenv(EnvLivenessListen), defaultLivenessListen),
		ForcePython:    strings.TrimSpace(getenv(EnvForcePython)),
	}
	if err := checkHostPort(EnvListen, cfg.Listen); err != nil {
		return Config{}, err
	}
	if err := checkHostPort(EnvLivenessListen, cfg.LivenessListen); err != nil {
		return Config{}, err
	}
	if cfg.Listen == cfg.LivenessListen {
		return Config{}, fmt.Errorf("%s and %s must differ, both are %q",
			EnvListen, EnvLivenessListen, cfg.Listen)
	}
	upstream, err := parseUpstream(getenv(EnvPythonUpstream))
	if err != nil {
		return Config{}, err
	}
	cfg.PythonUpstream = upstream
	return cfg, nil
}

func orDefault(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func checkHostPort(name, value string) error {
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("%s=%q is not host:port: %w", name, value, err)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("%s=%q has an invalid port", name, value)
	}
	return nil
}

// parseUpstream accepts only a bare origin. A path, query or credentials would
// silently change every proxied request, so they are refused rather than
// joined.
func parseUpstream(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New(EnvPythonUpstream + " is required, e.g. http://api:8000")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("%s=%q is not a URL: %w", EnvPythonUpstream, value, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s=%q must use http or https", EnvPythonUpstream, value)
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%s=%q has no host", EnvPythonUpstream, value)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("%s=%q must be a bare origin (scheme://host:port)",
			EnvPythonUpstream, value)
	}
	parsed.Path = ""
	parsed.RawPath = ""
	return parsed, nil
}
