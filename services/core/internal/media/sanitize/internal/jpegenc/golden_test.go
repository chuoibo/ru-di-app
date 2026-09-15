package jpegenc

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// TestGolden replays testdata/golden.json: seeds and parameters with the
// digest and size Pillow wrote for them (or the exception it raised), as
// recorded by TestOracleEncodeRGB.
func TestGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []struct {
			genCase
			SHA16 string `json:"sha16"`
			Size  int    `json:"size"`
			Error string `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) == 0 {
		t.Fatal("empty golden")
	}
	for _, c := range golden.Cases {
		got, err := EncodeRGB(c.W, c.H, c.pixels())
		if c.Error != "" {
			var encodeErr *EncodeError
			if !errors.As(err, &encodeErr) || encodeErr.PyType+": "+encodeErr.PyMessage != c.Error {
				t.Errorf("%s: got %v, want %s", c.name(), describeGoErr(got, err), c.Error)
			}
			continue
		}
		if err != nil || len(got) != c.Size || sha16(got) != c.SHA16 {
			t.Errorf("%s: got %s, want %d bytes %s", c.name(), describeGoErr(got, err), c.Size, c.SHA16)
		}
	}
}

func describeGoErr(got []byte, err error) string {
	if err != nil {
		return err.Error()
	}
	return sha16(got)
}

// TestGeneralEncoderDecodes checks that the test-input encoder writes files
// Go's decoder accepts, for the layouts other packages build inputs with.
func TestGeneralEncoderDecodes(t *testing.T) {
	for _, opts := range []Options{
		{Width: 13, Height: 7, Color: Gray, Quality: 75},
		{Width: 13, Height: 7, Color: Gray, Quality: 90, Optimize: true, RestartInterval: 3},
		{Width: 21, Height: 19, Color: YCbCr, Quality: 95, Sampling: []Sampling{{1, 1}, {1, 1}, {1, 1}}},
		{Width: 21, Height: 19, Color: YCbCr, Quality: 60, Sampling: []Sampling{{2, 1}, {1, 1}, {1, 1}}, RestartInterval: 2},
		{Width: 21, Height: 19, Color: YCbCr, Quality: 60, Sampling: []Sampling{{1, 2}, {1, 1}, {1, 1}}},
		{Width: 37, Height: 29, Color: YCbCr, Quality: 50, Sampling: []Sampling{{3, 1}, {1, 1}, {1, 1}}},
		{Width: 37, Height: 29, Color: YCbCr, Quality: 50, Sampling: []Sampling{{4, 1}, {2, 1}, {1, 1}}, Optimize: true},
		{Width: 11, Height: 17, Color: CMYK, Quality: 80, InvertCMYK: true},
		{Width: 9, Height: 9, Color: YCbCr, Quality: 88, Segments: []Segment{{Marker: 0xE1, Data: []byte("Exif\x00\x00MM\x00\x2a")}, {Marker: 0xFE, Data: []byte("comment")}}},
	} {
		n := map[ColorSpace]int{Gray: 1, YCbCr: 3, CMYK: 4}[opts.Color]
		pix := make([]byte, opts.Width*opts.Height*n)
		rng := &splitmix{state: uint64(len(pix))}
		for i := range pix {
			pix[i] = byte(rng.next() >> 60 * 17)
		}
		out, err := Encode(opts, pix)
		if err != nil {
			t.Fatalf("%+v: %v", opts, err)
		}
		if err := decodeCheck(out, opts); err != nil {
			t.Errorf("%+v: %v", opts, err)
		}
	}
	if _, err := Encode(Options{Width: 8, Height: 8, Color: YCbCr, Quality: 50,
		Sampling: []Sampling{{4, 4}, {1, 1}, {1, 1}}}, make([]byte, 8*8*3)); !IsEncodeError(err) {
		t.Errorf("16 blocks in an MCU: got %v, want JERR_BAD_MCU_SIZE", err)
	}
}
