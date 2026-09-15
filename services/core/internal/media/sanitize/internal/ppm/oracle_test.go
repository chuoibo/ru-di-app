//go:build oracle

package ppm

import (
	"fmt"
	"os"
	"sort"
	"testing"

	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type tally struct{ cases, decodeSame, pixelsSame int }

// TestOracle decodes every PPM corpus case with Pillow and this package and
// replays the sanitizer up to the encoder on both sides.
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

		got := goDecode(c.data)
		want := "ok"
		if dec.Result == "error" {
			want = pyClass(dec.Type)
		}
		problem := ""
		switch {
		case got.class != want:
			problem = fmt.Sprintf("class go=%s (%s) python=%s (%s: %s)", got.class, got.detail, want, dec.Type, dec.Message)
		case want == "ok":
			if got.img.Mode != dec.LoadedMode || got.img.Width != dec.LoadedW || got.img.Height != dec.LoadedH {
				problem = fmt.Sprintf("mode/size go=%s %dx%d python=%s %dx%d", got.img.Mode, got.img.Width, got.img.Height, dec.LoadedMode, dec.LoadedW, dec.LoadedH)
			} else if sha(pt.ToBytes(got.img)) != dec.PixelsSHA {
				problem = "pixels differ"
			}
		}
		if problem == "" {
			tl.decodeSame++
		} else {
			failures++
			t.Errorf("%s decode: %s", c.name, problem)
		}

		outcome, mode, pixels := goPixels(c.data)
		wantPix := pyClass(pix.PixelsError)
		problem = ""
		switch {
		case outcome.class != wantPix:
			problem = fmt.Sprintf("class go=%s (%s) python=%s", outcome.class, outcome.detail, pix.PixelsError)
		case wantPix == "ok":
			if mode != pix.PixelsMode || outcome.img.Width != pix.PixelsW || outcome.img.Height != pix.PixelsH {
				problem = fmt.Sprintf("go %s %dx%d python %s %dx%d", mode, outcome.img.Width, outcome.img.Height, pix.PixelsMode, pix.PixelsW, pix.PixelsH)
			} else if sha(pixels) != pix.PixelsSHA {
				problem = "encoder pixels differ"
			}
		}
		if problem == "" {
			tl.pixelsSame++
			if wantPix == "ok" {
				golden[c.name] = pix.PixelsMode + ":" + pix.PixelsSHA[:16]
			} else {
				golden[c.name] = wantPix
			}
		} else {
			failures++
			t.Errorf("%s pixels: %s", c.name, problem)
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
