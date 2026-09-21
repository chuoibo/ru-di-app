// Package pngtest builds PNG, EXIF and pixel test inputs from seeds for the
// sanitizer's decoder tests, and renders a pil.Image the way Pillow's
// Image.tobytes() does so pixels can be compared with the oracle. No test
// input is ever stored: every byte is regenerated from its parameters.
package pngtest

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"math"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Rand is splitmix64, stable across Go releases.
type Rand struct{ state uint64 }

// NewRand seeds a Rand.
func NewRand(seed uint64) *Rand { return &Rand{state: seed*0x9e3779b97f4a7c15 + 1} }

// Uint64 returns the next value.
func (r *Rand) Uint64() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

// Intn returns a value in [0, n).
func (r *Rand) Intn(n int) int { return int(r.Uint64() % uint64(n)) }

// Bytes returns n random bytes.
func (r *Rand) Bytes(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(r.Uint64())
	}
	return out
}

// Magic is the PNG signature.
var Magic = []byte("\x89PNG\r\n\x1a\n")

// Chunk returns a PNG chunk with a correct CRC.
func Chunk(typ string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, typ...)
	out = append(out, data...)
	crc := crc32.Update(crc32.ChecksumIEEE([]byte(typ)), crc32.IEEETable, data)
	return binary.BigEndian.AppendUint32(out, crc)
}

// BadCRC returns a chunk whose CRC is off by one.
func BadCRC(typ string, data []byte) []byte {
	out := Chunk(typ, data)
	out[len(out)-1]++
	return out
}

// IHDR returns the header chunk data.
func IHDR(w, h uint32, depth, colorType, interlace byte) []byte {
	data := binary.BigEndian.AppendUint32(nil, w)
	data = binary.BigEndian.AppendUint32(data, h)
	return append(data, depth, colorType, 0, 0, interlace)
}

// Channels is the sample count per pixel of a colour type.
func Channels(colorType byte) int {
	switch colorType {
	case 2:
		return 3
	case 4:
		return 2
	case 6:
		return 4
	}
	return 1
}

// Samples draws a w*h grid of samples for a colour type and bit depth.
// Palette images draw indices below indexLimit.
func Samples(r *Rand, w, h int, depth, colorType byte, pattern string, indexLimit int) []uint16 {
	ch := Channels(colorType)
	maxv := 1<<depth - 1
	out := make([]uint16, w*h*ch)
	base := make([]int, ch)
	for c := range base {
		base[c] = r.Intn(maxv + 1)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < ch; c++ {
				var v int
				switch pattern {
				case "noise":
					v = r.Intn(maxv + 1)
				case "flat":
					v = base[c]
				default:
					v = base[c] + (x*(c+3)+y*(5-c))*(maxv/255+1)/2 + r.Intn(7) - 3
					if v < 0 {
						v = 0
					}
					v %= maxv + 1
				}
				if colorType == 3 && indexLimit > 0 {
					v %= indexLimit
				}
				out[(y*w+x)*ch+c] = uint16(v)
			}
		}
	}
	return out
}

var (
	startRow = [7]int{0, 0, 4, 0, 2, 0, 1}
	startCol = [7]int{0, 4, 0, 2, 0, 1, 0}
	rowInc   = [7]int{8, 8, 8, 4, 4, 2, 2}
	colInc   = [7]int{8, 8, 4, 4, 2, 2, 1}
)

func packRow(samples []uint16, depth byte) []byte {
	if depth == 16 {
		out := make([]byte, 2*len(samples))
		for i, v := range samples {
			binary.BigEndian.PutUint16(out[2*i:], v)
		}
		return out
	}
	bits := int(depth)
	out := make([]byte, (len(samples)*bits+7)/8)
	for i, v := range samples {
		pos := i * bits
		out[pos/8] |= byte(v) << uint(8-bits-pos%8)
	}
	return out
}

// Scanlines returns the unfiltered rows (without filter bytes), pass by
// pass for an interlaced image.
func Scanlines(samples []uint16, w, h int, depth, colorType byte, interlace bool) [][]byte {
	ch := Channels(colorType)
	var rows [][]byte
	if !interlace {
		for y := 0; y < h; y++ {
			rows = append(rows, packRow(samples[y*w*ch:(y+1)*w*ch], depth))
		}
		return rows
	}
	for pass := 0; pass < 7; pass++ {
		if w <= startCol[pass] || h <= startRow[pass] {
			continue
		}
		for y := startRow[pass]; y < h; y += rowInc[pass] {
			var line []uint16
			for x := startCol[pass]; x < w; x += colInc[pass] {
				line = append(line, samples[(y*w+x)*ch:(y*w+x+1)*ch]...)
			}
			rows = append(rows, packRow(line, depth))
		}
	}
	return rows
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Filter prefixes every row with a filter type and filters it. A negative
// filter picks one at random per row; rows of a new pass restart from a
// zero previous row (the caller passes pass boundaries as zero-length
// separators is not needed: widths change, and the previous row is reset
// whenever the length changes).
func Filter(r *Rand, rows [][]byte, depth byte, colorType byte, filter int) []byte {
	bpp := (int(depth)*Channels(colorType) + 7) / 8
	var out []byte
	var prev []byte
	for _, row := range rows {
		if len(prev) != len(row) {
			prev = make([]byte, len(row))
		}
		f := filter
		if f < 0 {
			f = r.Intn(5)
		}
		out = append(out, byte(f))
		for i := range row {
			var left, up, upLeft byte
			if i >= bpp {
				left, upLeft = row[i-bpp], prev[i-bpp]
			}
			up = prev[i]
			var pred byte
			switch f {
			case 1:
				pred = left
			case 2:
				pred = up
			case 3:
				pred = byte((int(left) + int(up)) / 2)
			case 4:
				pred = paeth(left, up, upLeft)
			}
			out = append(out, row[i]-pred)
		}
		prev = row
	}
	return out
}

// Zlib compresses with compress/zlib at a level (0 stores).
func Zlib(data []byte, level int) []byte {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		panic(err)
	}
	if _, err := w.Write(data); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// IDATs splits a zlib stream into IDAT chunks of at most size bytes.
func IDATs(stream []byte, size int) []byte {
	var out []byte
	for len(stream) > size {
		out = append(out, Chunk("IDAT", stream[:size])...)
		stream = stream[size:]
	}
	return append(out, Chunk("IDAT", stream)...)
}

// ToBytes is Image.tobytes() of the image Pillow would hold.
func ToBytes(img *pil.Image) []byte {
	switch img.Mode {
	case "1":
		stride := (img.Width + 7) / 8
		out := make([]byte, stride*img.Height)
		for y := 0; y < img.Height; y++ {
			for x := 0; x < img.Width; x++ {
				if img.Pix[y*img.Width+x] != 0 {
					out[y*stride+x/8] |= 0x80 >> uint(x%8)
				}
			}
		}
		return out
	case "I;16B":
		out := make([]byte, len(img.Pix))
		for i := 0; i+1 < len(img.Pix); i += 2 {
			out[i], out[i+1] = img.Pix[i+1], img.Pix[i]
		}
		return out
	}
	return img.Pix
}

// TIFF entry types.
const (
	TypeByte      = 1
	TypeASCII     = 2
	TypeShort     = 3
	TypeLong      = 4
	TypeRational  = 5
	TypeSByte     = 6
	TypeUndefined = 7
	TypeSShort    = 8
	TypeSLong     = 9
	TypeSRational = 10
	TypeFloat     = 11
	TypeDouble    = 12
	TypeIFD       = 13
	TypeLong8     = 16
)

// byteOrder reads and appends in one byte order.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// Entry is one IFD entry: its values are written with the entry's byte
// order; Raw overrides them with literal bytes and Count overrides the
// count (to make a count that does not match the data).
type Entry struct {
	Tag, Type uint16
	Values    []float64
	Raw       []byte
	Count     int
	// Offset forces the value offset field when the data does not fit.
	Offset *uint32
}

func (e Entry) data(order byteOrder) []byte {
	if e.Raw != nil {
		return e.Raw
	}
	var out []byte
	for _, v := range e.Values {
		switch e.Type {
		case TypeByte, TypeSByte, TypeUndefined, TypeASCII:
			out = append(out, byte(int64(v)))
		case TypeShort, TypeSShort:
			out = order.AppendUint16(out, uint16(int64(v)))
		case TypeLong, TypeSLong, TypeIFD:
			out = order.AppendUint32(out, uint32(int64(v)))
		case TypeLong8:
			out = order.AppendUint64(out, uint64(int64(v)))
		case TypeRational, TypeSRational:
			out = order.AppendUint32(out, uint32(int64(v)))
		case TypeFloat:
			out = order.AppendUint32(out, math.Float32bits(float32(v)))
		case TypeDouble:
			out = order.AppendUint64(out, math.Float64bits(v))
		}
	}
	return out
}

var unit = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4, 16: 8}

// TIFF builds a classic TIFF/EXIF block: header with the given magic (4
// bytes, e.g. "II*\x00"), IFD0 at ifdOffset (0 means right after the
// header) and the entries in order.
func TIFF(magic string, ifdOffset uint32, entries []Entry) []byte {
	var order byteOrder = binary.LittleEndian
	if magic[0] == 'M' {
		order = binary.BigEndian
	}
	if ifdOffset == 0 {
		ifdOffset = 8
	}
	out := []byte(magic)
	out = order.AppendUint32(out, ifdOffset)
	for uint32(len(out)) < ifdOffset {
		out = append(out, 0)
	}
	dataStart := int(ifdOffset) + 2 + 12*len(entries) + 4
	var extra []byte
	out = order.AppendUint16(out, uint16(len(entries)))
	for _, e := range entries {
		data := e.data(order)
		count := e.Count
		if count == 0 {
			if u, ok := unit[e.Type]; ok && u > 0 {
				count = len(data) / u
			}
		}
		out = order.AppendUint16(out, e.Tag)
		out = order.AppendUint16(out, e.Type)
		out = order.AppendUint32(out, uint32(count))
		if len(data) <= 4 && e.Offset == nil {
			field := make([]byte, 4)
			copy(field, data)
			out = append(out, field...)
			continue
		}
		offset := uint32(dataStart + len(extra))
		if e.Offset != nil {
			offset = *e.Offset
		}
		out = order.AppendUint32(out, offset)
		extra = append(extra, data...)
	}
	out = order.AppendUint32(out, 0)
	return append(out, extra...)
}
