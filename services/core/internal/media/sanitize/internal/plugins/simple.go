package plugins

import (
	"bytes"
	"math/big"
	"regexp"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

func acceptMCIDAS(p []byte) bool { return hasPrefix(p, "\x00\x00\x00\x00\x00\x00\x00\x04") }

func openMCIDAS(f *file) (pil.Opened, error) {
	s := f.read(256)
	if !acceptMCIDAS(s) || len(s) != 256 {
		return nil, raise("SyntaxError", "not an McIdas area file")
	}
	w := make([]int64, 65)
	for i := 1; i <= 64; i++ {
		w[i], _ = s32be(s, 4*(i-1))
	}
	var mode, rawmode string
	switch w[11] {
	case 1:
		mode, rawmode = "L", "L"
	case 2:
		mode, rawmode = "I;16B", "I;16B"
	case 4:
		mode, rawmode = "I", "I;32B"
	default:
		return nil, raise("SyntaxError", "unsupported McIdas format")
	}
	width, height := w[10], w[9]
	offset := w[34] + w[15]
	stride := new(big.Int).Mul(big.NewInt(w[10]), big.NewInt(w[11]))
	stride.Mul(stride, big.NewInt(w[14]))
	stride.Add(stride, big.NewInt(w[15]))
	tiles := []tile{{decoder: "raw", x1: width, y1: height, offset: offset, rawmode: rawmode, stride: clampBig(stride), ystep: 1}}
	return finish(mode, width, height, func() (*pil.Image, error) {
		return loadRaster(f, "MCIDAS", mode, width, height, nil, tiles, nil)
	})
}

func acceptPIXAR(p []byte) bool { return hasPrefix(p, "\x80\xe8\x00\x00") }

func openPIXAR(f *file) (pil.Opened, error) {
	s := append([]byte(nil), f.read(4)...)
	if !acceptPIXAR(s) {
		return nil, raise("SyntaxError", "not a PIXAR file")
	}
	s = append(s, f.read(508)...)
	w, err := u16le(s, 418)
	if err != nil {
		return nil, err
	}
	h, _ := u16le(s, 416)
	m0, err := u16le(s, 424)
	if err != nil {
		return nil, err
	}
	m1, err := u16le(s, 426)
	if err != nil {
		return nil, err
	}
	mode := ""
	if m0 == 14 && m1 == 2 {
		mode = "RGB"
	}
	tiles := []tile{{decoder: "raw", x1: w, y1: h, offset: 1024, rawmode: mode}}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadRaster(f, "PIXAR", mode, w, h, nil, tiles, nil)
	})
}

var xvPalette = func() []byte {
	var p []byte
	for r := 0; r < 8; r++ {
		for g := 0; g < 8; g++ {
			for b := 0; b < 4; b++ {
				p = append(p, byte(r*255/7), byte(g*255/7), byte(b*255/3))
			}
		}
	}
	return p
}()

func acceptXVTHUMB(p []byte) bool { return hasPrefix(p, "P7 332") }

func openXVTHUMB(f *file) (pil.Opened, error) {
	if !acceptXVTHUMB(f.read(6)) {
		return nil, raise("SyntaxError", "not an XV thumbnail file")
	}
	f.readline()
	var s []byte
	for {
		s = f.readline()
		if len(s) == 0 {
			return nil, raise("SyntaxError", "Unexpected EOF reading XV thumbnail file")
		}
		if s[0] != '#' {
			break
		}
	}
	fields := asciiFields(s)
	if len(fields) < 2 {
		return nil, raise("ValueError", "not enough values to unpack")
	}
	w, ok := pyInt(fields[0], false)
	if !ok {
		return nil, raise("ValueError", "invalid literal for int()")
	}
	h, ok := pyInt(fields[1], false)
	if !ok {
		return nil, raise("ValueError", "invalid literal for int()")
	}
	pal := &pyPalette{mode: "RGB", rawmode: "RGB", data: xvPalette}
	tiles := []tile{{decoder: "raw", x1: w, y1: h, offset: f.tell(), rawmode: "P"}}
	return finish("P", w, h, func() (*pil.Image, error) {
		return loadRaster(f, "XVThumb", "P", w, h, pal, tiles, nil)
	})
}

func acceptSUN(p []byte) bool {
	v, err := u32be(p, 0)
	return err == nil && v == 0x59A66A95
}

func openSUN(f *file) (pil.Opened, error) {
	s := f.read(32)
	if !acceptSUN(s) {
		return nil, raise("SyntaxError", "not an SUN raster file")
	}
	vals := make([]int64, 8)
	for _, i := range []int{1, 2, 3, 5, 6, 7} {
		v, err := u32be(s, 4*i)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	w, h, depth, fileType, palType, palLen := vals[1], vals[2], vals[3], vals[5], vals[6], vals[7]
	offset := int64(32)
	var mode, rawmode string
	switch depth {
	case 1:
		mode, rawmode = "1", "1;I"
	case 4:
		mode, rawmode = "L", "L;4"
	case 8:
		mode, rawmode = "L", "L"
	case 24:
		mode, rawmode = "RGB", "BGR"
		if fileType == 3 {
			rawmode = "RGB"
		}
	case 32:
		mode, rawmode = "RGB", "BGRX"
		if fileType == 3 {
			rawmode = "RGBX"
		}
	default:
		return nil, raise("SyntaxError", "Unsupported Mode/Bit Depth")
	}
	var pal *pyPalette
	if palLen != 0 {
		if palLen > 1024 {
			return nil, raise("SyntaxError", "Unsupported Color Palette Length")
		}
		if palType != 1 {
			return nil, raise("SyntaxError", "Unsupported Palette Type")
		}
		offset += palLen
		pal = &pyPalette{mode: "RGB", rawmode: "RGB;L", data: f.read(palLen)}
		if mode == "L" {
			mode = "P"
			rawmode = string(bytes.ReplaceAll([]byte(rawmode), []byte("L"), []byte("P")))
		}
	}
	stride := (w*depth + 15) / 16 * 2
	var t tile
	switch fileType {
	case 0, 1, 3, 4, 5:
		t = tile{decoder: "raw", x1: w, y1: h, offset: offset, rawmode: rawmode, stride: stride}
	case 2:
		t = tile{decoder: "sun_rle", x1: w, y1: h, offset: offset, rawmode: rawmode}
	default:
		return nil, raise("SyntaxError", "Unsupported Sun Raster file type")
	}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadRaster(f, "SUN", mode, w, h, pal, []tile{t}, nil)
	})
}

var sgiModes = map[[3]int64]string{
	{1, 1, 1}: "L", {1, 2, 1}: "L", {2, 1, 1}: "L;16B", {2, 2, 1}: "L;16B",
	{1, 3, 3}: "RGB", {2, 3, 3}: "RGB;16B", {1, 3, 4}: "RGBA", {2, 3, 4}: "RGBA;16B",
}

func acceptSGI(p []byte) bool {
	v, err := u16be(p, 0)
	return err == nil && v == 474
}

func openSGI(f *file) (pil.Opened, error) {
	s := f.read(512)
	if !acceptSGI(s) {
		return nil, raise("ValueError", "Not an SGI image file")
	}
	compression, err := at(s, 2)
	if err != nil {
		return nil, err
	}
	bpc, err := at(s, 3)
	if err != nil {
		return nil, err
	}
	var dims [4]int64
	for i := range dims {
		if dims[i], err = u16be(s, 4+2*i); err != nil {
			return nil, err
		}
	}
	rawmode, ok := sgiModes[[3]int64{bpc, dims[0], dims[3]}]
	if !ok {
		return nil, raise("ValueError", "Unsupported SGI image mode")
	}
	w, h := dims[1], dims[2]
	mode := rawmode
	if i := bytes.IndexByte([]byte(rawmode), ';'); i >= 0 {
		mode = rawmode[:i]
	}
	var load func() (*pil.Image, error)
	switch {
	case compression == 0 && bpc == 2:
		load = func() (*pil.Image, error) { return loadSGI16(f, mode, w, h) }
	case compression == 0:
		var tiles []tile
		offset := int64(512)
		for _, layer := range mode {
			tiles = append(tiles, tile{decoder: "raw", x1: w, y1: h, offset: offset, rawmode: string(layer), ystep: -1})
			offset += w * h * bpc
		}
		load = func() (*pil.Image, error) { return loadRaster(f, "SGI", mode, w, h, nil, tiles, nil) }
	case compression == 1:
		load = func() (*pil.Image, error) { return loadSGIRle(f, mode, rawmode, w, h, bpc) }
	default:
		load = loadFails(raise("OSError", "cannot load this image"))
	}
	return finish(mode, w, h, load)
}

func loadSGI16(f *file, mode string, w, h int64) (*pil.Image, error) {
	c, err := newCanvas(mode, w, h)
	if err != nil {
		return nil, err
	}
	_ = f.seek(512, 0)
	for b := 0; b < len(mode); b++ {
		channel, err := newCanvas("L", w, h)
		if err != nil {
			return nil, err
		}
		if err := rawInto(channel, "L;16B", f.read(2*w*h), -1); err != nil {
			return nil, err
		}
		if c.ps == 1 {
			copy(c.pix, channel.pix)
			continue
		}
		for i := int64(0); i < w*h; i++ {
			c.pix[4*i+int64(b)] = channel.pix[i]
		}
	}
	return c.image("SGI"), nil
}

// loadSGIRle is ImagingSgiRleDecode, which pulls the whole file itself.
func loadSGIRle(f *file, mode, rawmode string, w, h, bpc int64) (*pil.Image, error) {
	c, err := newCanvas(mode, w, h)
	if err != nil {
		return nil, err
	}
	st, err := decoderFor(c, tile{decoder: "sgi_rle", x1: w, y1: h, rawmode: rawmode})
	if err != nil {
		return nil, err
	}
	bands := int64(len(mode))
	bufsize := f.size() - 512
	tablen := bands * h
	if bufsize < 8*tablen {
		return nil, raise("OSError", "decoder error -1 when reading image file")
	}
	ptr := f.data[512:]
	buffer := make([]byte, w*bands*2)
	be := func(i int64) int64 {
		return int64(ptr[i])<<24 | int64(ptr[i+1])<<16 | int64(ptr[i+2])<<8 | int64(ptr[i+3])
	}
	end := bufsize - 1
	y := h - 1
	code := 0
rows:
	for row := int64(0); row < h; row, y = row+1, y-1 {
		for ch := int64(0); ch < bands; ch++ {
			start := be(4 * (row + ch*h))
			length := int64(int32(uint32(be(4*tablen + 4*(row+ch*h)))))
			if start < 512 {
				code = codecOverrun
				break rows
			}
			status := sgiExpand(buffer, ch, ptr, start-512, length, bands, w, end, bpc == 2)
			if status == -1 {
				code = codecOverrun
				break rows
			}
			if status == 1 {
				break rows
			}
		}
		st.u.fn(c.row(int(y)), buffer, int(w))
	}
	if code < 0 {
		return nil, raise("OSError", "decoder error %d when reading image file", code)
	}
	return c.image("SGI"), nil
}

func sgiExpand(dest []byte, ch int64, buf []byte, src, n, z, xsize, end int64, wide bool) int {
	x := int64(0)
	d := ch
	if wide {
		d = ch * 2
	}
	for ; n > 0; n-- {
		var pixel byte
		if wide {
			if src+1 > end {
				return -1
			}
			pixel = buf[src+1]
			src += 2
		} else {
			if src > end {
				return -1
			}
			pixel = buf[src]
			src++
		}
		if n == 1 && pixel != 0 {
			return 1
		}
		count := int64(pixel & 0x7f)
		if count == 0 {
			return 0
		}
		if x+count > xsize {
			return -1
		}
		x += count
		if wide {
			if pixel&0x80 != 0 {
				if src+2*count > end {
					return -1
				}
				for ; count > 0; count-- {
					copy(dest[d:d+2], buf[src:src+2])
					src += 2
					d += z * 2
				}
			} else {
				if src+2 > end {
					return -1
				}
				for ; count > 0; count-- {
					copy(dest[d:d+2], buf[src:src+2])
					d += z * 2
				}
				src += 2
			}
			continue
		}
		if pixel&0x80 != 0 {
			if src+count > end {
				return -1
			}
			for ; count > 0; count-- {
				dest[d] = buf[src]
				src++
				d += z
			}
		} else {
			if src > end {
				return -1
			}
			pixel = buf[src]
			src++
			for ; count > 0; count-- {
				dest[d] = pixel
				d += z
			}
		}
	}
	return 0
}

func acceptQOI(p []byte) bool { return hasPrefix(p, "qoif") }

func openQOI(f *file) (pil.Opened, error) {
	if !acceptQOI(f.read(4)) {
		return nil, raise("SyntaxError", "not a QOI file")
	}
	w, err := u32be(f.read(4), 0)
	if err != nil {
		return nil, err
	}
	h, err := u32be(f.read(4), 0)
	if err != nil {
		return nil, err
	}
	channels, err := at(f.read(1), 0)
	if err != nil {
		return nil, err
	}
	mode := "RGBA"
	if channels == 3 {
		mode = "RGB"
	}
	_ = f.seek(1, 1)
	offset := f.tell()
	return finish(mode, w, h, func() (*pil.Image, error) {
		c, err := newCanvas(mode, w, h)
		if err != nil {
			return nil, err
		}
		_ = f.seek(offset, 0)
		bands := int64(len(mode))
		dest := w * h * bands
		data := make([]byte, 0, dest)
		prev := []byte{0, 0, 0, 255}
		seen := map[int][]byte{}
		for int64(len(data)) < dest {
			b, err := at(f.read(1), 0)
			if err != nil {
				return nil, err
			}
			var value []byte
			switch op := b >> 6; {
			case b == 254:
				value = append(append([]byte(nil), f.read(3)...), prev[3:]...)
			case b == 255:
				value = append([]byte(nil), f.read(4)...)
			case op == 0:
				if v, ok := seen[int(b&63)]; ok {
					value = v
				} else {
					value = []byte{0, 0, 0, 0}
				}
			case op == 1:
				value = []byte{
					byte(int(prev[0]) + int(b&48>>4) - 2),
					byte(int(prev[1]) + int(b&12>>2) - 2),
					byte(int(prev[2]) + int(b&3) - 2),
					prev[3],
				}
			case op == 2:
				second, err := at(f.read(1), 0)
				if err != nil {
					return nil, err
				}
				dg := int(b&63) - 32
				dr := int(second&240>>4) - 8
				db := int(second&15) - 8
				value = []byte{byte(int(prev[0]) + dg + dr), byte(int(prev[1]) + dg), byte(int(prev[2]) + dg + db), prev[3]}
			default:
				run := int(b&63) + 1
				v := prev
				if bands == 3 {
					v = v[:3]
				}
				for i := 0; i < run; i++ {
					data = append(data, v...)
				}
				continue
			}
			if len(value) != 4 {
				return nil, raise("ValueError", "not enough values to unpack")
			}
			prev = value
			seen[(int(value[0])*3+int(value[1])*5+int(value[2])*7+int(value[3])*11)%64] = value
			if bands == 3 {
				value = value[:3]
			}
			data = append(data, value...)
		}
		if err := rawInto(c, mode, data, 1); err != nil {
			return nil, err
		}
		return c.image("QOI"), nil
	})
}

func acceptMSP(p []byte) bool { return hasPrefix(p, "DanM", "LinS") }

func openMSP(f *file) (pil.Opened, error) {
	s := f.read(32)
	if !acceptMSP(s) {
		return nil, raise("SyntaxError", "not an MSP file")
	}
	var sum int64
	for i := 0; i < 32; i += 2 {
		v, err := u16le(s, i)
		if err != nil {
			return nil, err
		}
		sum ^= v
	}
	if sum != 0 {
		return nil, raise("SyntaxError", "bad MSP checksum")
	}
	w, _ := u16le(s, 4)
	h, _ := u16le(s, 6)
	if hasPrefix(s, "DanM") {
		tiles := []tile{{decoder: "raw", x1: w, y1: h, offset: 32, rawmode: "1"}}
		return finish("1", w, h, func() (*pil.Image, error) {
			return loadRaster(f, "MSP", "1", w, h, nil, tiles, nil)
		})
	}
	return finish("1", w, h, func() (*pil.Image, error) {
		c, err := newCanvas("1", w, h)
		if err != nil {
			return nil, err
		}
		blank := bytes.Repeat([]byte{255}, int((w+7)/8))
		_ = f.seek(32, 0)
		rowmap := f.read(h * 2)
		if int64(len(rowmap)) < h*2 {
			return nil, raise("OSError", "Truncated MSP file in row map")
		}
		var img []byte
		for x := int64(0); x < h; x++ {
			rowlen := int64(rowmap[2*x]) | int64(rowmap[2*x+1])<<8
			if rowlen == 0 {
				img = append(img, blank...)
				continue
			}
			row := f.read(rowlen)
			if int64(len(row)) != rowlen {
				return nil, raise("OSError", "Truncated MSP file, expected %d bytes on row %d", rowlen, x)
			}
			for idx := int64(0); idx < rowlen; {
				runtype := row[idx]
				idx++
				if runtype == 0 {
					if idx+2 > rowlen {
						return nil, raise("OSError", "Corrupted MSP file in row %d", x)
					}
					img = append(img, bytes.Repeat([]byte{row[idx+1]}, int(row[idx]))...)
					idx += 2
				} else {
					end := min(idx+int64(runtype), rowlen)
					img = append(img, row[idx:end]...)
					idx += int64(runtype)
				}
			}
		}
		if err := rawInto(c, "1", img, 1); err != nil {
			return nil, err
		}
		return c.image("MSP"), nil
	})
}

var xbmHead = regexp.MustCompile(`^[\t\n\v\f\r ]*#define[ \t]+.*_width[ \t]+(?P<width>[0-9]+)[\r\n]+#define[ \t]+.*_height[ \t]+(?P<height>[0-9]+)[\r\n]+(?P<hotspot>#define[ \t]+[^_]*_x_hot[ \t]+(?P<xhot>[0-9]+)[\r\n]+#define[ \t]+[^_]*_y_hot[ \t]+(?P<yhot>[0-9]+)[\r\n]+)?(?s:.)*_bits\[\]`)

func acceptXBM(p []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(p, " \t\n\r\x0b\x0c"), []byte("#define"))
}

func openXBM(f *file) (pil.Opened, error) {
	head := f.read(512)
	text := latin1(head)
	loc := xbmHead.FindStringSubmatchIndex(text)
	if loc == nil {
		return nil, raise("SyntaxError", "not a XBM file")
	}
	runes := func(byteIndex int) int64 { return int64(len([]rune(text[:byteIndex]))) }
	w, _ := pyInt([]byte(text[loc[2]:loc[3]]), false)
	h, _ := pyInt([]byte(text[loc[4]:loc[5]]), false)
	end := runes(loc[1])
	tiles := []tile{{decoder: "xbm", x1: w, y1: h, offset: end}}
	return finish("1", w, h, func() (*pil.Image, error) {
		return loadRaster(f, "XBM", "1", w, h, nil, tiles, nil)
	})
}

var xpmHead = regexp.MustCompile(`^"([0-9]*) ([0-9]*) ([0-9]*) ([0-9]*)`)

func acceptXPM(p []byte) bool { return hasPrefix(p, "/* XPM */") }

func pySlice(n, start, end int64) (int64, int64) {
	clamp := func(i int64) int64 {
		if i < 0 {
			i += n
			if i < 0 {
				i = 0
			}
		}
		if i > n {
			i = n
		}
		return i
	}
	start, end = clamp(start), clamp(end)
	if end < start {
		end = start
	}
	return start, end
}

func openXPM(f *file) (pil.Opened, error) {
	if !acceptXPM(f.read(9)) {
		return nil, raise("SyntaxError", "not an XPM file")
	}
	var m [][]byte
	for {
		line := f.readline()
		if len(line) == 0 {
			return nil, raise("SyntaxError", "broken XPM file")
		}
		if m = xpmHead.FindSubmatch(line); m != nil {
			break
		}
	}
	var nums [4]int64
	for i := range nums {
		v, ok := pyInt(m[i+1], false)
		if !ok {
			return nil, raise("ValueError", "invalid literal for int() with base 10: ''")
		}
		nums[i] = v
	}
	w, h, palLen, bpp := nums[0], nums[1], nums[2], nums[3]
	type entry struct {
		key   string
		value []byte
	}
	var keys []string
	palette := map[string][]byte{}
	var transparency []byte
	hasTransparency := false
	for i := int64(0); i < palLen; i++ {
		line := bytes.TrimRight(f.readline(), " \t\n\r\x0b\x0c")
		n := int64(len(line))
		cs, ce := pySlice(n, 1, bpp+1)
		c := line[cs:ce]
		ss, se := pySlice(n, bpp+1, -2)
		fields := asciiFields(line[ss:se])
		found := false
		for j := 0; j < len(fields); j += 2 {
			if string(fields[j]) != "c" {
				continue
			}
			if j+1 >= len(fields) {
				return nil, indexErr()
			}
			rgb := fields[j+1]
			switch {
			case string(rgb) == "None":
				transparency, hasTransparency = append([]byte(nil), c...), true
			case bytes.HasPrefix(rgb, []byte("#")):
				v, ok := pyIntBase16(rgb[1:])
				if !ok {
					return nil, raise("ValueError", "invalid literal for int() with base 16")
				}
				low := new(big.Int).And(v, big.NewInt(0xFFFFFF)).Int64()
				if _, seen := palette[string(c)]; !seen {
					keys = append(keys, string(c))
				}
				palette[string(c)] = []byte{byte(low >> 16), byte(low >> 8), byte(low)}
			default:
				return nil, raise("ValueError", "cannot read this XPM file")
			}
			found = true
			break
		}
		if !found {
			return nil, raise("ValueError", "cannot read this XPM file")
		}
	}
	mode := "P"
	var pal *pyPalette
	if palLen > 256 {
		mode = "RGB"
	} else {
		var joined []byte
		for _, k := range keys {
			joined = append(joined, palette[k]...)
		}
		pal = &pyPalette{mode: "RGB", rawmode: "RGB", data: joined}
	}
	offset := f.tell()
	return finish(mode, w, h, func() (*pil.Image, error) {
		c, err := newCanvas(mode, w, h)
		if err != nil {
			return nil, err
		}
		if pal != nil {
			if err := realizePalette(c, pal); err != nil {
				return nil, err
			}
		}
		_ = f.seek(offset, 0)
		dest := w * h
		if mode == "RGB" {
			dest *= 3
		}
		var data []byte
		header := false
		for int64(len(data)) < dest {
			line := f.readline()
			if len(line) == 0 {
				break
			}
			if string(bytes.TrimRight(line, " \t\n\r\x0b\x0c")) == "/* pixels */" && !header {
				header = true
				continue
			}
			parts := bytes.Split(line, []byte(`"`))
			if len(parts) > 2 {
				line = bytes.Join(parts[1:len(parts)-1], []byte(`"`))
			} else {
				line = nil
			}
			if bpp == 0 {
				return nil, raise("ValueError", "range() arg 3 must not be zero")
			}
			for i := int64(0); i < int64(len(line)); i += bpp {
				key := string(line[i:min(i+bpp, int64(len(line)))])
				if mode == "RGB" {
					v, ok := palette[key]
					if !ok {
						return nil, raise("KeyError", "%q", key)
					}
					data = append(data, v...)
					continue
				}
				index := -1
				for k, name := range keys {
					if name == key {
						index = k
						break
					}
				}
				if index < 0 {
					return nil, raise("ValueError", "tuple.index(x): x not in tuple")
				}
				data = append(data, byte(index))
			}
		}
		if bpp == 0 && len(data) == 0 && dest > 0 {
			return nil, raise("ValueError", "not enough image data")
		}
		if err := rawInto(c, mode, data, 1); err != nil {
			return nil, err
		}
		img := c.image("XPM")
		if hasTransparency {
			img.Info.Transparency = &pil.Transparency{Kind: pil.TransparencyBytes, Bytes: transparency}
		}
		return img, nil
	})
}

// pyIntBase16 is int(b, 16).
func pyIntBase16(s []byte) (*big.Int, bool) {
	s = trimSpace(s)
	for _, c := range s {
		if c >= 0x80 {
			return nil, false
		}
	}
	neg := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
		if len(s) > 0 && s[0] == '_' {
			s = s[1:]
		}
	}
	if len(s) == 0 {
		return nil, false
	}
	var digits []byte
	for i, c := range s {
		isHex := c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
		if c == '_' {
			if i == 0 || i == len(s)-1 || s[i-1] == '_' {
				return nil, false
			}
			continue
		}
		if !isHex {
			return nil, false
		}
		digits = append(digits, c)
	}
	v, ok := new(big.Int).SetString(string(digits), 16)
	if !ok {
		return nil, false
	}
	if neg {
		v.Neg(v)
	}
	return v, true
}

func acceptGBR(p []byte) bool {
	a, err := u32be(p, 0)
	if err != nil || len(p) < 8 {
		return false
	}
	b, _ := u32be(p, 4)
	return a >= 20 && (b == 1 || b == 2)
}

func openGBR(f *file) (pil.Opened, error) {
	next := func() (int64, error) { return u32be(f.read(4), 0) }
	hs, err := next()
	if err != nil {
		return nil, err
	}
	if hs < 20 {
		return nil, raise("SyntaxError", "not a GIMP brush")
	}
	version, err := next()
	if err != nil {
		return nil, err
	}
	if version != 1 && version != 2 {
		return nil, raise("SyntaxError", "Unsupported GIMP brush version")
	}
	var v [3]int64
	for i := range v {
		if v[i], err = next(); err != nil {
			return nil, err
		}
	}
	w, h, depth := v[0], v[1], v[2]
	if w == 0 || h == 0 {
		return nil, raise("SyntaxError", "not a GIMP brush")
	}
	if depth != 1 && depth != 4 {
		return nil, raise("SyntaxError", "Unsupported GIMP brush color depth")
	}
	commentLen := hs - 20
	if version == 2 {
		commentLen = hs - 28
		if string(f.read(4)) != "GIMP" {
			return nil, raise("SyntaxError", "not a GIMP brush, bad magic number")
		}
		if _, err := next(); err != nil {
			return nil, err
		}
	}
	f.read(commentLen)
	mode := "L"
	if depth == 4 {
		mode = "RGBA"
	}
	if err := bombCheck(w, h); err != nil {
		return nil, err
	}
	dataSize := w * h * depth
	return finish(mode, w, h, func() (*pil.Image, error) {
		c, err := newCanvas(mode, w, h)
		if err != nil {
			return nil, err
		}
		if err := rawInto(c, mode, f.read(dataSize), 1); err != nil {
			return nil, err
		}
		return c.image("GBR"), nil
	})
}

func acceptFTEX(p []byte) bool { return hasPrefix(p, "FTEX") }

func openFTEX(f *file) (pil.Opened, error) {
	if !acceptFTEX(f.read(4)) {
		return nil, raise("SyntaxError", "not an FTEX file")
	}
	if len(f.read(4)) != 4 {
		return nil, structErr()
	}
	pair := func() (int64, int64, error) {
		b := f.read(8)
		if len(b) != 8 {
			return 0, 0, structErr()
		}
		a, _ := s32le(b, 0)
		c, _ := s32le(b, 4)
		return a, c, nil
	}
	w, h, err := pair()
	if err != nil {
		return nil, err
	}
	_, formats, err := pair()
	if err != nil {
		return nil, err
	}
	if formats != 1 {
		return nil, raise("AssertionError", "")
	}
	format, where, err := pair()
	if err != nil {
		return nil, err
	}
	if err := f.seek(where, 0); err != nil {
		return nil, err
	}
	b := f.read(4)
	if len(b) != 4 {
		return nil, structErr()
	}
	size, _ := s32le(b, 0)
	data := f.read(size)
	switch format {
	case 0:
		return finish("RGBA", w, h, bcnLoad("FTEX", &file{data: data}, 0, w, h, 8))
	case 1:
		inner := &file{data: data}
		tiles := []tile{{decoder: "raw", x1: w, y1: h, rawmode: "RGB"}}
		return finish("RGB", w, h, func() (*pil.Image, error) {
			return loadRaster(inner, "FTEX", "RGB", w, h, nil, tiles, nil)
		})
	}
	return nil, raise("ValueError", "Invalid texture compression format")
}
