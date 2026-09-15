package jpegdec

// Ports of jdsample.c (upsampling), jdcolor.c (colour conversion) and the
// Pillow unpackers JpegDecode.c hands rows to. The main controller's
// context-row bookkeeping (jdmainct.c) is expressed as edge clamping: rows
// above a component repeat its first row, rows past its downsampled
// height repeat its last row.

const (
	upFullsize = iota
	upH2V1Fancy
	upH2V1
	upH1V2Fancy
	upH2V2Fancy
	upH2V2
	upInt
)

type upsampler struct {
	method  int
	hExpand int
	vExpand int
}

// selectUpsamplers is jinit_upsampler with do_fancy_upsampling on and
// DCT_scaled_size 8.
func (d *decoder) selectUpsamplers() []upsampler {
	ups := make([]upsampler, len(d.comps))
	for ci := range d.comps {
		c := &d.comps[ci]
		hIn, vIn, hOut, vOut := c.h, c.v, d.maxH, d.maxV
		u := &ups[ci]
		switch {
		case hIn == hOut && vIn == vOut:
			u.method = upFullsize
		case hIn*2 == hOut && vIn == vOut:
			if c.downsampledWidth > 2 {
				u.method = upH2V1Fancy
			} else {
				u.method = upH2V1
			}
		case hIn == hOut && vIn*2 == vOut:
			u.method = upH1V2Fancy
		case hIn*2 == hOut && vIn*2 == vOut:
			if c.downsampledWidth > 2 {
				u.method = upH2V2Fancy
			} else {
				u.method = upH2V2
			}
		case hOut%hIn == 0 && vOut%vIn == 0:
			u.method = upInt
			u.hExpand = hOut / hIn
			u.vExpand = vOut / vIn
		default:
			errexit("JERR_FRACT_SAMPLE_NOTIMPL")
		}
	}
	return ups
}

// checkColorDeconverter is the validation in jinit_color_deconverter.
func (d *decoder) checkColorDeconverter() {
	n := d.numComponents
	switch d.jpegColorSpace {
	case csGrayscale:
		if n != 1 {
			errexit("JERR_BAD_J_COLORSPACE")
		}
	case csRGB, csYCbCr:
		if n != 3 {
			errexit("JERR_BAD_J_COLORSPACE")
		}
	case csCMYK, csYCCK:
		if n != 4 {
			errexit("JERR_BAD_J_COLORSPACE")
		}
	default:
		if n < 1 {
			errexit("JERR_BAD_J_COLORSPACE")
		}
	}
	switch d.outColorSpace {
	case csGrayscale:
		if d.lossless && d.jpegColorSpace != d.outColorSpace {
			errexit("JERR_CONVERSION_NOTIMPL")
		}
		switch d.jpegColorSpace {
		case csGrayscale, csYCbCr, csRGB:
		default:
			errexit("JERR_CONVERSION_NOTIMPL")
		}
	case csExtRGBX:
		if d.lossless && d.jpegColorSpace != csRGB {
			errexit("JERR_CONVERSION_NOTIMPL")
		}
		switch d.jpegColorSpace {
		case csYCbCr, csGrayscale, csRGB:
		default:
			errexit("JERR_CONVERSION_NOTIMPL")
		}
	case csCMYK:
		if d.lossless && d.jpegColorSpace != d.outColorSpace {
			errexit("JERR_CONVERSION_NOTIMPL")
		}
		switch d.jpegColorSpace {
		case csYCCK, csCMYK:
		default:
			errexit("JERR_CONVERSION_NOTIMPL")
		}
	default:
		if d.outColorSpace != d.jpegColorSpace {
			errexit("JERR_CONVERSION_NOTIMPL")
		}
	}
}

// upsampleRow writes output row r of component c (at least imageWidth
// samples) into dst and returns the samples.
func (d *decoder) upsampleRow(c *component, u *upsampler, r int, dst []byte) []byte {
	g := r / d.maxV
	o := r % d.maxV
	stride := c.planeStride
	rowAt := func(row int) []byte { return c.plane[row*stride : row*stride+stride] }
	clamp := func(row int) int {
		if row < 0 {
			return 0
		}
		if row > c.downsampledHeight-1 {
			return c.downsampledHeight - 1
		}
		return row
	}
	dsw := c.downsampledWidth
	switch u.method {
	case upFullsize:
		return rowAt(g*c.v + o)
	case upH2V1Fancy:
		in := rowAt(g*c.v + o)
		out := dst[:2*dsw]
		out[0] = in[0]
		out[1] = byte((int(in[0])*3 + int(in[1]) + 2) >> 2)
		for i := 1; i < dsw-1; i++ {
			v := int(in[i]) * 3
			out[2*i] = byte((v + int(in[i-1]) + 1) >> 2)
			out[2*i+1] = byte((v + int(in[i+1]) + 2) >> 2)
		}
		last := dsw - 1
		out[2*last] = byte((int(in[last])*3 + int(in[last-1]) + 1) >> 2)
		out[2*last+1] = in[last]
		return out
	case upH2V1:
		in := rowAt(g*c.v + o)
		out := dst[:2*dsw]
		for i := 0; i < dsw; i++ {
			out[2*i] = in[i]
			out[2*i+1] = in[i]
		}
		return out
	case upH1V2Fancy:
		row := g*c.v + o/2
		in0 := rowAt(row)
		var in1 []byte
		bias := 1
		if o%2 == 0 {
			in1 = rowAt(clamp(row - 1))
		} else {
			in1 = rowAt(clamp(row + 1))
			bias = 2
		}
		out := dst[:dsw]
		for i := 0; i < dsw; i++ {
			out[i] = byte((int(in0[i])*3 + int(in1[i]) + bias) >> 2)
		}
		return out
	case upH2V2Fancy:
		row := g*c.v + o/2
		in0 := rowAt(row)
		var in1 []byte
		if o%2 == 0 {
			in1 = rowAt(clamp(row - 1))
		} else {
			in1 = rowAt(clamp(row + 1))
		}
		out := dst[:2*dsw]
		this := int(in0[0])*3 + int(in1[0])
		next := int(in0[1])*3 + int(in1[1])
		out[0] = byte((this*4 + 8) >> 4)
		out[1] = byte((this*3 + next + 7) >> 4)
		lastSum := this
		this = next
		for i := 2; i < dsw; i++ {
			next = int(in0[i])*3 + int(in1[i])
			out[2*i-2] = byte((this*3 + lastSum + 8) >> 4)
			out[2*i-1] = byte((this*3 + next + 7) >> 4)
			lastSum = this
			this = next
		}
		out[2*dsw-2] = byte((this*3 + lastSum + 8) >> 4)
		out[2*dsw-1] = byte((this*4 + 7) >> 4)
		return out
	case upH2V2:
		in := rowAt(g*c.v + o/2)
		out := dst[:2*dsw]
		for i := 0; i < dsw; i++ {
			out[2*i] = in[i]
			out[2*i+1] = in[i]
		}
		return out
	default:
		in := rowAt(g*c.v + o/u.vExpand)
		width := d.imageWidth
		n := 0
		for i := 0; n < width; i++ {
			v := in[i]
			for h := 0; h < u.hExpand; h++ {
				dst[n] = v
				n++
			}
		}
		return dst[:n]
	}
}

// yccTables are build_ycc_rgb_table's tables.
var yccCrR, yccCbB [256]int32
var yccCrG, yccCbG [256]int64

func init() {
	fix := func(x float64) int64 { return int64(x*65536 + 0.5) }
	for i := 0; i < 256; i++ {
		x := int64(i - 128)
		yccCrR[i] = int32((fix(1.40200)*x + 32768) >> 16)
		yccCbB[i] = int32((fix(1.77200)*x + 32768) >> 16)
		yccCrG[i] = -fix(0.71414) * x
		yccCbG[i] = -fix(0.34414)*x + 32768
	}
}

// sampleLimit is cinfo->sample_range_limit for the indices colour
// conversion can reach.
func sampleLimit(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// convertOutput upsamples, colour-converts and unpacks every output row
// into the Pillow mode's samples.
func (d *decoder) convertOutput(mode string, ups []upsampler) []byte {
	w, h := d.imageWidth, d.imageHeight
	var bpp int
	switch mode {
	case "L":
		bpp = 1
	case "RGB":
		bpp = 3
	default:
		bpp = 4
	}
	pix := make([]byte, w*h*bpp)
	bufs := make([][]byte, len(d.comps))
	for ci := range bufs {
		bufs[ci] = make([]byte, roundUp(w, d.maxH)+2*d.maxH+16)
	}
	rows := make([][]byte, len(d.comps))
	for r := 0; r < h; r++ {
		for ci := range d.comps {
			rows[ci] = d.upsampleRow(&d.comps[ci], &ups[ci], r, bufs[ci])
		}
		out := pix[r*w*bpp : (r+1)*w*bpp]
		switch d.outColorSpace {
		case csGrayscale:
			copy(out, rows[0][:w])
		case csExtRGBX:
			switch d.jpegColorSpace {
			case csYCbCr:
				y, cb, cr := rows[0], rows[1], rows[2]
				for x := 0; x < w; x++ {
					yy := int(y[x])
					b, rr := cb[x], cr[x]
					out[3*x] = sampleLimit(yy + int(yccCrR[rr]))
					out[3*x+1] = sampleLimit(yy + int((yccCbG[b]+yccCrG[rr])>>16))
					out[3*x+2] = sampleLimit(yy + int(yccCbB[b]))
				}
			case csGrayscale:
				for x := 0; x < w; x++ {
					v := rows[0][x]
					out[3*x], out[3*x+1], out[3*x+2] = v, v, v
				}
			default:
				for x := 0; x < w; x++ {
					out[3*x], out[3*x+1], out[3*x+2] = rows[0][x], rows[1][x], rows[2][x]
				}
			}
		case csCMYK:
			if d.jpegColorSpace == csYCCK {
				y, cb, cr, k := rows[0], rows[1], rows[2], rows[3]
				for x := 0; x < w; x++ {
					yy := int(y[x])
					b, rr := cb[x], cr[x]
					// Pillow's CMYK;I unpacker inverts every byte.
					out[4*x] = ^sampleLimit(255 - (yy + int(yccCrR[rr])))
					out[4*x+1] = ^sampleLimit(255 - (yy + int((yccCbG[b]+yccCrG[rr])>>16)))
					out[4*x+2] = ^sampleLimit(255 - (yy + int(yccCbB[b])))
					out[4*x+3] = ^k[x]
				}
			} else {
				for x := 0; x < w; x++ {
					out[4*x] = ^rows[0][x]
					out[4*x+1] = ^rows[1][x]
					out[4*x+2] = ^rows[2][x]
					out[4*x+3] = ^rows[3][x]
				}
			}
		}
	}
	return pix
}
