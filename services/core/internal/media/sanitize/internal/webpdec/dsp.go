package webpdec

// C kernels of dsp/dec.c. The SSE2/SSE4.1 kernels libwebp selects on the
// oracle's CPU are meant to be bit-exact with these; the oracle checks it.

const bps = 32

func clip8b(v int) byte {
	if v&^0xff == 0 {
		return byte(v)
	}
	if v < 0 {
		return 0
	}
	return 255
}

func mul1(a int) int { return ((a * 20091) >> 16) + a }
func mul2(a int) int { return (a * 35468) >> 16 }

func store(dst []byte, i, v int) {
	dst[i] = clip8b(int(dst[i]) + (v >> 3))
}

// w16 wraps like _mm_add_epi16 and _mm_sub_epi16.
func w16(v int) int { return int(int16(v)) }

// mulhi16 is _mm_mulhi_epi16 on one lane.
func mulhi16(x, k int) int { return (x * k) >> 16 }

// transformOne is Transform_SSE2 on one 4x4 block (VP8DspInitSSE2 replaces
// TransformTwo_C on x86-64): the same arithmetic as TransformOne_C, but
// every intermediate wraps at 16 bits and the result saturates through
// _mm_packus_epi16. The two only part on out-of-range coefficients, which
// corrupt bitstreams produce. d is the block origin in dst.
func transformOne(in []int16, dst []byte, d int) {
	const k1, k2 = 20091, -30068
	var c [16]int
	for j := 0; j < 4; j++ {
		i0, i1, i2, i3 := int(in[j]), int(in[4+j]), int(in[8+j]), int(in[12+j])
		a := w16(i0 + i2)
		b := w16(i0 - i2)
		cc := w16(w16(i1-i3) + w16(mulhi16(i1, k2)-mulhi16(i3, k1)))
		dd := w16(w16(i1+i3) + w16(mulhi16(i1, k1)+mulhi16(i3, k2)))
		c[4*j+0] = w16(a + dd)
		c[4*j+1] = w16(b + cc)
		c[4*j+2] = w16(b - cc)
		c[4*j+3] = w16(a - dd)
	}
	for m := 0; m < 4; m++ {
		t0, t1, t2, t3 := c[m], c[4+m], c[8+m], c[12+m]
		dc := w16(t0 + 4)
		a := w16(dc + t2)
		b := w16(dc - t2)
		cc := w16(w16(t1-t3) + w16(mulhi16(t1, k2)-mulhi16(t3, k1)))
		dd := w16(w16(t1+t3) + w16(mulhi16(t1, k1)+mulhi16(t3, k2)))
		row := d + m*bps
		for k, v := range [4]int{w16(a + dd), w16(b + cc), w16(b - cc), w16(a - dd)} {
			s := w16(int(dst[row+k]) + (v >> 3))
			if s < 0 {
				s = 0
			} else if s > 255 {
				s = 255
			}
			dst[row+k] = byte(s)
		}
	}
}

// transformTwo is TransformTwo_C.
func transformTwo(in []int16, dst []byte, d int, doTwo bool) {
	transformOne(in, dst, d)
	if doTwo {
		transformOne(in[16:], dst, d+4)
	}
}

// transformAC3 is TransformAC3_C.
func transformAC3(in []int16, dst []byte, d int) {
	a := int(in[0]) + 4
	c4 := mul2(int(in[4]))
	d4 := mul1(int(in[4]))
	c1 := mul2(int(in[1]))
	d1 := mul1(int(in[1]))
	store2 := func(y, dc int) {
		row := d + y*bps
		store(dst, row, dc+d1)
		store(dst, row+1, dc+c1)
		store(dst, row+2, dc-c1)
		store(dst, row+3, dc-d1)
	}
	store2(0, a+d4)
	store2(1, a+c4)
	store2(2, a-c4)
	store2(3, a-d4)
}

// transformDC is TransformDC_C.
func transformDC(in []int16, dst []byte, d int) {
	dc := int(in[0]) + 4
	for j := 0; j < 4; j++ {
		for i := 0; i < 4; i++ {
			store(dst, d+i+j*bps, dc)
		}
	}
}

// transformUV is TransformUV_C.
func transformUV(in []int16, dst []byte, d int) {
	transformTwo(in[0:], dst, d, true)
	transformTwo(in[2*16:], dst, d+4*bps, true)
}

// transformDCUV is TransformDCUV_C.
func transformDCUV(in []int16, dst []byte, d int) {
	if in[0*16] != 0 {
		transformDC(in[0*16:], dst, d)
	}
	if in[1*16] != 0 {
		transformDC(in[1*16:], dst, d+4)
	}
	if in[2*16] != 0 {
		transformDC(in[2*16:], dst, d+4*bps)
	}
	if in[3*16] != 0 {
		transformDC(in[3*16:], dst, d+4*bps+4)
	}
}

// transformWHT is TransformWHT_C.
func transformWHT(in []int16, out []int16) {
	var tmp [16]int
	for i := 0; i < 4; i++ {
		a0 := int(in[0+i]) + int(in[12+i])
		a1 := int(in[4+i]) + int(in[8+i])
		a2 := int(in[4+i]) - int(in[8+i])
		a3 := int(in[0+i]) - int(in[12+i])
		tmp[0+i] = a0 + a1
		tmp[8+i] = a0 - a1
		tmp[4+i] = a3 + a2
		tmp[12+i] = a3 - a2
	}
	o := 0
	for i := 0; i < 4; i++ {
		dc := tmp[0+i*4] + 3
		a0 := dc + tmp[3+i*4]
		a1 := tmp[1+i*4] + tmp[2+i*4]
		a2 := tmp[1+i*4] - tmp[2+i*4]
		a3 := dc - tmp[3+i*4]
		out[o+0] = int16((a0 + a1) >> 3)
		out[o+16] = int16((a3 + a2) >> 3)
		out[o+32] = int16((a0 - a1) >> 3)
		out[o+48] = int16((a3 - a2) >> 3)
		o += 64
	}
}

func clip1(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func sclip1(v int) int {
	if v < -128 {
		return -128
	}
	if v > 127 {
		return 127
	}
	return v
}

func sclip2(v int) int {
	if v < -16 {
		return -16
	}
	if v > 15 {
		return 15
	}
	return v
}

func abs0(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Predictors. b is the 832-byte yuv_b work buffer, d the block origin.

func trueMotion(b []byte, d, size int) {
	top := d - bps
	for y := 0; y < size; y++ {
		left := int(b[d-1]) - int(b[top-1])
		for x := 0; x < size; x++ {
			b[d+x] = byte(clip1(int(b[top+x]) + left))
		}
		d += bps
	}
}

func tm4(b []byte, d int)   { trueMotion(b, d, 4) }
func tm8uv(b []byte, d int) { trueMotion(b, d, 8) }
func tm16(b []byte, d int)  { trueMotion(b, d, 16) }

func ve16(b []byte, d int) {
	for j := 0; j < 16; j++ {
		copy(b[d+j*bps:d+j*bps+16], b[d-bps:d-bps+16])
	}
}

func he16(b []byte, d int) {
	for j := 16; j > 0; j-- {
		v := b[d-1]
		for i := 0; i < 16; i++ {
			b[d+i] = v
		}
		d += bps
	}
}

func put16(v int, b []byte, d int) {
	for j := 0; j < 16; j++ {
		for i := 0; i < 16; i++ {
			b[d+j*bps+i] = byte(v)
		}
	}
}

func dc16(b []byte, d int) {
	dc := 16
	for j := 0; j < 16; j++ {
		dc += int(b[d-1+j*bps]) + int(b[d+j-bps])
	}
	put16(dc>>5, b, d)
}

func dc16NoTop(b []byte, d int) {
	dc := 8
	for j := 0; j < 16; j++ {
		dc += int(b[d-1+j*bps])
	}
	put16(dc>>4, b, d)
}

func dc16NoLeft(b []byte, d int) {
	dc := 8
	for i := 0; i < 16; i++ {
		dc += int(b[d+i-bps])
	}
	put16(dc>>4, b, d)
}

func dc16NoTopLeft(b []byte, d int) { put16(0x80, b, d) }

func avg3(a, b, c int) byte { return byte((a + 2*b + c + 2) >> 2) }
func avg2(a, b int) byte    { return byte((a + b + 1) >> 1) }

func ve4(b []byte, d int) {
	top := d - bps
	vals := [4]byte{
		avg3(int(b[top-1]), int(b[top]), int(b[top+1])),
		avg3(int(b[top]), int(b[top+1]), int(b[top+2])),
		avg3(int(b[top+1]), int(b[top+2]), int(b[top+3])),
		avg3(int(b[top+2]), int(b[top+3]), int(b[top+4])),
	}
	for i := 0; i < 4; i++ {
		copy(b[d+i*bps:d+i*bps+4], vals[:])
	}
}

func he4(b []byte, d int) {
	A := int(b[d-1-bps])
	B := int(b[d-1])
	C := int(b[d-1+bps])
	D := int(b[d-1+2*bps])
	E := int(b[d-1+3*bps])
	fill := func(row int, v byte) {
		for i := 0; i < 4; i++ {
			b[d+row*bps+i] = v
		}
	}
	fill(0, avg3(A, B, C))
	fill(1, avg3(B, C, D))
	fill(2, avg3(C, D, E))
	fill(3, avg3(D, E, E))
}

func dc4(b []byte, d int) {
	dc := 4
	for i := 0; i < 4; i++ {
		dc += int(b[d+i-bps]) + int(b[d-1+i*bps])
	}
	dc >>= 3
	for j := 0; j < 4; j++ {
		for i := 0; i < 4; i++ {
			b[d+j*bps+i] = byte(dc)
		}
	}
}

func rd4(b []byte, d int) {
	I := int(b[d-1+0*bps])
	J := int(b[d-1+1*bps])
	K := int(b[d-1+2*bps])
	L := int(b[d-1+3*bps])
	X := int(b[d-1-bps])
	A := int(b[d+0-bps])
	B := int(b[d+1-bps])
	C := int(b[d+2-bps])
	D := int(b[d+3-bps])
	set := func(x, y int, v byte) { b[d+x+y*bps] = v }
	set(0, 3, avg3(J, K, L))
	v := avg3(I, J, K)
	set(1, 3, v)
	set(0, 2, v)
	v = avg3(X, I, J)
	set(2, 3, v)
	set(1, 2, v)
	set(0, 1, v)
	v = avg3(A, X, I)
	set(3, 3, v)
	set(2, 2, v)
	set(1, 1, v)
	set(0, 0, v)
	v = avg3(B, A, X)
	set(3, 2, v)
	set(2, 1, v)
	set(1, 0, v)
	v = avg3(C, B, A)
	set(3, 1, v)
	set(2, 0, v)
	set(3, 0, avg3(D, C, B))
}

func ld4(b []byte, d int) {
	A := int(b[d+0-bps])
	B := int(b[d+1-bps])
	C := int(b[d+2-bps])
	D := int(b[d+3-bps])
	E := int(b[d+4-bps])
	F := int(b[d+5-bps])
	G := int(b[d+6-bps])
	H := int(b[d+7-bps])
	set := func(x, y int, v byte) { b[d+x+y*bps] = v }
	set(0, 0, avg3(A, B, C))
	v := avg3(B, C, D)
	set(1, 0, v)
	set(0, 1, v)
	v = avg3(C, D, E)
	set(2, 0, v)
	set(1, 1, v)
	set(0, 2, v)
	v = avg3(D, E, F)
	set(3, 0, v)
	set(2, 1, v)
	set(1, 2, v)
	set(0, 3, v)
	v = avg3(E, F, G)
	set(3, 1, v)
	set(2, 2, v)
	set(1, 3, v)
	v = avg3(F, G, H)
	set(3, 2, v)
	set(2, 3, v)
	set(3, 3, avg3(G, H, H))
}

func vr4(b []byte, d int) {
	I := int(b[d-1+0*bps])
	J := int(b[d-1+1*bps])
	K := int(b[d-1+2*bps])
	X := int(b[d-1-bps])
	A := int(b[d+0-bps])
	B := int(b[d+1-bps])
	C := int(b[d+2-bps])
	D := int(b[d+3-bps])
	set := func(x, y int, v byte) { b[d+x+y*bps] = v }
	v := avg2(X, A)
	set(0, 0, v)
	set(1, 2, v)
	v = avg2(A, B)
	set(1, 0, v)
	set(2, 2, v)
	v = avg2(B, C)
	set(2, 0, v)
	set(3, 2, v)
	set(3, 0, avg2(C, D))
	set(0, 3, avg3(K, J, I))
	set(0, 2, avg3(J, I, X))
	v = avg3(I, X, A)
	set(0, 1, v)
	set(1, 3, v)
	v = avg3(X, A, B)
	set(1, 1, v)
	set(2, 3, v)
	v = avg3(A, B, C)
	set(2, 1, v)
	set(3, 3, v)
	set(3, 1, avg3(B, C, D))
}

func vl4(b []byte, d int) {
	A := int(b[d+0-bps])
	B := int(b[d+1-bps])
	C := int(b[d+2-bps])
	D := int(b[d+3-bps])
	E := int(b[d+4-bps])
	F := int(b[d+5-bps])
	G := int(b[d+6-bps])
	H := int(b[d+7-bps])
	set := func(x, y int, v byte) { b[d+x+y*bps] = v }
	set(0, 0, avg2(A, B))
	v := avg2(B, C)
	set(1, 0, v)
	set(0, 2, v)
	v = avg2(C, D)
	set(2, 0, v)
	set(1, 2, v)
	v = avg2(D, E)
	set(3, 0, v)
	set(2, 2, v)
	set(0, 1, avg3(A, B, C))
	v = avg3(B, C, D)
	set(1, 1, v)
	set(0, 3, v)
	v = avg3(C, D, E)
	set(2, 1, v)
	set(1, 3, v)
	v = avg3(D, E, F)
	set(3, 1, v)
	set(2, 3, v)
	set(3, 2, avg3(E, F, G))
	set(3, 3, avg3(F, G, H))
}

func hu4(b []byte, d int) {
	I := int(b[d-1+0*bps])
	J := int(b[d-1+1*bps])
	K := int(b[d-1+2*bps])
	L := int(b[d-1+3*bps])
	set := func(x, y int, v byte) { b[d+x+y*bps] = v }
	set(0, 0, avg2(I, J))
	v := avg2(J, K)
	set(2, 0, v)
	set(0, 1, v)
	v = avg2(K, L)
	set(2, 1, v)
	set(0, 2, v)
	set(1, 0, avg3(I, J, K))
	v = avg3(J, K, L)
	set(3, 0, v)
	set(1, 1, v)
	v = avg3(K, L, L)
	set(3, 1, v)
	set(1, 2, v)
	l := byte(L)
	set(3, 2, l)
	set(2, 2, l)
	set(0, 3, l)
	set(1, 3, l)
	set(2, 3, l)
	set(3, 3, l)
}

func hd4(b []byte, d int) {
	I := int(b[d-1+0*bps])
	J := int(b[d-1+1*bps])
	K := int(b[d-1+2*bps])
	L := int(b[d-1+3*bps])
	X := int(b[d-1-bps])
	A := int(b[d+0-bps])
	B := int(b[d+1-bps])
	C := int(b[d+2-bps])
	set := func(x, y int, v byte) { b[d+x+y*bps] = v }
	v := avg2(I, X)
	set(0, 0, v)
	set(2, 1, v)
	v = avg2(J, I)
	set(0, 1, v)
	set(2, 2, v)
	v = avg2(K, J)
	set(0, 2, v)
	set(2, 3, v)
	set(0, 3, avg2(L, K))
	set(3, 0, avg3(A, B, C))
	set(2, 0, avg3(X, A, B))
	v = avg3(I, X, A)
	set(1, 0, v)
	set(3, 1, v)
	v = avg3(J, I, X)
	set(1, 1, v)
	set(3, 2, v)
	v = avg3(K, J, I)
	set(1, 2, v)
	set(3, 3, v)
	set(1, 3, avg3(L, K, J))
}

func ve8uv(b []byte, d int) {
	for j := 0; j < 8; j++ {
		copy(b[d+j*bps:d+j*bps+8], b[d-bps:d-bps+8])
	}
}

func he8uv(b []byte, d int) {
	for j := 0; j < 8; j++ {
		v := b[d-1]
		for i := 0; i < 8; i++ {
			b[d+i] = v
		}
		d += bps
	}
}

func put8x8uv(v byte, b []byte, d int) {
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			b[d+j*bps+i] = v
		}
	}
}

func dc8uv(b []byte, d int) {
	dc0 := 8
	for i := 0; i < 8; i++ {
		dc0 += int(b[d+i-bps]) + int(b[d-1+i*bps])
	}
	put8x8uv(byte(dc0>>4), b, d)
}

func dc8uvNoLeft(b []byte, d int) {
	dc0 := 4
	for i := 0; i < 8; i++ {
		dc0 += int(b[d+i-bps])
	}
	put8x8uv(byte(dc0>>3), b, d)
}

func dc8uvNoTop(b []byte, d int) {
	dc0 := 4
	for i := 0; i < 8; i++ {
		dc0 += int(b[d-1+i*bps])
	}
	put8x8uv(byte(dc0>>3), b, d)
}

func dc8uvNoTopLeft(b []byte, d int) { put8x8uv(0x80, b, d) }

type predFunc func(b []byte, d int)

var predLuma4 = [10]predFunc{dc4, tm4, ve4, he4, rd4, vr4, ld4, vl4, hd4, hu4}
var predLuma16 = [7]predFunc{dc16, tm16, ve16, he16, dc16NoTop, dc16NoLeft, dc16NoTopLeft}
var predChroma8 = [7]predFunc{dc8uv, tm8uv, ve8uv, he8uv, dc8uvNoTop, dc8uvNoLeft, dc8uvNoTopLeft}

// Loop filters.

func doFilter2(p []byte, o, step int) {
	p1, p0, q0, q1 := int(p[o-2*step]), int(p[o-step]), int(p[o]), int(p[o+step])
	a := 3*(q0-p0) + sclip1(p1-q1)
	a1 := sclip2((a + 4) >> 3)
	a2 := sclip2((a + 3) >> 3)
	p[o-step] = byte(clip1(p0 + a2))
	p[o] = byte(clip1(q0 - a1))
}

func doFilter4(p []byte, o, step int) {
	p1, p0, q0, q1 := int(p[o-2*step]), int(p[o-step]), int(p[o]), int(p[o+step])
	a := 3 * (q0 - p0)
	a1 := sclip2((a + 4) >> 3)
	a2 := sclip2((a + 3) >> 3)
	a3 := (a1 + 1) >> 1
	p[o-2*step] = byte(clip1(p1 + a3))
	p[o-step] = byte(clip1(p0 + a2))
	p[o] = byte(clip1(q0 - a1))
	p[o+step] = byte(clip1(q1 - a3))
}

func doFilter6(p []byte, o, step int) {
	p2, p1, p0 := int(p[o-3*step]), int(p[o-2*step]), int(p[o-step])
	q0, q1, q2 := int(p[o]), int(p[o+step]), int(p[o+2*step])
	a := sclip1(3*(q0-p0) + sclip1(p1-q1))
	a1 := (27*a + 63) >> 7
	a2 := (18*a + 63) >> 7
	a3 := (9*a + 63) >> 7
	p[o-3*step] = byte(clip1(p2 + a3))
	p[o-2*step] = byte(clip1(p1 + a2))
	p[o-step] = byte(clip1(p0 + a1))
	p[o] = byte(clip1(q0 - a1))
	p[o+step] = byte(clip1(q1 - a2))
	p[o+2*step] = byte(clip1(q2 - a3))
}

func hev(p []byte, o, step, thresh int) bool {
	p1, p0, q0, q1 := int(p[o-2*step]), int(p[o-step]), int(p[o]), int(p[o+step])
	return abs0(p1-p0) > thresh || abs0(q1-q0) > thresh
}

func needsFilter(p []byte, o, step, t int) bool {
	p1, p0, q0, q1 := int(p[o-2*step]), int(p[o-step]), int(p[o]), int(p[o+step])
	return 4*abs0(p0-q0)+abs0(p1-q1) <= t
}

func needsFilter2(p []byte, o, step, t, it int) bool {
	p3, p2, p1 := int(p[o-4*step]), int(p[o-3*step]), int(p[o-2*step])
	p0, q0 := int(p[o-step]), int(p[o])
	q1, q2, q3 := int(p[o+step]), int(p[o+2*step]), int(p[o+3*step])
	if 4*abs0(p0-q0)+abs0(p1-q1) > t {
		return false
	}
	return abs0(p3-p2) <= it && abs0(p2-p1) <= it && abs0(p1-p0) <= it &&
		abs0(q3-q2) <= it && abs0(q2-q1) <= it && abs0(q1-q0) <= it
}

func simpleVFilter16(p []byte, o, stride, thresh int) {
	thresh2 := 2*thresh + 1
	for i := 0; i < 16; i++ {
		if needsFilter(p, o+i, stride, thresh2) {
			doFilter2(p, o+i, stride)
		}
	}
}

func simpleHFilter16(p []byte, o, stride, thresh int) {
	thresh2 := 2*thresh + 1
	for i := 0; i < 16; i++ {
		if needsFilter(p, o+i*stride, 1, thresh2) {
			doFilter2(p, o+i*stride, 1)
		}
	}
}

func simpleVFilter16i(p []byte, o, stride, thresh int) {
	for k := 3; k > 0; k-- {
		o += 4 * stride
		simpleVFilter16(p, o, stride, thresh)
	}
}

func simpleHFilter16i(p []byte, o, stride, thresh int) {
	for k := 3; k > 0; k-- {
		o += 4
		simpleHFilter16(p, o, stride, thresh)
	}
}

func filterLoop26(p []byte, o, hstride, vstride, size, thresh, ithresh, hevThresh int) {
	thresh2 := 2*thresh + 1
	for ; size > 0; size-- {
		if needsFilter2(p, o, hstride, thresh2, ithresh) {
			if hev(p, o, hstride, hevThresh) {
				doFilter2(p, o, hstride)
			} else {
				doFilter6(p, o, hstride)
			}
		}
		o += vstride
	}
}

func filterLoop24(p []byte, o, hstride, vstride, size, thresh, ithresh, hevThresh int) {
	thresh2 := 2*thresh + 1
	for ; size > 0; size-- {
		if needsFilter2(p, o, hstride, thresh2, ithresh) {
			if hev(p, o, hstride, hevThresh) {
				doFilter2(p, o, hstride)
			} else {
				doFilter4(p, o, hstride)
			}
		}
		o += vstride
	}
}

func vFilter16(p []byte, o, stride, thresh, ithresh, hevThresh int) {
	filterLoop26(p, o, stride, 1, 16, thresh, ithresh, hevThresh)
}

func hFilter16(p []byte, o, stride, thresh, ithresh, hevThresh int) {
	filterLoop26(p, o, 1, stride, 16, thresh, ithresh, hevThresh)
}

func vFilter16i(p []byte, o, stride, thresh, ithresh, hevThresh int) {
	for k := 3; k > 0; k-- {
		o += 4 * stride
		filterLoop24(p, o, stride, 1, 16, thresh, ithresh, hevThresh)
	}
}

func hFilter16i(p []byte, o, stride, thresh, ithresh, hevThresh int) {
	for k := 3; k > 0; k-- {
		o += 4
		filterLoop24(p, o, 1, stride, 16, thresh, ithresh, hevThresh)
	}
}

func vFilter8(p []byte, u, v, stride, thresh, ithresh, hevThresh int) {
	filterLoop26(p, u, stride, 1, 8, thresh, ithresh, hevThresh)
	filterLoop26(p, v, stride, 1, 8, thresh, ithresh, hevThresh)
}

func hFilter8(p []byte, u, v, stride, thresh, ithresh, hevThresh int) {
	filterLoop26(p, u, 1, stride, 8, thresh, ithresh, hevThresh)
	filterLoop26(p, v, 1, stride, 8, thresh, ithresh, hevThresh)
}

func vFilter8i(p []byte, u, v, stride, thresh, ithresh, hevThresh int) {
	filterLoop24(p, u+4*stride, stride, 1, 8, thresh, ithresh, hevThresh)
	filterLoop24(p, v+4*stride, stride, 1, 8, thresh, ithresh, hevThresh)
}

func hFilter8i(p []byte, u, v, stride, thresh, ithresh, hevThresh int) {
	filterLoop24(p, u+4, 1, stride, 8, thresh, ithresh, hevThresh)
	filterLoop24(p, v+4, 1, stride, 8, thresh, ithresh, hevThresh)
}
