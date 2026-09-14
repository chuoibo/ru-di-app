package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaneIsEmptyOrLimiter(t *testing.T) {
	body := func(lane string) string {
		return "id: t/lane\nroutes: ['GET /healthz']\nauth_mode: dev\n" + lane + "steps:\n  - id: one\n    as: anonymous\n    request:\n      method: GET\n      path: /healthz\n"
	}
	for name, tc := range map[string]struct {
		lane, want string
	}{
		"main":    {"", ""},
		"limiter": {"lane: limiter\n", LaneLimiter},
		"unknown": {"lane: limit\n", "error"},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "t.yaml")
		if err := os.WriteFile(path, []byte(body(tc.lane)), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadPaths(path)
		if tc.want == "error" {
			if err == nil || !strings.Contains(err.Error(), "lane") {
				t.Errorf("%s: accepted: %v", name, err)
			}
			continue
		}
		if err != nil || len(loaded) != 1 || loaded[0].Lane != tc.want {
			t.Errorf("%s: %v %v", name, loaded, err)
		}
	}
}
