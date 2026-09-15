package exif_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/convert"
	"mobile/services/core/internal/media/sanitize/internal/exif"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/pngdec"
	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
)

type exifCase struct {
	name  string
	class string
	data  []byte
}

// container wraps metadata chunks around a small seeded PNG.
func container(r *pt.Rand, w, h int, depth, ct byte, interlace bool, before, after [][]byte) []byte {
	out := append([]byte(nil), pt.Magic...)
	il := byte(0)
	if interlace {
		il = 1
	}
	out = append(out, pt.Chunk("IHDR", pt.IHDR(uint32(w), uint32(h), depth, ct, il))...)
	palette := 0
	if ct == 3 {
		palette = 1 << depth
		out = append(out, pt.Chunk("PLTE", r.Bytes(3*palette))...)
		out = append(out, pt.Chunk("tRNS", r.Bytes(palette/2))...)
	}
	for _, c := range before {
		out = append(out, c...)
	}
	samples := pt.Samples(r, w, h, depth, ct, "noise", palette)
	rows := pt.Scanlines(samples, w, h, depth, ct, interlace)
	out = append(out, pt.IDATs(pt.Zlib(pt.Filter(r, rows, depth, ct, -1), 6), 1<<16)...)
	for _, c := range after {
		out = append(out, c...)
	}
	return append(out, pt.Chunk("IEND", nil)...)
}

func orientation(typ uint16, values ...float64) pt.Entry {
	return pt.Entry{Tag: 0x0112, Type: typ, Values: values}
}

func corpus() []exifCase {
	var cases []exifCase
	r := pt.NewRand(0xe71f)
	rgb := func(before ...[]byte) []byte { return container(r, 5, 3, 8, 2, false, before, nil) }
	exifChunk := func(block []byte) []byte { return pt.Chunk("eXIf", block) }
	add := func(class, name string, data []byte) {
		cases = append(cases, exifCase{name: class + "/" + name, class: class, data: data})
	}

	for _, magic := range []string{"II*\x00", "MM\x00*"} {
		for v := 0; v <= 9; v++ {
			add("orientation", fmt.Sprintf("%s-%d", magic[:2], v), rgb(exifChunk(pt.TIFF(magic, 0, []pt.Entry{orientation(pt.TypeShort, float64(v))}))))
		}
	}
	add("orientation", "short-65535", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, 65535)}))))

	for _, c := range []struct {
		name  string
		entry pt.Entry
	}{
		{"byte-6", orientation(pt.TypeByte, 6)},
		{"ascii-6", pt.Entry{Tag: 0x0112, Type: pt.TypeASCII, Raw: []byte("6\x00")}},
		{"long-6", orientation(pt.TypeLong, 6)},
		{"rational-6-1", orientation(pt.TypeRational, 6, 1)},
		{"rational-12-2", orientation(pt.TypeRational, 12, 2)},
		{"rational-13-2", orientation(pt.TypeRational, 13, 2)},
		{"rational-6-0", orientation(pt.TypeRational, 6, 0)},
		{"srational-neg6-neg1", orientation(pt.TypeSRational, -6, -1)},
		{"srational-6-neg1", orientation(pt.TypeSRational, 6, -1)},
		{"float-6", orientation(pt.TypeFloat, 6)},
		{"float-6.5", orientation(pt.TypeFloat, 6.5)},
		{"double-8", orientation(pt.TypeDouble, 8)},
		{"sbyte-6", orientation(pt.TypeSByte, 6)},
		{"sbyte-neg", orientation(pt.TypeSByte, -3)},
		{"sshort-6", orientation(pt.TypeSShort, 6)},
		{"slong-3", orientation(pt.TypeSLong, 3)},
		{"ifd-8", orientation(pt.TypeIFD, 8)},
		{"long8-5", orientation(pt.TypeLong8, 5)},
		{"undefined-6", orientation(pt.TypeUndefined, 6)},
		{"type-14", pt.Entry{Tag: 0x0112, Type: 14, Raw: []byte{6, 0, 0, 0}, Count: 1}},
		{"type-0", pt.Entry{Tag: 0x0112, Type: 0, Raw: []byte{6, 0, 0, 0}, Count: 1}},
		{"short-count-2", orientation(pt.TypeShort, 6, 1)},
		{"short-count-0", pt.Entry{Tag: 0x0112, Type: pt.TypeShort, Raw: []byte{}, Count: 0}},
		{"long-count-3", orientation(pt.TypeLong, 8, 1, 1)},
		{"short-count-huge", pt.Entry{Tag: 0x0112, Type: pt.TypeShort, Raw: []byte{6, 0, 0, 0}, Count: 0x4000_0000}},
	} {
		add("types", c.name, rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{c.entry}))))
	}

	bad := uint32(5000)
	add("ifd", "bad-offset-before", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{
		{Tag: 0x010F, Type: pt.TypeASCII, Raw: []byte("Maker Inc.\x00"), Offset: &bad},
		orientation(pt.TypeShort, 6),
	}))))
	add("ifd", "bad-offset-after", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{
		orientation(pt.TypeShort, 6),
		{Tag: 0x010F, Type: pt.TypeASCII, Raw: []byte("Maker Inc.\x00"), Offset: &bad},
	}))))
	add("ifd", "duplicate", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, 3), orientation(pt.TypeShort, 6)}))))
	block := pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, 6)})
	add("ifd", "missing-next-offset", rgb(exifChunk(block[:len(block)-4])))
	add("ifd", "truncated-entry", rgb(exifChunk(block[:len(block)-8])))
	add("ifd", "ifd-offset-beyond", rgb(exifChunk(pt.TIFF("II*\x00", 900, nil)[:8])))
	add("ifd", "ifd-offset-zero", rgb(exifChunk(append([]byte("II*\x00\x00\x00\x00\x00"), 0, 0))))
	add("ifd", "exif-prefix", rgb(exifChunk(append([]byte("Exif\x00\x00"), block...))))
	add("ifd", "empty", rgb(exifChunk(nil)))

	for name, head := range map[string]string{
		"mm-2a00": "MM\x2a\x00", "ii-002a": "II\x00\x2a", "mm-bigtiff-magic": "MM\x00\x2b",
	} {
		var order interface {
			binary.ByteOrder
			binary.AppendByteOrder
		} = binary.LittleEndian
		if head[0] == 'M' {
			order = binary.BigEndian
		}
		data := append([]byte(head), order.AppendUint32(nil, 8)...)
		data = order.AppendUint16(data, 1)
		data = order.AppendUint16(data, 0x0112)
		data = order.AppendUint16(data, 3)
		data = order.AppendUint32(data, 1)
		data = order.AppendUint16(data, 8)
		data = append(data, 0, 0, 0, 0, 0, 0)
		add("header", name, rgb(exifChunk(data)))
	}
	add("header", "ii-bigtiff", rgb(exifChunk([]byte("II\x2b\x00\x08\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00\x00"))))
	add("header", "invalid", rgb(exifChunk([]byte("XX*\x00\x08\x00\x00\x00"))))
	add("header", "five-bytes", rgb(exifChunk([]byte("II*\x00\x08"))))
	add("header", "three-bytes", rgb(exifChunk([]byte("II*"))))

	xmpAttr := func(v string) []byte {
		return []byte(`<x:xmpmeta><rdf:Description tiff:Orientation="` + v + `"/></x:xmpmeta>`)
	}
	itxt := func(x []byte) []byte {
		return pt.Chunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"), x...))
	}
	add("xmp", "attr-3", rgb(itxt(xmpAttr("3"))))
	add("xmp", "element-8", rgb(itxt([]byte("<tiff:Orientation>8</tiff:Orientation>"))))
	add("xmp", "attr-9", rgb(itxt(xmpAttr("9"))))
	add("xmp", "single-quotes", rgb(itxt([]byte(`tiff:Orientation='6'`))))
	add("xmp", "spaced", rgb(itxt([]byte(`tiff:Orientation = "6"`))))
	add("xmp", "exif-wins", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, 1)})), itxt(xmpAttr("6"))))
	add("xmp", "exif-without-orientation", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{{Tag: 0x010F, Type: pt.TypeASCII, Raw: []byte("M\x00")}})), itxt(xmpAttr("6"))))
	add("xmp", "exif-orientation-bad-type", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{{Tag: 0x0112, Type: 14, Raw: []byte{1, 0, 0, 0}, Count: 1}})), itxt(xmpAttr("7"))))
	add("xmp", "text-chunk-str", rgb(pt.Chunk("tEXt", append([]byte("XML:com.adobe.xmp\x00"), xmpAttr("5")...))))
	add("xmp", "two-matches", rgb(itxt([]byte(`tiff:Orientation>2<tiff:Orientation="8"`))))

	hexBlock := hex.EncodeToString(append([]byte("Exif\x00\x00"), pt.TIFF("MM\x00*", 0, []pt.Entry{orientation(pt.TypeShort, 6)})...))
	spaced := strings.Join(splitEvery(hexBlock, 2), " ")
	raw := func(body string) []byte {
		return pt.Chunk("tEXt", []byte("Raw profile type exif\x00\nexif\n  40\n"+body))
	}
	add("rawprofile", "hex-lines", rgb(raw(hexBlock[:30]+"\n"+hexBlock[30:]+"\n")))
	add("rawprofile", "hex-spaced", rgb(raw(spaced)))
	add("rawprofile", "hex-odd", rgb(raw(hexBlock[:len(hexBlock)-1])))
	add("rawprofile", "hex-space-inside-byte", rgb(raw("4 5"+hexBlock)))
	add("rawprofile", "short", rgb(pt.Chunk("tEXt", []byte("Raw profile type exif\x00\nexif"))))
	add("rawprofile", "exif-key-wins", rgb(exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, 1)})), raw(hexBlock)))

	// Orientation 6 or 5 plus tags Exif.tobytes has to rewrite.
	withTags := func(extra ...pt.Entry) []byte {
		return rgb(exifChunk(pt.TIFF("II*\x00", 0, append([]pt.Entry{orientation(pt.TypeShort, 6)}, extra...))))
	}
	add("tobytes", "make-model", withTags(
		pt.Entry{Tag: 0x010F, Type: pt.TypeASCII, Raw: []byte("Camera Maker\x00")},
		pt.Entry{Tag: 0x0110, Type: pt.TypeASCII, Raw: []byte("Model X\x00")}))
	add("tobytes", "resolution", withTags(
		pt.Entry{Tag: 0x011A, Type: pt.TypeRational, Values: []float64{72, 1}},
		pt.Entry{Tag: 0x0128, Type: pt.TypeShort, Values: []float64{2}}))
	add("tobytes", "resolution-unit-long-big", withTags(pt.Entry{Tag: 0x0128, Type: pt.TypeLong, Values: []float64{70000}}))
	add("tobytes", "xresolution-short", withTags(pt.Entry{Tag: 0x011A, Type: pt.TypeShort, Values: []float64{72}}))
	add("tobytes", "make-as-short", withTags(pt.Entry{Tag: 0x010F, Type: pt.TypeShort, Values: []float64{7}}))
	add("tobytes", "unknown-tag-undefined", withTags(pt.Entry{Tag: 0xC000, Type: pt.TypeUndefined, Raw: []byte("abcdef")}))
	add("tobytes", "rational-zero-denominator", withTags(pt.Entry{Tag: 0x011A, Type: pt.TypeRational, Values: []float64{72, 0}}))
	add("tobytes", "srational-tag", withTags(pt.Entry{Tag: 0x9204, Type: pt.TypeSRational, Values: []float64{-1, 3}}))
	add("tobytes", "float-double", withTags(
		pt.Entry{Tag: 0xC001, Type: pt.TypeFloat, Values: []float64{1.5}},
		pt.Entry{Tag: 0xC002, Type: pt.TypeDouble, Values: []float64{2.25}}))
	add("tobytes", "exif-ifd-pointer", exifWithSubIFD(r, rgb))
	add("tobytes", "exif-ifd-pointer-count-2", withTags(pt.Entry{Tag: 0x8769, Type: pt.TypeLong, Values: []float64{8, 8}}))
	add("tobytes", "exif-ifd-pointer-bad", withTags(pt.Entry{Tag: 0x8769, Type: pt.TypeLong, Values: []float64{9999}}))
	add("tobytes", "gps-pointer-short", withTags(pt.Entry{Tag: 0x8825, Type: pt.TypeShort, Values: []float64{8}}))
	add("tobytes", "byte-tag", withTags(pt.Entry{Tag: 0xC003, Type: pt.TypeByte, Values: []float64{1, 2, 3, 4, 5}}))
	add("tobytes", "strip-offsets", withTags(
		pt.Entry{Tag: 0x0111, Type: pt.TypeLong, Values: []float64{1, 2}},
		pt.Entry{Tag: 0x0117, Type: pt.TypeLong, Values: []float64{1, 2}}))
	add("tobytes", "xmp-orientation-with-exif", rgb(
		exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{{Tag: 0x010F, Type: pt.TypeASCII, Raw: []byte("M\x00")}})),
		itxt(xmpAttr("6"))))

	// Every mode through each transpose.
	modes := []struct {
		depth, ct byte
	}{{1, 0}, {2, 0}, {8, 0}, {16, 0}, {8, 2}, {16, 2}, {4, 3}, {8, 3}, {8, 4}, {16, 4}, {8, 6}, {16, 6}}
	for _, m := range modes {
		for v := 2; v <= 8; v++ {
			block := pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, float64(v))})
			add("modes", fmt.Sprintf("d%d-c%d-o%d", m.depth, m.ct, v),
				container(r, 7, 4, m.depth, m.ct, v%2 == 0, [][]byte{exifChunk(block)}, nil))
		}
	}
	add("modes", "exif-after-idat", container(r, 6, 9, 8, 2, false, nil, [][]byte{exifChunk(pt.TIFF("MM\x00*", 0, []pt.Entry{orientation(pt.TypeShort, 8)}))}))
	add("modes", "wide", container(r, 300, 2, 8, 6, false, [][]byte{exifChunk(pt.TIFF("II*\x00", 0, []pt.Entry{orientation(pt.TypeShort, 7)}))}, nil))
	return append(cases, tobytesStress(r)...)
}

// withSub appends a sub-IFD (built by pt.TIFF at the end of the block) and
// points tag at it from IFD0.
func withSub(magic string, tag uint16, ifd0 []pt.Entry, sub []pt.Entry) []byte {
	build := func(pointer uint32) []byte {
		return pt.TIFF(magic, 0, append(append([]pt.Entry(nil), ifd0...), pt.Entry{Tag: tag, Type: pt.TypeLong, Values: []float64{float64(pointer)}}))
	}
	head := build(0)
	full := pt.TIFF(magic, uint32(len(head)), sub)
	return append(build(uint32(len(head))), full[len(head):]...)
}

// tobytesStress makes Exif.tobytes rewrite tags whose stored type or value
// does not fit the TiffTags entry, next to orientation 6.
func tobytesStress(r *pt.Rand) []exifCase {
	var cases []exifCase
	o6 := orientation(pt.TypeShort, 6)
	add := func(name string, block []byte) {
		cases = append(cases, exifCase{name: "tobytes2/" + name, class: "tobytes2",
			data: container(r, 4, 3, 8, 2, false, [][]byte{pt.Chunk("eXIf", block)}, nil)})
	}
	e := func(tag, typ uint16, vals ...float64) pt.Entry { return pt.Entry{Tag: tag, Type: typ, Values: vals} }
	ascii := func(tag uint16, s string) pt.Entry {
		return pt.Entry{Tag: tag, Type: pt.TypeASCII, Raw: []byte(s + "\x00")}
	}
	one := func(entries ...pt.Entry) []byte { return pt.TIFF("II*\x00", 0, append([]pt.Entry{o6}, entries...)) }
	nan := math.NaN()
	for _, c := range []struct {
		name  string
		entry pt.Entry
	}{
		{"compression-ascii-enum", ascii(259, "LZW")},
		{"compression-ascii-unknown", ascii(259, "lzw")},
		{"make-rational", e(271, pt.TypeRational, 3, 2)},
		{"make-short", e(271, pt.TypeShort, 7)},
		{"make-float", e(271, pt.TypeFloat, 1.5)},
		{"make-byte", e(271, pt.TypeByte, 65, 66)},
		{"xres-srational-neg", e(282, pt.TypeSRational, -72, 1)},
		{"xres-srational-pos", e(282, pt.TypeSRational, 72, 1)},
		{"xres-float-nan", e(282, pt.TypeFloat, nan)},
		{"xres-float-neg", e(282, pt.TypeFloat, -1.5)},
		{"xres-float-tiny-neg", e(282, pt.TypeDouble, -1e-300)},
		{"xres-float", e(282, pt.TypeFloat, 72.5)},
		{"xres-ascii", ascii(282, "72")},
		{"xres-short", e(282, pt.TypeShort, 72)},
		{"xres-rational-den0", e(282, pt.TypeRational, 72, 0)},
		{"xres-long8-big", e(282, pt.TypeLong8, 1<<62)},
		{"mpf-rational-big", e(45573, pt.TypeRational, 3_000_000_000, 1)},
		{"mpf-rational-den0-big", e(45573, pt.TypeRational, 3_000_000_000, 0)},
		{"mpf-rational-den0-small", e(45573, pt.TypeRational, 5, 0)},
		{"mpf-ascii-fraction", ascii(45573, "1/60")},
		{"mpf-ascii-decimal", ascii(45573, " -2.5e1 ")},
		{"mpf-ascii-bad", ascii(45573, "abc")},
		{"mpf-ascii-zero-den", ascii(45573, "1/0")},
		{"mpf-float-inf", e(45573, pt.TypeFloat, math.Inf(1))},
		{"mpf-undefined", pt.Entry{Tag: 45573, Type: pt.TypeUndefined, Raw: []byte{1, 2}}},
		{"mpf-srational-neg", e(45573, pt.TypeSRational, -1, 3)},
		{"mpf-slong", e(45573, pt.TypeSLong, -7)},
		{"undefined-len0-short3", e(45059, pt.TypeShort, 1, 2, 3)},
		{"undefined-short3", e(347, pt.TypeShort, 1, 2, 3)},
		{"undefined-float", e(347, pt.TypeFloat, 2.5)},
		{"undefined-rational-den0", e(347, pt.TypeRational, 5, 0)},
		{"undefined-rational", e(347, pt.TypeRational, 9, 2)},
		{"undefined-ascii-latin1", pt.Entry{Tag: 347, Type: pt.TypeASCII, Raw: []byte("h\xe9llo\x00")}},
		{"xmp-byte-short300", e(700, pt.TypeShort, 300)},
		{"xmp-byte-short5", e(700, pt.TypeShort, 5)},
		{"xmp-byte-short-pair", e(700, pt.TypeShort, 5, 6)},
		{"xmp-byte-ascii", ascii(700, "abc")},
		{"xmp-byte-rational", e(700, pt.TypeRational, 7, 2)},
		{"bits-long-big", e(258, pt.TypeLong, 70000, 8, 8)},
		{"bits-sshort-neg", e(258, pt.TypeSShort, -1)},
		{"bits-float", e(258, pt.TypeFloat, 8)},
		{"width-slong-neg", e(256, pt.TypeSLong, -5)},
		{"width-long8-big", e(256, pt.TypeLong8, 1<<40)},
		{"double-rational-den0", e(340, pt.TypeRational, 1, 0)},
		{"double-ascii", ascii(340, "1.5")},
		{"double-long", e(340, pt.TypeLong, 7)},
		{"unknown-long8-big", e(0xC100, pt.TypeLong8, 1<<33)},
		{"unknown-slong-neg", e(0xC101, pt.TypeSLong, -100000)},
		{"unknown-sshort-min", e(0xC102, pt.TypeSShort, -32768)},
		{"unknown-rational-mixed-sign", e(0xC103, pt.TypeSRational, 1, 2, -3, 4)},
		{"unknown-float", e(0xC104, pt.TypeFloat, 3.25)},
		{"strip-single-overflow", e(273, pt.TypeLong, 4_294_967_290)},
		{"strip-pair-overflow", e(273, pt.TypeLong, 1, 4_294_967_290)},
		{"strip-small", e(273, pt.TypeLong, 100)},
		{"exif-pointer-negative", e(34665, pt.TypeSLong, -8)},
		{"exif-pointer-float", e(34665, pt.TypeFloat, 8)},
		{"exif-pointer-long8-top-bit", pt.Entry{Tag: 34665, Type: pt.TypeLong8, Raw: []byte{0, 0, 0, 0, 0, 0, 0, 0x80}}},
		{"gps-pointer-ascii", ascii(34853, "8")},
	} {
		add(c.name, one(c.entry))
		if strings.HasPrefix(c.name, "xres") || strings.HasPrefix(c.name, "strip") || strings.HasPrefix(c.name, "mpf-ascii") {
			add(c.name+"-mm", pt.TIFF("MM\x00*", 0, []pt.Entry{o6, c.entry}))
		}
	}
	add("exif-inner-long8-big", withSub("II*\x00", 34665, []pt.Entry{o6}, []pt.Entry{e(0xA431, pt.TypeLong8, 1<<33)}))
	add("exif-inner-version-float", withSub("II*\x00", 34665, []pt.Entry{o6}, []pt.Entry{e(36864, pt.TypeFloat, 2.5)}))
	add("exif-inner-version-short4", withSub("II*\x00", 34665, []pt.Entry{o6}, []pt.Entry{e(36864, pt.TypeShort, 0, 2, 3, 2)}))
	add("exif-inner-ok", withSub("MM\x00*", 34665, []pt.Entry{o6}, []pt.Entry{e(0x829A, pt.TypeRational, 1, 60), e(0x8827, pt.TypeShort, 100), ascii(0x9003, "2026:09:16 10:00:00")}))
	interop := func(entry pt.Entry) []byte {
		ifd0 := []pt.Entry{o6}
		probe := withSub("II*\x00", 34665, ifd0, []pt.Entry{{Tag: 40965, Type: pt.TypeLong, Values: []float64{0}}})
		sub := pt.TIFF("II*\x00", uint32(len(probe)), []pt.Entry{entry})
		block := withSub("II*\x00", 34665, ifd0, []pt.Entry{{Tag: 40965, Type: pt.TypeLong, Values: []float64{float64(len(probe))}}})
		return append(block, sub[len(probe):]...)
	}
	add("interop-rational", interop(e(1, pt.TypeRational, 1, 3)))
	add("interop-ascii", interop(ascii(1, "R98")))
	add("interop-short", interop(e(1, pt.TypeShort, 98)))
	add("gps-latitude-srational-neg", withSub("II*\x00", 34853, []pt.Entry{o6}, []pt.Entry{e(2, pt.TypeSRational, -1, 1, 2, 1, 3, 1)}))
	add("gps-latitude-rational", withSub("II*\x00", 34853, []pt.Entry{o6}, []pt.Entry{e(2, pt.TypeRational, 1, 1, 2, 1, 3, 1)}))
	add("gps-version-short4", withSub("II*\x00", 34853, []pt.Entry{o6}, []pt.Entry{e(0, pt.TypeShort, 2, 2, 0, 0)}))
	add("gps-altitude-ascii", withSub("II*\x00", 34853, []pt.Entry{o6}, []pt.Entry{ascii(6, "12")}))

	xmp := pt.Chunk("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00tiff:Orientation=\"6\""))
	bad := pt.TIFF("II*\x00", 0, []pt.Entry{e(271, pt.TypeRational, 3, 2)})
	cases = append(cases,
		exifCase{name: "tobytes2/xmp-orientation-bad-tag", class: "tobytes2",
			data: container(r, 4, 3, 8, 2, false, [][]byte{pt.Chunk("eXIf", bad), xmp}, nil)},
		exifCase{name: "tobytes2/xmp-orientation-good-tags", class: "tobytes2",
			data: container(r, 4, 3, 8, 2, false, [][]byte{pt.Chunk("eXIf", pt.TIFF("II*\x00", 0, []pt.Entry{ascii(271, "Maker")})), xmp}, nil)},
	)
	rawBad := hex.EncodeToString(one(e(271, pt.TypeRational, 3, 2)))
	cases = append(cases, exifCase{name: "tobytes2/raw-profile-bad-tag", class: "tobytes2",
		data: container(r, 4, 3, 8, 2, false, [][]byte{pt.Chunk("tEXt", []byte("Raw profile type exif\x00\nexif\n 1\n"+rawBad))}, nil)})
	return cases
}

// tiffCorpus is small uncompressed TIFF files whose IFD0 carries the
// orientation, for getexif through tag_v2.
func tiffCorpus() []exifCase {
	var cases []exifCase
	build := func(magic string, orient pt.Entry) []byte {
		entries := func(strip uint32) []pt.Entry {
			return []pt.Entry{
				{Tag: 256, Type: pt.TypeShort, Values: []float64{3}},
				{Tag: 257, Type: pt.TypeShort, Values: []float64{2}},
				{Tag: 258, Type: pt.TypeShort, Values: []float64{8}},
				{Tag: 259, Type: pt.TypeShort, Values: []float64{1}},
				{Tag: 262, Type: pt.TypeShort, Values: []float64{1}},
				{Tag: 273, Type: pt.TypeLong, Values: []float64{float64(strip)}},
				orient,
				{Tag: 277, Type: pt.TypeShort, Values: []float64{1}},
				{Tag: 278, Type: pt.TypeShort, Values: []float64{2}},
				{Tag: 279, Type: pt.TypeLong, Values: []float64{6}},
			}
		}
		head := pt.TIFF(magic, 0, entries(0))
		return append(pt.TIFF(magic, 0, entries(uint32(len(head)))), 10, 20, 30, 40, 50, 60)
	}
	for _, magic := range []string{"II*\x00", "MM\x00*"} {
		for v := 1; v <= 8; v++ {
			cases = append(cases, exifCase{name: fmt.Sprintf("tiff/%s-short-%d", magic[:2], v), class: "tiff", data: build(magic, orientation(pt.TypeShort, float64(v)))})
		}
	}
	for name, entry := range map[string]pt.Entry{
		"rational-6-1":  orientation(pt.TypeRational, 6, 1),
		"float-6":       orientation(pt.TypeFloat, 6),
		"ascii-6":       {Tag: 274, Type: pt.TypeASCII, Raw: []byte("6\x00")},
		"byte-6":        orientation(pt.TypeByte, 6),
		"short-count-2": orientation(pt.TypeShort, 8, 1),
		"srational":     orientation(pt.TypeSRational, -3, -1),
		"undefined":     orientation(pt.TypeUndefined, 6),
	} {
		cases = append(cases, exifCase{name: "tiff/II-" + name, class: "tiff", data: build("II*\x00", entry)})
	}
	return cases
}

// exifWithSubIFD builds orientation 6 in IFD0 plus an Exif IFD holding
// DateTimeOriginal and a MakerNote.
func exifWithSubIFD(r *pt.Rand, rgb func(...[]byte) []byte) []byte {
	build := func(pointer uint32) []byte {
		return pt.TIFF("II*\x00", 0, []pt.Entry{
			orientation(pt.TypeShort, 6),
			{Tag: 0x8769, Type: pt.TypeLong, Values: []float64{float64(pointer)}},
		})
	}
	ifd0 := build(0)
	sub := binary.LittleEndian.AppendUint16(nil, 2)
	date := []byte("2026:09:16 10:11:12\x00")
	maker := []byte("MAKERNOTE-DATA")
	dataStart := len(ifd0) + 2 + 2*12 + 4
	sub = binary.LittleEndian.AppendUint16(sub, 0x9003)
	sub = binary.LittleEndian.AppendUint16(sub, pt.TypeASCII)
	sub = binary.LittleEndian.AppendUint32(sub, uint32(len(date)))
	sub = binary.LittleEndian.AppendUint32(sub, uint32(dataStart))
	sub = binary.LittleEndian.AppendUint16(sub, 0x927C)
	sub = binary.LittleEndian.AppendUint16(sub, pt.TypeUndefined)
	sub = binary.LittleEndian.AppendUint32(sub, uint32(len(maker)))
	sub = binary.LittleEndian.AppendUint32(sub, uint32(dataStart+len(date)))
	sub = binary.LittleEndian.AppendUint32(sub, 0)
	sub = append(append(sub, date...), maker...)
	block := append(build(uint32(len(ifd0))), sub...)
	return rgb(pt.Chunk("eXIf", block))
}

func splitEvery(s string, n int) []string {
	var out []string
	for len(s) > n {
		out = append(out, s[:n])
		s = s[n:]
	}
	return append(out, s)
}

type goOutcome struct {
	class  string
	detail string
	img    *pil.Image
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

func goPixels(data []byte) (goOutcome, string, []byte) {
	opened, err := pngdec.Open(data)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	w, h := opened.Size()
	if err := pil.CheckBomb(w, h); err != nil {
		return goOutcome{class: "bomb", detail: err.Error()}, "", nil
	}
	img, err := opened.Load()
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	transposed, err := exif.Transpose(img)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	mode, pix, err := convert.ForEncoder(transposed)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	return goOutcome{class: "ok", img: transposed}, mode, pix
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
