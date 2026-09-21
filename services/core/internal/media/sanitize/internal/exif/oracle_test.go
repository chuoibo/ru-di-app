//go:build oracle

package exif_test

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/exif"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

// TestOracle replays the sanitizer up to the encoder (decode,
// exif_transpose, convert) on every EXIF case, with Pillow and in Go.
func TestOracle(t *testing.T) {
	cases := corpus()
	var requests []pyoracle.Request
	for _, c := range cases {
		requests = append(requests, pyoracle.Request{ID: c.name, Op: "sanitize", Input: c.data, Want: []string{"pixels"}})
	}
	answers := pyoracle.Run(t, requests)
	type tally struct{ cases, same, pyErrors int }
	tallies := map[string]*tally{}
	golden := map[string]string{}
	failures := 0
	for i, c := range cases {
		pix := answers[i]
		tl := tallies[c.class]
		if tl == nil {
			tl = &tally{}
			tallies[c.class] = tl
		}
		tl.cases++
		outcome, mode, pixels := goPixels(c.data)
		want := pyClass(pix.PixelsError)
		if want != "ok" {
			tl.pyErrors++
		}
		problem := ""
		switch {
		case outcome.class != want:
			problem = fmt.Sprintf("class go=%s (%s) python=%s", outcome.class, outcome.detail, pix.PixelsError)
		case want == "ok":
			if mode != pix.PixelsMode || outcome.img.Width != pix.PixelsW || outcome.img.Height != pix.PixelsH {
				problem = fmt.Sprintf("go %s %dx%d python %s %dx%d", mode, outcome.img.Width, outcome.img.Height, pix.PixelsMode, pix.PixelsW, pix.PixelsH)
			} else if sha(pixels) != pix.PixelsSHA {
				problem = "encoder pixels differ"
			}
		}
		if problem != "" {
			failures++
			t.Errorf("%s: %s", c.name, problem)
			continue
		}
		tl.same++
		if want == "ok" {
			golden[c.name] = pix.PixelsMode + ":" + pix.PixelsSHA[:16]
		} else {
			golden[c.name] = want
		}
	}
	classes := make([]string, 0, len(tallies))
	for class := range tallies {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	total := 0
	same := 0
	for _, class := range classes {
		tl := tallies[class]
		t.Logf("class %-12s cases %3d  identical %3d  (Pillow refused %d)", class, tl.cases, tl.same, tl.pyErrors)
		total += tl.cases
		same += tl.same
	}
	t.Logf("total cases %d identical %d", total, same)
	if failures == 0 && os.Getenv("MEDIA_GOLDEN_UPDATE") == "1" {
		writeGolden(t, golden)
	}
}

// TestOracleTIFFSource checks getexif through tag_v2 on TIFF files. Pillow's
// TiffImageFile.load_end runs exif_transpose(in_place=True) and deletes the
// orientation tag, so the check compares the loaded pixels and size: Go
// reads the orientation from IFD0 of the same bytes and transposes the 3x2
// strip itself.
func TestOracleTIFFSource(t *testing.T) {
	cases := tiffCorpus()
	var requests []pyoracle.Request
	for _, c := range cases {
		requests = append(requests, pyoracle.Request{ID: c.name, Op: "decode", Input: c.data})
	}
	answers := pyoracle.Run(t, requests)
	same := 0
	for i, c := range cases {
		a := answers[i]
		src := &pil.TIFFSource{File: c.data, Offset: 8, BigEndian: c.data[0] == 'M'}
		method, err := exif.Orientation(pil.Info{TIFF: src})
		if a.Result != "ok" || a.Format != "TIFF" {
			if err != nil {
				same++
				continue
			}
			t.Errorf("%s: Pillow did not load the TIFF: %s %s: %s", c.name, a.Format, a.Type, a.Message)
			continue
		}
		if err != nil {
			t.Errorf("%s: go getexif raised %v, Pillow loaded", c.name, err)
			continue
		}
		strip := &pil.Image{Mode: "L", Width: 3, Height: 2, Pix: []byte{10, 20, 30, 40, 50, 60}}
		ops := map[int]int{2: exif.FlipLeftRight, 3: exif.Rotate180, 4: exif.FlipTopBottom, 5: exif.TransposeOp, 6: exif.Rotate270, 7: exif.Transverse, 8: exif.Rotate90}
		if op, ok := ops[method]; ok {
			strip = exif.Apply(strip, op)
		}
		if strip.Width == a.LoadedW && strip.Height == a.LoadedH && sha(strip.Pix) == a.PixelsSHA {
			same++
		} else {
			t.Errorf("%s: go method %d gives %dx%d, Pillow loaded %dx%d (pixels same: %v)", c.name, method, strip.Width, strip.Height, a.LoadedW, a.LoadedH, sha(strip.Pix) == a.PixelsSHA)
		}
	}
	t.Logf("TIFF source cases %d identical %d", len(cases), same)
}
