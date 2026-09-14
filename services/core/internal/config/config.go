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
	EnvAuthMode       = "MOBILE_AUTH_MODE"
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
	// AuthMode is MOBILE_AUTH_MODE resolved the way app/api/auth_mode.py
	// resolves it: "prod" or "dev". Go routes must authenticate in the same
	// mode as the Python process behind this front door.
	AuthMode string
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
	mode, err := resolveAuthMode(getenv(EnvAuthMode))
	if err != nil {
		return Config{}, err
	}
	cfg.AuthMode = mode
	return cfg, nil
}

// resolveAuthMode is resolve_auth_mode: absent or empty is prod; surrounding
// whitespace (str.strip's set) and case are forgiven; any other value refuses
// to start instead of guessing.
func resolveAuthMode(raw string) (string, error) {
	value := strings.ToLower(strings.TrimFunc(raw, pyIsSpace))
	switch value {
	case "":
		return "prod", nil
	case "prod", "dev":
		return value, nil
	}
	return "", fmt.Errorf("%s must be 'prod' or 'dev'; refusing to guess", EnvAuthMode)
}

// pyIsSpace is str.isspace for one code point. It differs from unicode.IsSpace
// in U+001C..U+001F, which Python counts as whitespace.
func pyIsSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r >= 0x1c && r <= 0x1f, r == ' ', r == 0x85, r == 0xa0, r == 0x1680,
		r >= 0x2000 && r <= 0x200a, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
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
