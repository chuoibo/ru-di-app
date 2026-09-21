package pngenc

import "math"

// corpusCase is one generated RGBA image: content kind, size and seed.
type corpusCase struct {
	Class   string `json:"class"`
	Content string `json:"content"`
	W       int    `json:"w"`
	H       int    `json:"h"`
	Seed    uint64 `json:"seed"`
}

type splitmix uint64

func (r *splitmix) next() uint64 {
	*r += 0x9e3779b97f4a7c15
	z := uint64(*r)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *splitmix) intn(n int) int { return int(r.next() % uint64(n)) }

func clamp(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// pixels builds the RGBA samples of a case. Every content is a pure
// function of (content, w, h, seed).
func (c corpusCase) pixels() []byte {
	w, h := c.W, c.H
	r := splitmix(c.Seed*0x100000001b3 + 17)
	pix := make([]byte, 4*w*h)
	set := func(x, y int, cr, cg, cb, ca byte) {
		o := 4 * (y*w + x)
		pix[o], pix[o+1], pix[o+2], pix[o+3] = cr, cg, cb, ca
	}
	switch c.Content {
	case "noise":
		for i := range pix {
			pix[i] = byte(r.next())
		}
	case "flat":
		cr, cg, cb, ca := byte(r.next()), byte(r.next()), byte(r.next()), byte(r.next())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				set(x, y, cr, cg, cb, ca)
			}
		}
	case "gradient":
		// smooth colour ramps with an alpha ramp and a little noise
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				fx := x * 255 / max(w-1, 1)
				fy := y * 255 / max(h-1, 1)
				n := r.intn(5) - 2
				set(x, y, clamp(fx+n), clamp(fy-n), clamp((fx+fy)/2), clamp((x*y)%256))
			}
		}
	case "rows":
		// one random row repeated: the Up filter zeroes every later line
		row := make([]byte, 4*w)
		for i := range row {
			row[i] = byte(r.next())
		}
		for y := 0; y < h; y++ {
			copy(pix[4*y*w:], row)
		}
	case "halfalpha":
		// opaque smooth left half, fully transparent right half with junk RGB
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if x < w/2 {
					set(x, y, clamp(128+int(60*math.Sin(float64(x)/9))), clamp(y%256), 90, 255)
				} else {
					set(x, y, byte(r.next()), byte(r.next()), byte(r.next()), 0)
				}
			}
		}
	case "text":
		// dark glyph-like boxes and strokes on an opaque white page
		for i := range pix {
			pix[i] = 255
		}
		strokes := w * h / 40
		for i := 0; i < strokes; i++ {
			x0, y0 := r.intn(w), r.intn(h)
			bw, bh := 1+r.intn(7), 1+r.intn(9)
			shade := byte(r.intn(60))
			for y := y0; y < min(h, y0+bh); y++ {
				for x := x0; x < min(w, x0+bw); x++ {
					set(x, y, shade, shade, shade, 255)
				}
			}
		}
	case "tiles":
		// a 16x16 random tile repeated, with a few alpha levels
		var tile [16 * 16 * 4]byte
		for i := range tile {
			tile[i] = byte(r.next())
			if i%4 == 3 {
				tile[i] = []byte{0, 128, 255}[r.intn(3)]
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				o := 4 * ((y%16)*16 + x%16)
				set(x, y, tile[o], tile[o+1], tile[o+2], tile[o+3])
			}
		}
	case "photo":
		// low-frequency waves plus sensor-like noise, opaque with a soft vignette alpha
		p1, p2, p3 := float64(r.intn(100)), float64(r.intn(100)), float64(r.intn(100))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				fx, fy := float64(x), float64(y)
				rr := 128 + 70*math.Sin(fx/97+p1) + 40*math.Cos(fy/53+p2)
				gg := 120 + 60*math.Sin((fx+fy)/131+p3)
				bb := 100 + 80*math.Cos(fx/41-fy/67)
				dx := fx/float64(max(w, 1)) - 0.5
				dy := fy/float64(max(h, 1)) - 0.5
				aa := 255 - 400*(dx*dx+dy*dy)
				set(x, y, clamp(int(rr)+r.intn(7)-3), clamp(int(gg)+r.intn(7)-3), clamp(int(bb)+r.intn(7)-3), clamp(int(aa)))
			}
		}
	default:
		panic("unknown content " + c.Content)
	}
	return pix
}
