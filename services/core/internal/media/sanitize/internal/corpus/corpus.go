// Package corpus builds sanitizer test inputs from seeds, so no image bytes
// are ever committed: a golden file stores a Spec and the digest of what
// Pillow answered, and the test rebuilds the input from the Spec.
//
// Only the tests of the sanitize packages import it.
package corpus

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"math"
	"math/rand"
)

// ErrNeedsOracle is returned for a Spec only Pillow can build (kind
// "pillow"): the oracle test asks the parity image for those bytes.
var ErrNeedsOracle = errors.New("corpus: input is built by the oracle")

// Spec describes one input. Params depend on Kind; see the build functions.
type Spec struct {
	Kind   string         `json:"kind"`
	Seed   int64          `json:"seed"`
	W      int            `json:"w,omitempty"`
	H      int            `json:"h,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// Build returns the bytes a Spec describes.
func Build(spec Spec) ([]byte, error) {
	rng := rand.New(rand.NewSource(spec.Seed))
	switch spec.Kind {
	case "png":
		var p PNGParams
		if err := decodeParams(spec.Params, &p); err != nil {
			return nil, err
		}
		return BuildPNG(rng, spec.W, spec.H, p)
	case "jpeg":
		var p JPEGParams
		if err := decodeParams(spec.Params, &p); err != nil {
			return nil, err
		}
		return BuildJPEG(rng, spec.W, spec.H, p)
	case "ppm":
		var p PPMParams
		if err := decodeParams(spec.Params, &p); err != nil {
			return nil, err
		}
		return BuildPPM(rng, spec.W, spec.H, p)
	case "gif":
		var p GIFParams
		if err := decodeParams(spec.Params, &p); err != nil {
			return nil, err
		}
		return BuildGIF(rng, spec.W, spec.H, p)
	case "bmp":
		var p BMPParams
		if err := decodeParams(spec.Params, &p); err != nil {
			return nil, err
		}
		return BuildBMP(rng, spec.W, spec.H, p)
	case "garbage":
		var p GarbageParams
		if err := decodeParams(spec.Params, &p); err != nil {
			return nil, err
		}
		return BuildGarbage(rng, p), nil
	case "pillow":
		return nil, ErrNeedsOracle
	}
	return nil, fmt.Errorf("corpus: unknown kind %q", spec.Kind)
}

func decodeParams(params map[string]any, into any) error {
	if params == nil {
		return nil
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into)
}

// Sample returns sample c of pixel (x, y) for a pattern, in 0..255.
//
//	"noise"   independent random bytes
//	"flat"    one colour per channel
//	"smooth"  gradients with a little noise
//	"photo"   low-frequency blobs, like a defocused photo
//	"edges"   hard stripes and a checkerboard, like text
type pattern struct {
	kind  string
	rng   *rand.Rand
	base  [8]int
	phase [8]float64
}

func newPattern(rng *rand.Rand, kind string) *pattern {
	p := &pattern{kind: kind, rng: rng}
	for i := range p.base {
		p.base[i] = rng.Intn(256)
		p.phase[i] = rng.Float64() * 2 * math.Pi
	}
	return p
}

func (p *pattern) sample(x, y, c int) int {
	switch p.kind {
	case "noise":
		return p.rng.Intn(256)
	case "flat":
		return p.base[c%8]
	case "photo":
		fx, fy := float64(x), float64(y)
		v := 128 + 60*math.Sin(fx/23+p.phase[c%8]) + 50*math.Cos(fy/31+p.phase[(c+3)%8]) +
			20*math.Sin((fx+fy)/7) + float64(p.rng.Intn(5)-2)
		return clamp8(int(v))
	case "edges":
		if (x/3+y/5)%2 == 0 || x%11 < 2 {
			return p.base[c%8]
		}
		return 255 - p.base[c%8]
	default: // smooth
		return clamp8(p.base[c%8] + (x*(c+3)+y*(5-c))/2 + p.rng.Intn(13) - 6)
	}
}

func clamp8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// Pixels returns w*h*channels bytes of a pattern.
func Pixels(rng *rand.Rand, kind string, w, h, channels int) []byte {
	p := newPattern(rng, kind)
	out := make([]byte, 0, w*h*channels)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < channels; c++ {
				out = append(out, byte(p.sample(x, y, c)))
			}
		}
	}
	return out
}

// GarbageParams: Len random bytes after Magic.
type GarbageParams struct {
	Len   int   `json:"len"`
	Magic []int `json:"magic,omitempty"`
	// Fill repeats Fill instead of random bytes (cheap oversize inputs).
	Fill *int `json:"fill,omitempty"`
}

// BuildGarbage returns Magic followed by Len bytes.
func BuildGarbage(rng *rand.Rand, p GarbageParams) []byte {
	out := make([]byte, 0, len(p.Magic)+p.Len)
	for _, b := range p.Magic {
		out = append(out, byte(b))
	}
	if p.Fill != nil {
		return append(out, bytes.Repeat([]byte{byte(*p.Fill)}, p.Len)...)
	}
	tail := make([]byte, p.Len)
	rng.Read(tail)
	return append(out, tail...)
}

// JPEGParams describe a baseline JPEG from Go's encoder (4:2:0 YCbCr, or
// gray) with optional APP1 segments after SOI.
type JPEGParams struct {
	Pattern string      `json:"pattern,omitempty"`
	Gray    bool        `json:"gray,omitempty"`
	Quality int         `json:"quality,omitempty"`
	Exif    *ExifParams `json:"exif,omitempty"`
	// XMPOrientation adds an XMP APP1 packet with tiff:Orientation.
	XMPOrientation string `json:"xmp_orientation,omitempty"`
	Truncate       int    `json:"truncate,omitempty"`
}

// BuildJPEG encodes pixels with image/jpeg and splices APP1 segments.
func BuildJPEG(rng *rand.Rand, w, h int, p JPEGParams) ([]byte, error) {
	quality := p.Quality
	if quality == 0 {
		quality = 90
	}
	var img image.Image
	if p.Gray {
		g := image.NewGray(image.Rect(0, 0, w, h))
		copy(g.Pix, Pixels(rng, p.Pattern, w, h, 1))
		img = g
	} else {
		rgba := image.NewNRGBA(image.Rect(0, 0, w, h))
		pix := Pixels(rng, p.Pattern, w, h, 3)
		for i := 0; i < w*h; i++ {
			copy(rgba.Pix[4*i:4*i+3], pix[3*i:3*i+3])
			rgba.Pix[4*i+3] = 255
		}
		img = rgba
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	var segments []byte
	if p.Exif != nil {
		payload := append([]byte("Exif\x00\x00"), BuildExif(*p.Exif)...)
		segments = append(segments, app1(payload)...)
	}
	if p.XMPOrientation != "" {
		xmp := append([]byte("http://ns.adobe.com/xap/1.0/\x00"), xmpPacket(p.XMPOrientation)...)
		segments = append(segments, app1(xmp)...)
	}
	out := append(append(append([]byte{}, data[:2]...), segments...), data[2:]...)
	return truncate(out, p.Truncate), nil
}

func app1(payload []byte) []byte {
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

func xmpPacket(orientation string) []byte {
	return []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description xmlns:tiff="http://ns.adobe.com/tiff/1.0/" tiff:Orientation="` + orientation +
		`"/></rdf:RDF></x:xmpmeta>`)
}

func truncate(data []byte, n int) []byte {
	if n > 0 && n < len(data) {
		return data[:n]
	}
	return data
}

// GIFParams describe a GIF from Go's encoder.
type GIFParams struct {
	Pattern     string `json:"pattern,omitempty"`
	Colors      int    `json:"colors,omitempty"`
	Transparent bool   `json:"transparent,omitempty"`
	Truncate    int    `json:"truncate,omitempty"`
}

// BuildGIF encodes a paletted image; with Transparent, palette entry 0 has
// alpha 0 so the encoder writes a transparent index.
func BuildGIF(rng *rand.Rand, w, h int, p GIFParams) ([]byte, error) {
	colors := p.Colors
	if colors < 2 || colors > 256 {
		colors = 16
	}
	palette := make(color.Palette, colors)
	for i := range palette {
		palette[i] = color.RGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), 255}
	}
	if p.Transparent {
		palette[0] = color.RGBA{}
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	for i, v := range Pixels(rng, p.Pattern, w, h, 1) {
		img.Pix[i] = v
		if colors < 256 {
			img.Pix[i] = v % uint8(colors)
		}
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, &gif.Options{NumColors: colors}); err != nil {
		return nil, err
	}
	return truncate(buf.Bytes(), p.Truncate), nil
}

// BMPParams describe an uncompressed BMP with a BITMAPINFOHEADER.
type BMPParams struct {
	Pattern  string `json:"pattern,omitempty"`
	Bits     int    `json:"bits,omitempty"` // 1, 4, 8, 24 or 32
	TopDown  bool   `json:"top_down,omitempty"`
	Truncate int    `json:"truncate,omitempty"`
}

// BuildBMP writes BITMAPFILEHEADER + BITMAPINFOHEADER + palette + rows.
func BuildBMP(rng *rand.Rand, w, h int, p BMPParams) ([]byte, error) {
	bits := p.Bits
	if bits == 0 {
		bits = 24
	}
	colors := 0
	if bits <= 8 {
		colors = 1 << bits
	}
	stride := (w*bits + 31) / 32 * 4
	dataOffset := 14 + 40 + 4*colors
	size := dataOffset + stride*h
	out := make([]byte, 0, size)
	le := binary.LittleEndian
	out = append(out, 'B', 'M')
	out = le.AppendUint32(out, uint32(size))
	out = le.AppendUint32(out, 0)
	out = le.AppendUint32(out, uint32(dataOffset))
	out = le.AppendUint32(out, 40)
	out = le.AppendUint32(out, uint32(int32(w)))
	height := int32(h)
	if p.TopDown {
		height = -height
	}
	out = le.AppendUint32(out, uint32(height))
	out = le.AppendUint16(out, 1)
	out = le.AppendUint16(out, uint16(bits))
	out = le.AppendUint32(out, 0) // BI_RGB
	out = le.AppendUint32(out, uint32(stride*h))
	out = le.AppendUint32(out, 2835)
	out = le.AppendUint32(out, 2835)
	out = le.AppendUint32(out, uint32(colors))
	out = le.AppendUint32(out, 0)
	for i := 0; i < colors; i++ {
		out = append(out, byte(rng.Intn(256)), byte(rng.Intn(256)), byte(rng.Intn(256)), 0)
	}
	channels := 1
	if bits >= 24 {
		channels = bits / 8
	}
	pix := Pixels(rng, p.Pattern, w, h, channels)
	row := make([]byte, stride)
	for r := 0; r < h; r++ {
		for i := range row {
			row[i] = 0
		}
		for x := 0; x < w; x++ {
			src := pix[(r*w+x)*channels : (r*w+x+1)*channels]
			switch bits {
			case 24, 32:
				copy(row[x*channels:], src)
			case 8:
				row[x] = src[0]
			default:
				v := src[0] % byte(colors)
				bitPos := x * bits
				shift := 8 - bits - bitPos%8
				row[bitPos/8] |= v << shift
			}
		}
		out = append(out, row...)
	}
	return truncate(out, p.Truncate), nil
}
