package webpdec

// Port of dec/frame_dec.c for mt_method 0 (one cache line, no dithering).

var scan = [16]int{
	0 + 0*bps, 4 + 0*bps, 8 + 0*bps, 12 + 0*bps,
	0 + 4*bps, 4 + 4*bps, 8 + 4*bps, 12 + 4*bps,
	0 + 8*bps, 4 + 8*bps, 8 + 8*bps, 12 + 8*bps,
	0 + 12*bps, 4 + 12*bps, 8 + 12*bps, 12 + 12*bps,
}

func checkMode(mbX, mbY int, mode uint8) int {
	if mode == bDCPred {
		if mbX == 0 {
			if mbY == 0 {
				return bDCPredNoTopLeft
			}
			return bDCPredNoLeft
		}
		if mbY == 0 {
			return bDCPredNoTop
		}
		return bDCPred
	}
	return int(mode)
}

func doTransform(bits uint32, src []int16, dst []byte, d int) {
	switch bits >> 30 {
	case 3:
		transformTwo(src, dst, d, false)
	case 2:
		transformAC3(src, dst, d)
	case 1:
		transformDC(src, dst, d)
	}
}

func doUVTransform(bits uint32, src []int16, dst []byte, d int) {
	if bits&0xff != 0 {
		if bits&0xaa != 0 {
			transformUV(src, dst, d)
		} else {
			transformDCUV(src, dst, d)
		}
	}
}

// reconstructRow is ReconstructRow.
func (dec *vp8Decoder) reconstructRow(mbY int) {
	b := dec.yuvB[:]
	yDst, uDst, vDst := yOff, uOff, vOff
	for j := 0; j < 16; j++ {
		b[yDst+j*bps-1] = 129
	}
	for j := 0; j < 8; j++ {
		b[uDst+j*bps-1] = 129
		b[vDst+j*bps-1] = 129
	}
	if mbY > 0 {
		b[yDst-1-bps], b[uDst-1-bps], b[vDst-1-bps] = 129, 129, 129
	} else {
		for i := 0; i < 16+4+1; i++ {
			b[yDst-bps-1+i] = 127
		}
		for i := 0; i < 8+1; i++ {
			b[uDst-bps-1+i] = 127
			b[vDst-bps-1+i] = 127
		}
	}
	for mbX := 0; mbX < dec.mbW; mbX++ {
		block := &dec.mbData[mbX]
		if mbX > 0 {
			for j := -1; j < 16; j++ {
				copy(b[yDst+j*bps-4:yDst+j*bps], b[yDst+j*bps+12:yDst+j*bps+16])
			}
			for j := -1; j < 8; j++ {
				copy(b[uDst+j*bps-4:uDst+j*bps], b[uDst+j*bps+4:uDst+j*bps+8])
				copy(b[vDst+j*bps-4:vDst+j*bps], b[vDst+j*bps+4:vDst+j*bps+8])
			}
		}
		top := &dec.yuvT[mbX]
		coeffs := block.coeffs[:]
		bits := block.nonZeroY
		if mbY > 0 {
			copy(b[yDst-bps:yDst-bps+16], top.y[:])
			copy(b[uDst-bps:uDst-bps+8], top.u[:])
			copy(b[vDst-bps:vDst-bps+8], top.v[:])
		}
		if block.isI4x4 {
			topRight := yDst - bps + 16
			if mbY > 0 {
				if mbX >= dec.mbW-1 {
					for i := 0; i < 4; i++ {
						b[topRight+i] = top.y[15]
					}
				} else {
					copy(b[topRight:topRight+4], dec.yuvT[mbX+1].y[:4])
				}
			}
			for k := 1; k <= 3; k++ {
				copy(b[topRight+4*k*bps:topRight+4*k*bps+4], b[topRight:topRight+4])
			}
			for n := 0; n < 16; n++ {
				dst := yDst + scan[n]
				predLuma4[block.imodes[n]](b, dst)
				doTransform(bits, coeffs[n*16:], b, dst)
				bits <<= 2
			}
		} else {
			predLuma16[checkMode(mbX, mbY, block.imodes[0])](b, yDst)
			if bits != 0 {
				for n := 0; n < 16; n++ {
					doTransform(bits, coeffs[n*16:], b, yDst+scan[n])
					bits <<= 2
				}
			}
		}
		bitsUV := block.nonZeroUV
		pf := checkMode(mbX, mbY, block.uvmode)
		predChroma8[pf](b, uDst)
		predChroma8[pf](b, vDst)
		doUVTransform(bitsUV>>0, coeffs[16*16:], b, uDst)
		doUVTransform(bitsUV>>8, coeffs[20*16:], b, vDst)
		if mbY < dec.mbH-1 {
			copy(top.y[:], b[yDst+15*bps:yDst+15*bps+16])
			copy(top.u[:], b[uDst+7*bps:uDst+7*bps+8])
			copy(top.v[:], b[vDst+7*bps:vDst+7*bps+8])
		}
		yOut := dec.cacheY + mbX*16
		uOut := dec.cacheU + mbX*8
		vOut := dec.cacheV + mbX*8
		for j := 0; j < 16; j++ {
			copy(dec.cache[yOut+j*dec.cacheYStride:yOut+j*dec.cacheYStride+16], b[yDst+j*bps:yDst+j*bps+16])
		}
		for j := 0; j < 8; j++ {
			copy(dec.cache[uOut+j*dec.cacheUVStride:uOut+j*dec.cacheUVStride+8], b[uDst+j*bps:uDst+j*bps+8])
			copy(dec.cache[vOut+j*dec.cacheUVStride:vOut+j*dec.cacheUVStride+8], b[vDst+j*bps:vDst+j*bps+8])
		}
	}
}

var filterExtraRows = [3]int{0, 2, 8}

func (dec *vp8Decoder) doFilter(mbX, mbY int) {
	yBps := dec.cacheYStride
	fi := &dec.fInfo[mbX]
	yDst := dec.cacheY + mbX*16
	ilevel := int(fi.fILevel)
	limit := int(fi.fLimit)
	if limit == 0 {
		return
	}
	c := dec.cache
	if dec.filterType == 1 {
		if mbX > 0 {
			simpleHFilter16(c, yDst, yBps, limit+4)
		}
		if fi.fInner != 0 {
			simpleHFilter16i(c, yDst, yBps, limit)
		}
		if mbY > 0 {
			simpleVFilter16(c, yDst, yBps, limit+4)
		}
		if fi.fInner != 0 {
			simpleVFilter16i(c, yDst, yBps, limit)
		}
		return
	}
	uvBps := dec.cacheUVStride
	uDst := dec.cacheU + mbX*8
	vDst := dec.cacheV + mbX*8
	hevT := int(fi.hevThresh)
	if mbX > 0 {
		hFilter16(c, yDst, yBps, limit+4, ilevel, hevT)
		hFilter8(c, uDst, vDst, uvBps, limit+4, ilevel, hevT)
	}
	if fi.fInner != 0 {
		hFilter16i(c, yDst, yBps, limit, ilevel, hevT)
		hFilter8i(c, uDst, vDst, uvBps, limit, ilevel, hevT)
	}
	if mbY > 0 {
		vFilter16(c, yDst, yBps, limit+4, ilevel, hevT)
		vFilter8(c, uDst, vDst, uvBps, limit+4, ilevel, hevT)
	}
	if fi.fInner != 0 {
		vFilter16i(c, yDst, yBps, limit, ilevel, hevT)
		vFilter8i(c, uDst, vDst, uvBps, limit, ilevel, hevT)
	}
}

func (dec *vp8Decoder) precomputeFilterStrengths() {
	if dec.filterType <= 0 {
		return
	}
	hdr := &dec.filterHdr
	for s := 0; s < 4; s++ {
		var baseLevel int
		if dec.segmentHdr.useSegment {
			baseLevel = int(dec.segmentHdr.filterStrength[s])
			if !dec.segmentHdr.absoluteDelta {
				baseLevel += hdr.level
			}
		} else {
			baseLevel = hdr.level
		}
		for i4x4 := 0; i4x4 <= 1; i4x4++ {
			info := &dec.fstrengths[s][i4x4]
			level := baseLevel
			if hdr.useLfDelta {
				level += hdr.refLfDelta[0]
				if i4x4 != 0 {
					level += hdr.modeLfDelta[0]
				}
			}
			if level < 0 {
				level = 0
			} else if level > 63 {
				level = 63
			}
			if level > 0 {
				ilevel := level
				if hdr.sharpness > 0 {
					if hdr.sharpness > 4 {
						ilevel >>= 2
					} else {
						ilevel >>= 1
					}
					if ilevel > 9-hdr.sharpness {
						ilevel = 9 - hdr.sharpness
					}
				}
				if ilevel < 1 {
					ilevel = 1
				}
				info.fILevel = uint8(ilevel)
				info.fLimit = uint8(2*level + ilevel)
				switch {
				case level >= 40:
					info.hevThresh = 2
				case level >= 15:
					info.hevThresh = 1
				default:
					info.hevThresh = 0
				}
			} else {
				info.fLimit = 0
			}
			info.fInner = uint8(i4x4)
		}
	}
}

// finishRow is FinishRow.
func (dec *vp8Decoder) finishRow(io *vp8Io) bool {
	extra := filterExtraRows[dec.filterType]
	ysize := extra * dec.cacheYStride
	uvsize := (extra / 2) * dec.cacheUVStride
	ydst := dec.cacheY - ysize
	udst := dec.cacheU - uvsize
	vdst := dec.cacheV - uvsize
	mbY := dec.ctxMbY
	isFirst := mbY == 0
	isLast := mbY >= dec.brMbY-1
	if dec.ctxFilterRy {
		for mbX := dec.tlMbX; mbX < dec.brMbX; mbX++ {
			dec.doFilter(mbX, mbY)
		}
	}
	ok := true
	yStart := mbY * 16
	yEnd := (mbY + 1) * 16
	if !isFirst {
		yStart -= extra
		io.y, io.u, io.v = ydst, udst, vdst
	} else {
		io.y, io.u, io.v = dec.cacheY, dec.cacheU, dec.cacheV
	}
	if !isLast {
		yEnd -= extra
	}
	if yEnd > io.cropBottom {
		yEnd = io.cropBottom
	}
	io.a = nil
	if dec.alphaData != nil && yStart < yEnd {
		plane, off, aok := dec.decompressAlphaRows(io, yStart, yEnd-yStart)
		if !aok {
			return dec.setError(statusBitstreamError)
		}
		io.a, io.aOff = plane, off
	}
	if yStart < yEnd {
		io.mbY = yStart - io.cropTop
		io.mbW = io.cropRight - io.cropLeft
		io.mbH = yEnd - yStart
		ok = io.params.put(io)
	}
	if !isLast {
		copy(dec.cache[dec.cacheY-ysize:dec.cacheY], dec.cache[ydst+16*dec.cacheYStride:ydst+16*dec.cacheYStride+ysize])
		copy(dec.cache[dec.cacheU-uvsize:dec.cacheU], dec.cache[udst+8*dec.cacheUVStride:udst+8*dec.cacheUVStride+uvsize])
		copy(dec.cache[dec.cacheV-uvsize:dec.cacheV], dec.cache[vdst+8*dec.cacheUVStride:vdst+8*dec.cacheUVStride+uvsize])
	}
	return ok
}

// processRow is VP8ProcessRow for mt_method 0.
func (dec *vp8Decoder) processRow(io *vp8Io) bool {
	filterRow := dec.filterType > 0 && dec.mbY >= dec.tlMbY && dec.mbY <= dec.brMbY
	dec.ctxMbY = dec.mbY
	dec.ctxFilterRy = filterRow
	dec.reconstructRow(dec.mbY)
	return dec.finishRow(io)
}

// enterCritical is VP8EnterCritical.
func (dec *vp8Decoder) enterCritical(io *vp8Io) statusCode {
	if !io.params.setup(io) {
		dec.setError(statusUserAbort)
		return dec.status
	}
	if io.bypassFiltering {
		dec.filterType = 0
	}
	extra := filterExtraRows[dec.filterType]
	if dec.filterType == 2 {
		dec.tlMbX, dec.tlMbY = 0, 0
	} else {
		dec.tlMbX = (io.cropLeft - extra) >> 4
		dec.tlMbY = (io.cropTop - extra) >> 4
		if dec.tlMbX < 0 {
			dec.tlMbX = 0
		}
		if dec.tlMbY < 0 {
			dec.tlMbY = 0
		}
	}
	dec.brMbY = (io.cropBottom + 15 + extra) >> 4
	dec.brMbX = (io.cropRight + 15 + extra) >> 4
	if dec.brMbX > dec.mbW {
		dec.brMbX = dec.mbW
	}
	if dec.brMbY > dec.mbH {
		dec.brMbY = dec.mbH
	}
	dec.precomputeFilterStrengths()
	return statusOK
}

// initFrame is VP8InitFrame: AllocateMemory and InitIo for one cache line.
func (dec *vp8Decoder) initFrame(io *vp8Io) bool {
	mbW := dec.mbW
	dec.intraT = make([]uint8, 4*mbW)
	for i := range dec.intraT {
		dec.intraT[i] = bDCPred
	}
	dec.yuvT = make([]vp8TopSamples, mbW)
	dec.mbInfo = make([]vp8MB, mbW+1)
	if dec.filterType > 0 {
		dec.fInfo = make([]vp8FInfo, mbW)
	}
	dec.mbData = make([]vp8MBData, mbW)
	dec.cacheYStride = 16 * mbW
	dec.cacheUVStride = 8 * mbW
	extra := filterExtraRows[dec.filterType]
	extraY := extra * dec.cacheYStride
	extraUV := (extra / 2) * dec.cacheUVStride
	dec.cacheY = extraY
	dec.cacheU = dec.cacheY + 16*dec.cacheYStride + extraUV
	dec.cacheV = dec.cacheU + 8*dec.cacheUVStride + extraUV
	dec.cache = make([]byte, dec.cacheV+8*dec.cacheUVStride)
	dec.initScanline()
	io.mbY = 0
	io.y, io.u, io.v = dec.cacheY, dec.cacheU, dec.cacheV
	io.yStride = dec.cacheYStride
	io.uvStride = dec.cacheUVStride
	io.buf = dec.cache
	io.a = nil
	return true
}
