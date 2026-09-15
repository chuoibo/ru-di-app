package bmpdec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	Name    string `json:"name"`
	Plugin  string `json:"plugin"`
	Result  string `json:"result"` // ok, next-plugin, open-error, load-error
	Mode    string `json:"mode,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Pixels  string `json:"pixels_sha256_prefix,omitempty"`
	Palette string `json:"palette_sha256_prefix,omitempty"`
}

type goldenInput struct {
	name string
	data []byte
}

func prefix16(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

func goldenInputs() []goldenInput {
	rng := rand.New(rand.NewSource(1609))
	var out []goldenInput
	add := func(name string, s bmpSpec) { out = append(out, goldenInput{name, s.bytes()}) }
	for _, v := range []struct {
		name       string
		headerSize int
		bits       int
		topDown    bool
		dib        bool
	}{
		{"os2-8bit", 12, 8, false, false},
		{"v3-1bit", 40, 1, false, false},
		{"v3-4bit-topdown", 40, 4, true, false},
		{"v5-16bit", 124, 16, false, false},
		{"v4-24bit", 108, 24, true, false},
		{"v3-32bit-dib", 40, 32, false, true},
	} {
		w, h := 13, 6
		bpp := 4
		if v.headerSize == 12 {
			bpp = 3
		}
		s := bmpSpec{headerSize: v.headerSize, width: int32(w), height: int32(h), bits: v.bits, dib: v.dib}
		if v.topDown {
			s.height = -s.height
		}
		max := uint64(1) << uint(v.bits)
		if v.bits <= 8 {
			s.palette = bgrxPalette(rng, 1<<uint(v.bits), bpp)
		}
		s.pixels = rawRows(randomSamples(rng, w*h, max), w, h, v.bits, v.topDown)
		add(v.name, s)
	}
	add("gray-1bit", bmpSpec{headerSize: 40, width: 9, height: 3, bits: 1,
		palette: grayBGRX(2, 4, func(i int) byte { return byte(255 * i) }),
		pixels:  rawRows(randomSamples(rng, 27, 2), 9, 3, 1, false)})
	add("gray-8bit", bmpSpec{headerSize: 40, width: 9, height: 3, bits: 8,
		palette: grayBGRX(256, 4, func(i int) byte { return byte(i) }),
		pixels:  rawRows(randomSamples(rng, 27, 256), 9, 3, 8, false)})
	add("bitfields-bgra", bmpSpec{headerSize: 56, width: 5, height: 4, bits: 32, compression: compBitfields,
		masks: [4]uint32{0xFF0000, 0xFF00, 0xFF, 0xFF000000}, pixels: rawRows(randomSamples(rng, 20, 1<<32), 5, 4, 32, false)})
	samples := make([]byte, 11*7)
	for i := range samples {
		samples[i] = byte(rng.Intn(4) * 60)
	}
	add("rle8", bmpSpec{headerSize: 40, width: 11, height: 7, bits: 8, compression: compRLE8,
		palette: bgrxPalette(rng, 256, 4), pixels: rleEncode(samples, 11, 7, false)})
	for i := range samples {
		samples[i] = byte(rng.Intn(16))
	}
	add("rle4-topdown", bmpSpec{headerSize: 40, width: 11, height: -7, bits: 4, compression: compRLE4,
		palette: bgrxPalette(rng, 16, 4), pixels: rleEncode(samples, 11, 7, true)})
	truncated := bmpSpec{headerSize: 40, width: 9, height: 9, bits: 24, pixels: rawRows(randomSamples(rng, 81, 1<<24), 9, 9, 24, false)}
	full := truncated.bytes()
	out = append(out, goldenInput{"truncated-pixels", full[:len(full)-40]})
	add("zero-height", bmpSpec{headerSize: 40, width: 4, height: 0, bits: 24})
	add("bad-bitfields", bmpSpec{headerSize: 52, width: 4, height: 4, bits: 32, compression: compBitfields,
		masks: [4]uint32{0xF, 0xF0, 0xF00, 0}})
	return out
}

func TestGolden(t *testing.T) {
	inputs := goldenInputs()
	got := make([]goldenEntry, len(inputs))
	for i, in := range inputs {
		plugin := Plugin
		if !Accept(in.data) {
			plugin = DIBPlugin
		}
		e := goldenEntry{Name: in.name, Plugin: plugin.Name}
		o, err := plugin.Open(in.data)
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
