package jpegdec

import (
	"encoding/binary"
	"math/bits"
	"math/rand/v2"
)

// A small baseline JPEG writer for test inputs: arbitrary sampling
// factors, component counts and IDs, restart intervals, 16-bit quantization
// tables and extra marker segments. It only has to produce valid (or
// deliberately broken) streams; pixel fidelity is not its job.

type wComp struct{ id, h, v, tq int }

type wSpec struct {
	width, height int
	comps         []wComp
	quality       int
	quant16       bool
	// quantSet overrides quantization entries: table -> natural index -> value.
	quantSet map[int]map[int]uint16
	restart  int
	jfif     bool
	adobe    int
	// extra holds whole marker segments written after SOI/JFIF/Adobe.
	extra  [][]byte
	planes [][]byte
	// dcBoost is added to every quantized DC coefficient of component 0.
	dcBoost int32
}

var stdLuminanceQuant = [64]int{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

var stdChrominanceQuant = [64]int{
	17, 18, 24, 47, 99, 99, 99, 99,
	18, 21, 26, 66, 99, 99, 99, 99,
	24, 26, 56, 99, 99, 99, 99, 99,
	47, 66, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
}

func (s *wSpec) quantTable(tq int) [64]uint16 {
	quality := s.quality
	if quality <= 0 {
		quality = 75
	}
	scale := 200 - quality*2
	if quality < 50 {
		scale = 5000 / quality
	}
	basic := stdLuminanceQuant
	if tq != 0 {
		basic = stdChrominanceQuant
	}
	var t [64]uint16
	for i := range t {
		v := (basic[i]*scale + 50) / 100
		v = max(v, 1)
		if !s.quant16 {
			v = min(v, 255)
		}
		t[i] = uint16(min(v, 65535))
	}
	for i, v := range s.quantSet[tq] {
		t[i] = v
	}
	return t
}

func segment16(marker byte, payload []byte) []byte {
	out := []byte{0xFF, marker, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	return append(out, payload...)
}

type encTable struct {
	code [256]uint32
	size [256]uint8
}

func makeEncTable(bitsTbl [17]uint8, val []uint8) *encTable {
	t := &encTable{}
	p, code := 0, uint32(0)
	for l := 1; l <= 16; l++ {
		for i := 0; i < int(bitsTbl[l]); i++ {
			t.code[val[p]] = code
			t.size[val[p]] = uint8(l)
			p++
			code++
		}
		code <<= 1
	}
	return t
}

type bitWriter struct {
	out *[]byte
	acc uint64
	n   uint
}

func (w *bitWriter) put(code uint32, size uint8) {
	if size == 0 {
		return
	}
	w.acc = w.acc<<size | uint64(code&(1<<size-1))
	w.n += uint(size)
	for w.n >= 8 {
		b := byte(w.acc >> (w.n - 8))
		*w.out = append(*w.out, b)
		if b == 0xFF {
			*w.out = append(*w.out, 0)
		}
		w.n -= 8
	}
}

func (w *bitWriter) flush() {
	pad := (8 - w.n%8) % 8
	w.put(1<<pad-1, uint8(pad))
}

func descale64(x int64, n uint) int64 { return (x + int64(1)<<(n-1)) >> n }

// fdct is jfdctint's islow forward DCT with full-width integers.
func fdct(data *[64]int64) {
	for r := 0; r < 8; r++ {
		p := data[r*8 : r*8+8]
		tmp0, tmp7 := p[0]+p[7], p[0]-p[7]
		tmp1, tmp6 := p[1]+p[6], p[1]-p[6]
		tmp2, tmp5 := p[2]+p[5], p[2]-p[5]
		tmp3, tmp4 := p[3]+p[4], p[3]-p[4]
		tmp10, tmp13 := tmp0+tmp3, tmp0-tmp3
		tmp11, tmp12 := tmp1+tmp2, tmp1-tmp2
		p[0] = (tmp10 + tmp11) << 2
		p[4] = (tmp10 - tmp11) << 2
		z1 := (tmp12 + tmp13) * fix0541
		p[2] = descale64(z1+tmp13*fix0765, 11)
		p[6] = descale64(z1-tmp12*fix1847, 11)
		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * fix1175
		tmp4 *= fix0298
		tmp5 *= fix2053
		tmp6 *= fix3072
		tmp7 *= fix1501
		z1 *= -fix0899
		z2 *= -fix2562
		z3 = z3*-fix1961 + z5
		z4 = z4*-fix0390 + z5
		p[7] = descale64(tmp4+z1+z3, 11)
		p[5] = descale64(tmp5+z2+z4, 11)
		p[3] = descale64(tmp6+z2+z3, 11)
		p[1] = descale64(tmp7+z1+z4, 11)
	}
	for c := 0; c < 8; c++ {
		at := func(r int) *int64 { return &data[r*8+c] }
		tmp0, tmp7 := *at(0)+*at(7), *at(0)-*at(7)
		tmp1, tmp6 := *at(1)+*at(6), *at(1)-*at(6)
		tmp2, tmp5 := *at(2)+*at(5), *at(2)-*at(5)
		tmp3, tmp4 := *at(3)+*at(4), *at(3)-*at(4)
		tmp10, tmp13 := tmp0+tmp3, tmp0-tmp3
		tmp11, tmp12 := tmp1+tmp2, tmp1-tmp2
		*at(0) = descale64(tmp10+tmp11, 2)
		*at(4) = descale64(tmp10-tmp11, 2)
		z1 := (tmp12 + tmp13) * fix0541
		*at(2) = descale64(z1+tmp13*fix0765, 15)
		*at(6) = descale64(z1-tmp12*fix1847, 15)
		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * fix1175
		tmp4 *= fix0298
		tmp5 *= fix2053
		tmp6 *= fix3072
		tmp7 *= fix1501
		z1 *= -fix0899
		z2 *= -fix2562
		z3 = z3*-fix1961 + z5
		z4 = z4*-fix0390 + z5
		*at(7) = descale64(tmp4+z1+z3, 15)
		*at(5) = descale64(tmp5+z2+z4, 15)
		*at(3) = descale64(tmp6+z2+z3, 15)
		*at(1) = descale64(tmp7+z1+z4, 15)
	}
}

func clampInt32(v, lo, hi int32) int32 { return max(lo, min(hi, v)) }

// buildJPEG writes the baseline file described by s.
func buildJPEG(s wSpec) []byte {
	out := []byte{0xFF, 0xD8}
	if s.jfif {
		out = append(out, segment16(0xE0, []byte{'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0})...)
	}
	if s.adobe >= 0 {
		out = append(out, segment16(0xEE, []byte{'A', 'd', 'o', 'b', 'e', 0, 100, 0, 0, 0, 0, byte(s.adobe)})...)
	}
	for _, e := range s.extra {
		out = append(out, e...)
	}
	used := map[int]bool{}
	for _, c := range s.comps {
		used[c.tq] = true
	}
	tables := map[int][64]uint16{}
	for tq := 0; tq < 4; tq++ {
		if !used[tq] {
			continue
		}
		t := s.quantTable(tq)
		tables[tq] = t
		payload := []byte{byte(tq)}
		if s.quant16 {
			payload[0] |= 0x10
		}
		for i := 0; i < 64; i++ {
			v := t[naturalOrder[i]]
			if s.quant16 {
				payload = append(payload, byte(v>>8))
			}
			payload = append(payload, byte(v))
		}
		out = append(out, segment16(0xDB, payload)...)
	}
	sofMarker := byte(0xC0)
	if s.quant16 {
		sofMarker = 0xC1
	}
	sof := []byte{8, byte(s.height >> 8), byte(s.height), byte(s.width >> 8), byte(s.width), byte(len(s.comps))}
	for _, c := range s.comps {
		sof = append(sof, byte(c.id), byte(c.h<<4|c.v), byte(c.tq))
	}
	out = append(out, segment16(sofMarker, sof)...)
	dht := func(class byte, bitsTbl [17]uint8, val []uint8) {
		payload := []byte{class}
		payload = append(payload, bitsTbl[1:]...)
		payload = append(payload, val...)
		out = append(out, segment16(0xC4, payload)...)
	}
	dht(0x00, bitsDCLuminance, valDC)
	dht(0x10, bitsACLuminance, valACLuminance)
	dht(0x01, bitsDCChrominance, valDC)
	dht(0x11, bitsACChrominance, valACChrominance)
	if s.restart > 0 {
		out = append(out, segment16(0xDD, []byte{byte(s.restart >> 8), byte(s.restart)})...)
	}
	sos := []byte{byte(len(s.comps))}
	for i, c := range s.comps {
		tbl := byte(0)
		if i > 0 {
			tbl = 0x11
		}
		sos = append(sos, byte(c.id), tbl)
	}
	sos = append(sos, 0, 63, 0)
	out = append(out, segment16(0xDA, sos)...)
	out = s.scan(out, tables)
	return append(out, 0xFF, 0xD9)
}

func (s *wSpec) scan(out []byte, tables map[int][64]uint16) []byte {
	maxH, maxV := 1, 1
	for _, c := range s.comps {
		maxH, maxV = max(maxH, c.h), max(maxV, c.v)
	}
	dcTbl := []*encTable{makeEncTable(bitsDCLuminance, valDC), makeEncTable(bitsDCChrominance, valDC)}
	acTbl := []*encTable{makeEncTable(bitsACLuminance, valACLuminance), makeEncTable(bitsACChrominance, valACChrominance)}
	w := &bitWriter{out: &out}
	preds := make([]int32, len(s.comps))
	sample := func(ci, x, y int) int64 {
		c := s.comps[ci]
		dsw := divRoundUp(s.width*c.h, maxH)
		dsh := divRoundUp(s.height*c.v, maxV)
		x, y = min(x, dsw-1), min(y, dsh-1)
		sx, sy := min(x*maxH/c.h, s.width-1), min(y*maxV/c.v, s.height-1)
		return int64(s.planes[ci][sy*s.width+sx])
	}
	encodeBlock := func(ci, bx, by int) {
		var data [64]int64
		for r := 0; r < 8; r++ {
			for col := 0; col < 8; col++ {
				data[r*8+col] = sample(ci, bx*8+col, by*8+r) - 128
			}
		}
		fdct(&data)
		q := tables[s.comps[ci].tq]
		var coef [64]int32
		for i := range data {
			div := int64(max(q[i], 1)) * 8
			t := data[i]
			if t < 0 {
				t = -((-t + div/2) / div)
			} else {
				t = (t + div/2) / div
			}
			coef[i] = clampInt32(int32(t), -1023, 1023)
		}
		if ci == 0 && s.dcBoost != 0 {
			coef[0] = clampInt32(coef[0]+s.dcBoost, -1023, 1023)
		}
		tbl := 0
		if ci > 0 {
			tbl = 1
		}
		diff := coef[0] - preds[ci]
		preds[ci] = coef[0]
		mag := diff
		if mag < 0 {
			mag = -mag
		}
		nb := uint8(bits.Len32(uint32(mag)))
		w.put(dcTbl[tbl].code[nb], dcTbl[tbl].size[nb])
		v := diff
		if v < 0 {
			v--
		}
		w.put(uint32(v), nb)
		run := 0
		for k := 1; k < 64; k++ {
			val := coef[naturalOrder[k]]
			if val == 0 {
				run++
				continue
			}
			for run > 15 {
				w.put(acTbl[tbl].code[0xF0], acTbl[tbl].size[0xF0])
				run -= 16
			}
			mag := val
			if mag < 0 {
				mag = -mag
			}
			nb := uint8(bits.Len32(uint32(mag)))
			sym := byte(run<<4) | nb
			w.put(acTbl[tbl].code[sym], acTbl[tbl].size[sym])
			vv := val
			if vv < 0 {
				vv--
			}
			w.put(uint32(vv), nb)
			run = 0
		}
		if run > 0 {
			w.put(acTbl[tbl].code[0], acTbl[tbl].size[0])
		}
	}
	mcu := 0
	restartNum := 0
	restartIfDue := func() {
		if s.restart > 0 && mcu > 0 && mcu%s.restart == 0 {
			w.flush()
			out = append(out, 0xFF, byte(0xD0+restartNum&7))
			restartNum++
			for i := range preds {
				preds[i] = 0
			}
		}
		mcu++
	}
	if len(s.comps) == 1 {
		c := s.comps[0]
		wib := divRoundUp(s.width*c.h, maxH*8)
		hib := divRoundUp(s.height*c.v, maxV*8)
		for by := 0; by < hib; by++ {
			for bx := 0; bx < wib; bx++ {
				restartIfDue()
				encodeBlock(0, bx, by)
			}
		}
	} else {
		mcusPerRow := divRoundUp(s.width, maxH*8)
		mcuRows := divRoundUp(s.height, maxV*8)
		for my := 0; my < mcuRows; my++ {
			for mx := 0; mx < mcusPerRow; mx++ {
				restartIfDue()
				for ci, c := range s.comps {
					for by := 0; by < c.v; by++ {
						for bx := 0; bx < c.h; bx++ {
							encodeBlock(ci, mx*c.h+bx, my*c.v+by)
						}
					}
				}
			}
		}
	}
	w.flush()
	return out
}

// testPlane is a seeded sample plane.
func testPlane(w, h int, seed uint64, pattern string) []byte {
	rng := rand.New(rand.NewPCG(seed, 0x9E3779B97F4A7C15))
	p := make([]byte, w*h)
	switch pattern {
	case "noise":
		for i := range p {
			p[i] = byte(rng.Uint32())
		}
	case "flat":
		v := byte(rng.Uint32())
		for i := range p {
			p[i] = v
		}
	default:
		base := rng.IntN(256)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				// A triangle wave of a diagonal ramp, plus noise.
				t := (base + (x*3+y*5)/2) % 510
				if t > 255 {
					t = 510 - t
				}
				p[y*w+x] = byte(max(0, min(255, t+rng.IntN(13)-6)))
			}
		}
	}
	return p
}

// tiffEntry is one IFD entry for tiffBlock.
type tiffEntry struct {
	tag, typ uint16
	count    uint32
	value    []byte
}

// tiffBlock is a TIFF header with IFD0 holding entries (values longer than
// four bytes go after the IFD).
func tiffBlock(bigEndian bool, entries []tiffEntry) []byte {
	var order binary.ByteOrder = binary.LittleEndian
	head := []byte("II\x2a\x00\x08\x00\x00\x00")
	if bigEndian {
		order = binary.BigEndian
		head = []byte("MM\x00\x2a\x00\x00\x00\x08")
	}
	ifdLen := 2 + 12*len(entries) + 4
	out := append([]byte(nil), head...)
	extra := []byte{}
	ifd := make([]byte, ifdLen)
	order.PutUint16(ifd, uint16(len(entries)))
	for i, e := range entries {
		at := 2 + 12*i
		order.PutUint16(ifd[at:], e.tag)
		order.PutUint16(ifd[at+2:], e.typ)
		order.PutUint32(ifd[at+4:], e.count)
		if len(e.value) <= 4 {
			copy(ifd[at+8:], e.value)
		} else {
			order.PutUint32(ifd[at+8:], uint32(8+ifdLen+len(extra)))
			extra = append(extra, e.value...)
		}
	}
	out = append(out, ifd...)
	return append(out, extra...)
}

func u16(bigEndian bool, v uint16) []byte {
	b := make([]byte, 2)
	if bigEndian {
		binary.BigEndian.PutUint16(b, v)
	} else {
		binary.LittleEndian.PutUint16(b, v)
	}
	return b
}

func u32(bigEndian bool, vs ...uint32) []byte {
	var b []byte
	for _, v := range vs {
		c := make([]byte, 4)
		if bigEndian {
			binary.BigEndian.PutUint32(c, v)
		} else {
			binary.LittleEndian.PutUint32(c, v)
		}
		b = append(b, c...)
	}
	return b
}

// mpEntries is the MPEntry blob: attribute, size, offset, two entry numbers.
func mpEntries(bigEndian bool, formats []uint32) []byte {
	var out []byte
	for i, f := range formats {
		out = append(out, u32(bigEndian, f<<24|0x030000, 1000, uint32(i)*1000)...)
		out = append(out, u16(bigEndian, 0)...)
		out = append(out, u16(bigEndian, 0)...)
	}
	return out
}

// scanSpec is one SOS: component indexes and spectral/approximation fields.
type scanSpec struct {
	comps          []int
	ss, se, ah, al int
}

// simpleProgression is jpeg_simple_progression's script.
func simpleProgression(n int) []scanSpec {
	if n == 3 {
		return []scanSpec{
			{comps: []int{0, 1, 2}, ss: 0, se: 0, ah: 0, al: 1},
			{comps: []int{0}, ss: 1, se: 5, ah: 0, al: 2},
			{comps: []int{2}, ss: 1, se: 63, ah: 0, al: 1},
			{comps: []int{1}, ss: 1, se: 63, ah: 0, al: 1},
			{comps: []int{0}, ss: 6, se: 63, ah: 0, al: 2},
			{comps: []int{0}, ss: 1, se: 63, ah: 2, al: 1},
			{comps: []int{0, 1, 2}, ss: 0, se: 0, ah: 1, al: 0},
			{comps: []int{2}, ss: 1, se: 63, ah: 1, al: 0},
			{comps: []int{1}, ss: 1, se: 63, ah: 1, al: 0},
			{comps: []int{0}, ss: 1, se: 63, ah: 1, al: 0},
		}
	}
	return []scanSpec{
		{comps: []int{0}, ss: 0, se: 0, ah: 0, al: 1},
		{comps: []int{0}, ss: 1, se: 5, ah: 0, al: 2},
		{comps: []int{0}, ss: 6, se: 63, ah: 0, al: 2},
		{comps: []int{0}, ss: 1, se: 63, ah: 2, al: 1},
		{comps: []int{0}, ss: 0, se: 0, ah: 1, al: 0},
		{comps: []int{0}, ss: 1, se: 63, ah: 1, al: 0},
	}
}

// garbageFile is a multi-scan header set whose scans carry seeded random
// entropy bytes (0xFF stuffed), optionally with RST markers every rstEvery
// bytes.
type garbageFile struct {
	sof      byte
	comps    []wComp
	w, h     int
	scans    []scanSpec
	seed     uint64
	perScan  int
	restart  int
	rstEvery int
	dht      bool
	noJFIF   bool
	adobe    int
	extra    [][]byte
}

func (g garbageFile) build() []byte {
	s := wSpec{quality: 75, comps: g.comps}
	out := []byte{0xFF, 0xD8}
	if !g.noJFIF && len(g.comps) != 4 {
		out = append(out, segment16(0xE0, []byte{'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0})...)
	}
	if len(g.comps) == 4 {
		out = append(out, segment16(0xEE, []byte{'A', 'd', 'o', 'b', 'e', 0, 100, 0, 0, 0, 0, byte(g.adobe)})...)
	}
	for tq := 0; tq < 2; tq++ {
		t := s.quantTable(tq)
		payload := []byte{byte(tq)}
		for i := 0; i < 64; i++ {
			payload = append(payload, byte(t[naturalOrder[i]]))
		}
		out = append(out, segment16(0xDB, payload)...)
	}
	sof := []byte{8, byte(g.h >> 8), byte(g.h), byte(g.w >> 8), byte(g.w), byte(len(g.comps))}
	for _, c := range g.comps {
		sof = append(sof, byte(c.id), byte(c.h<<4|c.v), byte(c.tq))
	}
	out = append(out, segment16(g.sof, sof)...)
	if g.dht {
		for _, t := range []struct {
			class byte
			bits  [17]uint8
			val   []uint8
		}{{0x00, bitsDCLuminance, valDC}, {0x10, bitsACLuminance, valACLuminance}, {0x01, bitsDCChrominance, valDC}, {0x11, bitsACChrominance, valACChrominance}} {
			payload := append([]byte{t.class}, t.bits[1:]...)
			out = append(out, segment16(0xC4, append(payload, t.val...))...)
		}
	}
	for _, e := range g.extra {
		out = append(out, e...)
	}
	if g.restart > 0 {
		out = append(out, segment16(0xDD, []byte{byte(g.restart >> 8), byte(g.restart)})...)
	}
	rng := rand.New(rand.NewPCG(g.seed, 0xC0FFEE))
	for _, scan := range g.scans {
		sos := []byte{byte(len(scan.comps))}
		for _, ci := range scan.comps {
			tbl := byte(0)
			if ci > 0 {
				tbl = 0x11
			}
			sos = append(sos, byte(g.comps[ci].id), tbl)
		}
		sos = append(sos, byte(scan.ss), byte(scan.se), byte(scan.ah<<4|scan.al))
		out = append(out, segment16(0xDA, sos)...)
		rst := 0
		for i := 0; i < g.perScan; i++ {
			if g.rstEvery > 0 && i > 0 && i%g.rstEvery == 0 {
				out = append(out, 0xFF, byte(0xD0+rst&7))
				rst++
			}
			b := byte(rng.Uint32())
			out = append(out, b)
			if b == 0xFF {
				out = append(out, 0)
			}
		}
	}
	return append(out, 0xFF, 0xD9)
}
