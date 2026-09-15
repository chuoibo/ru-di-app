package webpdec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// goldenEntry is what Pillow 12.2.0 made of one seeded input, recorded
// after the oracle test showed this package decodes it identically.
// Regenerate with MEDIA_GOLDEN_UPDATE=1 only after that oracle run.
type goldenEntry struct {
	Name   string `json:"name"`
	Result string `json:"result"` // ok, next-plugin, open-error, load-error
	Mode   string `json:"mode,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Pixels string `json:"pixels_sha256_prefix,omitempty"`
}

func decodeEntry(name string, plugin pil.Plugin, data []byte) goldenEntry {
	e := goldenEntry{Name: name}
	o, err := plugin.Open(data)
	if err != nil {
		e.Result = "open-error"
		if pil.IsNext(err) {
			e.Result = "next-plugin"
		}
		return e
	}
	img, err := o.Load()
	if err != nil {
		e.Result = "load-error"
		return e
	}
	sum := sha256.Sum256(img.Pix)
	e.Result, e.Mode, e.Width, e.Height = "ok", img.Mode, img.Width, img.Height
	e.Pixels = hex.EncodeToString(sum[:])[:16]
	return e
}

func checkGolden(t *testing.T, got []goldenEntry) {
	t.Helper()
	path := filepath.Join("testdata", "golden.json")
	if os.Getenv("MEDIA_GOLDEN_UPDATE") != "" {
		for _, e := range got {
			if e.Pixels != "" && allDigits(e.Pixels) {
				t.Fatalf("%s: an all-digit digest prefix trips the repo guard; change the seed", e.Name)
			}
		}
		data, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want []goldenEntry
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("golden has %d entries, the generator %d", len(want), len(got))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s: got %+v, want %+v", want[i].Name, got[i], want[i])
		}
	}
}

func TestGolden(t *testing.T) {
	inputs := goldenInputs()
	got := make([]goldenEntry, len(inputs))
	for i, in := range inputs {
		got[i] = decodeEntry(in.Name, Plugin, in.bytes())
	}
	checkGolden(t, got)
}

func TestAccept(t *testing.T) {
	for _, c := range []struct {
		prefix string
		want   bool
	}{
		{"RIFF\x00\x00\x00\x00WEBPVP8 ", true},
		{"RIFF\x00\x00\x00\x00WEBPVP8L", true},
		{"RIFF\x00\x00\x00\x00WEBPVP8X", true},
		{"RIFF\x00\x00\x00\x00WEBPALPH", false},
		{"RIFF\x00\x00\x00\x00WEBPVP8", false},
		{"RIFX\x00\x00\x00\x00WEBPVP8 ", false},
	} {
		if got := Accept([]byte(c.prefix)); got != c.want {
			t.Errorf("Accept(%q) = %v", c.prefix, got)
		}
	}
}

// allDigits reports a digest prefix made only of decimal digits, which the
// repo guard would read as a long number.
func allDigits(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}
