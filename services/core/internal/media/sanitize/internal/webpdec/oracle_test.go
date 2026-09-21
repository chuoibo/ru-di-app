//go:build oracle

package webpdec

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type webpCase struct {
	name  string
	class string
	input []byte
}

func b64(data []byte) map[string]any {
	return map[string]any{"b64": base64.StdEncoding.EncodeToString(data)}
}

type genSpec struct {
	class string
	gen   map[string]any
}

func pillowSpecs() []genSpec {
	var specs []genSpec
	add := func(class, mode string, w, h, seed int, pattern string, save map[string]any) {
		specs = append(specs, genSpec{class, map[string]any{
			"kind": "pillow", "format": "WEBP", "mode": mode, "w": w, "h": h,
			"seed": seed, "pattern": pattern, "save": save,
		}})
	}
	sizes := [][2]int{{1, 1}, {2, 2}, {3, 5}, {16, 16}, {17, 11}, {33, 47}, {100, 75}, {257, 129}, {640, 480}}
	seed := 1
	for _, s := range sizes {
		for _, pattern := range []string{"smooth", "noise", "flat"} {
			for _, q := range []int{1, 50, 90, 100} {
				add("lossy-rgb", "RGB", s[0], s[1], seed, pattern, map[string]any{"quality": q, "method": seed % 7})
				seed++
			}
		}
	}
	for _, s := range sizes {
		for _, pattern := range []string{"smooth", "noise"} {
			for _, aq := range []int{0, 30, 100} {
				add("lossy-rgba", "RGBA", s[0], s[1], seed, pattern, map[string]any{"quality": 70, "alpha_quality": aq, "method": seed % 7})
				seed++
			}
			add("lossy-rgba-exact", "RGBA", s[0], s[1], seed, pattern, map[string]any{"quality": 80, "exact": true})
			seed++
		}
	}
	for _, s := range sizes {
		for _, pattern := range []string{"smooth", "noise", "flat"} {
			for _, mode := range []string{"RGB", "RGBA"} {
				for _, q := range []int{0, 60, 100} {
					add("lossless-"+mode, mode, s[0], s[1], seed, pattern, map[string]any{"lossless": true, "quality": q, "method": seed % 7})
					seed++
				}
			}
		}
	}
	// Palette-like content goes through the colour-indexing transform.
	for i, colors := range []int{2, 3, 5, 16, 17, 200} {
		specs = append(specs, genSpec{"lossless-palette", map[string]any{
			"kind": "pillow", "format": "WEBP", "mode": "P", "colors": colors, "convert": "RGBA",
			"w": 37 + i, "h": 29, "seed": 900 + i, "pattern": "noise",
			"save": map[string]any{"lossless": true, "quality": 100},
		}})
		specs = append(specs, genSpec{"lossy-palette-alpha", map[string]any{
			"kind": "pillow", "format": "WEBP", "mode": "P", "colors": colors, "palette_mode": "RGBA", "convert": "RGBA",
			"w": 40, "h": 23 + i, "seed": 950 + i, "pattern": "noise",
			"save": map[string]any{"quality": 75, "alpha_quality": 50},
		}})
	}
	meta := []byte("Exif\x00\x00MM\x00*\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00\x06\x00\x00\x00\x00\x00\x00")
	xmp := []byte(`<x:xmpmeta><rdf:Description tiff:Orientation="3"/></x:xmpmeta>`)
	add("meta", "RGB", 30, 20, 2001, "smooth", map[string]any{"quality": 80, "exif": b64(meta)})
	add("meta", "RGBA", 30, 20, 2002, "smooth", map[string]any{"quality": 80, "xmp": b64(xmp)})
	add("meta", "RGBA", 30, 20, 2003, "smooth", map[string]any{"lossless": true, "exif": b64(meta), "xmp": b64(xmp), "icc_profile": b64([]byte("not really an icc profile"))})
	add("meta", "RGB", 31, 21, 2004, "noise", map[string]any{"lossless": true, "icc_profile": b64([]byte("icc"))})
	return specs
}

// chunk is one RIFF chunk of a WebP file.
type chunk struct {
	tag  string
	data []byte
}

func parseChunks(t *testing.T, file []byte) []chunk {
	t.Helper()
	var out []chunk
	b := file[12:]
	for len(b) >= 8 {
		size := int(binary.LittleEndian.Uint32(b[4:]))
		if 8+size > len(b) {
			t.Fatalf("chunk %q overruns", b[:4])
		}
		out = append(out, chunk{string(b[:4]), b[8 : 8+size]})
		b = b[8+size+size&1:]
	}
	return out
}

func chunkBytes(tag string, data []byte) []byte {
	var out bytes.Buffer
	out.WriteString(tag)
	binary.Write(&out, binary.LittleEndian, uint32(len(data)))
	out.Write(data)
	if len(data)&1 == 1 {
		out.WriteByte(0)
	}
	return out.Bytes()
}

func riff(body ...[]byte) []byte {
	var payload bytes.Buffer
	payload.WriteString("WEBP")
	for _, b := range body {
		payload.Write(b)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(payload.Len()))
	out.Write(payload.Bytes())
	return out.Bytes()
}

func vp8x(flags byte, w, h int) []byte {
	d := make([]byte, 10)
	d[0] = flags
	put24(d[4:], w-1)
	put24(d[7:], h-1)
	return chunkBytes("VP8X", d)
}

func put24(b []byte, v int) {
	b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16)
}

func anmf(x, y, w, h int, flags byte, body ...[]byte) []byte {
	d := make([]byte, 16)
	put24(d[0:], x/2)
	put24(d[3:], y/2)
	put24(d[6:], w-1)
	put24(d[9:], h-1)
	put24(d[12:], 100)
	d[15] = flags
	for _, b := range body {
		d = append(d, b...)
	}
	return chunkBytes("ANMF", d)
}

// imageInfo returns the bitstream chunks (ALPH first when present) and the
// frame size of a Pillow still WebP.
func imageInfo(t *testing.T, file []byte) (alph, image []byte, tag string, w, h int) {
	t.Helper()
	for _, c := range parseChunks(t, file) {
		switch c.tag {
		case "ALPH":
			alph = c.data
		case "VP8 ", "VP8L":
			image, tag = c.data, c.tag
		}
	}
	if tag == "VP8 " {
		w = int(binary.LittleEndian.Uint16(image[6:]) & 0x3fff)
		h = int(binary.LittleEndian.Uint16(image[8:]) & 0x3fff)
	} else {
		v := binary.LittleEndian.Uint32(image[1:])
		w = int(v&0x3fff) + 1
		h = int((v>>14)&0x3fff) + 1
	}
	return
}

func derivedCases(t *testing.T, base []webpCase) []webpCase {
	var out []webpCase
	rng := rand.New(rand.NewSource(7))
	pick := func(class string) []webpCase {
		var list []webpCase
		for _, c := range base {
			if c.class == class {
				list = append(list, c)
			}
		}
		return list
	}
	stills := append(append(pick("lossy-rgb"), pick("lossy-rgba")...), append(pick("lossless-RGB"), pick("lossless-RGBA")...)...)
	for i, c := range stills {
		if i%5 != 0 {
			continue
		}
		alph, image, tag, w, h := imageInfo(t, c.input)
		var frameChunks [][]byte
		if alph != nil {
			frameChunks = append(frameChunks, chunkBytes("ALPH", alph))
		}
		frameChunks = append(frameChunks, chunkBytes(tag, image))
		// VP8X still, with and without the alpha flag.
		for _, flags := range []byte{0x10, 0x00, 0x08 | 0x10, 0x01} {
			body := append([][]byte{vp8x(flags, w, h)}, frameChunks...)
			out = append(out, webpCase{fmt.Sprintf("%s-vp8x-%02x", c.name, flags), "container-vp8x", riff(body...)})
		}
		// Unknown chunk between VP8X and the bitstream.
		body := append([][]byte{vp8x(0x10, w, h), chunkBytes("ABCD", []byte{1, 2, 3})}, frameChunks...)
		out = append(out, webpCase{c.name + "-unknown-chunk", "container-vp8x", riff(body...)})
		// EXIF chunk after the image, flag set.
		body = append([][]byte{vp8x(0x10|0x08, w, h)}, frameChunks...)
		body = append(body, chunkBytes("EXIF", []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00")))
		out = append(out, webpCase{c.name + "-exif-after", "container-vp8x", riff(body...)})
		// Canvas mismatch.
		body = append([][]byte{vp8x(0x10, w+2, h)}, frameChunks...)
		out = append(out, webpCase{c.name + "-canvas-mismatch", "container-vp8x", riff(body...)})
		// Animation: frame 1 placed on a larger canvas at an offset.
		for _, off := range [][2]int{{0, 0}, {2, 4}, {6, 0}} {
			cw, ch := w+off[0]+4, h+off[1]+2
			body = [][]byte{vp8x(0x02|0x10, cw, ch), chunkBytes("ANIM", []byte{1, 2, 3, 4, 0, 0})}
			body = append(body, anmf(off[0], off[1], w, h, 0, frameChunks...))
			body = append(body, anmf(0, 0, w, h, 2, frameChunks...))
			out = append(out, webpCase{fmt.Sprintf("%s-anim-%d-%d", c.name, off[0], off[1]), "container-anim", riff(body...)})
		}
		// Truncation and trailing bytes.
		for _, cut := range []int{1, 2, 7, len(c.input) / 2, len(c.input) - 21} {
			if cut <= 0 || cut >= len(c.input) {
				continue
			}
			out = append(out, webpCase{fmt.Sprintf("%s-cut-%d", c.name, cut), "truncated", c.input[:len(c.input)-cut]})
		}
		trail := append(append([]byte{}, c.input...), 0, 1, 2, 3)
		out = append(out, webpCase{c.name + "-trailing", "container-simple", trail})
		// RIFF size smaller than the data.
		short := append([]byte{}, c.input...)
		binary.LittleEndian.PutUint32(short[4:], binary.LittleEndian.Uint32(short[4:])-2)
		out = append(out, webpCase{c.name + "-riff-short", "container-simple", short})
		// Corrupt bitstream bytes.
		for k := 0; k < 3; k++ {
			bad := append([]byte{}, c.input...)
			start := 30
			if start < len(bad) {
				pos := start + rng.Intn(len(bad)-start)
				bad[pos] ^= byte(1 + rng.Intn(255))
				out = append(out, webpCase{fmt.Sprintf("%s-corrupt-%d", c.name, k), "corrupt", bad})
			}
		}
	}
	return out
}

type infoValue struct {
	Bytes string `json:"bytes"`
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestOracleWebP(t *testing.T) {
	specs := pillowSpecs()
	genReqs := make([]pyoracle.Request, len(specs))
	for i, s := range specs {
		genReqs[i] = pyoracle.Request{Op: "gen", Gen: s.gen}
	}
	genResp := pyoracle.Run(t, genReqs)
	var base []webpCase
	for i, r := range genResp {
		if r.Result != "ok" {
			t.Fatalf("gen %d (%v): %s %s", i, specs[i].gen, r.Type, r.Message)
		}
		base = append(base, webpCase{fmt.Sprintf("%s-%d", specs[i].class, i), specs[i].class, r.InputBytes})
	}
	cases := append(base, derivedCases(t, base)...)
	for _, g := range goldenInputs() {
		cases = append(cases, webpCase{"golden-" + g.Name, "golden", g.bytes()})
	}
	reqs := make([]pyoracle.Request, len(cases))
	for i, c := range cases {
		reqs[i] = pyoracle.Request{Op: "decode", Input: c.input}
	}
	resp := pyoracle.Run(t, reqs)

	type tally struct{ cases, same int }
	tallies := map[string]*tally{}
	var failures []string
	for i, c := range cases {
		r := resp[i]
		tl := tallies[c.class]
		if tl == nil {
			tl = &tally{}
			tallies[c.class] = tl
		}
		tl.cases++
		if msg := compare(c, r); msg != "" {
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
		t.Logf("%-22s %4d cases, %4d identical", k, tallies[k].cases, tallies[k].same)
	}
	for i, f := range failures {
		if i >= 40 {
			t.Errorf("... %d more", len(failures)-40)
			break
		}
		t.Error(f)
	}
	diagnose(t, cases, resp)
}

// diagnose fetches Pillow's pixels for every pixel mismatch and logs where
// the two decodes part.
func diagnose(t *testing.T, cases []webpCase, resp []pyoracle.Response) {
	var idx []int
	for i, c := range cases {
		if compare(c, resp[i]) == "pixels differ" {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return
	}
	reqs := make([]pyoracle.Request, len(idx))
	for k, i := range idx {
		reqs[k] = pyoracle.Request{Op: "decode", Input: cases[i].input, Want: []string{"pixels"}}
	}
	for k, r := range pyoracle.Run(t, reqs) {
		c := cases[idx[k]]
		o, _ := Open(c.input)
		img, _ := o.Load()
		bpp := len(img.Pix) / (img.Width * img.Height)
		first, count, maxDelta := -1, 0, 0
		minX, minY, maxX, maxY := img.Width, img.Height, -1, -1
		for p := 0; p < len(img.Pix); p++ {
			if img.Pix[p] == r.Pixels[p] {
				continue
			}
			if first < 0 {
				first = p
			}
			count++
			d := int(img.Pix[p]) - int(r.Pixels[p])
			if d < 0 {
				d = -d
			}
			if d > maxDelta {
				maxDelta = d
			}
			x, y := (p/bpp)%img.Width, (p/bpp)/img.Width
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
		t.Logf("%s: %dx%d %s, %d bytes differ, first byte %d (x %d y %d), box x %d..%d y %d..%d, max delta %d",
			c.name, img.Width, img.Height, img.Mode, count, first, (first/bpp)%img.Width, (first/bpp)/img.Width,
			minX, maxX, minY, maxY, maxDelta)
	}
}

func compare(c webpCase, r pyoracle.Response) string {
	pyOpened := r.Mode != ""
	if pyOpened && r.Format != "WEBP" {
		return "opened by Pillow as " + r.Format
	}
	if !Accept(c.input[:min(16, len(c.input))]) {
		if pyOpened {
			return "Go rejects the prefix, Pillow opened it"
		}
		return ""
	}
	o, err := Open(c.input)
	if err != nil {
		if pyOpened {
			return fmt.Sprintf("Go open failed (%v), Pillow opened %s %dx%d", err, r.Mode, r.Width, r.Height)
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
		return "pixels differ"
	}
	var info map[string]json.RawMessage
	if err := json.Unmarshal(r.Info, &info); err != nil {
		return "info: " + err.Error()
	}
	for key, got := range map[string][]byte{"exif": img.Info.Exif, "xmp": img.Info.XMP} {
		raw, ok := info[key]
		if ok != (got != nil) {
			return fmt.Sprintf("info %s presence Go %v, Pillow %v", key, got != nil, ok)
		}
		if ok {
			var v infoValue
			json.Unmarshal(raw, &v)
			if v.Bytes != base64.StdEncoding.EncodeToString(got) {
				return "info " + key + " differs"
			}
		}
	}
	return ""
}
