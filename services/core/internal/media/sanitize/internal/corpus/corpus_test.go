package corpus

import (
	"bytes"
	"crypto/sha256"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"testing"
)

// TestBuildIsDeterministic rebuilds each spec twice and checks the bytes
// match, and that stdlib decoders accept the formats they know.
func TestBuildIsDeterministic(t *testing.T) {
	specs := []Spec{
		{Kind: "png", Seed: 1, W: 17, H: 11, Params: map[string]any{"color_type": 6}},
		{Kind: "png", Seed: 2, W: 9, H: 7, Params: map[string]any{"color_type": 3, "bit_depth": 4, "trns": "palette", "interlace": true}},
		{Kind: "png", Seed: 3, W: 5, H: 3, Params: map[string]any{"color_type": 0, "bit_depth": 16, "trns": "gray",
			"exif": map[string]any{"orientation": 6}, "text": []any{map[string]any{"type": "iTXt", "key": "XML:com.adobe.xmp", "value": "x"}}}},
		{Kind: "png", Seed: 4, W: 3, H: 3, Params: map[string]any{"color_type": 4, "bit_depth": 8}},
		{Kind: "jpeg", Seed: 5, W: 33, H: 17, Params: map[string]any{"pattern": "photo", "exif": map[string]any{"orientation": 8, "prefix": false}}},
		{Kind: "jpeg", Seed: 6, W: 8, H: 8, Params: map[string]any{"gray": true}},
		{Kind: "gif", Seed: 7, W: 10, H: 4, Params: map[string]any{"transparent": true}},
		{Kind: "bmp", Seed: 8, W: 7, H: 5, Params: map[string]any{"bits": 24}},
		{Kind: "bmp", Seed: 9, W: 7, H: 5, Params: map[string]any{"bits": 4, "top_down": true}},
		{Kind: "ppm", Seed: 10, W: 4, H: 2, Params: map[string]any{"magic": "P3"}},
		{Kind: "ppm", Seed: 11, W: 9, H: 2, Params: map[string]any{"magic": "P6", "maxval": 1023}},
		{Kind: "garbage", Seed: 12, Params: map[string]any{"len": 40, "magic": []any{255, 216, 255}}},
	}
	for _, spec := range specs {
		first, err := Build(spec)
		if err != nil {
			t.Fatalf("%+v: %v", spec, err)
		}
		second, _ := Build(spec)
		if sha256.Sum256(first) != sha256.Sum256(second) {
			t.Fatalf("%+v: not deterministic", spec)
		}
		switch spec.Kind {
		case "png", "jpeg", "gif":
			img, _, err := image.Decode(bytes.NewReader(first))
			if err != nil {
				t.Fatalf("%+v: stdlib decode: %v", spec, err)
			}
			if b := img.Bounds(); b.Dx() != spec.W || b.Dy() != spec.H {
				t.Fatalf("%+v: decoded %v", spec, b)
			}
		}
	}
	if _, err := Build(Spec{Kind: "pillow"}); err != ErrNeedsOracle {
		t.Fatalf("pillow spec: %v", err)
	}
}
