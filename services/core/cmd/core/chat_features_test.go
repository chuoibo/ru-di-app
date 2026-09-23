package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mobile/services/core/ownership"
)

// The flag rule as a table. On is the default in prod; "0" is the only way
// out; dev stays off unless asked, and asking is refused by validation.
func TestChatFeaturesAreOnUnlessTurnedOff(t *testing.T) {
	for _, c := range []struct {
		raw, mode string
		on        bool
		fails     bool
		reason    string
	}{
		{raw: "", mode: "prod", on: true},
		{raw: "1", mode: "prod", on: true},
		{raw: "0", mode: "prod", reason: "MOBILE_CHAT_CHANGES_CANDIDATE=0"},
		{raw: "0", mode: "dev", reason: "MOBILE_CHAT_CHANGES_CANDIDATE=0"},
		{raw: "", mode: "dev", reason: "MOBILE_AUTH_MODE=dev"},
		// "1" in dev resolves on so that validation, not silence, answers it.
		{raw: "1", mode: "dev", on: true},
		{raw: "true", mode: "prod", fails: true},
		{raw: " 1", mode: "prod", fails: true},
		{raw: "yes", mode: "dev", fails: true},
	} {
		got, err := resolveChatFeatures(c.raw, c.mode)
		if c.fails {
			if err == nil || !strings.Contains(err.Error(), "must be 0 or 1") {
				t.Errorf("%q/%s: err=%v, want a refusal", c.raw, c.mode, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q/%s: %v", c.raw, c.mode, err)
			continue
		}
		if got.on != c.on {
			t.Errorf("%q/%s: on=%v, want %v", c.raw, c.mode, got.on, c.on)
		}
		if got.on && got.off != "" {
			t.Errorf("%q/%s: on but carries an off reason %q", c.raw, c.mode, got.off)
		}
		if !got.on && !strings.Contains(got.off, c.reason) {
			t.Errorf("%q/%s: off reason %q does not name %q", c.raw, c.mode, got.off, c.reason)
		}
	}
}

func TestChatFeaturesRequireCompleteWriterAndRealSessions(t *testing.T) {
	all := []ownership.Route{{ID: "POST /contexts/{context_id}/messages", Group: "messages"}, {ID: "POST /votes/{vote_id}/ballots", Group: "votes"}}
	if err := validateChatFeatures("dev", all, all); err == nil {
		t.Fatal("accepted impersonation headers")
	}
	if err := validateChatFeatures("prod", all, all[:1]); err == nil {
		t.Fatal("accepted split legacy vote writer")
	}
	if err := validateChatFeatures("prod", all, all); err != nil {
		t.Fatal(err)
	}
}

// On by default means a host that forces a chat route back to Python must not
// come up with the AI quietly gone. It refuses, and says how to opt out. The
// refusal comes before any database is opened, so none is configured here.
func TestDefaultOnRefusesPythonChatWriterAndNamesTheWayOut(t *testing.T) {
	env := map[string]string{
		"MOBILE_PYTHON_UPSTREAM": "http://127.0.0.1:9",
		"MOBILE_FORCE_PYTHON":    "messages",
	}
	var logs bytes.Buffer
	code := serveUntil(context.Background(), func(k string) string { return env[k] }, &logs)
	if code != 1 {
		t.Fatalf("exit %d, want a refusal; log: %s", code, logs.String())
	}
	for _, want := range []string{"refusing to start", "messages", "MOBILE_CHAT_CHANGES_CANDIDATE=0"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("refusal does not mention %q: %s", want, logs.String())
		}
	}
}

// The migration no longer asks for the flag: with none set it goes straight
// to the database settings, and fails there only because there are none.
func TestChatMigrationNeedsNoFlag(t *testing.T) {
	for _, command := range []string{"migrate-chat", "migrate-chat-candidate"} {
		var out, errs bytes.Buffer
		code := run([]string{command}, func(string) string { return "" }, &out, &errs)
		if code == 0 {
			t.Fatalf("%s: migrated with no database", command)
		}
		if strings.Contains(errs.String(), "MOBILE_CHAT_CHANGES_CANDIDATE") {
			t.Fatalf("%s still asks for the flag: %s", command, errs.String())
		}
		if !strings.Contains(errs.String(), "invalid database configuration") {
			t.Fatalf("%s: did not reach the database settings: %s", command, errs.String())
		}
	}
}

// Asking for the features in dev is refused out loud, before any database is
// opened; leaving the flag unset in dev is not (see the postgres cases).
func TestExplicitOnInDevRefusesToStart(t *testing.T) {
	env := map[string]string{
		"MOBILE_PYTHON_UPSTREAM":        "http://127.0.0.1:9",
		"MOBILE_AUTH_MODE":              "dev",
		"MOBILE_CHAT_CHANGES_CANDIDATE": "1",
	}
	var logs bytes.Buffer
	if code := serveUntil(context.Background(), func(k string) string { return env[k] }, &logs); code != 1 {
		t.Fatalf("exit %d, want a refusal; log: %s", code, logs.String())
	}
	if !strings.Contains(logs.String(), "MOBILE_AUTH_MODE=prod") {
		t.Fatalf("refusal does not say why: %s", logs.String())
	}
}
