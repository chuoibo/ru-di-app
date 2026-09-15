package plugins

import (
	"bytes"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

func acceptPCX(p []byte) bool {
	return len(p) >= 2 && p[0] == 10 && (p[1] == 0 || p[1] == 2 || p[1] == 3 || p[1] == 5)
}

type pcxHeader struct {
	mode string
	w, h int64
	pal  *pyPalette
	t    tile
}

// readPCX is PcxImageFile._open at the current position.
func readPCX(f *file) (pcxHeader, error) {
	var hd pcxHeader
	s := f.read(68)
	if !acceptPCX(s) {
		return hd, raise("SyntaxError", "not a PCX file")
	}
	var box [4]int64
	for i := range box {
		v, err := u16le(s, 4+2*i)
		if err != nil {
			return hd, err
		}
		box[i] = v
	}
	box[2]++
	box[3]++
	if box[2] <= box[0] || box[3] <= box[1] {
		return hd, raise("SyntaxError", "bad PCX image size")
	}
	offset := f.tell() + 60
	version := int64(s[1])
	bits, err := at(s, 3)
	if err != nil {
		return hd, err
	}
	planes, err := at(s, 65)
	if err != nil {
		return hd, err
	}
	provided, err := u16le(s, 66)
	if err != nil {
		return hd, err
	}
	var rawmode string
	switch {
	case bits == 1 && planes == 1:
		hd.mode, rawmode = "1", "1"
	case bits == 1 && (planes == 2 || planes == 4):
		hd.mode = "P"
		rawmode = map[int64]string{2: "P;2L", 4: "P;4L"}[planes]
		hd.pal = &pyPalette{mode: "RGB", rawmode: "RGB", data: s[16:64]}
	case version == 5 && bits == 8 && planes == 1:
		hd.mode, rawmode = "L", "L"
		_ = f.seek(-769, 2)
		tail := f.read(769)
		if len(tail) == 769 && tail[0] == 12 {
			for i := 0; i < 256; i++ {
				if tail[i*3+1] != byte(i) || tail[i*3+2] != byte(i) || tail[i*3+3] != byte(i) {
					hd.mode, rawmode = "P", "P"
					break
				}
			}
			if hd.mode == "P" {
				hd.pal = &pyPalette{mode: "RGB", rawmode: "RGB", data: tail[1:]}
			}
		}
	case version == 5 && bits == 8 && planes == 3:
		hd.mode, rawmode = "RGB", "RGB;L"
	default:
		return hd, raise("OSError", "unknown PCX mode")
	}
	hd.w, hd.h = box[2]-box[0], box[3]-box[1]
	stride := (hd.w*bits + 7) / 8
	if provided != stride {
		stride += stride % 2
	}
	hd.t = tile{decoder: "pcx", x1: hd.w, y1: hd.h, offset: offset, rawmode: rawmode, stride: planes * stride}
	return hd, nil
}

func pcxOpened(f *file, format string, hd pcxHeader) (pil.Opened, error) {
	return finish(hd.mode, hd.w, hd.h, func() (*pil.Image, error) {
		return loadRaster(f, format, hd.mode, hd.w, hd.h, hd.pal, []tile{hd.t}, nil)
	})
}

func openPCX(f *file) (pil.Opened, error) {
	hd, err := readPCX(f)
	if err != nil {
		return nil, err
	}
	return pcxOpened(f, "PCX", hd)
}

func acceptDCX(p []byte) bool {
	v, err := u32le(p, 0)
	return err == nil && v == 0x3ADE68B1
}

func openDCX(f *file) (pil.Opened, error) {
	if !acceptDCX(f.read(4)) {
		return nil, raise("SyntaxError", "not a DCX file")
	}
	var offsets []int64
	for i := 0; i < 1024; i++ {
		v, err := u32le(f.read(4), 0)
		if err != nil {
			return nil, err
		}
		if v == 0 {
			break
		}
		offsets = append(offsets, v)
	}
	if len(offsets) == 0 {
		return nil, raise("EOFError", "attempt to seek outside sequence")
	}
	_ = f.seek(offsets[0], 0)
	hd, err := readPCX(f)
	if err != nil {
		return nil, err
	}
	return pcxOpened(f, "DCX", hd)
}

func acceptFITS(p []byte) bool { return hasPrefix(p, "SIMPLE") }

func openFITS(f *file) (pil.Opened, error) {
	headers := map[string][]byte{}
	inProgress := false
	decoder := ""
	var offset, w, h int64
	mode := ""
	parse := func() error {
		prefix := ""
		name := "raw"
		offset = 0
		size := func(prefix string) (bool, int64, int64, error) {
			n, err := fitsInt(headers, prefix+"NAXIS")
			if err != nil {
				return false, 0, 0, err
			}
			switch n {
			case 0:
				return false, 0, 0, nil
			case 1:
				v, err := fitsInt(headers, prefix+"NAXIS1")
				return true, 1, v, err
			}
			a, err := fitsInt(headers, prefix+"NAXIS1")
			if err != nil {
				return false, 0, 0, err
			}
			b, err := fitsInt(headers, prefix+"NAXIS2")
			return true, a, b, err
		}
		if x, ok := headers["XTENSION"]; ok && string(x) == "'BINTABLE'" {
			if z, ok := headers["ZIMAGE"]; ok && string(z) == "T" {
				ct, ok := headers["ZCMPTYPE"]
				if !ok {
					return raise("KeyError", "ZCMPTYPE")
				}
				if string(ct) == "'GZIP_1  '" {
					_, a, b, err := size("")
					if err != nil {
						return err
					}
					bits, err := fitsInt(headers, "BITPIX")
					if err != nil {
						return err
					}
					offset = a * b * floorDiv(bits, 8)
					prefix, name = "Z", "fits_gzip"
				}
			}
		}
		ok, a, b, err := size(prefix)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		w, h = a, b
		bits, err := fitsInt(headers, prefix+"BITPIX")
		if err != nil {
			return err
		}
		switch bits {
		case 8:
			mode = "L"
		case 16:
			mode = "I;16"
		case 32:
			mode = "I"
		case -32, -64:
			mode = "F"
		}
		decoder = name
		return nil
	}
	for {
		header := f.read(80)
		if len(header) == 0 {
			return nil, raise("OSError", "Truncated FITS file")
		}
		keyword := string(trimSpace(header[:min(8, len(header))]))
		switch {
		case keyword == "SIMPLE" || keyword == "XTENSION":
			inProgress = true
		case len(headers) > 0 && !inProgress:
			goto done
		case keyword == "END":
			_ = f.seek((f.tell()+2879)/2880*2880, 0)
			if decoder == "" {
				if err := parse(); err != nil {
					return nil, err
				}
			}
			inProgress = false
			continue
		}
		if decoder != "" {
			continue
		}
		rest := header[min(8, len(header)):]
		if i := bytes.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		value := trimSpace(rest)
		if bytes.HasPrefix(value, []byte("=")) {
			value = trimSpace(value[1:])
		}
		if len(headers) == 0 && (!hasPrefix([]byte(keyword), "SIMPLE") || string(value) != "T") {
			return nil, raise("SyntaxError", "Not a FITS file")
		}
		headers[keyword] = value
	}
done:
	if decoder == "" {
		return nil, raise("ValueError", "No image data")
	}
	offset += f.tell() - 80
	if decoder == "fits_gzip" {
		return finish(mode, w, h, unsupported("FITS", "gzip tile compression"))
	}
	tiles := []tile{{decoder: "raw", x1: w, y1: h, offset: offset, rawmode: mode, ystep: -1}}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadRaster(f, "FITS", mode, w, h, nil, tiles, nil)
	})
}

func fitsInt(headers map[string][]byte, key string) (int64, error) {
	v, ok := headers[key]
	if !ok {
		return 0, raise("KeyError", "%s", key)
	}
	n, ok := pyInt(v, false)
	if !ok {
		return 0, raise("ValueError", "invalid literal for int()")
	}
	return n, nil
}

func acceptFLI(p []byte) bool {
	if len(p) < 16 {
		return false
	}
	magic, _ := u16le(p, 4)
	flags, _ := u16le(p, 14)
	return (magic == 0xAF11 || magic == 0xAF12) && (flags == 0 || flags == 3)
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func openFLI(f *file) (pil.Opened, error) {
	s := f.read(128)
	if !(acceptFLI(s) && len(s) == 128 && allZero(s[20:22]) && allZero(s[42:80]) && allZero(s[88:])) {
		return nil, raise("SyntaxError", "not an FLI/FLC file")
	}
	frames, _ := u16le(s, 6)
	w, _ := u16le(s, 8)
	h, _ := u16le(s, 10)
	s = f.read(16)
	kind, err := u16le(s, 4)
	if err != nil {
		return nil, err
	}
	if kind == 0xF100 {
		off, _ := u32le(s, 0)
		_ = f.seek(128+off, 0)
		s = f.read(16)
	}
	if kind, err = u16le(s, 4); err != nil {
		return nil, err
	}
	if kind == 0xF1FA {
		subchunks, err := u16le(s, 6)
		if err != nil {
			return nil, err
		}
		chunkSize := int64(-1)
		for i := int64(0); i < subchunks; i++ {
			if chunkSize >= 0 {
				_ = f.seek(chunkSize-6, 1)
			}
			s = f.read(6)
			chunkType, err := u16le(s, 4)
			if err != nil {
				return nil, err
			}
			if chunkType == 4 || chunkType == 11 {
				if err := fliPalette(f); err != nil {
					return nil, err
				}
				break
			}
			chunkSize, _ = u32le(s, 0)
			if chunkSize == 0 {
				break
			}
		}
	}
	if frames == 0 {
		return nil, raise("EOFError", "attempt to seek outside sequence")
	}
	_ = f.seek(128, 0)
	head := f.read(4)
	if len(head) == 0 {
		return nil, raise("EOFError", "missing frame size")
	}
	framesize, err := u32le(head, 0)
	if err != nil {
		return nil, err
	}
	return finish("P", w, h, func() (*pil.Image, error) {
		// ImageFile.load reads framesize bytes at a time from offset 128;
		// ImagingFliDecode waits for four bytes and the whole frame.
		data := f.data[min(128, len(f.data)):]
		n := int64(len(data))
		if framesize == 0 || n < 4 || n+n%2 < framesize {
			return nil, raise("OSError", "image file is truncated")
		}
		if n < 8 {
			return nil, raise("OSError", "decoder error -1 when reading image file")
		}
		if magic, _ := u16le(data, 4); magic != 0xF1FA {
			return nil, raise("OSError", "decoder error -3 when reading image file")
		}
		return nil, pil.Unsupported("FLI", "FLI frame decoder")
	})
}

func fliPalette(f *file) error {
	count, err := u16le(f.read(2), 0)
	if err != nil {
		return err
	}
	i := int64(0)
	for e := int64(0); e < count; e++ {
		s := f.read(2)
		skip, err := at(s, 0)
		if err != nil {
			return err
		}
		i += skip
		n, err := at(s, 1)
		if err != nil {
			return err
		}
		if n == 0 {
			n = 256
		}
		s = f.read(n * 3)
		for k := 0; k < len(s); k += 3 {
			if _, err := at(s, k+1); err != nil {
				return err
			}
			if _, err := at(s, k+2); err != nil {
				return err
			}
			if i >= 256 {
				return indexErr()
			}
			i++
		}
	}
	return nil
}

var psdModes = map[[2]int64]struct {
	mode     string
	channels int64
}{
	{0, 1}: {"1", 1}, {0, 8}: {"L", 1}, {1, 8}: {"L", 1}, {2, 8}: {"P", 1},
	{3, 8}: {"RGB", 3}, {4, 8}: {"CMYK", 4}, {7, 8}: {"L", 1}, {8, 8}: {"L", 1}, {9, 8}: {"LAB", 3},
}

func acceptPSD(p []byte) bool { return hasPrefix(p, "8BPS") }

func openPSD(f *file) (pil.Opened, error) {
	s := f.read(26)
	if !acceptPSD(s) {
		return nil, raise("SyntaxError", "not a PSD file")
	}
	if v, err := u16be(s, 4); err != nil {
		return nil, err
	} else if v != 1 {
		return nil, raise("SyntaxError", "not a PSD file")
	}
	bits, err := u16be(s, 22)
	if err != nil {
		return nil, err
	}
	channels, _ := u16be(s, 12)
	psdMode, err := u16be(s, 24)
	if err != nil {
		return nil, err
	}
	m, ok := psdModes[[2]int64{psdMode, bits}]
	if !ok {
		return nil, raise("KeyError", "(%d, %d)", psdMode, bits)
	}
	if m.channels > channels {
		return nil, raise("OSError", "not enough channels")
	}
	mode, ch := m.mode, m.channels
	if mode == "RGB" && channels == 4 {
		mode, ch = "RGBA", 4
	}
	w, _ := u32be(s, 18)
	h, _ := u32be(s, 14)
	u32 := func() (int64, error) { return u32be(f.read(4), 0) }
	size, err := u32()
	if err != nil {
		return nil, err
	}
	var pal *pyPalette
	if size != 0 {
		data := f.read(size)
		if mode == "P" && size == 768 {
			pal = &pyPalette{mode: "RGB", rawmode: "RGB;L", data: data}
		}
	}
	if size, err = u32(); err != nil {
		return nil, err
	}
	if size != 0 {
		end := f.tell() + size
		for f.tell() < end {
			f.read(4)
			if _, err := u16be(f.read(2), 0); err != nil {
				return nil, err
			}
			n, err := at(f.read(1), 0)
			if err != nil {
				return nil, err
			}
			name := f.read(n)
			if len(name)&1 == 0 {
				f.read(1)
			}
			dl, err := u32()
			if err != nil {
				return nil, err
			}
			if len(f.read(dl))&1 == 1 {
				f.read(1)
			}
		}
	}
	if size, err = u32(); err != nil {
		return nil, err
	}
	if size != 0 {
		end := f.tell() + size
		if _, err := u32(); err != nil {
			return nil, err
		}
		_ = f.seek(end, 0)
	}
	// _maketile
	compression, err := u16be(f.read(2), 0)
	if err != nil {
		return nil, err
	}
	offset := f.tell()
	layer := func(c int64) string {
		name := string(mode[c])
		if mode == "CMYK" {
			name += ";I"
		}
		return name
	}
	var tiles []tile
	switch compression {
	case 0:
		for c := int64(0); c < ch; c++ {
			tiles = append(tiles, tile{decoder: "raw", x1: w, y1: h, offset: offset, rawmode: layer(c)})
			offset += w * h
		}
	case 1:
		counts := f.read(ch * h * 2)
		offset = f.tell()
		i := 0
		for c := int64(0); c < ch; c++ {
			tiles = append(tiles, tile{decoder: "packbits", x1: w, y1: h, offset: offset, rawmode: layer(c)})
			for y := int64(0); y < h; y++ {
				n, err := u16be(counts, i)
				if err != nil {
					return nil, err
				}
				offset += n
				i += 2
			}
		}
	}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadRaster(f, "PSD", mode, w, h, pal, tiles, nil)
	})
}

func fourCC(s string) int64 {
	v, _ := u32le([]byte(s), 0)
	return v
}

func acceptDDS(p []byte) bool { return hasPrefix(p, "DDS ") }

func openDDS(f *file) (pil.Opened, error) {
	if !acceptDDS(f.read(4)) {
		return nil, raise("SyntaxError", "not a DDS file")
	}
	hs, err := u32le(f.read(4), 0)
	if err != nil {
		return nil, err
	}
	if hs != 124 {
		return nil, raise("OSError", "Unsupported header size")
	}
	header := f.read(120)
	if len(header) != 120 {
		return nil, raise("OSError", "Incomplete header")
	}
	h, _ := u32le(header, 4)
	w, _ := u32le(header, 8)
	pfflags, _ := u32le(header, 72)
	cc, _ := u32le(header, 76)
	bitcount, _ := u32le(header, 80)
	bcn := func(mode string, block int64) (pil.Opened, error) {
		return finish(mode, w, h, bcnLoad("DDS", f, f.tell(), w, h, block))
	}
	raw := func(mode string, pal *pyPalette) (pil.Opened, error) {
		tiles := []tile{{decoder: "raw", x1: w, y1: h, offset: f.tell(), rawmode: mode}}
		return finish(mode, w, h, func() (*pil.Image, error) {
			return loadRaster(f, "DDS", mode, w, h, pal, tiles, nil)
		})
	}
	switch {
	case pfflags&0x40 != 0:
		count := 3
		mode := "RGB"
		if pfflags&1 != 0 {
			mode, count = "RGBA", 4
		}
		masks := make([]int64, count)
		for i := range masks {
			masks[i], _ = u32le(header, 84+4*i)
		}
		offset := f.tell()
		return finish(mode, w, h, func() (*pil.Image, error) {
			return loadDDSRGB(f, offset, mode, w, h, bitcount, masks)
		})
	case pfflags&0x20000 != 0:
		switch {
		case bitcount == 8:
			return raw("L", nil)
		case bitcount == 16 && pfflags&1 != 0:
			return raw("LA", nil)
		}
		return nil, raise("OSError", "Unsupported bitcount")
	case pfflags&0x20 != 0:
		pal := &pyPalette{mode: "RGBA", rawmode: "RGBA", data: f.read(1024)}
		return raw("P", pal)
	case pfflags&4 != 0:
		switch cc {
		case fourCC("DXT1"):
			return bcn("RGBA", 8)
		case fourCC("DXT3"), fourCC("DXT5"):
			return bcn("RGBA", 16)
		case fourCC("BC4U"), fourCC("ATI1"):
			return bcn("L", 8)
		case fourCC("BC5S"), fourCC("BC5U"), fourCC("ATI2"):
			return bcn("RGB", 16)
		case fourCC("DX10"):
			dxgi, err := u32le(f.read(4), 0)
			if err != nil {
				return nil, err
			}
			f.read(16)
			switch dxgi {
			case 70, 71:
				return bcn("RGBA", 8)
			case 73, 74, 76, 77, 97, 98, 99:
				return bcn("RGBA", 16)
			case 79, 80:
				return bcn("L", 8)
			case 82, 83, 84, 95, 96:
				return bcn("RGB", 16)
			case 27, 28, 29:
				return raw("RGBA", nil)
			}
			return nil, raise("NotImplementedError", "Unimplemented DXGI format")
		}
		return nil, raise("NotImplementedError", "Unimplemented pixel format")
	}
	return nil, raise("NotImplementedError", "Unknown pixel format flags")
}

// bcnLoad is ImageFile.load for a "bcn" tile: the decoder places fixed-size
// blocks until ceil(w/4)*ceil(h/4) are done, so less data is truncation.
// Block decoding itself is not ported.
func bcnLoad(format string, f *file, pos, w, h, block int64) func() (*pil.Image, error) {
	return func() (*pil.Image, error) {
		available := f.size() - pos
		if available < ((w+3)/4)*((h+3)/4)*block {
			return nil, raise("OSError", "image file is truncated")
		}
		return nil, pil.Unsupported(format, "BCn texture")
	}
}

// loadDDSRGB is DdsRgbDecoder: masked little-endian pixels scaled to 8 bits.
func loadDDSRGB(f *file, offset int64, mode string, w, h, bitcount int64, masks []int64) (*pil.Image, error) {
	c, err := newCanvas(mode, w, h)
	if err != nil {
		return nil, err
	}
	_ = f.seek(offset, 0)
	shifts := make([]uint, len(masks))
	totals := make([]int64, len(masks))
	for i, mask := range masks {
		var s uint
		if mask != 0 {
			for (mask>>(s+1))<<(s+1) == mask {
				s++
			}
		}
		shifts[i], totals[i] = s, mask>>s
	}
	count := bitcount / 8
	dest := w * h * int64(len(masks))
	data := make([]byte, 0, dest)
	for int64(len(data)) < dest {
		var value int64
		for i, b := range f.read(count) {
			if i < 8 {
				value |= int64(b) << (8 * i)
			}
		}
		for i, mask := range masks {
			if totals[i] == 0 {
				data = append(data, 0)
				continue
			}
			ratio := float64((value&mask)>>shifts[i]) / float64(totals[i]) * 255
			data = append(data, byte(int64(ratio)))
		}
	}
	if err := rawInto(c, mode, data, 1); err != nil {
		return nil, err
	}
	return c.image("DDS"), nil
}

func acceptBLP(p []byte) bool { return hasPrefix(p, "BLP1", "BLP2") }

func openBLP(f *file) (pil.Opened, error) {
	magic := f.read(4)
	if !acceptBLP(magic) {
		return nil, raise("NotImplementedError", "Bad BLP magic")
	}
	b := f.read(4)
	if len(b) != 4 {
		return nil, structErr()
	}
	compression, _ := s32le(b, 0)
	blp1 := string(magic) == "BLP1"
	var alpha bool
	var encoding, alphaEncoding int64
	if blp1 {
		b := f.read(4)
		if len(b) != 4 {
			return nil, structErr()
		}
		v, _ := u32le(b, 0)
		alpha = v != 0
	} else {
		var v [3]int64
		for i := range v {
			b := f.read(1)
			if len(b) != 1 {
				return nil, structErr()
			}
			v[i] = int64(int8(b[0]))
		}
		encoding, alpha, alphaEncoding = v[0], v[1] != 0, v[2]
		_ = f.seek(1, 1)
	}
	b = f.read(8)
	if len(b) != 8 {
		return nil, structErr()
	}
	w, _ := u32le(b, 0)
	h, _ := u32le(b, 4)
	offset := int64(20)
	if blp1 {
		b := f.read(4)
		if len(b) != 4 {
			return nil, structErr()
		}
		encoding, _ = s32le(b, 0)
		offset = 28
	}
	mode := "RGB"
	if alpha {
		mode = "RGBA"
	}
	return finish(mode, w, h, func() (*pil.Image, error) {
		return loadBLP(f, blp1, offset, compression, encoding, alphaEncoding, alpha, mode, w, h)
	})
}

// loadBLP is BLP1Decoder / BLP2Decoder, which pull the file themselves.
func loadBLP(f *file, blp1 bool, offset, compression, encoding, alphaEncoding int64, alpha bool, mode string, w, h int64) (*pil.Image, error) {
	c, err := newCanvas(mode, w, h)
	if err != nil {
		return nil, err
	}
	_ = f.seek(offset, 0)
	safeRead := func(n int64) ([]byte, error) {
		if n <= 0 {
			return nil, nil
		}
		b := f.read(n)
		if int64(len(b)) < n {
			return nil, raise("OSError", "Truncated File Read")
		}
		return b, nil
	}
	head, err := safeRead(64)
	if err != nil {
		return nil, err
	}
	offset0, _ := u32le(head, 0)
	lengths, err := safeRead(64)
	if err != nil {
		return nil, err
	}
	length0, _ := u32le(lengths, 0)
	readPalette := func() ([][4]byte, error) {
		palette := make([][4]byte, 256)
		for i := range palette {
			b, err := safeRead(4)
			if err != nil {
				return nil, err
			}
			copy(palette[i][:], b)
		}
		return palette, nil
	}
	readBGRA := func(palette [][4]byte) ([]byte, error) {
		indices, err := safeRead(length0)
		if err != nil {
			return nil, err
		}
		var data []byte
		for _, i := range indices {
			e := palette[i]
			data = append(data, e[2], e[1], e[0])
			if alpha {
				data = append(data, e[3])
			}
		}
		return data, nil
	}
	notImplemented := raise("NotImplementedError", "Unsupported BLP encoding or compression")
	var data []byte
	if blp1 {
		switch {
		case compression == 0:
			size, err := safeRead(4)
			if err != nil {
				return nil, err
			}
			n, _ := u32le(size, 0)
			if _, err := safeRead(n); err != nil {
				return nil, err
			}
			if _, err := safeRead(offset0 - f.tell()); err != nil {
				return nil, err
			}
			if _, err := safeRead(length0); err != nil {
				return nil, err
			}
			return nil, pil.Unsupported("BLP", "JPEG-compressed BLP1")
		case compression == 1 && (encoding == 4 || encoding == 5):
			palette, err := readPalette()
			if err != nil {
				return nil, err
			}
			if data, err = readBGRA(palette); err != nil {
				return nil, err
			}
		default:
			return nil, notImplemented
		}
	} else {
		palette, err := readPalette()
		if err != nil {
			return nil, err
		}
		_ = f.seek(offset0, 0)
		if compression != 1 {
			return nil, notImplemented
		}
		switch encoding {
		case 1:
			if data, err = readBGRA(palette); err != nil {
				return nil, err
			}
		case 2:
			var unit int64
			var decode func([]byte) [4][]byte
			switch alphaEncoding {
			case 0:
				unit, decode = 8, func(b []byte) [4][]byte { return dxt1(b, alpha) }
			case 1:
				unit, decode = 16, dxt3
			case 7:
				unit, decode = 16, dxt5
			default:
				return nil, notImplemented
			}
			line := (w + 3) / 4 * unit
			for yb := int64(0); yb < (h+3)/4; yb++ {
				b, err := safeRead(line)
				if err != nil {
					return nil, err
				}
				for _, row := range decode(b) {
					data = append(data, row...)
				}
			}
		default:
			return nil, notImplemented
		}
	}
	if err := rawInto(c, mode, data, 1); err != nil {
		return nil, err
	}
	return c.image("BLP"), nil
}

func unpack565(v int) (int, int, int) { return (v >> 11 & 31) << 3, (v >> 5 & 63) << 2, (v & 31) << 3 }

func dxt1(data []byte, alpha bool) [4][]byte {
	var ret [4][]byte
	for block := 0; block < len(data)/8; block++ {
		d := data[8*block:]
		c0, c1 := int(d[0])|int(d[1])<<8, int(d[2])|int(d[3])<<8
		bits := uint32(d[4]) | uint32(d[5])<<8 | uint32(d[6])<<16 | uint32(d[7])<<24
		r0, g0, b0 := unpack565(c0)
		r1, g1, b1 := unpack565(c1)
		for j := 0; j < 4; j++ {
			for i := 0; i < 4; i++ {
				control := bits & 3
				bits >>= 2
				r, g, b, a := 0, 0, 0, 255
				switch control {
				case 0:
					r, g, b = r0, g0, b0
				case 1:
					r, g, b = r1, g1, b1
				case 2:
					if c0 > c1 {
						r, g, b = (2*r0+r1)/3, (2*g0+g1)/3, (2*b0+b1)/3
					} else {
						r, g, b = (r0+r1)/2, (g0+g1)/2, (b0+b1)/2
					}
				case 3:
					if c0 > c1 {
						r, g, b = (2*r1+r0)/3, (2*g1+g0)/3, (2*b1+b0)/3
					} else {
						a = 0
					}
				}
				ret[j] = append(ret[j], byte(r), byte(g), byte(b))
				if alpha {
					ret[j] = append(ret[j], byte(a))
				}
			}
		}
	}
	return ret
}

func dxtColor(code uint32, r0, g0, b0, r1, g1, b1 int) (int, int, int) {
	switch code {
	case 0:
		return r0, g0, b0
	case 1:
		return r1, g1, b1
	case 2:
		return (2*r0 + r1) / 3, (2*g0 + g1) / 3, (2*b0 + b1) / 3
	}
	return (2*r1 + r0) / 3, (2*g1 + g0) / 3, (2*b1 + b0) / 3
}

func dxt3(data []byte) [4][]byte {
	var ret [4][]byte
	for block := 0; block < len(data)/16; block++ {
		d := data[16*block:]
		c0, c1 := int(d[8])|int(d[9])<<8, int(d[10])|int(d[11])<<8
		code := uint32(d[12]) | uint32(d[13])<<8 | uint32(d[14])<<16 | uint32(d[15])<<24
		r0, g0, b0 := unpack565(c0)
		r1, g1, b1 := unpack565(c1)
		for j := 0; j < 4; j++ {
			high := false
			for i := 0; i < 4; i++ {
				a := int(d[(4*j+i)/2])
				if high {
					high = false
					a >>= 4
				} else {
					high = true
					a &= 15
				}
				a *= 17
				r, g, b := dxtColor(code>>(2*(4*j+i))&3, r0, g0, b0, r1, g1, b1)
				ret[j] = append(ret[j], byte(r), byte(g), byte(b), byte(a))
			}
		}
	}
	return ret
}

func dxt5(data []byte) [4][]byte {
	var ret [4][]byte
	for block := 0; block < len(data)/16; block++ {
		d := data[16*block:]
		a0, a1 := int(d[0]), int(d[1])
		code1 := int(d[4]) | int(d[5])<<8 | int(d[6])<<16 | int(d[7])<<24
		code2 := int(d[2]) | int(d[3])<<8
		c0, c1 := int(d[8])|int(d[9])<<8, int(d[10])|int(d[11])<<8
		code := uint32(d[12]) | uint32(d[13])<<8 | uint32(d[14])<<16 | uint32(d[15])<<24
		r0, g0, b0 := unpack565(c0)
		r1, g1, b1 := unpack565(c1)
		for j := 0; j < 4; j++ {
			for i := 0; i < 4; i++ {
				index := 3 * (4*j + i)
				var ac int
				switch {
				case index <= 12:
					ac = code2 >> index & 7
				case index == 15:
					ac = code2>>15 | code1<<1&6
				default:
					ac = code1 >> (index - 16) & 7
				}
				var a int
				switch {
				case ac == 0:
					a = a0
				case ac == 1:
					a = a1
				case a0 > a1:
					a = ((8-ac)*a0 + (ac-1)*a1) / 7
				case ac == 6:
					a = 0
				case ac == 7:
					a = 255
				default:
					a = ((6-ac)*a0 + (ac-1)*a1) / 5
				}
				r, g, b := dxtColor(code>>(2*(4*j+i))&3, r0, g0, b0, r1, g1, b1)
				ret[j] = append(ret[j], byte(r), byte(g), byte(b), byte(a))
			}
		}
	}
	return ret
}
