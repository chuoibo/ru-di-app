package webpdec

// Port of the RGBA output path of dec/io_dec.c (CustomSetup, CustomPut,
// EmitFancyRGB, EmitAlphaRGB), dsp/upsampling.c and dsp/yuv.h, for the
// decoder config WebPAnimDecoder builds: MODE_RGBA into external memory,
// no cropping, no scaling, fancy upsampling on.

type statusCode int

const (
	statusOK statusCode = iota
	statusOutOfMemory
	statusInvalidParam
	statusBitstreamError
	statusUnsupportedFeature
	statusSuspended
	statusUserAbort
	statusNotEnoughData
)

// vp8Io is the part of VP8Io the decoders use. y, u, v index buf.
type vp8Io struct {
	width, height int
	mbY, mbW, mbH int

	buf               []byte
	y, u, v           int
	yStride, uvStride int

	fancyUpsampling bool
	bypassFiltering bool

	cropLeft, cropRight, cropTop, cropBottom int

	a    []byte
	aOff int

	data   []byte
	params *decParams
}

// decParams is WebPDecParams plus the external RGBA WebPDecBuffer: rows
// are written to out[rgba+y*stride:].
type decParams struct {
	out    []byte
	rgba   int
	stride int
	size   int

	width, height int

	tmpY, tmpU, tmpV []byte
	lastY            int
}

// allocate is WebPAllocateDecBuffer for external memory (CheckDecBuffer).
func (p *decParams) allocate(width, height int) statusCode {
	if width <= 0 || height <= 0 {
		return statusInvalidParam
	}
	p.width, p.height = width, height
	if uint64(p.stride)*uint64(height-1)+uint64(width)*4 > uint64(p.size) {
		return statusInvalidParam
	}
	if p.stride < width*4 {
		return statusInvalidParam
	}
	return statusOK
}

// initFromOptions is WebPIoInitFromOptions with zeroed options.
func initFromOptions(io *vp8Io) {
	io.cropLeft, io.cropTop = 0, 0
	io.cropRight, io.cropBottom = io.width, io.height
	io.mbW, io.mbH = io.width, io.height
	io.bypassFiltering = false
	io.fancyUpsampling = true
}

// setup is CustomSetup.
func (p *decParams) setup(io *vp8Io) bool {
	initFromOptions(io)
	uvW := (io.mbW + 1) >> 1
	p.tmpY = make([]byte, io.mbW)
	p.tmpU = make([]byte, uvW)
	p.tmpV = make([]byte, uvW)
	return true
}

// put is CustomPut.
func (p *decParams) put(io *vp8Io) bool {
	if io.mbW <= 0 || io.mbH <= 0 {
		return false
	}
	n := p.emitFancyRGB(io)
	p.emitAlphaRGB(io)
	p.lastY += n
	return true
}

func mulHi(v, coeff int) int { return (v * coeff) >> 8 }

func yuvClip8(v int) byte {
	if v&^((256<<6)-1) == 0 {
		return byte(v >> 6)
	}
	if v < 0 {
		return 0
	}
	return 255
}

// yuvToRgba is VP8YuvToRgba.
func yuvToRgba(y byte, u, v uint32, dst []byte) {
	yy, uu, vv := int(y), int(uint8(u)), int(uint8(v))
	dst[0] = yuvClip8(mulHi(yy, 19077) + mulHi(vv, 26149) - 14234)
	dst[1] = yuvClip8(mulHi(yy, 19077) - mulHi(uu, 6419) - mulHi(vv, 13320) + 8708)
	dst[2] = yuvClip8(mulHi(yy, 19077) + mulHi(uu, 33050) - 17685)
	dst[3] = 0xff
}

func loadUV(u, v byte) uint32 { return uint32(u) | uint32(v)<<16 }

// upsampleRgbaLinePair is UpsampleRgbaLinePair_C.
func upsampleRgbaLinePair(topY, bottomY, topU, topV, curU, curV, topDst, bottomDst []byte, length int) {
	lastPixelPair := (length - 1) >> 1
	tlUV := loadUV(topU[0], topV[0])
	lUV := loadUV(curU[0], curV[0])
	uv0 := (3*tlUV + lUV + 0x00020002) >> 2
	yuvToRgba(topY[0], uv0&0xff, uv0>>16, topDst)
	if bottomY != nil {
		uv0 := (3*lUV + tlUV + 0x00020002) >> 2
		yuvToRgba(bottomY[0], uv0&0xff, uv0>>16, bottomDst)
	}
	for x := 1; x <= lastPixelPair; x++ {
		tUV := loadUV(topU[x], topV[x])
		uv := loadUV(curU[x], curV[x])
		avg := tlUV + tUV + lUV + uv + 0x00080008
		diag12 := (avg + 2*(tUV+lUV)) >> 3
		diag03 := (avg + 2*(tlUV+uv)) >> 3
		a := (diag12 + tlUV) >> 1
		b := (diag03 + tUV) >> 1
		yuvToRgba(topY[2*x-1], a&0xff, a>>16, topDst[(2*x-1)*4:])
		yuvToRgba(topY[2*x], b&0xff, b>>16, topDst[(2*x)*4:])
		if bottomY != nil {
			a := (diag03 + lUV) >> 1
			b := (diag12 + uv) >> 1
			yuvToRgba(bottomY[2*x-1], a&0xff, a>>16, bottomDst[(2*x-1)*4:])
			yuvToRgba(bottomY[2*x], b&0xff, b>>16, bottomDst[(2*x)*4:])
		}
		tlUV = tUV
		lUV = uv
	}
	if length&1 == 0 {
		uv0 := (3*tlUV + lUV + 0x00020002) >> 2
		yuvToRgba(topY[length-1], uv0&0xff, uv0>>16, topDst[(length-1)*4:])
		if bottomY != nil {
			uv0 := (3*lUV + tlUV + 0x00020002) >> 2
			yuvToRgba(bottomY[length-1], uv0&0xff, uv0>>16, bottomDst[(length-1)*4:])
		}
	}
}

// emitFancyRGB is EmitFancyRGB.
func (p *decParams) emitFancyRGB(io *vp8Io) int {
	numLinesOut := io.mbH
	buf := io.buf
	dst := p.rgba + io.mbY*p.stride
	curY, curU, curV := io.y, io.u, io.v
	topU, topV := p.tmpU, p.tmpV
	y := io.mbY
	yEnd := io.mbY + io.mbH
	mbW := io.mbW
	uvW := (mbW + 1) / 2
	if y == 0 {
		upsampleRgbaLinePair(buf[curY:], nil, buf[curU:], buf[curV:], buf[curU:], buf[curV:], p.out[dst:], nil, mbW)
	} else {
		upsampleRgbaLinePair(p.tmpY, buf[curY:], topU, topV, buf[curU:], buf[curV:], p.out[dst-p.stride:], p.out[dst:], mbW)
		numLinesOut++
	}
	for ; y+2 < yEnd; y += 2 {
		topU, topV = buf[curU:], buf[curV:]
		curU += io.uvStride
		curV += io.uvStride
		dst += 2 * p.stride
		curY += 2 * io.yStride
		upsampleRgbaLinePair(buf[curY-io.yStride:], buf[curY:], topU, topV, buf[curU:], buf[curV:], p.out[dst-p.stride:], p.out[dst:], mbW)
	}
	curY += io.yStride
	if io.cropTop+yEnd < io.cropBottom {
		copy(p.tmpY, buf[curY:curY+mbW])
		copy(p.tmpU, buf[curU:curU+uvW])
		copy(p.tmpV, buf[curV:curV+uvW])
		numLinesOut--
	} else if yEnd&1 == 0 {
		upsampleRgbaLinePair(buf[curY:], nil, buf[curU:], buf[curV:], buf[curU:], buf[curV:], p.out[dst+p.stride:], nil, mbW)
	}
	return numLinesOut
}

// emitAlphaRGB is EmitAlphaRGB with GetAlphaSourceRow and
// WebPDispatchAlpha for a non-premultiplied mode.
func (p *decParams) emitAlphaRGB(io *vp8Io) {
	if io.a == nil {
		return
	}
	alpha := io.aOff
	mbW := io.mbW
	startY := io.mbY
	numRows := io.mbH
	if io.fancyUpsampling {
		if startY == 0 {
			numRows--
		} else {
			startY--
			alpha -= io.width
		}
		if io.cropTop+io.mbY+io.mbH == io.cropBottom {
			numRows = io.cropBottom - io.cropTop - startY
		}
	}
	dst := p.rgba + startY*p.stride + 3
	for j := 0; j < numRows; j++ {
		row := io.a[alpha : alpha+mbW]
		for i, a := range row {
			p.out[dst+4*i] = a
		}
		alpha += io.width
		dst += p.stride
	}
}
