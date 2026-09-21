package jpegenc

import "math/bits"

// huffTable is JHUFF_TBL: code counts per length and symbols in code order.
type huffTable struct {
	bits    [17]byte
	huffval [256]byte
}

func newHuffTable(counts [17]byte, values []byte) *huffTable {
	t := &huffTable{bits: counts}
	copy(t.huffval[:], values)
	return t
}

// derivedTable is c_derived_tbl.
type derivedTable struct {
	code [256]uint32
	size [256]uint
}

// makeDerived is jpeg_make_c_derived_tbl.
func makeDerived(t *huffTable, isDC bool) (*derivedTable, error) {
	var huffsize [257]int
	var huffcode [257]uint32
	p := 0
	for l := 1; l <= 16; l++ {
		count := int(t.bits[l])
		if p+count > 256 {
			return nil, libjpegError("JERR_BAD_HUFF_TABLE")
		}
		for ; count > 0; count-- {
			huffsize[p] = l
			p++
		}
	}
	huffsize[p] = 0
	lastp := p
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
			return nil, libjpegError("JERR_BAD_HUFF_TABLE")
		}
		code <<= 1
		si++
	}
	d := &derivedTable{}
	maxSymbol := 255
	if isDC {
		maxSymbol = 15
	}
	for p = 0; p < lastp; p++ {
		symbol := int(t.huffval[p])
		if symbol > maxSymbol || d.size[symbol] != 0 {
			return nil, libjpegError("JERR_BAD_HUFF_TABLE")
		}
		d.code[symbol] = huffcode[p]
		d.size[symbol] = uint(huffsize[p])
	}
	return d, nil
}

// forEachMCU walks the scan's MCUs in compress_output order and hands fn
// the component index and coefficients of every block.
func (e *encoder) forEachMCU(fn func(mcu int, blocks []mcuBlock) error) error {
	blocks := make([]mcuBlock, 0, maxBlocksInMCU)
	if len(e.comps) == 1 {
		c := &e.comps[0]
		mcu := 0
		for row := 0; row < c.heightInBlocks; row++ {
			for col := 0; col < c.widthInBlocks; col++ {
				at := (row*c.blocksWide + col) * 64
				blocks = append(blocks[:0], mcuBlock{ci: 0, coef: c.coef[at : at+64]})
				if err := fn(mcu, blocks); err != nil {
					return err
				}
				mcu++
			}
		}
		return nil
	}
	mcu := 0
	for mr := 0; mr < e.mcuRows; mr++ {
		for mc := 0; mc < e.mcusPerRow; mc++ {
			blocks = blocks[:0]
			for ci := range e.comps {
				c := &e.comps[ci]
				for yi := 0; yi < c.v; yi++ {
					for xi := 0; xi < c.h; xi++ {
						at := ((mr*c.v+yi)*c.blocksWide + mc*c.h + xi) * 64
						blocks = append(blocks, mcuBlock{ci: ci, coef: c.coef[at : at+64]})
					}
				}
			}
			if err := fn(mcu, blocks); err != nil {
				return err
			}
			mcu++
		}
	}
	return nil
}

type mcuBlock struct {
	ci   int
	coef []int16
}

// optimizeTables is the huff_opt pass: encode_mcu_gather over the scan,
// then jpeg_gen_optimal_table for each table a component uses.
func (e *encoder) optimizeTables(dcTables, acTables *[2]*huffTable) error {
	var dcCounts, acCounts [2][257]int64
	lastDC := make([]int, len(e.comps))
	interval := e.opts.RestartInterval
	restartsToGo := interval
	err := e.forEachMCU(func(_ int, blocks []mcuBlock) error {
		if interval > 0 {
			if restartsToGo == 0 {
				clear(lastDC)
				restartsToGo = interval
			}
			restartsToGo--
		}
		for _, block := range blocks {
			tbl := e.comps[block.ci].tbl
			if err := htestOneBlock(block.coef, lastDC[block.ci], &dcCounts[tbl], &acCounts[tbl]); err != nil {
				return err
			}
			lastDC[block.ci] = int(block.coef[0])
		}
		return nil
	})
	if err != nil {
		return err
	}
	var didDC, didAC [2]bool
	for _, c := range e.comps {
		if !didDC[c.tbl] {
			if err := genOptimalTable(dcTables[c.tbl], &dcCounts[c.tbl]); err != nil {
				return err
			}
			didDC[c.tbl] = true
		}
		if !didAC[c.tbl] {
			if err := genOptimalTable(acTables[c.tbl], &acCounts[c.tbl]); err != nil {
				return err
			}
			didAC[c.tbl] = true
		}
	}
	return nil
}

// htestOneBlock is jchuff.c htest_one_block.
func htestOneBlock(block []int16, lastDC int, dcCounts, acCounts *[257]int64) error {
	const maxCoefBits = 8 + 2
	temp := int(block[0]) - lastDC
	if temp < 0 {
		temp = -temp
	}
	nbits := bits.Len(uint(temp))
	if nbits > maxCoefBits+1 {
		return libjpegError("JERR_BAD_DCT_COEF")
	}
	dcCounts[nbits]++
	r := 0
	for k := 1; k < 64; k++ {
		temp = int(block[naturalOrder[k]])
		if temp == 0 {
			r++
			continue
		}
		for r > 15 {
			acCounts[0xF0]++
			r -= 16
		}
		if temp < 0 {
			temp = -temp
		}
		nbits = bits.Len(uint(temp))
		if nbits > maxCoefBits {
			return libjpegError("JERR_BAD_DCT_COEF")
		}
		acCounts[r<<4+nbits]++
		r = 0
	}
	if r > 0 {
		acCounts[0]++
	}
	return nil
}

// genOptimalTable is jpeg_gen_optimal_table as libjpeg-turbo 3.1 writes it
// (nonzero symbols compacted first; ties pick the higher index).
func genOptimalTable(t *huffTable, counts *[257]int64) error {
	const maxCLen = 32
	var bitsCount [maxCLen + 2]int
	var bitPos [maxCLen + 1]int
	var codesize, nzIndex, others [257]int
	freq := *counts
	for i := range others {
		others[i] = -1
	}
	freq[256] = 1
	numNZ := 0
	for i := 0; i < 257; i++ {
		if freq[i] != 0 {
			nzIndex[numNZ] = i
			freq[numNZ] = freq[i]
			numNZ++
		}
	}
	for {
		c1, c2 := -1, -1
		v, v2 := int64(1_000_000_000), int64(1_000_000_000)
		for i := 0; i < numNZ; i++ {
			if freq[i] <= v2 {
				if freq[i] <= v {
					c2 = c1
					v2 = v
					v = freq[i]
					c1 = i
				} else {
					v2 = freq[i]
					c2 = i
				}
			}
		}
		if c2 < 0 {
			break
		}
		freq[c1] += freq[c2]
		freq[c2] = 1_000_000_001
		codesize[c1]++
		for others[c1] >= 0 {
			c1 = others[c1]
			codesize[c1]++
		}
		others[c1] = c2
		codesize[c2]++
		for others[c2] >= 0 {
			c2 = others[c2]
			codesize[c2]++
		}
	}
	for i := 0; i < numNZ; i++ {
		if codesize[i] > maxCLen {
			return libjpegError("JERR_HUFF_CLEN_OVERFLOW")
		}
		bitsCount[codesize[i]]++
	}
	p := 0
	for i := 1; i <= maxCLen; i++ {
		bitPos[i] = p
		p += bitsCount[i]
	}
	i := maxCLen
	for ; i > 16; i-- {
		for bitsCount[i] > 0 {
			j := i - 2
			for bitsCount[j] == 0 {
				j--
			}
			bitsCount[i] -= 2
			bitsCount[i-1]++
			bitsCount[j+1] += 2
			bitsCount[j]--
		}
	}
	for bitsCount[i] == 0 {
		i--
	}
	bitsCount[i]--
	for l := 0; l <= 16; l++ {
		t.bits[l] = byte(bitsCount[l])
	}
	for i := 0; i < numNZ-1; i++ {
		t.huffval[bitPos[codesize[i]]] = byte(nzIndex[i])
		bitPos[codesize[i]]++
	}
	return nil
}

// bitWriter emits Huffman codes MSB first with 0xFF byte stuffing.
type bitWriter struct {
	out  []byte
	acc  uint64
	nbit uint
}

func (w *bitWriter) put(code uint32, size uint) {
	if size == 0 {
		return
	}
	w.acc = w.acc<<size | uint64(code)&(1<<size-1)
	w.nbit += size
	for w.nbit >= 8 {
		w.nbit -= 8
		b := byte(w.acc >> w.nbit)
		w.out = append(w.out, b)
		if b == 0xFF {
			w.out = append(w.out, 0)
		}
	}
}

// flush is jchuff.c flush_bits: pad the last partial byte with 1-bits.
func (w *bitWriter) flush() {
	if w.nbit > 0 {
		b := byte(w.acc<<(8-w.nbit)) | byte(0xFF>>w.nbit)
		w.out = append(w.out, b)
		if b == 0xFF {
			w.out = append(w.out, 0)
		}
	}
	w.acc, w.nbit = 0, 0
}

// encodeScan is the output pass: encode_mcu_huff with restart markers,
// then finish_pass_huff.
func (e *encoder) encodeScan(out []byte, dc, ac *[2]*derivedTable) []byte {
	w := &bitWriter{out: out}
	lastDC := make([]int, len(e.comps))
	interval := e.opts.RestartInterval
	restartsToGo := interval
	nextRestart := 0
	_ = e.forEachMCU(func(_ int, blocks []mcuBlock) error {
		if interval > 0 && restartsToGo == 0 {
			w.flush()
			w.out = append(w.out, 0xFF, byte(0xD0+nextRestart))
			clear(lastDC)
		}
		for _, block := range blocks {
			tbl := e.comps[block.ci].tbl
			encodeOneBlock(w, block.coef, lastDC[block.ci], dc[tbl], ac[tbl])
			lastDC[block.ci] = int(block.coef[0])
		}
		if interval > 0 {
			if restartsToGo == 0 {
				restartsToGo = interval
				nextRestart = (nextRestart + 1) & 7
			}
			restartsToGo--
		}
		return nil
	})
	w.flush()
	return w.out
}

// encodeOneBlock is jchuff.c encode_one_block.
func encodeOneBlock(w *bitWriter, block []int16, lastDC int, dc, ac *derivedTable) {
	temp := int(block[0]) - lastDC
	value := temp
	if temp < 0 {
		temp = -temp
		value--
	}
	nbits := uint(bits.Len(uint(temp)))
	w.put(dc.code[nbits], dc.size[nbits])
	w.put(uint32(value), nbits)
	r := 0
	for k := 1; k < 64; k++ {
		temp = int(block[naturalOrder[k]])
		if temp == 0 {
			r++
			continue
		}
		value = temp
		if temp < 0 {
			temp = -temp
			value--
		}
		nbits = uint(bits.Len(uint(temp)))
		for r > 15 {
			w.put(ac.code[0xF0], ac.size[0xF0])
			r -= 16
		}
		symbol := r<<4 + int(nbits)
		w.put(ac.code[symbol], ac.size[symbol])
		w.put(uint32(value), nbits)
		r = 0
	}
	if r > 0 {
		w.put(ac.code[0], ac.size[0])
	}
}
