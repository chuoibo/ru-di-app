package webpdec

// Port of demux/demux.c for a complete (non-partial) buffer.

const (
	parseOK = iota
	parseNeedMoreData
	parseError
)

const (
	demuxParsingHeader = iota
	demuxParsedHeader
	demuxDone
)

type memBuffer struct {
	start, end, riffEnd, bufSize int
	buf                          []byte
}

func (m *memBuffer) dataSize() int { return m.end - m.start }

func (m *memBuffer) sizeIsInvalid(size uint64) bool {
	return size > uint64(m.riffEnd-m.start)
}

func (m *memBuffer) skip(n int)   { m.start += n }
func (m *memBuffer) rewind(n int) { m.start -= n }

func (m *memBuffer) readByte() byte {
	b := m.buf[m.start]
	m.start++
	return b
}

func (m *memBuffer) readLE16s() int {
	v := int(le16(m.buf[m.start:]))
	m.start += 2
	return v
}

func (m *memBuffer) readLE24s() int {
	v := int(le24(m.buf[m.start:]))
	m.start += 3
	return v
}

func (m *memBuffer) readLE32() uint32 {
	v := le32(m.buf[m.start:])
	m.start += 4
	return v
}

type chunkData struct {
	offset, size int
}

type frame struct {
	xOffset, yOffset  int
	width, height     int
	hasAlpha          bool
	disposeBackground bool
	noBlend           bool
	frameNum          int
	complete          bool
	img               [2]chunkData
}

type demuxer struct {
	mem          memBuffer
	state        int
	isExtFormat  bool
	featureFlags uint32
	canvasWidth  int
	canvasHeight int
	loopCount    int
	bgcolor      uint32
	numFrames    int
	frames       []*frame
	chunks       []chunkData
}

func fourcc(s string) uint32 {
	return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
}

var (
	ccALPH = fourcc("ALPH")
	ccVP8  = fourcc("VP8 ")
	ccVP8L = fourcc("VP8L")
	ccVP8X = fourcc("VP8X")
	ccANIM = fourcc("ANIM")
	ccANMF = fourcc("ANMF")
	ccICCP = fourcc("ICCP")
	ccEXIF = fourcc("EXIF")
	ccXMP  = fourcc("XMP ")
)

func (d *demuxer) storeFrame(frameNum int, minSize uint32, f *frame) int {
	mem := &d.mem
	alphaChunks, imageChunks := 0, 0
	if mem.dataSize() < 8 || uint64(mem.dataSize()) < uint64(minSize) {
		return parseNeedMoreData
	}
	status := parseOK
	done := false
	for {
		chunkStart := mem.start
		cc := mem.readLE32()
		payloadSize := mem.readLE32()
		if payloadSize > maxChunkPayload {
			return parseError
		}
		padded := uint64(payloadSize + (payloadSize & 1))
		available := padded
		if uint64(mem.dataSize()) < available {
			available = uint64(mem.dataSize())
		}
		chunkSize := 8 + int(available)
		if mem.sizeIsInvalid(padded) {
			return parseError
		}
		if padded > uint64(mem.dataSize()) {
			status = parseNeedMoreData
		}
		stop := false
		switch cc {
		case ccALPH:
			if alphaChunks == 0 {
				alphaChunks++
				f.img[1] = chunkData{chunkStart, chunkSize}
				f.hasAlpha = true
				f.frameNum = frameNum
				mem.skip(int(available))
			} else {
				stop = true
			}
		case ccVP8L, ccVP8:
			if cc == ccVP8L && alphaChunks > 0 {
				return parseError
			}
			if imageChunks == 0 {
				feat, st := getFeatures(mem.buf[chunkStart : chunkStart+chunkSize])
				if status == parseNeedMoreData && st == statusNotEnoughData {
					return parseNeedMoreData
				} else if st != statusOK {
					return parseError
				}
				imageChunks++
				f.img[0] = chunkData{chunkStart, chunkSize}
				f.width = feat.width
				f.height = feat.height
				f.hasAlpha = f.hasAlpha || feat.hasAlpha
				f.frameNum = frameNum
				f.complete = status == parseOK
				mem.skip(int(available))
			} else {
				stop = true
			}
		default:
			stop = true
		}
		if stop {
			mem.rewind(8)
			done = true
		}
		if mem.start == mem.riffEnd {
			done = true
		} else if mem.dataSize() < 8 {
			status = parseNeedMoreData
		}
		if done || status != parseOK {
			return status
		}
	}
}

func (d *demuxer) parseAnimationFrame(frameChunkSize uint32) int {
	mem := &d.mem
	isAnimation := d.featureFlags&animationFlag != 0
	anmfPayloadSize := frameChunkSize - 16
	if mem.sizeIsInvalid(16) {
		return parseError
	}
	if frameChunkSize < 16 {
		return parseError
	}
	if mem.dataSize() < 16 {
		return parseNeedMoreData
	}
	f := &frame{}
	f.xOffset = 2 * mem.readLE24s()
	f.yOffset = 2 * mem.readLE24s()
	f.width = 1 + mem.readLE24s()
	f.height = 1 + mem.readLE24s()
	mem.readLE24s() // duration
	bits := mem.readByte()
	f.disposeBackground = bits&1 != 0
	f.noBlend = bits&2 != 0
	if uint64(f.width)*uint64(f.height) >= 1<<32 {
		return parseError
	}
	startOffset := mem.start
	status := d.storeFrame(d.numFrames+1, anmfPayloadSize, f)
	if status != parseError && uint64(mem.start-startOffset) > uint64(anmfPayloadSize) {
		status = parseError
	}
	if status != parseError && isAnimation && f.frameNum > 0 {
		d.frames = append(d.frames, f)
		d.numFrames++
	}
	return status
}

func (d *demuxer) readHeader() int {
	mem := &d.mem
	if mem.dataSize() < 12+8 {
		return parseNeedMoreData
	}
	b := mem.buf[mem.start:]
	if string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return parseError
	}
	riffSize := le32(b[4:])
	if riffSize < 8 || riffSize > maxChunkPayload {
		return parseError
	}
	mem.riffEnd = int(riffSize) + 8
	if mem.bufSize > mem.riffEnd {
		mem.bufSize = mem.riffEnd
		mem.end = mem.riffEnd
	}
	mem.skip(12)
	return parseOK
}

func (d *demuxer) parseSingleImage() int {
	mem := &d.mem
	if len(d.frames) != 0 {
		return parseError
	}
	if mem.sizeIsInvalid(8) {
		return parseError
	}
	if mem.dataSize() < 8 {
		return parseNeedMoreData
	}
	f := &frame{}
	status := d.storeFrame(1, 0, f)
	if status != parseError {
		hasAlpha := d.featureFlags&alphaFlag != 0
		if !hasAlpha && f.img[1].size > 0 {
			f.img[1] = chunkData{}
			f.hasAlpha = false
		}
		if !d.isExtFormat && f.width > 0 && f.height > 0 {
			d.state = demuxParsedHeader
			d.canvasWidth = f.width
			d.canvasHeight = f.height
			if f.hasAlpha {
				d.featureFlags |= alphaFlag
			}
		}
		d.frames = append(d.frames, f)
		d.numFrames = 1
	}
	return status
}

func (d *demuxer) parseVP8XChunks() int {
	mem := &d.mem
	isAnimation := d.featureFlags&animationFlag != 0
	animChunks := 0
	status := parseOK
	for {
		storeChunk := true
		chunkStart := mem.start
		cc := mem.readLE32()
		chunkSize := mem.readLE32()
		if chunkSize > maxChunkPayload {
			return parseError
		}
		padded := chunkSize + (chunkSize & 1)
		if mem.sizeIsInvalid(uint64(padded)) {
			return parseError
		}
		skipIt := false
		switch cc {
		case ccVP8X:
			return parseError
		case ccALPH, ccVP8, ccVP8L:
			if animChunks > 0 || isAnimation {
				return parseError
			}
			mem.rewind(8)
			status = d.parseSingleImage()
		case ccANIM:
			if padded < 6 {
				return parseError
			}
			if uint64(mem.dataSize()) < uint64(padded) {
				status = parseNeedMoreData
			} else if animChunks == 0 {
				animChunks++
				d.bgcolor = mem.readLE32()
				d.loopCount = mem.readLE16s()
				mem.skip(int(padded) - 6)
			} else {
				storeChunk = false
				skipIt = true
			}
		case ccANMF:
			if animChunks == 0 {
				return parseError
			}
			status = d.parseAnimationFrame(padded)
		case ccICCP:
			storeChunk = d.featureFlags&iccpFlag != 0
			skipIt = true
		case ccEXIF:
			storeChunk = d.featureFlags&exifFlag != 0
			skipIt = true
		case ccXMP:
			storeChunk = d.featureFlags&xmpFlag != 0
			skipIt = true
		default:
			skipIt = true
		}
		if skipIt {
			if uint64(padded) <= uint64(mem.dataSize()) {
				if storeChunk {
					d.chunks = append(d.chunks, chunkData{chunkStart, 8 + int(chunkSize)})
				}
				mem.skip(int(padded))
			} else {
				status = parseNeedMoreData
			}
		}
		if mem.start == mem.riffEnd {
			break
		} else if mem.dataSize() < 8 {
			status = parseNeedMoreData
		}
		if status != parseOK {
			break
		}
	}
	return status
}

func (d *demuxer) parseVP8X() int {
	mem := &d.mem
	if mem.dataSize() < 8 {
		return parseNeedMoreData
	}
	d.isExtFormat = true
	mem.skip(4)
	vp8xSize := mem.readLE32()
	if vp8xSize > maxChunkPayload || vp8xSize < 10 {
		return parseError
	}
	vp8xSize += vp8xSize & 1
	if mem.sizeIsInvalid(uint64(vp8xSize)) {
		return parseError
	}
	if uint64(mem.dataSize()) < uint64(vp8xSize) {
		return parseNeedMoreData
	}
	d.featureFlags = uint32(mem.readByte())
	mem.skip(3)
	d.canvasWidth = 1 + mem.readLE24s()
	d.canvasHeight = 1 + mem.readLE24s()
	if uint64(d.canvasWidth)*uint64(d.canvasHeight) >= 1<<32 {
		return parseError
	}
	mem.skip(int(vp8xSize) - 10)
	d.state = demuxParsedHeader
	if mem.sizeIsInvalid(8) {
		return parseError
	}
	if mem.dataSize() < 8 {
		return parseNeedMoreData
	}
	return d.parseVP8XChunks()
}

func (d *demuxer) isValidSimpleFormat() bool {
	if d.state == demuxParsingHeader {
		return true
	}
	if d.canvasWidth <= 0 || d.canvasHeight <= 0 {
		return false
	}
	if d.state == demuxDone && len(d.frames) == 0 {
		return false
	}
	if len(d.frames) == 0 {
		return false
	}
	f := d.frames[0]
	return f.width > 0 && f.height > 0
}

func checkFrameBounds(f *frame, exact bool, cw, ch int) bool {
	if exact {
		if f.xOffset != 0 || f.yOffset != 0 {
			return false
		}
		if f.width != cw || f.height != ch {
			return false
		}
		return true
	}
	if f.xOffset < 0 || f.yOffset < 0 {
		return false
	}
	if f.width+f.xOffset > cw {
		return false
	}
	if f.height+f.yOffset > ch {
		return false
	}
	return true
}

func (d *demuxer) isValidExtendedFormat() bool {
	isAnimation := d.featureFlags&animationFlag != 0
	if d.state == demuxParsingHeader {
		return true
	}
	if d.canvasWidth <= 0 || d.canvasHeight <= 0 {
		return false
	}
	if d.loopCount < 0 {
		return false
	}
	if d.state == demuxDone && len(d.frames) == 0 {
		return false
	}
	if d.featureFlags&^allValidFlags != 0 {
		return false
	}
	i := 0
	for i < len(d.frames) {
		curSet := d.frames[i].frameNum
		for ; i < len(d.frames) && d.frames[i].frameNum == curSet; i++ {
			f := d.frames[i]
			image := f.img[0]
			alpha := f.img[1]
			if !isAnimation && f.frameNum > 1 {
				return false
			}
			if f.complete {
				if alpha.size == 0 && image.size == 0 {
					return false
				}
				if alpha.size > 0 && alpha.offset > image.offset {
					return false
				}
				if f.width <= 0 || f.height <= 0 {
					return false
				}
			} else {
				if d.state == demuxDone {
					return false
				}
				if alpha.size > 0 && image.size > 0 && alpha.offset > image.offset {
					return false
				}
				if i+1 < len(d.frames) {
					return false
				}
			}
			if f.width > 0 && f.height > 0 && !checkFrameBounds(f, !isAnimation, d.canvasWidth, d.canvasHeight) {
				return false
			}
		}
	}
	return true
}

// webpDemux is WebPDemux (allow_partial 0); nil when libwebp returns NULL.
func webpDemux(data []byte) *demuxer {
	if len(data) == 0 {
		return nil
	}
	d := &demuxer{}
	d.mem = memBuffer{buf: data, end: len(data), bufSize: len(data)}
	switch d.readHeader() {
	case parseOK:
	default:
		// A PARSE_ERROR would try CreateRawImageDemuxer, which fails on
		// every input starting with "RIFF": WebPGetFeatures rejects the
		// same RIFF sizes first.
		return nil
	}
	if d.mem.bufSize < d.mem.riffEnd {
		return nil
	}
	d.state = demuxParsingHeader
	d.loopCount = 1
	d.bgcolor = 0xffffffff
	d.canvasWidth, d.canvasHeight = -1, -1
	tag := string(d.mem.buf[d.mem.start : d.mem.start+4])
	var status int
	var valid func() bool
	switch tag {
	case "VP8 ", "VP8L":
		status = d.parseSingleImage()
		valid = d.isValidSimpleFormat
	case "VP8X":
		status = d.parseVP8X()
		valid = d.isValidExtendedFormat
	default:
		return nil
	}
	if status == parseOK {
		d.state = demuxDone
	}
	if status == parseNeedMoreData {
		status = parseError
	}
	if status != parseError && !valid() {
		status = parseError
	}
	if status == parseError {
		return nil
	}
	return d
}

func (d *demuxer) getFrame(n int) *frame {
	for _, f := range d.frames {
		if f.frameNum == n {
			return f
		}
	}
	return nil
}

// framePayload is GetFramePayload.
func (d *demuxer) framePayload(f *frame) []byte {
	image := f.img[0]
	alpha := f.img[1]
	start := image.offset
	size := image.size
	if alpha.size > 0 {
		inter := 0
		if image.offset > 0 {
			inter = image.offset - (alpha.offset + alpha.size)
		}
		start = alpha.offset
		size += alpha.size + inter
	}
	return d.mem.buf[start : start+size]
}

// chunk is WebPDemuxGetChunk(dmux, tag, 1).
func (d *demuxer) chunk(tag string) []byte {
	for _, c := range d.chunks {
		if string(d.mem.buf[c.offset:c.offset+4]) == tag {
			return d.mem.buf[c.offset+8 : c.offset+c.size]
		}
	}
	return nil
}
