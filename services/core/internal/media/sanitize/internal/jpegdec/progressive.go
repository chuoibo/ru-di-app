package jpegdec

import "math"

// Port of jdphuff.c: progressive Huffman decoding.

// startPassPhuff is start_pass_phuff_decoder.
func (d *decoder) startPassPhuff() {
	e := &d.ent
	isDCBand := d.ss == 0
	bad := false
	if isDCBand {
		if d.se != 0 {
			bad = true
		}
	} else {
		if d.ss > d.se || d.se >= 64 {
			bad = true
		}
		if d.compsInScan != 1 {
			bad = true
		}
	}
	if d.ah != 0 && d.al != d.ah-1 {
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
	case d.ah == 0 && isDCBand:
		e.decode = d.decodeMCUDCFirst
	case d.ah == 0:
		e.decode = d.decodeMCUACFirst
	case isDCBand:
		e.decode = d.decodeMCUDCRefine
	default:
		e.decode = d.decodeMCUACRefine
	}
	for ci := 0; ci < d.compsInScan; ci++ {
		c := d.curComps[ci]
		if isDCBand {
			if d.ah == 0 {
				d.makeDerivedTable(true, c.dcTblNo, &e.derived[c.dcTblNo&(numHuffTbls-1)])
			}
		} else {
			d.makeDerivedTable(false, c.acTblNo, &e.derived[c.acTblNo&(numHuffTbls-1)])
			e.acDerivedTbl = e.derived[c.acTblNo]
		}
		e.lastDCVal[ci] = 0
	}
	e.bitsLeft = 0
	e.getBuffer = 0
	e.insufficientData = false
	e.eobrun = 0
	e.restartsToGo = d.restartInterval
}

func (d *decoder) processRestartPhuff() {
	e := &d.ent
	e.bitsLeft = 0
	d.readRestartMarker()
	for ci := 0; ci < d.compsInScan; ci++ {
		e.lastDCVal[ci] = 0
	}
	e.eobrun = 0
	e.restartsToGo = d.restartInterval
	if d.unreadMarker == 0 {
		e.insufficientData = false
	}
}

func (d *decoder) saveBits(br *bitState) {
	d.src.pos = br.pos
	d.ent.getBuffer = br.getBuffer
	d.ent.bitsLeft = br.bitsLeft
}

func (d *decoder) loadBits() bitState {
	return bitState{pos: d.src.pos, getBuffer: d.ent.getBuffer, bitsLeft: d.ent.bitsLeft}
}

func (d *decoder) decodeMCUDCFirst(blocks [][]int16) bool {
	e := &d.ent
	if d.restartInterval != 0 && e.restartsToGo == 0 {
		d.processRestartPhuff()
	}
	if !e.insufficientData {
		br := d.loadBits()
		state := e.lastDCVal
		for blkn := 0; blkn < d.blocksInMCU; blkn++ {
			block := blocks[blkn]
			ci := d.mcuMembership[blkn]
			tbl := e.derived[d.curComps[ci].dcTblNo]
			s, ok := d.huffDecode(&br, tbl)
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
			last := state[ci]
			if (last >= 0 && int64(s) > int64(math.MaxInt32)-int64(last)) ||
				(last < 0 && int64(s) < int64(math.MinInt32)-int64(last)) {
				errexit("JERR_BAD_DCT_COEF")
			}
			state[ci] = last + int32(s)
			block[0] = int16(uint32(state[ci]) << uint(d.al))
		}
		d.saveBits(&br)
		e.lastDCVal = state
	}
	if d.restartInterval != 0 {
		e.restartsToGo--
	}
	return true
}

func (d *decoder) decodeMCUACFirst(blocks [][]int16) bool {
	e := &d.ent
	if d.restartInterval != 0 && e.restartsToGo == 0 {
		d.processRestartPhuff()
	}
	if !e.insufficientData {
		eobrun := e.eobrun
		if eobrun > 0 {
			eobrun--
		} else {
			br := d.loadBits()
			block := blocks[0]
			tbl := e.acDerivedTbl
			for k := d.ss; k <= d.se; k++ {
				s, ok := d.huffDecode(&br, tbl)
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
					block[naturalOrder[k]] = int16(uint32(s) << uint(d.al))
				} else if r == 15 {
					k += 15
				} else {
					eobrun = 1 << uint(r)
					if r != 0 {
						if !d.checkBitBuffer(&br, r) {
							return false
						}
						eobrun += uint32(getBits(&br, r))
					}
					eobrun--
					break
				}
			}
			d.saveBits(&br)
		}
		e.eobrun = eobrun
	}
	if d.restartInterval != 0 {
		e.restartsToGo--
	}
	return true
}

func (d *decoder) decodeMCUDCRefine(blocks [][]int16) bool {
	e := &d.ent
	p1 := 1 << uint(d.al)
	if d.restartInterval != 0 && e.restartsToGo == 0 {
		d.processRestartPhuff()
	}
	br := d.loadBits()
	for blkn := 0; blkn < d.blocksInMCU; blkn++ {
		block := blocks[blkn]
		if !d.checkBitBuffer(&br, 1) {
			return false
		}
		if getBits(&br, 1) != 0 {
			block[0] = int16(int(block[0]) | p1)
		}
	}
	d.saveBits(&br)
	if d.restartInterval != 0 {
		e.restartsToGo--
	}
	return true
}

func (d *decoder) decodeMCUACRefine(blocks [][]int16) bool {
	e := &d.ent
	se := d.se
	p1 := 1 << uint(d.al)
	m1 := -1 << uint(d.al)
	if d.restartInterval != 0 && e.restartsToGo == 0 {
		d.processRestartPhuff()
	}
	if e.insufficientData {
		if d.restartInterval != 0 {
			e.restartsToGo--
		}
		return true
	}
	br := d.loadBits()
	eobrun := e.eobrun
	block := blocks[0]
	tbl := e.acDerivedTbl
	var newnzPos [64]int
	numNewnz := 0
	undo := func() bool {
		for numNewnz > 0 {
			numNewnz--
			block[newnzPos[numNewnz]] = 0
		}
		return false
	}
	correct := func(pos int) bool {
		thiscoef := int(block[pos])
		if !d.checkBitBuffer(&br, 1) {
			return false
		}
		if getBits(&br, 1) != 0 && thiscoef&p1 == 0 {
			if thiscoef >= 0 {
				block[pos] = int16(thiscoef + p1)
			} else {
				block[pos] = int16(thiscoef + m1)
			}
		}
		return true
	}
	k := d.ss
	if eobrun == 0 {
		for ; k <= se; k++ {
			s, ok := d.huffDecode(&br, tbl)
			if !ok {
				return undo()
			}
			r := s >> 4
			s &= 15
			if s != 0 {
				if !d.checkBitBuffer(&br, 1) {
					return undo()
				}
				if getBits(&br, 1) != 0 {
					s = p1
				} else {
					s = m1
				}
			} else if r != 15 {
				eobrun = 1 << uint(r)
				if r != 0 {
					if !d.checkBitBuffer(&br, r) {
						return undo()
					}
					eobrun += uint32(getBits(&br, r))
				}
				break
			}
			for {
				pos := naturalOrder[k]
				if block[pos] != 0 {
					if !correct(pos) {
						return undo()
					}
				} else {
					r--
					if r < 0 {
						break
					}
				}
				k++
				if k > se {
					break
				}
			}
			if s != 0 {
				pos := naturalOrder[k]
				block[pos] = int16(s)
				newnzPos[numNewnz] = pos
				numNewnz++
			}
		}
	}
	if eobrun > 0 {
		for ; k <= se; k++ {
			pos := naturalOrder[k]
			if block[pos] != 0 {
				if !correct(pos) {
					return undo()
				}
			}
		}
		eobrun--
	}
	d.saveBits(&br)
	e.eobrun = eobrun
	if d.restartInterval != 0 {
		e.restartsToGo--
	}
	return true
}
