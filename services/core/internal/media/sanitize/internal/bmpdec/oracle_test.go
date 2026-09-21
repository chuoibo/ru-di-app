//go:build oracle

package bmpdec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type bmpCase struct {
	name  string
	class string
	input []byte
}

func goCases() []bmpCase {
	var cases []bmpCase
	add := func(class, name string, data []byte) {
		cases = append(cases, bmpCase{class + "-" + name, class, data})
	}
	rng := rand.New(rand.NewSource(5))
	sizes := [][2]int{{1, 1}, {3, 2}, {7, 5}, {16, 9}, {33, 1}}
	for _, hs := range []int{12, 40, 52, 56, 64, 108, 124} {
		for _, bits := range []int{1, 4, 8, 16, 24, 32} {
			for si, sz := range sizes {
				w, h := sz[0], sz[1]
				for _, topDown := range []bool{false, true} {
					if topDown && hs == 12 {
						continue
					}
					bpp := 4
					if hs == 12 {
						bpp = 3
					}
					spec := bmpSpec{headerSize: hs, width: int32(w), height: int32(h), bits: bits, dib: si%3 == 2}
					if topDown {
						spec.height = -spec.height
					}
					var samples []uint32
					if bits <= 8 {
						colors := 1 << uint(bits)
						samples = randomSamples(rng, w*h, uint64(colors))
						spec.palette = bgrxPalette(rng, colors, bpp)
					} else {
						samples = randomSamples(rng, w*h, 1<<uint(bits))
					}
					spec.pixels = rawRows(samples, w, h, bits, topDown)
					add("raw", fmt.Sprintf("h%d-b%d-%dx%d-td%v-dib%v", hs, bits, w, h, topDown, spec.dib), spec.bytes())
				}
			}
		}
	}
	// Palettes: grayscale detection, colour counts.
	for k, pal := range []struct {
		bits, entries int
		colors        uint32
		gray          func(int) byte
	}{
		{1, 2, 0, func(i int) byte { return byte(255 * i) }},
		{1, 2, 2, func(i int) byte { return byte(i) }},
		{8, 256, 0, func(i int) byte { return byte(i) }},
		{8, 100, 100, func(i int) byte { return byte(i) }},
		{4, 16, 16, func(i int) byte { return byte(i) }},
		{4, 16, 16, func(i int) byte { return byte(17 * i) }},
		{8, 300, 300, func(i int) byte { return byte(i) }},
		{1, 16, 16, func(i int) byte { return byte(i) }},
	} {
		w, h := 9, 4
		samples := randomSamples(rng, w*h, uint64(min(pal.entries, 1<<uint(pal.bits))))
		spec := bmpSpec{headerSize: 40, width: int32(w), height: int32(h), bits: pal.bits, colors: pal.colors,
			palette: grayBGRX(pal.entries, 4, pal.gray), pixels: rawRows(samples, w, h, pal.bits, false)}
		add("palette", fmt.Sprint(k), spec.bytes())
	}
	{
		spec := bmpSpec{headerSize: 40, width: 4, height: 4, bits: 8, colors: 70000, palette: bgrxPalette(rng, 8, 4)}
		add("palette", "too-many-colors", spec.bytes())
		spec = bmpSpec{headerSize: 40, width: 4, height: 4, bits: 8, colors: 5, palette: bgrxPalette(rng, 5, 4),
			pixels: rawRows(randomSamples(rng, 16, 9), 4, 4, 8, false)}
		add("palette", "index-past-palette", spec.bytes())
	}
	// Bitfields.
	masks32 := [][4]uint32{
		{0xFF0000, 0xFF00, 0xFF, 0}, {0xFF000000, 0xFF0000, 0xFF00, 0}, {0xFF000000, 0xFF00, 0xFF, 0},
		{0xFF000000, 0xFF0000, 0xFF00, 0xFF}, {0xFF, 0xFF00, 0xFF0000, 0xFF000000}, {0xFF0000, 0xFF00, 0xFF, 0xFF000000},
		{0xFF000000, 0xFF00, 0xFF, 0xFF0000}, {0, 0, 0, 0}, {0xFF, 0xFF00, 0xFF0000, 0},
	}
	for _, hs := range []int{40, 52, 56, 108} {
		for i, m := range masks32 {
			spec := bmpSpec{headerSize: hs, width: 5, height: 3, bits: 32, compression: compBitfields, masks: m,
				masksAfter: hs == 40, pixels: rawRows(randomSamples(rng, 15, 1<<32), 5, 3, 32, false)}
			add("bitfields", fmt.Sprintf("h%d-32-%d", hs, i), spec.bytes())
		}
		for i, m := range [][4]uint32{{0xF800, 0x7E0, 0x1F, 0}, {0x7C00, 0x3E0, 0x1F, 0}, {0x1F, 0x7E0, 0xF800, 0}} {
			spec := bmpSpec{headerSize: hs, width: 6, height: 4, bits: 16, compression: compBitfields, masks: m,
				masksAfter: hs == 40, pixels: rawRows(randomSamples(rng, 24, 1<<16), 6, 4, 16, false)}
			add("bitfields", fmt.Sprintf("h%d-16-%d", hs, i), spec.bytes())
		}
		for i, m := range [][4]uint32{{0xFF0000, 0xFF00, 0xFF, 0}, {0xFF, 0xFF00, 0xFF0000, 0}} {
			spec := bmpSpec{headerSize: hs, width: 6, height: 4, bits: 24, compression: compBitfields, masks: m,
				masksAfter: hs == 40, pixels: rawRows(randomSamples(rng, 24, 1<<24), 6, 4, 24, false)}
			add("bitfields", fmt.Sprintf("h%d-24-%d", hs, i), spec.bytes())
		}
		spec := bmpSpec{headerSize: hs, width: 6, height: 4, bits: 8, compression: compBitfields, palette: bgrxPalette(rng, 256, 4)}
		add("bitfields", fmt.Sprintf("h%d-8", hs), spec.bytes())
	}
	add("bitfields", "masks-missing", bmpSpec{headerSize: 40, width: 2, height: 2, bits: 32, compression: compBitfields}.bytes())
	// RLE.
	for _, four := range []bool{false, true} {
		bits, colors := 8, 256
		comp := uint32(compRLE8)
		if four {
			bits, colors, comp = 4, 16, compRLE4
		}
		for k, sz := range [][2]int{{1, 1}, {5, 3}, {16, 7}, {31, 12}} {
			w, h := sz[0], sz[1]
			samples := make([]byte, w*h)
			for i := range samples {
				if rng.Intn(3) == 0 {
					samples[i] = byte(rng.Intn(colors))
				} else if i > 0 {
					samples[i] = samples[i-1]
				}
			}
			spec := bmpSpec{headerSize: 40, width: int32(w), height: int32(h), bits: bits, compression: comp,
				palette: bgrxPalette(rng, colors, 4), pixels: rleEncode(samples, w, h, four)}
			add("rle", fmt.Sprintf("b%d-%d", bits, k), spec.bytes())
			spec.height = -spec.height
			add("rle", fmt.Sprintf("b%d-%d-topdown", bits, k), spec.bytes())
		}
		crafted := map[string][]byte{
			"delta":          {3, 1, 0, 2, 1, 1, 2, 2, 0, 0, 4, 3, 0, 1},
			"overlong-run":   {200, 5, 0, 0, 0, 1},
			"early-eob":      {2, 7, 0, 1},
			"no-eob":         {5, 1, 0, 0, 5, 2},
			"absolute-odd":   {0, 3, 1, 2, 3, 0, 0, 0, 0, 5, 4, 5, 6, 7, 8, 0, 0, 1},
			"absolute-short": {0, 9, 1, 2},
			"delta-short":    {0, 2, 1},
			"only-eol":       {0, 0, 0, 0, 0, 0, 0, 1},
			"gray-palette":   {5, 1, 0, 0, 5, 2, 0, 0, 5, 3, 0, 1},
		}
		var names []string
		for n := range crafted {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			spec := bmpSpec{headerSize: 40, width: 5, height: 3, bits: bits, compression: comp,
				palette: bgrxPalette(rng, colors, 4), pixels: crafted[n]}
			if n == "gray-palette" {
				spec.palette = grayBGRX(colors, 4, func(i int) byte { return byte(i) })
			}
			add("rle", fmt.Sprintf("b%d-%s", bits, n), spec.bytes())
		}
	}
	add("rle", "rle8-1bit", bmpSpec{headerSize: 40, width: 4, height: 2, bits: 1, compression: compRLE8,
		palette: grayBGRX(2, 4, func(i int) byte { return byte(255 * i) }), pixels: []byte{4, 1, 0, 0, 4, 0, 0, 1}}.bytes())
	// Structure.
	good := bmpSpec{headerSize: 40, width: 6, height: 5, bits: 24, pixels: rawRows(randomSamples(rng, 30, 1<<24), 6, 5, 24, false)}
	add("structure", "zero-width", bmpSpec{headerSize: 40, width: 0, height: 5, bits: 24}.bytes())
	add("structure", "zero-height", bmpSpec{headerSize: 40, width: 5, height: 0, bits: 24}.bytes())
	add("structure", "bad-header-size", bmpSpec{headerSize: 44, width: 5, height: 5, bits: 24, pixels: good.pixels}.bytes())
	add("structure", "bits-2", bmpSpec{headerSize: 40, width: 5, height: 5, bits: 2, palette: bgrxPalette(rng, 4, 4)}.bytes())
	add("structure", "compression-jpeg", bmpSpec{headerSize: 40, width: 5, height: 5, bits: 24, compression: 4}.bytes())
	far := good
	far.offset = 5000
	add("structure", "offset-past-end", far.bytes())
	short := good
	short.offset = 14 + 40 + 7
	add("structure", "offset-inside", short.bytes())
	add("structure", "huge", bmpSpec{headerSize: 40, width: 30000, height: 30000, bits: 24}.bytes())
	add("structure", "big-warning", bmpSpec{headerSize: 40, width: 10000, height: 10000, bits: 24}.bytes())
	full := good.bytes()
	for n := 0; n < len(full); n += 3 {
		add("truncated", fmt.Sprint(n), full[:n])
	}
	pal := bmpSpec{headerSize: 40, width: 7, height: 3, bits: 4, palette: bgrxPalette(rng, 16, 4),
		pixels: rawRows(randomSamples(rng, 21, 16), 7, 3, 4, false)}.bytes()
	for n := 0; n < len(pal); n += 5 {
		add("truncated", fmt.Sprintf("pal-%d", n), pal[:n])
	}
	for k := 0; k < 60; k++ {
		src := cases[rng.Intn(200)].input
		bad := append([]byte{}, src...)
		for j := 0; j < 1+rng.Intn(3); j++ {
			bad[rng.Intn(min(len(bad), 64))] = byte(rng.Intn(256))
		}
		add("corrupt", fmt.Sprint(k), bad)
	}
	return cases
}

func pillowSpecs() []map[string]any {
	var specs []map[string]any
	seed := 700
	for _, s := range [][2]int{{1, 1}, {5, 3}, {17, 11}, {100, 64}} {
		for _, mode := range []string{"1", "L", "P", "RGB", "RGBA"} {
			specs = append(specs, map[string]any{
				"kind": "pillow", "format": "BMP", "mode": mode, "colors": 3 + seed%250,
				"w": s[0], "h": s[1], "seed": seed, "pattern": []string{"noise", "smooth"}[seed%2],
			})
			seed++
		}
	}
	return specs
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// tobytes is Pillow's tobytes: mode "1" packs eight pixels per byte.
func tobytes(img *pil.Image) []byte {
	if img.Mode != "1" {
		return img.Pix
	}
	stride := (img.Width + 7) / 8
	out := make([]byte, stride*img.Height)
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if img.Pix[y*img.Width+x] != 0 {
				out[y*stride+x/8] |= 0x80 >> uint(x%8)
			}
		}
	}
	return out
}

func TestOracleBMP(t *testing.T) {
	specs := pillowSpecs()
	genReqs := make([]pyoracle.Request, len(specs))
	for i, s := range specs {
		genReqs[i] = pyoracle.Request{Op: "gen", Gen: s}
	}
	cases := goCases()
	for _, g := range goldenInputs() {
		cases = append(cases, bmpCase{"golden-" + g.name, "golden", g.data})
	}
	for i, r := range pyoracle.Run(t, genReqs) {
		if r.Result != "ok" {
			t.Fatalf("gen %v: %s %s", specs[i], r.Type, r.Message)
		}
		cases = append(cases, bmpCase{fmt.Sprintf("pillow-%d", i), "pillow", r.InputBytes})
	}
	reqs := make([]pyoracle.Request, len(cases))
	for i, c := range cases {
		reqs[i] = pyoracle.Request{Op: "decode", Input: c.input, Want: []string{"pixels"}}
	}
	resp := pyoracle.Run(t, reqs)
	type tally struct{ cases, same int }
	tallies := map[string]*tally{}
	var failures []string
	for i, c := range cases {
		tl := tallies[c.class]
		if tl == nil {
			tl = &tally{}
			tallies[c.class] = tl
		}
		tl.cases++
		if msg := compare(c, resp[i]); msg != "" {
			failures = append(failures, c.name+": "+msg)
			continue
		}
		tl.same++
	}
	var classes []string
	for k := range tallies {
		classes = append(classes, k)
	}
	sort.Strings(classes)
	for _, k := range classes {
		t.Logf("%-10s %4d cases, %4d identical", k, tallies[k].cases, tallies[k].same)
	}
	for i, f := range failures {
		if i >= 40 {
			t.Errorf("... %d more", len(failures)-40)
			break
		}
		t.Error(f)
	}
}

func compare(c bmpCase, r pyoracle.Response) string {
	prefix := c.input[:min(16, len(c.input))]
	var plugin pil.Plugin
	switch {
	case Accept(prefix):
		plugin = Plugin
	case DIBAccept(prefix):
		plugin = DIBPlugin
	default:
		if r.Mode != "" && (r.Format == "BMP" || r.Format == "DIB") {
			return "Go rejects prefix, Pillow opened"
		}
		return ""
	}
	pyOpened := r.Mode != "" && r.Format == plugin.Name
	o, err := plugin.Open(c.input)
	if err != nil {
		if pyOpened {
			return fmt.Sprintf("Go open failed (%v), Pillow opened %s", err, r.Mode)
		}
		if r.Mode != "" {
			if !pil.IsNext(err) {
				return fmt.Sprintf("Go raised (%v), Pillow opened it as %s", err, r.Format)
			}
			return ""
		}
		if pil.IsNext(err) != (r.Type == "UnidentifiedImageError") {
			return fmt.Sprintf("Go %v, Pillow %s: %s", err, r.Type, r.Message)
		}
		return ""
	}
	w, h := o.Size()
	if !pyOpened {
		if pil.CheckBomb(w, h) != nil && (r.Type == "DecompressionBombError" || r.Type == "DecompressionBombWarning") {
			return ""
		}
		return fmt.Sprintf("Go opened %dx%d, Pillow: %s %s (format %q)", w, h, r.Type, r.Message, r.Format)
	}
	if w != r.Width || h != r.Height {
		return fmt.Sprintf("size %dx%d, Pillow %dx%d", w, h, r.Width, r.Height)
	}
	if pil.CheckBomb(w, h) != nil {
		return "Pillow opened a bomb"
	}
	img, err := o.Load()
	if err != nil {
		if r.Result == "ok" {
			return fmt.Sprintf("Go load failed (%v), Pillow loaded", err)
		}
		return ""
	}
	if r.Result != "ok" {
		return fmt.Sprintf("Go loaded, Pillow failed at load: %s %s", r.Type, r.Message)
	}
	if img.Mode != r.LoadedMode {
		return fmt.Sprintf("mode %s, Pillow %s", img.Mode, r.LoadedMode)
	}
	if got := tobytes(img); sha(got) != r.PixelsSHA {
		first := -1
		for i := range got {
			if i < len(r.Pixels) && got[i] != r.Pixels[i] {
				first = i
				break
			}
		}
		return fmt.Sprintf("pixels differ (first %d of %d/%d)", first, len(got), len(r.Pixels))
	}
	if img.Mode == "P" && string(img.Palette) != string(r.Palette) {
		return fmt.Sprintf("palette %d bytes, Pillow %d", len(img.Palette), len(r.Palette))
	}
	return ""
}
