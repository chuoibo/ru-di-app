package webpdec

// Inverse transforms of dsp/lossless.c and lossless_common.h. Every
// function works on one []uint32 with offsets, like the C pointers.

const argbBlack = 0xff000000

func addPixels(a, b uint32) uint32 {
	ag := (a & 0xff00ff00) + (b & 0xff00ff00)
	rb := (a & 0x00ff00ff) + (b & 0x00ff00ff)
	return (ag & 0xff00ff00) | (rb & 0x00ff00ff)
}

func average2(a0, a1 uint32) uint32 {
	return (((a0 ^ a1) & 0xfefefefe) >> 1) + (a0 & a1)
}

func average3(a0, a1, a2 uint32) uint32 { return average2(average2(a0, a2), a1) }

func average4(a0, a1, a2, a3 uint32) uint32 {
	return average2(average2(a0, a1), average2(a2, a3))
}

func clip255(a uint32) uint32 {
	if a < 256 {
		return a
	}
	return ^a >> 24
}

func addSubtractComponentFull(a, b, c int) uint32 {
	return clip255(uint32(a + b - c))
}

func clampedAddSubtractFull(c0, c1, c2 uint32) uint32 {
	a := addSubtractComponentFull(int(c0>>24), int(c1>>24), int(c2>>24))
	r := addSubtractComponentFull(int((c0>>16)&0xff), int((c1>>16)&0xff), int((c2>>16)&0xff))
	g := addSubtractComponentFull(int((c0>>8)&0xff), int((c1>>8)&0xff), int((c2>>8)&0xff))
	b := addSubtractComponentFull(int(c0&0xff), int(c1&0xff), int(c2&0xff))
	return a<<24 | r<<16 | g<<8 | b
}

func addSubtractComponentHalf(a, b int) uint32 {
	return clip255(uint32(a + (a-b)/2))
}

func clampedAddSubtractHalf(c0, c1, c2 uint32) uint32 {
	ave := average2(c0, c1)
	a := addSubtractComponentHalf(int(ave>>24), int(c2>>24))
	r := addSubtractComponentHalf(int((ave>>16)&0xff), int((c2>>16)&0xff))
	g := addSubtractComponentHalf(int((ave>>8)&0xff), int((c2>>8)&0xff))
	b := addSubtractComponentHalf(int(ave&0xff), int(c2&0xff))
	return a<<24 | r<<16 | g<<8 | b
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sub3(a, b, c int) int {
	return iabs(b-c) - iabs(a-c)
}

func selectPred(a, b, c uint32) uint32 {
	paMinusPb := sub3(int(a>>24), int(b>>24), int(c>>24)) +
		sub3(int((a>>16)&0xff), int((b>>16)&0xff), int((c>>16)&0xff)) +
		sub3(int((a>>8)&0xff), int((b>>8)&0xff), int((c>>8)&0xff)) +
		sub3(int(a&0xff), int(b&0xff), int(c&0xff))
	if paMinusPb <= 0 {
		return a
	}
	return b
}

// predict is VP8LPredictor<mode>_C for mode 2..13; top indexes px.
func predict(mode int, px []uint32, left uint32, top int) uint32 {
	switch mode {
	case 2:
		return px[top]
	case 3:
		return px[top+1]
	case 4:
		return px[top-1]
	case 5:
		return average3(left, px[top], px[top+1])
	case 6:
		return average2(left, px[top-1])
	case 7:
		return average2(left, px[top])
	case 8:
		return average2(px[top-1], px[top])
	case 9:
		return average2(px[top], px[top+1])
	case 10:
		return average4(left, px[top-1], px[top], px[top+1])
	case 11:
		return selectPred(px[top], left, px[top-1])
	case 12:
		return clampedAddSubtractFull(left, px[top], px[top-1])
	default:
		return clampedAddSubtractHalf(left, px[top], px[top-1])
	}
}

// predictorAdd is VP8LPredictorsAdd[mode].
func predictorAdd(mode int, px []uint32, in, upper, n, out int) {
	switch mode {
	case 0, 14, 15:
		for x := 0; x < n; x++ {
			px[out+x] = addPixels(px[in+x], argbBlack)
		}
	case 1:
		left := px[out-1]
		for x := 0; x < n; x++ {
			left = addPixels(px[in+x], left)
			px[out+x] = left
		}
	default:
		for x := 0; x < n; x++ {
			pred := predict(mode, px, px[out+x-1], upper+x)
			px[out+x] = addPixels(px[in+x], pred)
		}
	}
}

func predictorInverseTransform(t *vp8lTransform, yStart, yEnd int, px []uint32, in, out int) {
	width := t.xsize
	if yStart == 0 {
		predictorAdd(0, px, in, 0, 1, out)
		predictorAdd(1, px, in+1, 0, width-1, out+1)
		in += width
		out += width
		yStart++
	}
	y := yStart
	tileWidth := 1 << uint(t.bits)
	mask := tileWidth - 1
	tilesPerRow := subSampleSize(width, t.bits)
	predModeBase := (y >> uint(t.bits)) * tilesPerRow
	for y < yEnd {
		predModeSrc := predModeBase
		x := 1
		predictorAdd(2, px, in, out-width, 1, out)
		for x < width {
			mode := int((t.data[predModeSrc] >> 8) & 0xf)
			predModeSrc++
			xEnd := (x &^ mask) + tileWidth
			if xEnd > width {
				xEnd = width
			}
			predictorAdd(mode, px, in+x, out+x-width, xEnd-x, out+x)
			x = xEnd
		}
		in += width
		out += width
		y++
		if y&mask == 0 {
			predModeBase += tilesPerRow
		}
	}
}

func colorTransformDelta(pred, color int8) int {
	return (int(pred) * int(color)) >> 5
}

func colorSpaceInverseTransform(t *vp8lTransform, yStart, yEnd int, px []uint32, src, dst int) {
	width := t.xsize
	tileWidth := 1 << uint(t.bits)
	mask := tileWidth - 1
	safeWidth := width &^ mask
	remaining := width - safeWidth
	tilesPerRow := subSampleSize(width, t.bits)
	y := yStart
	predRow := (y >> uint(t.bits)) * tilesPerRow
	apply := func(code uint32, n int) {
		g2r := int8(code)
		g2b := int8(code >> 8)
		r2b := int8(code >> 16)
		for i := 0; i < n; i++ {
			argb := px[src+i]
			green := int8(argb >> 8)
			newRed := int((argb >> 16) & 0xff)
			newBlue := int(argb & 0xff)
			newRed += colorTransformDelta(g2r, green)
			newRed &= 0xff
			newBlue += colorTransformDelta(g2b, green)
			newBlue += colorTransformDelta(r2b, int8(newRed))
			newBlue &= 0xff
			px[dst+i] = (argb & 0xff00ff00) | uint32(newRed)<<16 | uint32(newBlue)
		}
		src += n
		dst += n
	}
	for y < yEnd {
		pred := predRow
		srcSafeEnd := src + safeWidth
		srcEnd := src + width
		for src < srcSafeEnd {
			apply(t.data[pred], tileWidth)
			pred++
		}
		if src < srcEnd {
			apply(t.data[pred], remaining)
			pred++
		}
		y++
		if y&mask == 0 {
			predRow += tilesPerRow
		}
	}
}

func colorIndexInverseTransform(t *vp8lTransform, yStart, yEnd int, px []uint32, src, dst int) {
	bitsPerPixel := 8 >> uint(t.bits)
	width := t.xsize
	colorMap := t.data
	if bitsPerPixel < 8 {
		countMask := (1 << uint(t.bits)) - 1
		bitMask := uint32(1<<uint(bitsPerPixel)) - 1
		for y := yStart; y < yEnd; y++ {
			packed := uint32(0)
			for x := 0; x < width; x++ {
				if x&countMask == 0 {
					packed = (px[src] >> 8) & 0xff
					src++
				}
				px[dst] = colorMap[packed&bitMask]
				dst++
				packed >>= uint(bitsPerPixel)
			}
		}
		return
	}
	for y := yStart; y < yEnd; y++ {
		for x := 0; x < width; x++ {
			px[dst] = colorMap[(px[src]>>8)&0xff]
			dst++
			src++
		}
	}
}

// colorIndexInverseTransformAlpha is VP8LColorIndexInverseTransformAlpha.
func colorIndexInverseTransformAlpha(t *vp8lTransform, yStart, yEnd int, src, dst []byte) {
	bitsPerPixel := 8 >> uint(t.bits)
	width := t.xsize
	colorMap := t.data
	s, d := 0, 0
	if bitsPerPixel < 8 {
		countMask := (1 << uint(t.bits)) - 1
		bitMask := uint32(1<<uint(bitsPerPixel)) - 1
		for y := yStart; y < yEnd; y++ {
			packed := uint32(0)
			for x := 0; x < width; x++ {
				if x&countMask == 0 {
					packed = uint32(src[s])
					s++
				}
				dst[d] = byte(colorMap[packed&bitMask] >> 8)
				d++
				packed >>= uint(bitsPerPixel)
			}
		}
		return
	}
	for y := yStart; y < yEnd; y++ {
		for x := 0; x < width; x++ {
			dst[d] = byte(colorMap[src[s]] >> 8)
			d++
			s++
		}
	}
}

// inverseTransform is VP8LInverseTransform.
func inverseTransform(t *vp8lTransform, rowStart, rowEnd int, px []uint32, in, out int) {
	width := t.xsize
	switch t.typ {
	case subtractGreenTransform:
		n := (rowEnd - rowStart) * width
		for i := 0; i < n; i++ {
			argb := px[in+i]
			green := (argb >> 8) & 0xff
			rb := argb & 0x00ff00ff
			rb += green<<16 | green
			rb &= 0x00ff00ff
			px[out+i] = (argb & 0xff00ff00) | rb
		}
	case predictorTransform:
		predictorInverseTransform(t, rowStart, rowEnd, px, in, out)
		if rowEnd != t.ysize {
			last := out + (rowEnd-rowStart-1)*width
			copy(px[out-width:out], px[last:last+width])
		}
	case crossColorTransform:
		colorSpaceInverseTransform(t, rowStart, rowEnd, px, in, out)
	case colorIndexingTransform:
		if in == out && t.bits > 0 {
			outStride := (rowEnd - rowStart) * width
			inStride := (rowEnd - rowStart) * subSampleSize(t.xsize, t.bits)
			src := out + outStride - inStride
			copy(px[src:src+inStride], px[out:out+inStride])
			colorIndexInverseTransform(t, rowStart, rowEnd, px, src, out)
		} else {
			colorIndexInverseTransform(t, rowStart, rowEnd, px, in, out)
		}
	}
}
