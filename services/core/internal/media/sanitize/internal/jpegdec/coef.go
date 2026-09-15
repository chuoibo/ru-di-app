package jpegdec

// Port of jdcoefct.c: the single-pass coefficient controller of a
// single-scan file, the whole-image controller of a multi-scan file, and
// the progressive block smoothing that runs when a progressive file leaves
// coefficients unsent.

// retryMCU runs one decode_mcu the way Pillow drives a suspending decoder:
// when the entropy decoder runs dry, ImageFile.load reads the next block
// and calls the decoder again, which resumes from the saved state; with no
// block left, load raises.
func (d *decoder) retryMCU(blocks [][]int16) {
	for !d.ent.decode(blocks) {
		if !d.src.more() {
			errexit("image file is truncated")
		}
	}
}

func (d *decoder) allocPlanes() {
	for ci := range d.comps {
		c := &d.comps[ci]
		c.planeStride = c.widthInBlocks * 8
		c.plane = make([]byte, c.planeStride*c.heightInBlocks*8)
	}
}

// decodeSingleScan is decompress_onepass over every iMCU row, with the
// IDCT output kept in whole-component planes.
func (d *decoder) decodeSingleScan() {
	var storage [dMaxBlocksInMCU][64]int16
	blocks := make([][]int16, dMaxBlocksInMCU)
	for i := range blocks {
		blocks[i] = storage[i][:]
	}
	lastMCUCol := d.mcusPerRow - 1
	lastIMCURow := d.totalIMCURows - 1
	for d.inputIMCURow < d.totalIMCURows {
		for yoffset := 0; yoffset < d.mcuRowsPerIMCURow; yoffset++ {
			for col := 0; col <= lastMCUCol; col++ {
				for i := 0; i < d.blocksInMCU; i++ {
					storage[i] = [64]int16{}
				}
				for {
					if !d.ent.insufficientData {
						d.lastGoodIMCURow = d.inputIMCURow
					}
					if d.ent.decode(blocks[:d.blocksInMCU]) {
						break
					}
					if !d.src.more() {
						errexit("image file is truncated")
					}
					for i := 0; i < d.blocksInMCU; i++ {
						storage[i] = [64]int16{}
					}
				}
				blkn := 0
				for ci := 0; ci < d.compsInScan; ci++ {
					c := d.curComps[ci]
					usefulWidth := c.mcuWidth
					if col == lastMCUCol {
						usefulWidth = c.lastColWidth
					}
					for yindex := 0; yindex < c.mcuHeight; yindex++ {
						if d.inputIMCURow < lastIMCURow || yoffset+yindex < c.lastRowHeight {
							row := d.inputIMCURow*c.v + yoffset + yindex
							for xindex := 0; xindex < usefulWidth; xindex++ {
								bcol := col*c.mcuWidth + xindex
								inverseDCT(storage[blkn+xindex][:], &c.dctTable, c.plane, row*8*c.planeStride+bcol*8, c.planeStride)
							}
						}
						blkn += c.mcuWidth
					}
				}
			}
		}
		d.inputIMCURow++
		if d.inputIMCURow < d.totalIMCURows {
			d.startIMCURow()
		}
	}
	d.finishInputPass()
}

func (d *decoder) allocCoefArrays() {
	for ci := range d.comps {
		c := &d.comps[ci]
		c.coefCols = roundUp(c.widthInBlocks, c.h)
		c.coefRows = roundUp(c.heightInBlocks, c.v)
		c.coef = make([]int16, c.coefCols*c.coefRows*64)
	}
}

// consumeScanData is consume_data over the whole of the current scan.
func (d *decoder) consumeScanData() int {
	blocks := make([][]int16, dMaxBlocksInMCU)
	for d.inputIMCURow < d.totalIMCURows {
		for yoffset := 0; yoffset < d.mcuRowsPerIMCURow; yoffset++ {
			for col := 0; col < d.mcusPerRow; col++ {
				blkn := 0
				for ci := 0; ci < d.compsInScan; ci++ {
					c := d.curComps[ci]
					startCol := col * c.mcuWidth
					for yindex := 0; yindex < c.mcuHeight; yindex++ {
						row := d.inputIMCURow*c.v + yindex + yoffset
						for xindex := 0; xindex < c.mcuWidth; xindex++ {
							at := (row*c.coefCols + startCol + xindex) * 64
							blocks[blkn] = c.coef[at : at+64]
							blkn++
						}
					}
				}
				for {
					if !d.ent.insufficientData {
						d.lastGoodIMCURow = d.inputIMCURow
					}
					if d.ent.decode(blocks[:d.blocksInMCU]) {
						break
					}
					if !d.src.more() {
						errexit("image file is truncated")
					}
				}
			}
		}
		d.inputIMCURow++
		if d.inputIMCURow < d.totalIMCURows {
			d.startIMCURow()
		}
	}
	d.finishInputPass()
	return reachedSOS
}

// outputMultiScan is the output pass of a multi-scan file: decompress_data,
// or decompress_smooth_data when start_output_pass selects smoothing.
func (d *decoder) outputMultiScan() {
	latch, prevLatch, smooth := d.smoothingOK()
	var workspace [64]int16
	for outRow := 0; outRow < d.totalIMCURows; outRow++ {
		for ci := range d.comps {
			c := &d.comps[ci]
			blockRows := c.v
			if outRow == d.totalIMCURows-1 {
				blockRows = c.heightInBlocks % c.v
				if blockRows == 0 {
					blockRows = c.v
				}
			}
			if !smooth {
				for blockRow := 0; blockRow < blockRows; blockRow++ {
					row := outRow*c.v + blockRow
					for col := 0; col < c.widthInBlocks; col++ {
						at := (row*c.coefCols + col) * 64
						inverseDCT(c.coef[at:at+64], &c.dctTable, c.plane, row*8*c.planeStride+col*8, c.planeStride)
					}
				}
				continue
			}
			bits := latch[ci]
			if outRow > d.lastGoodIMCURow {
				bits = prevLatch[ci]
			}
			d.smoothComponentRow(c, outRow, blockRows, bits, &workspace)
		}
	}
}

// smoothingOK is smoothing_ok: it latches the coefficient bit positions
// and reports whether any of the first nine AC coefficients is incomplete.
func (d *decoder) smoothingOK() (latch, prevLatch [][10]int, useful bool) {
	if !d.progressive || d.coefBits == nil {
		return nil, nil, false
	}
	n := d.numComponents
	latch = make([][10]int, n)
	prevLatch = make([][10]int, n)
	for ci := range d.comps {
		q := d.comps[ci].quant
		if q == nil {
			return nil, nil, false
		}
		if q[0] == 0 || q[1] == 0 || q[8] == 0 || q[16] == 0 || q[9] == 0 ||
			q[2] == 0 || q[3] == 0 || q[10] == 0 || q[17] == 0 || q[24] == 0 {
			return nil, nil, false
		}
		bits := d.coefBits[ci]
		prev := d.coefBits[ci+n]
		if bits[0] < 0 {
			return nil, nil, false
		}
		latch[ci][0] = bits[0]
		for coefi := 1; coefi < 10; coefi++ {
			if d.inputScanNumber > 1 {
				prevLatch[ci][coefi] = prev[coefi]
			} else {
				prevLatch[ci][coefi] = -1
			}
			latch[ci][coefi] = bits[coefi]
			if bits[coefi] != 0 {
				useful = true
			}
		}
	}
	return latch, prevLatch, useful
}

func smoothPredict(num, q int64, al int) int16 {
	if num >= 0 {
		pred := int32((q<<7 + num) / (q << 8))
		if al > 0 && pred >= 1<<uint(al) {
			pred = 1<<uint(al) - 1
		}
		return int16(pred)
	}
	pred := int32((q<<7 - num) / (q << 8))
	if al > 0 && pred >= 1<<uint(al) {
		pred = 1<<uint(al) - 1
	}
	return int16(-pred)
}

// smoothComponentRow is the per-component body of decompress_smooth_data,
// including its row bookkeeping, which counts the last iMCU row's image
// block rows with that row's (possibly smaller) block_rows.
func (d *decoder) smoothComponentRow(c *component, outRow, blockRows int, bits [10]int, ws *[64]int16) {
	dc := func(row, col int) int {
		return int(c.coef[(row*c.coefCols+col)*64])
	}
	changeDC := bits[1] == -1 && bits[2] == -1 && bits[3] == -1 && bits[4] == -1 &&
		bits[5] == -1 && bits[6] == -1 && bits[7] == -1 && bits[8] == -1 && bits[9] == -1
	q := c.quant
	q00, q01, q10, q20, q11, q02 := int64(q[0]), int64(q[1]), int64(q[8]), int64(q[16]), int64(q[9]), int64(q[2])
	var q03, q12, q21, q30 int64
	if changeDC {
		q03, q12, q21, q30 = int64(q[3]), int64(q[10]), int64(q[17]), int64(q[24])
	}
	imageBlockRows := blockRows * d.totalIMCURows
	lastBlockColumn := c.widthInBlocks - 1
	for blockRow := 0; blockRow < blockRows; blockRow++ {
		imageBlockRow := outRow*blockRows + blockRow
		cur := outRow*c.v + blockRow
		prev := cur
		if imageBlockRow > 0 {
			prev = cur - 1
		}
		prevPrev := prev
		if imageBlockRow > 1 {
			prevPrev = cur - 2
		}
		next := cur
		if imageBlockRow < imageBlockRows-1 {
			next = cur + 1
		}
		nextNext := next
		if imageBlockRow < imageBlockRows-2 {
			nextNext = cur + 2
		}
		dc01 := dc(prevPrev, 0)
		dc02, dc03, dc04, dc05 := dc01, dc01, dc01, dc01
		dc06 := dc(prev, 0)
		dc07, dc08, dc09, dc10 := dc06, dc06, dc06, dc06
		dc11 := dc(cur, 0)
		dc12, dc13, dc14, dc15 := dc11, dc11, dc11, dc11
		dc16 := dc(next, 0)
		dc17, dc18, dc19, dc20 := dc16, dc16, dc16, dc16
		dc21 := dc(nextNext, 0)
		dc22, dc23, dc24, dc25 := dc21, dc21, dc21, dc21
		for col := 0; col <= lastBlockColumn; col++ {
			at := (cur*c.coefCols + col) * 64
			copy(ws[:], c.coef[at:at+64])
			if col == 0 && col < lastBlockColumn {
				dc04, dc05 = dc(prevPrev, 1), dc(prevPrev, 1)
				dc09, dc10 = dc(prev, 1), dc(prev, 1)
				dc14, dc15 = dc(cur, 1), dc(cur, 1)
				dc19, dc20 = dc(next, 1), dc(next, 1)
				dc24, dc25 = dc(nextNext, 1), dc(nextNext, 1)
			}
			if col+1 < lastBlockColumn {
				dc05 = dc(prevPrev, col+2)
				dc10 = dc(prev, col+2)
				dc15 = dc(cur, col+2)
				dc20 = dc(next, col+2)
				dc25 = dc(nextNext, col+2)
			}
			if al := bits[1]; al != 0 && ws[1] == 0 {
				var sum int
				if changeDC {
					sum = -dc01 - dc02 + dc04 + dc05 - 3*dc06 + 13*dc07 -
						13*dc09 + 3*dc10 - 3*dc11 + 38*dc12 - 38*dc14 +
						3*dc15 - 3*dc16 + 13*dc17 - 13*dc19 + 3*dc20 -
						dc21 - dc22 + dc24 + dc25
				} else {
					sum = -7*dc11 + 50*dc12 - 50*dc14 + 7*dc15
				}
				ws[1] = smoothPredict(q00*int64(int32(sum)), q01, al)
			}
			if al := bits[2]; al != 0 && ws[8] == 0 {
				var sum int
				if changeDC {
					sum = -dc01 - 3*dc02 - 3*dc03 - 3*dc04 - dc05 - dc06 +
						13*dc07 + 38*dc08 + 13*dc09 - dc10 + dc16 -
						13*dc17 - 38*dc18 - 13*dc19 + dc20 + dc21 +
						3*dc22 + 3*dc23 + 3*dc24 + dc25
				} else {
					sum = -7*dc03 + 50*dc08 - 50*dc18 + 7*dc23
				}
				ws[8] = smoothPredict(q00*int64(int32(sum)), q10, al)
			}
			if al := bits[3]; al != 0 && ws[16] == 0 {
				var sum int
				if changeDC {
					sum = dc03 + 2*dc07 + 7*dc08 + 2*dc09 - 5*dc12 - 14*dc13 -
						5*dc14 + 2*dc17 + 7*dc18 + 2*dc19 + dc23
				} else {
					sum = -dc03 + 13*dc08 - 24*dc13 + 13*dc18 - dc23
				}
				ws[16] = smoothPredict(q00*int64(int32(sum)), q20, al)
			}
			if al := bits[4]; al != 0 && ws[9] == 0 {
				var sum int
				if changeDC {
					sum = -dc01 + dc05 + 9*dc07 - 9*dc09 - 9*dc17 +
						9*dc19 + dc21 - dc25
				} else {
					sum = dc10 + dc16 - 10*dc17 + 10*dc19 - dc02 - dc20 + dc22 -
						dc24 + dc04 - dc06 + 10*dc07 - 10*dc09
				}
				ws[9] = smoothPredict(q00*int64(int32(sum)), q11, al)
			}
			if al := bits[5]; al != 0 && ws[2] == 0 {
				var sum int
				if changeDC {
					sum = 2*dc07 - 5*dc08 + 2*dc09 + dc11 + 7*dc12 - 14*dc13 +
						7*dc14 + dc15 + 2*dc17 - 5*dc18 + 2*dc19
				} else {
					sum = -dc11 + 13*dc12 - 24*dc13 + 13*dc14 - dc15
				}
				ws[2] = smoothPredict(q00*int64(int32(sum)), q02, al)
			}
			if changeDC {
				if al := bits[6]; al != 0 && ws[3] == 0 {
					sum := dc07 - dc09 + 2*dc12 - 2*dc14 + dc17 - dc19
					ws[3] = smoothPredict(q00*int64(int32(sum)), q03, al)
				}
				if al := bits[7]; al != 0 && ws[10] == 0 {
					sum := dc07 - 3*dc08 + dc09 - dc17 + 3*dc18 - dc19
					ws[10] = smoothPredict(q00*int64(int32(sum)), q12, al)
				}
				if al := bits[8]; al != 0 && ws[17] == 0 {
					sum := dc07 - dc09 - 3*dc12 + 3*dc14 + dc17 - dc19
					ws[17] = smoothPredict(q00*int64(int32(sum)), q21, al)
				}
				if al := bits[9]; al != 0 && ws[24] == 0 {
					sum := dc07 + 2*dc08 + dc09 - dc17 - 2*dc18 - dc19
					ws[24] = smoothPredict(q00*int64(int32(sum)), q30, al)
				}
				sum := -2*dc01 - 6*dc02 - 8*dc03 - 6*dc04 - 2*dc05 -
					6*dc06 + 6*dc07 + 42*dc08 + 6*dc09 - 6*dc10 -
					8*dc11 + 42*dc12 + 152*dc13 + 42*dc14 - 8*dc15 -
					6*dc16 + 6*dc17 + 42*dc18 + 6*dc19 - 6*dc20 -
					2*dc21 - 6*dc22 - 8*dc23 - 6*dc24 - 2*dc25
				num := q00 * int64(int32(sum))
				if num >= 0 {
					ws[0] = int16(int32((q00<<7 + num) / (q00 << 8)))
				} else {
					ws[0] = int16(-int32((q00<<7 - num) / (q00 << 8)))
				}
			}
			inverseDCT(ws[:], &c.dctTable, c.plane, cur*8*c.planeStride+col*8, c.planeStride)
			dc01, dc02, dc03, dc04 = dc02, dc03, dc04, dc05
			dc06, dc07, dc08, dc09 = dc07, dc08, dc09, dc10
			dc11, dc12, dc13, dc14 = dc12, dc13, dc14, dc15
			dc16, dc17, dc18, dc19 = dc17, dc18, dc19, dc20
			dc21, dc22, dc23, dc24 = dc22, dc23, dc24, dc25
		}
	}
}
