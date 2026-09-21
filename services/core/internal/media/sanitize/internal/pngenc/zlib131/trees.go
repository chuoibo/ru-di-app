package zlib131

// A port of zlib 1.3.1 trees.c (and the tables of trees.h).

const (
	lengthCodes = 29
	literals    = 256
	lCodes      = literals + 1 + lengthCodes
	dCodes      = 30
	blCodes     = 19
	heapSize    = 2*lCodes + 1
	endBlock    = 256
	maxBits     = 15
	maxBLBits   = 7
	rep3To6     = 16
	repz3To10   = 17
	repz11To138 = 18
	storedBlock = 0
	staticTrees = 1
	dynTrees    = 2
	distCodeLen = 512
	bufSize     = 16
)

// ctData mirrors ct_data: fc is the Freq/Code union and dl the Dad/Len
// union, so writes through one name are visible through the other exactly
// as in C.
type ctData struct {
	fc uint16
	dl uint16
}

var extraLbits = [lengthCodes]int{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0}

var extraDbits = [dCodes]int{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13}

var extraBLbits = [blCodes]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 3, 7}

var blOrder = [blCodes]uint8{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}

var (
	staticLtree [lCodes + 2]ctData
	staticDtree [dCodes]ctData
	distCode    [distCodeLen]uint8
	lengthCode  [maxMatch - minMatch + 1]uint8
	baseLength  [lengthCodes]int
	baseDist    [dCodes]int
)

// init builds the tables exactly as tr_static_init does.
func init() {
	length := 0
	code := 0
	for code = 0; code < lengthCodes-1; code++ {
		baseLength[code] = length
		for n := 0; n < 1<<extraLbits[code]; n++ {
			lengthCode[length] = uint8(code)
			length++
		}
	}
	lengthCode[length-1] = uint8(code)

	dist := 0
	for code = 0; code < 16; code++ {
		baseDist[code] = dist
		for n := 0; n < 1<<extraDbits[code]; n++ {
			distCode[dist] = uint8(code)
			dist++
		}
	}
	dist >>= 7
	for ; code < dCodes; code++ {
		baseDist[code] = dist << 7
		for n := 0; n < 1<<(extraDbits[code]-7); n++ {
			distCode[256+dist] = uint8(code)
			dist++
		}
	}

	var blCount [maxBits + 1]uint16
	n := 0
	for ; n <= 143; n++ {
		staticLtree[n].dl = 8
		blCount[8]++
	}
	for ; n <= 255; n++ {
		staticLtree[n].dl = 9
		blCount[9]++
	}
	for ; n <= 279; n++ {
		staticLtree[n].dl = 7
		blCount[7]++
	}
	for ; n <= 287; n++ {
		staticLtree[n].dl = 8
		blCount[8]++
	}
	genCodes(staticLtree[:], lCodes+1, &blCount)
	for n = 0; n < dCodes; n++ {
		staticDtree[n].dl = 5
		staticDtree[n].fc = biReverse(uint32(n), 5)
	}
}

func dCode(dist uint32) uint8 {
	if dist < 256 {
		return distCode[dist]
	}
	return distCode[256+(dist>>7)]
}

type staticTreeDesc struct {
	staticTree []ctData
	extraBits  []int
	extraBase  int
	elems      int
	maxLength  int
}

var (
	staticLDesc  = staticTreeDesc{staticLtree[:], extraLbits[:], literals + 1, lCodes, maxBits}
	staticDDesc  = staticTreeDesc{staticDtree[:], extraDbits[:], 0, dCodes, maxBits}
	staticBLDesc = staticTreeDesc{nil, extraBLbits[:], 0, blCodes, maxBLBits}
)

type treeDesc struct {
	dynTree  []ctData
	maxCode  int
	statDesc *staticTreeDesc
}

func biReverse(code uint32, length int) uint16 {
	var res uint32
	for i := 0; i < length; i++ {
		res = res<<1 | code&1
		code >>= 1
	}
	return uint16(res)
}

func (s *state) trInit() {
	s.lDesc = treeDesc{dynTree: s.dynLtree[:], statDesc: &staticLDesc}
	s.dDesc = treeDesc{dynTree: s.dynDtree[:], statDesc: &staticDDesc}
	s.blDesc = treeDesc{dynTree: s.blTree[:], statDesc: &staticBLDesc}
	s.biBuf = 0
	s.biValid = 0
	s.initBlock()
}

func (s *state) initBlock() {
	for n := 0; n < lCodes; n++ {
		s.dynLtree[n].fc = 0
	}
	for n := 0; n < dCodes; n++ {
		s.dynDtree[n].fc = 0
	}
	for n := 0; n < blCodes; n++ {
		s.blTree[n].fc = 0
	}
	s.dynLtree[endBlock].fc = 1
	s.optLen = 0
	s.staticLen = 0
	s.symNext = 0
	s.matches = 0
}

func (s *state) smaller(tree []ctData, n, m int) bool {
	return tree[n].fc < tree[m].fc || (tree[n].fc == tree[m].fc && s.depth[n] <= s.depth[m])
}

func (s *state) pqdownheap(tree []ctData, k int) {
	v := s.heap[k]
	j := k << 1
	for j <= s.heapLen {
		if j < s.heapLen && s.smaller(tree, s.heap[j+1], s.heap[j]) {
			j++
		}
		if s.smaller(tree, v, s.heap[j]) {
			break
		}
		s.heap[k] = s.heap[j]
		k = j
		j <<= 1
	}
	s.heap[k] = v
}

func (s *state) buildTree(desc *treeDesc) {
	tree := desc.dynTree
	stree := desc.statDesc.staticTree
	elems := desc.statDesc.elems
	maxCode := -1

	s.heapLen = 0
	s.heapMax = heapSize
	for n := 0; n < elems; n++ {
		if tree[n].fc != 0 {
			s.heapLen++
			s.heap[s.heapLen] = n
			maxCode = n
			s.depth[n] = 0
		} else {
			tree[n].dl = 0
		}
	}
	for s.heapLen < 2 {
		node := 0
		if maxCode < 2 {
			maxCode++
			node = maxCode
		}
		s.heapLen++
		s.heap[s.heapLen] = node
		tree[node].fc = 1
		s.depth[node] = 0
		s.optLen--
		if stree != nil {
			s.staticLen -= uint64(stree[node].dl)
		}
	}
	desc.maxCode = maxCode

	for n := s.heapLen / 2; n >= 1; n-- {
		s.pqdownheap(tree, n)
	}

	node := elems
	for {
		n := s.heap[1]
		s.heap[1] = s.heap[s.heapLen]
		s.heapLen--
		s.pqdownheap(tree, 1)
		m := s.heap[1]

		s.heapMax--
		s.heap[s.heapMax] = n
		s.heapMax--
		s.heap[s.heapMax] = m

		tree[node].fc = tree[n].fc + tree[m].fc
		if s.depth[n] >= s.depth[m] {
			s.depth[node] = s.depth[n] + 1
		} else {
			s.depth[node] = s.depth[m] + 1
		}
		tree[n].dl = uint16(node)
		tree[m].dl = uint16(node)
		s.heap[1] = node
		node++
		s.pqdownheap(tree, 1)
		if s.heapLen < 2 {
			break
		}
	}
	s.heapMax--
	s.heap[s.heapMax] = s.heap[1]

	s.genBitlen(desc)
	genCodes(tree, maxCode, &s.blCount)
}

func (s *state) genBitlen(desc *treeDesc) {
	tree := desc.dynTree
	maxCode := desc.maxCode
	stree := desc.statDesc.staticTree
	extra := desc.statDesc.extraBits
	base := desc.statDesc.extraBase
	maxLength := desc.statDesc.maxLength
	overflow := 0

	for bits := 0; bits <= maxBits; bits++ {
		s.blCount[bits] = 0
	}
	tree[s.heap[s.heapMax]].dl = 0

	h := s.heapMax + 1
	for ; h < heapSize; h++ {
		n := s.heap[h]
		bits := int(tree[tree[n].dl].dl) + 1
		if bits > maxLength {
			bits = maxLength
			overflow++
		}
		tree[n].dl = uint16(bits)
		if n > maxCode {
			continue
		}
		s.blCount[bits]++
		xbits := 0
		if n >= base {
			xbits = extra[n-base]
		}
		f := uint64(tree[n].fc)
		s.optLen += f * uint64(bits+xbits)
		if stree != nil {
			s.staticLen += f * uint64(int(stree[n].dl)+xbits)
		}
	}
	if overflow == 0 {
		return
	}
	for {
		bits := maxLength - 1
		for s.blCount[bits] == 0 {
			bits--
		}
		s.blCount[bits]--
		s.blCount[bits+1] += 2
		s.blCount[maxLength]--
		overflow -= 2
		if overflow <= 0 {
			break
		}
	}
	for bits := maxLength; bits != 0; bits-- {
		n := int(s.blCount[bits])
		for n != 0 {
			h--
			m := s.heap[h]
			if m > maxCode {
				continue
			}
			if int(tree[m].dl) != bits {
				s.optLen += (uint64(bits) - uint64(tree[m].dl)) * uint64(tree[m].fc)
				tree[m].dl = uint16(bits)
			}
			n--
		}
	}
}

func genCodes(tree []ctData, maxCode int, blCount *[maxBits + 1]uint16) {
	var nextCode [maxBits + 1]uint16
	code := uint32(0)
	for bits := 1; bits <= maxBits; bits++ {
		code = (code + uint32(blCount[bits-1])) << 1
		nextCode[bits] = uint16(code)
	}
	for n := 0; n <= maxCode; n++ {
		length := int(tree[n].dl)
		if length == 0 {
			continue
		}
		tree[n].fc = biReverse(uint32(nextCode[length]), length)
		nextCode[length]++
	}
}

func (s *state) scanTree(tree []ctData, maxCode int) {
	prevlen := -1
	nextlen := int(tree[0].dl)
	count := 0
	maxCount := 7
	minCount := 4
	if nextlen == 0 {
		maxCount, minCount = 138, 3
	}
	tree[maxCode+1].dl = 0xffff
	for n := 0; n <= maxCode; n++ {
		curlen := nextlen
		nextlen = int(tree[n+1].dl)
		count++
		if count < maxCount && curlen == nextlen {
			continue
		} else if count < minCount {
			s.blTree[curlen].fc += uint16(count)
		} else if curlen != 0 {
			if curlen != prevlen {
				s.blTree[curlen].fc++
			}
			s.blTree[rep3To6].fc++
		} else if count <= 10 {
			s.blTree[repz3To10].fc++
		} else {
			s.blTree[repz11To138].fc++
		}
		count = 0
		prevlen = curlen
		if nextlen == 0 {
			maxCount, minCount = 138, 3
		} else if curlen == nextlen {
			maxCount, minCount = 6, 3
		} else {
			maxCount, minCount = 7, 4
		}
	}
}

func (s *state) sendTree(tree []ctData, maxCode int) {
	prevlen := -1
	nextlen := int(tree[0].dl)
	count := 0
	maxCount := 7
	minCount := 4
	if nextlen == 0 {
		maxCount, minCount = 138, 3
	}
	for n := 0; n <= maxCode; n++ {
		curlen := nextlen
		nextlen = int(tree[n+1].dl)
		count++
		if count < maxCount && curlen == nextlen {
			continue
		} else if count < minCount {
			for {
				s.sendCode(curlen, s.blTree[:])
				count--
				if count == 0 {
					break
				}
			}
		} else if curlen != 0 {
			if curlen != prevlen {
				s.sendCode(curlen, s.blTree[:])
				count--
			}
			s.sendCode(rep3To6, s.blTree[:])
			s.sendBits(count-3, 2)
		} else if count <= 10 {
			s.sendCode(repz3To10, s.blTree[:])
			s.sendBits(count-3, 3)
		} else {
			s.sendCode(repz11To138, s.blTree[:])
			s.sendBits(count-11, 7)
		}
		count = 0
		prevlen = curlen
		if nextlen == 0 {
			maxCount, minCount = 138, 3
		} else if curlen == nextlen {
			maxCount, minCount = 6, 3
		} else {
			maxCount, minCount = 7, 4
		}
	}
}

func (s *state) buildBLTree() int {
	s.scanTree(s.dynLtree[:], s.lDesc.maxCode)
	s.scanTree(s.dynDtree[:], s.dDesc.maxCode)
	s.buildTree(&s.blDesc)
	maxBLIndex := blCodes - 1
	for ; maxBLIndex >= 3; maxBLIndex-- {
		if s.blTree[blOrder[maxBLIndex]].dl != 0 {
			break
		}
	}
	s.optLen += 3*(uint64(maxBLIndex)+1) + 5 + 5 + 4
	return maxBLIndex
}

func (s *state) sendAllTrees(lcodes, dcodes, blcodes int) {
	s.sendBits(lcodes-257, 5)
	s.sendBits(dcodes-1, 5)
	s.sendBits(blcodes-4, 4)
	for rank := 0; rank < blcodes; rank++ {
		s.sendBits(int(s.blTree[blOrder[rank]].dl), 3)
	}
	s.sendTree(s.dynLtree[:], lcodes-1)
	s.sendTree(s.dynDtree[:], dcodes-1)
}

func (s *state) trStoredBlock(buf []byte, storedLen uint64, last int) {
	s.sendBits(storedBlock<<1+last, 3)
	s.biWindup()
	s.putShort(uint16(storedLen))
	s.putShort(^uint16(storedLen))
	if storedLen != 0 {
		s.ensurePending(storedLen)
		copy(s.pendingBuf[s.pending:], buf[:storedLen])
	}
	s.pending += storedLen
}

func (s *state) trAlign() {
	s.sendBits(staticTrees<<1, 3)
	s.sendCode(endBlock, staticLtree[:])
	s.biFlush()
}

// trFlushBlock is _tr_flush_block; buf is nil when block_start < 0.
func (s *state) trFlushBlock(buf []byte, storedLen uint64, last int) {
	var optLenb, staticLenb uint64
	maxBLIndex := 0
	if s.level > 0 {
		s.buildTree(&s.lDesc)
		s.buildTree(&s.dDesc)
		maxBLIndex = s.buildBLTree()
		optLenb = (s.optLen + 3 + 7) >> 3
		staticLenb = (s.staticLen + 3 + 7) >> 3
		if staticLenb <= optLenb || s.strategy == Fixed {
			optLenb = staticLenb
		}
	} else {
		optLenb = storedLen + 5
		staticLenb = optLenb
	}

	if storedLen+4 <= optLenb && buf != nil {
		s.trStoredBlock(buf, storedLen, last)
	} else if staticLenb == optLenb {
		s.sendBits(staticTrees<<1+last, 3)
		s.compressBlock(staticLtree[:], staticDtree[:])
	} else {
		s.sendBits(dynTrees<<1+last, 3)
		s.sendAllTrees(s.lDesc.maxCode+1, s.dDesc.maxCode+1, maxBLIndex+1)
		s.compressBlock(s.dynLtree[:], s.dynDtree[:])
	}
	s.initBlock()
	if last != 0 {
		s.biWindup()
	}
}

func (s *state) compressBlock(ltree, dtree []ctData) {
	for sx := uint32(0); sx < s.symNext; sx += 3 {
		dist := uint32(s.symBuf[sx]) | uint32(s.symBuf[sx+1])<<8
		lc := int(s.symBuf[sx+2])
		if dist == 0 {
			s.sendCode(lc, ltree)
			continue
		}
		code := int(lengthCode[lc])
		s.sendCode(code+literals+1, ltree)
		if extra := extraLbits[code]; extra != 0 {
			lc -= baseLength[code]
			s.sendBits(lc, extra)
		}
		dist--
		dcode := int(dCode(dist))
		s.sendCode(dcode, dtree)
		if extra := extraDbits[dcode]; extra != 0 {
			dist -= uint32(baseDist[dcode])
			s.sendBits(int(dist), extra)
		}
	}
	s.sendCode(endBlock, ltree)
}

func (s *state) sendCode(c int, tree []ctData) {
	s.sendBits(int(tree[c].fc), int(tree[c].dl))
}

// sendBits is the send_bits macro with its 16-bit bit buffer.
func (s *state) sendBits(value, length int) {
	if s.biValid > bufSize-length {
		s.biBuf |= uint16(value) << uint(s.biValid)
		s.putShort(s.biBuf)
		s.biBuf = uint16(value) >> uint(bufSize-s.biValid)
		s.biValid += length - bufSize
	} else {
		s.biBuf |= uint16(value) << uint(s.biValid)
		s.biValid += length
	}
}

func (s *state) biFlush() {
	if s.biValid == 16 {
		s.putShort(s.biBuf)
		s.biBuf = 0
		s.biValid = 0
	} else if s.biValid >= 8 {
		s.putByte(byte(s.biBuf))
		s.biBuf >>= 8
		s.biValid -= 8
	}
}

func (s *state) biWindup() {
	if s.biValid > 8 {
		s.putShort(s.biBuf)
	} else if s.biValid > 0 {
		s.putByte(byte(s.biBuf))
	}
	s.biBuf = 0
	s.biValid = 0
}
