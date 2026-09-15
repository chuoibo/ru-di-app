package webpdec

// Port of dec/vp8_dec.c and dec/tree_dec.c for a single-threaded key-frame
// decode, the only kind WebPDecode accepts.

type vp8FrameHeader struct {
	keyFrame        bool
	profile         uint32
	show            bool
	partitionLength uint32
}

type vp8SegmentHeader struct {
	useSegment     bool
	updateMap      bool
	absoluteDelta  bool
	quantizer      [4]int8
	filterStrength [4]int8
}

type vp8FilterHeader struct {
	simple      bool
	level       int
	sharpness   int
	useLfDelta  bool
	refLfDelta  [4]int
	modeLfDelta [4]int
}

type vp8FInfo struct {
	fLimit    uint8
	fILevel   uint8
	fInner    uint8
	hevThresh uint8
}

type vp8MB struct {
	nz   uint8
	nzDC uint8
}

type vp8QuantMatrix struct {
	y1, y2, uv [2]int
	uvQuant    int
}

type vp8MBData struct {
	coeffs    [384]int16
	isI4x4    bool
	imodes    [16]uint8
	uvmode    uint8
	nonZeroY  uint32
	nonZeroUV uint32
	skip      bool
	segment   uint8
}

type vp8TopSamples struct {
	y    [16]uint8
	u, v [8]uint8
}

const (
	bDCPred = 0
	bTMPred = 1
	bVEPred = 2
	bHEPred = 3
	bRDPred = 4
	bVRPred = 5
	bLDPred = 6
	bVLPred = 7
	bHDPred = 8
	bHUPred = 9

	dcPred = bDCPred
	vPred  = bVEPred
	hPred  = bHEPred
	tmPred = bTMPred

	bDCPredNoTop     = 4
	bDCPredNoLeft    = 5
	bDCPredNoTopLeft = 6
)

type vp8Decoder struct {
	status statusCode
	ready  bool

	br         vp8BitReader
	frmHdr     vp8FrameHeader
	segmentHdr vp8SegmentHeader
	filterHdr  vp8FilterHeader

	mbW, mbH       int
	tlMbX, tlMbY   int
	brMbX, brMbY   int
	numPartsMinus1 uint32
	parts          [8]vp8BitReader

	dqm          [4]vp8QuantMatrix
	segments     [3]uint8
	bands        [4 * 8 * 3 * 11]uint8
	useSkipProba bool
	skipP        int

	intraT []uint8
	intraL [4]uint8
	yuvT   []vp8TopSamples
	mbInfo []vp8MB // mbInfo[0] is mb_info[-1]
	fInfo  []vp8FInfo
	yuvB   [yuvSize]byte

	cache                       []byte
	cacheY, cacheU, cacheV      int
	cacheYStride, cacheUVStride int

	mbX, mbY   int
	mbData     []vp8MBData
	filterType int
	fstrengths [4][2]vp8FInfo

	// Row being finished (thread_ctx in the single-threaded path).
	ctxMbY      int
	ctxFilterRy bool

	alphaData      []byte
	alph           *alphDecoder
	isAlphaDecoded bool
	alphaPlane     []byte
	alphaPrevLine  int
}

const (
	yuvSize = bps*17 + bps*9
	yOff    = bps*1 + 8
	uOff    = yOff + bps*16 + bps
	vOff    = uOff + 16
)

func (dec *vp8Decoder) setError(st statusCode) bool {
	if dec.status == statusOK {
		dec.status = st
		dec.ready = false
	}
	return false
}

func vp8CheckSignature(data []byte) bool {
	return len(data) >= 3 && data[0] == 0x9d && data[1] == 0x01 && data[2] == 0x2a
}

// vp8GetInfo is VP8GetInfo.
func vp8GetInfo(data []byte, chunkSize uint64) (w, h int, ok bool) {
	if len(data) < 10 {
		return 0, 0, false
	}
	if !vp8CheckSignature(data[3:]) {
		return 0, 0, false
	}
	bits := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	keyFrame := bits&1 == 0
	w = int((uint32(data[7])<<8 | uint32(data[6])) & 0x3fff)
	h = int((uint32(data[9])<<8 | uint32(data[8])) & 0x3fff)
	if !keyFrame {
		return 0, 0, false
	}
	if (bits>>1)&7 > 3 {
		return 0, 0, false
	}
	if (bits>>4)&1 == 0 {
		return 0, 0, false
	}
	if uint64(bits>>5) >= chunkSize {
		return 0, 0, false
	}
	if w == 0 || h == 0 {
		return 0, 0, false
	}
	return w, h, true
}

func (dec *vp8Decoder) parseSegmentHeader() bool {
	br := &dec.br
	hdr := &dec.segmentHdr
	hdr.useSegment = br.getValue(1) != 0
	if hdr.useSegment {
		hdr.updateMap = br.getValue(1) != 0
		if br.getValue(1) != 0 {
			hdr.absoluteDelta = br.getValue(1) != 0
			for s := 0; s < 4; s++ {
				v := 0
				if br.getValue(1) != 0 {
					v = br.getSignedValue(7)
				}
				hdr.quantizer[s] = int8(v)
			}
			for s := 0; s < 4; s++ {
				v := 0
				if br.getValue(1) != 0 {
					v = br.getSignedValue(6)
				}
				hdr.filterStrength[s] = int8(v)
			}
		}
		if hdr.updateMap {
			for s := 0; s < 3; s++ {
				v := uint8(255)
				if br.getValue(1) != 0 {
					v = uint8(br.getValue(8))
				}
				dec.segments[s] = v
			}
		}
	} else {
		hdr.updateMap = false
	}
	return !br.eof
}

func (dec *vp8Decoder) parsePartitions(buf []byte) statusCode {
	br := &dec.br
	size := len(buf)
	dec.numPartsMinus1 = (1 << br.getValue(2)) - 1
	lastPart := int(dec.numPartsMinus1)
	if size < 3*lastPart {
		return statusNotEnoughData
	}
	partStart := lastPart * 3
	sizeLeft := size - lastPart*3
	for p := 0; p < lastPart; p++ {
		sz := buf[p*3:]
		psize := int(sz[0]) | int(sz[1])<<8 | int(sz[2])<<16
		if psize > sizeLeft {
			psize = sizeLeft
		}
		dec.parts[p].init(buf[partStart : partStart+psize])
		partStart += psize
		sizeLeft -= psize
	}
	dec.parts[lastPart].init(buf[partStart : partStart+sizeLeft])
	if partStart < size {
		return statusOK
	}
	return statusNotEnoughData
}

func (dec *vp8Decoder) parseFilterHeader() bool {
	br := &dec.br
	hdr := &dec.filterHdr
	hdr.simple = br.getValue(1) != 0
	hdr.level = int(br.getValue(6))
	hdr.sharpness = int(br.getValue(3))
	hdr.useLfDelta = br.getValue(1) != 0
	if hdr.useLfDelta {
		if br.getValue(1) != 0 {
			for i := 0; i < 4; i++ {
				if br.getValue(1) != 0 {
					hdr.refLfDelta[i] = br.getSignedValue(6)
				}
			}
			for i := 0; i < 4; i++ {
				if br.getValue(1) != 0 {
					hdr.modeLfDelta[i] = br.getSignedValue(6)
				}
			}
		}
	}
	switch {
	case hdr.level == 0:
		dec.filterType = 0
	case hdr.simple:
		dec.filterType = 1
	default:
		dec.filterType = 2
	}
	return !br.eof
}

// getHeaders is VP8GetHeaders.
func (dec *vp8Decoder) getHeaders(io *vp8Io) bool {
	dec.status = statusOK
	buf := io.data
	if len(buf) < 4 {
		return dec.setError(statusNotEnoughData)
	}
	bits := uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16
	frm := &dec.frmHdr
	frm.keyFrame = bits&1 == 0
	frm.profile = (bits >> 1) & 7
	frm.show = (bits>>4)&1 != 0
	frm.partitionLength = bits >> 5
	if frm.profile > 3 {
		return dec.setError(statusBitstreamError)
	}
	if !frm.show {
		return dec.setError(statusUnsupportedFeature)
	}
	buf = buf[3:]
	if frm.keyFrame {
		if len(buf) < 7 {
			return dec.setError(statusNotEnoughData)
		}
		if !vp8CheckSignature(buf) {
			return dec.setError(statusBitstreamError)
		}
		width := int((uint32(buf[4])<<8 | uint32(buf[3])) & 0x3fff)
		height := int((uint32(buf[6])<<8 | uint32(buf[5])) & 0x3fff)
		buf = buf[7:]
		dec.mbW = (width + 15) >> 4
		dec.mbH = (height + 15) >> 4
		io.width = width
		io.height = height
		io.cropTop, io.cropLeft = 0, 0
		io.cropRight, io.cropBottom = width, height
		io.mbW, io.mbH = width, height
		dec.segments = [3]uint8{255, 255, 255}
		dec.segmentHdr = vp8SegmentHeader{absoluteDelta: true}
	}
	if uint64(frm.partitionLength) > uint64(len(buf)) {
		return dec.setError(statusNotEnoughData)
	}
	dec.br.init(buf[:frm.partitionLength])
	buf = buf[frm.partitionLength:]
	if frm.keyFrame {
		dec.br.getValue(1) // colorspace
		dec.br.getValue(1) // clamp type
	}
	if !dec.parseSegmentHeader() {
		return dec.setError(statusBitstreamError)
	}
	if !dec.parseFilterHeader() {
		return dec.setError(statusBitstreamError)
	}
	if st := dec.parsePartitions(buf); st != statusOK {
		return dec.setError(st)
	}
	dec.parseQuant()
	if !frm.keyFrame {
		return dec.setError(statusUnsupportedFeature)
	}
	dec.br.getValue(1)
	dec.parseProba()
	dec.ready = true
	return true
}

func clipQ(v, m int) int {
	if v < 0 {
		return 0
	}
	if v > m {
		return m
	}
	return v
}

// parseQuant is VP8ParseQuant.
func (dec *vp8Decoder) parseQuant() {
	br := &dec.br
	baseQ0 := int(br.getValue(7))
	opt := func() int {
		if br.getValue(1) != 0 {
			return br.getSignedValue(4)
		}
		return 0
	}
	dqy1DC := opt()
	dqy2DC := opt()
	dqy2AC := opt()
	dquvDC := opt()
	dquvAC := opt()
	hdr := &dec.segmentHdr
	for i := 0; i < 4; i++ {
		var q int
		if hdr.useSegment {
			q = int(hdr.quantizer[i])
			if !hdr.absoluteDelta {
				q += baseQ0
			}
		} else {
			if i > 0 {
				dec.dqm[i] = dec.dqm[0]
				continue
			}
			q = baseQ0
		}
		m := &dec.dqm[i]
		m.y1[0] = dcTable[clipQ(q+dqy1DC, 127)]
		m.y1[1] = acTable[clipQ(q, 127)]
		m.y2[0] = dcTable[clipQ(q+dqy2DC, 127)] * 2
		m.y2[1] = (acTable[clipQ(q+dqy2AC, 127)] * 101581) >> 16
		if m.y2[1] < 8 {
			m.y2[1] = 8
		}
		m.uv[0] = dcTable[clipQ(q+dquvDC, 117)]
		m.uv[1] = acTable[clipQ(q+dquvAC, 127)]
		m.uvQuant = q + dquvAC
	}
}

var bandsOf = [17]int{0, 1, 2, 3, 6, 4, 5, 6, 6, 6, 6, 6, 6, 6, 6, 7, 0}

// parseProba is VP8ParseProba.
func (dec *vp8Decoder) parseProba() {
	br := &dec.br
	for i := 0; i < len(dec.bands); i++ {
		if br.getBit(int(coeffsUpdateProba[i])) != 0 {
			dec.bands[i] = uint8(br.getValue(8))
		} else {
			dec.bands[i] = coeffsProba0[i]
		}
	}
	dec.useSkipProba = br.getValue(1) != 0
	if dec.useSkipProba {
		dec.skipP = int(br.getValue(8))
	}
}

// proba returns the 11 probabilities of type t, coefficient index n
// (through kBands) and context ctx.
func (dec *vp8Decoder) proba(t, n, ctx int) []uint8 {
	o := ((t*8+bandsOf[n])*3 + ctx) * 11
	return dec.bands[o : o+11]
}

var yModesIntra4 = [18]int8{
	-bDCPred, 1,
	-bTMPred, 2,
	-bVEPred, 3,
	4, 6,
	-bHEPred, 5,
	-bRDPred, -bVRPred,
	-bLDPred, 7,
	-bVLPred, 8,
	-bHDPred, -bHUPred,
}

func (dec *vp8Decoder) parseIntraMode(mbX int) {
	br := &dec.br
	top := dec.intraT[4*mbX : 4*mbX+4]
	left := dec.intraL[:]
	block := &dec.mbData[mbX]
	if dec.segmentHdr.updateMap {
		if br.getBit(int(dec.segments[0])) == 0 {
			block.segment = uint8(br.getBit(int(dec.segments[1])))
		} else {
			block.segment = uint8(br.getBit(int(dec.segments[2])) + 2)
		}
	} else {
		block.segment = 0
	}
	if dec.useSkipProba {
		block.skip = br.getBit(dec.skipP) != 0
	}
	block.isI4x4 = br.getBit(145) == 0
	if !block.isI4x4 {
		var ymode uint8
		if br.getBit(156) != 0 {
			if br.getBit(128) != 0 {
				ymode = tmPred
			} else {
				ymode = hPred
			}
		} else {
			if br.getBit(163) != 0 {
				ymode = vPred
			} else {
				ymode = dcPred
			}
		}
		block.imodes[0] = ymode
		for i := 0; i < 4; i++ {
			top[i] = ymode
			left[i] = ymode
		}
	} else {
		modes := 0
		for y := 0; y < 4; y++ {
			ymode := int(left[y])
			for x := 0; x < 4; x++ {
				o := (int(top[x])*10 + ymode) * 9
				prob := bModesProba[o : o+9]
				i := int(yModesIntra4[br.getBit(int(prob[0]))])
				for i > 0 {
					i = int(yModesIntra4[2*i+br.getBit(int(prob[i]))])
				}
				ymode = -i
				top[x] = uint8(ymode)
			}
			copy(block.imodes[modes:modes+4], top)
			modes += 4
			left[y] = uint8(ymode)
		}
	}
	switch {
	case br.getBit(142) == 0:
		block.uvmode = dcPred
	case br.getBit(114) == 0:
		block.uvmode = vPred
	case br.getBit(183) != 0:
		block.uvmode = tmPred
	default:
		block.uvmode = hPred
	}
}

func (dec *vp8Decoder) parseIntraModeRow() bool {
	for mbX := 0; mbX < dec.mbW; mbX++ {
		dec.parseIntraMode(mbX)
	}
	return !dec.br.eof
}

var cat3 = []uint8{173, 148, 140, 0}
var cat4 = []uint8{176, 155, 140, 135, 0}
var cat5 = []uint8{180, 157, 141, 134, 130, 0}
var cat6 = []uint8{254, 254, 243, 230, 196, 177, 153, 140, 133, 130, 129, 0}
var cat3456 = [4][]uint8{cat3, cat4, cat5, cat6}
var zigzag = [16]int{0, 1, 4, 8, 5, 2, 3, 6, 9, 12, 13, 10, 7, 11, 14, 15}

func getLargeValue(br *vp8BitReader, p []uint8) int {
	var v int
	if br.getBit(int(p[3])) == 0 {
		if br.getBit(int(p[4])) == 0 {
			v = 2
		} else {
			v = 3 + br.getBit(int(p[5]))
		}
	} else {
		if br.getBit(int(p[6])) == 0 {
			if br.getBit(int(p[7])) == 0 {
				v = 5 + br.getBit(159)
			} else {
				v = 7 + 2*br.getBit(165)
				v += br.getBit(145)
			}
		} else {
			bit1 := br.getBit(int(p[8]))
			bit0 := br.getBit(int(p[9+bit1]))
			cat := 2*bit1 + bit0
			v = 0
			for _, t := range cat3456[cat] {
				if t == 0 {
					break
				}
				v += v + br.getBit(int(t))
			}
			v += 3 + (8 << uint(cat))
		}
	}
	return v
}

// getCoeffs is GetCoeffsFast; typ selects bands_ptr[typ].
func (dec *vp8Decoder) getCoeffs(br *vp8BitReader, typ, ctx int, dq [2]int, n int, out []int16) int {
	p := dec.proba(typ, n, ctx)
	for ; n < 16; n++ {
		if br.getBit(int(p[0])) == 0 {
			return n
		}
		for br.getBit(int(p[1])) == 0 {
			n++
			p = dec.proba(typ, n, 0)
			if n == 16 {
				return 16
			}
		}
		var v int
		if br.getBit(int(p[2])) == 0 {
			v = 1
			p = dec.proba(typ, n+1, 1)
		} else {
			v = getLargeValue(br, p)
			p = dec.proba(typ, n+1, 2)
		}
		q := dq[0]
		if n > 0 {
			q = dq[1]
		}
		out[zigzag[n]] = int16(br.getSigned(v) * q)
	}
	return 16
}

func nzCodeBits(nzCoeffs uint32, nz int, dcNz bool) uint32 {
	nzCoeffs <<= 2
	switch {
	case nz > 3:
		nzCoeffs |= 3
	case nz > 1:
		nzCoeffs |= 2
	case dcNz:
		nzCoeffs |= 1
	}
	return nzCoeffs
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (dec *vp8Decoder) parseResiduals(mbX int, tokenBr *vp8BitReader) bool {
	mb := &dec.mbInfo[1+mbX]
	leftMb := &dec.mbInfo[0]
	block := &dec.mbData[mbX]
	q := &dec.dqm[block.segment]
	dst := block.coeffs[:]
	for i := range dst {
		dst[i] = 0
	}
	var first, acType int
	if !block.isI4x4 {
		var dc [16]int16
		ctx := int(mb.nzDC) + int(leftMb.nzDC)
		nz := dec.getCoeffs(tokenBr, 1, ctx, q.y2, 0, dc[:])
		v := uint8(b2i(nz > 0))
		mb.nzDC = v
		leftMb.nzDC = v
		if nz > 1 {
			transformWHT(dc[:], dst)
		} else {
			dc0 := int16((int(dc[0]) + 3) >> 3)
			for i := 0; i < 16*16; i += 16 {
				dst[i] = dc0
			}
		}
		first = 1
		acType = 0
	} else {
		first = 0
		acType = 3
	}
	tnz := mb.nz & 0x0f
	lnz := leftMb.nz & 0x0f
	var nonZeroY, nonZeroUV uint32
	off := 0
	for y := 0; y < 4; y++ {
		l := int(lnz & 1)
		nzCoeffs := uint32(0)
		for x := 0; x < 4; x++ {
			ctx := l + int(tnz&1)
			nz := dec.getCoeffs(tokenBr, acType, ctx, q.y1, first, dst[off:])
			l = b2i(nz > first)
			tnz = (tnz >> 1) | uint8(l<<7)
			nzCoeffs = nzCodeBits(nzCoeffs, nz, dst[off] != 0)
			off += 16
		}
		tnz >>= 4
		lnz = (lnz >> 1) | uint8(l<<7)
		nonZeroY = (nonZeroY << 8) | nzCoeffs
	}
	outTNz := uint32(tnz)
	outLNz := uint32(lnz >> 4)
	for ch := 0; ch < 4; ch += 2 {
		nzCoeffs := uint32(0)
		tnz = mb.nz >> uint(4+ch)
		lnz = leftMb.nz >> uint(4+ch)
		for y := 0; y < 2; y++ {
			l := int(lnz & 1)
			for x := 0; x < 2; x++ {
				ctx := l + int(tnz&1)
				nz := dec.getCoeffs(tokenBr, 2, ctx, q.uv, 0, dst[off:])
				l = b2i(nz > 0)
				tnz = (tnz >> 1) | uint8(l<<3)
				nzCoeffs = nzCodeBits(nzCoeffs, nz, dst[off] != 0)
				off += 16
			}
			tnz >>= 2
			lnz = (lnz >> 1) | uint8(l<<5)
		}
		nonZeroUV |= nzCoeffs << uint(4*ch)
		outTNz |= uint32(tnz<<4) << uint(ch)
		outLNz |= uint32(lnz&0xf0) << uint(ch)
	}
	mb.nz = uint8(outTNz)
	leftMb.nz = uint8(outLNz)
	block.nonZeroY = nonZeroY
	block.nonZeroUV = nonZeroUV
	return nonZeroY|nonZeroUV == 0
}

// decodeMB is VP8DecodeMB.
func (dec *vp8Decoder) decodeMB(tokenBr *vp8BitReader) bool {
	left := &dec.mbInfo[0]
	mb := &dec.mbInfo[1+dec.mbX]
	block := &dec.mbData[dec.mbX]
	skip := dec.useSkipProba && block.skip
	if !skip {
		skip = dec.parseResiduals(dec.mbX, tokenBr)
	} else {
		left.nz, mb.nz = 0, 0
		if !block.isI4x4 {
			left.nzDC, mb.nzDC = 0, 0
		}
		block.nonZeroY = 0
		block.nonZeroUV = 0
	}
	if dec.filterType > 0 {
		finfo := &dec.fInfo[dec.mbX]
		*finfo = dec.fstrengths[block.segment][b2i(block.isI4x4)]
		finfo.fInner |= uint8(b2i(!skip))
	}
	return !tokenBr.eof
}

func (dec *vp8Decoder) initScanline() {
	dec.mbInfo[0].nz = 0
	dec.mbInfo[0].nzDC = 0
	dec.intraL = [4]uint8{bDCPred, bDCPred, bDCPred, bDCPred}
	dec.mbX = 0
}

// parseFrame is ParseFrame.
func (dec *vp8Decoder) parseFrame(io *vp8Io) bool {
	for dec.mbY = 0; dec.mbY < dec.brMbY; dec.mbY++ {
		tokenBr := &dec.parts[uint32(dec.mbY)&dec.numPartsMinus1]
		if !dec.parseIntraModeRow() {
			return dec.setError(statusNotEnoughData)
		}
		for ; dec.mbX < dec.mbW; dec.mbX++ {
			if !dec.decodeMB(tokenBr) {
				return dec.setError(statusNotEnoughData)
			}
		}
		dec.initScanline()
		if !dec.processRow(io) {
			return dec.setError(statusUserAbort)
		}
	}
	return true
}

// decode is VP8Decode after VP8GetHeaders succeeded.
func (dec *vp8Decoder) decode(io *vp8Io) bool {
	if !dec.ready {
		if !dec.getHeaders(io) {
			return false
		}
	}
	ok := dec.enterCritical(io) == statusOK
	if ok {
		ok = dec.initFrame(io)
		if ok {
			ok = dec.parseFrame(io)
		}
		// VP8ExitCritical: the teardown frees tmp buffers only.
	}
	if !ok {
		return false
	}
	dec.ready = false
	return true
}
