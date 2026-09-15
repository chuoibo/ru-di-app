package jpegdec

// Port of jidctint.c jpeg_idct_islow (8-bit samples: CONST_BITS 13,
// PASS1_BITS 2) with the range limit table of prepare_range_limit_table.

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

// idctRangeLimit is IDCT_range_limit(cinfo)[x & RANGE_MASK]: the table
// starting CENTERJSAMPLE entries into sample_range_limit, 1024 entries.
var idctRangeLimit = func() (t [1024]byte) {
	for x := 0; x < 1024; x++ {
		i := 128 + x // index into sample_range_limit
		switch {
		case i < 256:
			t[x] = byte(i)
		case i < 640:
			t[x] = 255
		case i < 1024:
			t[x] = 0
		default:
			t[x] = byte(i - 1024)
		}
	}
	return t
}()

func descale(x int64, n uint) int64 {
	return (x + int64(1)<<(n-1)) >> n
}

// idctIslow decodes one block into out, starting at off with row stride.
func idctIslow(coef []int16, quant *[64]int16, out []byte, off, stride int) {
	var ws [64]int32
	for ctr := 0; ctr < 8; ctr++ {
		if coef[8+ctr] == 0 && coef[16+ctr] == 0 && coef[24+ctr] == 0 &&
			coef[32+ctr] == 0 && coef[40+ctr] == 0 && coef[48+ctr] == 0 &&
			coef[56+ctr] == 0 {
			dcval := int32(uint64(int64(int32(coef[ctr])*int32(quant[ctr]))) << pass1Bits)
			for r := 0; r < 8; r++ {
				ws[r*8+ctr] = dcval
			}
			continue
		}
		deq := func(i int) int64 { return int64(int32(coef[i]) * int32(quant[i])) }
		z2 := deq(16 + ctr)
		z3 := deq(48 + ctr)
		z1 := (z2 + z3) * fix0541
		tmp2 := z1 + z3*-fix1847
		tmp3 := z1 + z2*fix0765
		z2 = deq(ctr)
		z3 = deq(32 + ctr)
		tmp0 := int64(uint64(z2+z3) << constBits)
		tmp1 := int64(uint64(z2-z3) << constBits)
		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2
		tmp0 = deq(56 + ctr)
		tmp1 = deq(40 + ctr)
		tmp2 = deq(24 + ctr)
		tmp3 = deq(8 + ctr)
		z1 = tmp0 + tmp3
		z2 = tmp1 + tmp2
		z3 = tmp0 + tmp2
		z4 := tmp1 + tmp3
		z5 := (z3 + z4) * fix1175
		tmp0 *= fix0298
		tmp1 *= fix2053
		tmp2 *= fix3072
		tmp3 *= fix1501
		z1 *= -fix0899
		z2 *= -fix2562
		z3 *= -fix1961
		z4 *= -fix0390
		z3 += z5
		z4 += z5
		tmp0 += z1 + z3
		tmp1 += z2 + z4
		tmp2 += z2 + z3
		tmp3 += z1 + z4
		ws[0*8+ctr] = int32(descale(tmp10+tmp3, constBits-pass1Bits))
		ws[7*8+ctr] = int32(descale(tmp10-tmp3, constBits-pass1Bits))
		ws[1*8+ctr] = int32(descale(tmp11+tmp2, constBits-pass1Bits))
		ws[6*8+ctr] = int32(descale(tmp11-tmp2, constBits-pass1Bits))
		ws[2*8+ctr] = int32(descale(tmp12+tmp1, constBits-pass1Bits))
		ws[5*8+ctr] = int32(descale(tmp12-tmp1, constBits-pass1Bits))
		ws[3*8+ctr] = int32(descale(tmp13+tmp0, constBits-pass1Bits))
		ws[4*8+ctr] = int32(descale(tmp13-tmp0, constBits-pass1Bits))
	}
	const shift = constBits + pass1Bits + 3
	for ctr := 0; ctr < 8; ctr++ {
		w := ws[ctr*8 : ctr*8+8]
		o := out[off+ctr*stride : off+ctr*stride+8]
		if w[1] == 0 && w[2] == 0 && w[3] == 0 && w[4] == 0 && w[5] == 0 && w[6] == 0 && w[7] == 0 {
			v := idctRangeLimit[int32(descale(int64(w[0]), pass1Bits+3))&1023]
			for i := range o {
				o[i] = v
			}
			continue
		}
		z2 := int64(w[2])
		z3 := int64(w[6])
		z1 := (z2 + z3) * fix0541
		tmp2 := z1 + z3*-fix1847
		tmp3 := z1 + z2*fix0765
		tmp0 := int64(uint64(int64(w[0])+int64(w[4])) << constBits)
		tmp1 := int64(uint64(int64(w[0])-int64(w[4])) << constBits)
		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2
		tmp0 = int64(w[7])
		tmp1 = int64(w[5])
		tmp2 = int64(w[3])
		tmp3 = int64(w[1])
		z1 = tmp0 + tmp3
		z2 = tmp1 + tmp2
		z3 = tmp0 + tmp2
		z4 := tmp1 + tmp3
		z5 := (z3 + z4) * fix1175
		tmp0 *= fix0298
		tmp1 *= fix2053
		tmp2 *= fix3072
		tmp3 *= fix1501
		z1 *= -fix0899
		z2 *= -fix2562
		z3 *= -fix1961
		z4 *= -fix0390
		z3 += z5
		z4 += z5
		tmp0 += z1 + z3
		tmp1 += z2 + z4
		tmp2 += z2 + z3
		tmp3 += z1 + z4
		o[0] = idctRangeLimit[int32(descale(tmp10+tmp3, shift))&1023]
		o[7] = idctRangeLimit[int32(descale(tmp10-tmp3, shift))&1023]
		o[1] = idctRangeLimit[int32(descale(tmp11+tmp2, shift))&1023]
		o[6] = idctRangeLimit[int32(descale(tmp11-tmp2, shift))&1023]
		o[2] = idctRangeLimit[int32(descale(tmp12+tmp1, shift))&1023]
		o[5] = idctRangeLimit[int32(descale(tmp12-tmp1, shift))&1023]
		o[3] = idctRangeLimit[int32(descale(tmp13+tmp0, shift))&1023]
		o[4] = idctRangeLimit[int32(descale(tmp13-tmp0, shift))&1023]
	}
}
