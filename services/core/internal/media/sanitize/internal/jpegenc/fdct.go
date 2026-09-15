package jpegenc

import "math/bits"

// jfdctint.c constants for CONST_BITS 13 and PASS1_BITS 2 (8-bit samples):
// fixNNNN is FIX_N_NNN... of the C source, the value scaled by 1 << 13.
const (
	constBits = 13
	pass1Bits = 2

	fix0298 = 2446
	fix0390 = 3196
	fix0541 = 4433
	fix0765 = 6270
	fix0899 = 7373
	fix1175 = 9633
	fix1501 = 12299
	fix1847 = 15137
	fix1961 = 16069
	fix2053 = 16819
	fix2562 = 20995
	fix3072 = 25172
)

func descale(x int64, n uint) int64 { return (x + 1<<(n-1)) >> n }

// fdctIslow is jpeg_fdct_islow. DCTELEM is 16 bits in a SIMD build, so
// every stored result is truncated to int16 as the C assignment does.
func fdctIslow(data *[64]int32) {
	for row := 0; row < 64; row += 8 {
		d := data[row : row+8 : row+8]
		tmp0 := int64(d[0] + d[7])
		tmp7 := int64(d[0] - d[7])
		tmp1 := int64(d[1] + d[6])
		tmp6 := int64(d[1] - d[6])
		tmp2 := int64(d[2] + d[5])
		tmp5 := int64(d[2] - d[5])
		tmp3 := int64(d[3] + d[4])
		tmp4 := int64(d[3] - d[4])

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		d[0] = int32(int16((tmp10 + tmp11) << pass1Bits))
		d[4] = int32(int16((tmp10 - tmp11) << pass1Bits))

		z1 := (tmp12 + tmp13) * fix0541
		d[2] = int32(int16(descale(z1+tmp13*fix0765, constBits-pass1Bits)))
		d[6] = int32(int16(descale(z1+tmp12*(-fix1847), constBits-pass1Bits)))

		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * fix1175

		tmp4 *= fix0298
		tmp5 *= fix2053
		tmp6 *= fix3072
		tmp7 *= fix1501
		z1 *= -fix0899
		z2 *= -fix2562
		z3 *= -fix1961
		z4 *= -fix0390

		z3 += z5
		z4 += z5

		d[7] = int32(int16(descale(tmp4+z1+z3, constBits-pass1Bits)))
		d[5] = int32(int16(descale(tmp5+z2+z4, constBits-pass1Bits)))
		d[3] = int32(int16(descale(tmp6+z2+z3, constBits-pass1Bits)))
		d[1] = int32(int16(descale(tmp7+z1+z4, constBits-pass1Bits)))
	}
	for col := 0; col < 8; col++ {
		tmp0 := int64(data[col] + data[col+56])
		tmp7 := int64(data[col] - data[col+56])
		tmp1 := int64(data[col+8] + data[col+48])
		tmp6 := int64(data[col+8] - data[col+48])
		tmp2 := int64(data[col+16] + data[col+40])
		tmp5 := int64(data[col+16] - data[col+40])
		tmp3 := int64(data[col+24] + data[col+32])
		tmp4 := int64(data[col+24] - data[col+32])

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		data[col] = int32(int16(descale(tmp10+tmp11, pass1Bits)))
		data[col+32] = int32(int16(descale(tmp10-tmp11, pass1Bits)))

		z1 := (tmp12 + tmp13) * fix0541
		data[col+16] = int32(int16(descale(z1+tmp13*fix0765, constBits+pass1Bits)))
		data[col+48] = int32(int16(descale(z1+tmp12*(-fix1847), constBits+pass1Bits)))

		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * fix1175

		tmp4 *= fix0298
		tmp5 *= fix2053
		tmp6 *= fix3072
		tmp7 *= fix1501
		z1 *= -fix0899
		z2 *= -fix2562
		z3 *= -fix1961
		z4 *= -fix0390

		z3 += z5
		z4 += z5

		data[col+56] = int32(int16(descale(tmp4+z1+z3, constBits+pass1Bits)))
		data[col+40] = int32(int16(descale(tmp5+z2+z4, constBits+pass1Bits)))
		data[col+24] = int32(int16(descale(tmp6+z2+z3, constBits+pass1Bits)))
		data[col+8] = int32(int16(descale(tmp7+z1+z4, constBits+pass1Bits)))
	}
}

// divisors is jcdctmgr.c's reciprocal table for one quantization table.
type divisors struct {
	recip [64]uint32
	corr  [64]uint32
	shift [64]uint
}

// newDivisors runs compute_reciprocal(quantval << 3) for every coefficient,
// with the 16-bit DCTELEM of a SIMD build.
func newDivisors(quant *[64]uint16) divisors {
	var d divisors
	for i, q := range quant {
		divisor := uint32(q) << 3
		if divisor <= 1 {
			d.recip[i], d.corr[i], d.shift[i] = 1, 0, 0
			continue
		}
		b := bits.Len32(divisor) - 1
		r := 16 + b
		fq := (uint32(1) << r) / divisor
		fr := (uint32(1) << r) % divisor
		c := divisor / 2
		switch {
		case fr == 0:
			fq >>= 1
			r--
		case fr <= divisor/2:
			c++
		default:
			fq++
		}
		d.recip[i] = uint32(uint16(fq))
		d.corr[i] = uint32(uint16(c))
		d.shift[i] = uint(r - 16)
	}
	return d
}

// quantize is jcdctmgr.c quantize (the arithmetic jsimd_quantize equals:
// abs, add the correction, two unsigned high multiplies, restore the sign).
func quantize(coef []int16, div *divisors, workspace *[64]int32) {
	for i := 0; i < 64; i++ {
		temp := workspace[i]
		if temp < 0 {
			product := (uint32(-temp) + div.corr[i]) * div.recip[i]
			coef[i] = -int16(product >> (div.shift[i] + 16))
		} else {
			product := (uint32(temp) + div.corr[i]) * div.recip[i]
			coef[i] = int16(product >> (div.shift[i] + 16))
		}
	}
}
