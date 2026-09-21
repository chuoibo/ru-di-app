package brain

import (
	"testing"
)

func TestConfiguredNilWithoutEnv(t *testing.T) {
	t.Setenv(envBrainURL, "")
	t.Setenv(envPythonUpstream, "")
	t.Setenv(envToken, "")
	if Configured() != nil {
		t.Fatal("expected nil without URL and token")
	}
}

func TestConfiguredNilWithoutToken(t *testing.T) {
	t.Setenv(envBrainURL, "http://127.0.0.1:8000")
	t.Setenv(envToken, "")
	if Configured() != nil {
		t.Fatal("expected nil without token")
	}
}

func TestConfiguredNilOnBadURL(t *testing.T) {
	t.Setenv(envBrainURL, "not a url")
	t.Setenv(envToken, "test-brain-token")
	if Configured() != nil {
		t.Fatal("expected nil on a URL Python would refuse")
	}
	t.Setenv(envBrainURL, "http://user:pass@127.0.0.1:8000")
	if Configured() != nil {
		t.Fatal("expected nil with userinfo")
	}
}

func TestConfiguredReadsBrainURLThenUpstream(t *testing.T) {
	t.Setenv(envBrainURL, "http://brain.example:8000")
	t.Setenv(envPythonUpstream, "http://api.example:8000")
	t.Setenv(envToken, "test-brain-token")
	client := Configured()
	if client == nil || client.baseURL != "http://brain.example:8000" {
		t.Fatalf("brain URL: %+v", client)
	}
	t.Setenv(envBrainURL, "")
	client = Configured()
	if client == nil || client.baseURL != "http://api.example:8000" {
		t.Fatalf("upstream fallback: %+v", client)
	}
}
