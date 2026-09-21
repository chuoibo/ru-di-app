package main

import (
	"bytes"
	"mobile/services/core/ownership"
	"strings"
	"testing"
)

func TestChatCandidateRequiresCompleteWriterAndRealSessions(t *testing.T) {
	all := []ownership.Route{{ID: "POST /contexts/{context_id}/messages", Group: "messages"}, {ID: "POST /votes/{vote_id}/ballots", Group: "votes"}}
	if err := validateChatCandidate("dev", all, all); err == nil {
		t.Fatal("accepted impersonation headers")
	}
	if err := validateChatCandidate("prod", all, all[:1]); err == nil {
		t.Fatal("accepted split legacy vote writer")
	}
	if err := validateChatCandidate("prod", all, all); err != nil {
		t.Fatal(err)
	}
}
func TestChatMigrationNeedsExplicitCandidateFlag(t *testing.T) {
	var out, errs bytes.Buffer
	if code := migrateChatCandidate(func(string) string { return "" }, &out, &errs); code == 0 || !strings.Contains(errs.String(), "MOBILE_CHAT_CHANGES_CANDIDATE=1") {
		t.Fatalf("code=%d error=%s", code, errs.String())
	}
}
