package pngdec

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/convert"
	"mobile/services/core/internal/media/sanitize/internal/exif"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
)

// pngCase is one generated input; class groups cases in the report.
type pngCase struct {
	name  string
	class string
	data  []byte
}

type builder struct {
	r *pt.Rand
}

// image draws a valid PNG body: IHDR, optional PLTE/tRNS, IDATs, IEND.
type imageSpec struct {
	w, h       int
	depth, ct  byte
	interlace  bool
	pattern    string
	level      int
	split      int
	filter     int
	palette    int
	indexLimit int
	before     [][]byte
	afterPLTE  [][]byte
	afterIDAT  [][]byte
	mutateZlib func([]byte) []byte
	noIEND     bool
}

func (b *builder) build(s imageSpec) []byte {
	out := append([]byte(nil), pt.Magic...)
	interlace := byte(0)
	if s.interlace {
		interlace = 1
	}
	out = append(out, pt.Chunk("IHDR", pt.IHDR(uint32(s.w), uint32(s.h), s.depth, s.ct, interlace))...)
	for _, c := range s.before {
		out = append(out, c...)
	}
	if s.palette > 0 {
		out = append(out, pt.Chunk("PLTE", b.r.Bytes(3*s.palette))...)
	}
	for _, c := range s.afterPLTE {
		out = append(out, c...)
	}
	limit := s.indexLimit
	if limit == 0 {
		limit = s.palette
	}
	samples := pt.Samples(b.r, s.w, s.h, s.depth, s.ct, s.pattern, limit)
	rows := pt.Scanlines(samples, s.w, s.h, s.depth, s.ct, s.interlace)
	stream := pt.Zlib(pt.Filter(b.r, rows, s.depth, s.ct, s.filter), s.level)
	if s.mutateZlib != nil {
		stream = s.mutateZlib(stream)
	}
	split := s.split
	if split <= 0 {
		split = 1 << 16
	}
	out = append(out, pt.IDATs(stream, split)...)
	for _, c := range s.afterIDAT {
		out = append(out, c...)
	}
	if !s.noIEND {
		out = append(out, pt.Chunk("IEND", nil)...)
	}
	return out
}

var validModes = [][2]byte{
	{1, 0}, {2, 0}, {4, 0}, {8, 0}, {16, 0}, {8, 2}, {16, 2},
	{1, 3}, {2, 3}, {4, 3}, {8, 3}, {8, 4}, {16, 4}, {8, 6}, {16, 6},
}

func be16(v int) []byte { return binary.BigEndian.AppendUint16(nil, uint16(v)) }

func exifOrientation(order string, value uint16) []byte {
	magic := "II*\x00"
	if order == "MM" {
		magic = "MM\x00*"
	}
	return pt.TIFF(magic, 0, []pt.Entry{{Tag: 0x0112, Type: pt.TypeShort, Values: []float64{float64(value)}}})
}

// corpus is every PNG case, deterministic for a seed base.
func corpus() []pngCase {
	var cases []pngCase
	add := func(class, name string, data []byte) {
		cases = append(cases, pngCase{name: class + "/" + name, class: class, data: data})
	}
	b := &builder{r: pt.NewRand(0x5eed)}
	sizes := [][2]int{{1, 1}, {3, 2}, {7, 5}, {17, 11}, {33, 1}, {1, 33}}
	levels := []int{0, 1, 6, 9, -2}
	patterns := []string{"smooth", "noise", "flat"}
	i := 0
	for _, m := range validModes {
		for _, interlace := range []bool{false, true} {
			for _, size := range sizes {
				i++
				spec := imageSpec{w: size[0], h: size[1], depth: m[0], ct: m[1], interlace: interlace,
					pattern: patterns[i%3], level: levels[i%5], split: []int{1 << 16, 7, 100}[i%3], filter: -1}
				if m[1] == 3 {
					spec.palette = 1 + b.r.Intn(1<<m[0])
				}
				add("valid", fmt.Sprintf("d%d-c%d-i%v-%dx%d", m[0], m[1], interlace, size[0], size[1]), b.build(spec))
			}
		}
	}

	for n, s := range []imageSpec{
		{w: 300, h: 300, depth: 8, ct: 6, pattern: "noise", level: 0, filter: -1},
		{w: 300, h: 257, depth: 8, ct: 2, pattern: "smooth", level: 6, filter: -1, interlace: true},
		{w: 640, h: 3, depth: 16, ct: 6, pattern: "noise", level: 1, filter: 4, split: 1 << 15},
		{w: 257, h: 400, depth: 8, ct: 3, palette: 256, pattern: "noise", level: 9, filter: -1, split: 70000},
		{w: 200, h: 200, depth: 8, ct: 0, pattern: "flat", level: 9, filter: 0},
	} {
		add("big", fmt.Sprintf("%d", n), b.build(s))
	}

	pal := func(extra ...[]byte) imageSpec {
		return imageSpec{w: 9, h: 7, depth: 8, ct: 3, palette: 6, pattern: "noise", level: 6, filter: -1, afterPLTE: extra}
	}
	add("trns", "p-simple", b.build(pal(pt.Chunk("tRNS", []byte{255, 255, 0, 255}))))
	add("trns", "p-bytes", b.build(pal(pt.Chunk("tRNS", []byte{10, 200, 0, 255, 77}))))
	add("trns", "p-simple-newline", b.build(pal(pt.Chunk("tRNS", []byte{255, 0, 255, '\n'}))))
	add("trns", "p-empty", b.build(pal(pt.Chunk("tRNS", nil))))
	long := make([]byte, 300)
	for k := range long {
		long[k] = 255
	}
	long[280] = 0
	add("trns", "p-simple-index-280", b.build(pal(pt.Chunk("tRNS", long))))
	add("trns", "p-bytes-300", b.build(pal(pt.Chunk("tRNS", b.r.Bytes(300)))))
	add("trns", "p-index-beyond-palette", b.build(imageSpec{w: 9, h: 7, depth: 8, ct: 3, palette: 3, indexLimit: 256,
		pattern: "noise", level: 6, afterPLTE: [][]byte{pt.Chunk("tRNS", []byte{1, 2, 3, 4, 5})}}))
	add("trns", "l", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 0, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", be16(7))}}))
	add("trns", "l-300", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 0, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", be16(300))}}))
	add("trns", "i16", b.build(imageSpec{w: 5, h: 5, depth: 16, ct: 0, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", be16(9000))}}))
	add("trns", "rgb", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 2, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", append(append(be16(1), be16(2)...), be16(3)...))}}))
	add("trns", "bit1", b.build(imageSpec{w: 5, h: 5, depth: 1, ct: 0, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", be16(1))}}))
	add("trns", "la-ignored", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 4, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", be16(1))}}))
	add("trns", "p-after-idat", b.build(imageSpec{w: 9, h: 7, depth: 8, ct: 3, palette: 6, pattern: "noise", afterIDAT: [][]byte{pt.Chunk("tRNS", []byte{0, 9})}}))
	add("trns", "l-short-before", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 0, pattern: "noise", before: [][]byte{pt.Chunk("tRNS", []byte{7})}}))
	add("trns", "l-short-after", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 0, pattern: "noise", afterIDAT: [][]byte{pt.Chunk("tRNS", []byte{7})}}))
	add("trns", "p-no-plte", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 3, pattern: "noise", indexLimit: 256}))
	add("trns", "p-plte-771", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 3, palette: 257, pattern: "noise"}))
	add("trns", "p-plte-770", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 3, pattern: "noise", indexLimit: 256,
		afterPLTE: [][]byte{pt.Chunk("PLTE", b.r.Bytes(770))}}))

	rgb := func(before ...[]byte) imageSpec {
		return imageSpec{w: 6, h: 4, depth: 8, ct: 2, pattern: "noise", level: 6, filter: -1, before: before}
	}
	xmp := []byte(`<x:xmpmeta><rdf:Description tiff:Orientation="6"/></x:xmpmeta>`)
	add("text", "text-adobe-xmp", b.build(rgb(pt.Chunk("tEXt", append([]byte("XML:com.adobe.xmp\x00"), xmp...)))))
	add("text", "itxt-xmp", b.build(rgb(pt.Chunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"), xmp...)))))
	add("text", "itxt-xmp-zip", b.build(rgb(pt.Chunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x01\x00\x00\x00"), pt.Zlib(xmp, 9)...)))))
	add("text", "itxt-xmp-bad-utf8", b.build(rgb(pt.Chunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00\xff"), xmp...)))))
	add("text", "itxt-method-1", b.build(rgb(pt.Chunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x01\x01\x00\x00"), xmp...)))))
	add("text", "itxt-zip-corrupt", b.build(rgb(pt.Chunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x01\x00\x00\x00"), 1, 2, 3, 4)))))
	add("text", "text-xmp-key-str", b.build(rgb(pt.Chunk("tEXt", append([]byte("xmp\x00"), xmp...)))))
	add("text", "text-xmp-key-str-no-orientation", b.build(rgb(pt.Chunk("tEXt", []byte("xmp\x00hello")))))
	hexExif := hex.EncodeToString(append([]byte("Exif\x00\x00"), exifOrientation("MM", 3)...))
	raw := "\nexif\n      " + fmt.Sprint(len(hexExif)/2) + "\n" + hexExif[:40] + "\n" + hexExif[40:] + "\n"
	add("text", "ztxt-raw-profile", b.build(rgb(pt.Chunk("zTXt", append([]byte("Raw profile type exif\x00\x00"), pt.Zlib([]byte(raw), 6)...)))))
	add("text", "text-raw-profile-bad-hex", b.build(rgb(pt.Chunk("tEXt", []byte("Raw profile type exif\x00\nexif\n 3\nzz")))))
	add("text", "text-exif-bytes", b.build(rgb(pt.Chunk("tEXt", append([]byte("exif\x00"), exifOrientation("II", 8)...)))))
	add("text", "ztxt-exif-str", b.build(rgb(pt.Chunk("zTXt", append([]byte("exif\x00\x00"), pt.Zlib([]byte("abc"), 6)...)))))
	add("text", "ztxt-exif-empty", b.build(rgb(pt.Chunk("zTXt", []byte("exif\x00\x00")))))
	add("text", "text-transparency-p", b.build(imageSpec{w: 5, h: 5, depth: 8, ct: 3, palette: 8, pattern: "noise",
		afterPLTE: [][]byte{pt.Chunk("tEXt", []byte("transparency\x003"))}}))
	add("text", "text-transparency-rgb", b.build(rgb(pt.Chunk("tEXt", []byte("transparency\x003")))))
	add("text", "text-interlace-flag", b.build(rgb(pt.Chunk("tEXt", []byte("interlace\x001")))))
	add("text", "text-interlace-empty", b.build(rgb(pt.Chunk("tEXt", []byte("interlace\x00")))))
	add("text", "text-no-null", b.build(rgb(pt.Chunk("tEXt", []byte("Comment")))))
	add("text", "text-empty-key", b.build(rgb(pt.Chunk("tEXt", []byte("\x00value")))))
	add("text", "ztxt-bad-method", b.build(rgb(pt.Chunk("zTXt", []byte("k\x00\x01abc")))))
	add("text", "ztxt-bad-method-after", b.build(imageSpec{w: 6, h: 4, depth: 8, ct: 2, pattern: "noise", afterIDAT: [][]byte{pt.Chunk("zTXt", []byte("k\x00\x01abc"))}}))
	add("text", "ztxt-corrupt", b.build(rgb(pt.Chunk("zTXt", []byte("k\x00\x00\x78\x9c\xff\xff")))))
	big := make([]byte, maxTextChunk+1)
	add("text", "ztxt-too-large", b.build(rgb(pt.Chunk("zTXt", append([]byte("k\x00\x00"), pt.Zlib(big, 9)...)))))
	add("text", "ztxt-exactly-limit", b.build(rgb(pt.Chunk("zTXt", append([]byte("k\x00\x00"), pt.Zlib(big[:maxTextChunk], 9)...)))))
	add("text", "exif-after-idat", b.build(imageSpec{w: 6, h: 4, depth: 8, ct: 2, pattern: "noise", afterIDAT: [][]byte{pt.Chunk("eXIf", exifOrientation("II", 6))}}))
	add("text", "exif-and-xmp", b.build(rgb(pt.Chunk("eXIf", exifOrientation("II", 1)), pt.Chunk("tEXt", append([]byte("XML:com.adobe.xmp\x00"), xmp...)))))

	gray := func() imageSpec {
		return imageSpec{w: 8, h: 6, depth: 8, ct: 0, pattern: "noise", level: 6, filter: -1}
	}
	s := gray()
	s.before = [][]byte{pt.BadCRC("gAMA", be16(0))}
	add("corrupt", "bad-crc-before-idat", b.build(s))
	full := b.build(gray())
	idatCRC := append([]byte(nil), full...)
	idatCRC[len(idatCRC)-12-1]++
	add("corrupt", "bad-crc-idat", idatCRC)
	for _, frac := range []int{10, 50, 90, 99} {
		d := b.build(imageSpec{w: 40, h: 30, depth: 8, ct: 2, pattern: "noise", level: 0, filter: 0})
		add("corrupt", fmt.Sprintf("truncated-%d", frac), d[:len(d)*frac/100])
	}
	s = gray()
	s.noIEND = true
	add("corrupt", "no-iend", b.build(s))
	s = gray()
	s.mutateZlib = func(z []byte) []byte { z[0] ^= 1; return z }
	add("corrupt", "zlib-header", b.build(s))
	s = gray()
	s.mutateZlib = func(z []byte) []byte { z[0], z[1] = 0x78, 0xbb; return z }
	add("corrupt", "zlib-fdict", b.build(s))
	s = gray()
	s.mutateZlib = func(z []byte) []byte { z[len(z)-1]++; return z }
	add("corrupt", "adler-same-chunk", b.build(s))
	s = gray()
	s.split = 1
	s.mutateZlib = func(z []byte) []byte { z[len(z)-1]++; return z }
	add("corrupt", "adler-split-chunks", b.build(s))
	s = gray()
	s.mutateZlib = func(z []byte) []byte { return z[:len(z)-4] }
	add("corrupt", "no-adler", b.build(s))
	s = gray()
	s.mutateZlib = func(z []byte) []byte { return append(z, 1, 2, 3, 4, 5) }
	add("corrupt", "trailing-junk", b.build(s))
	s = gray()
	s.mutateZlib = func(z []byte) []byte { return z[:len(z)/2] }
	add("corrupt", "zlib-half", b.build(s))
	short := b.r
	_ = short
	shortRows := pt.Zlib(append([]byte{0}, make([]byte, 8)...), 6)
	d := append([]byte(nil), pt.Magic...)
	d = append(d, pt.Chunk("IHDR", pt.IHDR(8, 6, 8, 0, 0))...)
	d = append(d, pt.Chunk("IDAT", shortRows)...)
	d = append(d, pt.Chunk("IEND", nil)...)
	add("corrupt", "stream-ends-after-row-same-chunk", d)
	d = append([]byte(nil), pt.Magic...)
	d = append(d, pt.Chunk("IHDR", pt.IHDR(8, 6, 8, 0, 0))...)
	d = append(d, pt.IDATs(shortRows, len(shortRows)-4)...)
	d = append(d, pt.Chunk("IEND", nil)...)
	add("corrupt", "stream-ends-after-row-split", d)
	s = gray()
	s.filter = 5
	add("corrupt", "filter-type-5", b.build(s))
	for _, h := range []struct {
		name string
		data []byte
	}{
		{"ihdr-depth-3", pt.IHDR(4, 4, 3, 0, 0)},
		{"ihdr-filter-1", append(pt.IHDR(4, 4, 8, 0, 0)[:11], 1, 0)},
		{"ihdr-short", pt.IHDR(4, 4, 8, 0, 0)[:12]},
		{"ihdr-width-0", pt.IHDR(0, 4, 8, 0, 0)},
		{"ihdr-huge", pt.IHDR(20000, 20000, 8, 0, 0)},
		{"ihdr-wide-zero-height", pt.IHDR(1<<31, 0, 8, 0, 0)},
	} {
		d := append([]byte(nil), pt.Magic...)
		d = append(d, pt.Chunk("IHDR", h.data)...)
		d = append(d, pt.Chunk("IDAT", pt.Zlib(make([]byte, 80), 6))...)
		d = append(d, pt.Chunk("IEND", nil)...)
		add("corrupt", h.name, d)
	}
	d = append([]byte(nil), pt.Magic...)
	d = append(d, pt.Chunk("IDAT", pt.Zlib(make([]byte, 20), 6))...)
	d = append(d, pt.Chunk("IHDR", pt.IHDR(4, 4, 8, 0, 0))...)
	d = append(d, pt.Chunk("IEND", nil)...)
	add("corrupt", "idat-before-ihdr", d)
	for _, c := range []struct {
		name  string
		chunk []byte
	}{
		{"unknown-lower", pt.Chunk("zzZz", []byte("abc"))},
		{"bad-name", pt.Chunk("ab#d", []byte("abc"))},
		{"iccp-empty", pt.Chunk("iCCP", nil)},
		{"iccp-method-1", pt.Chunk("iCCP", []byte("name\x00\x01abc"))},
		{"iccp-corrupt", pt.Chunk("iCCP", []byte("name\x00\x00abc"))},
		{"gama-short", pt.Chunk("gAMA", []byte{1, 2})},
		{"chrm-6", pt.Chunk("cHRM", []byte{1, 2, 3, 4, 5, 6})},
		{"srgb-empty", pt.Chunk("sRGB", nil)},
		{"phys-short", pt.Chunk("pHYs", []byte{1, 2, 3})},
	} {
		s := gray()
		s.before = [][]byte{c.chunk}
		add("chunks", c.name, b.build(s))
		s = gray()
		s.afterIDAT = [][]byte{c.chunk}
		add("chunks", c.name+"-after-idat", b.build(s))
	}
	huge := append(binary.BigEndian.AppendUint32(nil, 0xfffffff0), "zzZz"...)
	s = gray()
	s.before = [][]byte{huge}
	add("chunks", "unknown-huge-length", b.build(s))

	// APNG
	actl := func(n uint32) []byte {
		return pt.Chunk("acTL", binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, n), 0))
	}
	fctl := func(seq, w, h, x, y uint32, dispose, blend byte) []byte {
		data := binary.BigEndian.AppendUint32(nil, seq)
		for _, v := range []uint32{w, h, x, y} {
			data = binary.BigEndian.AppendUint32(data, v)
		}
		data = append(data, 0, 1, 0, 10, dispose, blend)
		return pt.Chunk("fcTL", data)
	}
	frame := func(w, h int) []byte {
		samples := pt.Samples(b.r, w, h, 8, 6, "noise", 0)
		return pt.Zlib(pt.Filter(b.r, pt.Scanlines(samples, w, h, 8, 6, false), 8, 6, -1), 6)
	}
	fdat := func(seq uint32, z []byte) []byte {
		return pt.Chunk("fdAT", append(binary.BigEndian.AppendUint32(nil, seq), z...))
	}
	apng := func(parts ...[]byte) []byte {
		d := append([]byte(nil), pt.Magic...)
		d = append(d, pt.Chunk("IHDR", pt.IHDR(10, 8, 8, 6, 0))...)
		for _, p := range parts {
			d = append(d, p...)
		}
		return append(d, pt.Chunk("IEND", nil)...)
	}
	add("apng", "frame0-subrect", apng(actl(2), fctl(0, 5, 4, 2, 3, 0, 0), pt.Chunk("IDAT", frame(5, 4)), fctl(1, 10, 8, 0, 0, 0, 0), fdat(2, frame(10, 8))))
	add("apng", "default-image", apng(actl(1), pt.Chunk("IDAT", frame(10, 8)), fctl(0, 10, 8, 0, 0, 0, 0), fdat(1, frame(10, 8))))
	add("apng", "dispose-background", apng(actl(2), fctl(0, 10, 8, 0, 0, 1, 0), pt.Chunk("IDAT", frame(10, 8)), fctl(1, 10, 8, 0, 0, 0, 0), fdat(2, frame(10, 8))))
	add("apng", "dispose-previous", apng(actl(2), fctl(0, 10, 8, 0, 0, 2, 1), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "actl-twice", apng(actl(2), actl(2), fctl(0, 10, 8, 0, 0, 1, 0), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "fctl-bad-seq", apng(actl(2), fctl(3, 10, 8, 0, 0, 0, 0), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "fctl-out-of-bounds", apng(actl(2), fctl(0, 10, 8, 1, 0, 0, 0), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "fctl-zero-size", apng(actl(2), fctl(0, 0, 8, 0, 0, 0, 0), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "text-bbox", apng(actl(2), fctl(0, 10, 8, 0, 0, 0, 0), pt.Chunk("tEXt", []byte("bbox\x00x")), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "text-bbox-dispose", apng(actl(2), fctl(0, 10, 8, 0, 0, 1, 0), pt.Chunk("tEXt", []byte("bbox\x00x")), pt.Chunk("IDAT", frame(10, 8))))
	add("apng", "fdat-first", apng(actl(2), fdat(0, frame(10, 8))))
	add("apng", "fdat-after-fctl", apng(actl(2), fctl(0, 10, 8, 0, 0, 0, 0), fdat(1, frame(10, 8))))
	add("apng", "eXIf-after-next-fctl", apng(actl(2), fctl(0, 10, 8, 0, 0, 0, 0), pt.Chunk("IDAT", frame(10, 8)), fctl(1, 10, 8, 0, 0, 0, 0), pt.Chunk("eXIf", exifOrientation("II", 6))))
	return cases
}

// goOutcome is what the Go side answers for one input.
type goOutcome struct {
	class  string // ok, next, bomb, error
	detail string
	img    *pil.Image
	err    error
}

func classify(err error) string {
	var bomb *pil.BombError
	switch {
	case pil.IsNext(err):
		return "next"
	case errors.As(err, &bomb):
		return "bomb"
	}
	return "error"
}

func goDecode(data []byte) goOutcome {
	opened, err := Open(data)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}
	}
	w, h := opened.Size()
	if err := pil.CheckBomb(w, h); err != nil {
		return goOutcome{class: "bomb", detail: err.Error()}
	}
	img, err := opened.Load()
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error(), err: err}
	}
	return goOutcome{class: "ok", img: img}
}

// goPixels runs decode, exif_transpose and the encoder conversion.
func goPixels(data []byte) (outcome goOutcome, mode string, pix []byte) {
	outcome = goDecode(data)
	if outcome.class != "ok" {
		return outcome, "", nil
	}
	transposed, err := exif.Transpose(outcome.img)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	mode, pix, err = convert.ForEncoder(transposed)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	outcome.img = transposed
	return outcome, mode, pix
}

func pyClass(errType string) string {
	switch {
	case errType == "":
		return "ok"
	case errType == "UnidentifiedImageError":
		return "next"
	case strings.HasPrefix(errType, "DecompressionBomb"):
		return "bomb"
	}
	return "error"
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
