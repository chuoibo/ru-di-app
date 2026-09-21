package convert_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"

	"mobile/services/core/internal/media/sanitize/internal/convert"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
)

// convertCase is an in-memory Pillow image: what Image.frombytes plus
// putpalette and info["transparency"] would build.
type convertCase struct {
	name string
	img  *pil.Image
	// strTransparency is a str info value, which pil.Info cannot carry;
	// Go expects the error Pillow raises for it (P and L only).
	strTransparency string
}

func seededPix(r *pt.Rand, mode string, n int) []byte {
	pix := r.Bytes(n * pil.BytesPerPixel(mode))
	switch mode {
	case "1":
		for i := range pix {
			if pix[i]&1 == 1 {
				pix[i] = 255
			} else {
				pix[i] = 0
			}
		}
	case "I":
		specials := []int32{-1, 0, 1, 254, 255, 256, math.MaxInt32, math.MinInt32, 1000}
		for i := 0; i < n; i++ {
			if i%3 == 0 {
				binary.LittleEndian.PutUint32(pix[4*i:], uint32(specials[(i/3)%len(specials)]))
			}
		}
	case "F":
		specials := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Copysign(0, -1)),
			254.9, 255, 0.5, -3, 1e10, 128.75}
		for i := 0; i < n; i++ {
			v := specials[i%len(specials)]
			if i%2 == 1 {
				v = float32(r.Intn(3000))/10 - 20
			}
			binary.LittleEndian.PutUint32(pix[4*i:], math.Float32bits(v))
		}
	case "I;16", "I;16L", "I;16B", "I;16N":
		for i := 0; i < n; i++ {
			if i%2 == 0 {
				pix[2*i+1] = 0
			}
		}
	}
	return pix
}

func corpus() []convertCase {
	r := pt.NewRand(0xc0de)
	var cases []convertCase
	add := func(name string, img *pil.Image) {
		cases = append(cases, convertCase{name: name, img: img})
	}
	modes := []string{"1", "L", "LA", "La", "P", "PA", "RGB", "RGBA", "RGBa", "RGBX", "CMYK",
		"YCbCr", "HSV", "LAB", "I", "I;16", "I;16L", "I;16B", "I;16N", "F"}
	for _, mode := range modes {
		for _, size := range [][2]int{{1, 1}, {16, 16}, {37, 3}} {
			n := size[0] * size[1]
			img := &pil.Image{Mode: mode, Width: size[0], Height: size[1], Pix: seededPix(r, mode, n)}
			if mode == "P" || mode == "PA" {
				img.PaletteMode, img.Palette = "RGB", r.Bytes(3*(1+r.Intn(256)))
			}
			add(fmt.Sprintf("mode/%s-%dx%d", mode, size[0], size[1]), img)
		}
	}
	hsv := &pil.Image{Mode: "HSV", Width: 256, Height: 3, Pix: make([]byte, 256*3*3)}
	for x := 0; x < 256; x++ {
		for y := 0; y < 3; y++ {
			i := 3 * (y*256 + x)
			hsv.Pix[i], hsv.Pix[i+1], hsv.Pix[i+2] = byte(x), byte([]int{255, 128, 1}[y]), byte(255-x)
		}
	}
	add("mode/HSV-sweep", hsv)

	palette := func(mode string, entries int, trns *pil.Transparency) *pil.Image {
		img := &pil.Image{Mode: "P", Width: 16, Height: 16, Pix: r.Bytes(256)}
		img.PaletteMode, img.Palette = mode, r.Bytes(len(mode)*entries)
		img.Info.Transparency = trns
		return img
	}
	tInt := func(v int) *pil.Transparency { return &pil.Transparency{Kind: pil.TransparencyInt, Int: v} }
	tBytes := func(n int) *pil.Transparency {
		return &pil.Transparency{Kind: pil.TransparencyBytes, Bytes: r.Bytes(n)}
	}
	tRGB := &pil.Transparency{Kind: pil.TransparencyRGB, RGB: [3]int{1, 2, 3}}
	add("palette/rgb-256", palette("RGB", 256, nil))
	add("palette/rgb-3", palette("RGB", 3, nil))
	add("palette/rgba-100", palette("RGBA", 100, nil))
	add("palette/rgba-100-trns-bytes", palette("RGBA", 100, tBytes(50)))
	add("palette/rgb-257", palette("RGB", 257, nil))
	add("palette/none", &pil.Image{Mode: "P", Width: 4, Height: 4, Pix: r.Bytes(16)})
	add("palette/trns-int-0", palette("RGB", 256, tInt(0)))
	add("palette/trns-int-255", palette("RGB", 10, tInt(255)))
	add("palette/trns-int-256", palette("RGB", 10, tInt(256)))
	add("palette/trns-int-neg", palette("RGB", 10, tInt(-1)))
	add("palette/trns-bytes-0", palette("RGB", 10, tBytes(0)))
	add("palette/trns-bytes-3", palette("RGB", 10, tBytes(3)))
	add("palette/trns-bytes-256", palette("RGB", 256, tBytes(256)))
	add("palette/trns-bytes-257", palette("RGB", 256, tBytes(257)))
	add("palette/trns-tuple", palette("RGB", 10, tRGB))
	cases = append(cases, convertCase{name: "palette/trns-str", img: palette("RGB", 10, nil), strTransparency: "3"})

	gray := func(mode string, trns *pil.Transparency) *pil.Image {
		img := &pil.Image{Mode: mode, Width: 8, Height: 8, Pix: seededPix(r, mode, 64)}
		img.Info.Transparency = trns
		return img
	}
	add("trns/L-int", gray("L", tInt(5)))
	add("trns/L-int-300", gray("L", tInt(300)))
	add("trns/L-bytes", gray("L", tBytes(4)))
	add("trns/L-tuple", gray("L", tRGB))
	cases = append(cases, convertCase{name: "trns/L-str", img: gray("L", nil), strTransparency: "5"})
	add("trns/RGB-tuple", gray("RGB", tRGB))
	add("trns/RGB-int", gray("RGB", tInt(4)))
	add("trns/1-int", gray("1", tInt(255)))
	add("trns/I16-int", gray("I;16", tInt(9)))
	add("trns/LA-int", gray("LA", tInt(9)))
	add("trns/RGBA-tuple", gray("RGBA", tRGB))
	add("trns/PA-int", func() *pil.Image {
		img := gray("PA", tInt(3))
		img.PaletteMode, img.Palette = "RGB", r.Bytes(30)
		return img
	}())
	return cases
}

// goConvert runs the sanitizer's alpha check and conversion.
func goConvert(c convertCase) (string, string, error) {
	img := *c.img
	if c.strTransparency != "" {
		img.Info.Transparency = &pil.Transparency{Kind: 0}
	}
	mode, pix, err := convert.ForEncoder(&img)
	if err != nil {
		return "", "", err
	}
	return mode, sha(pix), nil
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
