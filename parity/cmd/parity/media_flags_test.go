package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaFlagsGoInPairsAndNameTwoDirectories(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	file := filepath.Join(a, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(b, "link")
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string][2]string{
		"reference only":      {a, ""},
		"candidate only":      {"", b},
		"one directory":       {a, a},
		"one through a link":  {a, link},
		"missing directory":   {a, filepath.Join(b, "missing")},
		"a file, not a store": {a, file},
	} {
		if err := checkMediaPair("run", "--reference-media", pair[0], "--candidate-media", pair[1]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for _, pair := range [][2]string{{a, b}, {"", ""}} {
		if err := checkMediaPair("run", "--reference-media", pair[0], "--candidate-media", pair[1]); err != nil {
			t.Errorf("%v: %v", pair, err)
		}
	}

	// Both commands refuse before they reach a stack or read a scenario.
	for _, args := range [][]string{
		{"run", "--reference", "http://127.0.0.1:9", "--candidate", "http://127.0.0.1:9", "--reference-media", a, "no-such-path"},
		{"canary", "--reference", "http://127.0.0.1:9", "--target", "http://127.0.0.1:9", "--target-media", b, "no-such-path"},
	} {
		var stderr bytes.Buffer
		if code := run(args, io.Discard, &stderr); code != 2 || !strings.Contains(stderr.String(), "go together") {
			t.Errorf("%s: exit %d, stderr %q", args[0], code, stderr.String())
		}
	}
}
