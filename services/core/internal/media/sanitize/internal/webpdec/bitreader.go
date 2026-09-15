package webpdec

import (
	"encoding/binary"
	"math/bits"
)

// vp8Bits is BITS for x86-64 in utils/bit_reader_utils.h: the boolean
// decoder loads 56 bits (7 bytes) at a time.
const vp8Bits = 56

// vp8BitReader is VP8BitReader (utils/bit_reader_utils.[ch],
// bit_reader_inl_utils.h) for a 64-bit bit_t.
type vp8BitReader struct {
	value uint64
	rng   uint32
	nbits int
	buf   []byte
	pos   int
	end   int
	max   int
	eof   bool
}

// init is VP8InitBitReader.
func (br *vp8BitReader) init(buf []byte) {
	br.rng = 255 - 1
	br.value = 0
	br.nbits = -8
	br.eof = false
	br.buf = buf
	br.pos = 0
	br.end = len(buf)
	if len(buf) >= 8 {
		br.max = len(buf) - 8 + 1
	} else {
		br.max = 0
	}
	br.loadNewBytes()
}

// loadNewBytes is VP8LoadNewBytes.
func (br *vp8BitReader) loadNewBytes() {
	if br.pos < br.max {
		in := binary.BigEndian.Uint64(br.buf[br.pos:])
		br.pos += vp8Bits >> 3
		br.value = (in >> (64 - vp8Bits)) | (br.value << vp8Bits)
		br.nbits += vp8Bits
		return
	}
	br.loadFinalBytes()
}

// loadFinalBytes is VP8LoadFinalBytes.
func (br *vp8BitReader) loadFinalBytes() {
	switch {
	case br.pos < br.end:
		br.nbits += 8
		br.value = uint64(br.buf[br.pos]) | (br.value << 8)
		br.pos++
	case !br.eof:
		br.value <<= 8
		br.nbits += 8
		br.eof = true
	default:
		br.nbits = 0
	}
}

// getBit is VP8GetBit.
func (br *vp8BitReader) getBit(prob int) int {
	rng := br.rng
	if br.nbits < 0 {
		br.loadNewBytes()
	}
	pos := uint(br.nbits)
	split := (rng * uint32(prob)) >> 8
	value := uint32(br.value >> pos)
	bit := 0
	if value > split {
		rng -= split
		br.value -= uint64(split+1) << pos
		bit = 1
	} else {
		rng = split + 1
	}
	shift := 7 ^ (bits.Len32(rng) - 1)
	rng <<= uint(shift)
	br.nbits -= shift
	br.rng = rng - 1
	return bit
}

// getSigned is VP8GetSigned.
func (br *vp8BitReader) getSigned(v int) int {
	if br.nbits < 0 {
		br.loadNewBytes()
	}
	pos := uint(br.nbits)
	split := br.rng >> 1
	value := uint32(br.value >> pos)
	mask := int32(split-value) >> 31
	br.nbits--
	br.rng += uint32(mask)
	br.rng |= 1
	br.value -= uint64((split+1)&uint32(mask)) << pos
	return (v ^ int(mask)) - int(mask)
}

// getValue is VP8GetValue.
func (br *vp8BitReader) getValue(n int) uint32 {
	v := uint32(0)
	for n > 0 {
		n--
		v |= uint32(br.getBit(0x80)) << uint(n)
	}
	return v
}

// getSignedValue is VP8GetSignedValue.
func (br *vp8BitReader) getSignedValue(n int) int {
	value := int(br.getValue(n))
	if br.getValue(1) != 0 {
		return -value
	}
	return value
}

var bitMask = [25]uint32{
	0,
	0x000001, 0x000003, 0x000007, 0x00000f,
	0x00001f, 0x00003f, 0x00007f, 0x0000ff,
	0x0001ff, 0x0003ff, 0x0007ff, 0x000fff,
	0x001fff, 0x003fff, 0x007fff, 0x00ffff,
	0x01ffff, 0x03ffff, 0x07ffff, 0x0fffff,
	0x1fffff, 0x3fffff, 0x7fffff, 0xffffff,
}

// vp8lBitReader is VP8LBitReader with VP8L_USE_FAST_LOAD.
type vp8lBitReader struct {
	val    uint64
	buf    []byte
	length int
	pos    int
	bitPos int
	eos    bool
}

// init is VP8LInitBitReader.
func (br *vp8lBitReader) init(data []byte) {
	br.length = len(data)
	br.val = 0
	br.bitPos = 0
	br.eos = false
	n := len(data)
	if n > 8 {
		n = 8
	}
	for i := 0; i < n; i++ {
		br.val |= uint64(data[i]) << (8 * uint(i))
	}
	br.pos = n
	br.buf = data
}

func (br *vp8lBitReader) prefetch() uint32 {
	return uint32(br.val >> (uint(br.bitPos) & 63))
}

func (br *vp8lBitReader) isEndOfStream() bool {
	return br.eos || (br.pos == br.length && br.bitPos > 64)
}

func (br *vp8lBitReader) setEndOfStream() {
	br.eos = true
	br.bitPos = 0
}

func (br *vp8lBitReader) shiftBytes() {
	for br.bitPos >= 8 && br.pos < br.length {
		br.val >>= 8
		br.val |= uint64(br.buf[br.pos]) << 56
		br.pos++
		br.bitPos -= 8
	}
	if br.isEndOfStream() {
		br.setEndOfStream()
	}
}

// fillBitWindow is VP8LFillBitWindow.
func (br *vp8lBitReader) fillBitWindow() {
	if br.bitPos >= 32 {
		if br.pos+8 < br.length {
			br.val >>= 32
			br.bitPos -= 32
			br.val |= uint64(binary.LittleEndian.Uint32(br.buf[br.pos:])) << 32
			br.pos += 4
			return
		}
		br.shiftBytes()
	}
}

// readBits is VP8LReadBits.
func (br *vp8lBitReader) readBits(n int) uint32 {
	if !br.eos && n <= 24 {
		val := br.prefetch() & bitMask[n]
		br.bitPos += n
		br.shiftBytes()
		return val
	}
	br.setEndOfStream()
	return 0
}
