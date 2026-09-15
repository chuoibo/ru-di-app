package gifdec

import (
	"bytes"
	"compress/lzw"
	"math/rand"
)

// gifFile builds GIF inputs for tests from explicit parts, so malformed
// structures can be written on purpose.
type gifFile struct {
	version       string
	width, height int
	global        []byte // palette bytes; nil for no global table
	globalBits    int    // size field when global is set (table holds 3<<bits)
	background    byte
	body          bytes.Buffer
	noTrailer     bool
}

func (g *gifFile) bytes() []byte {
	var out bytes.Buffer
	out.WriteString(g.version)
	out.Write([]byte{byte(g.width), byte(g.width >> 8), byte(g.height), byte(g.height >> 8)})
	flags := byte(0)
	if g.global != nil {
		flags = 0x80 | byte(g.globalBits-1)
	}
	out.Write([]byte{flags, g.background, 0})
	out.Write(g.global)
	out.Write(g.body.Bytes())
	if !g.noTrailer {
		out.WriteByte(';')
	}
	return out.Bytes()
}

func (g *gifFile) gce(flags byte, delay int, transparent byte) {
	g.body.Write([]byte{'!', 0xf9, 4, flags, byte(delay), byte(delay >> 8), transparent, 0})
}

func (g *gifFile) comment(text string) {
	g.body.Write([]byte{'!', 0xfe})
	writeSubBlocks(&g.body, []byte(text), 255)
}

func (g *gifFile) netscape(loop int) {
	g.body.Write([]byte{'!', 0xff, 11})
	g.body.WriteString("NETSCAPE2.0")
	g.body.Write([]byte{3, 1, byte(loop), byte(loop >> 8), 0})
}

// image writes an image descriptor, an optional local table and LZW data.
func (g *gifFile) image(x, y, w, h int, interlace bool, local []byte, localBits int, lzwData []byte, minCodeSize byte) {
	flags := byte(0)
	if interlace {
		flags |= 0x40
	}
	if local != nil {
		flags |= 0x80 | byte(localBits-1)
	}
	g.body.Write([]byte{',', byte(x), byte(x >> 8), byte(y), byte(y >> 8), byte(w), byte(w >> 8), byte(h), byte(h >> 8), flags})
	g.body.Write(local)
	g.body.WriteByte(minCodeSize)
	writeSubBlocks(&g.body, lzwData, 255)
}

func writeSubBlocks(out *bytes.Buffer, data []byte, max int) {
	for len(data) > 0 {
		n := len(data)
		if n > max {
			n = max
		}
		out.WriteByte(byte(n))
		out.Write(data[:n])
		data = data[n:]
	}
	out.WriteByte(0)
}

// encodeLZW is the standard GIF LZW stream (clear code first, end code
// last) of indices with the given minimum code size.
func encodeLZW(indices []byte, minCodeSize int) []byte {
	var buf bytes.Buffer
	lit := minCodeSize
	if lit < 2 {
		lit = 2
	}
	w := lzw.NewWriter(&buf, lzw.LSB, lit)
	w.Write(indices)
	w.Close()
	return buf.Bytes()
}

// rawCodes packs explicit codes LSB-first with a fixed code width, for
// hand-made (often invalid) streams.
func rawCodes(codes []int, width int) []byte {
	var out []byte
	acc, n := 0, 0
	for _, c := range codes {
		acc |= c << uint(n)
		n += width
		for n >= 8 {
			out = append(out, byte(acc))
			acc >>= 8
			n -= 8
		}
	}
	if n > 0 {
		out = append(out, byte(acc))
	}
	return out
}

func palette(rng *rand.Rand, entries int) []byte {
	p := make([]byte, 3*entries)
	rng.Read(p)
	return p
}

func grayPalette(entries int) []byte {
	p := make([]byte, 3*entries)
	for i := 0; i < entries; i++ {
		p[3*i], p[3*i+1], p[3*i+2] = byte(i), byte(i), byte(i)
	}
	return p
}

func indices(rng *rand.Rand, n, colors int, pattern string) []byte {
	out := make([]byte, n)
	for i := range out {
		switch pattern {
		case "flat":
			out[i] = 0
		case "ramp":
			out[i] = byte((i / 7) % colors)
		default:
			out[i] = byte(rng.Intn(colors))
		}
	}
	return out
}
