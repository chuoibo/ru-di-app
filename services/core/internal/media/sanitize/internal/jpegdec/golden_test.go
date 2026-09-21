package jpegdec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// padSegments returns APP15 segments adding exactly total bytes.
func padSegments(total int) [][]byte {
	var out [][]byte
	for total > 0 {
		payload := min(total-4, 60000)
		if total-(payload+4) > 0 && total-(payload+4) < 4 {
			payload -= 4
		}
		out = append(out, segment16(0xEF, make([]byte, payload)))
		total -= payload + 4
	}
	return out
}

// outcome summarises Open and Load: the format, mode, size and the first
// 16 hex characters of the pixels' sha256, or how it failed.
func outcome(data []byte) string {
	op, err := Open(data)
	if err != nil {
		if pil.IsNext(err) {
			return "next-plugin"
		}
		return "open-error"
	}
	img, err := op.Load()
	if err != nil {
		var unsupported *pil.UnsupportedError
		if errors.As(err, &unsupported) {
			return "unsupported"
		}
		return "load-error"
	}
	sum := sha256.Sum256(img.Pix)
	return fmt.Sprintf("%s %s %dx%d %s", img.Format, img.Mode, img.Width, img.Height, hex.EncodeToString(sum[:8]))
}

type goldenFile struct {
	Note  string            `json:"note"`
	Cases map[string]string `json:"cases"`
}

// TestGolden decodes the Go-built golden inputs and compares them with the
// committed outcomes, which the oracle test verifies against Pillow.
// JPEGDEC_GOLDEN_UPDATE=1 rewrites the file.
func TestGolden(t *testing.T) {
	path := filepath.Join("testdata", "golden.json")
	got := map[string]string{}
	for _, c := range goCases() {
		if c.golden {
			got[c.class+"/"+c.name] = outcome(c.build())
		}
	}
	if os.Getenv("JPEGDEC_GOLDEN_UPDATE") != "" {
		file := goldenFile{
			Note:  "Go-built JPEG inputs (seeds in corpus_test.go) -> Pillow 12.2.0 decode outcome; pixel digests are sha256 prefixes.",
			Cases: got,
		}
		raw, err := json.MarshalIndent(file, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want goldenFile
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(got))
	for name := range got {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(want.Cases) != len(got) {
		t.Errorf("golden has %d cases, corpus marks %d", len(want.Cases), len(got))
	}
	for _, name := range names {
		if want.Cases[name] != got[name] {
			t.Errorf("%s: got %q, golden %q", name, got[name], want.Cases[name])
		}
	}
}
