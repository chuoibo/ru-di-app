package areas

import "math"

// This file reproduces, bit for bit, the libm routines that
// app/places/areas.py reaches through CPython's math module and float **:
// glibc 2.41's sin, cos, asin and pow, as selected on x86-64 when the CPU
// has FMA and AVX2 (the *_fma ifunc variants, compiled with -mfma -mavx2).
//
// Go's own math.Sin, math.Cos and math.Asin use different algorithms and
// disagree with glibc in the last bit often enough to move a round(x, 2), a
// 25 km comparison or a sort tie. So the port follows the C sources
// statement by statement, including where GCC contracts a*b+c into a fused
// multiply-add: every such site is an explicit math.FMA, and every product
// GCC leaves alone is wrapped in float64(), which the Go spec says prevents a
// compiler on another architecture from fusing it.
//
// The testdata oracle records Python's own answers for every branch and
// every table interval; the comment on each function names the C file.

func fromBits(bits uint64) float64 { return math.Float64frombits(bits) }

var (
	usS1    = fromBits(cUsncsS1Bits)
	usS2    = fromBits(cUsncsS2Bits)
	usS3    = fromBits(cUsncsS3Bits)
	usS4    = fromBits(cUsncsS4Bits)
	usS5    = fromBits(cUsncsS5Bits)
	usBig   = fromBits(cUsncsBigBits)
	usHp0   = fromBits(cUsncsHp0Bits)
	usHp1   = fromBits(cUsncsHp1Bits)
	usMp1   = fromBits(cUsncsMp1Bits)
	usMp2   = fromBits(cUsncsMp2Bits)
	usPp3   = fromBits(cUsncsPp3Bits)
	usPp4   = fromBits(cUsncsPp4Bits)
	usHpinv = fromBits(cUsncsHpinvBits)
	usToint = fromBits(cUsncsTointBits)

	sinSn3 = fromBits(cSinSn3Bits)
	sinSn5 = fromBits(cSinSn5Bits)
	sinCs2 = fromBits(cSinCs2Bits)
	sinCs4 = fromBits(cSinCs4Bits)
	sinCs6 = fromBits(cSinCs6Bits)

	brT576  = fromBits(cBranredT576Bits)
	brTm600 = fromBits(cBranredTm600Bits)
	brTm24  = fromBits(cBranredTm24Bits)
	brBig   = fromBits(cBranredBigBits)
	brBig1  = fromBits(cBranredBig1Bits)
	brHp0   = fromBits(cBranredHp0Bits)
	brHp1   = fromBits(cBranredHp1Bits)
	brMp1   = fromBits(cBranredMp1Bits)
	brMp2   = fromBits(cBranredMp2Bits)

	asF1  = fromBits(cAsinF1Bits)
	asF2  = fromBits(cAsinF2Bits)
	asF3  = fromBits(cAsinF3Bits)
	asF4  = fromBits(cAsinF4Bits)
	asF5  = fromBits(cAsinF5Bits)
	asF6  = fromBits(cAsinF6Bits)
	asRt0 = fromBits(cAsinRt0Bits)
	asRt1 = fromBits(cAsinRt1Bits)
	asRt2 = fromBits(cAsinRt2Bits)
	asRt3 = fromBits(cAsinRt3Bits)
	asHp0 = fromBits(cAsinHp0Bits)
	asHp1 = fromBits(cAsinHp1Bits)

	powLn2hi     = fromBits(powLn2hiBits)
	powLn2lo     = fromBits(powLn2loBits)
	expInvLn2N   = fromBits(expInvLn2NBits)
	expNegLn2hiN = fromBits(expNegLn2hiNBits)
	expNegLn2loN = fromBits(expNegLn2loNBits)
)

// branredSplit is CN from dla.h, 2^27 + 1.
const branredSplit = 1<<27 + 1

func highWord(x float64) int32 { return int32(math.Float64bits(x) >> 32) }
func lowWord(x float64) int32  { return int32(uint32(math.Float64bits(x))) }

func sincos(k int) float64 { return fromBits(sincostabBits[k]) }

// doCos is s_sin.c do_cos.
func doCos(x, dx float64) float64 {
	if x < 0 {
		dx = -dx
	}
	u := usBig + math.Abs(x)
	x = math.Abs(x) - (u - usBig) + dx
	xx := float64(x * x)
	s := math.FMA(float64(x*xx), math.FMA(xx, sinSn5, sinSn3), x)
	c := float64(xx * math.FMA(xx, math.FMA(xx, sinCs6, sinCs4), sinCs2))
	k := int(lowWord(u)) << 2
	sn, ssn, cs, ccs := sincos(k), sincos(k+1), sincos(k+2), sincos(k+3)
	cor := math.FMA(-sn, s, math.FMA(-cs, c, math.FMA(-s, ssn, ccs)))
	return cs + cor
}

// doSin is s_sin.c do_sin, TAYLOR_SIN included.
func doSin(x, dx float64) float64 {
	xold := x
	if math.Abs(x) < 0.126 {
		xx := float64(x * x)
		p := math.FMA(math.FMA(math.FMA(math.FMA(usS5, xx, usS4), xx, usS3), xx, usS2), xx, usS1)
		t := math.FMA(math.FMA(p, x, -float64(0.5*dx)), xx, dx)
		return x + t
	}
	if x <= 0 {
		dx = -dx
	}
	u := usBig + math.Abs(x)
	x = math.Abs(x) - (u - usBig)
	xx := float64(x * x)
	s := x + math.FMA(float64(x*xx), math.FMA(xx, sinSn5, sinSn3), dx)
	c := math.FMA(x, dx, float64(xx*math.FMA(xx, math.FMA(xx, sinCs6, sinCs4), sinCs2)))
	k := int(lowWord(u)) << 2
	sn, ssn, cs, ccs := sincos(k), sincos(k+1), sincos(k+2), sincos(k+3)
	cor := math.FMA(cs, s, math.FMA(-sn, c, math.FMA(s, ccs, ssn)))
	return math.Copysign(sn+cor, xold)
}

// reduceSinCos is s_sin.c reduce_sincos.
func reduceSinCos(x float64) (a, da float64, n int32) {
	t := math.FMA(x, usHpinv, usToint)
	xn := t - usToint
	n = lowWord(t) & 3
	y := math.FMA(-xn, usMp2, math.FMA(-xn, usMp1, x))
	t2 := math.FMA(-xn, usPp3, y)
	db := math.FMA(-xn, usPp3, y-t2)
	b := math.FMA(-xn, usPp4, t2)
	db += math.FMA(-xn, usPp4, t2-b)
	return b, db, n
}

func doSinCos(a, da float64, n int32) float64 {
	var r float64
	if n&1 != 0 {
		r = doCos(a, da)
	} else {
		r = doSin(a, da)
	}
	if n&2 != 0 {
		return -r
	}
	return r
}

// branredHalf is one of the two identical passes in branred.c.
func branredHalf(x float64) (sum, b, bb float64) {
	k := (highWord(x) >> 20) & 2047
	k = (k - 450) / 24
	if k < 0 {
		k = 0
	}
	gor := math.Float64frombits(math.Float64bits(brT576) - uint64(k*24)<<52)
	var r [6]float64
	for i := 0; i < 6; i++ {
		r[i] = float64(float64(x*branredToverp[int(k)+i]) * gor)
		gor = float64(gor * brTm24)
	}
	for i := 0; i < 3; i++ {
		s := (r[i] + brBig) - brBig
		sum += s
		r[i] -= s
	}
	t := 0.0
	for i := 0; i < 6; i++ {
		t += r[5-i]
	}
	bb = (((((r[0] - t) + r[1]) + r[2]) + r[3]) + r[4]) + r[5]
	s := (t + brBig) - brBig
	sum += s
	t -= s
	b = t + bb
	bb = (t - b) + bb
	s = (sum + brBig1) - brBig1
	sum -= s
	return sum, b, bb
}

// branred is branred.c __branred. That file is not one of the -mfma ifunc
// objects: s_sin-fma.c calls the shared SSE2 build, so nothing here fuses.
func branred(x float64) (a, aa float64, n int32) {
	x = float64(x * brTm600)
	t0 := float64(x * branredSplit)
	x1 := t0 - (t0 - x)
	x2 := x - x1
	sum1, b1, bb1 := branredHalf(x1)
	sum2, b2, bb2 := branredHalf(x2)
	sum := sum1 + sum2
	b := b1 + b2
	var bb float64
	if math.Abs(b1) > math.Abs(b2) {
		bb = (b1 - b) + b2
	} else {
		bb = (b2 - b) + b1
	}
	if b > 0.5 {
		b -= 1.0
		sum += 1.0
	} else if b < -0.5 {
		b += 1.0
		sum -= 1.0
	}
	s := b + (bb + bb1 + bb2)
	t := ((b - s) + bb) + (bb1 + bb2)
	bs := float64(s * branredSplit)
	t1 := bs - (bs - s)
	t2 := s - t1
	b = float64(s * brHp0)
	left := ((float64(t1*brMp1) - b) + float64(t1*brMp2)) + float64(t2*brMp1)
	right := (float64(t2*brMp2) + float64(s*brHp1)) + float64(t*brHp0)
	bb = left + right
	s = b + bb
	t = (b - s) + bb
	return s, t, int32(sum) & 3
}

// libmSin is s_sin.c __sin.
func libmSin(x float64) float64 {
	k := highWord(x) & 0x7fffffff
	switch {
	case k < 0x3e500000:
		return x
	case k < 0x3feb6000:
		return doSin(x, 0)
	case k < 0x400368fd:
		t := usHp0 - math.Abs(x)
		return math.Copysign(doCos(t, usHp1), x)
	case k < 0x419921fb:
		a, da, n := reduceSinCos(x)
		return doSinCos(a, da, n)
	case k < 0x7ff00000:
		a, da, n := branred(x)
		return doSinCos(a, da, n)
	default:
		return x / x
	}
}

// libmCos is s_sin.c __cos.
func libmCos(x float64) float64 {
	k := highWord(x) & 0x7fffffff
	switch {
	case k < 0x3e400000:
		return 1.0
	case k < 0x3feb6000:
		return doCos(x, 0)
	case k < 0x400368fd:
		y := usHp0 - math.Abs(x)
		a := y + usHp1
		da := (y - a) + usHp1
		return doSin(a, da)
	case k < 0x419921fb:
		a, da, n := reduceSinCos(x)
		return doSinCos(a, da, n+1)
	case k < 0x7ff00000:
		a, da, n := branred(x)
		return doSinCos(a, da, n+1)
	default:
		return x / x
	}
}

func asncs(i int) float64 { return fromBits(asncsBits[i]) }

// horner evaluates c[0] + xx*(c[1] + xx*(... + xx*c[last])) with every step
// fused, as GCC compiles the nested asincos polynomials.
func horner(xx float64, c []float64) float64 {
	h := c[len(c)-1]
	for i := len(c) - 2; i >= 0; i-- {
		h = math.FMA(xx, h, c[i])
	}
	return h
}

// asinTable is one table-driven branch of e_asin.c __ieee754_asin: degree is
// the number of polynomial coefficients after the linear one.
func asinTable(x float64, m int32, n, degree int) float64 {
	var xx float64
	if m > 0 {
		xx = x - asncs(n)
	} else {
		xx = -x - asncs(n)
	}
	coeffs := make([]float64, degree)
	for i := range coeffs {
		coeffs[i] = asncs(n + 2 + i)
	}
	p := math.FMA(float64(xx*xx), horner(xx, coeffs), asncs(n+2+degree))
	t := math.FMA(asncs(n+1), xx, p)
	res := asncs(n+3+degree) + t
	if m > 0 {
		return res
	}
	return -res
}

// libmAsin is e_asin.c __ieee754_asin.
func libmAsin(x float64) float64 {
	m := highWord(x)
	k := m & 0x7fffffff
	switch {
	case k < 0x3e500000:
		return x
	case k < 0x3fc00000:
		x2 := float64(x * x)
		p := math.FMA(math.FMA(math.FMA(math.FMA(math.FMA(asF6, x2, asF5), x2, asF4), x2, asF3), x2, asF2), x2, asF1)
		return math.FMA(p, float64(x2*x), x)
	case k < 0x3fe00000:
		var n int
		if k < 0x3fd00000 {
			n = 11 * int((k&0x000fffff)>>15)
		} else {
			n = 11*int((k&0x000fffff)>>14) + 352
		}
		return asinTable(x, m, n, 5)
	case k < 0x3fe80000:
		return asinTable(x, m, 1056+int((k&0x000fe000)>>11)*3, 6)
	case k < 0x3fed8000:
		return asinTable(x, m, 992+int((k&0x000fe000)>>13)*13, 7)
	case k < 0x3fee8000:
		return asinTable(x, m, 884+int((k&0x000fe000)>>13)*14, 8)
	case k < 0x3fef0000:
		return asinTable(x, m, 768+int((k&0x000fe000)>>13)*15, 9)
	case k < 0x3ff00000:
		var z float64
		if m > 0 {
			z = float64(0.5 * (1.0 - x))
		} else {
			z = float64(0.5 * (1.0 + x))
		}
		kz := highWord(z)
		t := float64(fromBits(inrootBits[(kz&0x001fffff)>>14]) * math.Ldexp(1, int(511-(kz>>21))))
		r := math.FMA(-float64(t*t), z, 1.0)
		t = float64(t * math.FMA(r, math.FMA(r, math.FMA(r, asRt3, asRt2), asRt1), asRt0))
		c := float64(t * z)
		d := math.FMA(-float64(0.5*t), c, 1.5)
		y := (c + 0x1p24) - 0x1p24 // t24 in uasncs.h
		cc := math.FMA(-y, y, z) / math.FMA(c, d, y)
		p := float64(math.FMA(math.FMA(math.FMA(math.FMA(math.FMA(asF6, z, asF5), z, asF4), z, asF3), z, asF2), z, asF1) * z)
		cor := math.FMA(-float64((y+cc)+(y+cc)), p, asHp1-(cc+cc))
		res := (asHp0 - (y + y)) + cor
		if m > 0 {
			return res
		}
		return -res
	case k == 0x3ff00000 && lowWord(x) == 0:
		if m > 0 {
			return asHp0
		}
		return -asHp0
	default:
		return (x - x) / (x - x)
	}
}

// errno values glibc reports and CPython inspects.
const (
	errnoNone   = 0
	errnoEDOM   = 33
	errnoERANGE = 34
)

func top12(x float64) uint32 { return uint32(math.Float64bits(x) >> 52) }

// powLogInline is e_pow.c log_inline, FMA branch.
func powLogInline(ix uint64) (y, tail float64) {
	const off = 0x3fe6_9555_0000_0000
	tmp := ix - off
	i := (tmp >> 45) % 128
	k := int64(tmp) >> 52
	iz := ix - (tmp & (0xfff << 52))
	z := math.Float64frombits(iz)
	kd := float64(k)
	invc := fromBits(powLogTabBits[i][0])
	logc := fromBits(powLogTabBits[i][1])
	logctail := fromBits(powLogTabBits[i][2])
	r := math.FMA(z, invc, -1.0)
	t1 := math.FMA(kd, powLn2hi, logc)
	t2 := t1 + r
	lo1 := math.FMA(kd, powLn2lo, logctail)
	lo2 := t1 - t2 + r
	a := func(j int) float64 { return fromBits(powLogPolyBits[j]) }
	ar := float64(a(0) * r)
	ar2 := float64(r * ar)
	ar3 := float64(r * ar2)
	hi := t2 + ar2
	lo3 := math.FMA(ar, r, -ar2)
	lo4 := t2 - hi + ar2
	inner := math.FMA(ar2, math.FMA(ar2, math.FMA(r, a(6), a(5)), math.FMA(r, a(4), a(3))), math.FMA(r, a(2), a(1)))
	lo := math.FMA(ar3, inner, lo1+lo2+lo3+lo4)
	y = hi + lo
	tail = hi - y + lo
	return y, tail
}

// powSpecialCase is e_pow.c specialcase.
func powSpecialCase(tmp float64, sbits, ki uint64) (float64, int) {
	if ki&0x80000000 == 0 {
		sbits -= 1009 << 52
		scale := math.Float64frombits(sbits)
		y := float64(0x1p1009 * math.FMA(scale, tmp, scale))
		if math.IsInf(y, 0) {
			return y, errnoERANGE
		}
		return y, errnoNone
	}
	// Both `scale * tmp` below are one value after GCC's CSE, and one use sits
	// in the inner block, so the product is not fused into either sum.
	sbits += 1022 << 52
	scale := math.Float64frombits(sbits)
	product := float64(scale * tmp)
	y := scale + product
	if math.Abs(y) < 1.0 {
		one := 1.0
		if y < 0.0 {
			one = -1.0
		}
		lo := (scale - y) + product
		hi := one + y
		lo = one - hi + y + lo
		y = (hi + lo) - one
		if y == 0.0 {
			y = math.Float64frombits(sbits & (1 << 63))
		}
	}
	y = float64(0x1p-1022 * y)
	if y == 0 {
		return y, errnoERANGE
	}
	return y, errnoNone
}

// powExpInline is e_pow.c exp_inline, TOINT_INTRINSICS off as on x86-64.
func powExpInline(x, xtail float64, signBias uint64) (float64, int) {
	abstop := top12(x) & 0x7ff
	if abstop-top12(0x1p-54) >= top12(512.0)-top12(0x1p-54) {
		if abstop-top12(0x1p-54) >= 0x80000000 {
			one := 1.0 + x
			if signBias != 0 {
				return -one, errnoNone
			}
			return one, errnoNone
		}
		if abstop >= top12(1024.0) {
			tiny := 0x1p-767
			huge := 0x1p769
			if math.Float64bits(x)>>63 != 0 {
				if signBias != 0 {
					return float64(-tiny * tiny), errnoERANGE
				}
				return float64(tiny * tiny), errnoERANGE
			}
			if signBias != 0 {
				return float64(-huge * huge), errnoERANGE
			}
			return float64(huge * huge), errnoERANGE
		}
		abstop = 0
	}
	const shift = 0x1.8p52
	kd := math.FMA(expInvLn2N, x, shift)
	ki := math.Float64bits(kd)
	kd -= shift
	r := math.FMA(kd, expNegLn2loN, math.FMA(kd, expNegLn2hiN, x))
	r += xtail
	idx := 2 * (ki % 128)
	top := (ki + signBias) << 45
	tail := math.Float64frombits(expTabBits[idx])
	sbits := expTabBits[idx+1] + top
	c := func(j int) float64 { return fromBits(expPolyBits[j]) }
	r2 := float64(r * r)
	tmp := math.FMA(float64(r2*r2), math.FMA(r, c(3), c(2)), math.FMA(r2, math.FMA(r, c(1), c(0)), tail+r))
	if abstop == 0 {
		return powSpecialCase(tmp, sbits, ki)
	}
	scale := math.Float64frombits(sbits)
	return math.FMA(scale, tmp, scale), errnoNone
}

func powCheckInt(iy uint64) int {
	e := int(iy >> 52 & 0x7ff)
	if e < 0x3ff {
		return 0
	}
	if e > 0x3ff+52 {
		return 2
	}
	if iy&((1<<(0x3ff+52-e))-1) != 0 {
		return 0
	}
	if iy&(1<<(0x3ff+52-e)) != 0 {
		return 1
	}
	return 2
}

func powZeroInfNaN(i uint64) bool {
	return 2*i-1 >= 2*math.Float64bits(math.Inf(1))-1
}

// libmPow is e_pow.c __pow. It returns the errno glibc would set.
func libmPow(x, y float64) (float64, int) {
	var signBias uint64
	ix := math.Float64bits(x)
	iy := math.Float64bits(y)
	topx := top12(x)
	topy := top12(y)
	if topx-0x001 >= 0x7ff-0x001 || (topy&0x7ff)-0x3be >= 0x43e-0x3be {
		if powZeroInfNaN(iy) {
			if 2*iy == 0 {
				return 1.0, errnoNone
			}
			if ix == math.Float64bits(1.0) {
				return 1.0, errnoNone
			}
			inf2 := 2 * math.Float64bits(math.Inf(1))
			if 2*ix > inf2 || 2*iy > inf2 {
				return x + y, errnoNone
			}
			if 2*ix == 2*math.Float64bits(1.0) {
				return 1.0, errnoNone
			}
			if (2*ix < 2*math.Float64bits(1.0)) == (iy>>63 == 0) {
				return 0.0, errnoNone
			}
			return float64(y * y), errnoNone
		}
		if powZeroInfNaN(ix) {
			x2 := float64(x * x)
			if ix>>63 != 0 && powCheckInt(iy) == 1 {
				x2 = -x2
				signBias = 1
			}
			if 2*ix == 0 && iy>>63 != 0 {
				if signBias != 0 {
					return math.Inf(-1), errnoERANGE
				}
				return math.Inf(1), errnoERANGE
			}
			if iy>>63 != 0 {
				return 1 / x2, errnoNone
			}
			return x2, errnoNone
		}
		if ix>>63 != 0 {
			yint := powCheckInt(iy)
			if yint == 0 {
				return math.NaN(), errnoEDOM
			}
			if yint == 1 {
				signBias = 0x800 << 7
			}
			ix &= 0x7fff_ffff_ffff_ffff
			topx &= 0x7ff
		}
		if (topy&0x7ff)-0x3be >= 0x43e-0x3be {
			if ix == math.Float64bits(1.0) {
				return 1.0, errnoNone
			}
			if (topy & 0x7ff) < 0x3be {
				if ix > math.Float64bits(1.0) {
					return 1.0 + y, errnoNone
				}
				return 1.0 - y, errnoNone
			}
			if (ix > math.Float64bits(1.0)) == (topy < 0x800) {
				return math.Inf(1), errnoERANGE
			}
			return 0, errnoERANGE
		}
		if topx == 0 {
			ix = math.Float64bits(float64(x * 0x1p52))
			ix &= 0x7fff_ffff_ffff_ffff
			ix -= 52 << 52
		}
	}
	hi, lo := powLogInline(ix)
	ehi := float64(y * hi)
	elo := math.FMA(y, lo, math.FMA(y, hi, -ehi))
	return powExpInline(ehi, elo, signBias)
}
