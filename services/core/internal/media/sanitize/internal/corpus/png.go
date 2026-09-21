package corpus

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"math/rand"
	"strconv"
)

// PNGParams describe a PNG built chunk by chunk.
type PNGParams struct {
	Pattern   string `json:"pattern,omitempty"`
	ColorType int    `json:"color_type"`          // 0, 2, 3, 4 or 6
	BitDepth  int    `json:"bit_depth,omitempty"` // default 8
	Interlace bool   `json:"interlace,omitempty"`
	// Colors is the palette size for colour type 3 (default 1<<bit depth).
	Colors int `json:"colors,omitempty"`
	// TRNS adds a tRNS chunk: "gray"/"rgb" pick a single transparent
	// value from the image, "palette" per-entry alpha for Colors/2 entries.
	TRNS string `json:"trns,omitempty"`
	// Text adds tEXt/zTXt/iTXt chunks before IDAT.
	Text []TextChunk `json:"text,omitempty"`
	// Exif adds an eXIf chunk, before IDAT unless ExifAfterIDAT.
	Exif          *ExifParams `json:"exif,omitempty"`
	ExifAfterIDAT bool        `json:"exif_after_idat,omitempty"`
	// HeaderOnly writes IHDR then IEND: a cheap way to claim any size.
	HeaderOnly bool `json:"header_only,omitempty"`
	// IDATSplit cuts the zlib stream into IDAT chunks of this size.
	IDATSplit int `json:"idat_split,omitempty"`
	// BadCRC corrupts the CRC of the first chunk of this type.
	BadCRC   string `json:"bad_crc,omitempty"`
	Truncate int    `json:"truncate,omitempty"`
}

// TextChunk is a PNG text chunk.
type TextChunk struct {
	Type  string `json:"type"` // "tEXt", "zTXt" or "iTXt"
	Key   string `json:"key"`
	Value string `json:"value"`
}

func pngChannels(colorType int) int {
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

// BuildPNG writes a PNG with each row's filter type drawn from the seed.
func BuildPNG(rng *rand.Rand, w, h int, p PNGParams) ([]byte, error) {
	depth := p.BitDepth
	if depth == 0 {
		depth = 8
	}
	channels := pngChannels(p.ColorType)
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8] = byte(depth)
	ihdr[9] = byte(p.ColorType)
	if p.Interlace {
		ihdr[12] = 1
	}
	badCRC := p.BadCRC
	chunk := func(kind string, data []byte) {
		var head [8]byte
		binary.BigEndian.PutUint32(head[:4], uint32(len(data)))
		copy(head[4:], kind)
		out.Write(head[:])
		out.Write(data)
		crc := crc32.NewIEEE()
		crc.Write(head[4:])
		crc.Write(data)
		sum := crc.Sum32()
		if kind == badCRC {
			sum ^= 0x5A5A5A5A
			badCRC = ""
		}
		var tail [4]byte
		binary.BigEndian.PutUint32(tail[:], sum)
		out.Write(tail[:])
	}
	chunk("IHDR", ihdr)
	if p.HeaderOnly {
		chunk("IEND", nil)
		return truncate(out.Bytes(), p.Truncate), nil
	}

	maxSample := (1 << depth) - 1
	colors := p.Colors
	if p.ColorType == 3 {
		if colors <= 0 || colors > 1<<depth {
			colors = 1 << depth
		}
		plte := make([]byte, 3*colors)
		rng.Read(plte)
		chunk("PLTE", plte)
	}
	samples := make([]int, w*h*channels)
	pat := newPattern(rng, p.Pattern)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < channels; c++ {
				v := pat.sample(x, y, c)
				switch {
				case p.ColorType == 3:
					v %= colors
				case depth == 16:
					v = v<<8 | rng.Intn(256)
				default:
					v >>= 8 - depth
				}
				samples[(y*w+x)*channels+c] = v
			}
		}
	}
	switch p.TRNS {
	case "gray", "rgb":
		if w > 0 && h > 0 {
			trns := make([]byte, 0, 6)
			for c := 0; c < channels && c < 3; c++ {
				trns = binary.BigEndian.AppendUint16(trns, uint16(samples[c]&maxSample))
			}
			chunk("tRNS", trns)
		}
	case "palette":
		alpha := make([]byte, colors/2+1)
		rng.Read(alpha)
		chunk("tRNS", alpha)
	}
	for _, text := range p.Text {
		chunk(text.Type, textPayload(text))
	}
	if p.Exif != nil && !p.ExifAfterIDAT {
		chunk("eXIf", BuildExif(*p.Exif))
	}

	raw := pngScanlines(rng, samples, w, h, channels, depth, p.Interlace)
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	stream := z.Bytes()
	split := p.IDATSplit
	if split <= 0 {
		split = len(stream) + 1
	}
	for len(stream) > 0 {
		n := min(split, len(stream))
		chunk("IDAT", stream[:n])
		stream = stream[n:]
	}
	if p.Exif != nil && p.ExifAfterIDAT {
		chunk("eXIf", BuildExif(*p.Exif))
	}
	chunk("IEND", nil)
	return truncate(out.Bytes(), p.Truncate), nil
}

func textPayload(text TextChunk) []byte {
	switch text.Type {
	case "zTXt":
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write([]byte(text.Value))
		zw.Close()
		return append(append([]byte(text.Key), 0, 0), z.Bytes()...)
	case "iTXt":
		return append(append([]byte(text.Key), 0, 0, 0, 0, 0), text.Value...)
	}
	return append(append([]byte(text.Key), 0), text.Value...)
}

var adam7 = [7][4]int{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}}

// pngScanlines packs and filters the samples, pass by pass when interlaced.
func pngScanlines(rng *rand.Rand, samples []int, w, h, channels, depth int, interlace bool) []byte {
	var raw []byte
	passes := [][4]int{{0, 0, 1, 1}}
	if interlace {
		passes = adam7[:]
	}
	bpp := (channels*depth + 7) / 8
	for _, pass := range passes {
		x0, y0, dx, dy := pass[0], pass[1], pass[2], pass[3]
		pw := (w - x0 + dx - 1) / dx
		ph := (h - y0 + dy - 1) / dy
		if pw <= 0 || ph <= 0 {
			continue
		}
		stride := (pw*channels*depth + 7) / 8
		prev := make([]byte, stride)
		for py := 0; py < ph; py++ {
			line := make([]byte, stride)
			bit := 0
			for px := 0; px < pw; px++ {
				x, y := x0+px*dx, y0+py*dy
				for c := 0; c < channels; c++ {
					v := samples[(y*w+x)*channels+c]
					if depth == 16 {
						line[bit/8] = byte(v >> 8)
						line[bit/8+1] = byte(v)
					} else {
						line[bit/8] |= byte(v << (8 - depth - bit%8))
					}
					bit += depth
				}
			}
			filter := rng.Intn(5)
			raw = append(raw, byte(filter))
			raw = append(raw, pngFilter(filter, line, prev, bpp)...)
			prev = line
		}
	}
	return raw
}

func pngFilter(filter int, line, prev []byte, bpp int) []byte {
	out := make([]byte, len(line))
	for i := range line {
		var a, b, c int
		if i >= bpp {
			a = int(line[i-bpp])
			c = int(prev[i-bpp])
		}
		b = int(prev[i])
		var pred int
		switch filter {
		case 1:
			pred = a
		case 2:
			pred = b
		case 3:
			pred = (a + b) / 2
		case 4:
			pa, pb, pc := abs(b-c), abs(a-c), abs(a+b-2*c)
			switch {
			case pa <= pb && pa <= pc:
				pred = a
			case pb <= pc:
				pred = b
			default:
				pred = c
			}
		}
		out[i] = line[i] - byte(pred)
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ExifParams describe a TIFF-structured EXIF blob holding tag 274.
type ExifParams struct {
	BigEndian bool `json:"big_endian,omitempty"`
	// Orientation is the stored value (numerator for RATIONAL types).
	Orientation int `json:"orientation"`
	// Type is the TIFF field type (default 3, SHORT).
	Type int `json:"type,omitempty"`
	// Count is the field count (default 1).
	Count int `json:"count,omitempty"`
	// Denominator for RATIONAL/SRATIONAL (default 1).
	Denominator int  `json:"denominator,omitempty"`
	Prefix      bool `json:"prefix,omitempty"` // prepend "Exif\0\0"
	// BadOffset points IFD0 past the end of the blob.
	BadOffset bool `json:"bad_offset,omitempty"`
	// Omit writes an IFD without tag 274.
	Omit bool `json:"omit,omitempty"`
}

// BuildExif writes a TIFF header and IFD0 with one entry (or none).
func BuildExif(p ExifParams) []byte {
	var order binary.AppendByteOrder = binary.LittleEndian
	head := []byte("II*\x00")
	if p.BigEndian {
		order = binary.BigEndian
		head = []byte("MM\x00*")
	}
	typ, count, den := p.Type, p.Count, p.Denominator
	if typ == 0 {
		typ = 3
	}
	if count == 0 {
		count = 1
	}
	if den == 0 {
		den = 1
	}
	out := append([]byte{}, head...)
	offset := uint32(8)
	if p.BadOffset {
		offset = 4096
	}
	out = order.AppendUint32(out, offset)
	entries := 1
	if p.Omit {
		entries = 0
	}
	out = order.AppendUint16(out, uint16(entries))
	if entries == 1 {
		value := make([]byte, 0, 16)
		for i := 0; i < count; i++ {
			switch typ {
			case 1, 6, 7:
				value = append(value, byte(p.Orientation))
			case 2:
				value = append(value, strconv.Itoa(p.Orientation)...)
			case 3, 8:
				value = order.AppendUint16(value, uint16(p.Orientation))
			case 4, 9:
				value = order.AppendUint32(value, uint32(p.Orientation))
			case 5, 10:
				value = order.AppendUint32(value, uint32(p.Orientation))
				value = order.AppendUint32(value, uint32(den))
			case 11:
				value = order.AppendUint32(value, math.Float32bits(float32(p.Orientation)))
			case 12:
				value = order.AppendUint64(value, math.Float64bits(float64(p.Orientation)))
			}
		}
		if typ == 2 {
			value = append(value, 0)
			count = len(value)
		}
		out = order.AppendUint16(out, 274)
		out = order.AppendUint16(out, uint16(typ))
		out = order.AppendUint32(out, uint32(count))
		if len(value) <= 4 {
			field := make([]byte, 4)
			copy(field, value)
			out = append(out, field...)
			out = order.AppendUint32(out, 0)
		} else {
			// entry value offset: header 8 + count 2 + entry 12 + next 4
			out = order.AppendUint32(out, 26)
			out = order.AppendUint32(out, 0)
			out = append(out, value...)
		}
	} else {
		out = order.AppendUint32(out, 0)
	}
	if p.Prefix {
		out = append([]byte("Exif\x00\x00"), out...)
	}
	return out
}

// PPMParams describe a netpbm file.
type PPMParams struct {
	Pattern string `json:"pattern,omitempty"`
	Magic   string `json:"magic"` // "P1" .. "P6"
	MaxVal  int    `json:"maxval,omitempty"`
	// Comment adds a "# ..." line after the magic.
	Comment    bool `json:"comment,omitempty"`
	HeaderOnly bool `json:"header_only,omitempty"`
	Truncate   int  `json:"truncate,omitempty"`
}

// BuildPPM writes P1-P6 with values scaled to MaxVal.
func BuildPPM(rng *rand.Rand, w, h int, p PPMParams) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString(p.Magic)
	out.WriteByte('\n')
	if p.Comment {
		out.WriteString("# corpus\n")
	}
	bitmap := p.Magic == "P1" || p.Magic == "P4"
	maxval := p.MaxVal
	if maxval == 0 {
		maxval = 255
	}
	if bitmap {
		fmt.Fprintf(&out, "%d %d\n", w, h)
	} else {
		fmt.Fprintf(&out, "%d %d\n%d\n", w, h, maxval)
	}
	if p.HeaderOnly {
		return out.Bytes(), nil
	}
	channels := 1
	if p.Magic == "P3" || p.Magic == "P6" {
		channels = 3
	}
	pix := Pixels(rng, p.Pattern, w, h, channels)
	switch p.Magic {
	case "P1", "P2", "P3":
		for i, v := range pix {
			value := int(v) * maxval / 255
			if bitmap {
				value = int(v) & 1
			}
			fmt.Fprintf(&out, "%d", value)
			if (i+1)%(w*channels) == 0 {
				out.WriteByte('\n')
			} else {
				out.WriteByte(' ')
			}
		}
	case "P4":
		stride := (w + 7) / 8
		for y := 0; y < h; y++ {
			row := make([]byte, stride)
			for x := 0; x < w; x++ {
				if pix[y*w+x]&1 == 1 {
					row[x/8] |= 0x80 >> (x % 8)
				}
			}
			out.Write(row)
		}
	default: // P5, P6
		for _, v := range pix {
			value := int(v) * maxval / 255
			if maxval > 255 {
				out.WriteByte(byte(value >> 8))
			}
			out.WriteByte(byte(value))
		}
	}
	return truncate(out.Bytes(), p.Truncate), nil
}
