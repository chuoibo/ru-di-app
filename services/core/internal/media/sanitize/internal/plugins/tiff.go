package plugins

import (
	"math"
	"math/big"
	"regexp"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// TIFF open: ImageFileDirectory_v2 loading, _setitem's value shaping and
// TiffImageFile._setup's mode and size resolution with the exceptions they
// raise. Pixel decoding (libtiff for compressed data) is not ported.

type tiffOpenEntry struct {
	prefix       string
	photo        int64
	sampleFormat []int64
	fill         int64
	bps          []int64
	extra        []int64
	mode         string
	rawmode      string
}

type pyKind int

const (
	kInt pyKind = iota
	kFloat
	kRat
	kBytes
	kStr
	kTuple
)

// pv is a Python value a TIFF tag decodes to.
type pv struct {
	kind     pyKind
	i        int64
	f        float64
	num, den int64
	b        []byte
	items    []pv
}

func pInt(i int64) pv    { return pv{kind: kInt, i: i} }
func pTuple(v ...pv) pv  { return pv{kind: kTuple, items: v} }
func (v pv) isNum() bool { return v.kind == kInt || v.kind == kFloat || v.kind == kRat }

// rat is the exact value of a number; nan for a float NaN or a zero
// denominator, inf for an infinite float.
func (v pv) rat() (r *big.Rat, nan bool, inf int) {
	switch v.kind {
	case kInt:
		return new(big.Rat).SetInt64(v.i), false, 0
	case kRat:
		if v.den == 0 {
			return nil, true, 0
		}
		return big.NewRat(v.num, v.den), false, 0
	}
	switch {
	case math.IsNaN(v.f):
		return nil, true, 0
	case math.IsInf(v.f, 1):
		return nil, false, 1
	case math.IsInf(v.f, -1):
		return nil, false, -1
	}
	r = new(big.Rat)
	r.SetFloat64(v.f)
	return r, false, 0
}

// cmpNum orders two numbers; ok is false when either is NaN.
func cmpNum(a, b pv) (int, bool) {
	ra, nanA, infA := a.rat()
	rb, nanB, infB := b.rat()
	if nanA || nanB {
		return 0, false
	}
	if infA != 0 || infB != 0 {
		switch {
		case infA == infB:
			return 0, true
		case infA > infB:
			return 1, true
		}
		return -1, true
	}
	return ra.Cmp(rb), true
}

func pyEq(a, b pv) bool {
	switch {
	case a.isNum() && b.isNum():
		c, ok := cmpNum(a, b)
		return ok && c == 0
	case a.kind != b.kind:
		return false
	case a.kind == kBytes || a.kind == kStr:
		return string(a.b) == string(b.b)
	case a.kind == kTuple:
		if len(a.items) != len(b.items) {
			return false
		}
		for i := range a.items {
			if !pyEq(a.items[i], b.items[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func eqInt(v pv, k int64) bool { return pyEq(v, pInt(k)) }

func eqInts(v pv, want []int64) bool {
	if v.kind != kTuple || len(v.items) != len(want) {
		return false
	}
	for i, item := range v.items {
		if !eqInt(item, want[i]) {
			return false
		}
	}
	return true
}

func typeErr() error { return raise("TypeError", "unsupported operand") }

func pyLen(v pv) (int64, error) {
	switch v.kind {
	case kTuple:
		return int64(len(v.items)), nil
	case kBytes, kStr:
		return int64(len(v.b)), nil
	}
	return 0, raise("TypeError", "object has no len()")
}

// items iterates a tuple, bytes (ints) or str (one-character strs).
func pyItems(v pv) ([]pv, error) {
	switch v.kind {
	case kTuple:
		return v.items, nil
	case kBytes:
		out := make([]pv, len(v.b))
		for i, c := range v.b {
			out[i] = pInt(int64(c))
		}
		return out, nil
	case kStr:
		out := make([]pv, len(v.b))
		for i := range v.b {
			out[i] = pv{kind: kStr, b: v.b[i : i+1]}
		}
		return out, nil
	}
	return nil, raise("TypeError", "object is not iterable")
}

func pyTruthy(v pv) bool {
	switch v.kind {
	case kInt:
		return v.i != 0
	case kFloat:
		return v.f != 0
	case kRat:
		return v.den == 0 || v.num != 0
	case kTuple:
		return len(v.items) > 0
	}
	return len(v.b) > 0
}

func pyGreater(a, b pv) (bool, error) {
	if a.isNum() && b.isNum() {
		c, ok := cmpNum(a, b)
		return ok && c > 0, nil
	}
	if a.kind == b.kind && (a.kind == kStr || a.kind == kBytes) {
		return string(a.b) > string(b.b), nil
	}
	return false, raise("TypeError", "'>' not supported")
}

func pyExtreme(v pv, max bool) (pv, error) {
	items, err := pyItems(v)
	if err != nil {
		return pv{}, err
	}
	if len(items) == 0 {
		return pv{}, raise("ValueError", "max() arg is an empty sequence")
	}
	best := items[0]
	for _, item := range items[1:] {
		a, b := item, best
		if !max {
			a, b = best, item
		}
		gt, err := pyGreater(a, b)
		if err != nil {
			return pv{}, err
		}
		if gt {
			best = item
		}
	}
	return best, nil
}

func pySliceSeq(v pv, lo, hi int64) (pv, error) {
	n, err := pyLen(v)
	if err != nil {
		return pv{}, raise("TypeError", "object is not subscriptable")
	}
	s, e := pySlice(n, lo, hi)
	if v.kind == kTuple {
		return pv{kind: kTuple, items: v.items[s:e]}, nil
	}
	return pv{kind: v.kind, b: v.b[s:e]}, nil
}

// ifd is an ImageFileDirectory_v2 after load().
type ifd struct {
	bigEndian, bigTIFF bool
	next               uint64
	tagdata            map[int64][]byte
	tagtype            map[int64]int64
	cache              map[int64]pv
}

var tiffUnitSize = map[int64]int64{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4, 16: 8}

func (d *ifd) uint(b []byte) uint64 {
	var v uint64
	for i := range b {
		if d.bigEndian {
			v = v<<8 | uint64(b[i])
		} else {
			v |= uint64(b[i]) << (8 * i)
		}
	}
	return v
}

// load is ImageFileDirectory_v2.load: an OSError inside (a short read)
// is only warned about, leaving the tags read so far and the old next.
func (d *ifd) load(f *file) error {
	d.tagdata, d.tagtype, d.cache = map[int64][]byte{}, map[int64]int64{}, map[int64]pv{}
	countLen, entryLen, inline := int64(2), int64(12), int64(4)
	if d.bigTIFF {
		countLen, entryLen, inline = 8, 20, 8
	}
	b := f.read(countLen)
	if int64(len(b)) != countLen {
		return nil
	}
	count := d.uint(b)
	for i := uint64(0); i < count; i++ {
		e := f.read(entryLen)
		if int64(len(e)) != entryLen {
			return nil
		}
		tag := int64(d.uint(e[0:2]))
		typ := int64(d.uint(e[2:4]))
		var n uint64
		var data []byte
		if d.bigTIFF {
			n, data = d.uint(e[4:12]), e[12:20]
		} else {
			n, data = d.uint(e[4:8]), e[8:12]
		}
		unit, ok := tiffUnitSize[typ]
		if !ok {
			continue
		}
		size := int64(saturate)
		if n < uint64(saturate)/uint64(unit) {
			size = int64(n) * unit
		}
		if size > inline {
			here := f.tell()
			offset := d.uint(data)
			if offset >= 1<<63 {
				return raise("OverflowError", "Python int too large to convert to C ssize_t")
			}
			_ = f.seek(int64(offset), 0)
			if f.size()-f.tell() < size {
				return nil
			}
			data = f.read(size)
			_ = f.seek(here, 0)
		} else {
			data = data[:size]
		}
		if len(data) == 0 {
			continue
		}
		d.tagdata[tag] = data
		d.tagtype[tag] = typ
	}
	b = f.read(inline)
	if int64(len(b)) != inline {
		return nil
	}
	d.next = d.uint(b)
	return nil
}

func (d *ifd) contains(tag int64) bool {
	_, ok := d.tagdata[tag]
	return ok
}

// value is ImageFileDirectory_v2.__getitem__ (the handler, then _setitem).
func (d *ifd) value(tag int64) (pv, bool) {
	if v, ok := d.cache[tag]; ok {
		return v, true
	}
	data, ok := d.tagdata[tag]
	if !ok {
		return pv{}, false
	}
	typ := d.tagtype[tag]
	var values []pv
	scalar := false
	switch typ {
	case 1, 7:
		values, scalar = []pv{{kind: kBytes, b: data}}, true
	case 2:
		s := data
		if len(s) > 0 && s[len(s)-1] == 0 {
			s = s[:len(s)-1]
		}
		values, scalar = []pv{{kind: kStr, b: s}}, true
	case 5, 10:
		for i := 0; i+8 <= len(data); i += 8 {
			num, den := int64(d.uint(data[i:i+4])), int64(d.uint(data[i+4:i+8]))
			if typ == 10 {
				num, den = int64(int32(num)), int64(int32(den))
			}
			values = append(values, pv{kind: kRat, num: num, den: den})
		}
	default:
		unit := int(tiffUnitSize[typ])
		for i := 0; i+unit <= len(data); i += unit {
			u := d.uint(data[i : i+unit])
			var item pv
			switch typ {
			case 3, 4, 13:
				item = pInt(int64(u))
			case 16:
				item = pInt(int64(min(u, uint64(saturate))))
			case 6:
				item = pInt(int64(int8(u)))
			case 8:
				item = pInt(int64(int16(u)))
			case 9:
				item = pInt(int64(int32(u)))
			case 11:
				item = pv{kind: kFloat, f: float64(math.Float32frombits(uint32(u)))}
			case 12:
				item = pv{kind: kFloat, f: math.Float64frombits(u)}
			}
			values = append(values, item)
		}
	}
	if enum, ok := tiffTagEnum[tag]; ok {
		for i, v := range values {
			if v.kind == kStr {
				if n, ok := enum[string(v.b)]; ok {
					values[i] = pInt(n)
				}
			}
		}
	}
	length, known := tiffTagLength[tag]
	var out pv
	if (known && length == 1) || typ == 1 || (!known && len(values) == 1) || scalar && typ != 7 && typ != 2 {
		out = values[0]
	} else if scalar {
		if known && length != 1 {
			out = pTuple(values...)
		} else {
			out = values[0]
		}
	} else {
		out = pTuple(values...)
	}
	d.cache[tag] = out
	return out, true
}

func (d *ifd) get(tag int64, fallback pv) pv {
	if v, ok := d.value(tag); ok {
		return v
	}
	return fallback
}

var tiffCompression = map[int64]string{
	1: "raw", 2: "tiff_ccitt", 3: "group3", 4: "group4", 5: "tiff_lzw", 6: "tiff_jpeg", 7: "jpeg",
	8: "tiff_adobe_deflate", 32771: "tiff_raw_16", 32773: "packbits", 32809: "tiff_thunderscan",
	32946: "tiff_deflate", 34676: "tiff_sgilog", 34677: "tiff_sgilog24", 34925: "lzma",
	50000: "zstd", 50001: "webp",
}

var tiffPrefixes = []string{"MM\x00*", "II*\x00", "MM*\x00", "II\x00*", "MM\x00+", "II+\x00"}

func acceptTIFF(p []byte) bool { return hasPrefix(p, tiffPrefixes...) }

func openTIFF(f *file) (pil.Opened, error) {
	ifh := append([]byte(nil), f.read(8)...)
	c2, err := at(ifh, 2)
	if err != nil {
		return nil, err
	}
	if c2 == 43 {
		ifh = append(ifh, f.read(8)...)
	}
	if !acceptTIFF(ifh) {
		return nil, raise("SyntaxError", "not a TIFF file")
	}
	d := &ifd{bigEndian: string(ifh[:2]) == "MM", bigTIFF: ifh[2] == 43}
	if d.bigTIFF {
		if len(ifh) != 16 {
			return nil, structErr()
		}
		d.next = d.uint(ifh[8:16])
	} else {
		if len(ifh) != 8 {
			return nil, structErr()
		}
		d.next = d.uint(ifh[4:8])
	}
	first := d.next
	if first == 0 {
		return nil, raise("EOFError", "no more images in TIFF file")
	}
	if first >= 1<<63 {
		return nil, raise("ValueError", "Unable to seek to frame")
	}
	_ = f.seek(int64(first), 0)
	if err := d.load(f); err != nil {
		return nil, err
	}
	animated := d.next != first && d.next != 0
	_ = f.seek(int64(first), 0)
	if err := d.load(f); err != nil {
		return nil, err
	}
	lay, err := tiffSetup(d, string(ifh[:2]))
	if err != nil {
		return nil, err
	}
	return finish(lay.mode, lay.w, lay.h, func() (*pil.Image, error) {
		return loadTIFF(f, d, lay, animated)
	})
}

// tiffLayout is what _setup leaves for load.
type tiffLayout struct {
	mode, rawmode string
	w, h          int64
	tileW, tileH  int64
	compression   string
	tiles         []tile
	tilesExact    bool
	palette       *pyPalette
}

// tiffGroupKeys are TiffTags.TAGS_V2_GROUPS: load_end reads those sub-IFDs.
var tiffGroupKeys = []int64{34665, 34853, 40965}

func loadTIFF(f *file, d *ifd, lay *tiffLayout, animated bool) (*pil.Image, error) {
	if lay.compression != "raw" {
		return nil, pil.Unsupported("TIFF", "libtiff decoder for %s", lay.compression)
	}
	if !lay.tilesExact {
		return nil, pil.Unsupported("TIFF", "non-integer strip geometry")
	}
	if !animated {
		for _, key := range tiffGroupKeys {
			if d.contains(key) {
				return nil, pil.Unsupported("TIFF", "load_end reads sub-IFD %d", key)
			}
		}
	}
	loadEnd := func(c *canvas) error {
		method, err := tiffOrientation(d)
		if err != nil || method == 0 {
			return err
		}
		transposeCanvas(c, method)
		if xv, ok := d.value(700); ok {
			if xv.kind == kTuple && len(xv.items) == 1 {
				xv = xv.items[0]
			}
			switch xv.kind {
			case kBytes, kStr:
			case kTuple:
				if len(xv.items) > 0 {
					return raise("TypeError", "expected string or bytes-like object")
				}
			default:
				return raise("TypeError", "expected string or bytes-like object")
			}
		}
		return nil
	}
	return loadRaster(f, "TIFF", lay.mode, lay.tileW, lay.tileH, lay.palette, lay.tiles, loadEnd)
}

var xmpOrientation = regexp.MustCompile(`tiff:Orientation(="|>)([0-9])`)

// tiffOrientation is the orientation ImageOps.exif_transpose reads during
// TiffImageFile.load_end: tag 274 of IFD0, else the XMP packet. It returns
// the orientation that selects a transposition, 0 for none.
func tiffOrientation(d *ifd) (int64, error) {
	var orientation pv
	if v, ok := d.value(274); ok {
		if v.kind == kTuple && len(v.items) == 1 {
			v = v.items[0]
		}
		orientation = v
	} else if xv, ok := d.value(700); ok {
		if xv.kind == kTuple && len(xv.items) == 1 {
			xv = xv.items[0]
		}
		if !pyTruthy(xv) {
			return 0, nil
		}
		if xv.kind != kBytes {
			return 0, raise("TypeError", "cannot use a bytes pattern on a non-bytes object")
		}
		m := xmpOrientation.FindSubmatch(xv.b)
		if m == nil {
			return 0, nil
		}
		orientation = pInt(int64(m[2][0] - '0'))
	} else {
		return 0, nil
	}
	for k := int64(2); k <= 8; k++ {
		if eqInt(orientation, k) {
			return k, nil
		}
	}
	return 0, nil
}

// transposeCanvas applies the Image.transpose that exif_transpose picks for
// an orientation (Geometry.c), keeping the palette.
func transposeCanvas(c *canvas, orientation int64) {
	w, h, ps := c.w, c.h, c.ps
	ow, oh := w, h
	if orientation >= 5 {
		ow, oh = h, w
	}
	out := make([]byte, len(c.pix))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch orientation {
			case 2: // FLIP_LEFT_RIGHT
				nx, ny = w-1-x, y
			case 3: // ROTATE_180
				nx, ny = w-1-x, h-1-y
			case 4: // FLIP_TOP_BOTTOM
				nx, ny = x, h-1-y
			case 5: // TRANSPOSE
				nx, ny = y, x
			case 6: // ROTATE_270
				nx, ny = h-1-y, x
			case 7: // TRANSVERSE
				nx, ny = h-1-y, w-1-x
			case 8: // ROTATE_90
				nx, ny = y, w-1-x
			}
			copy(out[(ny*ow+nx)*ps:(ny*ow+nx+1)*ps], c.pix[(y*w+x)*ps:(y*w+x+1)*ps])
		}
	}
	c.pix, c.w, c.h = out, ow, oh
}

func tiffSetup(d *ifd, prefix string) (*tiffLayout, error) {
	if d.contains(48129) {
		return nil, raise("OSError", "Windows Media Photo files not yet supported")
	}
	compValue := d.get(259, pInt(1))
	compression := ""
	for k, name := range tiffCompression {
		if eqInt(compValue, k) {
			compression = name
		}
	}
	if compression == "" {
		return nil, raise("KeyError", "compression")
	}
	planar := d.get(284, pInt(1))
	photo := d.get(262, pInt(0))
	if compression == "tiff_jpeg" {
		photo = pInt(6)
	}
	fill := d.get(266, pInt(1))
	d.get(530, pv{})
	xsize, okX := d.value(256)
	ysize, okY := d.value(257)
	if !okX || !okY {
		return nil, raise("TypeError", "Missing dimensions")
	}
	if xsize.kind != kInt || ysize.kind != kInt {
		return nil, raise("ValueError", "Invalid dimensions")
	}
	lay := &tiffLayout{compression: compression, tileW: xsize.i, tileH: ysize.i, w: xsize.i, h: ysize.i, tilesExact: true}
	if o, ok := d.value(274); ok && (eqInt(o, 5) || eqInt(o, 6) || eqInt(o, 7) || eqInt(o, 8)) {
		lay.w, lay.h = ysize.i, xsize.i
	}
	sampleFormat := d.get(339, pTuple(pInt(1)))
	n, err := pyLen(sampleFormat)
	if err != nil {
		return nil, err
	}
	if n > 1 {
		hi, err := pyExtreme(sampleFormat, true)
		if err != nil {
			return nil, err
		}
		lo, err := pyExtreme(sampleFormat, false)
		if err != nil {
			return nil, err
		}
		if pyEq(hi, lo) {
			items, _ := pyItems(sampleFormat)
			sampleFormat = pTuple(items[0])
		}
	}
	bps := d.get(258, pTuple(pInt(1)))
	extra := d.get(338, pTuple())
	sppDefault := int64(1)
	if compression == "tiff_jpeg" && (eqInt(photo, 2) || eqInt(photo, 6)) {
		sppDefault = 3
	}
	spp := d.get(277, pInt(sppDefault))
	bpsCount := int64(1)
	switch {
	case eqInt(photo, 2) || eqInt(photo, 6) || eqInt(photo, 8):
		bpsCount = 3
	case eqInt(photo, 5):
		bpsCount = 4
	}
	if eqInt(planar, 2) && pyTruthy(extra) {
		hi, err := pyExtreme(extra, true)
		if err != nil {
			return nil, err
		}
		if eqInt(hi, 0) {
			el, _ := pyLen(extra)
			if bps, err = pySliceSeq(bps, 0, -el); err != nil {
				return nil, err
			}
			if !spp.isNum() {
				return nil, typeErr()
			}
			if spp.kind == kInt {
				spp = pInt(spp.i - el)
			}
			extra = pTuple()
		}
	}
	el, err := pyLen(extra)
	if err != nil {
		return nil, err
	}
	bpsCount += el
	actual, err := pyLen(bps)
	if err != nil {
		return nil, err
	}
	if !spp.isNum() {
		return nil, typeErr()
	}
	if c, ok := cmpNum(spp, pInt(6)); ok && c > 0 {
		return nil, raise("SyntaxError", "Invalid value for samples per pixel")
	}
	if c, ok := cmpNum(spp, pInt(actual)); ok && c < 0 {
		if spp.kind != kInt {
			return nil, raise("TypeError", "slice indices must be integers")
		}
		if bps, err = pySliceSeq(bps, 0, spp.i); err != nil {
			return nil, err
		}
	} else if ok && c > 0 && actual == 1 {
		if spp.kind != kInt {
			return nil, raise("TypeError", "can't multiply sequence by non-int")
		}
		items, _ := pyItems(bps)
		if bps.kind == kTuple {
			var rep []pv
			for i := int64(0); i < spp.i; i++ {
				rep = append(rep, items...)
			}
			bps = pTuple(rep...)
		} else {
			var rep []byte
			for i := int64(0); i < spp.i; i++ {
				rep = append(rep, bps.b...)
			}
			bps = pv{kind: bps.kind, b: rep}
		}
	}
	if bl, _ := pyLen(bps); !eqInt(spp, bl) {
		return nil, raise("SyntaxError", "unknown data organization")
	}
	lookup := func(fillValue pv) (tiffOpenEntry, bool) {
		for _, e := range tiffOpenInfo {
			if e.prefix == prefix && eqInt(photo, e.photo) && eqInts(sampleFormat, e.sampleFormat) &&
				eqInt(fillValue, e.fill) && eqInts(bps, e.bps) && eqInts(extra, e.extra) {
				return e, true
			}
		}
		return tiffOpenEntry{}, false
	}
	entry, ok := lookup(fill)
	if !ok {
		return nil, raise("KeyError", "unknown pixel mode")
	}
	lay.mode, lay.rawmode = entry.mode, entry.rawmode
	xres := d.get(282, pInt(1))
	yres := d.get(283, pInt(1))
	if pyTruthy(xres) && pyTruthy(yres) {
		if unit, ok := d.value(296); ok && eqInt(unit, 3) && (!xres.isNum() || !yres.isNum()) {
			return nil, typeErr()
		}
	}
	if compression != "raw" {
		if eqInt(fill, 2) {
			if _, ok := lookup(pInt(1)); !ok {
				return nil, raise("KeyError", "unknown pixel mode")
			}
		}
	} else if d.contains(273) || d.contains(324) {
		var offsets, tw, th pv
		if d.contains(273) {
			offsets, _ = d.value(273)
			tw, th = xsize, d.get(278, ysize)
		} else {
			offsets, _ = d.value(324)
			var okW, okH bool
			tw, okW = d.value(322)
			th, okH = d.value(323)
			if !okW || !okH || tw.kind != kInt || th.kind != kInt {
				return nil, raise("ValueError", "Invalid tile dimensions")
			}
		}
		if pyEq(tw, xsize) && pyEq(th, ysize) && !eqInt(planar, 2) {
			if offsets, err = pySliceSeq(offsets, -1, saturate); err != nil {
				return nil, err
			}
		}
		items, err := pyItems(offsets)
		if err != nil {
			return nil, err
		}
		if !th.isNum() && len(items) > 0 {
			return nil, typeErr()
		}
		if th.kind != kInt {
			lay.tilesExact = false
		}
		bitsItems, _ := pyItems(bps)
		x, y, layer := int64(0), int64(0), 0
		for _, it := range items {
			stride := 0.0
			if x+tw.i > xsize.i {
				sum := 0.0
				for _, b := range bitsItems {
					if !b.isNum() {
						return nil, typeErr()
					}
					switch b.kind {
					case kInt:
						sum += float64(b.i)
					case kFloat:
						sum += b.f
					default:
						sum += float64(b.num) / float64(b.den)
					}
				}
				stride = float64(tw.i) * sum / 8
			}
			rawmode := entry.rawmode
			if eqInt(planar, 2) {
				if layer >= len(entry.rawmode) {
					return nil, indexErr()
				}
				rawmode = entry.rawmode[layer : layer+1]
				stride /= float64(bpsCount)
			}
			if it.kind != kInt {
				lay.tilesExact = false
			}
			lay.tiles = append(lay.tiles, tile{
				decoder: "raw", x0: x, y0: y, x1: min(x+tw.i, xsize.i), y1: min(y+th.i, ysize.i),
				offset: it.i, rawmode: rawmode, stride: int64(stride), ystep: 1,
			})
			x += tw.i
			if x >= xsize.i {
				x, y = 0, y+th.i
				if y >= ysize.i {
					y, layer = 0, layer+1
				}
			}
		}
	} else {
		return nil, raise("SyntaxError", "unknown data organization")
	}
	if entry.mode == "P" || entry.mode == "PA" {
		cm, ok := d.value(320)
		if !ok {
			return nil, raise("KeyError", "colormap")
		}
		items, err := pyItems(cm)
		if err != nil {
			return nil, err
		}
		data := make([]byte, 0, len(items))
		for _, it := range items {
			switch {
			case it.kind == kInt:
				data = append(data, byte(floorDiv(it.i, 256)))
			case it.kind == kRat && it.den != 0:
				data = append(data, byte(floorDiv(floorDiv(it.num, it.den), 256)))
			default:
				return nil, typeErr()
			}
		}
		lay.palette = &pyPalette{mode: "RGB", rawmode: "RGB;L", data: data}
	}
	return lay, nil
}
