//go:build oracle

package oracletest

// Live runs the differential fuzz of scripts/render_domain_w4_goldens.py
// against the real Python code at test time, instead of replaying a frozen
// case list: the script draws the cases inside the parity API image and
// records what Python answered, and the calling test replays every case in Go
// with Agree. Only the edge cases and a small sample are committed; this is
// where the large runs live. Run from services/core:
//
//	go test -tags oracle -run TestLive -v -timeout 60m \
//	  ./internal/domain/allocator/ ./internal/domain/billdraft/ \
//	  ./internal/domain/budget/ ./internal/domain/expense/ \
//	  ./internal/domain/collection/ ./internal/domain/capability/ \
//	  ./internal/domain/ledger/ ./internal/domain/moneysteps/
//
// Environment:
//
//	W4_ORACLE_IMAGE  image (default mobile-parity-api:7bf58e3d); its /srv/app
//	                 must be this tree's services/api/app
//	W4_ORACLE_SEED   generator seed (default 1; the committed sample uses 44)
//	W4_ORACLE_COUNT  cases to draw for every module of the run (default: each
//	                 test's own, 21000 for the allocator)
//	W4_ORACLE_CACHE  directory caching Python's answers per script, image,
//	                 module, seed and count, so a mutated Go tree can be
//	                 re-checked against the same answers in seconds

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const liveScript = "scripts/render_domain_w4_goldens.py"

func liveEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// LiveSeed is the seed of this run.
func LiveSeed(t testing.TB) uint64 {
	t.Helper()
	seed, err := strconv.ParseUint(liveEnv("W4_ORACLE_SEED", "1"), 10, 63)
	if err != nil {
		t.Fatalf("W4_ORACLE_SEED: %v", err)
	}
	return seed
}

// LiveCount is how many cases a module draws: W4_ORACLE_COUNT, or the test's
// default.
func LiveCount(t testing.TB, defaultCount int) int {
	t.Helper()
	raw := os.Getenv("W4_ORACLE_COUNT")
	if raw == "" {
		return defaultCount
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 1 {
		t.Fatalf("W4_ORACLE_COUNT %q is not a positive int", raw)
	}
	return count
}

func findScript(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, liveScript)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the test directory", liveScript)
		}
		dir = parent
	}
}

// Live draws a module's fuzz inside the image and returns it as one File of
// mode "<module>-fuzz-live", ready for Agree.
func Live(t testing.TB, module string, defaultCount int) File {
	t.Helper()
	image := liveEnv("W4_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	seed := LiveSeed(t)
	count := LiveCount(t, defaultCount)
	script, err := os.ReadFile(findScript(t))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%s", image, module, seed, count, script)))
	cachePath := ""
	if dir := os.Getenv("W4_ORACLE_CACHE"); dir != "" {
		cachePath = filepath.Join(dir, fmt.Sprintf("w4-%s-%d-%d-%s.json", module, seed, count, hex.EncodeToString(digest[:8])))
	}

	var raw []byte
	if cachePath != "" {
		if cached, err := os.ReadFile(cachePath); err == nil {
			raw = cached
			t.Logf("python answers for %s from cache %s", module, cachePath)
		}
	}
	if raw == nil {
		started := time.Now()
		cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none", "--entrypoint", "python", image,
			"-", "--live", module, strconv.FormatUint(seed, 10), strconv.Itoa(count))
		cmd.Stdin = bytes.NewReader(script)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("rendering %s in %s: %v\n%s", module, image, err, stderr.String())
		}
		raw = out
		t.Logf("python rendered %s: seed %d, count %d, image %s, %d bytes in %s",
			module, seed, count, image, len(raw), time.Since(started).Round(time.Millisecond))
		if cachePath != "" {
			if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err == nil {
				if err := os.WriteFile(cachePath, raw, 0o600); err != nil {
					t.Logf("cache not written: %v", err)
				}
			}
		}
	}

	file, err := Parse(module+" live", raw)
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode != module+"-fuzz-live" || file.Fuzz == nil || file.Fuzz.Total != len(file.Cases) {
		t.Fatalf("%s: the live render is mode %q with %d cases, fuzz %+v", module, file.Mode, len(file.Cases), file.Fuzz)
	}
	return file
}
