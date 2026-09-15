package webpdec

// Port of dec/webp_dec.c: header parsing, WebPGetFeatures and WebPDecode.

const (
	maxChunkPayload = ^uint32(0) - 8 - 1

	animationFlag = 0x02
	xmpFlag       = 0x04
	exifFlag      = 0x08
	alphaFlag     = 0x10
	iccpFlag      = 0x20
	allValidFlags = 0x3e
)

func le16(b []byte) uint32 { return uint32(b[0]) | uint32(b[1])<<8 }
func le24(b []byte) uint32 { return le16(b) | uint32(b[2])<<16 }
func le32(b []byte) uint32 { return le24(b) | uint32(b[3])<<24 }

type features struct {
	width, height int
	hasAlpha      bool
	hasAnimation  bool
	format        int
}

type headerStructure struct {
	haveAllData    bool
	riffSize       uint32
	alphaData      []byte
	compressedSize uint64
	isLossless     bool
	offset         int
}

func parseOptionalChunks(buf *[]byte, riffSize uint32, alpha *[]byte) statusCode {
	totalSize := uint32(4 + 8 + 10)
	for {
		data := *buf
		if len(data) < 8 {
			return statusNotEnoughData
		}
		chunkSize := le32(data[4:])
		if chunkSize > maxChunkPayload {
			return statusBitstreamError
		}
		disk := (8 + chunkSize + 1) &^ 1
		totalSize += disk
		if riffSize > 0 && totalSize > riffSize {
			return statusBitstreamError
		}
		tag := string(data[:4])
		if tag == "VP8 " || tag == "VP8L" {
			return statusOK
		}
		if uint64(len(data)) < uint64(disk) {
			return statusNotEnoughData
		}
		if tag == "ALPH" {
			*alpha = data[8 : 8+uint64(chunkSize)]
		}
		*buf = data[disk:]
	}
}

func parseVP8Header(buf *[]byte, haveAll bool, riffSize uint32, compressed *uint64, lossless *bool) statusCode {
	data := *buf
	if len(data) < 8 {
		return statusNotEnoughData
	}
	tag := string(data[:4])
	if tag == "VP8 " || tag == "VP8L" {
		size := le32(data[4:])
		if riffSize >= 12 && size > riffSize-12 {
			return statusBitstreamError
		}
		if haveAll && uint64(size) > uint64(len(data)-8) {
			return statusNotEnoughData
		}
		*compressed = uint64(size)
		*buf = data[8:]
		*lossless = tag == "VP8L"
		return statusOK
	}
	*lossless = vp8lCheckSignature(data)
	*compressed = uint64(len(data))
	return statusOK
}

// parseHeadersInternal is ParseHeadersInternal. feat is always written;
// headers is nil for WebPGetFeatures.
func parseHeadersInternal(data []byte, feat *features, headers *headerStructure) statusCode {
	var canvasW, canvasH, imageW, imageH int
	foundVP8X := false
	haveAll := headers != nil && headers.haveAllData
	if len(data) < 12 {
		return statusNotEnoughData
	}
	var hdrs headerStructure
	buf := data
	if string(buf[:4]) == "RIFF" {
		if string(buf[8:12]) != "WEBP" {
			return statusBitstreamError
		}
		size := le32(buf[4:])
		if size < 4+8 {
			return statusBitstreamError
		}
		if size > maxChunkPayload {
			return statusBitstreamError
		}
		if haveAll && uint64(size) > uint64(len(buf)-8) {
			return statusNotEnoughData
		}
		hdrs.riffSize = size
		buf = buf[12:]
	}
	foundRiff := hdrs.riffSize > 0
	flags := uint32(0)
	if len(buf) < 8 {
		return statusNotEnoughData
	}
	if string(buf[:4]) == "VP8X" {
		if le32(buf[4:]) != 10 {
			return statusBitstreamError
		}
		if len(buf) < 18 {
			return statusNotEnoughData
		}
		flags = le32(buf[8:])
		w := 1 + int(le24(buf[12:]))
		h := 1 + int(le24(buf[15:]))
		if uint64(w)*uint64(h) >= 1<<32 {
			return statusBitstreamError
		}
		canvasW, canvasH = w, h
		buf = buf[18:]
		foundVP8X = true
	}
	animation := flags&animationFlag != 0
	if !foundRiff && foundVP8X {
		return statusBitstreamError
	}
	feat.hasAlpha = flags&alphaFlag != 0
	feat.hasAnimation = animation
	feat.format = 0
	imageW, imageH = canvasW, canvasH

	finish := func(st statusCode) statusCode {
		if st == statusOK || (st == statusNotEnoughData && foundVP8X && headers == nil) {
			if hdrs.alphaData != nil {
				feat.hasAlpha = true
			}
			feat.width, feat.height = imageW, imageH
			return statusOK
		}
		return st
	}

	if foundVP8X && animation && headers == nil {
		return finish(statusOK)
	}
	if len(buf) < 4 {
		return finish(statusNotEnoughData)
	}
	if (foundRiff && foundVP8X) || (!foundRiff && !foundVP8X && string(buf[:4]) == "ALPH") {
		if st := parseOptionalChunks(&buf, hdrs.riffSize, &hdrs.alphaData); st != statusOK {
			return finish(st)
		}
	}
	if st := parseVP8Header(&buf, haveAll, hdrs.riffSize, &hdrs.compressedSize, &hdrs.isLossless); st != statusOK {
		return finish(st)
	}
	if hdrs.compressedSize > uint64(maxChunkPayload) {
		return statusBitstreamError
	}
	if !animation {
		if hdrs.isLossless {
			feat.format = 2
		} else {
			feat.format = 1
		}
	}
	if !hdrs.isLossless {
		if len(buf) < 10 {
			return finish(statusNotEnoughData)
		}
		w, h, ok := vp8GetInfo(buf, hdrs.compressedSize)
		if !ok {
			return statusBitstreamError
		}
		imageW, imageH = w, h
	} else {
		if len(buf) < 5 {
			return finish(statusNotEnoughData)
		}
		w, h, a, ok := vp8lGetInfo(buf)
		if !ok {
			return statusBitstreamError
		}
		imageW, imageH = w, h
		feat.hasAlpha = a
	}
	if foundVP8X && (canvasW != imageW || canvasH != imageH) {
		return statusBitstreamError
	}
	if headers != nil {
		hdrs.offset = len(data) - len(buf)
		*headers = hdrs
	}
	return finish(statusOK)
}

// getFeatures is WebPGetFeatures.
func getFeatures(data []byte) (features, statusCode) {
	var f features
	st := parseHeadersInternal(data, &f, nil)
	return f, st
}

// decodeInto is DecodeInto for an RGBA external buffer.
func decodeInto(data []byte, p *decParams) statusCode {
	headers := headerStructure{haveAllData: true}
	var feat features
	status := parseHeadersInternal(data, &feat, &headers)
	if (status == statusOK || status == statusNotEnoughData) && feat.hasAnimation {
		status = statusUnsupportedFeature
	}
	if status != statusOK {
		return status
	}
	io := &vp8Io{params: p, data: data[headers.offset:]}
	if !headers.isLossless {
		dec := &vp8Decoder{alphaData: headers.alphaData}
		if !dec.getHeaders(io) {
			return nonOK(dec.status)
		}
		status = p.allocate(io.width, io.height)
		if status == statusOK && !dec.decode(io) {
			status = nonOK(dec.status)
		}
		return status
	}
	dec := &vp8lDecoder{params: p}
	if !dec.decodeHeader(io) {
		return nonOK(dec.status)
	}
	status = p.allocate(io.width, io.height)
	if status == statusOK && !dec.decodeImage() {
		status = nonOK(dec.status)
	}
	return status
}

func nonOK(st statusCode) statusCode {
	if st == statusOK {
		return statusBitstreamError
	}
	return st
}

// webpDecode is WebPDecode.
func webpDecode(data []byte, p *decParams) statusCode {
	if _, st := getFeatures(data); st != statusOK {
		if st == statusNotEnoughData {
			return statusBitstreamError
		}
		return st
	}
	return decodeInto(data, p)
}
