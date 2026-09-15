// Package zlib131 is a pure Go port of the deflate side of zlib 1.3.1, the
// library Pillow 12.2.0 calls at run time in the parity image: its
// manylinux _imaging.so lists libz.so.1 as NEEDED (libz is on the manylinux
// allow-list), and the image resolves that to Debian trixie's zlib1g
// 1:1.3.dfsg+really1.3.1-1+b1, whose patch series is empty. The zlib-ng
// headers Pillow was compiled against do not change the code that runs.
//
// The port reproduces zlib's output byte for byte. Only the zlib wrapper
// and the deflate_slow strategy (levels 4 to 9 with the default,
// Z_FILTERED or Z_FIXED strategies) are ported; DeflateInit2 refuses the
// other configurations.
package zlib131

import "encoding/binary"

// Flush values.
const (
	NoFlush      = 0
	PartialFlush = 1
	SyncFlush    = 2
	FullFlush    = 3
	Finish       = 4
	Block        = 5
)

// Return codes.
const (
	OK          = 0
	StreamEnd   = 1
	StreamError = -2
	BufError    = -5
)

// Compression levels and strategies.
const (
	DefaultCompression = -1
	DefaultStrategy    = 0
	Filtered           = 1
	HuffmanOnly        = 2
	RLE                = 3
	Fixed              = 4
)

const (
	minMatch     = 3
	maxMatch     = 258
	minLookahead = maxMatch + minMatch + 1
	winInit      = maxMatch
	tooFar       = 4096
	maxMemLevel  = 9
	zDeflated    = 8
	presetDict   = 0x20
	windowPad    = 8

	initState   = 42
	busyState   = 113
	finishState = 666
)

type blockState int

const (
	needMore blockState = iota
	blockDone
	finishStarted
	finishDone
)

// Stream is the part of z_stream this port uses. avail_in is len(NextIn)
// and avail_out is len(NextOut); deflate consumes NextIn and fills NextOut
// from the front, reslicing both.
type Stream struct {
	NextIn   []byte
	NextOut  []byte
	TotalIn  uint64
	TotalOut uint64
	Adler    uint32

	state *state
}

type state struct {
	strm *Stream

	status     int
	pendingBuf []byte
	pendingOut uint64
	pending    uint64
	wrap       int
	lastFlush  int

	wSize      uint32
	wBits      uint32
	wMask      uint32
	window     []byte
	windowSize uint64
	prev       []uint16
	head       []uint16

	insH      uint32
	hashSize  uint32
	hashBits  uint32
	hashMask  uint32
	hashShift uint32

	blockStart     int64
	matchLength    uint32
	prevMatch      uint32
	matchAvailable bool
	strstart       uint32
	matchStart     uint32
	lookahead      uint32
	prevLength     uint32
	maxChainLength uint32
	maxLazyMatch   uint32
	level          int
	strategy       int
	goodMatch      uint32
	niceMatch      int

	dynLtree [heapSize]ctData
	dynDtree [2*dCodes + 1]ctData
	blTree   [2*blCodes + 1]ctData
	lDesc    treeDesc
	dDesc    treeDesc
	blDesc   treeDesc
	blCount  [maxBits + 1]uint16
	heap     [2*lCodes + 1]int
	heapLen  int
	heapMax  int
	depth    [2*lCodes + 1]uint8

	symBuf     []byte // three bytes per symbol, as without LIT_MEM
	litBufsize uint32
	symNext    uint32
	symEnd     uint32
	optLen     uint64
	staticLen  uint64
	matches    uint32
	insert     uint32

	biBuf     uint16
	biValid   int
	highWater uint64
}

func (s *state) maxDist() uint32 { return s.wSize - minLookahead }

// DeflateInit2 is deflateInit2 with method Z_DEFLATED and the zlib wrapper.
func (z *Stream) DeflateInit2(level, windowBits, memLevel, strategy int) int {
	const wrap = 1
	if level == DefaultCompression {
		level = 6
	}
	if windowBits < 0 || windowBits > 15 {
		// raw deflate and gzip are not ported
		return StreamError
	}
	if memLevel < 1 || memLevel > maxMemLevel || windowBits < 8 ||
		level < 0 || level > 9 || strategy < 0 || strategy > Fixed {
		return StreamError
	}
	if windowBits == 8 {
		windowBits = 9
	}
	if configurationTable[level].fn != funcSlow || strategy == HuffmanOnly || strategy == RLE {
		// only deflate_slow is ported
		return StreamError
	}

	s := &state{strm: z, status: initState, wrap: wrap}
	s.wBits = uint32(windowBits)
	s.wSize = 1 << s.wBits
	s.wMask = s.wSize - 1
	s.hashBits = uint32(memLevel) + 7
	s.hashSize = 1 << s.hashBits
	s.hashMask = s.hashSize - 1
	s.hashShift = (s.hashBits + minMatch - 1) / minMatch
	s.window = make([]byte, 2*s.wSize+windowPad)
	s.prev = make([]uint16, s.wSize)
	s.head = make([]uint16, s.hashSize)
	s.highWater = 0
	s.litBufsize = 1 << (memLevel + 6)
	s.pendingBuf = make([]byte, uint64(s.litBufsize)*4)
	s.symBuf = make([]byte, uint64(s.litBufsize)*3)
	s.symEnd = (s.litBufsize - 1) * 3
	s.level = level
	s.strategy = strategy
	z.state = s

	// deflateReset: deflateResetKeep, then lm_init
	z.TotalIn, z.TotalOut = 0, 0
	s.pending = 0
	s.pendingOut = 0
	s.status = initState
	z.Adler = 1
	s.lastFlush = -2
	s.trInit()

	s.windowSize = 2 * uint64(s.wSize)
	clear(s.head)
	c := configurationTable[level]
	s.maxLazyMatch = uint32(c.maxLazy)
	s.goodMatch = uint32(c.goodLength)
	s.niceMatch = int(c.niceLength)
	s.maxChainLength = uint32(c.maxChain)
	s.strstart = 0
	s.blockStart = 0
	s.lookahead = 0
	s.insert = 0
	s.matchLength = minMatch - 1
	s.prevLength = minMatch - 1
	s.matchAvailable = false
	s.insH = 0
	return OK
}

func rank(flush int) int {
	r := flush * 2
	if flush > 4 {
		r -= 9
	}
	return r
}

// Deflate is deflate().
func (z *Stream) Deflate(flush int) int {
	s := z.state
	if s == nil || flush > Block || flush < 0 {
		return StreamError
	}
	if s.status == finishState && flush != Finish {
		return StreamError
	}
	if len(z.NextOut) == 0 {
		return BufError
	}
	oldFlush := s.lastFlush
	s.lastFlush = flush

	if s.pending != 0 {
		s.flushPending()
		if len(z.NextOut) == 0 {
			s.lastFlush = -1
			return OK
		}
	} else if len(z.NextIn) == 0 && rank(flush) <= rank(oldFlush) && flush != Finish {
		return BufError
	}

	if s.status == finishState && len(z.NextIn) != 0 {
		return BufError
	}

	if s.status == initState && s.wrap == 0 {
		s.status = busyState
	}
	if s.status == initState {
		header := uint32(zDeflated+((s.wBits-8)<<4)) << 8
		var levelFlags uint32
		switch {
		case s.strategy >= HuffmanOnly || s.level < 2:
			levelFlags = 0
		case s.level < 6:
			levelFlags = 1
		case s.level == 6:
			levelFlags = 2
		default:
			levelFlags = 3
		}
		header |= levelFlags << 6
		if s.strstart != 0 {
			header |= presetDict
		}
		header += 31 - (header % 31)
		s.putShortMSB(header)
		if s.strstart != 0 {
			s.putShortMSB(z.Adler >> 16)
			s.putShortMSB(z.Adler & 0xffff)
		}
		z.Adler = 1
		s.status = busyState
		s.flushPending()
		if s.pending != 0 {
			s.lastFlush = -1
			return OK
		}
	}

	if len(z.NextIn) != 0 || s.lookahead != 0 || (flush != NoFlush && s.status != finishState) {
		bstate := s.deflateSlow(flush)
		if bstate == finishStarted || bstate == finishDone {
			s.status = finishState
		}
		if bstate == needMore || bstate == finishStarted {
			if len(z.NextOut) == 0 {
				s.lastFlush = -1
			}
			return OK
		}
		if bstate == blockDone {
			if flush == PartialFlush {
				s.trAlign()
			} else if flush != Block {
				s.trStoredBlock(nil, 0, 0)
				if flush == FullFlush {
					clear(s.head)
					if s.lookahead == 0 {
						s.strstart = 0
						s.blockStart = 0
						s.insert = 0
					}
				}
			}
			s.flushPending()
			if len(z.NextOut) == 0 {
				s.lastFlush = -1
				return OK
			}
		}
	}

	if flush != Finish {
		return OK
	}
	if s.wrap <= 0 {
		return StreamEnd
	}
	s.putShortMSB(z.Adler >> 16)
	s.putShortMSB(z.Adler & 0xffff)
	s.flushPending()
	if s.wrap > 0 {
		s.wrap = -s.wrap
	}
	if s.pending != 0 {
		return OK
	}
	return StreamEnd
}

// flushBlockOnly is FLUSH_BLOCK_ONLY; callers check avail_out for FLUSH_BLOCK.
func (s *state) flushBlockOnly(last int) {
	var buf []byte
	if s.blockStart >= 0 {
		buf = s.window[s.blockStart:]
	}
	s.trFlushBlock(buf, uint64(int64(s.strstart)-s.blockStart), last)
	s.blockStart = int64(s.strstart)
	s.flushPending()
}

func (s *state) flushPending() {
	s.biFlush()
	z := s.strm
	n := s.pending
	if n > uint64(len(z.NextOut)) {
		n = uint64(len(z.NextOut))
	}
	if n == 0 {
		return
	}
	copy(z.NextOut, s.pendingBuf[s.pendingOut:s.pendingOut+n])
	z.NextOut = z.NextOut[n:]
	s.pendingOut += n
	z.TotalOut += n
	s.pending -= n
	if s.pending == 0 {
		s.pendingOut = 0
	}
}

func (s *state) readBuf(dst []byte, size uint32) uint32 {
	z := s.strm
	n := uint32(len(z.NextIn))
	if n > size {
		n = size
	}
	if n == 0 {
		return 0
	}
	copy(dst[:n], z.NextIn[:n])
	if s.wrap == 1 {
		z.Adler = adler32(z.Adler, dst[:n])
	}
	z.NextIn = z.NextIn[n:]
	z.TotalIn += uint64(n)
	return n
}

// insertString is INSERT_STRING (without FASTEST): it returns the previous
// head of the hash chain.
func (s *state) insertString(str uint32) uint32 {
	s.insH = ((s.insH << s.hashShift) ^ uint32(s.window[str+minMatch-1])) & s.hashMask
	head := s.head[s.insH]
	s.prev[str&s.wMask] = head
	s.head[s.insH] = uint16(str)
	return uint32(head)
}

func (s *state) slideHash() {
	wsize := s.wSize
	for i, m := range s.head {
		if uint32(m) >= wsize {
			s.head[i] = uint16(uint32(m) - wsize)
		} else {
			s.head[i] = 0
		}
	}
	for i, m := range s.prev {
		if uint32(m) >= wsize {
			s.prev[i] = uint16(uint32(m) - wsize)
		} else {
			s.prev[i] = 0
		}
	}
}

func (s *state) fillWindow() {
	wsize := s.wSize
	for {
		more := uint32(s.windowSize - uint64(s.lookahead) - uint64(s.strstart))
		if s.strstart >= wsize+s.maxDist() {
			// zlib copies only the bytes up to strstart + lookahead
			n := wsize - more
			copy(s.window[:n], s.window[wsize:wsize+n])
			s.matchStart -= wsize
			s.strstart -= wsize
			s.blockStart -= int64(wsize)
			if s.insert > s.strstart {
				s.insert = s.strstart
			}
			s.slideHash()
			more += wsize
		}
		if len(s.strm.NextIn) == 0 {
			break
		}
		n := s.readBuf(s.window[s.strstart+s.lookahead:], more)
		s.lookahead += n

		if s.lookahead+s.insert >= minMatch {
			str := s.strstart - s.insert
			s.insH = uint32(s.window[str])
			s.insH = ((s.insH << s.hashShift) ^ uint32(s.window[str+1])) & s.hashMask
			for s.insert != 0 {
				s.insH = ((s.insH << s.hashShift) ^ uint32(s.window[str+minMatch-1])) & s.hashMask
				s.prev[str&s.wMask] = s.head[s.insH]
				s.head[s.insH] = uint16(str)
				str++
				s.insert--
				if s.lookahead+s.insert < minMatch {
					break
				}
			}
		}
		if !(s.lookahead < minLookahead && len(s.strm.NextIn) != 0) {
			break
		}
	}

	if s.highWater < s.windowSize {
		curr := uint64(s.strstart) + uint64(s.lookahead)
		if s.highWater < curr {
			init := s.windowSize - curr
			if init > winInit {
				init = winInit
			}
			clear(s.window[curr : curr+init])
			s.highWater = curr + init
		} else if s.highWater < curr+winInit {
			init := curr + winInit - s.highWater
			if init > s.windowSize-s.highWater {
				init = s.windowSize - s.highWater
			}
			clear(s.window[s.highWater : s.highWater+init])
			s.highWater += init
		}
	}
}

// longestMatch is longest_match without UNALIGNED_OK. Like zlib it never
// compares scan[2] with match[2]: equal hashes with equal first two bytes
// imply an equal third byte. zlib's four byte checks (match[best_len],
// match[best_len-1], match[0], match[1]) are side-effect free, so testing
// them as two 16-bit words selects the same candidates.
func (s *state) longestMatch(curMatch uint32) uint32 {
	chainLength := s.maxChainLength
	window := s.window
	scan := s.strstart
	bestLen := s.prevLength
	niceMatch := s.niceMatch
	var limit uint32
	if s.strstart > s.maxDist() {
		limit = s.strstart - s.maxDist()
	}
	wmask := s.wMask
	prev := s.prev[:wmask+1]
	scanStart := binary.LittleEndian.Uint16(window[scan:])
	scanEnd := binary.LittleEndian.Uint16(window[scan+bestLen-1:])

	if s.prevLength >= s.goodMatch {
		chainLength >>= 2
	}
	if uint32(niceMatch) > s.lookahead {
		niceMatch = int(s.lookahead)
	}

	for {
		if binary.LittleEndian.Uint16(window[curMatch+bestLen-1:]) == scanEnd &&
			binary.LittleEndian.Uint16(window[curMatch:]) == scanStart {
			length := matchLength(window[scan:scan+maxMatch+1], window[curMatch:curMatch+maxMatch+1])
			if length > bestLen {
				s.matchStart = curMatch
				bestLen = length
				if int(length) >= niceMatch {
					break
				}
				scanEnd = binary.LittleEndian.Uint16(window[scan+bestLen-1:])
			}
		}
		curMatch = uint32(prev[curMatch&wmask])
		if curMatch <= limit {
			break
		}
		chainLength--
		if chainLength == 0 {
			break
		}
	}
	if bestLen <= s.lookahead {
		return bestLen
	}
	return s.lookahead
}

// matchLength is the length the do/while scan of longest_match yields when
// the first two bytes agree: the offset of the first differing byte from
// offset 3 on, or MAX_MATCH.
func matchLength(scan, match []byte) uint32 {
	p := 3
	for ; p+8 <= maxMatch; p += 8 {
		if diff := binary.LittleEndian.Uint64(scan[p:]) ^ binary.LittleEndian.Uint64(match[p:]); diff != 0 {
			for scan[p] == match[p] {
				p++
			}
			return uint32(p)
		}
	}
	for ; p < maxMatch; p++ {
		if scan[p] != match[p] {
			return uint32(p)
		}
	}
	return maxMatch
}

func (s *state) deflateSlow(flush int) blockState {
	for {
		if s.lookahead < minLookahead {
			s.fillWindow()
			if s.lookahead < minLookahead && flush == NoFlush {
				return needMore
			}
			if s.lookahead == 0 {
				break
			}
		}

		hashHead := uint32(0)
		if s.lookahead >= minMatch {
			hashHead = s.insertString(s.strstart)
		}

		s.prevLength = s.matchLength
		s.prevMatch = s.matchStart
		s.matchLength = minMatch - 1

		if hashHead != 0 && s.prevLength < s.maxLazyMatch && s.strstart-hashHead <= s.maxDist() {
			s.matchLength = s.longestMatch(hashHead)
			if s.matchLength <= 5 && (s.strategy == Filtered ||
				(s.matchLength == minMatch && s.strstart-s.matchStart > tooFar)) {
				s.matchLength = minMatch - 1
			}
		}

		if s.prevLength >= minMatch && s.matchLength <= s.prevLength {
			maxInsert := s.strstart + s.lookahead - minMatch
			bflush := s.tallyDist(s.strstart-1-s.prevMatch, s.prevLength-minMatch)
			s.lookahead -= s.prevLength - 1
			s.prevLength -= 2
			for {
				s.strstart++
				if s.strstart <= maxInsert {
					s.insertString(s.strstart)
				}
				s.prevLength--
				if s.prevLength == 0 {
					break
				}
			}
			s.matchAvailable = false
			s.matchLength = minMatch - 1
			s.strstart++
			if bflush {
				s.flushBlockOnly(0)
				if len(s.strm.NextOut) == 0 {
					return needMore
				}
			}
		} else if s.matchAvailable {
			if s.tallyLit(s.window[s.strstart-1]) {
				s.flushBlockOnly(0)
			}
			s.strstart++
			s.lookahead--
			if len(s.strm.NextOut) == 0 {
				return needMore
			}
		} else {
			s.matchAvailable = true
			s.strstart++
			s.lookahead--
		}
	}

	if s.matchAvailable {
		s.tallyLit(s.window[s.strstart-1])
		s.matchAvailable = false
	}
	if s.strstart < minMatch-1 {
		s.insert = s.strstart
	} else {
		s.insert = minMatch - 1
	}
	if flush == Finish {
		s.flushBlockOnly(1)
		if len(s.strm.NextOut) == 0 {
			return finishStarted
		}
		return finishDone
	}
	if s.symNext != 0 {
		s.flushBlockOnly(0)
		if len(s.strm.NextOut) == 0 {
			return needMore
		}
	}
	return blockDone
}

// tallyLit is _tr_tally_lit.
func (s *state) tallyLit(c byte) bool {
	s.symBuf[s.symNext] = 0
	s.symBuf[s.symNext+1] = 0
	s.symBuf[s.symNext+2] = c
	s.symNext += 3
	s.dynLtree[c].fc++
	return s.symNext == s.symEnd
}

// tallyDist is _tr_tally_dist.
func (s *state) tallyDist(distance, length uint32) bool {
	lc := uint8(length)
	dist := uint16(distance)
	s.symBuf[s.symNext] = uint8(dist)
	s.symBuf[s.symNext+1] = uint8(dist >> 8)
	s.symBuf[s.symNext+2] = lc
	s.symNext += 3
	dist--
	s.dynLtree[int(lengthCode[lc])+literals+1].fc++
	s.dynDtree[dCode(uint32(dist))].fc++
	return s.symNext == s.symEnd
}

func (s *state) ensurePending(extra uint64) {
	if need := s.pending + extra; need > uint64(len(s.pendingBuf)) {
		grown := make([]byte, 2*need)
		copy(grown, s.pendingBuf)
		s.pendingBuf = grown
	}
}

func (s *state) putByte(c byte) {
	s.ensurePending(1)
	s.pendingBuf[s.pending] = c
	s.pending++
}

func (s *state) putShort(w uint16) {
	s.putByte(byte(w))
	s.putByte(byte(w >> 8))
}

func (s *state) putShortMSB(b uint32) {
	s.putByte(byte(b >> 8))
	s.putByte(byte(b))
}

func adler32(adler uint32, p []byte) uint32 {
	const base = 65521
	const nmax = 5552
	s1, s2 := adler&0xffff, adler>>16
	for len(p) > 0 {
		n := len(p)
		if n > nmax {
			n = nmax
		}
		for _, b := range p[:n] {
			s1 += uint32(b)
			s2 += s1
		}
		s1 %= base
		s2 %= base
		p = p[n:]
	}
	return s2<<16 | s1
}
