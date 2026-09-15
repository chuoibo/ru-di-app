package webpdec

// Port of dec/alpha_dec.c and the unfilters of dsp/filters.c.

const (
	filterNone       = 0
	filterHorizontal = 1
	filterVertical   = 2
	filterGradient   = 3
)

type alphDecoder struct {
	width, height int
	method        int
	filter        int
	preProcessing int
	vp8l          *vp8lDecoder
	io            vp8Io
	use8bDecode   bool
	output        []byte
	prevLine      int // offset into output, -1 for NULL
}

func gradientPredictor(a, b, c byte) byte {
	g := int(a) + int(b) - int(c)
	if g&^0xff == 0 {
		return byte(g)
	}
	if g < 0 {
		return 0
	}
	return 255
}

// unfilter is WebPUnfilters[filter]; prev nil stands for NULL and in may
// alias out.
func unfilter(filter int, prev, in, out []byte, width int) {
	switch filter {
	case filterNone:
		copy(out[:width], in[:width])
	case filterHorizontal:
		horizontalUnfilter(prev, in, out, width)
	case filterVertical:
		if prev == nil {
			horizontalUnfilter(nil, in, out, width)
			return
		}
		for i := 0; i < width; i++ {
			out[i] = prev[i] + in[i]
		}
	case filterGradient:
		if prev == nil {
			horizontalUnfilter(nil, in, out, width)
			return
		}
		top := prev[0]
		topLeft, left := top, top
		for i := 0; i < width; i++ {
			top = prev[i]
			left = in[i] + gradientPredictor(left, top, topLeft)
			topLeft = top
			out[i] = left
		}
	}
}

func horizontalUnfilter(prev, in, out []byte, width int) {
	pred := byte(0)
	if prev != nil {
		pred = prev[0]
	}
	for i := 0; i < width; i++ {
		out[i] = pred + in[i]
		pred = out[i]
	}
}

// alphInit is ALPHInit.
func alphInit(a *alphDecoder, data []byte, src *vp8Io, output []byte) bool {
	a.output = output
	a.width = src.width
	a.height = src.height
	a.prevLine = -1
	if len(data) <= 1 {
		return false
	}
	a.method = int(data[0] & 0x03)
	a.filter = int((data[0] >> 2) & 0x03)
	a.preProcessing = int((data[0] >> 4) & 0x03)
	rsrv := (data[0] >> 6) & 0x03
	if a.method > 1 || a.preProcessing > 1 || rsrv != 0 {
		return false
	}
	a.io = vp8Io{
		width:           src.width,
		height:          src.height,
		cropLeft:        src.cropLeft,
		cropRight:       src.cropRight,
		cropTop:         src.cropTop,
		cropBottom:      src.cropBottom,
		fancyUpsampling: src.fancyUpsampling,
	}
	alphaData := data[1:]
	if a.method == 0 {
		return len(alphaData) >= a.width*a.height
	}
	return vp8lDecodeAlphaHeader(a, alphaData)
}

// alphDecode is ALPHDecode.
func (dec *vp8Decoder) alphDecode(row, numRows int) bool {
	a := dec.alph
	width := a.width
	height := a.io.cropBottom
	if a.method == 0 {
		prev := dec.alphaPrevLine
		deltas := 1 + row*width
		dst := row * width
		for y := 0; y < numRows; y++ {
			var pv []byte
			if prev >= 0 {
				pv = dec.alphaPlane[prev : prev+width]
			}
			unfilter(a.filter, pv, dec.alphaData[deltas:deltas+width], dec.alphaPlane[dst:dst+width], width)
			prev = dst
			dst += width
			deltas += width
		}
		dec.alphaPrevLine = prev
	} else if !vp8lDecodeAlphaImageStream(a, row+numRows) {
		return false
	}
	if row+numRows >= height {
		dec.isAlphaDecoded = true
	}
	return true
}

// decompressAlphaRows is VP8DecompressAlphaRows; it returns the alpha plane
// and the offset of row in it.
func (dec *vp8Decoder) decompressAlphaRows(io *vp8Io, row, numRows int) ([]byte, int, bool) {
	width := io.width
	height := io.cropBottom
	if row < 0 || numRows <= 0 || row+numRows > height {
		return nil, 0, false
	}
	if !dec.isAlphaDecoded {
		if dec.alph == nil {
			dec.alph = &alphDecoder{}
			dec.alphaPlane = make([]byte, width*height)
			dec.alphaPrevLine = -1
			if !alphInit(dec.alph, dec.alphaData, io, dec.alphaPlane) {
				dec.alph, dec.alphaPlane = nil, nil
				return nil, 0, false
			}
			if dec.alph.preProcessing == 1 {
				numRows = height - row
			}
		}
		if !dec.alphDecode(row, numRows) {
			dec.alph, dec.alphaPlane = nil, nil
			return nil, 0, false
		}
		if dec.isAlphaDecoded {
			dec.alph = nil
		}
	}
	return dec.alphaPlane, row * width, true
}

// applyFilter is AlphaApplyFilter on output rows starting at out.
func (a *alphDecoder) applyFilter(firstRow, lastRow, out, stride int) {
	if a.filter == filterNone {
		return
	}
	prev := a.prevLine
	for y := firstRow; y < lastRow; y++ {
		var pv []byte
		if prev >= 0 {
			pv = a.output[prev : prev+stride]
		}
		row := a.output[out : out+stride]
		unfilter(a.filter, pv, row, row, stride)
		prev = out
		out += stride
	}
	a.prevLine = prev
}
