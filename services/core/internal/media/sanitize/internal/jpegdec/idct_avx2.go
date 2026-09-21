package jpegdec

// inverseDCT is the IDCT the decoder runs: the kernel the parity image's
// libjpeg-turbo selects on an AVX2 CPU.
var inverseDCT = idctIslowAVX2

// idctIslowAVX2 reproduces the arithmetic of libjpeg-turbo's
// jsimd_idct_islow_avx2, which the parity image runs (its CPU has AVX2),
// instead of jidctint.c. The two agree for coefficients a real encoder
// writes; they part where the kernel works in 16-bit lanes:
//
//   - dequantization keeps the low 16 bits of coef*quant (vpmullw);
//   - the DC-only shortcut applies to the whole block (every coefficient
//     outside row 0 zero), shifting the 16-bit product left by PASS1_BITS;
//   - in0+in4, in0-in4, in7+in3 and in5+in1 wrap at 16 bits (vpaddw);
//   - each pass saturates its results to 16 bits (vpackssdw), and the
//     output saturates to a signed byte before adding CENTERJSAMPLE
//     (vpacksswb, vpaddb) instead of indexing the range-limit table.
func idctIslowAVX2(coef []int16, quant *[64]int16, out []byte, off, stride int) {
	var ws [64]int16
	var deq [64]int16
	for i := range deq {
		deq[i] = int16(int32(coef[i]) * int32(quant[i]))
	}
	acZero := true
	for i := 8; i < 64; i++ {
		if coef[i] != 0 {
			acZero = false
			break
		}
	}
	var line, result [8]int16
	if acZero {
		for col := 0; col < 8; col++ {
			v := int16(uint16(deq[col]) << pass1Bits)
			for row := 0; row < 8; row++ {
				ws[row*8+col] = v
			}
		}
	} else {
		for col := 0; col < 8; col++ {
			for row := 0; row < 8; row++ {
				line[row] = deq[row*8+col]
			}
			avx2Pass(&line, &result, constBits-pass1Bits)
			for row := 0; row < 8; row++ {
				ws[row*8+col] = result[row]
			}
		}
	}
	for row := 0; row < 8; row++ {
		copy(line[:], ws[row*8:row*8+8])
		avx2Pass(&line, &result, constBits+pass1Bits+3)
		o := out[off+row*stride : off+row*stride+8]
		for col, v := range result {
			if v < -128 {
				v = -128
			} else if v > 127 {
				v = 127
			}
			o[col] = byte(int8(v)) + 128
		}
	}
}

func saturate16(v int32) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}

// avx2Pass is one DODCT macro application on eight 16-bit inputs.
func avx2Pass(in, out *[8]int16, n uint) {
	r := func(i int) int32 { return int32(in[i]) }
	tmp3 := r(2)*(fix0541+fix0765) + r(6)*fix0541
	tmp2 := r(6)*(fix0541-fix1847) + r(2)*fix0541
	tmp0 := int32(in[0]+in[4]) << constBits
	tmp1 := int32(in[0]-in[4]) << constBits
	tmp13 := tmp0 - tmp3
	tmp10 := tmp0 + tmp3
	tmp12 := tmp1 - tmp2
	tmp11 := tmp1 + tmp2
	sumZ3 := int32(in[7] + in[3])
	sumZ4 := int32(in[5] + in[1])
	z3 := sumZ3*(fix1175-fix1961) + sumZ4*fix1175
	z4 := sumZ4*(fix1175-fix0390) + sumZ3*fix1175
	t0 := r(7)*(fix0298-fix0899) + r(1)*-fix0899 + z3
	t1 := r(5)*(fix2053-fix2562) + r(3)*-fix2562 + z4
	t3 := r(7)*-fix0899 + r(1)*(fix1501-fix0899) + z4
	t2 := r(5)*-fix2562 + r(3)*(fix3072-fix2562) + z3
	round := int32(1) << (n - 1)
	d := func(x int32) int16 { return saturate16((x + round) >> n) }
	out[0] = d(tmp10 + t3)
	out[7] = d(tmp10 - t3)
	out[1] = d(tmp11 + t2)
	out[6] = d(tmp11 - t2)
	out[3] = d(tmp13 + t0)
	out[4] = d(tmp13 - t0)
	out[2] = d(tmp12 + t1)
	out[5] = d(tmp12 - t1)
}
