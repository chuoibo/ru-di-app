package jpegdec

import (
	"math/rand/v2"
	"testing"
)

// TestIDCTKernelsAgreeOnEncoderOutput shows that the AVX2 arithmetic the
// decoder uses equals jidctint.c on coefficients an encoder derives from
// real samples: the dequantized values and every intermediate stay inside
// the 16-bit lanes and the output inside the range-limit window. The two
// only part on out-of-range (corrupt or crafted) data, where Pillow
// follows AVX2.
func TestIDCTKernelsAgreeOnEncoderOutput(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	outC := make([]byte, 64)
	outSIMD := make([]byte, 64)
	for trial := 0; trial < 20000; trial++ {
		var quant [64]int16
		scale := 1 + rng.IntN(40)
		for i := range quant {
			quant[i] = int16(1 + rng.IntN(scale))
		}
		var data [64]int64
		base := rng.IntN(256)
		spread := rng.IntN(256)
		for i := range data {
			v := base + rng.IntN(spread+1) - spread/2
			data[i] = int64(max(0, min(255, v))) - 128
		}
		fdct(&data)
		coef := make([]int16, 64)
		for i := range data {
			div := int64(quant[i]) * 8
			v := data[i]
			if v < 0 {
				v = -((-v + div/2) / div)
			} else {
				v = (v + div/2) / div
			}
			coef[i] = int16(v)
		}
		idctIslow(coef, &quant, outC, 0, 8)
		idctIslowAVX2(coef, &quant, outSIMD, 0, 8)
		for i := range outC {
			if outC[i] != outSIMD[i] {
				t.Fatalf("trial %d sample %d: C %d, AVX2 %d (coefficients %v)", trial, i, outC[i], outSIMD[i], coef)
			}
		}
	}
}

// TestIDCTKernelsPartOutOfRange pins the divergence the oracle measured: a
// dequantized DC past 16 bits wraps in the AVX2 lanes.
func TestIDCTKernelsPartOutOfRange(t *testing.T) {
	var quant [64]int16
	for i := range quant {
		quant[i] = 200
	}
	coef := make([]int16, 64)
	coef[0] = 700
	outC := make([]byte, 64)
	outSIMD := make([]byte, 64)
	idctIslow(coef, &quant, outC, 0, 8)
	idctIslowAVX2(coef, &quant, outSIMD, 0, 8)
	if outC[0] == outSIMD[0] {
		t.Fatalf("expected the kernels to differ, both gave %d", outC[0])
	}
}
