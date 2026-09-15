//go:build oracle

package gifdec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type gifCase struct {
	name  string
	class string
	input []byte
}

func goCases() []gifCase {
	var cases []gifCase
	add := func(class, name string, data []byte) {
		cases = append(cases, gifCase{class + "-" + name, class, data})
	}
	rng := rand.New(rand.NewSource(11))
	sizes := [][2]int{{1, 1}, {2, 3}, {7, 5}, {8, 8}, {17, 11}, {64, 1}, {1, 40}, {100, 61}}
	for si, s := range sizes {
		for _, bits := range []int{1, 2, 4, 8} {
			colors := 1 << uint(bits)
			for _, pattern := range []string{"noise", "ramp", "flat"} {
				for _, interlace := range []bool{false, true} {
					g := &gifFile{version: "GIF89a", width: s[0], height: s[1], global: palette(rng, colors), globalBits: bits}
					idx := indices(rng, s[0]*s[1], colors, pattern)
					g.image(0, 0, s[0], s[1], interlace, nil, 0, encodeLZW(idx, bits), byte(max(bits, 2)))
					add("valid", fmt.Sprintf("%dx%d-b%d-%s-i%v-%d", s[0], s[1], bits, pattern, interlace, si), g.bytes())
				}
			}
		}
	}
	// Transparency, gray palettes, local tables, offsets.
	for k := 0; k < 12; k++ {
		w, h := 5+rng.Intn(40), 3+rng.Intn(40)
		g := &gifFile{version: "GIF89a", width: w, height: h}
		switch k % 4 {
		case 0:
			g.global, g.globalBits = grayPalette(256), 8
		case 1:
			g.global, g.globalBits = palette(rng, 16), 4
		case 2:
			g.global, g.globalBits = grayPalette(16), 4
		}
		g.gce(byte(k%2)|byte((k%4)<<2), 10, byte(rng.Intn(16)))
		g.comment("seeded comment")
		if k%3 == 0 {
			g.netscape(0)
		}
		var local []byte
		localBits := 0
		if k%4 == 3 {
			local, localBits = palette(rng, 4), 2
			if k%8 == 7 {
				local = grayPalette(4)
			}
		}
		fx, fy := rng.Intn(4), rng.Intn(4)
		fw, fh := w-fx, h-fy
		if k%5 == 0 {
			fw += 3 // extends the screen
		}
		idx := indices(rng, fw*fh, 4, "noise")
		g.image(fx, fy, fw, fh, k%2 == 1, local, localBits, encodeLZW(idx, 2), 2)
		add("features", fmt.Sprint(k), g.bytes())
	}
	// Two frames: only the first is read.
	{
		g := &gifFile{version: "GIF89a", width: 9, height: 9, global: palette(rng, 4), globalBits: 2}
		g.gce(0x09, 5, 1)
		g.image(0, 0, 9, 9, false, nil, 0, encodeLZW(indices(rng, 81, 4, "noise"), 2), 2)
		g.gce(0x05, 5, 2)
		g.image(2, 2, 4, 4, false, nil, 0, encodeLZW(indices(rng, 16, 4, "noise"), 2), 2)
		add("features", "two-frames", g.bytes())
	}
	// Malformed LZW streams.
	lzwCases := map[string][]byte{
		"early-end":       rawCodes([]int{4, 1, 2, 5}, 3),
		"no-end":          rawCodes([]int{4, 1, 2, 3, 1}, 3),
		"code-past-next":  rawCodes([]int{4, 1, 7, 5}, 3),
		"first-code-high": rawCodes([]int{4, 6, 1, 5}, 3),
		"only-clear":      rawCodes([]int{4, 4, 4, 5}, 3),
		"no-clear":        rawCodes([]int{1, 2, 3, 0, 1, 2, 3, 0, 1, 5}, 3),
		"kwkwk":           rawCodes([]int{4, 1, 6, 6, 6, 5}, 3),
		"empty":           nil,
	}
	var names []string
	for n := range lzwCases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		g := &gifFile{version: "GIF87a", width: 3, height: 3, global: palette(rng, 4), globalBits: 2}
		g.image(0, 0, 3, 3, false, nil, 0, lzwCases[n], 2)
		add("lzw", n, g.bytes())
		big := &gifFile{version: "GIF89a", width: 2, height: 2, global: palette(rng, 4), globalBits: 2}
		big.image(0, 0, 2, 2, false, nil, 0, lzwCases[n], 2)
		add("lzw", n+"-2x2", big.bytes())
	}
	for _, bits := range []byte{0, 1, 11, 12, 13, 200} {
		g := &gifFile{version: "GIF89a", width: 4, height: 4, global: palette(rng, 4), globalBits: 2}
		g.image(0, 0, 4, 4, false, nil, 0, encodeLZW(indices(rng, 16, 2, "noise"), 2), bits)
		add("lzw", fmt.Sprintf("bits-%d", bits), g.bytes())
	}
	// Structure edge cases.
	base := func() *gifFile {
		return &gifFile{version: "GIF89a", width: 6, height: 5, global: palette(rng, 8), globalBits: 3}
	}
	{
		g := base()
		g.image(0, 0, 0, 5, false, nil, 0, encodeLZW(nil, 3), 3)
		add("structure", "zero-width-frame", g.bytes())
		g = base()
		g.body.Write([]byte{0x00, 0x42, 0x13})
		g.image(0, 0, 6, 5, false, nil, 0, encodeLZW(indices(rng, 30, 8, "noise"), 3), 3)
		add("structure", "junk-before-image", g.bytes())
		g = base()
		add("structure", "no-image", g.bytes())
		g = base()
		g.noTrailer = true
		g.image(0, 0, 6, 5, false, nil, 0, encodeLZW(indices(rng, 30, 8, "noise"), 3), 3)
		add("structure", "no-trailer", g.bytes())
		g = base()
		g.body.Write([]byte{'!', 0xf9, 2, 1, 0, 0})
		g.image(0, 0, 6, 5, false, nil, 0, encodeLZW(indices(rng, 30, 8, "noise"), 3), 3)
		add("structure", "short-gce", g.bytes())
		g = base()
		g.body.Write([]byte{'!', 0xf9, 3, 1, 0, 0, 0})
		g.image(0, 0, 6, 5, false, nil, 0, encodeLZW(indices(rng, 30, 8, "noise"), 3), 3)
		add("structure", "gce-no-index", g.bytes())
		g = base()
		g.body.Write([]byte{'!', 0x01, 5, 1, 2, 3, 4, 5, 0})
		g.image(0, 0, 6, 5, false, nil, 0, encodeLZW(indices(rng, 30, 8, "noise"), 3), 3)
		add("structure", "unknown-extension", g.bytes())
		g = base()
		g.image(3000, 0, 6, 5, false, nil, 0, encodeLZW(indices(rng, 30, 8, "noise"), 3), 3)
		add("structure", "offset-expands", g.bytes())
		g = &gifFile{version: "GIF89a", width: 5, height: 5, global: palette(rng, 8)[:20], globalBits: 3}
		add("structure", "short-palette-bytes", g.bytes())
	}
	// Truncations of a valid file at every length.
	g := base()
	g.gce(0x01, 3, 2)
	g.image(1, 0, 5, 5, true, palette(rng, 2), 1, encodeLZW(indices(rng, 25, 2, "noise"), 2), 2)
	full := g.bytes()
	for n := 0; n < len(full); n++ {
		add("truncated", fmt.Sprint(n), full[:n])
	}
	// Random corruption of valid files.
	for k := 0; k < 60; k++ {
		src := cases[rng.Intn(48)].input
		bad := append([]byte{}, src...)
		for j := 0; j < 1+rng.Intn(3); j++ {
			bad[13+rng.Intn(len(bad)-13)] = byte(rng.Intn(256))
		}
		add("corrupt", fmt.Sprint(k), bad)
	}
	return cases
}

func pillowSpecs() []map[string]any {
	var specs []map[string]any
	seed := 500
	for _, s := range [][2]int{{1, 1}, {16, 16}, {33, 17}, {200, 150}} {
		for _, mode := range []string{"P", "L", "RGB", "1"} {
			for _, save := range []map[string]any{
				{}, {"optimize": false}, {"interlace": false}, {"transparency": 3},
				{"comment": "hello", "loop": 0, "duration": 40}, {"disposal": 2, "transparency": 0},
			} {
				specs = append(specs, map[string]any{
					"kind": "pillow", "format": "GIF", "mode": mode, "colors": 7 + seed%200,
					"w": s[0], "h": s[1], "seed": seed, "pattern": []string{"noise", "smooth", "flat"}[seed%3],
					"save": save,
				})
				seed++
			}
		}
	}
	return specs
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type infoValue struct {
	Repr string `json:"repr"`
}

func TestOracleGIF(t *testing.T) {
	specs := pillowSpecs()
	genReqs := make([]pyoracle.Request, len(specs))
	for i, s := range specs {
		genReqs[i] = pyoracle.Request{Op: "gen", Gen: s}
	}
	cases := goCases()
	for _, g := range goldenInputs() {
		cases = append(cases, gifCase{"golden-" + g.name, "golden", g.data})
	}
	for i, r := range pyoracle.Run(t, genReqs) {
		if r.Result != "ok" {
			t.Fatalf("gen %v: %s %s", specs[i], r.Type, r.Message)
		}
		cases = append(cases, gifCase{fmt.Sprintf("pillow-%d", i), "pillow", r.InputBytes})
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

func compare(c gifCase, r pyoracle.Response) string {
	pyOpened := r.Mode != ""
	if pyOpened && r.Format != "GIF" {
		if Accept(c.input[:min(16, len(c.input))]) {
			// Only valid when GIF raised a next-plugin error.
			if _, err := Open(c.input); err == nil || !pil.IsNext(err) {
				return "Pillow opened as " + r.Format + " but GIF would not pass"
			}
		}
		return ""
	}
	if !Accept(c.input[:min(16, len(c.input))]) {
		if pyOpened {
			return "Go rejects prefix, Pillow opened"
		}
		return ""
	}
	o, err := Open(c.input)
	if err != nil {
		if pyOpened {
			return fmt.Sprintf("Go open failed (%v), Pillow opened %s", err, r.Mode)
		}
		if pil.IsNext(err) && r.Type != "UnidentifiedImageError" {
			return fmt.Sprintf("Go next-plugin (%v), Pillow raised %s: %s", err, r.Type, r.Message)
		}
		if !pil.IsNext(err) && r.Type == "UnidentifiedImageError" {
			return fmt.Sprintf("Go raised (%v), Pillow tried other plugins", err)
		}
		return ""
	}
	w, h := o.Size()
	if !pyOpened {
		return fmt.Sprintf("Go opened %dx%d, Pillow failed at open: %s %s", w, h, r.Type, r.Message)
	}
	if w != r.Width || h != r.Height {
		return fmt.Sprintf("size %dx%d, Pillow %dx%d", w, h, r.Width, r.Height)
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
	if sha(img.Pix) != r.PixelsSHA {
		first := -1
		for i := range img.Pix {
			if i < len(r.Pixels) && img.Pix[i] != r.Pixels[i] {
				first = i
				break
			}
		}
		return fmt.Sprintf("pixels differ (first %d)", first)
	}
	if img.Mode == "P" && string(img.Palette) != string(r.Palette) {
		return fmt.Sprintf("palette %d bytes, Pillow %d", len(img.Palette), len(r.Palette))
	}
	var info map[string]json.RawMessage
	json.Unmarshal(r.Info, &info)
	raw, ok := info["transparency"]
	if ok != (img.Info.Transparency != nil) {
		return "transparency presence differs"
	}
	if ok {
		var v infoValue
		json.Unmarshal(raw, &v)
		if v.Repr != fmt.Sprint(img.Info.Transparency.Int) {
			return "transparency " + v.Repr + " vs " + fmt.Sprint(img.Info.Transparency.Int)
		}
	}
	return ""
}
