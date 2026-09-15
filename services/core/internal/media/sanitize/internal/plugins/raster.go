package plugins

import (
	"encoding/binary"
	"math"
	"math/bits"
	"sort"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Pillow codec status codes (Imaging.h).
const (
	codecOverrun = -1
	codecUnknown = -3
	codecConfig  = -8
)

// canvas is Pillow's in-memory image (Storage.c): one byte per pixel for
// "1", "L" and "P", two for the "I;16" modes, four for everything else.
type canvas struct {
	mode string
	w, h int
	ps   int
	pix  []byte
	pal  *corePalette
}

// corePalette is ImagingPalette: 256 entries of four bytes, alpha 255.
type corePalette struct {
	mode    string
	size    int
	entries [1024]byte
}

// pyPalette is an ImagePalette made by ImagePalette.raw.
type pyPalette struct {
	mode    string
	rawmode string
	data    []byte
}

var pixelSizes = map[string]int{
	"1": 1, "L": 1, "P": 1,
	"I;16": 2, "I;16L": 2, "I;16B": 2, "I;16N": 2,
	"I": 4, "F": 4, "LA": 4, "La": 4, "PA": 4, "RGB": 4, "RGBA": 4, "RGBa": 4,
	"RGBX": 4, "CMYK": 4, "YCbCr": 4, "LAB": 4, "HSV": 4,
}

// newCanvas is Image.core.new(mode, size), zero-filled.
func newCanvas(mode string, w, h int64) (*canvas, error) {
	ps, ok := pixelSizes[mode]
	if !ok {
		return nil, raise("ValueError", "unrecognized image mode")
	}
	if w < 0 || h < 0 || w > math.MaxInt32 || h > math.MaxInt32 {
		return nil, raise("ValueError", "invalid size")
	}
	n := w * h * int64(ps)
	if n > 16*int64(pil.MaxImagePixels) {
		return nil, raise("MemoryError", "")
	}
	return &canvas{mode: mode, w: int(w), h: int(h), ps: ps, pix: make([]byte, n)}, nil
}

func (c *canvas) row(y int) []byte {
	start := y * c.w * c.ps
	return c.pix[start : start+c.w*c.ps]
}

// realizePalette is Image.load's palette step: im.putpalette(palette.mode,
// rawmode, data), which also turns an "L" core image into "P".
func realizePalette(c *canvas, p *pyPalette) error {
	switch c.mode {
	case "L", "LA", "P", "PA":
	default:
		return raise("ValueError", "illegal image mode")
	}
	if p.mode != "RGB" && p.mode != "RGBA" {
		return raise("ValueError", "illegal image mode")
	}
	u, ok := findUnpacker(p.mode, p.rawmode)
	if !ok {
		if pillowUnpackers[p.mode+"\x00"+p.rawmode] {
			return pil.Unsupported("palette", "unpacker %s from %s", p.mode, p.rawmode)
		}
		return raise("ValueError", "unrecognized raw mode")
	}
	size := len(p.data) * 8 / u.bits
	if size > 256 {
		return raise("ValueError", "invalid palette size")
	}
	pal := &corePalette{mode: p.mode, size: size}
	for i := 0; i < 256; i++ {
		pal.entries[i*4+3] = 255
	}
	u.fn(pal.entries[:], p.data, size)
	c.pal = pal
	switch c.mode {
	case "L":
		c.mode = "P"
	case "LA":
		c.mode = "PA"
	}
	return nil
}

// image turns the canvas into the sanitizer's model: im.tobytes() layout
// with multi-byte samples little-endian.
func (c *canvas) image(format string) *pil.Image {
	stride := pil.BytesPerPixel(c.mode)
	out := make([]byte, c.w*c.h*stride)
	n := c.w * c.h
	switch c.mode {
	case "1", "L", "P", "I;16", "I;16L", "I;16N", "RGBA", "RGBa", "RGBX", "CMYK", "I", "F":
		copy(out, c.pix)
	case "I;16B":
		for i := 0; i < n; i++ {
			out[2*i], out[2*i+1] = c.pix[2*i+1], c.pix[2*i]
		}
	case "LA", "La", "PA":
		for i := 0; i < n; i++ {
			out[2*i], out[2*i+1] = c.pix[4*i], c.pix[4*i+3]
		}
	default:
		for i := 0; i < n; i++ {
			copy(out[3*i:3*i+3], c.pix[4*i:4*i+3])
		}
		if c.mode == "LAB" {
			// ImagingPackLAB stores a and b offset by 128.
			for i := 0; i < n; i++ {
				out[3*i+1] ^= 128
				out[3*i+2] ^= 128
			}
		}
	}
	img := &pil.Image{Mode: c.mode, Width: c.w, Height: c.h, Pix: out, Format: format}
	if c.pal != nil {
		per := len(c.pal.mode)
		img.PaletteMode = c.pal.mode
		img.Palette = make([]byte, 0, c.pal.size*per)
		for i := 0; i < c.pal.size; i++ {
			img.Palette = append(img.Palette, c.pal.entries[i*4:i*4+per]...)
		}
	}
	return img
}

// ---------------------------------------------------------------------------
// Unpack.c

type unpacker struct {
	bits int
	fn   func(out, in []byte, pixels int)
}

// pillowUnpackers lists every (mode, rawmode) pair Unpack.c knows, so a
// pair this port lacks is told apart from one Pillow rejects.
var pillowUnpackers = func() map[string]bool {
	set := map[string]bool{}
	for _, pair := range strings.Split(pillowUnpackerPairs, "|") {
		mode, rawmode, _ := strings.Cut(pair, "/")
		set[mode+"\x00"+rawmode] = true
	}
	return set
}()

func findUnpacker(mode, rawmode string) (unpacker, bool) {
	u, ok := unpackers[mode+"\x00"+rawmode]
	return u, ok
}

func set4(out []byte, i int, a, b, c, d byte) {
	o := out[4*i : 4*i+4]
	o[0], o[1], o[2], o[3] = a, b, c, d
}

func bits1(invert, lsb bool) func(out, in []byte, n int) {
	return func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			var bit byte
			if lsb {
				bit = in[i>>3] >> (i & 7) & 1
			} else {
				bit = in[i>>3] >> (7 - i&7) & 1
			}
			if (bit == 1) != invert {
				out[i] = 255
			} else {
				out[i] = 0
			}
		}
	}
}

func nibbles(scale byte) func(out, in []byte, n int) {
	return func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			v := in[i>>1] >> (4 * (1 - i&1)) & 15
			out[i] = v * scale
		}
	}
}

func planar(planes int) func(out, in []byte, n int) {
	return func(out, in []byte, n int) {
		s := (n + 7) / 8
		for i := 0; i < n; i++ {
			m := byte(0x80) >> (i & 7)
			j := i >> 3
			var v byte
			for p := 0; p < planes; p++ {
				if in[j+p*s]&m != 0 {
					v |= 1 << p
				}
			}
			out[i] = v
		}
	}
}

func band(k int) func(out, in []byte, n int) {
	return func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			out[4*i+k] = in[i]
		}
	}
}

func copyN(width int) func(out, in []byte, n int) {
	return func(out, in []byte, n int) { copy(out[:n*width], in[:n*width]) }
}

var unpackers = map[string]unpacker{}

func init() {
	add := func(mode, rawmode string, bits int, fn func(out, in []byte, n int)) {
		unpackers[mode+"\x00"+rawmode] = unpacker{bits: bits, fn: fn}
	}
	add("1", "1", 1, bits1(false, false))
	add("1", "1;I", 1, bits1(true, false))
	add("1", "1;R", 1, bits1(false, true))
	add("L", "L", 8, copyN(1))
	add("L", "L;4", 4, nibbles(0x11))
	add("L", "L;16B", 16, func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			out[i] = in[2*i]
		}
	})
	add("P", "P", 8, copyN(1))
	add("P", "L", 8, copyN(1))
	add("P", "P;4", 4, nibbles(1))
	add("P", "P;2L", 2, planar(2))
	add("P", "P;4L", 4, planar(4))
	la := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[2*i], in[2*i], in[2*i], in[2*i+1])
		}
	}
	lal := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[i], in[i], in[i], in[i+n])
		}
	}
	add("LA", "LA", 16, la)
	add("LA", "LA;L", 16, lal)
	add("PA", "PA", 16, la)
	add("PA", "PA;L", 16, lal)
	add("PA", "LA", 16, la)
	rgb := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[3*i], in[3*i+1], in[3*i+2], 255)
		}
	}
	rgbl := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[i], in[i+n], in[i+2*n], 255)
		}
	}
	bgr := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[3*i+2], in[3*i+1], in[3*i], 255)
		}
	}
	bgrx := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[4*i+2], in[4*i+1], in[4*i], 255)
		}
	}
	bgra := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[4*i+2], in[4*i+1], in[4*i], in[4*i+3])
		}
	}
	rgbal := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[i], in[i+n], in[i+2*n], in[i+3*n])
		}
	}
	for _, m := range []string{"RGB", "RGBX"} {
		add(m, "RGB", 24, rgb)
		add(m, "RGB;L", 24, rgbl)
		add(m, "BGR", 24, bgr)
		add(m, "RGBX", 32, copyN(4))
		add(m, "BGRX", 32, bgrx)
		add(m, "R", 8, band(0))
		add(m, "G", 8, band(1))
		add(m, "B", 8, band(2))
		add(m, "RGBX;L", 32, rgbal)
		add(m, "RGB;16B", 48, func(out, in []byte, n int) {
			for i := 0; i < n; i++ {
				set4(out, i, in[6*i], in[6*i+2], in[6*i+4], 255)
			}
		})
	}
	add("RGB", "RGBA;L", 32, rgbal)
	add("RGBA", "RGBA", 32, copyN(4))
	add("RGBA", "BGR", 24, bgr)
	add("RGBA", "BGRA", 32, bgra)
	add("RGBA", "RGBA;L", 32, rgbal)
	add("RGBA", "R", 8, band(0))
	add("RGBA", "G", 8, band(1))
	add("RGBA", "B", 8, band(2))
	add("RGBA", "A", 8, band(3))
	add("RGBA", "BGRA;15Z", 16, func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			p := int(in[2*i]) | int(in[2*i+1])<<8
			o := out[4*i : 4*i+4]
			o[2] = byte((p & 31) * 255 / 31)
			o[1] = byte((p >> 5 & 31) * 255 / 31)
			o[0] = byte((p >> 10 & 31) * 255 / 31)
			o[3] = ^byte((p >> 15) * 255)
		}
	})
	add("RGBA", "RGBA;16B", 64, func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[8*i], in[8*i+2], in[8*i+4], in[8*i+6])
		}
	})
	add("CMYK", "CMYK", 32, copyN(4))
	add("CMYK", "CMYK;L", 32, rgbal)
	add("CMYK", "CMYK;I", 32, func(out, in []byte, n int) {
		for i := 0; i < 4*n; i++ {
			out[i] = ^in[i]
		}
	})
	add("YCbCr", "YCbCr;L", 24, rgbl)
	add("I;16", "I;16", 16, copyN(2))
	add("I;16B", "I;16B", 16, copyN(2))
	add("I", "I", 32, copyN(4))
	reverse4 := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			o, s := out[4*i:4*i+4], in[4*i:4*i+4]
			o[0], o[1], o[2], o[3] = s[3], s[2], s[1], s[0]
		}
	}
	add("I", "I;32B", 32, reverse4)
	add("I", "I;32", 32, copyN(4))
	add("I", "I;32S", 32, copyN(4))
	add("I;16L", "I;16L", 16, copyN(2))
	add("I;16", "I;16B", 16, func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			out[2*i], out[2*i+1] = in[2*i+1], in[2*i]
		}
	})
	perByte := func(f func(v byte) byte) func(out, in []byte, n int) {
		return func(out, in []byte, n int) {
			for i := 0; i < n; i++ {
				out[i] = f(in[i])
			}
		}
	}
	// packed samples of `bits` bits, MSB first, each byte optionally
	// bit-reversed first, mapped through f.
	packed := func(width uint, reverse bool, f func(v byte) byte) func(out, in []byte, n int) {
		return func(out, in []byte, n int) {
			per := int(8 / width)
			for i := 0; i < n; i++ {
				b := in[i/per]
				if reverse {
					b = bits.Reverse8(b)
				}
				shift := 8 - width*uint(i%per+1)
				out[i] = f(b >> shift & byte(1<<width-1))
			}
		}
	}
	add("L", "L;I", 8, perByte(func(v byte) byte { return ^v }))
	add("L", "L;R", 8, perByte(bits.Reverse8))
	add("P", "P;R", 8, perByte(bits.Reverse8))
	add("L", "L;2", 2, packed(2, false, func(v byte) byte { return v * 0x55 }))
	add("L", "L;2I", 2, packed(2, false, func(v byte) byte { return 0xFF - v*0x55 }))
	add("L", "L;2R", 2, packed(2, true, func(v byte) byte { return v * 0x55 }))
	add("L", "L;4I", 4, packed(4, false, func(v byte) byte { return 0xFF - v*0x11 }))
	add("L", "L;4R", 4, packed(4, true, func(v byte) byte { return v * 0x11 }))
	add("1", "1;IR", 1, bits1(true, true))
	add("P", "P;1", 1, packed(1, false, func(v byte) byte { return v }))
	add("P", "P;2", 2, packed(2, false, func(v byte) byte { return v }))
	add("P", "PX", 16, func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			out[i] = in[2*i]
		}
	})
	skip := func(stride int) func(out, in []byte, n int) {
		return func(out, in []byte, n int) {
			for i := 0; i < n; i++ {
				copy(out[4*i:4*i+4], in[stride*i:stride*i+4])
			}
		}
	}
	for _, m := range []string{"RGB", "RGBX"} {
		add(m, "RGBXX", 40, skip(5))
		add(m, "RGBXXX", 48, skip(6))
	}
	add("RGBA", "RGBAX", 40, skip(5))
	add("RGBA", "RGBAXX", 48, skip(6))
	add("CMYK", "CMYKX", 40, skip(5))
	add("CMYK", "CMYKXX", 48, skip(6))
	rgb16l := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[6*i+1], in[6*i+3], in[6*i+5], 255)
		}
	}
	rgba16l := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[8*i+1], in[8*i+3], in[8*i+5], in[8*i+7])
		}
	}
	rgba16b := func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[8*i], in[8*i+2], in[8*i+4], in[8*i+6])
		}
	}
	add("RGB", "RGB;16L", 48, rgb16l)
	add("RGBA", "RGBA;16L", 64, rgba16l)
	add("RGB", "RGBX;16L", 64, rgba16l)
	add("RGB", "RGBX;16B", 64, rgba16b)
	add("RGBX", "RGBX;16L", 64, rgba16l)
	add("RGBX", "RGBX;16B", 64, rgba16b)
	add("CMYK", "CMYK;16L", 64, rgba16l)
	add("CMYK", "CMYK;16B", 64, rgba16b)
	add("LAB", "LAB", 24, func(out, in []byte, n int) {
		for i := 0; i < n; i++ {
			set4(out, i, in[3*i], in[3*i+1]^128, in[3*i+2]^128, 255)
		}
	})
	for k, name := range []string{"C;I", "M;I", "Y;I", "K;I"} {
		k := k
		add("CMYK", name, 8, func(out, in []byte, n int) {
			for i := 0; i < n; i++ {
				out[4*i+k] = ^in[i]
			}
		})
	}
	for k, name := range []string{"C", "M", "Y", "K"} {
		add("CMYK", name, 8, band(k))
	}
	for k, name := range []string{"L", "A", "B"} {
		add("LAB", name, 8, band(k))
	}
	add("F", "F", 32, copyN(4))
	// Numeric raw modes: the sample converted to int32 or float32 as C does.
	type sample func(in []byte) (int64, float64, bool)
	numeric := func(width int, get sample) (func(out, in []byte, n int), func(out, in []byte, n int)) {
		toI := func(out, in []byte, n int) {
			for i := 0; i < n; i++ {
				v, f, isFloat := get(in[i*width:])
				if isFloat {
					v = int64(int32(f))
				}
				binary.LittleEndian.PutUint32(out[4*i:], uint32(int32(v)))
			}
		}
		toF := func(out, in []byte, n int) {
			for i := 0; i < n; i++ {
				v, f, isFloat := get(in[i*width:])
				if !isFloat {
					f = float64(v)
				}
				var bits uint32
				if isFloat {
					bits = math.Float32bits(float32(f))
				} else if v >= 0 {
					bits = math.Float32bits(float32(uint64(v)))
				} else {
					bits = math.Float32bits(float32(v))
				}
				binary.LittleEndian.PutUint32(out[4*i:], bits)
			}
		}
		return toI, toF
	}
	le := binary.LittleEndian
	be := binary.BigEndian
	samples := map[string]struct {
		width int
		get   sample
	}{
		"8":    {1, func(b []byte) (int64, float64, bool) { return int64(b[0]), 0, false }},
		"8S":   {1, func(b []byte) (int64, float64, bool) { return int64(int8(b[0])), 0, false }},
		"16":   {2, func(b []byte) (int64, float64, bool) { return int64(le.Uint16(b)), 0, false }},
		"16S":  {2, func(b []byte) (int64, float64, bool) { return int64(int16(le.Uint16(b))), 0, false }},
		"16B":  {2, func(b []byte) (int64, float64, bool) { return int64(be.Uint16(b)), 0, false }},
		"16BS": {2, func(b []byte) (int64, float64, bool) { return int64(int16(be.Uint16(b))), 0, false }},
		"16N":  {2, func(b []byte) (int64, float64, bool) { return int64(le.Uint16(b)), 0, false }},
		"16NS": {2, func(b []byte) (int64, float64, bool) { return int64(int16(le.Uint16(b))), 0, false }},
		"32":   {4, func(b []byte) (int64, float64, bool) { return int64(le.Uint32(b)), 0, false }},
		"32S":  {4, func(b []byte) (int64, float64, bool) { return int64(int32(le.Uint32(b))), 0, false }},
		"32B":  {4, func(b []byte) (int64, float64, bool) { return int64(be.Uint32(b)), 0, false }},
		"32BS": {4, func(b []byte) (int64, float64, bool) { return int64(int32(be.Uint32(b))), 0, false }},
		"32N":  {4, func(b []byte) (int64, float64, bool) { return int64(le.Uint32(b)), 0, false }},
		"32NS": {4, func(b []byte) (int64, float64, bool) { return int64(int32(le.Uint32(b))), 0, false }},
		"32NF": {4, func(b []byte) (int64, float64, bool) { return 0, float64(math.Float32frombits(le.Uint32(b))), true }},
		"64F":  {8, func(b []byte) (int64, float64, bool) { return 0, math.Float64frombits(le.Uint64(b)), true }},
		"64BF": {8, func(b []byte) (int64, float64, bool) { return 0, math.Float64frombits(be.Uint64(b)), true }},
		"64NF": {8, func(b []byte) (int64, float64, bool) { return 0, math.Float64frombits(le.Uint64(b)), true }},
	}
	for name, s := range samples {
		toI, toF := numeric(s.width, s.get)
		if pillowUnpackers["I\x00I;"+name] {
			if _, done := unpackers["I\x00I;"+name]; !done {
				add("I", "I;"+name, 8*s.width, toI)
			}
		}
		if pillowUnpackers["F\x00F;"+name] {
			if _, done := unpackers["F\x00F;"+name]; !done {
				add("F", "F;"+name, 8*s.width, toF)
			}
		}
	}
	add("F", "F;32F", 32, copyN(4))
	add("F", "F;32BF", 32, reverse4)
}

// ---------------------------------------------------------------------------
// Tiles and decoders

// tile is an ImageFile._Tile with its decoder arguments spelled out.
type tile struct {
	decoder        string
	x0, y0, x1, y1 int64
	offset         int64
	rawmode        string
	stride         int64 // raw: stride; pcx: bytes per line
	ystep          int64
	depth          int64 // tga_rle: bits per pixel; sgi_rle: bytes per sample
}

func (t tile) sameArgs(o tile) bool {
	return t.decoder == o.decoder && t.x0 == o.x0 && t.y0 == o.y0 && t.x1 == o.x1 &&
		t.y1 == o.y1 && t.rawmode == o.rawmode && t.stride == o.stride &&
		t.ystep == o.ystep && t.depth == o.depth
}

// state is the part of ImagingCodecState the decoders use.
type state struct {
	xoff, yoff, xsize, ysize int
	bits, bytes              int
	u                        unpacker
}

func (st *state) put(c *canvas, y int, line []byte) {
	row := c.row(y + st.yoff)
	st.u.fn(row[st.xoff*c.ps:], line, st.xsize)
}

func cInt(v int64) bool { return v >= math.MinInt32 && v <= math.MaxInt32 }

// decoderFor is Image._getdecoder plus decoder.setimage for a C decoder.
func decoderFor(c *canvas, t tile) (*state, error) {
	rawmode := t.rawmode
	mode := c.mode
	if t.decoder == "xbm" {
		mode, rawmode = "1", "1;R"
	}
	if !cInt(t.stride) || !cInt(t.ystep) || !cInt(t.depth) {
		return nil, raise("OverflowError", "signed integer is greater than maximum")
	}
	u, ok := findUnpacker(mode, rawmode)
	if !ok {
		if pillowUnpackers[mode+"\x00"+rawmode] {
			return nil, pil.Unsupported("raw", "unpacker %s from %s", mode, rawmode)
		}
		return nil, raise("ValueError", "unknown raw mode for given image mode")
	}
	if t.x0 < 0 || t.y0 < 0 || t.x1 <= t.x0 || t.y1 <= t.y0 || t.x1 > int64(c.w) || t.y1 > int64(c.h) {
		return nil, raise("ValueError", "tile cannot extend outside image")
	}
	st := &state{xoff: int(t.x0), yoff: int(t.y0), xsize: int(t.x1 - t.x0), ysize: int(t.y1 - t.y0), bits: u.bits, u: u}
	if t.decoder == "pcx" {
		st.bytes = int(t.stride)
	}
	if st.bytes == 0 {
		if st.xsize > math.MaxInt32/st.bits-7 {
			return nil, raise("MemoryError", "")
		}
		st.bytes = (st.bits*st.xsize + 7) / 8
	}
	return st, nil
}

// decodeAll feeds a whole tile to its decoder the way ImageFile.load's read
// loop does: the decoders keep partial packets for the next call, so one
// call over every remaining byte reaches the same end. It returns whether
// the decoder finished and its error code.
type decodeFunc func(c *canvas, st *state, t tile, data []byte) (done bool, code int)

func rawDecode(c *canvas, st *state, t tile, data []byte) (bool, int) {
	skip := 0
	if t.stride != 0 {
		skip = int(t.stride) - st.bytes
		if skip < 0 {
			return true, codecConfig
		}
	}
	y, step := 0, 1
	if t.ystep < 0 {
		y, step = st.ysize-1, -1
	}
	pos := 0
	for first := true; ; first = false {
		if !first {
			if len(data)-pos < skip {
				return false, 0
			}
			pos += skip
		}
		if len(data)-pos < st.bytes {
			return false, 0
		}
		st.put(c, y, data[pos:pos+st.bytes])
		pos += st.bytes
		y += step
		if y < 0 || y >= st.ysize {
			return true, 0
		}
	}
}

func tgaRleDecode(c *canvas, st *state, t tile, data []byte) (bool, int) {
	y, step := 0, 1
	if t.ystep < 0 {
		y, step = st.ysize-1, -1
	}
	depth := int(t.depth / 8)
	buf := make([]byte, st.bytes)
	x, pos := 0, 0
	for {
		if len(data)-pos < 1 {
			return false, 0
		}
		head := data[pos]
		n := depth * (int(head&0x7f) + 1)
		extra := 0
		if head&0x80 != 0 {
			if len(data)-pos < 1+depth {
				return false, 0
			}
			if x+n > st.bytes {
				return true, codecOverrun
			}
			for i := 0; i < n; i += depth {
				copy(buf[x+i:x+i+depth], data[pos+1:pos+1+depth])
			}
			pos += 1 + depth
		} else {
			if len(data)-pos < 1+n {
				return false, 0
			}
			if x+n > st.bytes {
				extra = n
				n = st.bytes - x
				extra -= n
			}
			copy(buf[x:x+n], data[pos+1:pos+1+n])
			pos += 1 + n
		}
		for {
			x += n
			if x >= st.bytes {
				st.put(c, y, buf)
				x = 0
				y += step
				if y < 0 || y >= st.ysize {
					return true, 0
				}
			}
			if extra == 0 || x > 0 {
				break
			}
			n = extra
			if n > st.bytes {
				n = st.bytes
			}
			copy(buf[x:x+n], data[pos:pos+n])
			pos += n
			extra -= n
		}
	}
}

func sunRleDecode(c *canvas, st *state, t tile, data []byte) (bool, int) {
	buf := make([]byte, st.bytes)
	x, y, pos := 0, 0, 0
	extra, extraData := 0, byte(0)
	for {
		if len(data)-pos < 1 {
			return false, 0
		}
		n := 1
		if data[pos] == 0x80 {
			if len(data)-pos < 2 {
				return false, 0
			}
			n = int(data[pos+1])
			if n == 0 {
				n = 1
				buf[x] = 0x80
				pos += 2
			} else {
				if len(data)-pos < 3 {
					return false, 0
				}
				n++
				if x+n > st.bytes {
					extra = n
					n = st.bytes - x
					extra -= n
					extraData = data[pos+2]
				}
				for i := 0; i < n; i++ {
					buf[x+i] = data[pos+2]
				}
				pos += 3
			}
		} else {
			buf[x] = data[pos]
			pos++
		}
		for {
			x += n
			if x >= st.bytes {
				st.put(c, y, buf)
				x = 0
				y++
				if y >= st.ysize {
					return true, 0
				}
			}
			if extra == 0 || x > 0 {
				break
			}
			n = extra
			if n > st.bytes {
				n = st.bytes
			}
			for i := 0; i < n; i++ {
				buf[x+i] = extraData
			}
			extra -= n
		}
	}
}

func pcxDecode(c *canvas, st *state, t tile, data []byte) (bool, int) {
	if (st.xsize*st.bits+7)/8 > st.bytes {
		return true, codecOverrun
	}
	buf := make([]byte, st.bytes)
	code := 0
	x, y, pos := 0, 0, 0
	for {
		if len(data)-pos < 1 {
			return false, code
		}
		if data[pos]&0xC0 == 0xC0 {
			if len(data)-pos < 2 {
				return false, code
			}
			for n := int(data[pos] & 0x3F); n > 0; n-- {
				if x >= st.bytes {
					code = codecOverrun
					break
				}
				buf[x] = data[pos+1]
				x++
			}
			pos += 2
		} else {
			buf[x] = data[pos]
			x++
			pos++
		}
		if x >= st.bytes {
			var bands, xsize, stride int
			if st.bits == 2 || st.bits == 4 {
				xsize = (st.xsize + 7) / 8
				bands = st.bits
				stride = st.bytes / st.bits
			} else {
				xsize = st.xsize
				bands = st.bytes / st.xsize
				if bands != 0 {
					stride = st.bytes / bands
				}
			}
			if stride > xsize {
				for i := 1; i < bands; i++ {
					copy(buf[i*xsize:i*xsize+xsize], buf[i*stride:i*stride+xsize])
				}
			}
			st.put(c, y, buf)
			x = 0
			y++
			if y >= st.ysize {
				return true, code
			}
		}
	}
}

func hexDigit(v byte) byte {
	switch {
	case v >= '0' && v <= '9':
		return v - '0'
	case v >= 'a' && v <= 'f':
		return v - 'a' + 10
	case v >= 'A' && v <= 'F':
		return v - 'A' + 10
	}
	return 0
}

func xbmDecode(c *canvas, st *state, t tile, data []byte) (bool, int) {
	buf := make([]byte, st.bytes)
	x, y, pos := 0, 0, 0
	for {
		for pos < len(data) && data[pos] != 'x' {
			pos++
		}
		if pos == len(data) {
			return false, 0
		}
		if len(data)-pos < 3 {
			return false, 0
		}
		buf[x] = hexDigit(data[pos+1])<<4 + hexDigit(data[pos+2])
		x++
		if x >= st.bytes {
			st.u.fn(c.row(y), buf, st.xsize)
			x = 0
			y++
			if y >= st.ysize {
				return true, 0
			}
		}
		pos += 3
	}
}

func packbitsDecode(c *canvas, st *state, t tile, data []byte) (bool, int) {
	buf := make([]byte, st.bytes)
	x, y, pos := 0, 0, 0
	for {
		if len(data)-pos < 1 {
			return false, 0
		}
		head := data[pos]
		switch {
		case head == 0x80:
			pos++
			continue
		case head&0x80 != 0:
			if len(data)-pos < 2 {
				return false, 0
			}
			for n := 257 - int(head); n > 0 && x < st.bytes; n-- {
				buf[x] = data[pos+1]
				x++
			}
			pos += 2
		default:
			n := int(head) + 2
			if len(data)-pos < n {
				return false, 0
			}
			for i := 1; i < n && x < st.bytes; i++ {
				buf[x] = data[pos+i]
				x++
			}
			pos += n
		}
		if x >= st.bytes {
			st.put(c, y, buf)
			x = 0
			y++
			if y >= st.ysize {
				return true, 0
			}
		}
	}
}

var decoders = map[string]decodeFunc{
	"packbits": packbitsDecode,
	"raw":      rawDecode,
	"tga_rle":  tgaRleDecode,
	"sun_rle":  sunRleDecode,
	"pcx":      pcxDecode,
	"xbm":      xbmDecode,
}

// runTiles is the tile loop of ImageFile.load over a prepared canvas. It
// returns the error code of the last decode call.
func runTiles(f *file, c *canvas, tiles []tile) (int, error) {
	sorted := append([]tile(nil), tiles...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].offset < sorted[j].offset })
	var kept []tile
	for i, t := range sorted {
		if i+1 < len(sorted) && t.sameArgs(sorted[i+1]) {
			continue
		}
		kept = append(kept, t)
	}
	code := codecUnknown
	for _, t := range kept {
		if err := f.seek(t.offset, 0); err != nil {
			return 0, err
		}
		decode, ok := decoders[t.decoder]
		if !ok {
			return 0, pil.Unsupported("decoder", "%s", t.decoder)
		}
		st, err := decoderFor(c, t)
		if err != nil {
			return 0, err
		}
		data := f.read(-1)
		if len(data) == 0 {
			return 0, raise("OSError", "image file is truncated (0 bytes not processed)")
		}
		done, status := decode(c, st, t, data)
		code = status
		if !done {
			return 0, raise("OSError", "image file is truncated")
		}
	}
	return code, nil
}

// loadRaster is ImageFile.load for a plugin with the default loop:
// load_prepare (a zero canvas, the palette realized for "P"), the tiles,
// load_end, the decoder error check, and Image.load's palette step.
func loadRaster(f *file, format, mode string, w, h int64, pal *pyPalette, tiles []tile, loadEnd func(*canvas) error) (*pil.Image, error) {
	if len(tiles) == 0 {
		return nil, raise("OSError", "cannot load this image")
	}
	c, err := newCanvas(mode, w, h)
	if err != nil {
		return nil, err
	}
	if mode == "P" && pal != nil {
		if err := realizePalette(c, pal); err != nil {
			return nil, err
		}
		pal = nil
	}
	code, err := runTiles(f, c, tiles)
	if err != nil {
		return nil, err
	}
	if loadEnd != nil {
		if err := loadEnd(c); err != nil {
			return nil, err
		}
	}
	if code < 0 {
		return nil, raise("OSError", "decoder error %d when reading image file", code)
	}
	if pal != nil {
		if err := realizePalette(c, pal); err != nil {
			return nil, err
		}
		if c.mode != mode {
			return nil, pil.Unsupported(format, "a palette on a %s image turns the core image into %s", mode, c.mode)
		}
	}
	return c.image(format), nil
}

// rawInto is PyDecoder.set_as_raw / Image.frombytes: decode data as one raw
// tile over the whole canvas; short data is ValueError.
func rawInto(c *canvas, rawmode string, data []byte, ystep int64) error {
	t := tile{decoder: "raw", x1: int64(c.w), y1: int64(c.h), rawmode: rawmode, ystep: ystep}
	st, err := decoderFor(c, t)
	if err != nil {
		return err
	}
	done, code := rawDecode(c, st, t, data)
	if !done {
		return raise("ValueError", "not enough image data")
	}
	if code != 0 {
		return raise("ValueError", "cannot decode image data")
	}
	return nil
}

var _ = binary.LittleEndian
