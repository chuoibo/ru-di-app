package bmpdec

import (
	"runtime"
	"testing"
)

// A delta escape may ask to skip far past the image; the skip stops at the
// image's end (audit 2026-10-05, CODEC-01). Before, a 20000-pixel row with a
// delta of 255 rows asked for 5.1 MB here, and 12.75 GB at the 50-megapixel
// limit. The image still decodes: the bytes past the end were never used.
func TestRLEDeltaPastTheEndAllocatesNothingExtra(t *testing.T) {
	const w = 20000
	palette := make([]byte, 256*4)
	for i := range 256 {
		palette[i*4], palette[i*4+1], palette[i*4+2] = byte(i), byte(i), byte(i)
	}
	pixels := []byte{
		0x02, 0x07, // two pixels of index 7
		0x00, 0x02, 0x00, 0xFF, // delta: right 0, up 255 rows
		0x00, 0x01, // end of bitmap
	}
	data := bmpSpec{headerSize: 40, width: w, height: 1, bits: 8, compression: compRLE8, colors: 256, palette: palette, pixels: pixels}.bytes()
	o, err := Plugin.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	img, err := o.Load()
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != w || img.Height != 1 || img.Pix[0] != 7 || img.Pix[1] != 7 || img.Pix[2] != 0 {
		t.Fatalf("decoded %dx%d %v", img.Width, img.Height, img.Pix[:3])
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 1<<20 {
		t.Fatalf("loading a %d-byte image allocated %d bytes", w, grown)
	}
}
