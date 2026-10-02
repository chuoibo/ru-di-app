package congdong

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // community keeps PNG uploads as PNG
)

// canhAnh are the longer sides, largest first, a reading tries for its
// images: the largest at which every image of the submission fits
// MaxByteAnh is the one sent. A post holds ten images and a shared diary
// forty, so the stored photographs (up to 12 MiB each) seldom fit as they are.
var canhAnh = []int{1536, 1024, 768, 512}

const chatLuongAnh = 80

// thuNho is an image as a JPEG whose longer side is at most canh pixels,
// box-averaged down and laid on white. A reading needs what the picture
// shows, not its bytes: shrinking is what lets every image reach the model.
// It reads the source one row at a time, so memory follows the width only.
func thuNho(src image.Image, canh int) ([]byte, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	if w > canh || h > canh {
		if w >= h {
			nw, nh = canh, max(1, h*canh/w)
		} else {
			nw, nh = max(1, w*canh/h), canh
		}
	}
	row := image.NewRGBA(image.Rect(0, 0, w, 1))
	sum := make([]int, nw*4)
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := range nh {
		y0, y1 := khoang(y, h, nh)
		clear(sum)
		for sy := y0; sy < y1; sy++ {
			draw.Draw(row, row.Bounds(), src, image.Pt(b.Min.X, b.Min.Y+sy), draw.Src)
			for x := range nw {
				x0, x1 := khoang(x, w, nw)
				s := sum[x*4 : x*4+4]
				p := row.Pix[x0*4 : x1*4]
				for i := 0; i < len(p); i += 4 {
					s[0] += int(p[i])
					s[1] += int(p[i+1])
					s[2] += int(p[i+2])
					s[3] += int(p[i+3])
				}
			}
		}
		for x := range nw {
			x0, x1 := khoang(x, w, nw)
			n := (y1 - y0) * (x1 - x0)
			s := sum[x*4 : x*4+4]
			// Premultiplied colour over white: c + (255 - alpha).
			white := 255 - s[3]/n
			d := out.Pix[y*out.Stride+x*4:]
			d[0] = uint8(min(255, s[0]/n+white))
			d[1] = uint8(min(255, s[1]/n+white))
			d[2] = uint8(min(255, s[2]/n+white))
			d[3] = 255
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: chatLuongAnh}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// khoang is the source pixels [from, to) that output pixel i of n averages,
// out of size source pixels: never empty.
func khoang(i, size, n int) (int, int) {
	from := i * size / n
	return from, max((i+1)*size/n, from+1)
}

// vuaAnh is every image shrunk to the largest side of canhAnh at which they
// all fit MaxByteAnh together, or ok false when one does not decode or even
// the smallest side does not fit. Each stored image is decoded once; the
// smaller sides are cut from its largest cut.
func vuaAnh(anh []Media) (out [][]byte, ok bool) {
	first := make([][]byte, 0, len(anh))
	total := 0
	for _, m := range anh {
		img, _, err := image.Decode(bytes.NewReader(m.Data))
		if err != nil {
			return nil, false
		}
		b, err := thuNho(img, canhAnh[0])
		if err != nil {
			return nil, false
		}
		first = append(first, b)
		total += len(b)
	}
	if total <= MaxByteAnh {
		return first, true
	}
	for _, canh := range canhAnh[1:] {
		out = make([][]byte, 0, len(first))
		total = 0
		for _, b := range first {
			img, err := jpeg.Decode(bytes.NewReader(b))
			if err == nil {
				b, err = thuNho(img, canh)
			}
			if err != nil {
				return nil, false
			}
			total += len(b)
			out = append(out, b)
		}
		if total <= MaxByteAnh {
			return out, true
		}
	}
	return nil, false
}
