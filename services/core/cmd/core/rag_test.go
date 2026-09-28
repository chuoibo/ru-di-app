package main

import (
	"bytes"
	"strings"
	"testing"
)

// `core rag` refuses a malformed call before it opens any database: exit 2
// and the usage line, with MOBILE_DATABASE_URL unset.
func TestRagArgumentsAreCheckedFirst(t *testing.T) {
	noEnv := func(string) string { return "" }
	for _, args := range [][]string{
		{"rag"}, {"rag", "burn"}, {"rag", "eval"}, {"rag", "eval", "x"}, {"rag", "promote", "0"},
		{"rag", "promote", "-3"}, {"rag", "build", "extra"}, {"rag", "tombstone", "p-1"}, {"rag", "status", "now"},
		// A build's own reasons are refused by hand: the next build would
		// lift them silently (review finding 9).
		{"rag", "tombstone", "p-1", "unsafe"}, {"rag", "tombstone", "p-1", "source_deleted"}, {"rag", "tombstone", "p-1", "vi_sao"},
		{"rag", "untombstone"}, {"rag", "untombstone", "p-1", "takedown"},
	} {
		var out, errs bytes.Buffer
		if code := run(args, noEnv, &out, &errs); code != 2 || !strings.Contains(errs.String(), "usage: core rag") {
			t.Errorf("%v: exit %d, %q", args, code, errs.String())
		}
	}
	for _, args := range [][]string{{"rag", "build"}, {"rag", "eval", "7"}, {"rag", "status"}, {"migrate-rag"}} {
		var out, errs bytes.Buffer
		// Well formed, no database configured: a configuration failure, not
		// a usage error.
		if code := run(args, noEnv, &out, &errs); code != 1 {
			t.Errorf("%v: exit %d, %q", args, code, errs.String())
		}
	}
	c, err := parseRag([]string{"tombstone", "p-quan", "takedown"})
	if err != nil || c.docID != "p-quan" || c.reason != "takedown" {
		t.Fatalf("%+v %v", c, err)
	}
	if c, err := parseRag([]string{"tombstone", "p-quan", "closed"}); err != nil || c.reason != "closed" {
		t.Fatalf("%+v %v", c, err)
	}
	if c, err := parseRag([]string{"untombstone", "p-quan"}); err != nil || c.name != "untombstone" || c.docID != "p-quan" {
		t.Fatalf("%+v %v", c, err)
	}
}
