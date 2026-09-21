package jpegdec

// Ports of jdhuff.c (sequential Huffman decoding, both the slow path and
// decode_mcu_fast) and jstdhuff.c. Entropy decoders return false where
// libjpeg suspends for input; the coefficient controller then lets Pillow
// hand over the next block and retries the MCU from its saved state, as
// Pillow's next decode call does.

const (
	huffLookahead = 8
	minGetBits    = 64 - 7
	fastBufSize   = 64 * 8
)

// derivedTable is d_derived_tbl.
type derivedTable struct {
	maxcode   [18]int64
	valoffset [18]int64
	pub       *huffTable
	lookup    [1 << huffLookahead]int32
}

// bitState is bitread_working_state: a reader position and bit buffer.
type bitState struct {
	pos       int
	getBuffer uint64
	bitsLeft  int
}

// entropy is the state of the active entropy decoder (jdhuff or jdphuff).
type entropy struct {
	getBuffer        uint64
	bitsLeft         int
	insufficientData bool
	restartsToGo     int
	lastDCVal        [maxCompsInScan]int32
	eobrun           uint32

	dcDerived [numHuffTbls]*derivedTable
	acDerived [numHuffTbls]*derivedTable
	dcCur     [dMaxBlocksInMCU]*derivedTable
	acCur     [dMaxBlocksInMCU]*derivedTable

	derived      [numHuffTbls]*derivedTable
	acDerivedTbl *derivedTable

	decode func(blocks [][]int16) bool
}

// makeDerivedTable is jpeg_make_d_derived_tbl.
func (d *decoder) makeDerivedTable(isDC bool, tblno int, slot **derivedTable) {
	if tblno < 0 || tblno >= numHuffTbls {
		errexit("JERR_NO_HUFF_TABLE")
	}
	var htbl *huffTable
	if isDC {
		htbl = d.dcHuffTbls[tblno]
	} else {
		htbl = d.acHuffTbls[tblno]
	}
	if htbl == nil {
		errexit("JERR_NO_HUFF_TABLE")
	}
	if *slot == nil {
		*slot = &derivedTable{}
	}
	dt := *slot
	dt.pub = htbl

	var huffsize [257]int
	var huffcode [257]uint32
	p := 0
	for l := 1; l <= 16; l++ {
		i := int(htbl.bits[l])
		if p+i > 256 {
			errexit("JERR_BAD_HUFF_TABLE")
		}
		for ; i > 0; i-- {
			huffsize[p] = l
			p++
		}
	}
	huffsize[p] = 0
	numsymbols := p

	code := uint32(0)
	si := huffsize[0]
	p = 0
	for huffsize[p] != 0 {
		for huffsize[p] == si {
			huffcode[p] = code
			p++
			code++
		}
		if int64(code) >= int64(1)<<si {
			errexit("JERR_BAD_HUFF_TABLE")
		}
		code <<= 1
		si++
	}

	p = 0
	for l := 1; l <= 16; l++ {
		if htbl.bits[l] != 0 {
			dt.valoffset[l] = int64(p) - int64(huffcode[p])
			p += int(htbl.bits[l])
			dt.maxcode[l] = int64(huffcode[p-1])
		} else {
			dt.maxcode[l] = -1
		}
	}
	dt.valoffset[17] = 0
	dt.maxcode[17] = 0xFFFFF

	for i := range dt.lookup {
		dt.lookup[i] = (huffLookahead + 1) << huffLookahead
	}
	p = 0
	for l := 1; l <= huffLookahead; l++ {
		for i := 1; i <= int(htbl.bits[l]); i++ {
			lookbits := int(huffcode[p]) << (huffLookahead - l)
			for ctr := 1 << (huffLookahead - l); ctr > 0; ctr-- {
				dt.lookup[lookbits] = int32(l<<huffLookahead) | int32(htbl.huffval[p])
				lookbits++
			}
			p++
		}
	}

	if isDC {
		limit := 15
		if d.lossless {
			limit = 16
		}
		for i := 0; i < numsymbols; i++ {
			if int(htbl.huffval[i]) > limit {
				errexit("JERR_BAD_HUFF_TABLE")
			}
		}
	}
}

// fillBitBuffer is jpeg_fill_bit_buffer. It returns false where libjpeg's
// fill_input_buffer would suspend.
func (d *decoder) fillBitBuffer(br *bitState, nbits int) bool {
	data := d.src.data
	limit := d.src.limit
	if d.unreadMarker == 0 {
		for br.bitsLeft < minGetBits {
			if br.pos >= limit {
				return false
			}
			c := int(data[br.pos])
			br.pos++
			if c == 0xFF {
				for {
					if br.pos >= limit {
						return false
					}
					c = int(data[br.pos])
					br.pos++
					if c != 0xFF {
						break
					}
				}
				if c == 0 {
					c = 0xFF
				} else {
					d.unreadMarker = c
					d.noMoreBytes(br, nbits)
					return true
				}
			}
			br.getBuffer = br.getBuffer<<8 | uint64(c)
			br.bitsLeft += 8
		}
		return true
	}
	d.noMoreBytes(br, nbits)
	return true
}

// noMoreBytes is the no_more_bytes branch: past a marker the stream reads
// as zeros, and the first shortfall marks the rest of the restart interval
// as missing.
func (d *decoder) noMoreBytes(br *bitState, nbits int) {
	if nbits > br.bitsLeft {
		d.ent.insufficientData = true
		br.getBuffer <<= uint(minGetBits - br.bitsLeft)
		br.bitsLeft = minGetBits
	}
}

func getBits(br *bitState, n int) int {
	br.bitsLeft -= n
	return int(br.getBuffer>>uint(br.bitsLeft)) & (1<<n - 1)
}

func peekBits(br *bitState, n int) int {
	return int(br.getBuffer>>uint(br.bitsLeft-n)) & (1<<n - 1)
}

// checkBitBuffer is CHECK_BIT_BUFFER.
func (d *decoder) checkBitBuffer(br *bitState, nbits int) bool {
	if br.bitsLeft < nbits {
		return d.fillBitBuffer(br, nbits)
	}
	return true
}

// huffDecode is HUFF_DECODE.
func (d *decoder) huffDecode(br *bitState, tbl *derivedTable) (int, bool) {
	if br.bitsLeft < huffLookahead {
		if !d.fillBitBuffer(br, 0) {
			return 0, false
		}
		if br.bitsLeft < huffLookahead {
			return d.huffDecodeSlow(br, tbl, 1)
		}
	}
	look := peekBits(br, huffLookahead)
	entry := tbl.lookup[look]
	nb := int(entry >> huffLookahead)
	if nb <= huffLookahead {
		br.bitsLeft -= nb
		return int(entry & (1<<huffLookahead - 1)), true
	}
	return d.huffDecodeSlow(br, tbl, nb)
}

// huffDecodeSlow is jpeg_huff_decode.
func (d *decoder) huffDecodeSlow(br *bitState, tbl *derivedTable, minBits int) (int, bool) {
	l := minBits
	if !d.checkBitBuffer(br, l) {
		return 0, false
	}
	code := int64(getBits(br, l))
	for code > tbl.maxcode[l] {
		code <<= 1
		if !d.checkBitBuffer(br, 1) {
			return 0, false
		}
		code |= int64(getBits(br, 1))
		l++
	}
	if l > 16 {
		return 0, true
	}
	return int(tbl.pub.huffval[int(code+tbl.valoffset[l])&0xFF]), true
}

// huffExtend is HUFF_EXTEND.
func huffExtend(x, s int) int {
	if x < 1<<(s-1) {
		return x + (-1 << s) + 1
	}
	return x
}

// stdHuffTables installs the standard tables into empty slots, as
// jinit_huff_decoder does for Motion-JPEG streams without DHT.
func (d *decoder) stdHuffTables() {
	add := func(slot **huffTable, bits [17]uint8, val []uint8) {
		if *slot != nil {
			return
		}
		t := &huffTable{bits: bits}
		copy(t.huffval[:], val)
		*slot = t
	}
	add(&d.dcHuffTbls[0], bitsDCLuminance, valDC)
	add(&d.acHuffTbls[0], bitsACLuminance, valACLuminance)
	add(&d.dcHuffTbls[1], bitsDCChrominance, valDC)
	add(&d.acHuffTbls[1], bitsACChrominance, valACChrominance)
}

var (
	bitsDCLuminance   = [17]uint8{0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	bitsDCChrominance = [17]uint8{0, 0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0}
	valDC             = []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	bitsACLuminance   = [17]uint8{0, 0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 0x7d}
	valACLuminance    = []uint8{
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
		0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
		0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
		0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
		0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
		0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
		0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
		0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
		0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
		0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
		0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
		0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
		0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
		0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
		0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
		0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
		0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
		0xf9, 0xfa,
	}
	bitsACChrominance = [17]uint8{0, 0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 0x77}
	valACChrominance  = []uint8{
		0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
		0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
		0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
		0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
		0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
		0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
		0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
		0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
		0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
		0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
		0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
		0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
		0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
		0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
		0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
		0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
		0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
		0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
		0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
		0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
		0xf9, 0xfa,
	}
)

// startPassHuff is start_pass_huff_decoder.
func (d *decoder) startPassHuff() {
	e := &d.ent
	for ci := 0; ci < d.compsInScan; ci++ {
		c := d.curComps[ci]
		d.makeDerivedTable(true, c.dcTblNo, &e.dcDerived[c.dcTblNo&(numHuffTbls-1)])
		d.makeDerivedTable(false, c.acTblNo, &e.acDerived[c.acTblNo&(numHuffTbls-1)])
		e.lastDCVal[ci] = 0
	}
	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		c := d.curComps[d.mcuMembership[blkn]]
		e.dcCur[blkn] = e.dcDerived[c.dcTblNo]
		e.acCur[blkn] = e.acDerived[c.acTblNo]
	}
	e.bitsLeft = 0
	e.getBuffer = 0
	e.insufficientData = false
	e.restartsToGo = d.restartInterval
	e.decode = d.decodeMCUHuff
}

// processRestartHuff is jdhuff.c process_restart.
func (d *decoder) processRestartHuff() {
	e := &d.ent
	e.bitsLeft = 0
	d.readRestartMarker()
	for ci := 0; ci < d.compsInScan; ci++ {
		e.lastDCVal[ci] = 0
	}
	e.restartsToGo = d.restartInterval
	if d.unreadMarker == 0 {
		e.insufficientData = false
	}
}

// decodeMCUHuff is jdhuff.c decode_mcu.
func (d *decoder) decodeMCUHuff(blocks [][]int16) bool {
	e := &d.ent
	usefast := true
	if d.restartInterval != 0 {
		if e.restartsToGo == 0 {
			d.processRestartHuff()
		}
		usefast = false
	}
	if d.src.available(d.src.pos) < fastBufSize*d.blocksInMCU || d.unreadMarker != 0 {
		usefast = false
	}
	if !e.insufficientData {
		if !usefast || !d.decodeMCUFast(blocks) {
			if !d.decodeMCUSlow(blocks) {
				return false
			}
		}
	}
	if d.restartInterval != 0 {
		e.restartsToGo--
	}
	return true
}

func (d *decoder) decodeMCUSlow(blocks [][]int16) bool {
	e := &d.ent
	br := bitState{pos: d.src.pos, getBuffer: e.getBuffer, bitsLeft: e.bitsLeft}
	state := e.lastDCVal
	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		block := blocks[blkn]
		dctbl := e.dcCur[blkn]
		actbl := e.acCur[blkn]
		s, ok := d.huffDecode(&br, dctbl)
		if !ok {
			return false
		}
		if s != 0 {
			if !d.checkBitBuffer(&br, s) {
				return false
			}
			r := getBits(&br, s)
			s = huffExtend(r, s)
		}
		ci := d.mcuMembership[blkn]
		state[ci] += int32(s)
		block[0] = int16(state[ci])
		for k := 1; k < 64; k++ {
			s, ok := d.huffDecode(&br, actbl)
			if !ok {
				return false
			}
			r := s >> 4
			s &= 15
			if s != 0 {
				k += r
				if !d.checkBitBuffer(&br, s) {
					return false
				}
				r = getBits(&br, s)
				s = huffExtend(r, s)
				block[naturalOrder[k]] = int16(s)
			} else {
				if r != 15 {
					break
				}
				k += 15
			}
		}
	}
	d.src.pos = br.pos
	e.getBuffer = br.getBuffer
	e.bitsLeft = br.bitsLeft
	e.lastDCVal = state
	return true
}

// decodeMCUFast is decode_mcu_fast: it reads whole bytes ahead without
// input checks, which the caller guarantees by requiring 512 bytes per
// block, and gives up (for the slow path) on meeting a marker.
func (d *decoder) decodeMCUFast(blocks [][]int16) bool {
	e := &d.ent
	data := d.src.data
	buffer := d.src.pos
	getBuffer := e.getBuffer
	bitsLeft := e.bitsLeft
	state := e.lastDCVal

	byteAt := func(i int) int {
		if i < len(data) {
			return int(data[i])
		}
		return 0
	}
	getByte := func() {
		c0 := byteAt(buffer)
		buffer++
		c1 := byteAt(buffer)
		getBuffer = getBuffer<<8 | uint64(c0)
		bitsLeft += 8
		if c0 == 0xFF {
			buffer++
			if c1 != 0 {
				d.unreadMarker = c1
				buffer -= 2
				getBuffer &^= 0xFF
			}
		}
	}
	fill := func() {
		if bitsLeft <= 16 {
			getByte()
			getByte()
			getByte()
			getByte()
			getByte()
			getByte()
		}
	}
	gb := func(n int) int {
		bitsLeft -= n
		return int(getBuffer>>uint(bitsLeft)) & (1<<n - 1)
	}
	decodeFast := func(tbl *derivedTable) int {
		fill()
		s := int(getBuffer>>uint(bitsLeft-huffLookahead)) & (1<<huffLookahead - 1)
		s = int(tbl.lookup[s])
		nb := s >> huffLookahead
		bitsLeft -= nb
		s &= 1<<huffLookahead - 1
		if nb > huffLookahead {
			s = int(getBuffer>>uint(bitsLeft)) & (1<<nb - 1)
			for int64(s) > tbl.maxcode[nb] {
				s <<= 1
				s |= gb(1)
				nb++
			}
			if nb > 16 {
				s = 0
			} else {
				s = int(tbl.pub.huffval[(s+int(tbl.valoffset[nb]))&0xFF])
			}
		}
		return s
	}

	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		block := blocks[blkn]
		dctbl := e.dcCur[blkn]
		actbl := e.acCur[blkn]
		s := decodeFast(dctbl)
		if s != 0 {
			fill()
			r := gb(s)
			s = huffExtend(r, s)
		}
		ci := d.mcuMembership[blkn]
		state[ci] += int32(s)
		block[0] = int16(state[ci])
		for k := 1; k < 64; k++ {
			s := decodeFast(actbl)
			r := s >> 4
			s &= 15
			if s != 0 {
				k += r
				fill()
				r = gb(s)
				s = huffExtend(r, s)
				block[naturalOrder[k]] = int16(s)
			} else {
				if r != 15 {
					break
				}
				k += 15
			}
		}
	}
	if d.unreadMarker != 0 {
		d.unreadMarker = 0
		return false
	}
	d.src.pos = buffer
	e.getBuffer = getBuffer
	e.bitsLeft = bitsLeft
	e.lastDCVal = state
	return true
}
