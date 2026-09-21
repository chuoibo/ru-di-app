//go:build oracle

package jpegdec

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

// Run from services/core:
//
//	go test -tags oracle -run 'TestOracle' -v -timeout 30m ./internal/media/sanitize/internal/jpegdec/
//
// JPEGDEC_ORACLE_SOFT=1 logs disagreements without failing.

// oracleCase is one input: built by Pillow from gen, or by Go as input.
type oracleCase struct {
	class string
	name  string
	gen   map[string]any
	input []byte
}

type tally struct {
	cases, identical, bothFailed, unported, mismatched int
	notes                                              []string
}

func pillowGen(mode string, w, h, seed int, pattern string, save map[string]any) map[string]any {
	g := map[string]any{"kind": "pillow", "format": "JPEG", "mode": mode, "w": w, "h": h, "seed": seed, "pattern": pattern}
	if save != nil {
		g["save"] = save
	}
	return g
}

func b64(data []byte) map[string]any {
	return map[string]any{"b64": base64.StdEncoding.EncodeToString(data)}
}

// runDecode sends every case to op "decode" and compares Pillow's answer
// with Open/Load. It returns every input by ID (Pillow-built ones as
// Pillow returned them) and the responses.
func runDecode(t *testing.T, cases []oracleCase, tallies map[string]*tally) (map[string][]byte, map[string]pyoracle.Response) {
	t.Helper()
	requests := make([]pyoracle.Request, len(cases))
	for i, c := range cases {
		requests[i] = pyoracle.Request{ID: c.class + "/" + c.name, Op: "decode", Input: c.input, Gen: c.gen, Want: []string{"pixels", "input"}}
	}
	started := time.Now()
	responses := pyoracle.Run(t, requests)
	t.Logf("oracle answered %d decode requests in %s", len(requests), time.Since(started).Round(time.Millisecond))
	inputs := map[string][]byte{}
	byID := map[string]pyoracle.Response{}
	for i, c := range cases {
		r := responses[i]
		id := requests[i].ID
		input := c.input
		if input == nil {
			input = r.InputBytes
		}
		inputs[id] = input
		byID[id] = r
		tl := tallies[c.class]
		if tl == nil {
			tl = &tally{}
			tallies[c.class] = tl
		}
		tl.cases++
		switch verdict := compareDecode(input, r); {
		case verdict == "identical":
			tl.identical++
		case verdict == "both-failed":
			tl.identical++
			tl.bothFailed++
		case strings.HasPrefix(verdict, "unported:"):
			tl.unported++
			tl.notes = append(tl.notes, id+": "+verdict)
		default:
			tl.mismatched++
			tl.notes = append(tl.notes, id+": "+verdict)
		}
	}
	return inputs, byID
}

// compareDecode classifies Go against one Pillow answer.
func compareDecode(input []byte, r pyoracle.Response) string {
	jpegOpened := r.Format == "JPEG" || r.Format == "MPO"
	op, openErr := Open(input)
	if !jpegOpened {
		if openErr == nil {
			return fmt.Sprintf("Go opened, Pillow did not (%q %s: %s)", r.Format, r.Type, trim(r.Message))
		}
		if r.Format != "" {
			if !pil.IsNext(openErr) {
				return fmt.Sprintf("Pillow fell through to %s, Go raised %v", r.Format, openErr)
			}
			return "both-failed"
		}
		if r.Type == "UnidentifiedImageError" && !pil.IsNext(openErr) {
			return fmt.Sprintf("Pillow tried other plugins, Go raised %v", openErr)
		}
		if r.Type != "UnidentifiedImageError" && pil.IsNext(openErr) {
			return fmt.Sprintf("Pillow raised %s (%s), Go passed to the next plugin (%v)", r.Type, trim(r.Message), openErr)
		}
		return "both-failed"
	}
	if openErr != nil {
		return fmt.Sprintf("Pillow opened %s, Go: %v", r.Format, openErr)
	}
	w, h := op.Size()
	o := op.(*opened)
	if o.format != r.Format || o.mode != r.Mode || w != r.Width || h != r.Height {
		return fmt.Sprintf("header: Go %s %s %dx%d, Pillow %s %s %dx%d", o.format, o.mode, w, h, r.Format, r.Mode, r.Width, r.Height)
	}
	if r.Result == "ok" {
		// The oracle reports info only after a successful load.
		if note := compareInfo(o, r.Info); note != "" {
			return note
		}
	}
	img, loadErr := op.Load()
	if r.Result != "ok" {
		if loadErr == nil {
			return fmt.Sprintf("Go loaded, Pillow raised %s: %s", r.Type, trim(r.Message))
		}
		var unsupported *pil.UnsupportedError
		if errors.As(loadErr, &unsupported) {
			return "unported (Pillow failed too): " + loadErr.Error()
		}
		return "both-failed"
	}
	if loadErr != nil {
		var unsupported *pil.UnsupportedError
		if errors.As(loadErr, &unsupported) {
			return "unported: Pillow loaded it, Go reports " + loadErr.Error()
		}
		return fmt.Sprintf("Pillow loaded, Go: %v", loadErr)
	}
	if len(img.Pix) != len(r.Pixels) {
		return fmt.Sprintf("pixel length %d vs %d", len(img.Pix), len(r.Pixels))
	}
	if diff, maxDiff := sampleDiff(img.Pix, r.Pixels); diff != 0 {
		return fmt.Sprintf("pixels: %d of %d samples differ, max abs %d", diff, len(img.Pix), maxDiff)
	}
	return "identical"
}

func sampleDiff(a, b []byte) (diff, maxDiff int) {
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d != 0 {
			diff++
			maxDiff = max(maxDiff, d)
		}
	}
	return diff, maxDiff
}

func compareInfo(o *opened, raw json.RawMessage) string {
	var info map[string]json.RawMessage
	if err := json.Unmarshal(raw, &info); err != nil {
		return "info: " + err.Error()
	}
	check := func(key string, has bool, value []byte) string {
		entry, ok := info[key]
		if ok != has {
			return fmt.Sprintf("info %q presence Go %v Pillow %v", key, has, ok)
		}
		if !ok {
			return ""
		}
		var v struct {
			Bytes []byte `json:"bytes"`
		}
		if err := json.Unmarshal(entry, &v); err != nil {
			return "info " + key + ": " + err.Error()
		}
		if !bytes.Equal(v.Bytes, value) {
			return fmt.Sprintf("info %q differs (%d vs %d bytes)", key, len(value), len(v.Bytes))
		}
		return ""
	}
	if note := check("exif", o.info.HasExif, o.info.Exif); note != "" {
		return note
	}
	return check("xmp", o.info.HasXMP, o.info.XMP)
}

func trim(s string) string {
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

func report(t *testing.T, tallies map[string]*tally) {
	t.Helper()
	classes := make([]string, 0, len(tallies))
	for class := range tallies {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	total, same, unported := 0, 0, 0
	for _, class := range classes {
		tl := tallies[class]
		total += tl.cases
		same += tl.identical
		unported += tl.unported
		t.Logf("%-26s cases %4d  agree %4d (both failed %3d)  unported %d  mismatched %d", class, tl.cases, tl.identical, tl.bothFailed, tl.unported, tl.mismatched)
		for i, note := range tl.notes {
			if i == 12 {
				t.Logf("    ... %d more", len(tl.notes)-12)
				break
			}
			t.Logf("    %s", note)
		}
	}
	t.Logf("total %d, agree %d, unported %d", total, same, unported)
	if same+unported != total && os.Getenv("JPEGDEC_ORACLE_SOFT") == "" {
		t.Errorf("%d cases disagree", total-same-unported)
	}
}

// pillowCases are inputs Pillow's own encoder writes.
func pillowCases() []oracleCase {
	var cases []oracleCase
	add := func(class, name string, gen map[string]any) {
		cases = append(cases, oracleCase{class: class, name: name, gen: gen})
	}
	sizes := [][2]int{{1, 1}, {2, 2}, {7, 9}, {8, 8}, {9, 9}, {15, 17}, {16, 16}, {17, 15}, {33, 1}, {1, 33}, {64, 128}, {640, 480}, {1001, 750}}
	qualities := []int{50, 75, 95, 100}
	seed := 1
	for zi, size := range sizes {
		for ss := 0; ss <= 2; ss++ {
			seed++
			pattern := []string{"smooth", "noise", "flat"}[(zi+ss)%3]
			q := qualities[(zi+ss)%len(qualities)]
			add("pillow-baseline", fmt.Sprintf("%dx%d-ss%d-q%d-%s", size[0], size[1], ss, q, pattern),
				pillowGen("RGB", size[0], size[1], seed, pattern, map[string]any{"quality": q, "subsampling": ss}))
			add("pillow-progressive", fmt.Sprintf("%dx%d-ss%d-q%d-%s", size[0], size[1], ss, q, pattern),
				pillowGen("RGB", size[0], size[1], seed, pattern, map[string]any{"quality": q, "subsampling": ss, "progressive": true}))
		}
		add("pillow-l", fmt.Sprintf("%dx%d", size[0], size[1]), pillowGen("L", size[0], size[1], seed, "smooth", map[string]any{"quality": 90}))
		add("pillow-l-progressive", fmt.Sprintf("%dx%d", size[0], size[1]), pillowGen("L", size[0], size[1], seed, "noise", map[string]any{"progressive": true}))
		add("pillow-cmyk", fmt.Sprintf("%dx%d", size[0], size[1]), pillowGen("CMYK", size[0], size[1], seed, "smooth", nil))
		add("pillow-cmyk-progressive", fmt.Sprintf("%dx%d", size[0], size[1]), pillowGen("CMYK", size[0], size[1], seed, "noise", map[string]any{"progressive": true}))
	}
	for _, blocks := range []int{1, 3, 7} {
		add("pillow-restart", fmt.Sprintf("blocks-%d", blocks), pillowGen("RGB", 131, 97, 400+blocks, "noise", map[string]any{"restart_marker_blocks": blocks}))
		add("pillow-restart", fmt.Sprintf("blocks-%d-progressive", blocks), pillowGen("RGB", 131, 97, 410+blocks, "smooth", map[string]any{"restart_marker_blocks": blocks, "progressive": true}))
	}
	for _, rows := range []int{1, 2} {
		add("pillow-restart", fmt.Sprintf("rows-%d", rows), pillowGen("RGB", 77, 55, 420+rows, "smooth", map[string]any{"restart_marker_rows": rows, "subsampling": 1}))
	}
	add("pillow-optimize", "baseline", pillowGen("RGB", 300, 200, 430, "noise", map[string]any{"optimize": true}))
	add("pillow-optimize", "gray", pillowGen("L", 300, 200, 431, "smooth", map[string]any{"optimize": true}))
	flat := make([]int, 64)
	big := make([]int, 64)
	for i := range flat {
		flat[i] = 1 + i%7
		big[i] = 300 + i*9
	}
	add("pillow-qtables", "custom-8bit", pillowGen("RGB", 90, 60, 440, "smooth", map[string]any{"qtables": []any{flat, flat}}))
	add("pillow-qtables", "custom-16bit", pillowGen("RGB", 90, 60, 441, "noise", map[string]any{"qtables": []any{big, flat}}))
	add("pillow-qtables", "keep-rgb", pillowGen("RGB", 90, 60, 442, "smooth", map[string]any{"keep_rgb": true, "subsampling": 0}))
	rng := rand.New(rand.NewPCG(7, 11))
	icc := make([]byte, 70000)
	for i := range icc {
		icc[i] = byte(rng.Uint32())
	}
	add("pillow-metadata", "icc-two-segments", pillowGen("RGB", 50, 40, 450, "smooth", map[string]any{"icc_profile": b64(icc)}))
	add("pillow-metadata", "xmp", pillowGen("RGB", 50, 40, 451, "smooth", map[string]any{"xmp": b64([]byte(`<x:xmpmeta tiff:Orientation="8"/>`))}))
	add("pillow-metadata", "comment", pillowGen("RGB", 50, 40, 452, "smooth", map[string]any{"comment": "a comment"}))
	add("pillow-metadata", "dpi", pillowGen("RGB", 50, 40, 453, "smooth", map[string]any{"dpi": map[string]any{"tuple": []int{300, 300}}}))
	for o := 1; o <= 8; o++ {
		g := pillowGen("RGB", 37, 21, 460+o, "smooth", nil)
		g["exif_orientation"] = o
		add("pillow-metadata", fmt.Sprintf("orientation-%d", o), g)
	}
	add("pillow-large", "2000x1500-420", pillowGen("RGB", 2000, 1500, 470, "smooth", map[string]any{"quality": 92}))
	add("pillow-large", "2000x1500-progressive", pillowGen("RGB", 2000, 1500, 471, "smooth", map[string]any{"progressive": true}))
	return cases
}

// markerOffsets lists every marker in data: segment lengths are followed
// outside entropy-coded data, which is scanned byte by byte.
func markerOffsets(data []byte) []int {
	var out []int
	pos := 2
	for pos+1 < len(data) {
		if data[pos] != 0xFF {
			pos++
			continue
		}
		m := data[pos+1]
		switch {
		case m == 0xFF:
			pos++
			continue
		case m == 0x00, m >= 0xD0 && m <= 0xD7:
			pos += 2
			continue
		}
		out = append(out, pos)
		if m == 0xD9 || pos+4 > len(data) {
			break
		}
		pos += 2 + (int(data[pos+2])<<8 | int(data[pos+3]))
	}
	return out
}

// mutationCases derives truncated, scan-stripped and corrupted files from
// Pillow-built inputs.
func mutationCases(inputs map[string][]byte) []oracleCase {
	var cases []oracleCase
	add := func(class, name string, data []byte) {
		cases = append(cases, oracleCase{class: class, name: name, input: data})
	}
	truncSources := []string{
		"pillow-baseline/640x480-ss2-q75-noise", "pillow-progressive/640x480-ss2-q75-noise",
		"pillow-restart/blocks-3", "pillow-restart/blocks-3-progressive",
		"pillow-cmyk-progressive/64x128", "pillow-l-progressive/64x128",
	}
	for _, id := range truncSources {
		data := inputs[id]
		if data == nil {
			continue
		}
		offsets := markerOffsets(data)
		for i, at := range offsets {
			add("pillow-truncate", fmt.Sprintf("%s@marker-%d", id, i), data[:at])
			if at+5 < len(data) {
				add("pillow-truncate", fmt.Sprintf("%s@marker-%d+5", id, i), data[:at+5])
			}
		}
		for _, frac := range []int{10, 50, 97} {
			add("pillow-truncate", fmt.Sprintf("%s@%d-percent", id, frac), data[:len(data)*frac/100])
		}
		add("pillow-truncate", id+"@no-eoi", data[:len(data)-2])
		add("pillow-truncate", id+"@half-eoi", data[:len(data)-1])
	}
	for id, data := range inputs {
		if len(data) == 0 || !(bytes.Contains([]byte(id), []byte("progressive"))) {
			continue
		}
		var sos []int
		for _, at := range markerOffsets(data) {
			if data[at+1] == 0xDA {
				sos = append(sos, at)
			}
		}
		for k := 1; k < len(sos); k++ {
			stripped := append(append([]byte(nil), data[:sos[k]]...), 0xFF, 0xD9)
			add("pillow-scan-strip", fmt.Sprintf("%s@keep-%d-of-%d", id, k, len(sos)), stripped)
		}
	}
	corruptSources := []string{"pillow-baseline/640x480-ss2-q75-noise", "pillow-progressive/640x480-ss2-q75-noise", "pillow-restart/blocks-7"}
	for _, id := range corruptSources {
		data := inputs[id]
		if data == nil {
			continue
		}
		offsets := markerOffsets(data)
		scanStart := 0
		for _, at := range offsets {
			if data[at+1] == 0xDA {
				scanStart = at + 2 + (int(data[at+2])<<8 | int(data[at+3]))
				break
			}
		}
		for seed := uint64(0); seed < 12; seed++ {
			rng := rand.New(rand.NewPCG(seed, 99))
			out := append([]byte(nil), data...)
			for n := 0; n < 1+int(seed%4); n++ {
				at := scanStart + rng.IntN(len(out)-2-scanStart)
				out[at] ^= byte(rng.IntN(255) + 1)
			}
			add("pillow-corrupt", fmt.Sprintf("%s@seed-%d", id, seed), out)
		}
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].class+cases[i].name < cases[j].class+cases[j].name })
	return cases
}

func TestOracleDecode(t *testing.T) {
	tallies := map[string]*tally{}
	var cases []oracleCase
	cases = append(cases, pillowCases()...)
	for _, c := range goCases() {
		cases = append(cases, oracleCase{class: c.class, name: c.name, input: c.build()})
	}
	inputs, _ := runDecode(t, cases, tallies)
	runDecode(t, mutationCases(inputs), tallies)
	report(t, tallies)
	for _, c := range goCases() {
		if c.class != "go-large" {
			continue
		}
		data := c.build()
		started := time.Now()
		op, err := Open(data)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := op.Load(); err != nil {
			t.Fatal(err)
		}
		w, h := op.Size()
		t.Logf("decode of %s (%dx%d, %d bytes) took %s", c.name, w, h, len(data), time.Since(started).Round(time.Millisecond))
	}
}

// transpose is Image.transpose for the methods exif_transpose uses.
func transpose(pix []byte, w, h, bpp, orientation int) ([]byte, int, int) {
	outW, outH := w, h
	if orientation >= 5 {
		outW, outH = h, w
	}
	out := make([]byte, len(pix))
	for y := 0; y < outH; y++ {
		for x := 0; x < outW; x++ {
			var sx, sy int
			switch orientation {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			default:
				sx, sy = x, y
			}
			copy(out[(y*outW+x)*bpp:(y*outW+x+1)*bpp], pix[(sy*w+sx)*bpp:(sy*w+sx+1)*bpp])
		}
	}
	return out, outW, outH
}

// TestOracleSanitizePixels checks that decoded RGB and L JPEGs, transposed
// by their EXIF orientation, are exactly what sanitize_image hands to the
// encoder.
func TestOracleSanitizePixels(t *testing.T) {
	var requests []pyoracle.Request
	type meta struct{ orientation int }
	var metas []meta
	for i, mode := range []string{"RGB", "L"} {
		for o := 1; o <= 8; o++ {
			for j, save := range []map[string]any{{"subsampling": 2}, {"subsampling": 1, "progressive": true}} {
				g := pillowGen(mode, 45, 26, 600+i*100+o*10+j, "noise", save)
				g["exif_orientation"] = o
				requests = append(requests, pyoracle.Request{ID: fmt.Sprintf("%s-o%d-%d", mode, o, j), Op: "sanitize", Gen: g, Want: []string{"pixels", "input"}})
				metas = append(metas, meta{orientation: o})
			}
		}
	}
	responses := pyoracle.Run(t, requests)
	agree := 0
	for i, r := range responses {
		op, err := Open(r.InputBytes)
		if err != nil {
			t.Errorf("%s: open: %v", r.ID, err)
			continue
		}
		img, err := op.Load()
		if err != nil {
			t.Errorf("%s: load: %v", r.ID, err)
			continue
		}
		pix := img.Pix
		if img.Mode == "L" {
			rgb := make([]byte, 0, len(pix)*3)
			for _, v := range pix {
				rgb = append(rgb, v, v, v)
			}
			pix = rgb
		}
		got, w, h := transpose(pix, img.Width, img.Height, 3, metas[i].orientation)
		if r.PixelsMode != "RGB" || w != r.PixelsW || h != r.PixelsH || !bytes.Equal(got, r.Pixels) {
			diff, maxDiff := 0, 0
			if len(got) == len(r.Pixels) {
				diff, maxDiff = sampleDiff(got, r.Pixels)
			}
			t.Errorf("%s: sanitizer pixels %s %dx%d differ from Go %dx%d (%d samples, max %d)", r.ID, r.PixelsMode, r.PixelsW, r.PixelsH, w, h, diff, maxDiff)
			continue
		}
		agree++
	}
	t.Logf("sanitize pixels: %d of %d cases identical", agree, len(responses))
}

// TestOracleStdlib measures Go's image/jpeg against Pillow on baseline
// inputs, to show why it is not used.
func TestOracleStdlib(t *testing.T) {
	var requests []pyoracle.Request
	for i, c := range pillowCases() {
		if c.class != "pillow-baseline" && c.class != "pillow-l" && c.class != "pillow-progressive" {
			continue
		}
		requests = append(requests, pyoracle.Request{ID: fmt.Sprintf("%d-%s/%s", i, c.class, c.name), Op: "decode", Gen: c.gen, Want: []string{"pixels", "input"}})
	}
	responses := pyoracle.Run(t, requests)
	cases, identical, samples, differing, worst := 0, 0, 0, 0, 0
	for _, r := range responses {
		img, err := jpeg.Decode(bytes.NewReader(r.InputBytes))
		if err != nil || r.Result != "ok" {
			continue
		}
		b := img.Bounds()
		var got []byte
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				switch m := img.(type) {
				case *image.Gray:
					got = append(got, m.GrayAt(x, y).Y)
				case *image.YCbCr:
					c := m.YCbCrAt(x, y)
					r8, g8, b8 := color.YCbCrToRGB(c.Y, c.Cb, c.Cr)
					got = append(got, r8, g8, b8)
				}
			}
		}
		if len(got) != len(r.Pixels) {
			continue
		}
		cases++
		diff, maxDiff := sampleDiff(got, r.Pixels)
		samples += len(got)
		differing += diff
		worst = max(worst, maxDiff)
		if diff == 0 {
			identical++
		}
	}
	t.Logf("image/jpeg vs Pillow: %d cases, %d pixel-identical, %d of %d samples differ, max abs diff %d", cases, identical, differing, samples, worst)
}
