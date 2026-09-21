//go:build oracle

package pngdec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type tally struct{ cases, decodeSame, pixelsSame int }

// TestOracle decodes every corpus case with Pillow and with this package,
// then replays the sanitizer up to the encoder on both sides.
func TestOracle(t *testing.T) {
	cases := corpus()
	var requests []pyoracle.Request
	for _, c := range cases {
		requests = append(requests,
			pyoracle.Request{ID: c.name + "#decode", Op: "decode", Input: c.data},
			pyoracle.Request{ID: c.name + "#pixels", Op: "sanitize", Input: c.data, Want: []string{"pixels"}},
		)
	}
	answers := pyoracle.Run(t, requests)
	tallies := map[string]*tally{}
	golden := map[string]string{}
	failures := 0
	for i, c := range cases {
		dec, pix := answers[2*i], answers[2*i+1]
		tl := tallies[c.class]
		if tl == nil {
			tl = &tally{}
			tallies[c.class] = tl
		}
		tl.cases++

		// decode: open + load
		got := goDecode(c.data)
		want := "ok"
		if dec.Result == "error" {
			want = pyClass(dec.Type)
		}
		decodeProblem := ""
		switch {
		case want == "ok" && errors.Is(got.err, errDeferred):
			// Pillow loads it and raises later, in getexif or convert:
			// the pixels comparison below checks that outcome.
		case got.class != want:
			decodeProblem = fmt.Sprintf("class go=%s (%s) python=%s (%s: %s)", got.class, got.detail, want, dec.Type, dec.Message)
		case want == "ok":
			img := got.img
			if img.Mode != dec.LoadedMode || img.Width != dec.LoadedW || img.Height != dec.LoadedH {
				decodeProblem = fmt.Sprintf("mode/size go=%s %dx%d python=%s %dx%d", img.Mode, img.Width, img.Height, dec.LoadedMode, dec.LoadedW, dec.LoadedH)
			} else if s := sha(pt.ToBytes(img)); s != dec.PixelsSHA {
				decodeProblem = "pixels differ"
			} else if dec.PaletteMode != "" {
				p := img.Palette
				p = p[:len(p)/3*3]
				if sha(p) != sha(dec.Palette) {
					decodeProblem = fmt.Sprintf("palette differs (go %d bytes, python %d)", len(p), len(dec.Palette))
				}
			}
			if decodeProblem == "" {
				decodeProblem = compareTransparency(img, dec.Info)
			}
		}
		if decodeProblem == "" {
			tl.decodeSame++
		} else {
			failures++
			t.Errorf("%s decode: %s", c.name, decodeProblem)
		}

		// sanitize pixels: decode + exif_transpose + convert
		outcome, mode, pixels := goPixels(c.data)
		wantPix := pyClass(pix.PixelsError)
		pixProblem := ""
		switch {
		case outcome.class != wantPix:
			pixProblem = fmt.Sprintf("class go=%s (%s) python=%s", outcome.class, outcome.detail, pix.PixelsError)
		case wantPix == "ok":
			if mode != pix.PixelsMode || outcome.img.Width != pix.PixelsW || outcome.img.Height != pix.PixelsH {
				pixProblem = fmt.Sprintf("go %s %dx%d python %s %dx%d", mode, outcome.img.Width, outcome.img.Height, pix.PixelsMode, pix.PixelsW, pix.PixelsH)
			} else if sha(pixels) != pix.PixelsSHA {
				pixProblem = "encoder pixels differ"
			}
		}
		if pixProblem == "" {
			tl.pixelsSame++
			if wantPix == "ok" {
				golden[c.name] = pix.PixelsMode + ":" + pix.PixelsSHA[:16]
			} else {
				golden[c.name] = wantPix
			}
		} else {
			failures++
			t.Errorf("%s pixels: %s", c.name, pixProblem)
		}
	}
	classes := make([]string, 0, len(tallies))
	for class := range tallies {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	total := tally{}
	for _, class := range classes {
		tl := tallies[class]
		t.Logf("class %-8s cases %4d  decode identical %4d  sanitizer pixels identical %4d", class, tl.cases, tl.decodeSame, tl.pixelsSame)
		total.cases += tl.cases
		total.decodeSame += tl.decodeSame
		total.pixelsSame += tl.pixelsSame
	}
	t.Logf("total    cases %4d  decode identical %4d  sanitizer pixels identical %4d", total.cases, total.decodeSame, total.pixelsSame)
	if failures == 0 && os.Getenv("MEDIA_GOLDEN_UPDATE") == "1" {
		writeGolden(t, golden)
	}
}

// compareTransparency checks info["transparency"] against the oracle's
// info summary.
func compareTransparency(img *pil.Image, info json.RawMessage) string {
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(info, &summary); err != nil {
		return "info summary: " + err.Error()
	}
	raw, ok := summary["transparency"]
	tr := img.Info.Transparency
	if !ok {
		if tr != nil {
			return "go has transparency, Pillow does not"
		}
		return ""
	}
	if tr == nil {
		if strings.HasPrefix(string(raw), `{"str"`) && img.Mode != "P" && img.Mode != "L" {
			return "" // a text-chunk str no conversion of this mode reads
		}
		return "Pillow has transparency " + string(raw) + ", go does not"
	}
	var value struct {
		Bytes []byte `json:"bytes"`
		Repr  string `json:"repr"`
		Tuple []int  `json:"tuple"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return err.Error()
	}
	switch tr.Kind {
	case pil.TransparencyInt:
		if value.Repr != fmt.Sprint(tr.Int) {
			return fmt.Sprintf("transparency go=%d python=%s", tr.Int, raw)
		}
	case pil.TransparencyBytes:
		if value.Bytes == nil && string(raw) != `{"bytes": ""}` || sha(value.Bytes) != sha(tr.Bytes) {
			return fmt.Sprintf("transparency bytes differ: python %s", raw)
		}
	case pil.TransparencyRGB:
		if len(value.Tuple) != 3 || value.Tuple[0] != tr.RGB[0] || value.Tuple[1] != tr.RGB[1] || value.Tuple[2] != tr.RGB[2] {
			return fmt.Sprintf("transparency go=%v python=%s", tr.RGB, raw)
		}
	}
	return ""
}
