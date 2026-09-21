package bmpdec

import (
	"bytes"
	"encoding/binary"
	"math/rand"
)

// bmpSpec describes a BMP or DIB for tests; fields are written as given,
// so inconsistent files can be built on purpose.
type bmpSpec struct {
	dib         bool
	headerSize  int
	width       int32
	height      int32
	bits        int
	compression uint32
	colors      uint32
	masks       [4]uint32
	masksAfter  bool // 40-byte BITFIELDS: masks follow the header
	palette     []byte
	pixels      []byte
	offset      int // 0: computed
}

func (s bmpSpec) bytes() []byte {
	var hdr bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&hdr, le, uint32(s.headerSize))
	if s.headerSize == 12 {
		binary.Write(&hdr, le, uint16(s.width))
		binary.Write(&hdr, le, uint16(s.height))
		binary.Write(&hdr, le, uint16(1))
		binary.Write(&hdr, le, uint16(s.bits))
	} else {
		binary.Write(&hdr, le, s.width)
		binary.Write(&hdr, le, s.height)
		binary.Write(&hdr, le, uint16(1))
		binary.Write(&hdr, le, uint16(s.bits))
		binary.Write(&hdr, le, s.compression)
		binary.Write(&hdr, le, uint32(len(s.pixels)))
		binary.Write(&hdr, le, uint32(2835))
		binary.Write(&hdr, le, uint32(2835))
		binary.Write(&hdr, le, s.colors)
		binary.Write(&hdr, le, uint32(0))
		for hdr.Len() < s.headerSize {
			i := (hdr.Len() - 40) / 4
			if i < 4 {
				binary.Write(&hdr, le, s.masks[i])
			} else {
				hdr.WriteByte(0)
			}
		}
		hdr.Truncate(s.headerSize)
		if s.masksAfter {
			for i := 0; i < 3; i++ {
				binary.Write(&hdr, le, s.masks[i])
			}
		}
	}
	var out bytes.Buffer
	if !s.dib {
		offset := s.offset
		if offset == 0 {
			offset = 14 + hdr.Len() + len(s.palette)
		}
		out.WriteString("BM")
		binary.Write(&out, le, uint32(14+hdr.Len()+len(s.palette)+len(s.pixels)))
		binary.Write(&out, le, uint32(0))
		binary.Write(&out, le, uint32(offset))
	}
	out.Write(hdr.Bytes())
	out.Write(s.palette)
	out.Write(s.pixels)
	return out.Bytes()
}

// rawRows packs row-major samples (one int per pixel) into stored rows,
// bottom-up unless topDown, each padded to four bytes.
func rawRows(samples []uint32, w, h, bits int, topDown bool) []byte {
	stride := ((w*bits + 31) >> 3) &^ 3
	out := make([]byte, stride*h)
	for y := 0; y < h; y++ {
		row := out[stride*(h-1-y):]
		if topDown {
			row = out[stride*y:]
		}
		for x := 0; x < w; x++ {
			v := samples[y*w+x]
			switch bits {
			case 1, 2, 4:
				pos := x * bits
				row[pos/8] |= byte(v<<uint(8-bits)) >> uint(pos%8)
			case 8:
				row[x] = byte(v)
			case 16:
				binary.LittleEndian.PutUint16(row[2*x:], uint16(v))
			case 24:
				row[3*x], row[3*x+1], row[3*x+2] = byte(v), byte(v>>8), byte(v>>16)
			case 32:
				binary.LittleEndian.PutUint32(row[4*x:], v)
			}
		}
	}
	return out
}

func randomSamples(rng *rand.Rand, n int, max uint64) []uint32 {
	out := make([]uint32, n)
	for i := range out {
		out[i] = uint32(rng.Uint64() % max)
	}
	return out
}

func bgrxPalette(rng *rand.Rand, entries, bpp int) []byte {
	p := make([]byte, entries*bpp)
	rng.Read(p)
	return p
}

func grayBGRX(entries, bpp int, values func(int) byte) []byte {
	p := make([]byte, entries*bpp)
	for i := 0; i < entries; i++ {
		v := values(i)
		p[i*bpp], p[i*bpp+1], p[i*bpp+2] = v, v, v
	}
	return p
}

// rle8 encodes rows (row-major, bottom row stored first) with runs,
// absolute blocks and end-of-line codes, then end of bitmap.
func rleEncode(samples []byte, w, h int, four bool) []byte {
	var out []byte
	for yy := h - 1; yy >= 0; yy-- {
		row := samples[yy*w : (yy+1)*w]
		for x := 0; x < w; {
			run := 1
			for x+run < w && row[x+run] == row[x] && run < 255 {
				run++
			}
			if run >= 3 || w-x < 3 {
				if four {
					out = append(out, byte(run), row[x]<<4|row[x]&0x0f)
				} else {
					out = append(out, byte(run), row[x])
				}
				x += run
				continue
			}
			n := min(w-x, 7)
			out = append(out, 0, byte(n))
			if four {
				for i := 0; i < n; i += 2 {
					b := row[x+i] << 4
					if i+1 < n {
						b |= row[x+i+1] & 0x0f
					}
					out = append(out, b)
				}
				if (n+1)/2%2 == 1 {
					out = append(out, 0)
				}
			} else {
				out = append(out, row[x:x+n]...)
				if n%2 == 1 {
					out = append(out, 0)
				}
			}
			x += n
		}
		out = append(out, 0, 0)
	}
	return append(out, 0, 1)
}
