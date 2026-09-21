//go:build oracle

package guest

// The large differential fuzz, drawn at test time by running
// scripts/render_guest_web_goldens.py --live inside the parity API image and
// replaying every case. Only the edge cases and a small sample are committed.
// Run from services/core:
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/web/guest/
//
// Environment:
//
//	W5_ORACLE_IMAGE  image (default mobile-parity-api:7bf58e3d); its /srv/app
//	                 must be this tree's services/api/app
//	W5_ORACLE_SEED   generator seed (default 1; the committed sample uses 45)
//	W5_ORACLE_COUNT  cases per module (default 6000 views, 1500 pages)

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"mobile/services/core/internal/oracletest"
)

const liveScript = "scripts/render_guest_web_goldens.py"

func liveSetting(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func liveFile(t *testing.T, module string, defaultCount int) oracletest.File {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, liveScript)); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the package", liveScript)
		}
		dir = parent
	}
	script, err := os.ReadFile(filepath.Join(dir, liveScript))
	if err != nil {
		t.Fatal(err)
	}
	image := liveSetting("W5_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	seed := liveSetting("W5_ORACLE_SEED", "1")
	count := liveSetting("W5_ORACLE_COUNT", strconv.Itoa(defaultCount))
	started := time.Now()
	cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none", "--entrypoint", "python", image,
		"-", "--live", module, seed, count)
	cmd.Stdin = bytes.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rendering %s in %s: %v\n%s", module, image, err, stderr.String())
	}
	t.Logf("python rendered %s: seed %s, count %s, %d bytes in %s", module, seed, count, len(out), time.Since(started).Round(time.Millisecond))
	file, err := oracletest.Parse(module+" live", out)
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode != module+"-fuzz-live" || file.Fuzz == nil || file.Fuzz.Total != len(file.Cases) {
		t.Fatalf("%s: live render is mode %q with %d cases", module, file.Mode, len(file.Cases))
	}
	return file
}

func TestLiveViewsMatchPython(t *testing.T) {
	agree(t, []oracletest.File{liveFile(t, "views", 6000)}, viewReplays)
}

func TestLivePagesMatchPython(t *testing.T) {
	agree(t, []oracletest.File{liveFile(t, "pages", 1500)}, pageReplays)
}
