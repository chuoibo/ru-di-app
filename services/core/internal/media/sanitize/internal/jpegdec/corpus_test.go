package jpegdec

import (
	"bytes"
	"fmt"
)

// goCase is an input the tests build in Go from fixed parameters and seeds.
type goCase struct {
	class, name string
	// golden marks the cases the committed golden file pins.
	golden bool
	build  func() []byte
}

func yccSpec(w, h int, seed uint64, pattern string, factors [][2]int) wSpec {
	s := wSpec{width: w, height: h, quality: 85, jfif: true, adobe: -1}
	for i, f := range factors {
		tq := 0
		if i > 0 {
			tq = 1
		}
		s.comps = append(s.comps, wComp{id: i + 1, h: f[0], v: f[1], tq: tq})
		s.planes = append(s.planes, testPlane(w, h, seed+uint64(i)*7, pattern))
	}
	return s
}

// scanEnd is the offset of the EOI a buildJPEG file ends with.
func scanEnd(data []byte) int { return len(data) - 2 }

// segmentBoundaries lists the offsets where each marker segment before SOS
// starts, plus the SOS payload end.
func segmentBoundaries(data []byte) []int {
	var cuts []int
	pos := 2
	for pos+4 <= len(data) && data[pos] == 0xFF {
		cuts = append(cuts, pos)
		marker := data[pos+1]
		length := int(data[pos+2])<<8 | int(data[pos+3])
		pos += 2 + length
		if marker == 0xDA {
			cuts = append(cuts, pos)
			break
		}
	}
	return cuts
}

func withSpec(f func(*wSpec), s wSpec) wSpec {
	f(&s)
	return s
}

func goCases() []goCase {
	var cases []goCase
	add := func(class, name string, golden bool, build func() []byte) {
		cases = append(cases, goCase{class: class, name: name, golden: golden, build: build})
	}

	// Sampling factors, including the upsamplers libjpeg picks per ratio
	// and the ones it refuses.
	samplings := []struct {
		name    string
		factors [][2]int
	}{
		{"gray", [][2]int{{1, 1}}},
		{"gray-2x2", [][2]int{{2, 2}}},
		{"444", [][2]int{{1, 1}, {1, 1}, {1, 1}}},
		{"422", [][2]int{{2, 1}, {1, 1}, {1, 1}}},
		{"420", [][2]int{{2, 2}, {1, 1}, {1, 1}}},
		{"440", [][2]int{{1, 2}, {1, 1}, {1, 1}}},
		{"411", [][2]int{{4, 1}, {1, 1}, {1, 1}}},
		{"410", [][2]int{{4, 2}, {1, 1}, {1, 1}}},
		{"luma-sub", [][2]int{{1, 1}, {2, 2}, {2, 2}}},
		{"mixed", [][2]int{{2, 2}, {1, 2}, {2, 1}}},
		{"311", [][2]int{{3, 1}, {1, 1}, {1, 1}}},
		{"33-11", [][2]int{{3, 3}, {1, 1}, {1, 1}}},
		{"32-11", [][2]int{{3, 2}, {1, 1}, {1, 1}}},
		{"221", [][2]int{{2, 2}, {2, 2}, {1, 1}}},
		{"44-too-many-blocks", [][2]int{{4, 4}, {1, 1}, {1, 1}}},
		{"32-21-fract", [][2]int{{3, 2}, {2, 1}, {1, 1}}},
		{"42-22", [][2]int{{4, 2}, {2, 2}, {1, 1}}},
	}
	sizes := [][2]int{{1, 1}, {2, 3}, {7, 5}, {17, 9}, {33, 31}, {64, 64}, {131, 37}, {8, 200}, {300, 2}}
	for si, sp := range samplings {
		for zi, size := range sizes {
			sp, size := sp, size
			seed := uint64(si*100 + zi)
			pattern := "smooth"
			if zi%3 == 1 {
				pattern = "noise"
			}
			golden := (sp.name == "420" || sp.name == "mixed" || sp.name == "411") && zi == 4
			add("go-sampling", fmt.Sprintf("%s-%dx%d", sp.name, size[0], size[1]), golden, func() []byte {
				return buildJPEG(yccSpec(size[0], size[1], seed, pattern, sp.factors))
			})
		}
	}

	// Colour spaces chosen by default_decompress_parms.
	color := func(name string, golden bool, f func(*wSpec)) {
		add("go-colorspace", name, golden, func() []byte {
			s := yccSpec(40, 23, 7, "smooth", [][2]int{{2, 2}, {1, 1}, {1, 1}})
			f(&s)
			return buildJPEG(s)
		})
	}
	color("adobe-rgb", true, func(s *wSpec) { s.jfif, s.adobe = false, 0 })
	color("ids-rgb", false, func(s *wSpec) {
		s.jfif = false
		s.comps[0].id, s.comps[1].id, s.comps[2].id = 82, 71, 66
	})
	color("ids-123-no-jfif", false, func(s *wSpec) { s.jfif = false })
	color("ids-other-no-jfif", false, func(s *wSpec) {
		s.jfif = false
		s.comps[0].id, s.comps[1].id, s.comps[2].id = 9, 8, 7
	})
	color("adobe-transform-5-rgb", false, func(s *wSpec) { s.jfif, s.adobe = false, 5 })
	cmyk := func(name string, golden bool, adobe int, factors [][2]int) {
		add("go-colorspace", name, golden, func() []byte {
			s := yccSpec(29, 18, 11, "smooth", factors)
			s.jfif, s.adobe = false, adobe
			return buildJPEG(s)
		})
	}
	cmyk("cmyk-adobe0", true, 0, [][2]int{{1, 1}, {1, 1}, {1, 1}, {1, 1}})
	cmyk("ycck-adobe2", true, 2, [][2]int{{2, 2}, {1, 1}, {1, 1}, {2, 2}})
	cmyk("ycck-adobe1", false, 1, [][2]int{{1, 1}, {1, 1}, {1, 1}, {1, 1}})
	cmyk("cmyk-no-adobe", false, -1, [][2]int{{1, 1}, {1, 1}, {1, 1}, {1, 1}})
	cmyk("two-components", false, -1, [][2]int{{1, 1}, {1, 1}})

	// Restart intervals, well-formed and broken.
	restart := func(name string, golden bool, interval int, mutate func([]byte) []byte) {
		add("go-restart", name, golden, func() []byte {
			s := yccSpec(97, 61, 13, "noise", [][2]int{{2, 2}, {1, 1}, {1, 1}})
			s.restart = interval
			data := buildJPEG(s)
			if mutate != nil {
				data = mutate(data)
			}
			return data
		})
	}
	restart("every-1", false, 1, nil)
	restart("every-2", true, 2, nil)
	restart("every-7", false, 7, nil)
	rstPositions := func(data []byte) []int {
		start := segmentBoundaries(data)
		from := start[len(start)-1]
		var at []int
		for i := from; i+1 < len(data); i++ {
			if data[i] == 0xFF && data[i+1] >= 0xD0 && data[i+1] <= 0xD7 {
				at = append(at, i)
			}
		}
		return at
	}
	for shift := 1; shift <= 5; shift++ {
		shift := shift
		restart(fmt.Sprintf("renumbered-%d", shift), shift == 2, 3, func(data []byte) []byte {
			out := append([]byte(nil), data...)
			pos := rstPositions(out)
			if len(pos) > 2 {
				out[pos[2]+1] = 0xD0 + (out[pos[2]+1]-0xD0+byte(shift))&7
			}
			return out
		})
	}
	restart("missing-rst", false, 3, func(data []byte) []byte {
		pos := rstPositions(data)
		out := append([]byte(nil), data[:pos[1]]...)
		return append(out, data[pos[1]+2:]...)
	})
	restart("garbage-before-rst", false, 3, func(data []byte) []byte {
		pos := rstPositions(data)
		out := append([]byte(nil), data[:pos[1]]...)
		out = append(out, 0x12, 0x34, 0x56, 0x00, 0x99)
		return append(out, data[pos[1]:]...)
	})
	restart("rst-early", false, 3, func(data []byte) []byte {
		pos := rstPositions(data)
		out := append([]byte(nil), data[:pos[1]-20]...)
		return append(out, data[pos[1]:]...)
	})
	restart("non-rst-marker-in-scan", false, 3, func(data []byte) []byte {
		out := append([]byte(nil), data...)
		pos := rstPositions(out)
		out[pos[2]+1] = 0xE5
		return out
	})

	// Marker-level quirks of both parsers.
	base := func() wSpec { return yccSpec(35, 27, 17, "smooth", [][2]int{{2, 1}, {1, 1}, {1, 1}}) }
	marker := func(name string, golden bool, mutate func([]byte) []byte) {
		add("go-markers", name, golden, func() []byte { return mutate(buildJPEG(base())) })
	}
	insertAt := func(data []byte, at int, chunk []byte) []byte {
		out := append([]byte(nil), data[:at]...)
		out = append(out, chunk...)
		return append(out, data[at:]...)
	}
	marker("tables-only-prefix", true, func(d []byte) []byte {
		prefix := []byte{0xFF, 0xD8}
		cuts := segmentBoundaries(d)
		for _, at := range cuts {
			if d[at+1] == 0xDB || d[at+1] == 0xC4 {
				length := int(d[at+2])<<8 | int(d[at+3])
				prefix = append(prefix, d[at:at+2+length]...)
			}
		}
		prefix = append(prefix, 0xFF, 0xD9)
		return append(prefix, d...)
	})
	marker("tables-only-then-junk", false, func(d []byte) []byte {
		return append([]byte{0xFF, 0xD8, 0xFF, 0xD9, 0x00}, d...)
	})
	marker("fill-bytes", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[2], []byte{0xFF, 0xFF, 0xFF})
	})
	marker("ff00-junk", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[2], []byte{0xFF, 0x00, 0x33})
	})
	marker("plain-junk", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], []byte("junk between segments"))
	})
	marker("tem-marker", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], []byte{0xFF, 0x01})
	})
	marker("rst-in-header", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], []byte{0xFF, 0xD3})
	})
	for _, m := range []byte{0xC8, 0xF0, 0xFD, 0xDE, 0xDF, 0xC5, 0xCD, 0xDC} {
		m := m
		marker(fmt.Sprintf("segment-%02x", m), false, func(d []byte) []byte {
			cuts := segmentBoundaries(d)
			return insertAt(d, cuts[1], segment16(m, []byte{0, 4, 1, 2}))
		})
	}
	marker("app-length-0", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], []byte{0xFF, 0xE3, 0x00, 0x00})
	})
	marker("app-length-1", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], []byte{0xFF, 0xE3, 0x00, 0x01})
	})
	marker("app-past-end", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return append(append([]byte(nil), d[:cuts[1]]...), 0xFF, 0xE4, 0x7F, 0xFF, 1, 2, 3)
	})
	marker("comment", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], segment16(0xFE, bytes.Repeat([]byte("comment "), 500)))
	})
	marker("dri-bad-length", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], segment16(0xDD, []byte{0, 1, 0}))
	})
	marker("dqt-index-5", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], segment16(0xDB, append([]byte{5}, bytes.Repeat([]byte{1}, 64)...)))
	})
	marker("dqt-short", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		return insertAt(d, cuts[1], segment16(0xDB, append([]byte{2}, bytes.Repeat([]byte{1}, 30)...)))
	})
	marker("dht-overfull", false, func(d []byte) []byte {
		cuts := segmentBoundaries(d)
		payload := append([]byte{0x02, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 1, 2, 3)
		return insertAt(d, cuts[1], segment16(0xC4, payload))
	})
	marker("sof-extra-bytes", false, func(d []byte) []byte {
		out := append([]byte(nil), d...)
		for _, at := range segmentBoundaries(out) {
			if out[at+1] == 0xC0 {
				length := int(out[at+2])<<8 | int(out[at+3])
				seg := append([]byte(nil), out[at:at+2+length]...)
				seg[3] += 2
				seg = append(seg, 0x11, 0x22)
				return append(append(append([]byte(nil), out[:at]...), seg...), out[at+2+length:]...)
			}
		}
		return out
	})
	sofByte := func(name string, offset int, value byte) {
		marker(name, false, func(d []byte) []byte {
			out := append([]byte(nil), d...)
			for _, at := range segmentBoundaries(out) {
				if out[at+1] == 0xC0 {
					out[at+4+offset] = value
					return out
				}
			}
			return out
		})
	}
	sofByte("sof-12-bit", 0, 12)
	sofByte("sof-height-0", 2, 0)
	sofByte("sof-marker-c3", -3, 0xC3)
	sofByte("sof-marker-c9", -3, 0xC9)
	sofByte("sof-marker-c2", -3, 0xC2)
	sofByte("sof-marker-c1", -3, 0xC1)
	sofByte("sof-sampling-0", 7, 0x01)
	sofByte("sof-quant-table-7", 8, 7)
	marker("duplicate-sof", false, func(d []byte) []byte {
		for _, at := range segmentBoundaries(d) {
			if d[at+1] == 0xC0 {
				length := int(d[at+2])<<8 | int(d[at+3])
				return insertAt(d, at, d[at:at+2+length])
			}
		}
		return d
	})
	marker("sos-reversed-components", false, func(d []byte) []byte {
		out := append([]byte(nil), d...)
		for _, at := range segmentBoundaries(out) {
			if out[at+1] == 0xDA {
				out[at+5], out[at+7] = out[at+7], out[at+5]
				return out
			}
		}
		return out
	})
	marker("sos-dc-table-5", false, func(d []byte) []byte {
		out := append([]byte(nil), d...)
		for _, at := range segmentBoundaries(out) {
			if out[at+1] == 0xDA {
				out[at+6] = 0x50
				return out
			}
		}
		return out
	})
	marker("second-sos-after-scan", false, func(d []byte) []byte {
		var sos []byte
		for _, at := range segmentBoundaries(d) {
			if d[at+1] == 0xDA {
				length := int(d[at+2])<<8 | int(d[at+3])
				sos = d[at : at+2+length]
			}
		}
		return insertAt(d, scanEnd(d), sos)
	})
	marker("trailing-after-eoi", false, func(d []byte) []byte {
		return append(append([]byte(nil), d...), bytes.Repeat([]byte{0xFF, 0xC5, 0, 3}, 10)...)
	})

	// Truncation and the end of the entropy data.
	truncBase := buildJPEG(yccSpec(61, 45, 19, "noise", [][2]int{{2, 2}, {1, 1}, {1, 1}}))
	cutAt := func(name string, golden bool, at int) {
		add("go-truncation", name, golden, func() []byte { return append([]byte(nil), truncBase[:at]...) })
	}
	for i, at := range segmentBoundaries(truncBase) {
		cutAt(fmt.Sprintf("segment-%d", i), false, at)
		cutAt(fmt.Sprintf("segment-%d-plus-3", i), false, at+3)
	}
	scanStart := segmentBoundaries(truncBase)[len(segmentBoundaries(truncBase))-1]
	for _, frac := range []int{1, 25, 50, 90, 99} {
		cutAt(fmt.Sprintf("scan-%d-percent", frac), false, scanStart+(scanEnd(truncBase)-scanStart)*frac/100)
	}
	cutAt("no-eoi", true, len(truncBase)-2)
	cutAt("half-eoi", false, len(truncBase)-1)
	for _, n := range []int{1, 7, 8, 64} {
		n := n
		add("go-truncation", fmt.Sprintf("junk-%d-instead-of-eoi", n), n == 8, func() []byte {
			return append(append([]byte(nil), truncBase[:len(truncBase)-2]...), bytes.Repeat([]byte{0x5A}, n)...)
		})
	}
	add("go-truncation", "junk-then-eoi", false, func() []byte {
		out := append([]byte(nil), truncBase[:len(truncBase)-2]...)
		out = append(out, bytes.Repeat([]byte{0x5A, 0xFF, 0x00}, 30)...)
		return append(out, 0xFF, 0xD9)
	})
	for seed := uint64(0); seed < 8; seed++ {
		seed := seed
		add("go-corrupt-scan", fmt.Sprintf("flip-%d", seed), seed == 3, func() []byte {
			out := append([]byte(nil), truncBase...)
			plane := testPlane(8, 1, seed+500, "noise")
			for i := 0; i < 4; i++ {
				at := scanStart + int(plane[i])*(scanEnd(out)-scanStart)/256
				out[at] ^= plane[4+i] | 1
			}
			return out
		})
	}

	// The finish pass reads markers only from what Pillow has handed over:
	// a bad marker right after the scan fails the load only when it lies
	// in the last 64 KiB block the rows needed.
	for _, delta := range []int{-600, -8, -2, -1, 0, 1, 2, 8, 600} {
		delta := delta
		add("go-finish-limit", fmt.Sprintf("marker-at-block-%+d", delta), delta == -2 || delta == 8, func() []byte {
			s := yccSpec(260, 260, 23, "noise", [][2]int{{1, 1}, {1, 1}, {1, 1}})
			s.quality = 97
			const zeros = 16
			end := scanEnd(buildJPEG(s))
			// Put the bad marker at a block boundary plus delta, after a run
			// of zero bytes the last MCU's lookahead does not reach.
			target := (end/pillowBlock+2)*pillowBlock + delta
			s.extra = padSegments(target - end - zeros)
			out := buildJPEG(s)
			out = append(out[:len(out)-2], make([]byte, zeros)...)
			return append(out, 0xFF, 0xC5, 0x00, 0x04, 0x01, 0x02)
		})
	}

	// Exif blocks read at open time by _read_dpi_from_exif.
	exifCase := func(name string, golden bool, segments ...[]byte) {
		add("go-exif", name, golden, func() []byte {
			s := base()
			s.jfif = false
			s.extra = segments
			return buildJPEG(s)
		})
	}
	exifSeg := func(block []byte) []byte { return segment16(0xE1, append([]byte("Exif\x00\x00"), block...)) }
	unit := tiffEntry{tag: 0x0128, typ: 3, count: 1, value: []byte{2, 0}}
	orient := tiffEntry{tag: 0x0112, typ: 3, count: 1, value: []byte{6, 0}}
	exifCase("rational-dpi", true, exifSeg(tiffBlock(false, []tiffEntry{orient, unit, {tag: 0x011A, typ: 5, count: 1, value: u32(false, 72, 1)}})))
	exifCase("ascii-one-digit", true, exifSeg(tiffBlock(false, []tiffEntry{unit, {tag: 0x011A, typ: 2, count: 2, value: []byte("7\x00")}})))
	exifCase("ascii-two-digits", false, exifSeg(tiffBlock(false, []tiffEntry{unit, {tag: 0x011A, typ: 2, count: 3, value: []byte("72\x00")}})))
	exifCase("ascii-empty", false, exifSeg(tiffBlock(false, []tiffEntry{unit, {tag: 0x011A, typ: 2, count: 1, value: []byte{0}}})))
	exifCase("byte-one", false, exifSeg(tiffBlock(false, []tiffEntry{unit, {tag: 0x011A, typ: 1, count: 1, value: []byte{9}}})))
	exifCase("undefined-two", false, exifSeg(tiffBlock(true, []tiffEntry{unit, {tag: 0x011A, typ: 7, count: 2, value: []byte{9, 3}}})))
	exifCase("no-unit", false, exifSeg(tiffBlock(false, []tiffEntry{{tag: 0x011A, typ: 1, count: 1, value: []byte{9}}})))
	exifCase("with-jfif-dpi", false, segment16(0xE0, []byte{'J', 'F', 'I', 'F', 0, 1, 1, 1, 0, 72, 0, 72, 0, 0}),
		exifSeg(tiffBlock(false, []tiffEntry{unit, {tag: 0x011A, typ: 1, count: 1, value: []byte{9}}})))
	exifCase("bigtiff-head", false, exifSeg([]byte("II\x2b\x00\x08\x00\x00\x00\x00\x00\x00\x00")))
	exifCase("truncated-ifd", false, exifSeg(tiffBlock(false, []tiffEntry{unit, {tag: 0x011A, typ: 2, count: 2, value: []byte("7\x00")}})[:20]))
	exifCase("two-exif-segments", true,
		exifSeg(tiffBlock(false, []tiffEntry{orient})),
		exifSeg([]byte("second segment payload")))
	exifCase("xmp", false, segment16(0xE1, append(append([]byte(nil), xmpPrefix...), []byte(`<x tiff:Orientation="3"/>`)...)))

	// MP index handling in jpeg_factory.
	mpCase := func(name string, golden bool, payload []byte, extra ...[]byte) {
		add("go-mpo", name, golden, func() []byte {
			s := base()
			s.extra = append([][]byte{segment16(0xE2, append([]byte("MPF\x00"), payload...))}, extra...)
			return buildJPEG(s)
		})
	}
	mpIndex := func(bigEndian bool, quant tiffEntry, entries []byte) []byte {
		return tiffBlock(bigEndian, []tiffEntry{
			{tag: 0xB000, typ: 7, count: 4, value: []byte("0100")},
			quant,
			{tag: 0xB002, typ: 7, count: uint32(len(entries)), value: entries},
		})
	}
	mpCase("two-images", true, mpIndex(false, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(false, 2)}, mpEntries(false, []uint32{0, 0})))
	mpCase("two-images-big-endian", false, mpIndex(true, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(true, 2)}, mpEntries(true, []uint32{0, 0})))
	mpCase("one-image", false, mpIndex(false, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(false, 1)}, mpEntries(false, []uint32{0})))
	mpCase("short-entries", true, mpIndex(false, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(false, 3)}, mpEntries(false, []uint32{0, 0})))
	mpCase("format-not-jpeg", false, mpIndex(false, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(false, 2)}, mpEntries(false, []uint32{0, 1})))
	mpCase("quant-ascii", false, mpIndex(false, tiffEntry{tag: 0xB001, typ: 2, count: 2, value: []byte("2\x00")}, mpEntries(false, []uint32{0, 0})))
	mpCase("quant-short-pair", false, mpIndex(false, tiffEntry{tag: 0xB001, typ: 3, count: 2, value: []byte{2, 0, 5, 0}}, mpEntries(false, []uint32{0, 0})))
	mpCase("quant-huge", false, mpIndex(false, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(false, 0xFFFFFFF0)}, mpEntries(false, []uint32{0, 0})))
	mpCase("no-entries-tag", false, tiffBlock(false, []tiffEntry{{tag: 0xB001, typ: 4, count: 1, value: u32(false, 2)}}))
	mpCase("bad-header", false, []byte("XX\x00\x2a\x00\x00\x00\x08"))
	mpCase("ultra-hdr", false, mpIndex(false, tiffEntry{tag: 0xB001, typ: 4, count: 1, value: u32(false, 2)}, mpEntries(false, []uint32{0, 0})),
		segment16(0xE1, []byte(`<x hdrgm:Version="1.0"/>`)))

	// APP13 and ICC fragments.
	add("go-app-segments", "photoshop-index-error", false, func() []byte {
		s := base()
		s.extra = [][]byte{segment16(0xED, append([]byte("Photoshop 3.0\x00"), []byte("8BIM\x04\x04")...))}
		return buildJPEG(s)
	})
	add("go-app-segments", "photoshop-resolution-short", false, func() []byte {
		s := base()
		payload := append([]byte("Photoshop 3.0\x00"), []byte("8BIM\x03\xED\x00\x00\x00\x00\x00\x04\x01\x02\x03\x04")...)
		s.extra = [][]byte{segment16(0xED, payload)}
		return buildJPEG(s)
	})
	add("go-app-segments", "icc-short-fragment", false, func() []byte {
		s := base()
		s.extra = [][]byte{segment16(0xE2, []byte("ICC_PROFILE\x00\x01"))}
		return buildJPEG(s)
	})
	add("go-app-segments", "icc-two-fragments", false, func() []byte {
		s := base()
		s.extra = [][]byte{
			segment16(0xE2, append([]byte("ICC_PROFILE\x00\x02\x02"), []byte("second")...)),
			segment16(0xE2, append([]byte("ICC_PROFILE\x00\x01\x02"), []byte("first")...)),
		}
		return buildJPEG(s)
	})
	add("go-app-segments", "adobe-short", false, func() []byte {
		s := base()
		s.extra = [][]byte{segment16(0xEE, []byte("Adobe"))}
		return buildJPEG(s)
	})
	add("go-app-segments", "jfif-short", false, func() []byte {
		s := base()
		s.jfif = false
		s.extra = [][]byte{segment16(0xE0, []byte("JFIF\x00"))}
		return buildJPEG(s)
	})

	// Dequantized coefficients beyond 16 bits: libjpeg's C IDCT and its
	// SIMD kernels differ there.
	coef := func(name string, golden bool, f func(*wSpec)) {
		add("go-coefficients", name, golden, func() []byte {
			s := yccSpec(24, 16, 29, "smooth", [][2]int{{1, 1}, {1, 1}, {1, 1}})
			f(&s)
			return buildJPEG(s)
		})
	}
	coef("quant16-small", false, func(s *wSpec) { s.quant16 = true; s.quality = 90 })
	coef("quant16-large-dc", false, func(s *wSpec) {
		s.quant16 = true
		s.quantSet = map[int]map[int]uint16{0: {0: 30000}}
	})
	coef("quant16-over-int16", false, func(s *wSpec) {
		s.quant16 = true
		s.quantSet = map[int]map[int]uint16{0: {0: 40000, 1: 50000}}
	})
	coef("dc-boost", false, func(s *wSpec) {
		s.quant16 = true
		s.quantSet = map[int]map[int]uint16{0: {0: 200}}
		s.dcBoost = 700
	})
	coef("quality-1", false, func(s *wSpec) { s.quality = 1; s.quant16 = true })
	coef("quant-zero", false, func(s *wSpec) {
		s.quantSet = map[int]map[int]uint16{0: {0: 0, 5: 0}}
	})

	// Seeded garbage entropy data under multi-scan headers: it drives every
	// branch of the arithmetic and progressive Huffman decoders, including
	// overflow warnings and restart resynchronisation.
	threeComps := []wComp{{id: 1, h: 2, v: 2, tq: 0}, {id: 2, h: 1, v: 1, tq: 1}, {id: 3, h: 1, v: 1, tq: 1}}
	grayComp := []wComp{{id: 1, h: 1, v: 1, tq: 0}}
	cmykComps := []wComp{{id: 1, h: 1, v: 1, tq: 0}, {id: 2, h: 1, v: 1, tq: 1}, {id: 3, h: 1, v: 1, tq: 1}, {id: 4, h: 1, v: 1, tq: 0}}
	sequential := func(n int) []scanSpec {
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		return []scanSpec{{comps: all, ss: 0, se: 63}}
	}
	garbage := func(class, name string, golden bool, f garbageFile) {
		add(class, name, golden, func() []byte { return f.build() })
	}
	for seed := uint64(1); seed <= 3; seed++ {
		garbage("go-arith", fmt.Sprintf("seq-420-%d", seed), seed == 1, garbageFile{sof: 0xC9, comps: threeComps, w: 64, h: 48, scans: sequential(3), seed: seed, perScan: 400})
		garbage("go-arith", fmt.Sprintf("seq-gray-%d", seed), false, garbageFile{sof: 0xC9, comps: grayComp, w: 40, h: 33, scans: sequential(1), seed: seed + 10, perScan: 300})
		garbage("go-arith", fmt.Sprintf("seq-cmyk-%d", seed), false, garbageFile{sof: 0xC9, comps: cmykComps, w: 21, h: 19, scans: sequential(4), seed: seed + 20, perScan: 300, adobe: 0})
		garbage("go-arith", fmt.Sprintf("seq-dac-%d", seed), false, garbageFile{sof: 0xC9, comps: threeComps, w: 48, h: 40, scans: sequential(3), seed: seed + 30, perScan: 400,
			extra: [][]byte{segment16(0xCC, []byte{0x00, 0x52, 0x01, 0x10, 0x10, 0x05, 0x11, 0x3F})}})
		garbage("go-arith", fmt.Sprintf("seq-restart-%d", seed), false, garbageFile{sof: 0xC9, comps: threeComps, w: 64, h: 64, scans: sequential(3), seed: seed + 40, perScan: 600, restart: 2, rstEvery: 45})
		garbage("go-arith", fmt.Sprintf("prog-420-%d", seed), seed == 2, garbageFile{sof: 0xCA, comps: threeComps, w: 64, h: 48, scans: simpleProgression(3), seed: seed + 50, perScan: 150})
		garbage("go-arith", fmt.Sprintf("prog-gray-%d", seed), false, garbageFile{sof: 0xCA, comps: grayComp, w: 37, h: 29, scans: simpleProgression(1), seed: seed + 60, perScan: 120})
		garbage("go-arith", fmt.Sprintf("prog-restart-%d", seed), false, garbageFile{sof: 0xCA, comps: threeComps, w: 48, h: 48, scans: simpleProgression(3), seed: seed + 70, perScan: 200, restart: 3, rstEvery: 30})
		garbage("go-progressive-garbage", fmt.Sprintf("huff-420-%d", seed), seed == 3, garbageFile{sof: 0xC2, comps: threeComps, w: 64, h: 48, scans: simpleProgression(3), seed: seed + 80, perScan: 200, dht: true})
		garbage("go-progressive-garbage", fmt.Sprintf("huff-gray-%d", seed), false, garbageFile{sof: 0xC2, comps: grayComp, w: 33, h: 41, scans: simpleProgression(1), seed: seed + 90, perScan: 150, dht: true})
		garbage("go-progressive-garbage", fmt.Sprintf("huff-restart-%d", seed), false, garbageFile{sof: 0xC2, comps: threeComps, w: 48, h: 40, scans: simpleProgression(3), seed: seed + 100, perScan: 250, dht: true, restart: 2, rstEvery: 25})
		garbage("go-progressive-garbage", fmt.Sprintf("huff-sequential-scans-%d", seed), false, garbageFile{sof: 0xC0, comps: threeComps, w: 40, h: 40,
			scans: []scanSpec{{comps: []int{0}, se: 63}, {comps: []int{1}, se: 63}, {comps: []int{2}, se: 63}}, seed: seed + 110, perScan: 250, dht: true})
	}
	garbage("go-arith", "seq-past-first-block", false, garbageFile{sof: 0xC9, comps: grayComp, w: 800, h: 800, scans: sequential(1), seed: 7, perScan: 90000})
	garbage("go-arith", "seq-small-in-first-block", false, garbageFile{sof: 0xC9, comps: grayComp, w: 800, h: 800, scans: sequential(1), seed: 8, perScan: 20000})
	garbage("go-lossless", "gray", false, garbageFile{sof: 0xC3, comps: grayComp, w: 30, h: 20, scans: []scanSpec{{comps: []int{0}, ss: 1}}, seed: 9, perScan: 300, dht: true})
	garbage("go-lossless", "rgb", false, garbageFile{sof: 0xC3, comps: []wComp{{id: 'R', h: 1, v: 1}, {id: 'G', h: 1, v: 1}, {id: 'B', h: 1, v: 1}}, w: 30, h: 20,
		scans: []scanSpec{{comps: []int{0, 1, 2}, ss: 1}}, seed: 10, perScan: 300, dht: true, noJFIF: true})
	garbage("go-lossless", "ycbcr-jfif", false, garbageFile{sof: 0xC3, comps: []wComp{{id: 1, h: 1, v: 1}, {id: 2, h: 1, v: 1}, {id: 3, h: 1, v: 1}}, w: 30, h: 20,
		scans: []scanSpec{{comps: []int{0, 1, 2}, ss: 1}}, seed: 11, perScan: 300, dht: true})

	add("go-large", "phone-4032x3024", false, func() []byte {
		return buildJPEG(yccSpec(4032, 3024, 31, "smooth", [][2]int{{2, 2}, {1, 1}, {1, 1}}))
	})
	return cases
}
