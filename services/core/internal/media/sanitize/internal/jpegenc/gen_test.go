package jpegenc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// genCase is a seeded RGB image: every test input is rebuilt from these
// fields, so goldens hold parameters and digests, never pixels.
type genCase struct {
	W       int    `json:"w"`
	H       int    `json:"h"`
	Pattern string `json:"pattern"`
	Seed    uint64 `json:"seed"`
	// Noise, for pattern "mixed", is how many leading pixels (row-major)
	// are noise on top of the "photo" pattern.
	Noise int `json:"noise,omitempty"`
}

func (c genCase) name() string {
	if c.Noise > 0 {
		return fmt.Sprintf("%dx%d-%s-%d-n%d", c.W, c.H, c.Pattern, c.Seed, c.Noise)
	}
	return fmt.Sprintf("%dx%d-%s-%d", c.W, c.H, c.Pattern, c.Seed)
}

// splitmix is SplitMix64, a generator whose sequence no Go release changes.
type splitmix struct{ state uint64 }

func (s *splitmix) next() uint64 {
	s.state += 0x9E3779B97F4A7C15
	z := s.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (s *splitmix) intn(n int) int { return int(s.next() % uint64(n)) }

func clamp8(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// pixels builds the case's RGB bytes.
func (c genCase) pixels() []byte {
	w, h := c.W, c.H
	pix := make([]byte, w*h*3)
	rng := &splitmix{state: c.Seed}
	switch c.Pattern {
	case "noise":
		for i := range pix {
			pix[i] = byte(rng.next())
		}
	case "flat":
		r, g, b := byte(rng.next()), byte(rng.next()), byte(rng.next())
		for i := 0; i < len(pix); i += 3 {
			pix[i], pix[i+1], pix[i+2] = r, g, b
		}
	case "black":
	case "white":
		for i := range pix {
			pix[i] = 255
		}
	case "gradient":
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 3
				pix[i] = byte(x * 255 / max(w-1, 1))
				pix[i+1] = byte(y * 255 / max(h-1, 1))
				pix[i+2] = byte((x + y) * 255 / max(w+h-2, 1))
			}
		}
	case "saturated":
		for x := 0; x < w; {
			stripe := 1 + rng.intn(9)
			color := rng.intn(8)
			for xx := x; xx < min(x+stripe, w); xx++ {
				for y := 0; y < h; y++ {
					i := (y*w + xx) * 3
					pix[i] = byte(color & 1 * 255)
					pix[i+1] = byte(color >> 1 & 1 * 255)
					pix[i+2] = byte(color >> 2 & 1 * 255)
				}
			}
			x += stripe
		}
	case "edges":
		for i := range pix {
			pix[i] = 255
		}
		strokes := w*h/40 + 3
		for k := 0; k < strokes; k++ {
			x0, y0 := rng.intn(w), rng.intn(h)
			length := 1 + rng.intn(max(max(w, h)/2, 1))
			thick := 1 + rng.intn(2)
			horizontal := rng.intn(2) == 0
			shade := byte(rng.intn(64))
			for step := 0; step < length; step++ {
				for t := 0; t < thick; t++ {
					x, y := x0+step, y0+t
					if !horizontal {
						x, y = x0+t, y0+step
					}
					if x < w && y < h {
						i := (y*w + x) * 3
						pix[i], pix[i+1], pix[i+2] = shade, shade, shade
					}
				}
			}
		}
	case "photo", "mixed":
		r0, g0, b0 := rng.intn(256), rng.intn(256), rng.intn(256)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 3
				wave := (x + 2*y) % 64
				if wave > 32 {
					wave = 64 - wave
				}
				pix[i] = clamp8(r0 + x*97/max(w, 1) - y*53/max(h, 1) + wave + rng.intn(9) - 4)
				pix[i+1] = clamp8(g0 - x*41/max(w, 1) + y*71/max(h, 1) - wave/2 + rng.intn(9) - 4)
				pix[i+2] = clamp8(b0 + (x+y)*29/max(w+h, 1) + wave/3 + rng.intn(9) - 4)
			}
		}
		for i := 0; i < c.Noise && i < w*h; i++ {
			pix[3*i], pix[3*i+1], pix[3*i+2] = byte(rng.next()), byte(rng.next()), byte(rng.next())
		}
	default:
		panic("unknown pattern " + c.Pattern)
	}
	return pix
}

func sha16(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// unlimited is the sanitizer configuration without Pillow's buffer bound,
// used to measure output sizes near that bound.
func unlimited(w, h int) Options {
	opts := SanitizerOptions(w, h)
	opts.BufferSize = 0
	return opts
}
