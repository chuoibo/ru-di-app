package runner

// Image files for body_parts (scenario.Image says what each field means and
// why the files are generated rather than committed).
//
// Everything here is a pure function of the spec. JPEG, PNG and GIF come from
// Go's standard encoders; BMP and WebP are written by hand because the
// standard library has no encoder for them, and each writer emits the
// smallest valid form of its format: an uncompressed 24-bit BMP, and a
// lossless WebP (VP8L) with no transforms, no colour cache and one fixed
// prefix code in which every byte value is 8 bits long.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/bits"

	"mobile/parity/internal/scenario"
)

// jpegQuality is what the generated JPEGs are encoded at.
const jpegQuality = 90

// paletteSize is the number of colours a palette picture uses.
const paletteSize = 16

// GenerateImage returns the file a spec describes. One spec, one byte string.
func GenerateImage(spec scenario.Image) ([]byte, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	var (
		data []byte
		err  error
	)
	switch spec.Format {
	case "jpeg":
		data, err = encodeJPEG(spec)
	case "png":
		data, err = encodePNG(spec)
	case "gif":
		data, err = encodeGIF(spec)
	case "webp":
		data = encodeWebP(spec)
	case "bmp":
		data = encodeBMP(spec)
	}
	if err != nil {
		return nil, fmt.Errorf("image %s: %w", spec.Format, err)
	}
	if (spec.HeaderWidth > 0 || spec.HeaderHeight > 0) && spec.Format != "bmp" {
		if data, err = claimDimensions(spec, data); err != nil {
			return nil, err
		}
	}
	if spec.TruncateAt > 0 {
		if spec.TruncateAt >= len(data) {
			return nil, fmt.Errorf("image truncate_at %d does not shorten the %d-byte %s", spec.TruncateAt, len(data), spec.Format)
		}
		data = append([]byte(nil), data[:spec.TruncateAt]...)
	}
	if spec.PadToBytes > 0 {
		if spec.PadToBytes < len(data) {
			return nil, fmt.Errorf("image pad_to_bytes %d is shorter than the %d-byte %s", spec.PadToBytes, len(data), spec.Format)
		}
		padded := make([]byte, spec.PadToBytes)
		copy(padded, data)
		data = padded
	}
	return data, nil
}

// The picture every format encodes. Red rises left to right and green top to
// bottom, so no flip or rotation maps it onto itself and every EXIF
// orientation changes what a decoder returns; blue is noise from the seed, so
// two seeds are two pictures. With alpha, opacity rises from the top-left
// pixel (fully transparent) to the bottom-right one (opaque).
func pixelAt(spec scenario.Image, x, y int) color.NRGBA {
	c := color.NRGBA{R: gradient(x, spec.Width), G: gradient(y, spec.Height), B: noise(spec.Seed, x, y), A: 255}
	if spec.Alpha {
		c.A = uint8((x + y) * 255 / max(1, spec.Width+spec.Height-2))
	}
	return c
}

func gradient(i, n int) uint8 {
	if n <= 1 {
		return 0
	}
	return uint8(i * 255 / (n - 1))
}

// noise is a small integer hash of (seed, x, y); it has no state, so a pixel's
// value does not depend on the order pixels are visited in.
func noise(seed uint32, x, y int) uint8 {
	h := seed ^ uint32(x)*0x9e3779b1 ^ uint32(y)*0x85ebca77
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return uint8(h >> 24)
}

func rgbImage(spec scenario.Image) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, spec.Width, spec.Height))
	for y := 0; y < spec.Height; y++ {
		for x := 0; x < spec.Width; x++ {
			img.SetNRGBA(x, y, pixelAt(spec, x, y))
		}
	}
	return img
}

func grayImage(spec scenario.Image) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, spec.Width, spec.Height))
	for y := 0; y < spec.Height; y++ {
		for x := 0; x < spec.Width; x++ {
			c := pixelAt(spec, x, y)
			img.SetGray(x, y, color.Gray{Y: uint8((int(c.R) + int(c.G) + int(c.B)) / 3)})
		}
	}
	return img
}

// palettedImage uses 16 seeded colours in a 4x4 grid of blocks; with alpha,
// colour 0 (the top-left block) is fully transparent.
func palettedImage(spec scenario.Image) *image.Paletted {
	palette := make(color.Palette, paletteSize)
	for i := range palette {
		c := color.NRGBA{R: noise(spec.Seed, i, 1<<16), G: noise(spec.Seed, i, 2<<16), B: noise(spec.Seed, i, 3<<16), A: 255}
		if spec.Alpha && i == 0 {
			c = color.NRGBA{}
		}
		palette[i] = c
	}
	img := image.NewPaletted(image.Rect(0, 0, spec.Width, spec.Height), palette)
	for y := 0; y < spec.Height; y++ {
		for x := 0; x < spec.Width; x++ {
			img.SetColorIndex(x, y, uint8(x*4/spec.Width+4*(y*4/spec.Height)))
		}
	}
	return img
}

func encodeJPEG(spec scenario.Image) ([]byte, error) {
	var img image.Image = rgbImage(spec)
	if spec.EffectiveColor() == "gray" {
		img = grayImage(spec)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if spec.Orientation == 0 {
		return data, nil
	}
	// An APP1 Exif segment straight after SOI, where a camera writes it.
	tiff := exifOrientation(spec.Orientation)
	segment := binary.BigEndian.AppendUint16([]byte{0xff, 0xe1}, uint16(2+6+len(tiff)))
	segment = append(segment, "Exif\x00\x00"...)
	segment = append(segment, tiff...)
	out := append([]byte(nil), data[:2]...)
	out = append(out, segment...)
	return append(out, data[2:]...), nil
}

func encodePNG(spec scenario.Image) ([]byte, error) {
	var img image.Image
	switch spec.EffectiveColor() {
	case "gray":
		img = grayImage(spec)
	case "palette":
		img = palettedImage(spec)
	default:
		// Go writes an opaque NRGBA as 8-bit RGB and a translucent one as RGBA.
		img = rgbImage(spec)
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if spec.Orientation == 0 {
		return data, nil
	}
	// eXIf straight after IHDR: 8 signature bytes, then 25 bytes of IHDR.
	const afterIHDR = 8 + 25
	out := append([]byte(nil), data[:afterIHDR]...)
	out = append(out, pngChunk("eXIf", exifOrientation(spec.Orientation))...)
	return append(out, data[afterIHDR:]...), nil
}

func encodeGIF(spec scenario.Image) ([]byte, error) {
	var buf bytes.Buffer
	// A palette image of 16 colours is written with that palette; the first
	// fully transparent colour becomes the transparent index.
	if err := gif.Encode(&buf, palettedImage(spec), nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeBMP writes BITMAPFILEHEADER + BITMAPINFOHEADER and 24-bit BGR rows,
// bottom row first, each padded to four bytes. The header carries the claimed
// dimensions; the rows are the encoded ones.
func encodeBMP(spec scenario.Image) []byte {
	width, height := claimedSize(spec)
	row := (3*spec.Width + 3) &^ 3
	size := row * spec.Height
	le := binary.LittleEndian
	out := make([]byte, 54, 54+size)
	out[0], out[1] = 'B', 'M'
	le.PutUint32(out[2:], uint32(54+size))
	le.PutUint32(out[10:], 54) // pixel data offset
	le.PutUint32(out[14:], 40) // BITMAPINFOHEADER
	le.PutUint32(out[18:], uint32(width))
	le.PutUint32(out[22:], uint32(height)) // positive: bottom-up
	le.PutUint16(out[26:], 1)              // planes
	le.PutUint16(out[28:], 24)             // bits per pixel, BI_RGB
	le.PutUint32(out[34:], uint32(size))
	le.PutUint32(out[38:], 2835) // 72 dpi
	le.PutUint32(out[42:], 2835)
	for y := spec.Height - 1; y >= 0; y-- {
		start := len(out)
		for x := 0; x < spec.Width; x++ {
			c := pixelAt(spec, x, y)
			out = append(out, c.B, c.G, c.R)
		}
		for len(out)-start < row {
			out = append(out, 0)
		}
	}
	return out
}

// encodeWebP writes a lossless WebP. The VP8L bitstream (RFC 9649 §3) is the
// header, then one ARGB image with no transform, no colour cache and no meta
// prefix codes, so a single group of five prefix codes covers every pixel:
// green, red, blue and alpha each give all 256 byte values 8-bit codes, and the
// distance code, never used, is a one-symbol simple code. Every pixel is then
// four 8-bit literals, green first. With an orientation the file is the
// extended format: VP8X, VP8L, EXIF.
func encodeWebP(spec scenario.Image) []byte {
	w := &bitWriter{}
	w.write(0x2f, 8) // signature
	w.write(uint64(spec.Width-1), 14)
	w.write(uint64(spec.Height-1), 14)
	alpha := uint64(0)
	if spec.Alpha {
		alpha = 1
	}
	w.write(alpha, 1)        // alpha_is_used
	w.write(0, 3)            // version
	w.write(0, 1)            // no transform
	w.write(0, 1)            // no colour cache
	w.write(0, 1)            // no meta prefix codes
	w.eightBitCode(256 + 24) // green, and the 24 length prefixes no pixel uses
	w.eightBitCode(256)      // red
	w.eightBitCode(256)      // blue
	w.eightBitCode(256)      // alpha
	w.write(1, 1)            // distance: simple code,
	w.write(0, 1)            // one symbol,
	w.write(0, 1)            // written in 1 bit:
	w.write(0, 1)            // symbol 0
	for y := 0; y < spec.Height; y++ {
		for x := 0; x < spec.Width; x++ {
			c := pixelAt(spec, x, y)
			for _, v := range [4]uint8{c.G, c.R, c.B, c.A} {
				// Prefix codes are packed most significant bit first into a
				// least-significant-first stream: the code of v is v, reversed.
				w.write(uint64(bits.Reverse8(v)), 8)
			}
		}
	}
	stream := w.bytes()

	var chunks []byte
	if spec.Orientation > 0 {
		vp8x := make([]byte, 10)
		vp8x[0] = 0x08 // EXIF
		if spec.Alpha {
			vp8x[0] |= 0x10 // ALPHA
		}
		putUint24(vp8x[4:], spec.Width-1)
		putUint24(vp8x[7:], spec.Height-1)
		chunks = riffChunk(chunks, "VP8X", vp8x)
		chunks = riffChunk(chunks, "VP8L", stream)
		chunks = riffChunk(chunks, "EXIF", exifOrientation(spec.Orientation))
	} else {
		chunks = riffChunk(chunks, "VP8L", stream)
	}
	out := binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(4+len(chunks)))
	out = append(out, "WEBP"...)
	return append(out, chunks...)
}

// bitWriter packs values least significant bit first, as VP8L reads them.
type bitWriter struct {
	out []byte
	acc uint64
	n   uint
}

func (w *bitWriter) write(value uint64, count uint) {
	w.acc |= value << w.n
	w.n += count
	for w.n >= 8 {
		w.out = append(w.out, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
	}
}

func (w *bitWriter) bytes() []byte {
	if w.n > 0 {
		w.out = append(w.out, byte(w.acc))
		w.acc, w.n = 0, 0
	}
	return w.out
}

// eightBitCode writes a normal prefix code over symbols in which the first 256
// have length 8 and any others length 0: a complete code under which byte
// value v is spelled as the 8 bits of v. The code lengths are themselves coded
// with a code of two symbols, length 0 and length 8, one bit each ("0" and "1",
// canonical order); twelve code-length code lengths reach symbol 8 in the
// order 17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8.
func (w *bitWriter) eightBitCode(symbols int) {
	w.write(0, 1)    // normal code
	w.write(12-4, 4) // number of code-length code lengths, minus 4
	for _, symbol := range [12]int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8} {
		if symbol == 0 || symbol == 8 {
			w.write(1, 3)
		} else {
			w.write(0, 3)
		}
	}
	w.write(0, 1) // no max_symbol: a length for every symbol follows
	for s := 0; s < symbols; s++ {
		if s < 256 {
			w.write(1, 1)
		} else {
			w.write(0, 1)
		}
	}
}

func riffChunk(dst []byte, kind string, payload []byte) []byte {
	dst = append(dst, kind...)
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(payload)))
	dst = append(dst, payload...)
	if len(payload)%2 == 1 {
		dst = append(dst, 0)
	}
	return dst
}

func putUint24(dst []byte, v int) {
	dst[0], dst[1], dst[2] = byte(v), byte(v>>8), byte(v>>16)
}

func pngChunk(kind string, payload []byte) []byte {
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
	chunk = append(chunk, kind...)
	chunk = append(chunk, payload...)
	return binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
}

// exifOrientation is a little-endian TIFF structure holding one IFD with one
// entry, Orientation (0x0112, SHORT, count 1), and no next IFD: 26 bytes.
func exifOrientation(value int) []byte {
	return []byte{
		'I', 'I', 42, 0, 8, 0, 0, 0, // header, IFD0 at offset 8
		1, 0, // one entry
		0x12, 0x01, 3, 0, 1, 0, 0, 0, byte(value), byte(value >> 8), 0, 0,
		0, 0, 0, 0, // no next IFD
	}
}

func claimedSize(spec scenario.Image) (int, int) {
	width, height := spec.Width, spec.Height
	if spec.HeaderWidth > 0 {
		width = spec.HeaderWidth
	}
	if spec.HeaderHeight > 0 {
		height = spec.HeaderHeight
	}
	return width, height
}

// claimDimensions overwrites the size a PNG, JPEG or GIF header states.
func claimDimensions(spec scenario.Image, data []byte) ([]byte, error) {
	width, height := claimedSize(spec)
	out := append([]byte(nil), data...)
	switch spec.Format {
	case "png":
		// IHDR is the first chunk: width at 16, height at 20, CRC at 29 over
		// the type and data at 12..29.
		binary.BigEndian.PutUint32(out[16:], uint32(width))
		binary.BigEndian.PutUint32(out[20:], uint32(height))
		binary.BigEndian.PutUint32(out[29:], crc32.ChecksumIEEE(out[12:29]))
	case "gif":
		// The logical screen, which is the size a decoder reports.
		binary.LittleEndian.PutUint16(out[6:], uint16(width))
		binary.LittleEndian.PutUint16(out[8:], uint16(height))
	case "jpeg":
		at, err := jpegFrameHeader(out)
		if err != nil {
			return nil, err
		}
		// FF Cn, length (2), precision (1), height (2), width (2).
		binary.BigEndian.PutUint16(out[at+5:], uint16(height))
		binary.BigEndian.PutUint16(out[at+7:], uint16(width))
	}
	return out, nil
}

// jpegFrameHeader finds the SOF segment by walking the marker segments that
// precede the scan.
func jpegFrameHeader(data []byte) (int, error) {
	for at := 2; at+9 <= len(data); {
		if data[at] != 0xff {
			return 0, errors.New("image jpeg: no marker where one was expected")
		}
		switch marker := data[at+1]; marker {
		case 0xc0, 0xc1, 0xc2:
			return at, nil
		case 0xda:
			return 0, errors.New("image jpeg: the scan starts before any frame header")
		}
		at += 2 + int(binary.BigEndian.Uint16(data[at+2:]))
	}
	return 0, errors.New("image jpeg: no frame header")
}
