package jpegdec

import (
	"strings"
	"testing"
)

// A progressive file may not keep adding scans (audit 2026-10-05,
// CODEC-JPEG-SRC-01): each one walks every coefficient block again. The
// same file with a normal script still decodes.
func TestTooManyProgressiveScansAreRefused(t *testing.T) {
	script := func(n int) []scanSpec {
		scans := simpleProgression(1)
		for len(scans) < n {
			scans = append(scans, scanSpec{comps: []int{0}, ss: 1, se: 63, ah: 1, al: 0})
		}
		return scans
	}
	load := func(n int) error {
		f := garbageFile{sof: 0xC2, comps: []wComp{{id: 1, h: 1, v: 1, tq: 0}}, w: 16, h: 16, scans: script(n), seed: 5, perScan: 4, dht: true}
		o, err := Open(f.build())
		if err != nil {
			return err
		}
		_, err = o.Load()
		return err
	}
	if err := load(MaxScans); err != nil && strings.Contains(err.Error(), "TOO_MANY_SCANS") {
		t.Fatalf("%d scans were refused: %v", MaxScans, err)
	}
	err := load(MaxScans + 1)
	if err == nil || !strings.Contains(err.Error(), "TOO_MANY_SCANS") {
		t.Fatalf("%d scans: %v", MaxScans+1, err)
	}
}
