package webpdec

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"math/rand"
)

// Test inputs built in Go: a minimal VP8L lossless writer (8-bit literal
// codes, no colour cache, one Huffman group, optional subtract-green and
// predictor transforms) and RIFF wrapping.

type bitWriter struct {
	buf []byte
	acc uint64
	n   uint
}

func (w *bitWriter) put(v uint32, n uint) {
	w.acc |= uint64(v) << w.n
	w.n += n
	for w.n >= 8 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
	}
}

func (w *bitWriter) bytes() []byte {
	if w.n > 0 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc, w.n = 0, 0
	}
	return w.buf
}

// putLengths writes a normal Huffman code whose symbols below literals
// have length 8 and the rest length 0, through a code-length code with
// symbols 0 and 8 of length 1.
func (w *bitWriter) putLengths(numSymbols, literals int) {
	w.put(0, 1)
	w.put(12-4, 4) // twelve code-length code lengths, up to symbol 8
	order := codeLengthCodeOrder[:12]
	for _, sym := range order {
		if sym == 0 || sym == 8 {
			w.put(1, 3)
		} else {
			w.put(0, 3)
		}
	}
	w.put(0, 1) // max_symbol not used
	for i := 0; i < numSymbols; i++ {
		if i < literals {
			w.put(1, 1)
		} else {
			w.put(0, 1)
		}
	}
}

func (w *bitWriter) putSymbol(v uint32) {
	w.put(uint32(bits.Reverse8(uint8(v))), 8)
}

// putImage writes one entropy-coded image of ARGB values.
func (w *bitWriter) putImage(argb []uint32) {
	w.put(0, 1) // no colour cache
	w.putLengths(280, 256)
	w.putLengths(256, 256)
	w.putLengths(256, 256)
	w.putLengths(256, 256)
	w.put(1, 1) // simple distance code: one symbol, 0
	w.put(0, 1)
	w.put(0, 1)
	w.put(0, 1)
	for _, p := range argb {
		w.putSymbol(p >> 8)
		w.putSymbol(p >> 16)
		w.putSymbol(p)
		w.putSymbol(p >> 24)
	}
}

func sub(a, b uint32) uint32 {
	var out uint32
	for s := uint(0); s < 32; s += 8 {
		out |= (((a >> s) - (b >> s)) & 0xff) << s
	}
	return out
}

// vp8lEncode returns a VP8L bitstream of argb. predictor selects no
// predictor transform (0) or one with every tile on mode 1 or 2.
func vp8lEncode(w, h int, argb []uint32, alphaHint, subtractGreen bool, predictor int) []byte {
	bw := &bitWriter{}
	bw.put(0x2f, 8)
	bw.put(uint32(w-1), 14)
	bw.put(uint32(h-1), 14)
	if alphaHint {
		bw.put(1, 1)
	} else {
		bw.put(0, 1)
	}
	bw.put(0, 3)
	data := append([]uint32{}, argb...)
	if predictor != 0 {
		// Residuals against the decoder's predictions: black, left, top.
		res := make([]uint32, len(data))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var pred uint32
				switch {
				case y == 0 && x == 0:
					pred = argbBlack
				case y == 0:
					pred = data[x-1]
				case x == 0:
					pred = data[(y-1)*w]
				case predictor == 1:
					pred = data[y*w+x-1]
				default:
					pred = data[(y-1)*w+x]
				}
				res[y*w+x] = sub(data[y*w+x], pred)
			}
		}
		data = res
	}
	if subtractGreen {
		for i, p := range data {
			g := (p >> 8) & 0xff
			r := ((p>>16)&0xff - g) & 0xff
			b := (p&0xff - g) & 0xff
			data[i] = p&0xff00ff00 | r<<16 | b
		}
	}
	// Transforms are read in reverse of their application: subtract-green
	// was applied last, so it is written first.
	if subtractGreen {
		bw.put(1, 1)
		bw.put(subtractGreenTransform, 2)
	}
	if predictor != 0 {
		bw.put(1, 1)
		bw.put(predictorTransform, 2)
		const tileBits = 3
		bw.put(tileBits-2, 3)
		tiles := subSampleSize(w, tileBits) * subSampleSize(h, tileBits)
		modes := make([]uint32, tiles)
		for i := range modes {
			modes[i] = uint32(predictor) << 8
		}
		bw.putImage(modes)
	}
	bw.put(0, 1) // no more transforms
	bw.put(0, 1) // no colour cache
	bw.put(0, 1) // no meta Huffman image
	bw.putLengths(280, 256)
	bw.putLengths(256, 256)
	bw.putLengths(256, 256)
	bw.putLengths(256, 256)
	bw.put(1, 1)
	bw.put(0, 1)
	bw.put(0, 1)
	bw.put(0, 1)
	for _, p := range data {
		bw.putSymbol(p >> 8)
		bw.putSymbol(p >> 16)
		bw.putSymbol(p)
		bw.putSymbol(p >> 24)
	}
	return bw.bytes()
}

func riffChunk(tag string, data []byte) []byte {
	var out bytes.Buffer
	out.WriteString(tag)
	binary.Write(&out, binary.LittleEndian, uint32(len(data)))
	out.Write(data)
	if len(data)&1 == 1 {
		out.WriteByte(0)
	}
	return out.Bytes()
}

func riffFile(chunks ...[]byte) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		body.Write(c)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// goldenInput is one seeded Go-built WebP.
type goldenInput struct {
	Name          string `json:"name"`
	Seed          int64  `json:"seed"`
	W             int    `json:"w"`
	H             int    `json:"h"`
	Alpha         bool   `json:"alpha"`
	SubtractGreen bool   `json:"subtract_green"`
	Predictor     int    `json:"predictor"`
	Container     string `json:"container"`
}

func (g goldenInput) bytes() []byte {
	rng := rand.New(rand.NewSource(g.Seed))
	argb := make([]uint32, g.W*g.H)
	for i := range argb {
		x, y := i%g.W, i/g.W
		base := uint32(rng.Intn(1 << 24))
		if rng.Intn(4) != 0 {
			base = uint32(x*7+y*3)<<16 | uint32(x*5)<<8 | uint32(y*9)
		}
		a := uint32(255)
		if g.Alpha {
			a = uint32((x * 31) & 0xff)
		}
		argb[i] = a<<24 | base&0xffffff
	}
	bs := vp8lEncode(g.W, g.H, argb, g.Alpha, g.SubtractGreen, g.Predictor)
	switch g.Container {
	case "vp8x":
		flags := byte(0x08)
		if g.Alpha {
			flags |= 0x10
		}
		d := make([]byte, 10)
		d[0] = flags
		d[4], d[5], d[6] = byte(g.W-1), byte((g.W-1)>>8), byte((g.W-1)>>16)
		d[7], d[8], d[9] = byte(g.H-1), byte((g.H-1)>>8), byte((g.H-1)>>16)
		return riffFile(riffChunk("VP8X", d), riffChunk("VP8L", bs),
			riffChunk("EXIF", []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00")))
	default:
		return riffFile(riffChunk("VP8L", bs))
	}
}

func goldenInputs() []goldenInput {
	var out []goldenInput
	seed := int64(1)
	for _, sz := range [][2]int{{1, 1}, {3, 2}, {16, 16}, {37, 23}} {
		for _, v := range []struct {
			alpha, sg bool
			pred      int
			container string
		}{
			{false, false, 0, "simple"}, {true, false, 0, "simple"}, {false, true, 0, "simple"},
			{true, true, 1, "simple"}, {false, false, 2, "vp8x"}, {true, true, 2, "vp8x"},
		} {
			out = append(out, goldenInput{
				Name: "", Seed: seed, W: sz[0], H: sz[1], Alpha: v.alpha,
				SubtractGreen: v.sg, Predictor: v.pred, Container: v.container,
			})
			seed++
		}
	}
	for i := range out {
		g := &out[i]
		g.Name = goldenName(*g)
	}
	return out
}

func goldenName(g goldenInput) string {
	return string(rune('a'+g.Seed%26)) + "-" + itoa(g.W) + "x" + itoa(g.H) + "-" + g.Container
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
