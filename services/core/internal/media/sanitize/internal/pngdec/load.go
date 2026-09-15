package pngdec

import (
	"errors"
	"fmt"

	"mobile/services/core/internal/media/sanitize/internal/exif"
	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Codec error codes from Imaging.h.
const (
	codecBroken  = -2
	codecUnknown = -3
	codecConfig  = -8
)

// unpacker is one Unpack.c shuffler: bits per input pixel and a function
// writing n pixels of the pil model from src into dst.
type unpacker struct {
	bits int
	fn   func(dst, src []byte, n int)
}

func bitSample(src []byte, i, bits int) byte {
	perByte := 8 / bits
	shift := uint(8 - bits - (i%perByte)*bits)
	return src[i/perByte] >> shift & byte(1<<bits-1)
}

var unpackers = map[[2]string]unpacker{
	{"1", "1"}: {1, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[i] = 0
			if bitSample(src, i, 1) != 0 {
				dst[i] = 255
			}
		}
	}},
	{"L", "L;2"}: {2, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[i] = bitSample(src, i, 2) * 0x55
		}
	}},
	{"L", "L;4"}: {4, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[i] = bitSample(src, i, 4) * 0x11
		}
	}},
	{"L", "L"}: {8, func(dst, src []byte, n int) { copy(dst[:n], src[:n]) }},
	{"I;16", "I;16B"}: {16, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[2*i], dst[2*i+1] = src[2*i+1], src[2*i]
		}
	}},
	{"RGB", "RGB"}: {24, func(dst, src []byte, n int) { copy(dst[:3*n], src[:3*n]) }},
	{"RGB", "RGB;16B"}: {48, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[3*i], dst[3*i+1], dst[3*i+2] = src[6*i], src[6*i+2], src[6*i+4]
		}
	}},
	{"P", "P;1"}: {1, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[i] = bitSample(src, i, 1)
		}
	}},
	{"P", "P;2"}: {2, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[i] = bitSample(src, i, 2)
		}
	}},
	{"P", "P;4"}: {4, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[i] = bitSample(src, i, 4)
		}
	}},
	{"P", "P"}:   {8, func(dst, src []byte, n int) { copy(dst[:n], src[:n]) }},
	{"LA", "LA"}: {16, func(dst, src []byte, n int) { copy(dst[:2*n], src[:2*n]) }},
	{"RGBA", "LA;16B"}: {32, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			v := src[4*i]
			dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = v, v, v, src[4*i+2]
		}
	}},
	{"RGBA", "RGBA"}: {32, func(dst, src []byte, n int) { copy(dst[:4*n], src[:4*n]) }},
	{"RGBA", "RGBA;16B"}: {64, func(dst, src []byte, n int) {
		for i := 0; i < n; i++ {
			dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = src[8*i], src[8*i+2], src[8*i+4], src[8*i+6]
		}
	}},
}

var (
	adamOffset   = [7]int{7, 3, 3, 1, 1, 0, 0}
	adamStartCol = [7]int{0, 4, 0, 2, 0, 1, 0}
	adamStartRow = [7]int{0, 0, 4, 0, 2, 0, 1}
	adamColInc   = [7]int{8, 8, 4, 4, 2, 2, 1}
	adamRowInc   = [7]int{8, 8, 8, 4, 4, 2, 2}
)

// zipDecoder is ImagingZipDecode with its ZIPSTATE, in ZIP_PNG mode.
type zipDecoder struct {
	pix        []byte
	stride     int
	imageW     int
	unpack     unpacker
	xoff, yoff int
	xsize      int
	ysize      int
	rowBytes   int
	interlaced bool

	started    bool
	pass       int
	y          int
	buffer     []byte
	previous   []byte
	lastOutput int
	inf        *inflater
	errcode    int
}

func (d *zipDecoder) rowLen(pass int) int {
	n := (d.xsize + adamOffset[pass]) / adamColInc[pass]
	return (n*d.unpack.bits + 7) / 8
}

// decode is one ImagingZipDecode call: it returns len(buf), or -1 when the
// decoder is done (errcode 0) or failed (errcode < 0).
func (d *zipDecoder) decode(buf []byte) int {
	if !d.started {
		d.buffer = make([]byte, d.rowBytes+1)
		d.previous = make([]byte, d.rowBytes+1)
		d.inf = newInflater()
		if d.interlaced {
			d.pass = 0
			d.y = adamStartRow[0]
		}
		d.started = true
	}
	rowLen := d.rowBytes
	if d.interlaced {
		rowLen = d.rowLen(d.pass)
	}
	in := buf
	for len(in) > 0 {
		consumed, produced, err := d.inf.inflate(in, d.buffer[d.lastOutput:rowLen+1])
		in = in[consumed:]
		if err < 0 {
			if err == zDataError {
				d.errcode = codecBroken
			} else {
				d.errcode = codecConfig
			}
			return -1
		}
		n := d.lastOutput + produced
		if n < rowLen+1 {
			d.lastOutput = n
			break
		}
		bpp := (d.unpack.bits + 7) / 8
		b, prev := d.buffer, d.previous
		switch b[0] {
		case 0:
		case 1:
			for i := bpp + 1; i <= rowLen; i++ {
				b[i] += b[i-bpp]
			}
		case 2:
			for i := 1; i <= rowLen; i++ {
				b[i] += prev[i]
			}
		case 3:
			i := 1
			for ; i <= bpp && i <= rowLen; i++ {
				b[i] += prev[i] / 2
			}
			for ; i <= rowLen; i++ {
				b[i] += byte((int(b[i-bpp]) + int(prev[i])) / 2)
			}
		case 4:
			i := 1
			for ; i <= bpp && i <= rowLen; i++ {
				b[i] += prev[i]
			}
			for ; i <= rowLen; i++ {
				a, bb, c := int(b[i-bpp]), int(prev[i]), int(prev[i-bpp])
				pa, pb, pc := abs(bb-c), abs(a-c), abs(a+bb-2*c)
				switch {
				case pa <= pb && pa <= pc:
					b[i] += byte(a)
				case pb <= pc:
					b[i] += byte(bb)
				default:
					b[i] += byte(c)
				}
			}
		default:
			d.errcode = codecUnknown
			return -1
		}
		if d.interlaced {
			col := adamStartCol[d.pass]
			if d.unpack.bits >= 8 {
				for i := 0; i < rowLen; i += bpp {
					at := (d.y*d.imageW + col) * d.stride
					d.unpack.fn(d.pix[at:at+d.stride], b[1+i:], 1)
					col += adamColInc[d.pass]
				}
			} else {
				rowBits := (d.xsize + adamOffset[d.pass]) / adamColInc[d.pass] * d.unpack.bits
				for i := 0; i < rowBits; i += d.unpack.bits {
					one := [1]byte{b[1+i/8] << uint(i%8)}
					at := (d.y*d.imageW + col) * d.stride
					d.unpack.fn(d.pix[at:at+d.stride], one[:], 1)
					col += adamColInc[d.pass]
				}
			}
			d.y += adamRowInc[d.pass]
			for d.y >= d.ysize || rowLen <= 0 {
				d.pass++
				if d.pass == 7 {
					d.y = d.ysize
					break
				}
				d.y = adamStartRow[d.pass]
				rowLen = d.rowLen(d.pass)
				clear(d.buffer)
			}
		} else {
			at := ((d.y+d.yoff)*d.imageW + d.xoff) * d.stride
			d.unpack.fn(d.pix[at:], b[1:], d.xsize)
			d.y++
		}
		d.lastOutput = 0
		if d.y >= d.ysize || err == zStreamEnd {
			return -1
		}
		d.buffer, d.previous = d.previous, d.buffer
	}
	return len(buf)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// errDeferred marks an exception Pillow raises after load, in getexif or
// convert, which this package reports from Load because pil.Info cannot
// carry the text-chunk value (a str) that causes it. The sanitizer's outcome
// is the same: not_an_image.
var errDeferred = errors.New("raised after load by getexif or convert")

func deferred(msg string) error { return fmt.Errorf("%w: %s", errDeferred, msg) }

// loadError wraps an exception raised inside load (always not_an_image).
func loadError(err error) error {
	return fmt.Errorf("PNG load: %w", err)
}

// Load is PngImageFile.load (ImageFile.load with load_prepare, load_read
// and load_end) followed by the info keys the sanitizer reads.
func (p *pngFile) Load() (*pil.Image, error) {
	if p.tile == nil {
		return nil, loadError(errNoTile)
	}
	// load_prepare
	interlaced := p.info["interlace"].truthy()
	idat := p.prepareIDAT
	if err := coreNewCheck(p.mode, p.width, p.height); err != nil {
		return nil, loadError(err)
	}
	w, h := int(p.width), int(p.height)
	stride := pil.BytesPerPixel(p.mode)
	if int64(w)*int64(h)*int64(stride) > 1<<34 {
		return nil, loadError(raise(excMemory, "image too large"))
	}
	pix := make([]byte, w*h*stride)
	if p.mode == "P" && p.hasPalette && len(p.palette)*8/24 > 256 {
		return nil, loadError(raise(excValue, "invalid palette size"))
	}

	// _getdecoder and setimage
	unpack, ok := unpackers[[2]string{p.mode, p.tile.rawmode}]
	if !ok {
		return nil, loadError(raise(excValue, "unknown raw mode for given image mode"))
	}
	ext := p.tile.extents
	if ext.kind != pyTuple || len(ext.tuple) != 4 {
		return nil, loadError(raise(excValue, "invalid extents"))
	}
	x0, y0, x1, y1 := int64(int32(ext.tuple[0])), int64(int32(ext.tuple[1])), int64(int32(ext.tuple[2])), int64(int32(ext.tuple[3]))
	if x0 < 0 || y0 < 0 || x1 <= x0 || y1 <= y0 || x1 > int64(w) || y1 > int64(h) {
		return nil, loadError(raise(excValue, "tile cannot extend outside image"))
	}
	xsize := int(x1 - x0)
	if xsize > intMax/unpack.bits-7 {
		return nil, loadError(raise(excMemory, "decoder buffer"))
	}
	dec := &zipDecoder{
		pix: pix, stride: stride, imageW: w, unpack: unpack,
		xoff: int(x0), yoff: int(y0), xsize: xsize, ysize: int(y1 - y0),
		rowBytes: (unpack.bits*xsize + 7) / 8, interlaced: interlaced,
	}

	// ImageFile.load's read loop over load_read
	p.pos = p.tile.offset
	errCode := -3
	for {
		s, err := p.loadRead(decoderBlock, &idat)
		if err != nil {
			if err.exc == excStruct || err.exc == excIndex {
				return nil, loadError(raise(excOS, "image file is truncated"))
			}
			return nil, loadError(err)
		}
		if len(s) == 0 {
			return nil, loadError(raise(excOS, "image file is truncated"))
		}
		n := dec.decode(s)
		errCode = dec.errcode
		if n < 0 {
			break
		}
	}
	if err := p.loadEnd(idat); err != nil {
		return nil, loadError(err)
	}
	if errCode < 0 {
		return nil, loadError(raise(excOS, "decoder error %d", errCode))
	}
	return p.image(pix)
}

// loadRead is PngImageFile.load_read.
func (p *pngFile) loadRead(n int, idat *int) ([]byte, *pyError) {
	for *idat == 0 {
		p.read(4)
		ref, err := p.readChunk()
		if err != nil {
			return nil, err
		}
		switch string(ref.cid) {
		case "IDAT", "DDAT":
			*idat = ref.length
		case "fdAT":
			if _, err := p.call(ref.cid, ref.pos, ref.length); err != nil && err.exc != excEOF {
				return nil, err
			}
			*idat = ref.length - 4
		default:
			p.queue = append(p.queue, ref)
			return nil, nil
		}
	}
	if n <= 0 {
		n = *idat
	} else if *idat < n {
		n = *idat
	}
	*idat -= n
	return p.read(n), nil
}

// loadEnd is PngImageFile.load_end.
func (p *pngFile) loadEnd(idat int) *pyError {
	if idat != 0 {
		p.read(idat)
	}
	for {
		p.read(4)
		ref, err := p.readChunk()
		if err != nil {
			break
		}
		if string(ref.cid) == "IEND" {
			break
		}
		if string(ref.cid) == "fcTL" && p.animated {
			p.queue = append(p.queue, ref)
			break
		}
		_, err = p.call(ref.cid, ref.pos, ref.length)
		if err == nil {
			continue
		}
		switch err.exc {
		case excEOF:
			length := ref.length
			if string(ref.cid) == "fdAT" {
				length -= 4
			}
			if _, err := p.safeRead(length); err != nil {
				return err
			}
		case excAttribute:
			if _, err := p.safeRead(ref.length); err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

// image builds the pil model of the loaded PNG and its info keys.
func (p *pngFile) image(pix []byte) (*pil.Image, error) {
	img := &pil.Image{Mode: p.mode, Width: int(p.width), Height: int(p.height), Pix: pix, Format: "PNG"}
	if p.mode == "P" && p.hasPalette {
		img.PaletteMode = "RGB"
		img.Palette = p.palette
	}
	if v, ok := p.info["transparency"]; ok {
		switch v.kind {
		case pyInt:
			img.Info.Transparency = &pil.Transparency{Kind: pil.TransparencyInt, Int: int(v.i)}
		case pyBytes:
			img.Info.Transparency = &pil.Transparency{Kind: pil.TransparencyBytes, Bytes: v.b}
		case pyTuple:
			t := &pil.Transparency{Kind: pil.TransparencyRGB}
			copy(t.RGB[:], []int{int(v.tuple[0]), int(v.tuple[1]), int(v.tuple[2])})
			img.Info.Transparency = t
		default:
			// A text chunk named "transparency": convert("RGBA") of a P
			// image raises ValueError and convert("RGB") of an L image
			// raises TypeError from putpixel; other modes never read it.
			if p.mode == "P" || p.mode == "L" {
				return nil, deferred("convert reads a str transparency")
			}
		}
	}
	if v, ok := p.info["exif"]; ok {
		switch {
		case v.kind == pyBytes:
			img.Info.HasExif, img.Info.Exif = true, v.b
		case v.kind == pyStr && v.s == "":
			img.Info.HasExif = true
		default:
			// getexif calls str.startswith(bytes): TypeError.
			return nil, deferred("getexif calls str.startswith(bytes)")
		}
	}
	if v, ok := p.info["Raw profile type exif"]; ok {
		img.Info.HasRawProfileExif, img.Info.RawProfileExif = true, v.s
	}
	if v, ok := p.info["XML:com.adobe.xmp"]; ok {
		img.Info.HasAdobeXMP, img.Info.AdobeXMP = true, v.s
	}
	if v, ok := p.info["xmp"]; ok {
		if v.kind == pyBytes {
			img.Info.HasXMP, img.Info.XMP = true, v.b
		} else if v.truthy() && !img.Info.HasAdobeXMP || v.truthy() && img.Info.AdobeXMP == "" {
			// getexif searches a bytes pattern in a str: TypeError, unless
			// the EXIF block already has an orientation tag.
			present, err := exif.OrientationTagPresent(img.Info)
			if err != nil {
				return nil, loadError(err)
			}
			if !present {
				return nil, deferred("getexif searches a bytes pattern in a str")
			}
		}
	}
	return img, nil
}
