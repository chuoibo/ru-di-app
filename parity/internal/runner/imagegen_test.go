package runner

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/bits"
	"strings"
	"testing"

	"mobile/parity/internal/scenario"
)

// These tests pin what a scenario relies on: the same spec is the same bytes,
// and the bytes say what the spec says (size, pixels, EXIF, header fields).
// Pillow in the pinned API image was also checked against every format once
// (the W6 report): WebP and BMP decode to the pixels of the PNG of the same
// spec, GIF to those of the palette PNG, orientations 5..8 swap the size.

func mustGenerate(t *testing.T, spec scenario.Image) []byte {
	t.Helper()
	data, err := GenerateImage(spec)
	if err != nil {
		t.Fatalf("%+v: %v", spec, err)
	}
	return data
}

var everyShape = []scenario.Image{
	{Format: "jpeg", Width: 16, Height: 12, Seed: 7},
	{Format: "jpeg", Width: 16, Height: 12, Seed: 7, Color: "gray", Orientation: 6},
	{Format: "png", Width: 16, Height: 12, Seed: 7},
	{Format: "png", Width: 16, Height: 12, Seed: 7, Alpha: true, Orientation: 3},
	{Format: "png", Width: 16, Height: 12, Seed: 7, Color: "gray"},
	{Format: "png", Width: 16, Height: 12, Seed: 7, Color: "palette", Alpha: true},
	{Format: "gif", Width: 16, Height: 12, Seed: 7, Alpha: true},
	{Format: "webp", Width: 16, Height: 12, Seed: 7, Alpha: true, Orientation: 8},
	{Format: "bmp", Width: 15, Height: 12, Seed: 7},
}

func TestTheSameSpecIsTheSameBytesAndTheSeedChangesThem(t *testing.T) {
	for _, spec := range everyShape {
		first, again := mustGenerate(t, spec), mustGenerate(t, spec)
		if !bytes.Equal(first, again) {
			t.Errorf("%+v: two generations differ", spec)
		}
		other := spec
		other.Seed++
		if bytes.Equal(first, mustGenerate(t, other)) {
			t.Errorf("%+v: seed %d and %d give the same bytes", spec, spec.Seed, other.Seed)
		}
	}
}

func TestPNGAndGIFCarryTheSpecsPixels(t *testing.T) {
	rgba := scenario.Image{Format: "png", Width: 9, Height: 5, Seed: 3, Alpha: true}
	decoded, err := png.Decode(bytes.NewReader(mustGenerate(t, rgba)))
	if err != nil {
		t.Fatal(err)
	}
	nrgba, ok := decoded.(*image.NRGBA)
	if !ok {
		t.Fatalf("alpha png decodes as %T, want *image.NRGBA", decoded)
	}
	for y := 0; y < rgba.Height; y++ {
		for x := 0; x < rgba.Width; x++ {
			if got, want := nrgba.NRGBAAt(x, y), pixelAt(rgba, x, y); got != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
	if nrgba.NRGBAAt(0, 0).A != 0 || nrgba.NRGBAAt(8, 4).A != 255 {
		t.Fatal("alpha must run from transparent top-left to opaque bottom-right")
	}
	opaque, err := png.Decode(bytes.NewReader(mustGenerate(t, scenario.Image{Format: "png", Width: 9, Height: 5})))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := opaque.(*image.RGBA); !ok {
		// Go's decoder returns *image.RGBA for colour type 2 (RGB, no alpha).
		t.Fatalf("opaque png decodes as %T: it was not written as RGB", opaque)
	}

	for _, format := range []string{"png", "gif"} {
		spec := scenario.Image{Format: format, Width: 8, Height: 8, Seed: 3, Color: "palette", Alpha: true}
		var img image.Image
		if format == "png" {
			img, err = png.Decode(bytes.NewReader(mustGenerate(t, spec)))
		} else {
			img, err = gif.Decode(bytes.NewReader(mustGenerate(t, spec)))
		}
		if err != nil {
			t.Fatal(err)
		}
		paletted, ok := img.(*image.Paletted)
		if !ok || len(paletted.Palette) != paletteSize {
			t.Fatalf("%s palette image decodes as %T", format, img)
		}
		if _, _, _, a := paletted.At(0, 0).RGBA(); a != 0 {
			t.Errorf("%s: the top-left block is not transparent", format)
		}
		if paletted.ColorIndexAt(7, 7) != 15 || paletted.ColorIndexAt(0, 7) != 12 {
			t.Errorf("%s: blocks are not laid out 4x4", format)
		}
	}
}

func TestBMPRowsAreBottomUpBGR(t *testing.T) {
	spec := scenario.Image{Format: "bmp", Width: 5, Height: 3, Seed: 9}
	data := mustGenerate(t, spec)
	le := binary.LittleEndian
	row := 16 // 5*3 = 15 bytes, padded to 16
	if string(data[:2]) != "BM" || int(le.Uint32(data[2:])) != len(data) || len(data) != 54+row*3 {
		t.Fatalf("header or length wrong: %d bytes", len(data))
	}
	if le.Uint32(data[18:]) != 5 || le.Uint32(data[22:]) != 3 || le.Uint16(data[28:]) != 24 {
		t.Fatal("dimensions or depth wrong")
	}
	for y := 0; y < spec.Height; y++ {
		for x := 0; x < spec.Width; x++ {
			at := 54 + (spec.Height-1-y)*row + 3*x
			want := pixelAt(spec, x, y)
			if data[at] != want.B || data[at+1] != want.G || data[at+2] != want.R {
				t.Fatalf("pixel (%d,%d) wrong", x, y)
			}
		}
	}
}

// TestWebPLayout reads the file back by the layout encodeWebP documents: the
// RIFF chunks, the VP8L header fields, the fixed prefix codes bit for bit, and
// then every pixel as four reversed 8-bit literals.
func TestWebPLayout(t *testing.T) {
	for _, spec := range []scenario.Image{
		{Format: "webp", Width: 7, Height: 3, Seed: 5},
		{Format: "webp", Width: 7, Height: 3, Seed: 5, Alpha: true, Orientation: 6},
	} {
		data := mustGenerate(t, spec)
		le := binary.LittleEndian
		if string(data[:4]) != "RIFF" || int(le.Uint32(data[4:]))+8 != len(data) || string(data[8:12]) != "WEBP" {
			t.Fatalf("%+v: RIFF header wrong", spec)
		}
		chunks := map[string][]byte{}
		var order []string
		for at := 12; at < len(data); {
			kind, size := string(data[at:at+4]), int(le.Uint32(data[at+4:]))
			chunks[kind] = data[at+8 : at+8+size]
			order = append(order, kind)
			at += 8 + size + size%2
		}
		wantOrder := "VP8L"
		if spec.Orientation > 0 {
			wantOrder = "VP8X,VP8L,EXIF"
			if flags := chunks["VP8X"][0]; flags != 0x18 {
				t.Errorf("VP8X flags %#x, want EXIF|ALPHA", flags)
			}
			if !bytes.Equal(chunks["EXIF"], exifOrientation(6)) {
				t.Error("EXIF chunk does not hold the orientation")
			}
		}
		if strings.Join(order, ",") != wantOrder {
			t.Fatalf("chunks %v, want %s", order, wantOrder)
		}
		r := &bitReader{data: chunks["VP8L"]}
		alpha := uint64(0)
		if spec.Alpha {
			alpha = 1
		}
		header := []struct {
			name  string
			count uint
			want  uint64
		}{
			{"signature", 8, 0x2f}, {"width-1", 14, uint64(spec.Width - 1)}, {"height-1", 14, uint64(spec.Height - 1)},
			{"alpha", 1, alpha}, {"version", 3, 0}, {"transform", 1, 0}, {"colour cache", 1, 0}, {"meta codes", 1, 0},
		}
		for _, field := range header {
			if got := r.read(field.count); got != field.want {
				t.Fatalf("%s = %d, want %d", field.name, got, field.want)
			}
		}
		for _, symbols := range []int{280, 256, 256, 256} {
			if r.read(1) != 0 || r.read(4) != 8 {
				t.Fatal("prefix code is not normal with 12 code-length code lengths")
			}
			for _, symbol := range []int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8} {
				want := uint64(0)
				if symbol == 0 || symbol == 8 {
					want = 1
				}
				if r.read(3) != want {
					t.Fatalf("code-length code length of %d wrong", symbol)
				}
			}
			if r.read(1) != 0 {
				t.Fatal("max_symbol written")
			}
			for s := 0; s < symbols; s++ {
				want := uint64(0)
				if s < 256 {
					want = 1
				}
				if r.read(1) != want {
					t.Fatalf("length of symbol %d wrong", s)
				}
			}
		}
		if r.read(4) != 1 {
			t.Fatal("distance code is not a one-symbol simple code for symbol 0")
		}
		for y := 0; y < spec.Height; y++ {
			for x := 0; x < spec.Width; x++ {
				var got [4]uint8
				for i := range got {
					got[i] = bits.Reverse8(uint8(r.read(8)))
				}
				want := pixelAt(spec, x, y)
				if got != [4]uint8{want.G, want.R, want.B, want.A} {
					t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
				}
			}
		}
		if rest := len(r.data)*8 - r.pos; rest >= 8 {
			t.Fatalf("%d bits left after the last pixel", rest)
		}
	}
}

type bitReader struct {
	data []byte
	pos  int
}

func (r *bitReader) read(count uint) uint64 {
	var v uint64
	for i := uint(0); i < count; i++ {
		bit := uint64(r.data[r.pos/8]>>(r.pos%8)) & 1
		v |= bit << i
		r.pos++
	}
	return v
}

func TestEXIFOrientationIsWhereDecodersLookForIt(t *testing.T) {
	jpegData := mustGenerate(t, scenario.Image{Format: "jpeg", Width: 16, Height: 12, Orientation: 8})
	if !bytes.Equal(jpegData[:4], []byte{0xff, 0xd8, 0xff, 0xe1}) || string(jpegData[6:12]) != "Exif\x00\x00" {
		t.Fatalf("jpeg does not open with SOI then APP1 Exif: % x", jpegData[:12])
	}
	if !bytes.Equal(jpegData[12:12+26], exifOrientation(8)) {
		t.Fatal("APP1 does not hold the orientation")
	}
	if config, err := jpeg.DecodeConfig(bytes.NewReader(jpegData)); err != nil || config.Width != 16 || config.Height != 12 {
		t.Fatalf("jpeg with EXIF does not decode: %+v %v", config, err)
	}
	tiff := exifOrientation(8)
	if string(tiff[:4]) != "II*\x00" || binary.LittleEndian.Uint16(tiff[10:]) != 0x0112 || binary.LittleEndian.Uint16(tiff[18:]) != 8 {
		t.Fatal("TIFF structure wrong")
	}

	pngData := mustGenerate(t, scenario.Image{Format: "png", Width: 16, Height: 12, Orientation: 6})
	if string(pngData[37:41]) != "eXIf" {
		t.Fatalf("eXIf is not the chunk after IHDR: %q", pngData[37:41])
	}
	// Go's decoder verifies the CRC of every chunk, known or not.
	if _, err := png.Decode(bytes.NewReader(pngData)); err != nil {
		t.Fatalf("png with eXIf does not decode: %v", err)
	}
}

func TestHeaderDimensionsAreClaimedWithoutReencoding(t *testing.T) {
	type decodeConfig func([]byte) (image.Config, error)
	reader := func(f func(r *bytes.Reader) (image.Config, error)) decodeConfig {
		return func(b []byte) (image.Config, error) { return f(bytes.NewReader(b)) }
	}
	for format, decode := range map[string]decodeConfig{
		"png":  reader(func(r *bytes.Reader) (image.Config, error) { return png.DecodeConfig(r) }),
		"jpeg": reader(func(r *bytes.Reader) (image.Config, error) { return jpeg.DecodeConfig(r) }),
		"gif":  reader(func(r *bytes.Reader) (image.Config, error) { return gif.DecodeConfig(r) }),
	} {
		spec := scenario.Image{Format: format, Width: 8, Height: 8, HeaderWidth: 10001, HeaderHeight: 5000}
		if format == "jpeg" {
			spec.Orientation = 1 // the frame header is found past an APP1 segment too
		}
		config, err := decode(mustGenerate(t, spec))
		if err != nil || config.Width != 10001 || config.Height != 5000 {
			t.Errorf("%s claims %dx%d (%v), want 10001x5000", format, config.Width, config.Height, err)
		}
	}
	if _, err := png.Decode(bytes.NewReader(mustGenerate(t, scenario.Image{Format: "png", Width: 8, Height: 8, HeaderHeight: 9}))); err == nil ||
		strings.Contains(err.Error(), "checksum") {
		t.Fatalf("a png claiming another height must fail on its pixels, not its IHDR checksum: %v", err)
	}
	bmp := mustGenerate(t, scenario.Image{Format: "bmp", Width: 2, Height: 2, HeaderWidth: 10001})
	if binary.LittleEndian.Uint32(bmp[18:]) != 10001 || binary.LittleEndian.Uint32(bmp[22:]) != 2 || len(bmp) != 54+2*8 {
		t.Fatal("bmp header claim wrong, or the rows followed it")
	}
}

func TestTruncateAndPad(t *testing.T) {
	spec := scenario.Image{Format: "png", Width: 8, Height: 8, Seed: 1}
	whole := mustGenerate(t, spec)
	spec.TruncateAt = 40
	if cut := mustGenerate(t, spec); !bytes.Equal(cut, whole[:40]) {
		t.Fatal("truncate_at does not keep the first bytes")
	}
	spec.TruncateAt, spec.PadToBytes = 0, len(whole)+10
	padded := mustGenerate(t, spec)
	if len(padded) != len(whole)+10 || !bytes.Equal(padded[:len(whole)], whole) || !bytes.Equal(padded[len(whole):], make([]byte, 10)) {
		t.Fatal("pad_to_bytes does not append zero bytes")
	}
	spec.TruncateAt, spec.PadToBytes = 40, 50
	if both := mustGenerate(t, spec); len(both) != 50 || !bytes.Equal(both[:40], whole[:40]) {
		t.Fatal("truncate then pad wrong")
	}
	for name, bad := range map[string]scenario.Image{
		"truncate past the end": {Format: "png", Width: 8, Height: 8, Seed: 1, TruncateAt: len(whole)},
		"pad shorter":           {Format: "png", Width: 8, Height: 8, Seed: 1, PadToBytes: len(whole) - 1},
		"invalid spec":          {Format: "tiff", Width: 8, Height: 8},
	} {
		if _, err := GenerateImage(bad); err == nil {
			t.Errorf("%s: generated", name)
		}
	}
}

func TestGrayJPEGHasOneComponent(t *testing.T) {
	img, err := jpeg.Decode(bytes.NewReader(mustGenerate(t, scenario.Image{Format: "jpeg", Width: 8, Height: 8, Color: "gray"})))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.Gray); !ok {
		t.Fatalf("gray jpeg decodes as %T", img)
	}
	if img.ColorModel() != color.GrayModel {
		t.Fatal("colour model is not gray")
	}
}
