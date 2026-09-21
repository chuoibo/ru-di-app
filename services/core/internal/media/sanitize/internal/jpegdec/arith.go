package jpegdec

// Port of jdarith.c: arithmetic-coded sequential and progressive scans.
// Its get_byte cannot suspend: under Pillow's suspending source a read
// past the block Pillow has handed over is JERR_CANT_SUSPEND, and so is a
// restart marker that runs past it.

const (
	dcStatBins = 64
	acStatBins = 256
)

// aritab is jpeg_aritab: Qe<<16 | NextMPS<<8 | SwitchMPS<<7 | NextLPS.
var aritab = func() (t [114]int64) {
	rows := [114][4]int64{
		{0x5a1d, 1, 1, 1}, {0x2586, 14, 2, 0}, {0x1114, 16, 3, 0}, {0x080b, 18, 4, 0}, {0x03d8, 20, 5, 0},
		{0x01da, 23, 6, 0}, {0x00e5, 25, 7, 0}, {0x006f, 28, 8, 0}, {0x0036, 30, 9, 0}, {0x001a, 33, 10, 0},
		{0x000d, 35, 11, 0}, {0x0006, 9, 12, 0}, {0x0003, 10, 13, 0}, {0x0001, 12, 13, 0}, {0x5a7f, 15, 15, 1},
		{0x3f25, 36, 16, 0}, {0x2cf2, 38, 17, 0}, {0x207c, 39, 18, 0}, {0x17b9, 40, 19, 0}, {0x1182, 42, 20, 0},
		{0x0cef, 43, 21, 0}, {0x09a1, 45, 22, 0}, {0x072f, 46, 23, 0}, {0x055c, 48, 24, 0}, {0x0406, 49, 25, 0},
		{0x0303, 51, 26, 0}, {0x0240, 52, 27, 0}, {0x01b1, 54, 28, 0}, {0x0144, 56, 29, 0}, {0x00f5, 57, 30, 0},
		{0x00b7, 59, 31, 0}, {0x008a, 60, 32, 0}, {0x0068, 62, 33, 0}, {0x004e, 63, 34, 0}, {0x003b, 32, 35, 0},
		{0x002c, 33, 9, 0}, {0x5ae1, 37, 37, 1}, {0x484c, 64, 38, 0}, {0x3a0d, 65, 39, 0}, {0x2ef1, 67, 40, 0},
		{0x261f, 68, 41, 0}, {0x1f33, 69, 42, 0}, {0x19a8, 70, 43, 0}, {0x1518, 72, 44, 0}, {0x1177, 73, 45, 0},
		{0x0e74, 74, 46, 0}, {0x0bfb, 75, 47, 0}, {0x09f8, 77, 48, 0}, {0x0861, 78, 49, 0}, {0x0706, 79, 50, 0},
		{0x05cd, 48, 51, 0}, {0x04de, 50, 52, 0}, {0x040f, 50, 53, 0}, {0x0363, 51, 54, 0}, {0x02d4, 52, 55, 0},
		{0x025c, 53, 56, 0}, {0x01f8, 54, 57, 0}, {0x01a4, 55, 58, 0}, {0x0160, 56, 59, 0}, {0x0125, 57, 60, 0},
		{0x00f6, 58, 61, 0}, {0x00cb, 59, 62, 0}, {0x00ab, 61, 63, 0}, {0x008f, 61, 32, 0}, {0x5b12, 65, 65, 1},
		{0x4d04, 80, 66, 0}, {0x412c, 81, 67, 0}, {0x37d8, 82, 68, 0}, {0x2fe8, 83, 69, 0}, {0x293c, 84, 70, 0},
		{0x2379, 86, 71, 0}, {0x1edf, 87, 72, 0}, {0x1aa9, 87, 73, 0}, {0x174e, 72, 74, 0}, {0x1424, 72, 75, 0},
		{0x119c, 74, 76, 0}, {0x0f6b, 74, 77, 0}, {0x0d51, 75, 78, 0}, {0x0bb6, 77, 79, 0}, {0x0a40, 77, 48, 0},
		{0x5832, 80, 81, 1}, {0x4d1c, 88, 82, 0}, {0x438e, 89, 83, 0}, {0x3bdd, 90, 84, 0}, {0x34ee, 91, 85, 0},
		{0x2eae, 92, 86, 0}, {0x299a, 93, 87, 0}, {0x2516, 86, 71, 0}, {0x5570, 88, 89, 1}, {0x4ca9, 95, 90, 0},
		{0x44d9, 96, 91, 0}, {0x3e22, 97, 92, 0}, {0x3824, 99, 93, 0}, {0x32b4, 99, 94, 0}, {0x2e17, 93, 86, 0},
		{0x56a8, 95, 96, 1}, {0x4f46, 101, 97, 0}, {0x47e5, 102, 98, 0}, {0x41cf, 103, 99, 0}, {0x3c3d, 104, 100, 0},
		{0x375e, 99, 93, 0}, {0x5231, 105, 102, 0}, {0x4c0f, 106, 103, 0}, {0x4639, 107, 104, 0}, {0x415e, 103, 99, 0},
		{0x5627, 105, 106, 1}, {0x50e7, 108, 107, 0}, {0x4b85, 109, 103, 0}, {0x5597, 110, 109, 0}, {0x504f, 111, 107, 0},
		{0x5a10, 110, 111, 1}, {0x5522, 112, 109, 0}, {0x59eb, 112, 111, 1}, {0x5a1d, 113, 113, 0},
	}
	for i, r := range rows {
		t[i] = r[0]<<16 | r[2]<<8 | r[3]<<7 | r[1]
	}
	return t
}()

type arithState struct {
	c, a        int64
	ct          int
	lastDCVal   [maxCompsInScan]int
	dcContext   [maxCompsInScan]int
	restarts    int
	dcStats     [numArithTbls][]byte
	acStats     [numArithTbls][]byte
	fixedBin    [4]byte
	initialized bool
}

func (d *decoder) arithGetByte() int {
	s := d.src
	if s.pos >= s.limit {
		errexit("JERR_CANT_SUSPEND")
	}
	c := int(s.data[s.pos])
	s.pos++
	return c
}

// arithDecode is arith_decode on the statistics bin st[i].
func (d *decoder) arithDecode(st []byte, i int) int {
	e := d.arithS
	for e.a < 0x8000 {
		e.ct--
		if e.ct < 0 {
			var data int
			if d.unreadMarker != 0 {
				data = 0
			} else {
				data = d.arithGetByte()
				if data == 0xFF {
					for {
						data = d.arithGetByte()
						if data != 0xFF {
							break
						}
					}
					if data == 0 {
						data = 0xFF
					} else {
						d.unreadMarker = data
						data = 0
					}
				}
			}
			e.c = e.c<<8 | int64(data)
			e.ct += 8
			if e.ct < 0 {
				e.ct++
				if e.ct == 0 {
					e.a = 0x8000
				}
			}
		}
		e.a <<= 1
	}
	sv := int(st[i])
	qe := aritab[sv&0x7F]
	nl := byte(qe & 0xFF)
	qe >>= 8
	nm := byte(qe & 0xFF)
	qe >>= 8
	temp := e.a - qe
	e.a = temp
	temp <<= uint(e.ct)
	if e.c >= temp {
		e.c -= temp
		if e.a < qe {
			e.a = qe
			st[i] = byte(sv&0x80) ^ nm
		} else {
			e.a = qe
			st[i] = byte(sv&0x80) ^ nl
			sv ^= 0x80
		}
	} else if e.a < 0x8000 {
		if e.a < qe {
			st[i] = byte(sv&0x80) ^ nl
			sv ^= 0x80
		} else {
			st[i] = byte(sv&0x80) ^ nm
		}
	}
	return sv >> 7
}

func (d *decoder) arithProcessRestart() {
	e := d.arithS
	d.readRestartMarker()
	for ci := 0; ci < d.compsInScan; ci++ {
		c := d.curComps[ci]
		if !d.progressive || (d.ss == 0 && d.ah == 0) {
			clear(e.dcStats[c.dcTblNo])
			e.lastDCVal[ci] = 0
			e.dcContext[ci] = 0
		}
		if !d.progressive || d.ss != 0 {
			clear(e.acStats[c.acTblNo])
		}
	}
	e.c, e.a, e.ct = 0, 0, -16
	e.restarts = d.restartInterval
}

func (d *decoder) arithRestartDue() {
	if d.restartInterval != 0 {
		if d.arithS.restarts == 0 {
			d.arithProcessRestart()
		}
		d.arithS.restarts--
	}
}

// arithACBlock decodes coefficients ss..se of one block (sequential
// decode_mcu and decode_mcu_AC_first); false means the spectral overflow
// warning ended the MCU.
func (d *decoder) arithACBlock(block []int16, tbl, ss, se int, shift uint) bool {
	e := d.arithS
	stats := e.acStats[tbl]
	for k := ss; k <= se; k++ {
		si := 3 * (k - 1)
		if d.arithDecode(stats, si) != 0 {
			break
		}
		for d.arithDecode(stats, si+1) == 0 {
			si += 3
			k++
			if k > se {
				e.ct = -1
				return false
			}
		}
		sign := d.arithDecode(e.fixedBin[:], 0)
		si += 2
		m := d.arithDecode(stats, si)
		magBase := si
		if m != 0 {
			if d.arithDecode(stats, si) != 0 {
				m <<= 1
				magBase = 217
				if k <= int(d.arithAcK[tbl]) {
					magBase = 189
				}
				si = magBase
				for d.arithDecode(stats, si) != 0 {
					m <<= 1
					if m == 0x8000 {
						e.ct = -1
						return false
					}
					si++
				}
				magBase = si
			}
		}
		v := m
		si = magBase + 14
		for m >>= 1; m != 0; m >>= 1 {
			if d.arithDecode(stats, si) != 0 {
				v |= m
			}
		}
		v++
		if sign != 0 {
			v = -v
		}
		block[naturalOrder[k]] = int16(uint32(v) << shift)
	}
	return true
}

func (d *decoder) decodeMCUArith(blocks [][]int16) bool {
	d.arithRestartDue()
	if d.arithS.ct == -1 {
		return true
	}
	d.arithSequential(blocks)
	return true
}

// arithSequential is jdarith.c decode_mcu.
func (d *decoder) arithSequential(blocks [][]int16) {
	e := d.arithS
	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		block := blocks[blkn]
		ci := d.mcuMembership[blkn]
		comp := d.curComps[ci]
		tbl := comp.dcTblNo
		if !d.arithDCValue(ci, tbl) {
			return
		}
		block[0] = int16(e.lastDCVal[ci])
		if !d.arithACBlock(block, comp.acTblNo, 1, 63, 0) {
			return
		}
	}
}

// arithDCValue decodes one DC difference into lastDCVal; false is the
// category overflow.
func (d *decoder) arithDCValue(ci, tbl int) bool {
	e := d.arithS
	stats := e.dcStats[tbl]
	si := e.dcContext[ci]
	if d.arithDecode(stats, si) == 0 {
		e.dcContext[ci] = 0
		return true
	}
	sign := d.arithDecode(stats, si+1)
	si += 2 + sign
	m := d.arithDecode(stats, si)
	if m != 0 {
		si = 20
		for d.arithDecode(stats, si) != 0 {
			m <<= 1
			if m == 0x8000 {
				e.ct = -1
				return false
			}
			si++
		}
	}
	switch {
	case m < int((int64(1)<<d.arithDcL[tbl])>>1):
		e.dcContext[ci] = 0
	case m > int((int64(1)<<d.arithDcU[tbl])>>1):
		e.dcContext[ci] = 12 + sign*4
	default:
		e.dcContext[ci] = 4 + sign*4
	}
	v := m
	si += 14
	for m >>= 1; m != 0; m >>= 1 {
		if d.arithDecode(stats, si) != 0 {
			v |= m
		}
	}
	v++
	if sign != 0 {
		v = -v
	}
	e.lastDCVal[ci] = (e.lastDCVal[ci] + v) & 0xffff
	return true
}

func (d *decoder) decodeMCUArithDCFirst(blocks [][]int16) bool {
	d.arithRestartDue()
	e := d.arithS
	if e.ct == -1 {
		return true
	}
	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		ci := d.mcuMembership[blkn]
		if !d.arithDCValue(ci, d.curComps[ci].dcTblNo) {
			return true
		}
		blocks[blkn][0] = int16(uint32(e.lastDCVal[ci]) << uint(d.al))
	}
	return true
}

func (d *decoder) decodeMCUArithACFirst(blocks [][]int16) bool {
	d.arithRestartDue()
	if d.arithS.ct == -1 {
		return true
	}
	d.arithACBlock(blocks[0], d.curComps[0].acTblNo, d.ss, d.se, uint(d.al))
	return true
}

func (d *decoder) decodeMCUArithDCRefine(blocks [][]int16) bool {
	d.arithRestartDue()
	e := d.arithS
	p1 := 1 << uint(d.al)
	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		if d.arithDecode(e.fixedBin[:], 0) != 0 {
			blocks[blkn][0] = int16(int(blocks[blkn][0]) | p1)
		}
	}
	return true
}

func (d *decoder) decodeMCUArithACRefine(blocks [][]int16) bool {
	d.arithRestartDue()
	e := d.arithS
	if e.ct == -1 {
		return true
	}
	block := blocks[0]
	tbl := d.curComps[0].acTblNo
	stats := e.acStats[tbl]
	p1 := 1 << uint(d.al)
	m1 := -1 << uint(d.al)
	kex := d.se
	for ; kex > 0; kex-- {
		if block[naturalOrder[kex]] != 0 {
			break
		}
	}
	for k := d.ss; k <= d.se; k++ {
		si := 3 * (k - 1)
		if k > kex {
			if d.arithDecode(stats, si) != 0 {
				break
			}
		}
		for {
			pos := naturalOrder[k]
			if block[pos] != 0 {
				if d.arithDecode(stats, si+2) != 0 {
					if block[pos] < 0 {
						block[pos] = int16(int(block[pos]) + m1)
					} else {
						block[pos] = int16(int(block[pos]) + p1)
					}
				}
				break
			}
			if d.arithDecode(stats, si+1) != 0 {
				if d.arithDecode(e.fixedBin[:], 0) != 0 {
					block[pos] = int16(m1)
				} else {
					block[pos] = int16(p1)
				}
				break
			}
			si += 3
			k++
			if k > d.se {
				e.ct = -1
				return true
			}
		}
	}
	return true
}

// startPassArith is jdarith.c start_pass.
func (d *decoder) startPassArith() {
	if d.arithS == nil {
		d.arithS = &arithState{}
		d.arithS.fixedBin[0] = 113
	}
	e := d.arithS
	if d.progressive {
		bad := false
		if d.ss == 0 {
			if d.se != 0 {
				bad = true
			}
		} else if d.se < d.ss || d.se > 63 || d.compsInScan != 1 {
			bad = true
		}
		if d.ah != 0 && d.ah-1 != d.al {
			bad = true
		}
		if d.al > 13 {
			bad = true
		}
		if bad {
			errexit("JERR_BAD_PROGRESSION")
		}
		for ci := 0; ci < d.compsInScan; ci++ {
			cindex := d.curComps[ci].index
			coefBits := &d.coefBits[cindex]
			prevBits := &d.coefBits[cindex+d.numComponents]
			for coefi := min(d.ss, 1); coefi <= max(d.se, 9); coefi++ {
				if d.inputScanNumber > 1 {
					prevBits[coefi] = coefBits[coefi]
				} else {
					prevBits[coefi] = 0
				}
			}
			for coefi := d.ss; coefi <= d.se; coefi++ {
				coefBits[coefi] = d.al
			}
		}
		switch {
		case d.ah == 0 && d.ss == 0:
			d.ent.decode = d.decodeMCUArithDCFirst
		case d.ah == 0:
			d.ent.decode = d.decodeMCUArithACFirst
		case d.ss == 0:
			d.ent.decode = d.decodeMCUArithDCRefine
		default:
			d.ent.decode = d.decodeMCUArithACRefine
		}
	} else {
		d.ent.decode = d.decodeMCUArith
	}
	inner := d.ent.decode
	d.ent.decode = func(blocks [][]int16) bool {
		d.src.strict = true
		ok := inner(blocks)
		d.src.strict = false
		return ok
	}
	for ci := 0; ci < d.compsInScan; ci++ {
		c := d.curComps[ci]
		if !d.progressive || (d.ss == 0 && d.ah == 0) {
			if e.dcStats[c.dcTblNo] == nil {
				e.dcStats[c.dcTblNo] = make([]byte, dcStatBins)
			}
			clear(e.dcStats[c.dcTblNo])
			e.lastDCVal[ci] = 0
			e.dcContext[ci] = 0
		}
		if !d.progressive || d.ss != 0 {
			if e.acStats[c.acTblNo] == nil {
				e.acStats[c.acTblNo] = make([]byte, acStatBins)
			}
			clear(e.acStats[c.acTblNo])
		}
	}
	e.c, e.a, e.ct = 0, 0, -16
	d.ent.insufficientData = false
	e.restarts = d.restartInterval
}
