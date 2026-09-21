package jpegenc

// The rgb_ycc_convert tables (jccolor.c rgb_ycc_start): SCALEBITS 16 fixed
// point with FIX(x) = (JLONG)(x * 65536 + 0.5).
const (
	scaleBits  = 16
	cbcrOffset = 128 << scaleBits
	oneHalf    = 1 << (scaleBits - 1)
)

var rYTab, gYTab, bYTab, rCbTab, gCbTab, bCbTab, gCrTab, bCrTab [256]int32

func fix(x float64) int32 { return int32(x*(1<<scaleBits) + 0.5) }

func init() {
	for i := int32(0); i < 256; i++ {
		rYTab[i] = fix(0.29900) * i
		gYTab[i] = fix(0.58700) * i
		bYTab[i] = fix(0.11400)*i + oneHalf
		rCbTab[i] = -fix(0.16874) * i
		gCbTab[i] = -fix(0.33126) * i
		// B_CB_OFF and R_CR_OFF share one entry in the C table.
		bCbTab[i] = fix(0.50000)*i + cbcrOffset + oneHalf - 1
		gCrTab[i] = -fix(0.41869) * i
		bCrTab[i] = -fix(0.08131) * i
	}
}

// firstPass runs jpeg_write_scanlines for every row and then
// compress_first_pass for every iMCU row, filling each component's
// coefficient buffer.
func (e *encoder) firstPass(pix []byte) {
	width := e.opts.Width
	nc := len(e.comps)
	colorBuf := make([][]byte, nc)
	mainBuf := make([][]byte, nc)
	for ci := range e.comps {
		c := &e.comps[ci]
		colorBuf[ci] = make([]byte, c.colorStride*e.maxV)
		mainBuf[ci] = make([]byte, c.stride*c.v*dctSize)
		c.coef = make([]int16, c.blocksWide*c.blocksHigh*64)
	}
	y := 0
	rowsToGo := e.opts.Height
	for imcu := 0; imcu < e.totalIMCURows; imcu++ {
		// jcmainct.c process_data_simple_main with jcprepct.c
		// pre_process_data: DCTSIZE row groups of max_v_samp_factor rows.
		for group := 0; group < dctSize; {
			next := 0
			for next < e.maxV && rowsToGo > 0 {
				e.convertRow(pix, y, colorBuf, next)
				y++
				next++
				rowsToGo--
			}
			if rowsToGo == 0 && next < e.maxV {
				for ci := range e.comps {
					stride := e.comps[ci].colorStride
					src := colorBuf[ci][(next-1)*stride : (next-1)*stride+width]
					for row := next; row < e.maxV; row++ {
						copy(colorBuf[ci][row*stride:row*stride+width], src)
					}
				}
			}
			for ci := range e.comps {
				e.downsample(ci, colorBuf[ci], mainBuf[ci], group*e.comps[ci].v)
			}
			group++
			if rowsToGo == 0 && group < dctSize {
				for ci := range e.comps {
					c := &e.comps[ci]
					last := group*c.v - 1
					src := mainBuf[ci][last*c.stride : (last+1)*c.stride]
					for row := group * c.v; row < dctSize*c.v; row++ {
						copy(mainBuf[ci][row*c.stride:(row+1)*c.stride], src)
					}
				}
				group = dctSize
			}
		}
		e.dctIMCURow(imcu, mainBuf)
	}
}

// convertRow color-converts input row y into row `row` of the color buffer.
func (e *encoder) convertRow(pix []byte, y int, colorBuf [][]byte, row int) {
	width := e.opts.Width
	switch e.opts.Color {
	case Gray:
		stride := e.comps[0].colorStride
		copy(colorBuf[0][row*stride:row*stride+width], pix[y*width:(y+1)*width])
	case YCbCr:
		in := pix[y*width*3 : (y+1)*width*3]
		out0 := colorBuf[0][row*e.comps[0].colorStride:]
		out1 := colorBuf[1][row*e.comps[1].colorStride:]
		out2 := colorBuf[2][row*e.comps[2].colorStride:]
		for x := 0; x < width; x++ {
			r, g, b := in[3*x], in[3*x+1], in[3*x+2]
			out0[x] = byte((rYTab[r] + gYTab[g] + bYTab[b]) >> scaleBits)
			out1[x] = byte((rCbTab[r] + gCbTab[g] + bCbTab[b]) >> scaleBits)
			out2[x] = byte((bCbTab[r] + gCrTab[g] + bCrTab[b]) >> scaleBits)
		}
	case CMYK:
		in := pix[y*width*4 : (y+1)*width*4]
		for ci := 0; ci < 4; ci++ {
			out := colorBuf[ci][row*e.comps[ci].colorStride:]
			for x := 0; x < width; x++ {
				value := in[4*x+ci]
				if e.opts.InvertCMYK {
					value = 255 - value
				}
				out[x] = value
			}
		}
	}
}

// expandRightEdge is jcsample.c expand_right_edge for one row.
func expandRightEdge(row []byte, inputCols, outputCols int) {
	if outputCols > inputCols {
		value := row[inputCols-1]
		for x := inputCols; x < outputCols; x++ {
			row[x] = value
		}
	}
}

// downsample fills the component's rows outRow.. of the main buffer from
// max_v_samp_factor rows of the color buffer (jcsample.c).
func (e *encoder) downsample(ci int, in, out []byte, outRow int) {
	c := &e.comps[ci]
	width := e.opts.Width
	outCols := c.stride
	inStride := c.colorStride
	switch c.method {
	case fullsizeDownsample:
		for r := 0; r < e.maxV; r++ {
			dst := out[(outRow+r)*outCols : (outRow+r+1)*outCols]
			copy(dst[:width], in[r*inStride:r*inStride+width])
			expandRightEdge(dst, width, outCols)
		}
	case h2v1Downsample:
		for r := 0; r < e.maxV; r++ {
			expandRightEdge(in[r*inStride:(r+1)*inStride], width, outCols*2)
		}
		for r := 0; r < c.v; r++ {
			src := in[r*inStride:]
			dst := out[(outRow+r)*outCols:]
			bias := 0
			for x := 0; x < outCols; x++ {
				dst[x] = byte((int(src[2*x]) + int(src[2*x+1]) + bias) >> 1)
				bias ^= 1
			}
		}
	case h2v2Downsample:
		for r := 0; r < e.maxV; r++ {
			expandRightEdge(in[r*inStride:(r+1)*inStride], width, outCols*2)
		}
		for r := 0; r < c.v; r++ {
			src0 := in[2*r*inStride:]
			src1 := in[(2*r+1)*inStride:]
			dst := out[(outRow+r)*outCols:]
			bias := 1
			for x := 0; x < outCols; x++ {
				dst[x] = byte((int(src0[2*x]) + int(src0[2*x+1]) + int(src1[2*x]) + int(src1[2*x+1]) + bias) >> 2)
				bias ^= 3
			}
		}
	case intDownsample:
		hExpand := e.maxH / c.h
		vExpand := e.maxV / c.v
		numpix := hExpand * vExpand
		for r := 0; r < e.maxV; r++ {
			expandRightEdge(in[r*inStride:(r+1)*inStride], width, outCols*hExpand)
		}
		inRow := 0
		for r := 0; r < c.v; r++ {
			dst := out[(outRow+r)*outCols:]
			for x := 0; x < outCols; x++ {
				sum := 0
				for vv := 0; vv < vExpand; vv++ {
					src := in[(inRow+vv)*inStride+x*hExpand:]
					for hh := 0; hh < hExpand; hh++ {
						sum += int(src[hh])
					}
				}
				dst[x] = byte((sum + numpix/2) / numpix)
			}
			inRow += vExpand
		}
	}
}

// dctIMCURow is compress_first_pass for one iMCU row: forward DCT of the
// real blocks, then the right-edge and bottom dummy blocks, which are zero
// with the DC of their left or upper neighbour.
func (e *encoder) dctIMCURow(imcu int, mainBuf [][]byte) {
	var workspace [64]int32
	last := imcu == e.totalIMCURows-1
	for ci := range e.comps {
		c := &e.comps[ci]
		div := &e.div[c.tbl]
		blockRows := c.v
		if last {
			blockRows = c.heightInBlocks % c.v
			if blockRows == 0 {
				blockRows = c.v
			}
		}
		ndummy := c.widthInBlocks % c.h
		if ndummy > 0 {
			ndummy = c.h - ndummy
		}
		buf := mainBuf[ci]
		for br := 0; br < blockRows; br++ {
			rowBase := (imcu*c.v + br) * c.blocksWide
			for bx := 0; bx < c.widthInBlocks; bx++ {
				for yy := 0; yy < dctSize; yy++ {
					src := buf[(br*dctSize+yy)*c.stride+bx*dctSize:]
					for xx := 0; xx < dctSize; xx++ {
						workspace[yy*dctSize+xx] = int32(src[xx]) - 128
					}
				}
				fdctIslow(&workspace)
				quantize(c.coef[(rowBase+bx)*64:(rowBase+bx+1)*64], div, &workspace)
			}
			if ndummy > 0 {
				lastDC := c.coef[(rowBase+c.widthInBlocks-1)*64]
				for bi := 0; bi < ndummy; bi++ {
					block := c.coef[(rowBase+c.widthInBlocks+bi)*64 : (rowBase+c.widthInBlocks+bi+1)*64]
					clear(block)
					block[0] = lastDC
				}
			}
		}
		if last {
			mcusAcross := (c.widthInBlocks + ndummy) / c.h
			for br := blockRows; br < c.v; br++ {
				rowBase := (imcu*c.v + br) * c.blocksWide
				aboveBase := (imcu*c.v + br - 1) * c.blocksWide
				clear(c.coef[rowBase*64 : (rowBase+c.blocksWide)*64])
				for m := 0; m < mcusAcross; m++ {
					lastDC := c.coef[(aboveBase+m*c.h+c.h-1)*64]
					for bi := 0; bi < c.h; bi++ {
						c.coef[(rowBase+m*c.h+bi)*64] = lastDC
					}
				}
			}
		}
	}
}
