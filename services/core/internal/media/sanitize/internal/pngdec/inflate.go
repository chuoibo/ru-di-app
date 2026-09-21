package pngdec

// This file ports the decoding loop of zlib-ng 2.3.3 inflate.c and
// inftrees.c (the compat zlib linked into Pillow 12.2.0), for a zlib-wrapped
// stream with windowBits 15. Only the slow path is ported: inflate_fast
// decodes the same symbols and hands back to the slow loop before it runs
// out of input or output, so where a call stops and what it has consumed
// are the same. That matters because Pillow's ZipDecode feeds the stream in
// pieces and stops at row boundaries, and a corrupt trailer is only seen
// when inflate reaches it inside a call.

const (
	zOK        = 0
	zStreamEnd = 1
	zNeedDict  = 2
	zDataError = -3
	zBufError  = -5
)

type inflateMode int

const (
	modeHead inflateMode = iota
	modeDictID
	modeDict
	modeType
	modeTypeDo
	modeStored
	modeCopyStart
	modeCopy
	modeTable
	modeLenLens
	modeCodeLens
	modeLenStart
	modeLen
	modeLenExt
	modeDist
	modeDistExt
	modeMatch
	modeLit
	modeCheck
	modeDone
	modeBad
)

type code struct {
	op   uint8
	bits uint8
	val  uint16
}

const (
	tableCodes = iota
	tableLens
	tableDists
)

const (
	enoughLens  = 1332
	enoughDists = 592
	windowSize  = 1 << 15
)

var (
	lbase = [31]uint16{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31,
		35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258, 0, 0}
	lext = [31]uint16{16, 16, 16, 16, 16, 16, 16, 16, 17, 17, 17, 17, 18, 18, 18, 18,
		19, 19, 19, 19, 20, 20, 20, 20, 21, 21, 21, 21, 16, 203, 77}
	dbase = [32]uint16{1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193,
		257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145,
		8193, 12289, 16385, 24577, 0, 0}
	dext = [32]uint16{16, 16, 16, 16, 17, 17, 18, 18, 19, 19, 20, 20, 21, 21, 22, 22,
		23, 23, 24, 24, 25, 25, 26, 26, 27, 27,
		28, 28, 29, 29, 64, 64}
	codeLengthOrder = [19]uint16{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}
)

// inflateTable is zng_inflate_table: it builds the table for lens[:n] at
// codes[*next:], advances *next by the entries used and sets *bits to the
// root size. It returns -1 for an over-subscribed or incomplete set and 1
// when the table would not fit.
func inflateTable(kind int, lens []uint16, n int, codes []code, next *int, bits *uint, work []uint16) int {
	const maxBits = 15
	var count, offs [maxBits + 1]uint16
	for sym := 0; sym < n; sym++ {
		count[lens[sym]]++
	}
	root := *bits
	maxLen := uint(maxBits)
	for ; maxLen >= 1; maxLen-- {
		if count[maxLen] != 0 {
			break
		}
	}
	if root > maxLen {
		root = maxLen
	}
	if maxLen == 0 {
		invalid := code{op: 64, bits: 1}
		codes[*next] = invalid
		codes[*next+1] = invalid
		*next += 2
		*bits = 1
		return 0
	}
	minLen := uint(1)
	for ; minLen < maxLen; minLen++ {
		if count[minLen] != 0 {
			break
		}
	}
	if root < minLen {
		root = minLen
	}
	left := 1
	for l := 1; l <= maxBits; l++ {
		left <<= 1
		left -= int(count[l])
		if left < 0 {
			return -1
		}
	}
	if left > 0 && (kind == tableCodes || maxLen != 1) {
		return -1
	}
	offs[1] = 0
	for l := 1; l < maxBits; l++ {
		offs[l+1] = offs[l] + count[l]
	}
	for sym := 0; sym < n; sym++ {
		if lens[sym] != 0 {
			work[offs[lens[sym]]] = uint16(sym)
			offs[lens[sym]]++
		}
	}
	var base, extra []uint16
	var match uint
	switch kind {
	case tableCodes:
		base, extra, match = work, work, 20
	case tableLens:
		base, extra, match = lbase[:], lext[:], 257
	default:
		base, extra, match = dbase[:], dext[:], 0
	}
	start := *next
	huff := uint(0)
	sym := 0
	length := minLen
	cur := start
	curr := root
	drop := uint(0)
	low := ^uint(0)
	used := uint(1) << root
	mask := used - 1
	if (kind == tableLens && used > enoughLens) || (kind == tableDists && used > enoughDists) {
		return 1
	}
	for {
		here := code{bits: uint8(length - drop)}
		w := uint(work[sym])
		switch {
		case w >= match:
			here.op = uint8(extra[w-match])
			here.val = base[w-match]
		case w+1 < match:
			here.val = uint16(w)
		default:
			here.op = 32 + 64
		}
		incr := uint(1) << (length - drop)
		fill := uint(1) << curr
		size := fill
		for {
			fill -= incr
			codes[cur+int((huff>>drop)+fill)] = here
			if fill == 0 {
				break
			}
		}
		incr = uint(1) << (length - 1)
		for huff&incr != 0 {
			incr >>= 1
		}
		if incr != 0 {
			huff &= incr - 1
			huff += incr
		} else {
			huff = 0
		}
		sym++
		count[length]--
		if count[length] == 0 {
			if length == maxLen {
				break
			}
			length = uint(lens[work[sym]])
		}
		if length > root && (huff&mask) != low {
			if drop == 0 {
				drop = root
			}
			cur += int(size)
			curr = length - drop
			left = 1 << curr
			for curr+drop < maxLen {
				left -= int(count[curr+drop])
				if left <= 0 {
					break
				}
				curr++
				left <<= 1
			}
			used += 1 << curr
			if (kind == tableLens && used > enoughLens) || (kind == tableDists && used > enoughDists) {
				return 1
			}
			low = huff & mask
			codes[start+int(low)] = code{op: uint8(curr), bits: uint8(root), val: uint16(cur - start)}
		}
	}
	if huff != 0 {
		codes[cur+int(huff)] = code{op: 64, bits: uint8(length - drop)}
	}
	*next += int(used)
	*bits = root
	return 0
}

var fixedLen, fixedDist []code

func init() {
	var lens [320]uint16
	var work [288]uint16
	for sym := 0; sym < 144; sym++ {
		lens[sym] = 8
	}
	for sym := 144; sym < 256; sym++ {
		lens[sym] = 9
	}
	for sym := 256; sym < 280; sym++ {
		lens[sym] = 7
	}
	for sym := 280; sym < 288; sym++ {
		lens[sym] = 8
	}
	table := make([]code, 1<<9)
	next, bits := 0, uint(9)
	if inflateTable(tableLens, lens[:], 288, table, &next, &bits, work[:]) != 0 {
		panic("pngdec: fixed literal table")
	}
	fixedLen = table
	for sym := 0; sym < 32; sym++ {
		lens[sym] = 5
	}
	table = make([]code, 1<<5)
	next, bits = 0, 5
	if inflateTable(tableDists, lens[:], 32, table, &next, &bits, work[:]) != 0 {
		panic("pngdec: fixed distance table")
	}
	fixedDist = table
}

// inflater is one z_stream in inflate mode after inflateInit (windowBits 15).
type inflater struct {
	mode     inflateMode
	last     bool
	hold     uint64
	bits     uint
	length   uint
	offset   uint
	extra    uint
	lencode  []code
	lenbits  uint
	distcode []code
	distbits uint
	ncode    uint
	nlen     uint
	ndist    uint
	have     uint
	lens     [320]uint16
	work     [288]uint16
	codes    [enoughLens + enoughDists]code
	check    uint32
	window   [windowSize]byte
	wnext    int
	whave    int
}

func newInflater() *inflater {
	return &inflater{mode: modeHead, check: 1}
}

// inflate runs one inflate(strm, Z_NO_FLUSH) call over in and out and
// reports how much of each it used and the return code.
func (s *inflater) inflate(in, out []byte) (consumed, produced int, ret int) {
	if s.mode == modeType {
		s.mode = modeTypeDo
	}
	next, have := 0, len(in)
	put, left := 0, len(out)
	hold, bits := s.hold, s.bits
	checked := 0
	ret = zOK

	// dropBits and bitsOf mirror DROPBITS and BITS.
	bitsOf := func(n uint) uint64 { return hold & (1<<n - 1) }

loop:
	for {
		switch s.mode {
		case modeHead:
			for bits < 16 {
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			if ((bitsOf(8)<<8)+(hold>>8))%31 != 0 {
				s.mode = modeBad
				continue
			}
			if bitsOf(4) != 8 {
				s.mode = modeBad
				continue
			}
			hold >>= 4
			bits -= 4
			if bitsOf(4)+8 > 15 {
				s.mode = modeBad
				continue
			}
			s.check = 1
			if hold&0x200 != 0 {
				s.mode = modeDictID
			} else {
				s.mode = modeType
			}
			hold, bits = 0, 0
		case modeDictID:
			for bits < 32 {
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			hold, bits = 0, 0
			s.mode = modeDict
		case modeDict:
			// No dictionary is ever set: RESTORE and return Z_NEED_DICT
			// without the inf_leave bookkeeping.
			s.hold, s.bits = hold, bits
			return next, put, zNeedDict
		case modeType, modeTypeDo:
			if s.last {
				hold >>= bits & 7
				bits -= bits & 7
				s.mode = modeCheck
				continue
			}
			for bits < 3 {
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			s.last = bitsOf(1) == 1
			hold >>= 1
			bits--
			switch bitsOf(2) {
			case 0:
				s.mode = modeStored
			case 1:
				s.lencode, s.lenbits = fixedLen, 9
				s.distcode, s.distbits = fixedDist, 5
				s.mode = modeLenStart
			case 2:
				s.mode = modeTable
			case 3:
				s.mode = modeBad
			}
			hold >>= 2
			bits -= 2
		case modeStored:
			hold >>= bits & 7
			bits -= bits & 7
			for bits < 32 {
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			if hold&0xffff != (hold>>16)^0xffff {
				s.mode = modeBad
				continue
			}
			s.length = uint(hold & 0xffff)
			hold, bits = 0, 0
			s.mode = modeCopy
		case modeCopyStart, modeCopy:
			s.mode = modeCopy
			if s.length != 0 {
				n := int(s.length)
				if n > have {
					n = have
				}
				if n > left {
					n = left
				}
				if n == 0 {
					break loop
				}
				copy(out[put:put+n], in[next:next+n])
				have -= n
				next += n
				left -= n
				put += n
				s.length -= uint(n)
				continue
			}
			s.mode = modeType
		case modeTable:
			for bits < 14 {
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			s.nlen = uint(bitsOf(5)) + 257
			hold >>= 5
			bits -= 5
			s.ndist = uint(bitsOf(5)) + 1
			hold >>= 5
			bits -= 5
			s.ncode = uint(bitsOf(4)) + 4
			hold >>= 4
			bits -= 4
			if s.nlen > 286 || s.ndist > 30 {
				s.mode = modeBad
				continue
			}
			s.have = 0
			s.mode = modeLenLens
		case modeLenLens:
			for s.have < s.ncode {
				for bits < 3 {
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				s.lens[codeLengthOrder[s.have]] = uint16(bitsOf(3))
				s.have++
				hold >>= 3
				bits -= 3
			}
			for s.have < 19 {
				s.lens[codeLengthOrder[s.have]] = 0
				s.have++
			}
			tnext := 0
			s.lenbits = 7
			if inflateTable(tableCodes, s.lens[:], 19, s.codes[:], &tnext, &s.lenbits, s.work[:]) != 0 {
				s.mode = modeBad
				continue
			}
			s.lencode = s.codes[:]
			s.have = 0
			s.mode = modeCodeLens
		case modeCodeLens:
			bad := false
			for s.have < s.nlen+s.ndist {
				var here code
				for {
					here = s.lencode[bitsOf(s.lenbits)]
					if uint(here.bits) <= bits {
						break
					}
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				if here.val < 16 {
					hold >>= here.bits
					bits -= uint(here.bits)
					s.lens[s.have] = here.val
					s.have++
					continue
				}
				var fill uint16
				var repeat uint
				extraBits := uint(2)
				switch here.val {
				case 17:
					extraBits = 3
				case 18:
					extraBits = 7
				}
				for bits < uint(here.bits)+extraBits {
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				hold >>= here.bits
				bits -= uint(here.bits)
				switch here.val {
				case 16:
					if s.have == 0 {
						bad = true
					} else {
						fill = s.lens[s.have-1]
						repeat = 3 + uint(bitsOf(2))
						hold >>= 2
						bits -= 2
					}
				case 17:
					repeat = 3 + uint(bitsOf(3))
					hold >>= 3
					bits -= 3
				default:
					repeat = 11 + uint(bitsOf(7))
					hold >>= 7
					bits -= 7
				}
				if bad || s.have+repeat > s.nlen+s.ndist {
					bad = true
					break
				}
				for ; repeat > 0; repeat-- {
					s.lens[s.have] = fill
					s.have++
				}
			}
			if bad {
				s.mode = modeBad
				continue
			}
			if s.lens[256] == 0 {
				s.mode = modeBad
				continue
			}
			tnext := 0
			s.lenbits = 10
			if inflateTable(tableLens, s.lens[:], int(s.nlen), s.codes[:], &tnext, &s.lenbits, s.work[:]) != 0 {
				s.mode = modeBad
				continue
			}
			s.lencode = s.codes[:]
			distStart := tnext
			s.distbits = 9
			if inflateTable(tableDists, s.lens[s.nlen:], int(s.ndist), s.codes[:], &tnext, &s.distbits, s.work[:]) != 0 {
				s.mode = modeBad
				continue
			}
			s.distcode = s.codes[distStart:]
			s.mode = modeLenStart
		case modeLenStart, modeLen:
			s.mode = modeLen
			var here code
			for {
				here = s.lencode[bitsOf(s.lenbits)]
				if uint(here.bits) <= bits {
					break
				}
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			if here.op != 0 && here.op&0xf0 == 0 {
				last := here
				for {
					here = s.lencode[uint(last.val)+uint(bitsOf(uint(last.bits)+uint(last.op))>>last.bits)]
					if uint(last.bits)+uint(here.bits) <= bits {
						break
					}
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				hold >>= last.bits
				bits -= uint(last.bits)
			}
			hold >>= here.bits
			bits -= uint(here.bits)
			s.length = uint(here.val)
			if here.op == 0 {
				s.mode = modeLit
				continue
			}
			if here.op&32 != 0 {
				s.mode = modeType
				continue
			}
			if here.op&64 != 0 {
				s.mode = modeBad
				continue
			}
			s.extra = uint(here.op & 15)
			s.mode = modeLenExt
		case modeLenExt:
			if s.extra != 0 {
				for bits < s.extra {
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				s.length += uint(bitsOf(s.extra))
				hold >>= s.extra
				bits -= s.extra
			}
			s.mode = modeDist
		case modeDist:
			var here code
			for {
				here = s.distcode[bitsOf(s.distbits)]
				if uint(here.bits) <= bits {
					break
				}
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			if here.op&0xf0 == 0 {
				last := here
				for {
					here = s.distcode[uint(last.val)+uint(bitsOf(uint(last.bits)+uint(last.op))>>last.bits)]
					if uint(last.bits)+uint(here.bits) <= bits {
						break
					}
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				hold >>= last.bits
				bits -= uint(last.bits)
			}
			hold >>= here.bits
			bits -= uint(here.bits)
			if here.op&64 != 0 {
				s.mode = modeBad
				continue
			}
			s.offset = uint(here.val)
			s.extra = uint(here.op & 15)
			s.mode = modeDistExt
		case modeDistExt:
			if s.extra != 0 {
				for bits < s.extra {
					if have == 0 {
						break loop
					}
					have--
					hold |= uint64(in[next]) << bits
					next++
					bits += 8
				}
				s.offset += uint(bitsOf(s.extra))
				hold >>= s.extra
				bits -= s.extra
			}
			s.mode = modeMatch
		case modeMatch:
			if left == 0 {
				break loop
			}
			n := put
			if int(s.offset) > n {
				dist := int(s.offset) - n
				if dist > s.whave {
					s.mode = modeBad
					continue
				}
				var from int
				if dist > s.wnext {
					dist -= s.wnext
					from = windowSize - dist
				} else {
					from = s.wnext - dist
				}
				if dist > int(s.length) {
					dist = int(s.length)
				}
				if dist > left {
					dist = left
				}
				copy(out[put:put+dist], s.window[from:from+dist])
				put += dist
				left -= dist
				s.length -= uint(dist)
			} else {
				n = int(s.length)
				if n > left {
					n = left
				}
				for i := 0; i < n; i++ {
					out[put] = out[put-int(s.offset)]
					put++
				}
				left -= n
				s.length -= uint(n)
			}
			if s.length == 0 {
				s.mode = modeLen
			}
		case modeLit:
			if left == 0 {
				break loop
			}
			out[put] = byte(s.length)
			put++
			left--
			s.mode = modeLen
		case modeCheck:
			for bits < 32 {
				if have == 0 {
					break loop
				}
				have--
				hold |= uint64(in[next]) << bits
				next++
				bits += 8
			}
			s.check = adler32Update(s.check, out[checked:put])
			checked = put
			trailer := uint32(hold)
			trailer = trailer>>24 | trailer>>8&0xff00 | trailer<<8&0xff0000 | trailer<<24
			if trailer != s.check {
				s.mode = modeBad
				continue
			}
			hold, bits = 0, 0
			s.mode = modeDone
		case modeDone:
			ret = zStreamEnd
			break loop
		case modeBad:
			ret = zDataError
			break loop
		}
	}
	s.hold, s.bits = hold, bits
	if s.mode < modeBad {
		s.check = adler32Update(s.check, out[checked:put])
		s.updateWindow(out[:put])
	}
	if next == 0 && put == 0 && ret == zOK {
		ret = zBufError
	}
	return next, put, ret
}

func (s *inflater) updateWindow(produced []byte) {
	if len(produced) >= windowSize {
		copy(s.window[:], produced[len(produced)-windowSize:])
		s.wnext = 0
		s.whave = windowSize
		return
	}
	dist := windowSize - s.wnext
	if dist > len(produced) {
		dist = len(produced)
	}
	copy(s.window[s.wnext:], produced[:dist])
	rest := len(produced) - dist
	if rest > 0 {
		copy(s.window[:], produced[dist:])
		s.wnext = rest
		s.whave = windowSize
		return
	}
	s.wnext += dist
	if s.wnext == windowSize {
		s.wnext = 0
	}
	if s.whave < windowSize {
		s.whave += dist
	}
}

func adler32Update(adler uint32, p []byte) uint32 {
	if len(p) == 0 {
		return adler
	}
	const mod = 65521
	s1, s2 := adler&0xffff, adler>>16
	for len(p) > 0 {
		n := len(p)
		if n > 5552 {
			n = 5552
		}
		for _, b := range p[:n] {
			s1 += uint32(b)
			s2 += s1
		}
		s1 %= mod
		s2 %= mod
		p = p[n:]
	}
	return s2<<16 | s1
}

// pyDecompress is zlib.decompressobj().decompress(data, limit) as
// PngImagePlugin._safe_zlib_decompress calls it: tooLarge reports a
// non-empty unconsumed_tail (the ValueError), zerr a zlib.error.
func pyDecompress(data []byte, limit int) (out []byte, tooLarge, zerr bool) {
	s := newInflater()
	buf := make([]byte, limit)
	consumed, produced, ret := s.inflate(data, buf)
	switch ret {
	case zOK, zBufError, zStreamEnd:
	default:
		return nil, false, true
	}
	if ret != zStreamEnd && produced == limit && consumed < len(data) {
		return nil, true, false
	}
	return buf[:produced], false, false
}
