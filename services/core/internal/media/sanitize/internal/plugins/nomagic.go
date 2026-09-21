package plugins

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// The six plugins registered without an accept function see every upload
// that reaches them: TGA, SPIDER, IM, IMT, IPTC and PCD.

var tgaModes = map[[2]int64]string{
	{1, 8}: "P", {3, 1}: "1", {3, 8}: "L", {3, 16}: "LA",
	{2, 16}: "BGRA;15Z", {2, 24}: "BGR", {2, 32}: "BGRA",
}

func openTGA(f *file) (pil.Opened, error) {
	s := f.read(18)
	fields := make([]int64, 18)
	for _, i := range []int{0, 1, 2, 16, 17} {
		v, err := at(s, i)
		if err != nil {
			return nil, err
		}
		fields[i] = v
	}
	idLen, colormap, imagetype, depth, flags := fields[0], fields[1], fields[2], fields[16], fields[17]
	w, _ := u16le(s, 12)
	h, _ := u16le(s, 14)
	if (colormap != 0 && colormap != 1) || w <= 0 || h <= 0 ||
		(depth != 1 && depth != 8 && depth != 16 && depth != 24 && depth != 32) {
		return nil, raise("SyntaxError", "not a TGA file")
	}
	var mode string
	switch imagetype {
	case 3, 11:
		mode = "L"
		if depth == 1 {
			mode = "1"
		} else if depth == 16 {
			mode = "LA"
		}
	case 1, 9:
		mode = "L"
		if colormap == 1 {
			mode = "P"
		}
	case 2, 10:
		mode = "RGBA"
		if depth == 24 {
			mode = "RGB"
		}
	default:
		return nil, raise("SyntaxError", "unknown TGA mode")
	}
	orientation := flags & 0x30
	flip := orientation == 0x10 || orientation == 0x30
	ystep := int64(-1)
	if orientation == 0x20 || orientation == 0x30 {
		ystep = 1
	}
	if idLen > 0 {
		f.read(idLen)
	}
	var pal *pyPalette
	if colormap == 1 {
		start, _ := u16le(s, 3)
		size, _ := u16le(s, 5)
		switch s[7] {
		case 16:
			pal = &pyPalette{mode: "RGBA", rawmode: "BGRA;15Z", data: append(make([]byte, 2*start), f.read(2*size)...)}
		case 24:
			pal = &pyPalette{mode: "RGB", rawmode: "BGR", data: append(make([]byte, 3*start), f.read(3*size)...)}
		case 32:
			pal = &pyPalette{mode: "RGB", rawmode: "BGRA", data: append(make([]byte, 4*start), f.read(4*size)...)}
		default:
			return nil, raise("SyntaxError", "unknown TGA map depth")
		}
	}
	var tiles []tile
	if rawmode, ok := tgaModes[[2]int64{imagetype & 7, depth}]; ok {
		t := tile{decoder: "raw", x1: w, y1: h, offset: f.tell(), rawmode: rawmode, ystep: ystep}
		if imagetype&8 != 0 {
			t.decoder, t.depth = "tga_rle", depth
		}
		tiles = []tile{t}
	}
	loadEnd := func(c *canvas) error {
		if mode == "RGBA" {
			_ = f.seek(-26, 2)
			footer := f.read(26)
			if bytes.HasSuffix(footer, []byte("TRUEVISION-XFILE.\x00")) {
				ext, _ := u32le(footer, 0)
				if ext != 0 {
					_ = f.seek(ext+494, 0)
					if string(f.read(1)) == "\x00" {
						for i := 3; i < len(c.pix); i += 4 {
							c.pix[i] = 255
						}
					}
				}
			}
		}
		if flip {
			for y := 0; y < c.h; y++ {
				row := c.row(y)
				for l, r := 0, c.w-1; l < r; l, r = l+1, r-1 {
					for k := 0; k < c.ps; k++ {
						row[l*c.ps+k], row[r*c.ps+k] = row[r*c.ps+k], row[l*c.ps+k]
					}
				}
			}
		}
		return nil
	}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadRaster(f, "TGA", mode, w, h, pal, tiles, loadEnd)
	})
}

// SPIDER headers are 27 float32 values; the integer checks run on the
// exact float values, so they use big numbers.

func spiderFloats(raw []byte, bigEndian bool) []float64 {
	out := make([]float64, 28)
	out[0] = 99
	for i := 0; i < 27; i++ {
		var bits uint32
		if bigEndian {
			bits = uint32(raw[4*i])<<24 | uint32(raw[4*i+1])<<16 | uint32(raw[4*i+2])<<8 | uint32(raw[4*i+3])
		} else {
			bits = uint32(raw[4*i+3])<<24 | uint32(raw[4*i+2])<<16 | uint32(raw[4*i+1])<<8 | uint32(raw[4*i])
		}
		out[i+1] = float64(math.Float32frombits(bits))
	}
	return out
}

func isIntegral(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Trunc(v) }

func bigOf(v float64) *big.Int {
	b, _ := new(big.Float).SetFloat64(math.Trunc(v)).Int(nil)
	return b
}

// floatInt is int(v) for a float: ValueError for nan, OverflowError for inf.
func floatInt(v float64) (int64, error) {
	if math.IsNaN(v) {
		return 0, raise("ValueError", "cannot convert float NaN to integer")
	}
	if math.IsInf(v, 0) {
		return 0, raise("OverflowError", "cannot convert float infinity to integer")
	}
	return clampBig(bigOf(v)), nil
}

func spiderHeader(h []float64) *big.Int {
	for _, i := range []int{1, 2, 5, 12, 13, 22, 23} {
		if !isIntegral(h[i]) {
			return new(big.Int)
		}
	}
	switch bigOf(h[5]).Int64() {
	case 1, 3, -11, -12, -21, -22:
	default:
		if !bigOf(h[5]).IsInt64() || true {
			return new(big.Int)
		}
	}
	labrec, labbyt, lenbyt := bigOf(h[13]), bigOf(h[22]), bigOf(h[23])
	if labbyt.Cmp(new(big.Int).Mul(labrec, lenbyt)) != 0 {
		return new(big.Int)
	}
	return labbyt
}

func openSPIDER(f *file) (pil.Opened, error) {
	raw := f.read(108)
	if len(raw) != 108 {
		return nil, raise("SyntaxError", "not a valid Spider file")
	}
	h := spiderFloats(raw, true)
	hdr := spiderHeader(h)
	bigEndian := true
	if hdr.Sign() == 0 {
		h = spiderFloats(raw, false)
		hdr = spiderHeader(h)
		bigEndian = false
	}
	if hdr.Sign() == 0 {
		return nil, raise("SyntaxError", "not a valid Spider file")
	}
	if bigOf(h[5]).Int64() != 1 || !bigOf(h[5]).IsInt64() {
		return nil, raise("SyntaxError", "not a Spider 2D image")
	}
	w, height := clampBig(bigOf(h[12])), clampBig(bigOf(h[2]))
	istack, err := floatInt(h[24])
	if err != nil {
		return nil, err
	}
	imgnumber, err := floatInt(h[27])
	if err != nil {
		return nil, err
	}
	hdrlen := clampBig(hdr)
	var offset int64
	switch {
	case istack == 0 && imgnumber == 0:
		offset = hdrlen
	case istack > 0 && imgnumber == 0:
		if _, err := floatInt(h[26]); err != nil {
			return nil, err
		}
		offset = clampBig(new(big.Int).Mul(hdr, big.NewInt(2)))
	case istack == 0 && imgnumber > 0:
		return nil, raise("AttributeError", "'SpiderImageFile' object has no attribute 'stkoffset'")
	default:
		return nil, raise("SyntaxError", "inconsistent stack header values")
	}
	rawmode := "F;32F"
	if bigEndian {
		rawmode = "F;32BF"
	}
	tiles := []tile{{decoder: "raw", x1: w, y1: height, offset: offset, rawmode: rawmode}}
	return finish("F", w, height, func() (*pil.Image, error) {
		return loadRaster(f, "SPIDER", "F", w, height, nil, tiles, nil)
	})
}

// IM: the IFUNC text header.

const (
	imComment = "Comment"
	imFrames  = "File size (no of images)"
	imLUT     = "Lut"
	imScale   = "Scale (x,y)"
	imSize    = "Image size (x*y)"
	imMode    = "Image type"
)

var imTags = map[string]bool{
	imComment: true, "Date": true, "Digitalization equipment": true, imFrames: true,
	imLUT: true, "Name": true, imScale: true, imSize: true, imMode: true,
}

var imOpen = map[string][2]string{
	"0 1 image": {"1", "1"}, "L 1 image": {"1", "1"}, "Greyscale image": {"L", "L"},
	"Grayscale image": {"L", "L"}, "RGB image": {"RGB", "RGB;L"}, "RLB image": {"RGB", "RLB"},
	"RYB image": {"RGB", "RLB"}, "B1 image": {"1", "1"}, "B2 image": {"P", "P;2"},
	"B4 image": {"P", "P;4"}, "X 24 image": {"RGB", "RGB"}, "L 32 S image": {"I", "I;32"},
	"L 32 F image": {"F", "F;32"}, "RGB3 image": {"RGB", "RGB;T"}, "RYB3 image": {"RGB", "RYB;T"},
	"LA image": {"LA", "LA;L"}, "PA image": {"LA", "PA;L"}, "RGBA image": {"RGBA", "RGBA;L"},
	"RGBX image": {"RGB", "RGBX;L"}, "CMYK image": {"CMYK", "CMYK;L"}, "YCC image": {"YCbCr", "YCbCr;L"},
}

func init() {
	for _, i := range []string{"8", "8S", "16", "16S", "32", "32F"} {
		imOpen["L "+i+" image"] = [2]string{"F", "F;" + i}
		imOpen["L*"+i+" image"] = [2]string{"F", "F;" + i}
	}
	for _, i := range []string{"16", "16L", "16B"} {
		imOpen["L "+i+" image"] = [2]string{"I;" + i, "I;" + i}
		imOpen["L*"+i+" image"] = [2]string{"I;" + i, "I;" + i}
	}
	imOpen["L 32S image"] = [2]string{"I", "I;32S"}
	imOpen["L*32S image"] = [2]string{"I", "I;32S"}
	for j := 2; j <= 32; j++ {
		imOpen[fmt.Sprintf("L*%d image", j)] = [2]string{"F", fmt.Sprintf("F;%d", j)}
	}
}

// pyNumber is IM's number(): int(s), else float(s).
type pyNumber struct {
	isFloat bool
	i       int64
	f       float64
}

func imNumber(s []byte) (pyNumber, error) {
	if v, ok := pyInt(s, true); ok {
		return pyNumber{i: v}, nil
	}
	if v, ok := pyFloat(s, true); ok {
		return pyNumber{isFloat: true, f: v}, nil
	}
	return pyNumber{}, raise("ValueError", "could not convert string to float")
}

func (n pyNumber) float() float64 {
	if n.isFloat {
		return n.f
	}
	return float64(n.i)
}

func openIM(f *file) (pil.Opened, error) {
	if bytes.IndexByte(f.read(100), '\n') < 0 {
		return nil, raise("SyntaxError", "not an IM file")
	}
	_ = f.seek(0, 0)
	count := 0
	mode, rawmode := "L", "L"
	size := []pyNumber{{i: 512}, {i: 512}}
	sizeScalar := false
	lut := false
	var s []byte
	for {
		s = f.read(1)
		if len(s) == 1 && s[0] == '\r' {
			continue
		}
		if len(s) == 0 || s[0] == 0 || s[0] == 0x1a {
			break
		}
		s = append(append([]byte(nil), s...), f.readline()...)
		if len(s) > 100 {
			return nil, raise("SyntaxError", "not an IM file")
		}
		line := s
		if bytes.HasSuffix(line, []byte("\r\n")) {
			line = line[:len(line)-2]
		} else if bytes.HasSuffix(line, []byte("\n")) {
			line = line[:len(line)-1]
		}
		colon := bytes.IndexByte(line, ':')
		first := byte(0)
		if len(line) > 0 {
			first = line[0]
		}
		if colon < 0 || !(first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z') || bytes.IndexByte(line, '\n') >= 0 {
			return nil, raise("SyntaxError", "Syntax error in IM header")
		}
		k := string(line[:colon])
		v := line[colon+1:]
		for len(v) > 0 && (v[0] == ' ' || v[0] == '\t') {
			v = v[1:]
		}
		switch {
		case k == imFrames || k == imScale || k == imSize:
			parts := bytes.Split(bytes.ReplaceAll(v, []byte("*"), []byte(",")), []byte(","))
			nums := make([]pyNumber, len(parts))
			for i, part := range parts {
				n, err := imNumber(part)
				if err != nil {
					return nil, err
				}
				nums[i] = n
			}
			if k == imSize {
				size, sizeScalar = nums, len(nums) == 1
			}
		case k == imMode:
			if m, ok := imOpen[string(v)]; ok {
				mode, rawmode = m[0], m[1]
			} else {
				mode = latin1(v)
			}
		case k == imLUT:
			lut = true
		}
		if imTags[k] {
			count++
		}
	}
	if count == 0 {
		return nil, raise("SyntaxError", "Not an IM file")
	}
	for len(s) > 0 && s[0] != 0x1a {
		s = f.read(1)
	}
	if len(s) == 0 {
		return nil, raise("SyntaxError", "File truncated")
	}
	var pal *pyPalette
	lutTable := false
	if lut {
		palette := f.read(768)
		grey, linear := true, true
		for i := 0; i < 256; i++ {
			a, err := at(palette, i)
			if err != nil {
				return nil, err
			}
			b, err := at(palette, i+256)
			if err != nil {
				return nil, err
			}
			if a != b {
				grey = false
				continue
			}
			c, err := at(palette, i+512)
			if err != nil {
				return nil, err
			}
			if b == c {
				if a != int64(i) {
					linear = false
				}
			} else {
				grey = false
			}
		}
		switch mode {
		case "L", "LA", "P", "PA":
			if grey {
				lutTable = !linear
			} else {
				if mode == "L" || mode == "P" {
					mode, rawmode = "P", "P"
				} else {
					mode, rawmode = "PA", "PA;L"
				}
				pal = &pyPalette{mode: "RGB", rawmode: "RGB;L", data: palette}
			}
		case "RGB":
			lutTable = !grey || !linear
		}
	}
	offs := f.tell()
	var tiles []tile
	bitTile := false
	if strings.HasPrefix(rawmode, "F;") {
		if bits, ok := pyInt([]byte(rawmode[2:]), true); ok && bits != 8 && bits != 16 && bits != 32 {
			bitTile = true
		}
	}

	// ImageFile.__init__'s check and the bomb check on a size that may be
	// a scalar, longer than two, or hold floats.
	if sizeScalar {
		return nil, raise("TypeError", "'int' object is not subscriptable")
	}
	w0, h0 := size[0], size[1]
	if mode == "" || w0.float() <= 0 || h0.float() <= 0 {
		return nil, pil.Next("not identified by this driver")
	}
	if len(size) != 2 || w0.isFloat || h0.isFloat {
		wf, hf := math.Max(1, w0.float()), math.Max(1, h0.float())
		if math.IsNaN(w0.float()) {
			wf = 1
		}
		if math.IsNaN(h0.float()) {
			hf = 1
		}
		pixels := wf * hf
		if pixels > 2*pil.MaxImagePixels || pixels > pil.MaxImagePixels {
			return nil, &pil.BombError{Pixels: int64(math.Min(pixels, math.MaxInt64)), Warning: pixels <= 2*pil.MaxImagePixels}
		}
		// Python then fails: unpacking a size that is not a pair, or
		// Image.core.new rejecting floats. Only a pair of floats reaches
		// the sanitizer's pixel limit first.
		fw, fh := 1, 1
		if len(size) == 2 && w0.float()*h0.float() > 50_000_000 {
			fh = 50_000_001
		}
		return &opened{mode: mode, w: fw, h: fh, load: loadFails(raise("TypeError", "'float' object cannot be interpreted as an integer"))}, nil
	}
	w, h := w0.i, h0.i
	switch {
	case bitTile:
		return finish(mode, w, h, unsupported("IM", "bit decoder"))
	case lutTable:
		return finish(mode, w, h, unsupported("IM", "lut"))
	case rawmode == "RGB;T" || rawmode == "RYB;T":
		n := w * h
		tiles = []tile{
			{decoder: "raw", x1: w, y1: h, offset: offs, rawmode: "G", ystep: -1},
			{decoder: "raw", x1: w, y1: h, offset: offs + n, rawmode: "R", ystep: -1},
			{decoder: "raw", x1: w, y1: h, offset: offs + 2*n, rawmode: "B", ystep: -1},
		}
	default:
		tiles = []tile{{decoder: "raw", x1: w, y1: h, offset: offs, rawmode: rawmode, ystep: -1}}
	}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadRaster(f, "IM", mode, w, h, pal, tiles, nil)
	})
}

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// IMT: IM Tools text header.
func openIMT(f *file) (pil.Opened, error) {
	buffer := append([]byte(nil), f.read(100)...)
	if bytes.IndexByte(buffer, '\n') < 0 {
		return nil, raise("SyntaxError", "not an IM file")
	}
	var xsize, ysize, w, h int64
	mode := ""
	tileSet := false
	var t tile
	for {
		var s []byte
		if len(buffer) > 0 {
			s = []byte{buffer[0]}
			buffer = buffer[1:]
		} else {
			s = append([]byte(nil), f.read(1)...)
		}
		if len(s) == 0 {
			break
		}
		if s[0] == 0x0c {
			t = tile{decoder: "raw", x1: w, y1: h, offset: f.tell() - int64(len(buffer)), rawmode: mode}
			tileSet = true
			break
		}
		if bytes.IndexByte(buffer, '\n') < 0 {
			buffer = append(buffer, f.read(100)...)
		}
		nl := bytes.IndexByte(buffer, '\n')
		if nl < 0 {
			s = append(s, buffer...)
			buffer = nil
		} else {
			s = append(s, buffer[:nl]...)
			buffer = append([]byte(nil), buffer[nl+1:]...)
		}
		if len(s) == 1 || len(s) > 100 {
			break
		}
		if s[0] == '*' {
			continue
		}
		i := 0
		for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
			i++
		}
		if i >= len(s) || s[i] != ' ' {
			break
		}
		k := string(s[:i])
		j := i + 1
		for j < len(s) && s[j] != ' ' && s[j] != '\r' && s[j] != '\n' {
			j++
		}
		v := s[i+1 : j]
		switch {
		case k == "width":
			n, ok := pyInt(v, false)
			if !ok {
				return nil, raise("ValueError", "invalid literal for int()")
			}
			xsize = n
			w, h = xsize, ysize
		case k == "height":
			n, ok := pyInt(v, false)
			if !ok {
				return nil, raise("ValueError", "invalid literal for int()")
			}
			ysize = n
			w, h = xsize, ysize
		case k == "pixel" && string(v) == "n8":
			mode = "L"
		}
	}
	return finish(mode, w, h, func() (*pil.Image, error) {
		if !tileSet {
			return nil, raise("OSError", "cannot load this image")
		}
		return loadRaster(f, "IMT", mode, w, h, nil, []tile{t}, nil)
	})
}

// IPTC/NAA.

type iptcValue struct {
	none  bool
	data  []byte
	list  []iptcValue
	multi bool
}

func iptcField(f *file) (tag [2]int64, ok bool, size int64, err error) {
	s := f.read(5)
	if len(bytes.Trim(s, "\x00")) == 0 {
		return tag, false, 0, nil
	}
	t0, err := at(s, 1)
	if err != nil {
		return tag, false, 0, err
	}
	t1, err := at(s, 2)
	if err != nil {
		return tag, false, 0, err
	}
	tag = [2]int64{t0, t1}
	valid := s[0] == 28
	if valid {
		valid = false
		for _, v := range []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 240} {
			if t0 == v {
				valid = true
			}
		}
	}
	if !valid {
		return tag, false, 0, raise("SyntaxError", "invalid IPTC/NAA file")
	}
	sz, err := at(s, 3)
	if err != nil {
		return tag, false, 0, err
	}
	switch {
	case sz > 132:
		return tag, false, 0, raise("OSError", "illegal field length in IPTC/NAA file")
	case sz == 128:
		size = 0
	case sz > 128:
		c := append(make([]byte, 4), f.read(sz-128)...)
		size, _ = u32be(c, len(c)-4)
	default:
		size, err = u16be(s, 3)
		if err != nil {
			return tag, false, 0, err
		}
	}
	return tag, true, size, nil
}

// iptcIndex is value[i] on an info entry: None raises TypeError, bytes
// give an int, a list gives its element.
func iptcIndex(v iptcValue, i int) (int64, *iptcValue, error) {
	switch {
	case v.multi:
		if i >= len(v.list) {
			return 0, nil, indexErr()
		}
		return 0, &v.list[i], nil
	case v.none:
		return 0, nil, raise("TypeError", "'NoneType' object is not subscriptable")
	}
	b, err := at(v.data, i)
	return b, nil, err
}

func iptcInt(info map[[2]int64]iptcValue, key [2]int64) (int64, error) {
	v, ok := info[key]
	if !ok {
		return 0, raise("KeyError", "%v", key)
	}
	if v.none || v.multi {
		return 0, raise("TypeError", "can't concat")
	}
	c := append(make([]byte, 4), v.data...)
	n, _ := u32be(c, len(c)-4)
	return n, nil
}

func openIPTC(f *file) (pil.Opened, error) {
	info := map[[2]int64]iptcValue{}
	var offset int64
	var last [2]int64
	lastOK := false
	for {
		offset = f.tell()
		tag, ok, size, err := iptcField(f)
		if err != nil {
			return nil, err
		}
		last, lastOK = tag, ok
		if !ok || tag == [2]int64{8, 10} {
			break
		}
		value := iptcValue{none: size == 0}
		if size != 0 {
			value.data = f.read(size)
		}
		if old, seen := info[tag]; seen {
			if old.multi {
				old.list = append(old.list, value)
				info[tag] = old
			} else {
				info[tag] = iptcValue{multi: true, list: []iptcValue{old, value}}
			}
		} else {
			info[tag] = value
		}
	}
	v360, ok := info[[2]int64{3, 60}]
	if !ok {
		return nil, raise("KeyError", "(3, 60)")
	}
	layers, layersObj, err := iptcIndex(v360, 0)
	if err != nil {
		return nil, err
	}
	component, componentObj, err := iptcIndex(v360, 1)
	if err != nil {
		return nil, err
	}
	mode := ""
	band := int64(-1)
	hasBand := false
	numeric := layersObj == nil && componentObj == nil
	if numeric && layers == 1 && component == 0 {
		mode = "L"
	} else {
		if numeric && layers == 3 && component != 0 {
			mode = "RGB"
		} else if numeric && layers == 4 && component != 0 {
			mode = "CMYK"
		}
		hasBand = true
		band = 0
		if v365, ok := info[[2]int64{3, 65}]; ok {
			b, obj, err := iptcIndex(v365, 0)
			if err != nil {
				return nil, err
			}
			if obj != nil {
				return nil, raise("TypeError", "unsupported operand type(s) for -")
			}
			band = b - 1
		}
	}
	w, err := iptcInt(info, [2]int64{3, 20})
	if err != nil {
		return nil, err
	}
	h, err := iptcInt(info, [2]int64{3, 30})
	if err != nil {
		return nil, err
	}
	comp, err := iptcInt(info, [2]int64{3, 120})
	if err != nil {
		if excType(err) == "KeyError" {
			return nil, raise("OSError", "Unknown IPTC image compression")
		}
		return nil, err
	}
	if comp != 1 && comp != 5 {
		return nil, raise("OSError", "Unknown IPTC image compression")
	}
	tileSet := lastOK && last == [2]int64{8, 10}
	return finish(mode, w, h, func() (*pil.Image, error) {
		if !tileSet {
			return nil, raise("OSError", "cannot load this image")
		}
		if comp == 5 {
			return nil, pil.Unsupported("IPTC", "embedded JPEG")
		}
		_ = f.seek(offset, 0)
		var data []byte
		for {
			tag, ok, size, err := iptcField(f)
			if err != nil {
				return nil, err
			}
			if !ok || tag != [2]int64{8, 10} {
				break
			}
			for size > 0 {
				chunk := f.read(min(size, 8192))
				if len(chunk) == 0 {
					break
				}
				data = append(data, chunk...)
				size -= int64(len(chunk))
			}
		}
		// Image.open of "P5 w h 255" plus data: a raw L image.
		if int64(len(data)) < w*h {
			return nil, raise("OSError", "image file is truncated")
		}
		if !hasBand {
			c, err := newCanvas("L", w, h)
			if err != nil {
				return nil, err
			}
			copy(c.pix, data[:w*h])
			return c.image("IPTC"), nil
		}
		bands := map[string]int64{"RGB": 3, "CMYK": 4}[mode]
		if band >= bands || band < -bands {
			return nil, indexErr()
		}
		if band < 0 {
			band += bands
		}
		c, err := newCanvas(mode, w, h)
		if err != nil {
			return nil, err
		}
		for i := int64(0); i < w*h; i++ {
			c.pix[4*i+band] = data[i]
		}
		if mode == "RGB" {
			for i := int64(0); i < w*h; i++ {
				c.pix[4*i+3] = 255
			}
		}
		return c.image("IPTC"), nil
	})
}

func openPCD(f *file) (pil.Opened, error) {
	_ = f.seek(2048, 0)
	s := f.read(1539)
	if !hasPrefix(s, "PCD_") {
		return nil, raise("SyntaxError", "not a PCD file")
	}
	o, err := at(s, 1538)
	if err != nil {
		return nil, err
	}
	w, h := int64(768), int64(512)
	if o&3 == 1 || o&3 == 3 {
		w, h = 512, 768
	}
	return finish("RGB", w, h, func() (*pil.Image, error) {
		// ImagingPcdDecode takes 3*768 bytes per two rows from 96*2048.
		if f.size()-96*2048 < 256*3*768 {
			return nil, raise("OSError", "image file is truncated")
		}
		return nil, pil.Unsupported("PCD", "PhotoCD YCC unpacker")
	})
}
