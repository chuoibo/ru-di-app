// Package bmpdec decodes BMP and DIB files the way Pillow 12.2.0's
// BmpImagePlugin does: its header parsing and errors, the raw decoder with
// the rawmode it picks, and its Python RLE decoder.
package bmpdec

import (
	"errors"
	"fmt"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Plugin is the BMP entry of Image.OPEN.
var Plugin = pil.Plugin{Name: "BMP", Accept: Accept, Open: Open}

// DIBPlugin is the DIB entry of Image.OPEN.
var DIBPlugin = pil.Plugin{Name: "DIB", Accept: DIBAccept, Open: OpenDIB}

// Accept is BmpImagePlugin._accept.
func Accept(prefix []byte) bool {
	return len(prefix) >= 2 && prefix[0] == 'B' && prefix[1] == 'M'
}

// DIBAccept is _dib_accept; a prefix shorter than four bytes raises
// struct.error, which Image.open treats as not accepted.
func DIBAccept(prefix []byte) bool {
	if len(prefix) < 4 {
		return false
	}
	switch le32(prefix) {
	case 12, 40, 52, 56, 64, 108, 124:
		return true
	}
	return false
}

func le16(b []byte) uint32 { return uint32(b[0]) | uint32(b[1])<<8 }
func le32(b []byte) uint32 { return le16(b) | uint32(b[2])<<16 | uint32(b[3])<<24 }

type reader struct {
	data []byte
	pos  int
}

func (r *reader) read(n int) []byte {
	if n < 0 {
		n = 0
	}
	start := r.pos
	if start > len(r.data) {
		start = len(r.data)
	}
	end := start + n
	if end > len(r.data) || end < start {
		end = len(r.data)
	}
	b := r.data[start:end]
	r.pos += len(b)
	if r.pos < start {
		r.pos = start
	}
	return b
}

const (
	compRaw       = 0
	compRLE8      = 1
	compRLE4      = 2
	compBitfields = 3
)

type opened struct {
	data          []byte
	width, height int64
	mode          string
	rawMode       string
	rle           bool
	rle4          bool
	stride        int64
	direction     int
	offset        int
	paletteRaw    []byte
	paletteBpp    int
}

func structError(what string) error { return pil.Next("struct.error: %s", what) }

// Open is BmpImageFile._open.
func Open(data []byte) (pil.Opened, error) {
	r := &reader{data: data}
	head := r.read(14)
	if !Accept(head) {
		return nil, pil.Next("Not a BMP file")
	}
	if len(head) < 14 {
		return nil, structError("file header")
	}
	return bitmap(r, int64(le32(head[10:])), true)
}

// OpenDIB is DibImageFile._open.
func OpenDIB(data []byte) (pil.Opened, error) {
	return bitmap(&reader{data: data}, 0, false)
}

var bit2mode = map[int]([2]string){
	1:  {"P", "P;1"},
	4:  {"P", "P;4"},
	8:  {"P", "P"},
	16: {"RGB", "BGR;15"},
	24: {"RGB", "BGR"},
	32: {"RGB", "BGRX"},
}

type mask4 [4]uint32

var supported32 = []mask4{
	{0xFF0000, 0xFF00, 0xFF, 0x0},
	{0xFF000000, 0xFF0000, 0xFF00, 0x0},
	{0xFF000000, 0xFF00, 0xFF, 0x0},
	{0xFF000000, 0xFF0000, 0xFF00, 0xFF},
	{0xFF, 0xFF00, 0xFF0000, 0xFF000000},
	{0xFF0000, 0xFF00, 0xFF, 0xFF000000},
	{0xFF000000, 0xFF00, 0xFF, 0xFF0000},
	{0x0, 0x0, 0x0, 0x0},
}

var maskModes32 = map[mask4]string{
	{0xFF0000, 0xFF00, 0xFF, 0x0}:        "BGRX",
	{0xFF000000, 0xFF0000, 0xFF00, 0x0}:  "XBGR",
	{0xFF000000, 0xFF00, 0xFF, 0x0}:      "BGXR",
	{0xFF000000, 0xFF0000, 0xFF00, 0xFF}: "ABGR",
	{0xFF, 0xFF00, 0xFF0000, 0xFF000000}: "RGBA",
	{0xFF0000, 0xFF00, 0xFF, 0xFF000000}: "BGRA",
	{0xFF000000, 0xFF00, 0xFF, 0xFF0000}: "BGAR",
	{0x0, 0x0, 0x0, 0x0}:                 "BGRA",
}

// bitmap is BmpImageFile._bitmap with header 0.
func bitmap(r *reader, offset int64, bmp bool) (pil.Opened, error) {
	hs := r.read(4)
	if len(hs) < 4 {
		return nil, structError("header size")
	}
	headerSize := int64(le32(hs))
	// ImageFile._safe_read(fp, header_size - 4)
	var hd []byte
	if n := headerSize - 4; n > 0 {
		hd = r.read(int(min(n, int64(len(r.data)+1))))
		if int64(len(hd)) < n {
			return nil, errors.New("bmp: Truncated File Read")
		}
	}
	var width, height, bits, compression, colors int64
	var palettePadding int64
	direction := -1
	var rMask, gMask, bMask, aMask uint32
	haveColors := false
	switch headerSize {
	case 12:
		width = int64(le16(hd[0:]))
		height = int64(le16(hd[2:]))
		bits = int64(le16(hd[6:]))
		compression = compRaw
		palettePadding = 3
	case 40, 52, 56, 64, 108, 124:
		yFlip := hd[7] == 0xFF
		if yFlip {
			direction = 1
		}
		width = int64(le32(hd[0:]))
		if yFlip {
			height = 1<<32 - int64(le32(hd[4:]))
		} else {
			height = int64(le32(hd[4:]))
		}
		bits = int64(le16(hd[10:]))
		compression = int64(le32(hd[12:]))
		colors = int64(le32(hd[28:]))
		haveColors = true
		palettePadding = 4
		if compression == compBitfields {
			if len(hd) >= 48 {
				if len(hd) >= 52 {
					aMask = le32(hd[48:])
				}
				rMask, gMask, bMask = le32(hd[36:]), le32(hd[40:]), le32(hd[44:])
			} else {
				for _, m := range []*uint32{&rMask, &gMask, &bMask} {
					b := r.read(4)
					if len(b) < 4 {
						return nil, structError("bitfields mask")
					}
					*m = le32(b)
				}
			}
		}
	default:
		return nil, fmt.Errorf("bmp: Unsupported BMP header type (%d)", headerSize)
	}
	if !haveColors || colors == 0 {
		if bits >= 62 {
			colors = -1 // 1 << bits, larger than any test below cares about
		} else {
			colors = 1 << uint(bits)
		}
	}
	if bmp && offset == 14+headerSize && bits <= 8 {
		offset += palettePadding * colors
	}
	m, ok := bit2mode[int(bits)]
	if !ok {
		return nil, fmt.Errorf("bmp: Unsupported BMP pixel depth (%d)", bits)
	}
	mode, rawMode := m[0], m[1]
	rle := false
	switch compression {
	case compBitfields:
		switch bits {
		case 32:
			key := mask4{rMask, gMask, bMask, aMask}
			found := false
			for _, s := range supported32 {
				if s == key {
					found = true
				}
			}
			if !found {
				return nil, errors.New("bmp: Unsupported BMP bitfields layout")
			}
			rawMode = maskModes32[key]
			for _, c := range rawMode {
				if c == 'A' {
					mode = "RGBA"
				}
			}
		case 24:
			if (mask4{rMask, gMask, bMask}) != (mask4{0xFF0000, 0xFF00, 0xFF}) {
				return nil, errors.New("bmp: Unsupported BMP bitfields layout")
			}
			rawMode = "BGR"
		case 16:
			switch (mask4{rMask, gMask, bMask}) {
			case mask4{0xF800, 0x7E0, 0x1F}:
				rawMode = "BGR;16"
			case mask4{0x7C00, 0x3E0, 0x1F}:
				rawMode = "BGR;15"
			default:
				return nil, errors.New("bmp: Unsupported BMP bitfields layout")
			}
		default:
			return nil, errors.New("bmp: Unsupported BMP bitfields layout")
		}
	case compRaw:
	case compRLE8, compRLE4:
		rle = true
	default:
		return nil, fmt.Errorf("bmp: Unsupported BMP compression (%d)", compression)
	}
	o := &opened{data: r.data, width: width, height: height, direction: direction, rle: rle, rle4: compression == compRLE4}
	if mode == "P" {
		if !(0 < colors && colors <= 65536) {
			return nil, fmt.Errorf("bmp: Unsupported BMP Palette size (%d)", colors)
		}
		palette := r.read(int(palettePadding * colors))
		grayscale := true
		for ind := int64(0); ind < colors; ind++ {
			val := ind
			if colors == 2 {
				val = []int64{0, 255}[ind]
			}
			start := ind * palettePadding
			var rgb []byte
			if start < int64(len(palette)) {
				rgb = palette[start:min(start+3, int64(len(palette)))]
			}
			v := byte(val)
			if len(rgb) != 3 || rgb[0] != v || rgb[1] != v || rgb[2] != v {
				grayscale = false
			}
		}
		if grayscale {
			if colors == 2 {
				mode = "1"
			} else {
				mode = "L"
			}
			rawMode = mode
		} else {
			mode = "P"
			o.paletteRaw = palette
			o.paletteBpp = int(palettePadding)
		}
	}
	o.mode = mode
	o.rawMode = rawMode
	if !rle {
		o.stride = ((width*bits + 31) >> 3) &^ 3
	}
	if offset != 0 {
		o.offset = int(min(offset, int64(len(r.data))+1))
	} else {
		o.offset = r.pos
	}
	// ImageFile.__init__ after _open.
	if width <= 0 || height <= 0 {
		return nil, pil.Next("not identified by this driver")
	}
	return o, nil
}

func (o *opened) Size() (int, int) { return int(o.width), int(o.height) }

var unpackBits = map[string]int{
	"P;1": 1, "P;4": 4, "P": 8, "1": 1, "L": 8,
	"BGR;15": 16, "BGR;16": 16, "BGR": 24,
	"BGRX": 32, "XBGR": 32, "BGXR": 32, "ABGR": 32, "RGBA": 32, "BGRA": 32, "BGAR": 32,
}

// validUnpacker lists the (mode, rawmode) pairs of Unpack.c this plugin
// can ask for.
func validUnpacker(mode, raw string) bool {
	switch mode {
	case "P":
		return raw == "P;1" || raw == "P;4" || raw == "P"
	case "1":
		return raw == "1"
	case "L":
		return raw == "L"
	case "RGB":
		switch raw {
		case "BGR;15", "BGR;16", "BGR", "BGRX", "XBGR", "BGXR":
			return true
		}
	case "RGBA":
		switch raw {
		case "BGR", "ABGR", "RGBA", "BGRA", "BGAR", "BGRX":
			return raw != "BGRX"
		}
	}
	return false
}

// unpack is the Unpack.c shuffler for rawmode, into Pix bytes of mode.
func unpack(raw string, dst, src []byte, pixels int) {
	switch raw {
	case "P;1":
		for x := 0; x < pixels; x++ {
			dst[x] = (src[x>>3] >> uint(7-x&7)) & 1
		}
	case "1":
		for x := 0; x < pixels; x++ {
			if src[x>>3]&(0x80>>uint(x&7)) != 0 {
				dst[x] = 255
			} else {
				dst[x] = 0
			}
		}
	case "P;4":
		for x := 0; x < pixels; x++ {
			dst[x] = (src[x>>1] >> uint(4-4*(x&1))) & 15
		}
	case "P", "L":
		copy(dst[:pixels], src[:pixels])
	case "BGR;15", "BGR;16":
		for x := 0; x < pixels; x++ {
			p := int(src[2*x]) | int(src[2*x+1])<<8
			b := (p & 31) * 255 / 31
			var g, r int
			if raw == "BGR;15" {
				g = ((p >> 5) & 31) * 255 / 31
				r = ((p >> 10) & 31) * 255 / 31
			} else {
				g = ((p >> 5) & 63) * 255 / 63
				r = ((p >> 11) & 31) * 255 / 31
			}
			dst[3*x], dst[3*x+1], dst[3*x+2] = byte(r), byte(g), byte(b)
		}
	case "BGR":
		for x := 0; x < pixels; x++ {
			dst[3*x], dst[3*x+1], dst[3*x+2] = src[3*x+2], src[3*x+1], src[3*x]
		}
	default:
		// 32-bit layouts: indices of R, G, B and A (-1 for 255) in the input.
		idx := map[string][4]int{
			"BGRX": {2, 1, 0, -1}, "XBGR": {3, 2, 1, -1}, "BGXR": {3, 1, 0, -1},
			"ABGR": {3, 2, 1, 0}, "RGBA": {0, 1, 2, 3}, "BGRA": {2, 1, 0, 3}, "BGAR": {3, 1, 0, 2},
		}[raw]
		stride := len(dst) / pixels
		for x := 0; x < pixels; x++ {
			s := src[4*x : 4*x+4]
			d := dst[stride*x : stride*x+stride]
			d[0], d[1], d[2] = s[idx[0]], s[idx[1]], s[idx[2]]
			if stride == 4 {
				if idx[3] < 0 {
					d[3] = 255
				} else {
					d[3] = s[idx[3]]
				}
			}
		}
	}
}

// rawDecode is ImagingRawDecode fed with everything after offset; done is
// false where ImageFile.load would read again and find the file truncated.
func rawDecode(pix []byte, w, h int, raw string, stride int64, ystep int, data []byte) (done bool, err error) {
	bits := unpackBits[raw]
	bytesPerLine := (w*bits + 7) / 8
	skip := 0
	if stride != 0 {
		if stride-int64(bytesPerLine) < 0 {
			return true, errors.New("bmp: decoder error (config)")
		}
		skip = int(stride) - bytesPerLine
	}
	bpp := len(pix) / (w * h)
	y := 0
	if ystep < 0 {
		y, ystep = h-1, -1
	} else {
		ystep = 1
	}
	ptr := 0
	first := true
	for {
		if !first {
			if len(data)-ptr < skip {
				return false, nil
			}
			ptr += skip
		}
		first = false
		if len(data)-ptr < bytesPerLine {
			return false, nil
		}
		unpack(raw, pix[y*w*bpp:(y+1)*w*bpp], data[ptr:ptr+bytesPerLine], w)
		ptr += bytesPerLine
		y += ystep
		if y < 0 || y >= h {
			return true, nil
		}
	}
}

// Load is ImageFile.load for the raw or bmp_rle tile.
func (o *opened) Load() (*pil.Image, error) {
	w, h := int(o.width), int(o.height)
	img := &pil.Image{Format: "BMP", Mode: o.mode, Width: w, Height: h}
	bpp := pil.BytesPerPixel(o.mode)
	if o.mode == "P" {
		bits := o.paletteBpp * 8
		entries := len(o.paletteRaw) * 8 / bits
		if entries > 256 {
			return nil, errors.New("bmp: invalid palette size")
		}
		img.PaletteMode = "RGB"
		img.Palette = make([]byte, 3*entries)
		for i := 0; i < entries; i++ {
			s := o.paletteRaw[i*o.paletteBpp:]
			img.Palette[3*i], img.Palette[3*i+1], img.Palette[3*i+2] = s[2], s[1], s[0]
		}
	}
	if w <= 0 || h <= 0 {
		return nil, errors.New("bmp: tile cannot extend outside image")
	}
	img.Pix = make([]byte, w*h*bpp)
	data := []byte{}
	if o.offset <= len(o.data) {
		data = o.data[o.offset:]
	}
	if o.rle {
		return img, o.loadRLE(img, data)
	}
	if !validUnpacker(o.mode, o.rawMode) {
		return nil, errors.New("bmp: unknown raw mode for given image mode")
	}
	if o.stride > 1<<31-1 {
		return nil, errors.New("bmp: stride overflows C int")
	}
	if len(data) == 0 {
		return nil, errors.New("bmp: image file is truncated")
	}
	done, err := rawDecode(img.Pix, w, h, o.rawMode, o.stride, o.direction, data)
	if err != nil {
		return nil, err
	}
	if !done {
		return nil, errors.New("bmp: image file is truncated")
	}
	return img, nil
}

// loadRLE is BmpRleDecoder.decode followed by set_as_raw.
func (o *opened) loadRLE(img *pil.Image, data []byte) error {
	w, h := img.Width, img.Height
	pos := o.offset
	rd := func(n int) []byte {
		if pos >= len(o.data) {
			return nil
		}
		end := min(pos+n, len(o.data))
		b := o.data[pos:end]
		pos = end
		return b
	}
	var out []byte
	x := 0
	dest := w * h
	for len(out) < dest {
		pixels := rd(1)
		b := rd(1)
		if len(pixels) == 0 || len(b) == 0 {
			break
		}
		num := int(pixels[0])
		if num != 0 {
			if x+num > w {
				num = max(0, w-x)
			}
			if o.rle4 {
				first, second := b[0]>>4, b[0]&0x0F
				for i := 0; i < num; i++ {
					if i%2 == 0 {
						out = append(out, first)
					} else {
						out = append(out, second)
					}
				}
			} else {
				for i := 0; i < num; i++ {
					out = append(out, b[0])
				}
			}
			x += num
			continue
		}
		switch b[0] {
		case 0:
			for len(out)%w != 0 {
				out = append(out, 0)
			}
			x = 0
		case 1:
			goto finished
		case 2:
			d := rd(2)
			if len(d) < 2 {
				goto finished
			}
			n := int(d[0]) + int(d[1])*w
			out = append(out, make([]byte, n)...)
			x = len(out) % w
		default:
			var got []byte
			count := int(b[0])
			if o.rle4 {
				count = int(b[0]) / 2
				got = rd(count)
				for _, v := range got {
					out = append(out, v>>4, v&0x0F)
				}
			} else {
				got = rd(count)
				out = append(out, got...)
			}
			if len(got) < count {
				goto finished
			}
			x += int(b[0])
			if pos%2 != 0 {
				pos++
			}
		}
	}
finished:
	raw := "P"
	if o.mode == "L" {
		raw = "L"
	}
	if !validUnpacker(o.mode, raw) {
		return errors.New("bmp: unknown raw mode for given image mode")
	}
	done, err := rawDecode(img.Pix, w, h, raw, 0, o.direction, out)
	if err != nil {
		return err
	}
	if !done {
		return errors.New("bmp: not enough image data")
	}
	return nil
}
