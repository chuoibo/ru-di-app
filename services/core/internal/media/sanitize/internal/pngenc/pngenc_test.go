package pngenc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// goldenCase is a corpus case with the first 16 hex digits of the sha256
// and the size of what Pillow 12.2.0 wrote for it (recorded by the oracle
// test with PNGENC_WRITE_GOLDEN=1).
type goldenCase struct {
	corpusCase
	SHA256Prefix string `json:"sha256_prefix"`
	Size         int    `json:"size"`
}

func TestGoldenPillowPNG(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden []goldenCase
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden) == 0 {
		t.Fatal("empty golden")
	}
	for _, g := range golden {
		out := EncodeRGBA(g.W, g.H, g.pixels())
		sum := sha256.Sum256(out)
		if got := hex.EncodeToString(sum[:])[:16]; got != g.SHA256Prefix || len(out) != g.Size {
			t.Errorf("%+v: got %s (%d bytes), Pillow wrote %s (%d bytes)", g.corpusCase, got, len(out), g.SHA256Prefix, g.Size)
		}
	}
}

// TestDecodesBack checks the stream is a valid PNG carrying the pixels, a
// property independent of Pillow.
func TestDecodesBack(t *testing.T) {
	for _, c := range []corpusCase{
		{Content: "gradient", W: 1, H: 1, Seed: 1},
		{Content: "noise", W: 300, H: 200, Seed: 2},
		{Content: "text", W: 17000, H: 3, Seed: 3},
		{Content: "photo", W: 700, H: 500, Seed: 4},
	} {
		pix := c.pixels()
		img, err := png.Decode(bytes.NewReader(EncodeRGBA(c.W, c.H, pix)))
		if err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		nrgba, ok := img.(*image.NRGBA)
		if !ok {
			t.Fatalf("%+v: decoded as %T", c, img)
		}
		if !bytes.Equal(nrgba.Pix, pix) {
			t.Fatalf("%+v: pixels differ after decoding", c)
		}
	}
}
