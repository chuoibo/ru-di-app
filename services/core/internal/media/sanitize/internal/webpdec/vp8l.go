package webpdec

// Port of dec/vp8l_dec.c without incremental decoding or rescaling.

const (
	numLiteralCodes        = 256
	numLengthCodes         = 24
	numDistanceCodes       = 40
	numArgbCacheRows       = 16
	maxCacheBits           = 11
	huffmanPackedBits      = 6
	huffmanPackedTableSize = 1 << huffmanPackedBits
	codeToPlaneCodes       = 120
	bitsSpecialMarker      = 0x100

	predictorTransform     = 0
	crossColorTransform    = 1
	subtractGreenTransform = 2
	colorIndexingTransform = 3

	hGreen = 0
	hRed   = 1
	hBlue  = 2
	hAlpha = 3
	hDist  = 4
)

var alphabetSizes = [5]int{numLiteralCodes + numLengthCodes, numLiteralCodes, numLiteralCodes, numLiteralCodes, numDistanceCodes}
var literalMap = [5]int{0, 1, 1, 1, 0}
var codeLengthCodeOrder = [19]int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
var codeLengthExtraBits = [3]int{2, 3, 7}
var codeLengthRepeatOffsets = [3]int{3, 3, 11}

var codeToPlane = [codeToPlaneCodes]uint8{
	0x18, 0x07, 0x17, 0x19, 0x28, 0x06, 0x27, 0x29, 0x16, 0x1a,
	0x26, 0x2a, 0x38, 0x05, 0x37, 0x39, 0x15, 0x1b, 0x36, 0x3a,
	0x25, 0x2b, 0x48, 0x04, 0x47, 0x49, 0x14, 0x1c, 0x35, 0x3b,
	0x46, 0x4a, 0x24, 0x2c, 0x58, 0x45, 0x4b, 0x34, 0x3c, 0x03,
	0x57, 0x59, 0x13, 0x1d, 0x56, 0x5a, 0x23, 0x2d, 0x44, 0x4c,
	0x55, 0x5b, 0x33, 0x3d, 0x68, 0x02, 0x67, 0x69, 0x12, 0x1e,
	0x66, 0x6a, 0x22, 0x2e, 0x54, 0x5c, 0x43, 0x4d, 0x65, 0x6b,
	0x32, 0x3e, 0x78, 0x01, 0x77, 0x79, 0x53, 0x5d, 0x11, 0x1f,
	0x64, 0x6c, 0x42, 0x4e, 0x76, 0x7a, 0x21, 0x2f, 0x75, 0x7b,
	0x31, 0x3f, 0x63, 0x6d, 0x52, 0x5e, 0x00, 0x74, 0x7c, 0x41,
	0x4f, 0x10, 0x20, 0x62, 0x6e, 0x30, 0x73, 0x7d, 0x51, 0x5f,
	0x40, 0x72, 0x7e, 0x61, 0x6f, 0x50, 0x71, 0x7f, 0x60, 0x70,
}

type huffmanCode32 struct {
	bits  int
	value uint32
}

type htreeGroup struct {
	htrees           [5][]huffmanCode
	isTrivialLiteral bool
	literalArb       uint32
	isTrivialCode    bool
	usePackedTable   bool
	packedTable      [huffmanPackedTableSize]huffmanCode32
}

type vp8lTransform struct {
	typ   int
	bits  int
	xsize int
	ysize int
	data  []uint32
}

type vp8lMetadata struct {
	colorCacheSize       int
	colorCache           []uint32
	colorCacheShift      uint
	huffmanMask          int
	huffmanSubsampleBits int
	huffmanXSize         int
	huffmanImage         []uint32
	numHtreeGroups       int
	htreeGroups          []htreeGroup
}

type vp8lDecoder struct {
	status statusCode
	io     *vp8Io
	params *decParams
	alph   *alphDecoder

	pixels    []uint32
	pixels8   []byte
	argbCache int

	br vp8lBitReader

	width, height int
	lastRow       int
	lastPixel     int
	lastOutRow    int

	hdr            vp8lMetadata
	nextTransform  int
	transforms     [4]vp8lTransform
	transformsSeen uint32
}

func (dec *vp8lDecoder) setError(st statusCode) bool {
	if dec.status == statusOK || dec.status == statusSuspended {
		dec.status = st
	}
	return false
}

func subSampleSize(size, bits int) int {
	return (size + (1 << uint(bits)) - 1) >> uint(bits)
}

func vp8lCheckSignature(data []byte) bool {
	return len(data) >= 5 && data[0] == 0x2f && (data[4]>>5) == 0
}

func readImageInfo(br *vp8lBitReader) (w, h int, alpha, ok bool) {
	if br.readBits(8) != 0x2f {
		return 0, 0, false, false
	}
	w = int(br.readBits(14)) + 1
	h = int(br.readBits(14)) + 1
	alpha = br.readBits(1) != 0
	if br.readBits(3) != 0 {
		return 0, 0, false, false
	}
	return w, h, alpha, !br.eos
}

// vp8lGetInfo is VP8LGetInfo.
func vp8lGetInfo(data []byte) (w, h int, alpha, ok bool) {
	if len(data) < 5 || !vp8lCheckSignature(data) {
		return 0, 0, false, false
	}
	var br vp8lBitReader
	br.init(data)
	return readImageInfo(&br)
}

func getCopyDistance(sym int, br *vp8lBitReader) int {
	if sym < 4 {
		return sym + 1
	}
	extraBits := (sym - 2) >> 1
	offset := (2 + (sym & 1)) << uint(extraBits)
	return offset + int(br.readBits(extraBits)) + 1
}

func planeCodeToDistance(xsize, planeCode int) int {
	if planeCode > codeToPlaneCodes {
		return planeCode - codeToPlaneCodes
	}
	distCode := int(codeToPlane[planeCode-1])
	yoffset := distCode >> 4
	xoffset := 8 - (distCode & 0xf)
	dist := yoffset*xsize + xoffset
	if dist >= 1 {
		return dist
	}
	return 1
}

func readSymbol(table []huffmanCode, br *vp8lBitReader) int {
	val := br.prefetch()
	t := int(val & huffmanTableMask)
	nbits := int(table[t].bits) - huffmanTableBits
	if nbits > 0 {
		br.bitPos += huffmanTableBits
		val = br.prefetch()
		t += int(table[t].value)
		t += int(val & ((1 << uint(nbits)) - 1))
	}
	br.bitPos += int(table[t].bits)
	return int(table[t].value)
}

func readPackedSymbols(g *htreeGroup, br *vp8lBitReader, dst *uint32) int {
	val := br.prefetch() & (huffmanPackedTableSize - 1)
	code := g.packedTable[val]
	if code.bits < bitsSpecialMarker {
		br.bitPos += code.bits
		*dst = code.value
		return 0
	}
	br.bitPos += code.bits - bitsSpecialMarker
	return int(code.value)
}

func accumulateHCode(h huffmanCode, shift uint, huff *huffmanCode32) uint {
	huff.bits += int(h.bits)
	huff.value |= uint32(h.value) << shift
	return uint(h.bits)
}

func buildPackedTable(g *htreeGroup) {
	for code := 0; code < huffmanPackedTableSize; code++ {
		bits := uint(code)
		huff := &g.packedTable[code]
		h := g.htrees[hGreen][bits]
		if int(h.value) >= numLiteralCodes {
			huff.bits = int(h.bits) + bitsSpecialMarker
			huff.value = uint32(h.value)
			continue
		}
		huff.bits = 0
		huff.value = 0
		bits >>= accumulateHCode(h, 8, huff)
		bits >>= accumulateHCode(g.htrees[hRed][bits], 16, huff)
		bits >>= accumulateHCode(g.htrees[hBlue][bits], 0, huff)
		accumulateHCode(g.htrees[hAlpha][bits], 24, huff)
	}
}

func (dec *vp8lDecoder) readHuffmanCodeLengths(clcl []int, numSymbols int, codeLengths []int) bool {
	br := &dec.br
	prevCodeLen := 8
	table, size := buildHuffmanTable(lengthsTableBits, clcl, true)
	if size == 0 {
		return dec.setError(statusBitstreamError)
	}
	var maxSymbol int
	if br.readBits(1) != 0 {
		lengthNBits := 2 + 2*int(br.readBits(3))
		maxSymbol = 2 + int(br.readBits(lengthNBits))
		if maxSymbol > numSymbols {
			return dec.setError(statusBitstreamError)
		}
	} else {
		maxSymbol = numSymbols
	}
	symbol := 0
	for symbol < numSymbols {
		if maxSymbol == 0 {
			break
		}
		maxSymbol--
		br.fillBitWindow()
		p := table[br.prefetch()&lengthsTableMask]
		br.bitPos += int(p.bits)
		codeLen := int(p.value)
		if codeLen < 16 {
			codeLengths[symbol] = codeLen
			symbol++
			if codeLen != 0 {
				prevCodeLen = codeLen
			}
			continue
		}
		slot := codeLen - 16
		repeat := int(br.readBits(codeLengthExtraBits[slot])) + codeLengthRepeatOffsets[slot]
		if symbol+repeat > numSymbols {
			return dec.setError(statusBitstreamError)
		}
		length := 0
		if codeLen == 16 {
			length = prevCodeLen
		}
		for ; repeat > 0; repeat-- {
			codeLengths[symbol] = length
			symbol++
		}
	}
	return true
}

func (dec *vp8lDecoder) readHuffmanCode(alphabetSize int, codeLengths []int, build bool) ([]huffmanCode, int) {
	br := &dec.br
	simple := br.readBits(1) != 0
	for i := 0; i < alphabetSize; i++ {
		codeLengths[i] = 0
	}
	var ok bool
	if simple {
		numSymbols := int(br.readBits(1)) + 1
		n := 1
		if br.readBits(1) != 0 {
			n = 8
		}
		symbol := int(br.readBits(n))
		codeLengths[symbol] = 1
		if numSymbols == 2 {
			symbol = int(br.readBits(8))
			codeLengths[symbol] = 1
		}
		ok = true
	} else {
		var clcl [19]int
		numCodes := int(br.readBits(4)) + 4
		for i := 0; i < numCodes; i++ {
			clcl[codeLengthCodeOrder[i]] = int(br.readBits(3))
		}
		ok = dec.readHuffmanCodeLengths(clcl[:], alphabetSize, codeLengths)
	}
	ok = ok && !br.eos
	var table []huffmanCode
	size := 0
	if ok {
		table, size = buildHuffmanTable(huffmanTableBits, codeLengths[:alphabetSize], build)
	}
	if !ok || size == 0 {
		dec.setError(statusBitstreamError)
		return nil, 0
	}
	return table, size
}

func (dec *vp8lDecoder) readHuffmanCodes(xsize, ysize, colorCacheBits int, allowRecursion bool) bool {
	br := &dec.br
	hdr := &dec.hdr
	var huffmanImage []uint32
	numHtreeGroups := 1
	numHtreeGroupsMax := 1
	var mapping []int
	if allowRecursion && br.readBits(1) != 0 {
		precision := 2 + int(br.readBits(3))
		hx := subSampleSize(xsize, precision)
		hy := subSampleSize(ysize, precision)
		img, ok := dec.decodeImageStream(hx, hy, false)
		if !ok {
			return false
		}
		huffmanImage = img
		hdr.huffmanSubsampleBits = precision
		for i := 0; i < hx*hy; i++ {
			group := int((img[i] >> 8) & 0xffff)
			img[i] = uint32(group)
			if group >= numHtreeGroupsMax {
				numHtreeGroupsMax = group + 1
			}
		}
		if numHtreeGroupsMax > 1000 || numHtreeGroupsMax > xsize*ysize {
			mapping = make([]int, numHtreeGroupsMax)
			for i := range mapping {
				mapping[i] = -1
			}
			numHtreeGroups = 0
			for i := 0; i < hx*hy; i++ {
				m := &mapping[img[i]]
				if *m == -1 {
					*m = numHtreeGroups
					numHtreeGroups++
				}
				img[i] = uint32(*m)
			}
		} else {
			numHtreeGroups = numHtreeGroupsMax
		}
	}
	if br.eos {
		return false
	}
	groups, ok := dec.readHuffmanCodesHelper(colorCacheBits, numHtreeGroups, numHtreeGroupsMax, mapping)
	if !ok {
		return false
	}
	hdr.huffmanImage = huffmanImage
	hdr.numHtreeGroups = numHtreeGroups
	hdr.htreeGroups = groups
	return true
}

func (dec *vp8lDecoder) readHuffmanCodesHelper(colorCacheBits, numHtreeGroups, numHtreeGroupsMax int, mapping []int) ([]htreeGroup, bool) {
	maxAlphabetSize := alphabetSizes[0]
	if colorCacheBits > 0 {
		maxAlphabetSize += 1 << uint(colorCacheBits)
	}
	if (mapping == nil && numHtreeGroups != numHtreeGroupsMax) || numHtreeGroups > numHtreeGroupsMax {
		return nil, false
	}
	codeLengths := make([]int, maxAlphabetSize)
	groups := make([]htreeGroup, numHtreeGroups)
	for i := 0; i < numHtreeGroupsMax; i++ {
		if mapping != nil && mapping[i] == -1 {
			for j := 0; j < 5; j++ {
				as := alphabetSizes[j]
				if j == 0 && colorCacheBits > 0 {
					as += 1 << uint(colorCacheBits)
				}
				if _, size := dec.readHuffmanCode(as, codeLengths, false); size == 0 {
					return nil, false
				}
			}
			continue
		}
		idx := i
		if mapping != nil {
			idx = mapping[i]
		}
		g := &groups[idx]
		totalSize := 0
		isTrivialLiteral := true
		maxBits := 0
		for j := 0; j < 5; j++ {
			as := alphabetSizes[j]
			if j == 0 && colorCacheBits > 0 {
				as += 1 << uint(colorCacheBits)
			}
			table, size := dec.readHuffmanCode(as, codeLengths, true)
			g.htrees[j] = table
			if size == 0 {
				return nil, false
			}
			if isTrivialLiteral && literalMap[j] == 1 {
				isTrivialLiteral = table[0].bits == 0
			}
			totalSize += int(table[0].bits)
			if j <= hAlpha {
				localMax := codeLengths[0]
				for k := 1; k < as; k++ {
					if codeLengths[k] > localMax {
						localMax = codeLengths[k]
					}
				}
				maxBits += localMax
			}
		}
		g.isTrivialLiteral = isTrivialLiteral
		g.isTrivialCode = false
		if isTrivialLiteral {
			red := uint32(g.htrees[hRed][0].value)
			blue := uint32(g.htrees[hBlue][0].value)
			alpha := uint32(g.htrees[hAlpha][0].value)
			g.literalArb = alpha<<24 | red<<16 | blue
			if totalSize == 0 && int(g.htrees[hGreen][0].value) < numLiteralCodes {
				g.isTrivialCode = true
				g.literalArb |= uint32(g.htrees[hGreen][0].value) << 8
			}
		}
		g.usePackedTable = !g.isTrivialCode && maxBits < huffmanPackedBits
		if g.usePackedTable {
			buildPackedTable(g)
		}
	}
	return groups, true
}

func (dec *vp8lDecoder) groupForPos(x, y int) *htreeGroup {
	hdr := &dec.hdr
	idx := 0
	if bits := hdr.huffmanSubsampleBits; bits != 0 {
		idx = int(hdr.huffmanImage[hdr.huffmanXSize*(y>>uint(bits))+(x>>uint(bits))])
	}
	return &hdr.htreeGroups[idx]
}

func (dec *vp8lDecoder) cacheInsert(argb uint32) {
	hdr := &dec.hdr
	hdr.colorCache[(argb*0x1e35a7bd)>>hdr.colorCacheShift] = argb
}

// decodeImageData is DecodeImageData for a non-incremental decoder.
func (dec *vp8lDecoder) decodeImageData(data []uint32, width, height, lastRow int, process func(row int)) bool {
	row := dec.lastPixel / width
	col := dec.lastPixel % width
	br := &dec.br
	hdr := &dec.hdr
	src := dec.lastPixel
	lastCached := src
	srcEnd := width * height
	srcLast := width * lastRow
	lenCodeLimit := numLiteralCodes + numLengthCodes
	colorCacheLimit := lenCodeLimit + hdr.colorCacheSize
	useCache := hdr.colorCacheSize > 0
	mask := hdr.huffmanMask
	var g *htreeGroup
	if src < srcLast {
		g = dec.groupForPos(col, row)
	}
	for src < srcLast {
		if col&mask == 0 {
			g = dec.groupForPos(col, row)
		}
		advance := false
		if g.isTrivialCode {
			data[src] = g.literalArb
			advance = true
		} else {
			var code int
			br.fillBitWindow()
			if g.usePackedTable {
				code = readPackedSymbols(g, br, &data[src])
				if br.isEndOfStream() {
					break
				}
				if code == 0 {
					advance = true
				}
			} else {
				code = readSymbol(g.htrees[hGreen], br)
			}
			if !advance {
				if br.isEndOfStream() {
					break
				}
				switch {
				case code < numLiteralCodes:
					if g.isTrivialLiteral {
						data[src] = g.literalArb | uint32(code)<<8
					} else {
						red := readSymbol(g.htrees[hRed], br)
						br.fillBitWindow()
						blue := readSymbol(g.htrees[hBlue], br)
						alpha := readSymbol(g.htrees[hAlpha], br)
						if br.isEndOfStream() {
							goto done
						}
						data[src] = uint32(alpha)<<24 | uint32(red)<<16 | uint32(code)<<8 | uint32(blue)
					}
					advance = true
				case code < lenCodeLimit:
					length := getCopyDistance(code-numLiteralCodes, br)
					distSymbol := readSymbol(g.htrees[hDist], br)
					br.fillBitWindow()
					distCode := getCopyDistance(distSymbol, br)
					dist := planeCodeToDistance(width, distCode)
					if br.isEndOfStream() {
						goto done
					}
					if src < dist || srcEnd-src < length {
						return dec.setError(statusBitstreamError)
					}
					for i := 0; i < length; i++ {
						data[src+i] = data[src+i-dist]
					}
					src += length
					col += length
					for col >= width {
						col -= width
						row++
						if process != nil && row <= lastRow && row%numArgbCacheRows == 0 {
							process(row)
						}
					}
					if col&mask != 0 {
						g = dec.groupForPos(col, row)
					}
					if useCache {
						for lastCached < src {
							dec.cacheInsert(data[lastCached])
							lastCached++
						}
					}
				case code < colorCacheLimit:
					key := code - lenCodeLimit
					for lastCached < src {
						dec.cacheInsert(data[lastCached])
						lastCached++
					}
					data[src] = hdr.colorCache[key]
					advance = true
				default:
					return dec.setError(statusBitstreamError)
				}
			}
		}
		if advance {
			src++
			col++
			if col >= width {
				col = 0
				row++
				if process != nil && row <= lastRow && row%numArgbCacheRows == 0 {
					process(row)
				}
				if useCache {
					for lastCached < src {
						dec.cacheInsert(data[lastCached])
						lastCached++
					}
				}
			}
		}
	}
done:
	br.eos = br.isEndOfStream()
	if br.eos {
		return dec.setError(statusBitstreamError)
	}
	if process != nil {
		r := row
		if r > lastRow {
			r = lastRow
		}
		process(r)
	}
	dec.status = statusOK
	dec.lastPixel = src
	return true
}

// decodeAlphaData is DecodeAlphaData.
func (dec *vp8lDecoder) decodeAlphaData(data []byte, width, height, lastRow int) bool {
	ok := true
	row := dec.lastPixel / width
	col := dec.lastPixel % width
	br := &dec.br
	pos := dec.lastPixel
	end := width * height
	last := width * lastRow
	lenCodeLimit := numLiteralCodes + numLengthCodes
	mask := dec.hdr.huffmanMask
	var g *htreeGroup
	if pos < last {
		g = dec.groupForPos(col, row)
	}
	jumped := false
	for !br.eos && pos < last {
		if col&mask == 0 {
			g = dec.groupForPos(col, row)
		}
		br.fillBitWindow()
		code := readSymbol(g.htrees[hGreen], br)
		if code < numLiteralCodes {
			data[pos] = byte(code)
			pos++
			col++
			if col >= width {
				col = 0
				row++
				if row <= lastRow && row%numArgbCacheRows == 0 {
					dec.extractPalettedAlphaRows(row)
				}
			}
		} else if code < lenCodeLimit {
			length := getCopyDistance(code-numLiteralCodes, br)
			distSymbol := readSymbol(g.htrees[hDist], br)
			br.fillBitWindow()
			distCode := getCopyDistance(distSymbol, br)
			dist := planeCodeToDistance(width, distCode)
			if pos >= dist && end-pos >= length {
				for i := 0; i < length; i++ {
					data[pos+i] = data[pos+i-dist]
				}
			} else {
				ok = false
				jumped = true
				break
			}
			pos += length
			col += length
			for col >= width {
				col -= width
				row++
				if row <= lastRow && row%numArgbCacheRows == 0 {
					dec.extractPalettedAlphaRows(row)
				}
			}
			if pos < last && col&mask != 0 {
				g = dec.groupForPos(col, row)
			}
		} else {
			ok = false
			jumped = true
			break
		}
		br.eos = br.isEndOfStream()
	}
	if !jumped {
		r := row
		if r > lastRow {
			r = lastRow
		}
		dec.extractPalettedAlphaRows(r)
	}
	br.eos = br.isEndOfStream()
	if !ok || (br.eos && pos < end) {
		return dec.setError(statusBitstreamError)
	}
	dec.lastPixel = pos
	return ok
}

func expandColorMap(numColors int, t *vp8lTransform) {
	final := 1 << uint(8>>uint(t.bits))
	oldBytes := make([]byte, 4*len(t.data))
	for i, v := range t.data {
		oldBytes[4*i] = byte(v)
		oldBytes[4*i+1] = byte(v >> 8)
		oldBytes[4*i+2] = byte(v >> 16)
		oldBytes[4*i+3] = byte(v >> 24)
	}
	newBytes := make([]byte, 4*final)
	copy(newBytes[:4], oldBytes[:4])
	i := 4
	for ; i < 4*numColors; i++ {
		newBytes[i] = oldBytes[i] + newBytes[i-4]
	}
	newMap := make([]uint32, final)
	for k := range newMap {
		newMap[k] = uint32(newBytes[4*k]) | uint32(newBytes[4*k+1])<<8 | uint32(newBytes[4*k+2])<<16 | uint32(newBytes[4*k+3])<<24
	}
	t.data = newMap
}

func (dec *vp8lDecoder) readTransform(xsize *int, ysize int) bool {
	br := &dec.br
	typ := int(br.readBits(2))
	if dec.transformsSeen&(1<<uint(typ)) != 0 {
		return false
	}
	dec.transformsSeen |= 1 << uint(typ)
	t := &dec.transforms[dec.nextTransform]
	t.typ = typ
	t.xsize = *xsize
	t.ysize = ysize
	t.data = nil
	dec.nextTransform++
	ok := true
	switch typ {
	case predictorTransform, crossColorTransform:
		t.bits = 2 + int(br.readBits(3))
		t.data, ok = dec.decodeImageStream(subSampleSize(t.xsize, t.bits), subSampleSize(t.ysize, t.bits), false)
	case colorIndexingTransform:
		numColors := int(br.readBits(8)) + 1
		bits := 3
		switch {
		case numColors > 16:
			bits = 0
		case numColors > 4:
			bits = 1
		case numColors > 2:
			bits = 2
		}
		*xsize = subSampleSize(t.xsize, bits)
		t.bits = bits
		t.data, ok = dec.decodeImageStream(numColors, 1, false)
		if ok {
			expandColorMap(numColors, t)
		}
	}
	return ok
}

func (dec *vp8lDecoder) updateDecoder(w, h int) {
	hdr := &dec.hdr
	bits := hdr.huffmanSubsampleBits
	dec.width = w
	dec.height = h
	hdr.huffmanXSize = subSampleSize(w, bits)
	if bits == 0 {
		hdr.huffmanMask = -1
	} else {
		hdr.huffmanMask = (1 << uint(bits)) - 1
	}
}

// decodeImageStream is DecodeImageStream.
func (dec *vp8lDecoder) decodeImageStream(xsize, ysize int, isLevel0 bool) ([]uint32, bool) {
	ok := true
	tx, ty := xsize, ysize
	br := &dec.br
	hdr := &dec.hdr
	var data []uint32
	colorCacheBits := 0
	if isLevel0 {
		for ok && br.readBits(1) != 0 {
			ok = dec.readTransform(&tx, ty)
		}
	}
	if ok && br.readBits(1) != 0 {
		colorCacheBits = int(br.readBits(4))
		ok = colorCacheBits >= 1 && colorCacheBits <= maxCacheBits
		if !ok {
			dec.setError(statusBitstreamError)
		}
	}
	if ok {
		ok = dec.readHuffmanCodes(tx, ty, colorCacheBits, isLevel0)
		if !ok {
			dec.setError(statusBitstreamError)
		}
	} else {
		dec.setError(statusBitstreamError)
	}
	if ok {
		if colorCacheBits > 0 {
			hdr.colorCacheSize = 1 << uint(colorCacheBits)
			hdr.colorCache = make([]uint32, 1<<uint(colorCacheBits))
			hdr.colorCacheShift = uint(32 - colorCacheBits)
		} else {
			hdr.colorCacheSize = 0
		}
		dec.updateDecoder(tx, ty)
		if !isLevel0 {
			data = make([]uint32, tx*ty)
			ok = dec.decodeImageData(data, tx, ty, ty, nil)
			ok = ok && !br.eos
		}
	}
	if !ok {
		dec.hdr = vp8lMetadata{}
		return nil, false
	}
	dec.lastPixel = 0
	if !isLevel0 {
		dec.hdr = vp8lMetadata{}
	}
	return data, true
}

func (dec *vp8lDecoder) allocateInternalBuffers32b(finalWidth int) {
	numPixels := dec.width * dec.height
	cacheTop := int(uint16(finalWidth))
	dec.pixels = make([]uint32, numPixels+cacheTop+finalWidth*numArgbCacheRows)
	dec.argbCache = numPixels + cacheTop
}

func (dec *vp8lDecoder) applyInverseTransforms(startRow, numRows, rowsIn int) {
	n := dec.nextTransform
	cachePixs := dec.width * numRows
	endRow := startRow + numRows
	rowsOut := dec.argbCache
	for n > 0 {
		n--
		inverseTransform(&dec.transforms[n], startRow, endRow, dec.pixels, rowsIn, rowsOut)
		rowsIn = rowsOut
	}
	if rowsIn != rowsOut {
		copy(dec.pixels[rowsOut:rowsOut+cachePixs], dec.pixels[rowsIn:rowsIn+cachePixs])
	}
}

// processRows is ProcessRows into the RGBA buffer.
func (dec *vp8lDecoder) processRows(row int) {
	numRows := row - dec.lastRow
	if numRows > 0 {
		io := dec.io
		dec.applyInverseTransforms(dec.lastRow, numRows, dec.width*dec.lastRow)
		yStart, yEnd := dec.lastRow, row
		if yEnd > io.cropBottom {
			yEnd = io.cropBottom
		}
		rowsData := dec.argbCache
		if yStart < io.cropTop {
			rowsData += (io.cropTop - yStart) * io.width
			yStart = io.cropTop
		}
		if yStart < yEnd {
			rowsData += io.cropLeft
			io.mbY = yStart - io.cropTop
			io.mbW = io.cropRight - io.cropLeft
			io.mbH = yEnd - yStart
			p := dec.params
			out := p.rgba + dec.lastOutRow*p.stride
			for l := 0; l < io.mbH; l++ {
				src := dec.pixels[rowsData : rowsData+io.mbW]
				dst := p.out[out : out+4*io.mbW]
				for i, argb := range src {
					dst[4*i] = byte(argb >> 16)
					dst[4*i+1] = byte(argb >> 8)
					dst[4*i+2] = byte(argb)
					dst[4*i+3] = byte(argb >> 24)
				}
				rowsData += io.width
				out += p.stride
			}
			dec.lastOutRow += io.mbH
		}
	}
	dec.lastRow = row
}

func (dec *vp8lDecoder) is8bOptimizable() bool {
	hdr := &dec.hdr
	if hdr.colorCacheSize > 0 {
		return false
	}
	for i := 0; i < hdr.numHtreeGroups; i++ {
		h := &hdr.htreeGroups[i].htrees
		if h[hRed][0].bits > 0 || h[hBlue][0].bits > 0 || h[hAlpha][0].bits > 0 {
			return false
		}
	}
	return true
}

func (dec *vp8lDecoder) extractAlphaRows(lastRow int) {
	curRow := dec.lastRow
	numRows := lastRow - curRow
	in := dec.width * curRow
	a := dec.alph
	width := dec.io.width
	for numRows > 0 {
		n := numRows
		if n > numArgbCacheRows {
			n = numArgbCacheRows
		}
		dst := width * curRow
		dec.applyInverseTransforms(curRow, n, in)
		for i := 0; i < width*n; i++ {
			a.output[dst+i] = byte(dec.pixels[dec.argbCache+i] >> 8)
		}
		a.applyFilter(curRow, curRow+n, dst, width)
		numRows -= n
		in += n * dec.width
		curRow += n
	}
	dec.lastRow = lastRow
	dec.lastOutRow = lastRow
}

func (dec *vp8lDecoder) extractPalettedAlphaRows(lastRow int) {
	a := dec.alph
	topRow := dec.lastRow
	if a.filter == filterNone || a.filter == filterHorizontal {
		topRow = dec.io.cropTop
	}
	firstRow := dec.lastRow
	if firstRow < topRow {
		firstRow = topRow
	}
	if lastRow > firstRow {
		width := dec.io.width
		out := width * firstRow
		in := dec.width * firstRow
		colorIndexInverseTransformAlpha(&dec.transforms[0], firstRow, lastRow, dec.pixels8[in:], a.output[out:])
		a.applyFilter(firstRow, lastRow, out, width)
	}
	dec.lastRow = lastRow
	dec.lastOutRow = lastRow
}

// vp8lDecodeAlphaHeader is VP8LDecodeAlphaHeader.
func vp8lDecodeAlphaHeader(a *alphDecoder, data []byte) bool {
	dec := &vp8lDecoder{alph: a}
	dec.width = a.width
	dec.height = a.height
	dec.io = &a.io
	dec.br.init(data)
	if _, ok := dec.decodeImageStream(a.width, a.height, true); !ok {
		return false
	}
	if dec.nextTransform == 1 && dec.transforms[0].typ == colorIndexingTransform && dec.is8bOptimizable() {
		a.use8bDecode = true
		dec.pixels8 = make([]byte, dec.width*dec.height)
	} else {
		a.use8bDecode = false
		dec.allocateInternalBuffers32b(a.width)
	}
	a.vp8l = dec
	return true
}

// vp8lDecodeAlphaImageStream is VP8LDecodeAlphaImageStream.
func vp8lDecodeAlphaImageStream(a *alphDecoder, lastRow int) bool {
	dec := a.vp8l
	if dec.lastRow >= lastRow {
		return true
	}
	if a.use8bDecode {
		return dec.decodeAlphaData(dec.pixels8, dec.width, dec.height, lastRow)
	}
	return dec.decodeImageData(dec.pixels, dec.width, dec.height, lastRow, dec.extractAlphaRows)
}

// decodeHeader is VP8LDecodeHeader.
func (dec *vp8lDecoder) decodeHeader(io *vp8Io) bool {
	dec.io = io
	dec.status = statusOK
	dec.br.init(io.data)
	w, h, _, ok := readImageInfo(&dec.br)
	if !ok {
		return dec.setError(statusBitstreamError)
	}
	io.width, io.height = w, h
	if _, ok := dec.decodeImageStream(w, h, true); !ok {
		if dec.status == statusOK {
			dec.status = statusBitstreamError
		}
		return false
	}
	return true
}

// decodeImage is VP8LDecodeImage.
func (dec *vp8lDecoder) decodeImage() bool {
	io := dec.io
	initFromOptions(io)
	dec.allocateInternalBuffers32b(io.width)
	if !dec.decodeImageData(dec.pixels, dec.width, dec.height, io.cropBottom, dec.processRows) {
		if dec.status == statusOK {
			dec.status = statusBitstreamError
		}
		return false
	}
	dec.params.lastY = dec.lastOutRow
	return true
}
