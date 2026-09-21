package ppm

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/convert"
	"mobile/services/core/internal/media/sanitize/internal/exif"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
)

type ppmCase struct {
	name  string
	class string
	data  []byte
}

// plainBody writes values as ASCII tokens separated by a seeded mix of
// whitespace, sometimes with comments.
func plainBody(r *pt.Rand, values []int, comments bool) []byte {
	seps := []string{" ", "\n", "\t", "  ", "\r\n", "\x0b", "\x0c"}
	var b strings.Builder
	for i, v := range values {
		b.WriteString(fmt.Sprint(v))
		b.WriteString(seps[r.Intn(len(seps))])
		if comments && i%7 == 3 {
			b.WriteString("# note " + fmt.Sprint(i) + "\n")
		}
	}
	return []byte(b.String())
}

func corpus() []ppmCase {
	var cases []ppmCase
	add := func(class, name string, data []byte) {
		cases = append(cases, ppmCase{name: class + "/" + name, class: class, data: data})
	}
	r := pt.NewRand(0x99)
	sizes := [][2]int{{1, 1}, {3, 2}, {17, 5}, {9, 33}}

	// Binary formats at 8 and 16 bits and other maxvals.
	for _, magic := range []string{"P5", "P6", "P0CMYK", "PyP", "PyRGBA", "PyCMYK"} {
		bands := map[string]int{"P5": 1, "P6": 3, "P0CMYK": 4, "PyP": 1, "PyRGBA": 4, "PyCMYK": 4}[magic]
		for _, maxval := range []int{255, 1, 100, 256, 1000, 65535} {
			for si, size := range sizes {
				if si > 1 && maxval != 255 && maxval != 65535 {
					continue
				}
				n := size[0] * size[1] * bands
				header := fmt.Sprintf("%s\n%d %d\n%d\n", magic, size[0], size[1], maxval)
				var body []byte
				for k := 0; k < n; k++ {
					v := r.Intn(maxval + 1)
					if k%11 == 5 && maxval < 65535 {
						v = maxval + r.Intn(3)
					}
					if maxval < 256 {
						body = append(body, byte(v))
					} else {
						body = binary.BigEndian.AppendUint16(body, uint16(v))
					}
				}
				add("binary", fmt.Sprintf("%s-max%d-%dx%d", magic, maxval, size[0], size[1]), append([]byte(header), body...))
			}
		}
	}
	for _, size := range sizes {
		n := (size[0] + 7) / 8 * size[1]
		add("binary", fmt.Sprintf("P4-%dx%d", size[0], size[1]), append([]byte(fmt.Sprintf("P4 %d %d ", size[0], size[1])), r.Bytes(n)...))
	}
	for _, scale := range []string{"1.0", "-1.0", "-0.5", "2e3"} {
		w, h := 4, 3
		body := make([]byte, 0, 4*w*h)
		for k := 0; k < w*h; k++ {
			v := math.Float32bits(float32(r.Intn(600)) - 100 + 0.75)
			if scale[0] == '-' {
				body = binary.LittleEndian.AppendUint32(body, v)
			} else {
				body = binary.BigEndian.AppendUint32(body, v)
			}
		}
		add("float", "Pf-scale"+scale, append([]byte(fmt.Sprintf("Pf\n%d %d\n%s\n", w, h, scale)), body...))
	}
	add("float", "Pf-scale-zero", []byte("Pf\n1 1\n0.0\n\x00\x00\x00\x00"))
	add("float", "Pf-scale-nan", []byte("Pf\n1 1\nnan\n\x00\x00\x00\x00"))
	add("float", "Pf-scale-underscore", []byte("Pf\n1 1\n1_0.5\n\x00\x00\x80\x3f"))
	add("float", "Pf-scale-hex", []byte("Pf\n1 1\n0x1p3\n\x00\x00\x80\x3f"))
	add("float", "Pf-nan-pixel", []byte("Pf\n2 1\n-1\n\x00\x00\xc0\x7f\x00\x00\x80\xff"))

	// Plain formats.
	for _, magic := range []string{"P1", "P2", "P3"} {
		bands := map[string]int{"P1": 1, "P2": 1, "P3": 3}[magic]
		for _, maxval := range []int{255, 15, 65535, 300} {
			if magic == "P1" && maxval != 255 {
				continue
			}
			for _, size := range sizes {
				n := size[0] * size[1] * bands
				values := make([]int, n)
				for k := range values {
					if magic == "P1" {
						values[k] = r.Intn(2)
					} else {
						values[k] = r.Intn(maxval + 1)
					}
				}
				header := fmt.Sprintf("%s\n%d %d\n", magic, size[0], size[1])
				if magic != "P1" {
					header += fmt.Sprintf("%d\n", maxval)
				}
				add("plain", fmt.Sprintf("%s-max%d-%dx%d", magic, maxval, size[0], size[1]), append([]byte(header), plainBody(r, values, size[0] > 3)...))
			}
		}
	}
	add("plain", "parity-p3", []byte("P3\n1 1\n255\n200 120 40\n"))
	add("plain", "p1-no-spaces", []byte("P1\n4 2\n01101001\n"))
	add("plain", "p1-comment-inside", []byte("P1\n4 2\n0110#c\n1001\n"))
	add("plain", "p1-bad-token", []byte("P1\n2 1\n0 2\n"))
	add("plain", "p1-short", []byte("P1\n4 2\n0110\n"))
	add("plain", "p2-comment-joins-tokens", []byte("P2\n2 1\n255\n12#x\n3 4\n"))
	add("plain", "p2-cr-comment", []byte("P2\n2 1\n255\n# c\r5 6\n"))
	add("plain", "p2-newline-first-comment", []byte("P2 2 1 255 #a\n7 #b\r\n8"))
	add("plain", "p2-value-too-large", []byte("P2\n2 1\n9\n5 10\n"))
	add("plain", "p2-negative", []byte("P2\n2 1\n9\n-1 3\n"))
	add("plain", "p2-plus-underscore", []byte("P2\n2 1\n99\n+5 1_2\n"))
	add("plain", "p2-junk-after-values", []byte("P2\n2 1\n9\n1 2 zz\n"))
	add("plain", "p2-junk-before-values", []byte("P2\n2 1\n9\nzz 1 2\n"))
	add("plain", "p2-token-too-long", []byte("P2\n2 1\n9\n00000000001 2\n"))
	add("plain", "p2-half-token-eof", []byte("P2\n2 1\n9\n1 2"))
	add("plain", "p2-short", []byte("P2\n2 2\n9\n1 2 3\n"))
	add("plain", "p2-rounding", []byte("P2\n4 1\n6\n1 3 5 2\n"))
	add("plain", "p2-16bit-rounding", []byte("P2\n3 1\n300\n1 150 299\n"))
	add("plain", "p3-comment-at-end", []byte("P3\n1 1\n255\n1 2 3 # end"))
	add("plain", "p2-unicode-space", []byte("P2\n2 1\n9\n1\xc2\x852\n"))

	// Header parsing.
	for name, data := range map[string]string{
		"comment-in-header":    "P5 #c\n2 #d\r1 255\nab",
		"token-joined-comment": "P5\n1#c\n2 1 255\nab",
		"underscore-size":      "P5\n1_2 1 255\n" + strings.Repeat("a", 12),
		"plus-size":            "P5\n+2 1 255\nab",
		"negative-size":        "P5\n-2 1 255\nab",
		"zero-size":            "P5\n0 1 255\n",
		"huge-size":            "P5\n2000000000 2 255\n",
		"bomb-size":            "P5\n20000 20000 255\n",
		"size-too-long":        "P5\n12345678901 1 255\n",
		"maxval-zero":          "P5\n1 1 0\na",
		"maxval-65536":         "P5\n1 1 65536\na",
		"no-maxval":            "P5\n1 1",
		"bad-magic":            "P7\n1 1 255\na",
		"magic-no-space":       "P61 1 255\nabc",
		"magic-6-bytes":        "P0CMYK1 1 255 abcd",
		"eof-header":           "P6\n",
		"x1c-in-token":         "P5\n\x1c1 1 255\na",
		"truncated-body":       "P6\n4 4 255\nabcdef",
		"extra-body":           "P5\n1 1 255\nabcdef",
		"p4-comment-eof":       "P4 8 1 #",
		"py-p":                 "PyP\n2 1 255\n\x01\x02",
		"cr-only-separators":   "P5\r2\r1\r255\rab",
	} {
		add("header", name, []byte(data))
	}
	return cases
}

type goOutcome struct {
	class  string
	detail string
	img    *pil.Image
}

func classify(err error) string {
	var bomb *pil.BombError
	switch {
	case pil.IsNext(err):
		return "next"
	case errors.As(err, &bomb):
		return "bomb"
	}
	return "error"
}

func goDecode(data []byte) goOutcome {
	opened, err := Open(data)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}
	}
	w, h := opened.Size()
	if err := pil.CheckBomb(w, h); err != nil {
		return goOutcome{class: "bomb", detail: err.Error()}
	}
	img, err := opened.Load()
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}
	}
	return goOutcome{class: "ok", img: img}
}

func goPixels(data []byte) (goOutcome, string, []byte) {
	outcome := goDecode(data)
	if outcome.class != "ok" {
		return outcome, "", nil
	}
	transposed, err := exif.Transpose(outcome.img)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	mode, pix, err := convert.ForEncoder(transposed)
	if err != nil {
		return goOutcome{class: classify(err), detail: err.Error()}, "", nil
	}
	outcome.img = transposed
	return outcome, mode, pix
}

func pyClass(errType string) string {
	switch {
	case errType == "":
		return "ok"
	case errType == "UnidentifiedImageError":
		return "next"
	case strings.HasPrefix(errType, "DecompressionBomb"):
		return "bomb"
	}
	return "error"
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
