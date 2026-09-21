package gifdec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
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
	Name         string `json:"name"`
	Result       string `json:"result"` // ok, next-plugin, open-error, load-error
	Mode         string `json:"mode,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Pixels       string `json:"pixels_sha256_prefix,omitempty"`
	Palette      string `json:"palette_sha256_prefix,omitempty"`
	Transparency int    `json:"transparency,omitempty"`
}

type goldenInput struct {
	name string
	data []byte
}

func prefix16(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

// goldenInputs builds the seeded inputs; compress/lzw of the pinned Go
// toolchain writes the LZW streams.
func goldenInputs() []goldenInput {
	rng := rand.New(rand.NewSource(2026))
	var out []goldenInput
	add := func(name string, data []byte) { out = append(out, goldenInput{name, data}) }
	for i, v := range []struct {
		w, h, bits int
		interlace  bool
	}{{1, 1, 1, false}, {7, 5, 2, true}, {17, 11, 4, false}, {64, 33, 8, true}} {
		g := &gifFile{version: "GIF89a", width: v.w, height: v.h, global: palette(rng, 1<<uint(v.bits)), globalBits: v.bits}
		if i%2 == 1 {
			g.gce(0x01, 4, 1)
		}
		idx := indices(rng, v.w*v.h, 1<<uint(v.bits), "noise")
		g.image(0, 0, v.w, v.h, v.interlace, nil, 0, encodeLZW(idx, v.bits), byte(max(v.bits, 2)))
		add(fmt.Sprintf("palette-%dx%d-b%d", v.w, v.h, v.bits), g.bytes())
	}
	{
		g := &gifFile{version: "GIF89a", width: 12, height: 9, global: grayPalette(256), globalBits: 8}
		g.image(0, 0, 12, 9, false, nil, 0, encodeLZW(indices(rng, 108, 256, "ramp"), 8), 8)
		add("gray-becomes-L", g.bytes())
	}
	{
		g := &gifFile{version: "GIF89a", width: 10, height: 8, global: palette(rng, 4), globalBits: 2}
		g.gce(0x09, 2, 3)
		g.image(2, 1, 11, 9, true, palette(rng, 8), 3, encodeLZW(indices(rng, 99, 8, "noise"), 3), 3)
		add("local-palette-expands-screen", g.bytes())
	}
	{
		g := &gifFile{version: "GIF87a", width: 20, height: 20, global: palette(rng, 16), globalBits: 4}
		g.image(0, 0, 20, 20, false, nil, 0, encodeLZW(indices(rng, 400, 16, "noise"), 4), 4)
		full := g.bytes()
		add("truncated-data", full[:len(full)/2])
		add("header-only", full[:13])
	}
	{
		g := &gifFile{version: "GIF89a", width: 3, height: 3, global: palette(rng, 4), globalBits: 2}
		g.image(0, 0, 3, 3, false, nil, 0, rawCodes([]int{4, 1, 2, 5}, 3), 2)
		add("lzw-early-end", g.bytes())
		z := &gifFile{version: "GIF89a", width: 0, height: 4, global: palette(rng, 4), globalBits: 2}
		z.image(0, 0, 1, 4, false, nil, 0, encodeLZW(indices(rng, 4, 4, "noise"), 2), 2)
		add("zero-width-screen-expanded", z.bytes())
		b := &gifFile{version: "GIF89a", width: 4, height: 4, global: palette(rng, 4), globalBits: 2}
		b.image(0, 0, 4, 4, false, nil, 0, encodeLZW(indices(rng, 16, 4, "noise"), 2), 13)
		add("lzw-bits-13", b.bytes())
	}
	return out
}

func TestGolden(t *testing.T) {
	inputs := goldenInputs()
	got := make([]goldenEntry, len(inputs))
	for i, in := range inputs {
		e := goldenEntry{Name: in.name}
		o, err := Open(in.data)
		switch {
		case err != nil && pil.IsNext(err):
			e.Result = "next-plugin"
		case err != nil:
			e.Result = "open-error"
		default:
			img, err := o.Load()
			if err != nil {
				e.Result = "load-error"
				break
			}
			e.Result, e.Mode, e.Width, e.Height = "ok", img.Mode, img.Width, img.Height
			e.Pixels = prefix16(img.Pix)
			if img.Mode == "P" {
				e.Palette = prefix16(img.Palette)
			}
			if img.Info.Transparency != nil {
				e.Transparency = img.Info.Transparency.Int + 1
			}
		}
		got[i] = e
	}
	path := filepath.Join("testdata", "golden.json")
	if os.Getenv("MEDIA_GOLDEN_UPDATE") != "" {
		for _, e := range got {
			for _, v := range []string{e.Pixels, e.Palette} {
				if v != "" && allDigits(v) {
					t.Fatalf("%s: an all-digit digest prefix trips the repo guard; change the seed", e.Name)
				}
			}
		}
		data, _ := json.MarshalIndent(got, "", "  ")
		os.MkdirAll("testdata", 0o755)
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

// allDigits reports a digest prefix made only of decimal digits, which the
// repo guard would read as a long number.
func allDigits(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}
