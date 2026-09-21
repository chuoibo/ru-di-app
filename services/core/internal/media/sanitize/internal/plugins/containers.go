package plugins

import (
	"bytes"
	"math"
	"sort"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// bmpBitmap is BmpImageFile._bitmap as CUR and ICO frames reach it (offset
// argument 0). It returns the mode and size the header selects.
func bmpBitmap(f *file, header int64) (string, int64, int64, error) {
	if header != 0 {
		if err := f.seek(header, 0); err != nil {
			return "", 0, 0, err
		}
	}
	hs, err := u32le(f.read(4), 0)
	if err != nil {
		return "", 0, 0, err
	}
	var hd []byte
	if hs-4 > 0 {
		hd = f.read(hs - 4)
		if int64(len(hd)) < hs-4 {
			return "", 0, 0, raise("OSError", "Truncated File Read")
		}
	}
	var w, h, bits, compression, colors, padding int64
	var masks [4]int64
	switch hs {
	case 12:
		w, _ = u16le(hd, 0)
		h, _ = u16le(hd, 2)
		bits, _ = u16le(hd, 6)
		padding = 3
	case 40, 52, 56, 64, 108, 124:
		w, _ = u32le(hd, 0)
		h, _ = u32le(hd, 4)
		if hd[7] == 255 {
			h = 1<<32 - h
		}
		bits, _ = u16le(hd, 10)
		compression, _ = u32le(hd, 12)
		colors, _ = u32le(hd, 28)
		padding = 4
		if compression == 3 {
			if len(hd) >= 48 {
				for i := 0; i < 3; i++ {
					masks[i], _ = u32le(hd, 36+4*i)
				}
				if len(hd) >= 52 {
					masks[3], _ = u32le(hd, 48)
				}
			} else {
				for i := 0; i < 3; i++ {
					if masks[i], err = u32le(f.read(4), 0); err != nil {
						return "", 0, 0, err
					}
				}
			}
		}
	default:
		return "", 0, 0, raise("OSError", "Unsupported BMP header type")
	}
	if colors == 0 {
		if bits < 62 {
			colors = 1 << bits
		} else {
			colors = saturate
		}
	}
	modes := map[int64]string{1: "P", 4: "P", 8: "P", 16: "RGB", 24: "RGB", 32: "RGB"}
	mode, ok := modes[bits]
	if !ok {
		return "", 0, 0, raise("OSError", "Unsupported BMP pixel depth")
	}
	switch compression {
	case 3:
		supported := false
		switch bits {
		case 32:
			for _, m := range [][4]int64{
				{0xff0000, 0xff00, 0xff, 0}, {0xff000000, 0xff0000, 0xff00, 0}, {0xff000000, 0xff00, 0xff, 0},
				{0xff000000, 0xff0000, 0xff00, 0xff}, {0xff, 0xff00, 0xff0000, 0xff000000},
				{0xff0000, 0xff00, 0xff, 0xff000000}, {0xff000000, 0xff00, 0xff, 0xff0000}, {0, 0, 0, 0},
			} {
				if m == masks {
					supported = true
					if m[3] != 0 || m == [4]int64{} {
						mode = "RGBA"
					}
				}
			}
		case 24:
			supported = masks[0] == 0xff0000 && masks[1] == 0xff00 && masks[2] == 0xff
		case 16:
			supported = masks[2] == 0x1f && (masks[0] == 0xf800 && masks[1] == 0x7e0 || masks[0] == 0x7c00 && masks[1] == 0x3e0)
		}
		if !supported {
			return "", 0, 0, raise("OSError", "Unsupported BMP bitfields layout")
		}
	case 0:
		if bits == 32 && header == 22 {
			mode = "RGBA"
		}
	case 1, 2:
	default:
		return "", 0, 0, raise("OSError", "Unsupported BMP compression")
	}
	if mode == "P" {
		if !(colors > 0 && colors <= 65536) {
			return "", 0, 0, raise("OSError", "Unsupported BMP Palette size")
		}
		palette := f.read(padding * colors)
		grey := true
		for i := int64(0); i < colors; i++ {
			val := i
			if colors == 2 {
				val = []int64{0, 255}[i]
			}
			start, end := pySlice(int64(len(palette)), i*padding, i*padding+3)
			rgb := palette[start:end]
			if len(rgb) != 3 || rgb[0] != byte(val) || rgb[1] != byte(val) || rgb[2] != byte(val) {
				grey = false
			}
		}
		if grey {
			mode = "L"
			if colors == 2 {
				mode = "1"
			}
		}
	}
	return mode, w, h, nil
}

func acceptCUR(p []byte) bool { return hasPrefix(p, "\x00\x00\x02\x00") }

func openCUR(f *file) (pil.Opened, error) {
	s := f.read(6)
	if !acceptCUR(s) {
		return nil, raise("SyntaxError", "not a CUR file")
	}
	count, err := u16le(s, 4)
	if err != nil {
		return nil, err
	}
	var m []byte
	for i := int64(0); i < count; i++ {
		s = f.read(16)
		if len(m) == 0 {
			m = s
			continue
		}
		s0, err := at(s, 0)
		if err != nil {
			return nil, err
		}
		if s0 > int64(m[0]) {
			s1, err := at(s, 1)
			if err != nil {
				return nil, err
			}
			m1, err := at(m, 1)
			if err != nil {
				return nil, err
			}
			if s1 > m1 {
				m = s
			}
		}
	}
	if len(m) == 0 {
		return nil, raise("TypeError", "No cursors were found")
	}
	offset, err := u32le(m, 12)
	if err != nil {
		return nil, err
	}
	mode, w, h, err := bmpBitmap(f, offset)
	if err != nil {
		return nil, err
	}
	return finish(mode, w, h/2, unsupported("CUR", "embedded bitmap"))
}

func acceptICO(p []byte) bool { return hasPrefix(p, "\x00\x00\x01\x00") }

type icoEntry struct {
	w, h, bpp, size, offset int64
	square, depth           int64
}

// openICO is IcoImageFile._open, which loads the largest frame while
// opening. The frame itself (PNG, or a DIB with its mask) is not decoded
// here: after the checks that fail before decoding, open answers
// Unsupported.
func openICO(f *file) (pil.Opened, error) {
	s := f.read(6)
	if !acceptICO(s) {
		return nil, raise("SyntaxError", "not an ICO file")
	}
	count, err := u16le(s, 4)
	if err != nil {
		return nil, err
	}
	var entries []icoEntry
	for i := int64(0); i < count; i++ {
		s = f.read(16)
		var e icoEntry
		var head [3]int64
		for k := range head {
			if head[k], err = at(s, k); err != nil {
				return nil, err
			}
		}
		if e.bpp, err = u16le(s, 6); err != nil {
			return nil, err
		}
		if _, err = at(s, 3); err != nil {
			return nil, err
		}
		if _, err = u16le(s, 4); err != nil {
			return nil, err
		}
		if e.size, err = u32le(s, 8); err != nil {
			return nil, err
		}
		if e.offset, err = u32le(s, 12); err != nil {
			return nil, err
		}
		e.w, e.h = head[0], head[1]
		if e.w == 0 {
			e.w = 256
		}
		if e.h == 0 {
			e.h = 256
		}
		e.square = e.w * e.h
		e.depth = e.bpp
		if e.depth == 0 && head[2] != 0 {
			e.depth = int64(math.Ceil(math.Log(float64(head[2])) / math.Log(2)))
		}
		if e.depth == 0 {
			e.depth = 256
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].depth < entries[j].depth })
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].square > entries[j].square })
	if len(entries) == 0 {
		return nil, indexErr()
	}
	e := entries[0]
	_ = f.seek(e.offset, 0)
	head := f.read(8)
	_ = f.seek(e.offset, 0)
	if string(head) == "\x89PNG\r\n\x1a\n" {
		return nil, pil.Unsupported("ICO", "embedded PNG frame")
	}
	mode, w, h, err := bmpBitmap(f, 0)
	if err != nil {
		return nil, err
	}
	if mode == "" || w <= 0 || h <= 0 {
		return nil, raise("SyntaxError", "not identified by this driver")
	}
	if err := bombCheck(w, h); err != nil {
		return nil, err
	}
	h2 := int64(float64(h) / 2)
	if h2 == 0 {
		return nil, raise("ValueError", "tile cannot extend outside image")
	}
	if e.bpp == 32 {
		alpha := int64(len(f.read(w * h2 * 4)))
		if alpha/4 < w*h2 {
			return nil, raise("ValueError", "buffer is not large enough")
		}
	} else {
		w8 := w
		if w%32 > 0 {
			w8 += 32 - w%32
		}
		total := w8 * h2 / 8
		if err := f.seek(e.offset+e.size-total, 0); err != nil {
			return nil, err
		}
		stride := w8 / 8
		if int64(len(f.read(total))) < (h2-1)*stride+(w+7)/8 {
			return nil, raise("ValueError", "not enough image data")
		}
	}
	return nil, pil.Unsupported("ICO", "embedded bitmap frame")
}

var icnsSizes = []struct {
	size  [3]int64
	codes []string
}{
	{[3]int64{512, 512, 2}, []string{"ic10"}},
	{[3]int64{512, 512, 1}, []string{"ic09"}},
	{[3]int64{256, 256, 2}, []string{"ic14"}},
	{[3]int64{256, 256, 1}, []string{"ic08"}},
	{[3]int64{128, 128, 2}, []string{"ic13"}},
	{[3]int64{128, 128, 1}, []string{"ic07", "it32", "t8mk"}},
	{[3]int64{64, 64, 1}, []string{"icp6"}},
	{[3]int64{32, 32, 2}, []string{"ic12"}},
	{[3]int64{48, 48, 1}, []string{"ih32", "h8mk"}},
	{[3]int64{32, 32, 1}, []string{"icp5", "il32", "l8mk"}},
	{[3]int64{16, 16, 2}, []string{"ic11"}},
	{[3]int64{16, 16, 1}, []string{"icp4", "is32", "s8mk"}},
}

func acceptICNS(p []byte) bool { return hasPrefix(p, "icns") }

func openICNS(f *file) (pil.Opened, error) {
	b := f.read(8)
	if len(b) != 8 {
		return nil, structErr()
	}
	if !acceptICNS(b[:4]) {
		return nil, raise("SyntaxError", "not an icns file")
	}
	filesize, _ := u32be(b, 4)
	blocks := map[string]bool{}
	for i := int64(8); i < filesize; {
		b = f.read(8)
		if len(b) != 8 {
			return nil, structErr()
		}
		blocksize, _ := u32be(b, 4)
		if blocksize <= 0 {
			return nil, raise("SyntaxError", "invalid block header")
		}
		i += 8
		blocksize -= 8
		blocks[string(b[:4])] = true
		_ = f.seek(blocksize, 1)
		i += blocksize
	}
	var best [3]int64
	found := false
	for _, entry := range icnsSizes {
		for _, code := range entry.codes {
			if blocks[code] {
				if !found || lessTriple(best, entry.size) {
					best = entry.size
				}
				found = true
				break
			}
		}
	}
	if !found {
		return nil, raise("SyntaxError", "No 32bit icon resources found")
	}
	return finish("RGBA", best[0]*best[2], best[1]*best[2], unsupported("ICNS", "icon resources"))
}

func lessTriple(a, b [3]int64) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func acceptAVIF(p []byte) bool {
	if len(p) < 8 || string(p[4:8]) != "ftyp" {
		return false
	}
	major := ""
	if len(p) >= 12 {
		major = string(p[8:12])
	} else {
		major = string(p[8:])
	}
	return major == "avif" || major == "avis" || major == "mif1" || major == "msf1"
}

// openAVIF emulates the first check libavif's decoder makes: the ftyp box
// must be complete and name an AVIF brand. Past that, Pillow's open depends
// on the whole libavif parse, so open answers Unsupported.
func openAVIF(f *file) (pil.Opened, error) {
	data := f.data
	size, _ := u32be(data, 0)
	if size < 16 || size > int64(len(data)) {
		return nil, raise("SyntaxError", "Failed to decode image: Truncated data")
	}
	brands := []string{string(data[8:12])}
	for i := int64(16); i+4 <= size; i += 4 {
		brands = append(brands, string(data[i:i+4]))
	}
	for _, b := range brands {
		if b == "avif" || b == "avis" {
			return nil, pil.Unsupported("AVIF", "AV1 decoder")
		}
	}
	return nil, raise("SyntaxError", "Failed to decode image: Invalid ftyp")
}

func acceptJPEG2000(p []byte) bool {
	return hasPrefix(p, "\xffO\xffQ", "\x00\x00\x00\x0cjP  \x0d\x0a\x87\x0a")
}

type boxReader struct {
	f         *file
	hasLength bool
	length    int64
	remaining int64
}

func (r *boxReader) canRead(n int64) bool {
	if r.hasLength && r.f.tell()+n > r.length {
		return false
	}
	if r.remaining >= 0 {
		return n <= r.remaining
	}
	return true
}

func (r *boxReader) readBytes(n int64) ([]byte, error) {
	if !r.canRead(n) {
		return nil, raise("SyntaxError", "Not enough data in header")
	}
	data := r.f.read(n)
	if int64(len(data)) < n {
		return nil, raise("OSError", "Expected to read %d bytes", n)
	}
	if r.remaining > 0 {
		r.remaining -= n
	}
	return data, nil
}

func (r *boxReader) hasNext() bool {
	if r.hasLength {
		return r.f.tell()+r.remaining < r.length
	}
	return true
}

func (r *boxReader) nextType() (string, error) {
	if r.remaining > 0 {
		_ = r.f.seek(r.remaining, 1)
	}
	r.remaining = -1
	d, err := r.readBytes(8)
	if err != nil {
		return "", err
	}
	lbox, _ := u32be(d, 0)
	hlen := int64(8)
	if lbox == 1 {
		q, err := r.readBytes(8)
		if err != nil {
			return "", err
		}
		hi, _ := u32be(q, 0)
		lo, _ := u32be(q, 4)
		if hi >= 1<<30 {
			lbox = saturate
		} else {
			lbox = hi<<32 | lo
		}
		hlen = 16
	}
	if lbox < hlen || !r.canRead(lbox-hlen) {
		return "", raise("SyntaxError", "Invalid header length")
	}
	r.remaining = lbox - hlen
	return string(d[4:8]), nil
}

func (r *boxReader) sub() (*boxReader, error) {
	size := r.remaining
	data, err := r.readBytes(size)
	if err != nil {
		return nil, err
	}
	return &boxReader{f: &file{data: data}, hasLength: true, length: size, remaining: -1}, nil
}

func parseCodestream(f *file) (string, int64, int64, error) {
	hdr := f.read(2)
	lsiz, err := u16be(hdr, 0)
	if err != nil {
		return "", 0, 0, err
	}
	siz := append(append([]byte(nil), hdr...), f.read(lsiz-2)...)
	if len(siz) < 38 {
		return "", 0, 0, structErr()
	}
	xsiz, _ := u32be(siz, 4)
	ysiz, _ := u32be(siz, 8)
	xo, _ := u32be(siz, 12)
	yo, _ := u32be(siz, 16)
	csiz, _ := u16be(siz, 36)
	var mode string
	switch csiz {
	case 1:
		if len(siz) < 39 {
			return "", 0, 0, structErr()
		}
		mode = "L"
		if int(siz[38]&127)+1 > 8 {
			mode = "I;16"
		}
	case 2:
		mode = "LA"
	case 3:
		mode = "RGB"
	case 4:
		mode = "RGBA"
	default:
		return "", 0, 0, raise("SyntaxError", "unable to determine J2K image mode")
	}
	return mode, xsiz - xo, ysiz - yo, nil
}

// parseComment is Jpeg2KImageFile._parse_comment. A marker length below two
// seeks backwards; when the position repeats Pillow loops forever.
func parseComment(f *file) error {
	seen := map[int64]bool{}
	for {
		if seen[f.tell()] {
			return pil.Unsupported("JPEG2000", "Pillow does not terminate on this marker loop")
		}
		seen[f.tell()] = true
		marker := f.read(2)
		if len(marker) == 0 {
			return nil
		}
		typ, err := at(marker, 1)
		if err != nil {
			return err
		}
		if typ == 144 || typ == 217 {
			return nil
		}
		length, err := u16be(f.read(2), 0)
		if err != nil {
			return err
		}
		if typ == 100 {
			f.read(length - 2)
			return nil
		}
		_ = f.seek(length-2, 1)
	}
}

func parseJP2Header(f *file) (string, int64, int64, error) {
	top := &boxReader{f: f, remaining: -1}
	var header *boxReader
	for top.hasNext() {
		typ, err := top.nextType()
		if err != nil {
			return "", 0, 0, err
		}
		if typ == "jp2h" {
			if header, err = top.sub(); err != nil {
				return "", 0, 0, err
			}
			break
		}
		if typ == "ftyp" {
			if _, err := top.readBytes(4); err != nil {
				return "", 0, 0, err
			}
		}
	}
	var mode, colr string
	var w, h, nc int64
	sized := false
	for header.hasNext() {
		typ, err := header.nextType()
		if err != nil {
			return "", 0, 0, err
		}
		switch {
		case typ == "ihdr":
			d, err := header.readBytes(11)
			if err != nil {
				return "", 0, 0, err
			}
			h, _ = u32be(d, 0)
			w, _ = u32be(d, 4)
			nc, _ = u16be(d, 8)
			bpc := d[10]
			sized = true
			switch {
			case nc == 1 && bpc&127 > 8:
				mode = "I;16"
			case nc == 1:
				mode = "L"
			case nc == 2:
				mode = "LA"
			case nc == 3:
				mode = "RGB"
			case nc == 4:
				mode = "RGBA"
			}
		case typ == "colr":
			d, err := header.readBytes(7)
			if err != nil {
				return "", 0, 0, err
			}
			enumcs, _ := u32be(d, 3)
			if d[0] == 1 {
				switch enumcs {
				case 0, 15:
					colr = "1"
				case 12:
					colr = "CMYK"
					if nc == 4 {
						mode = "CMYK"
					}
				case 17:
					colr = "L"
				}
			}
		case typ == "pclr" && (mode == "L" || mode == "LA") && colr != "1" && colr != "L":
			d, err := header.readBytes(3)
			if err != nil {
				return "", 0, 0, err
			}
			ne, _ := u16be(d, 0)
			npc := int64(d[2])
			depths, err := header.readBytes(npc)
			if err != nil {
				return "", 0, 0, err
			}
			maxDepth := byte(0)
			for _, v := range depths {
				if v > maxDepth {
					maxDepth = v
				}
			}
			if maxDepth <= 8 {
				per := int64(3)
				if npc == 4 {
					per = 4
				}
				unique := map[string]bool{}
				for i := int64(0); i < ne; i++ {
					color, err := header.readBytes(npc)
					if err != nil {
						return "", 0, 0, err
					}
					key := string(color)
					if per == 3 && len(color) > 3 {
						key = string(color[:3])
					}
					if !unique[key] {
						if int64(len(unique))*per/3 >= 256 {
							return "", 0, 0, raise("ValueError", "cannot allocate more than 256 colors")
						}
						unique[key] = true
					}
				}
				if mode == "L" {
					mode = "P"
				} else {
					mode = "PA"
				}
			}
		case typ == "res ":
			res, err := header.sub()
			if err != nil {
				return "", 0, 0, err
			}
			for res.hasNext() {
				t, err := res.nextType()
				if err != nil {
					return "", 0, 0, err
				}
				if t == "resc" {
					if _, err := res.readBytes(10); err != nil {
						return "", 0, 0, err
					}
					break
				}
			}
		}
	}
	if !sized || mode == "" {
		return "", 0, 0, raise("SyntaxError", "Malformed JP2 header")
	}
	return mode, w, h, nil
}

func openJPEG2000(f *file) (pil.Opened, error) {
	sig := f.read(4)
	var mode string
	var w, h int64
	var err error
	if string(sig) == "\xffO\xffQ" {
		if mode, w, h, err = parseCodestream(f); err != nil {
			return nil, err
		}
		if err := parseComment(f); err != nil {
			return nil, err
		}
	} else {
		sig = append(append([]byte(nil), sig...), f.read(8)...)
		if string(sig) != "\x00\x00\x00\x0cjP  \r\n\x87\n" {
			return nil, raise("SyntaxError", "not a JPEG 2000 file")
		}
		if mode, w, h, err = parseJP2Header(f); err != nil {
			return nil, err
		}
		if bytes.HasSuffix(f.read(12), []byte("jp2c\xffO\xffQ")) {
			length, err := u16be(f.read(2), 0)
			if err != nil {
				return nil, err
			}
			_ = f.seek(length-2, 1)
			if err := parseComment(f); err != nil {
				return nil, err
			}
		}
	}
	return finish(mode, w, h, unsupported("JPEG2000", "OpenJPEG decoder"))
}
