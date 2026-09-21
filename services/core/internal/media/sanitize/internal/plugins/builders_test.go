package plugins

// Seeded input builders and the Go side of the comparisons, shared by the
// plain golden test and the oracle test.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// outcome is what one plugin did with one input, in comparable classes.
type outcome struct {
	accept     string
	open       string // ok, next, raise, bomb, unsupported
	w, h       float64
	mode       string
	load       string // ok, raise, unsupported
	loadedMode string
	lw, lh     float64
	sha        string
	detail     string
}

func (o outcome) String() string {
	return fmt.Sprintf("accept=%s open=%s %vx%v %q load=%s %q %vx%v sha=%.12s %s",
		o.accept, o.open, o.w, o.h, o.mode, o.load, o.loadedMode, o.lw, o.lh, o.sha, o.detail)
}

func tobytes(img *pil.Image) []byte {
	switch img.Mode {
	case "1":
		stride := (img.Width + 7) / 8
		out := make([]byte, stride*img.Height)
		for y := 0; y < img.Height; y++ {
			for x := 0; x < img.Width; x++ {
				if img.Pix[y*img.Width+x] != 0 {
					out[y*stride+x/8] |= 0x80 >> (x % 8)
				}
			}
		}
		return out
	case "I;16B":
		out := append([]byte(nil), img.Pix...)
		for i := 0; i+1 < len(out); i += 2 {
			out[i], out[i+1] = out[i+1], out[i]
		}
		return out
	}
	return img.Pix
}

func goOutcome(p pil.Plugin, data []byte) outcome {
	var o outcome
	prefix := data
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	switch {
	case p.Accept == nil:
		o.accept = "none"
	case p.Accept(prefix):
		o.accept = "true"
	default:
		o.accept = "false"
	}
	handle, err := p.Open(data)
	var bomb *pil.BombError
	var unsup *pil.UnsupportedError
	switch {
	case err == nil:
		o.open = "ok"
	case pil.IsNext(err):
		o.open = "next"
	case errors.As(err, &bomb):
		o.open = "bomb"
	case errors.As(err, &unsup):
		o.open, o.detail = "unsupported", err.Error()
		return o
	default:
		o.open, o.detail = "raise", err.Error()
		return o
	}
	if o.open != "ok" {
		o.detail = err.Error()
		return o
	}
	w, h := handle.Size()
	o.w, o.h = float64(w), float64(h)
	if op, ok := handle.(*opened); ok {
		o.mode = op.mode
	}
	if int64(w)*int64(h) > 16_000_000 {
		o.load = "skipped"
		return o
	}
	img, err := handle.Load()
	switch {
	case err == nil:
		o.load = "ok"
		o.loadedMode, o.lw, o.lh = img.Mode, float64(img.Width), float64(img.Height)
		sum := sha256.Sum256(tobytes(img))
		o.sha = hex.EncodeToString(sum[:])
	case errors.As(err, &unsup):
		o.load, o.detail = "unsupported", err.Error()
	default:
		o.load, o.detail = "raise", err.Error()
	}
	return o
}

type testCase struct {
	format string
	label  string
	data   []byte
}

// ---------------------------------------------------------------------------
// Builders

func le16(v int) []byte { return binary.LittleEndian.AppendUint16(nil, uint16(v)) }
func le32(v int) []byte { return binary.LittleEndian.AppendUint32(nil, uint32(v)) }
func be16(v int) []byte { return binary.BigEndian.AppendUint16(nil, uint16(v)) }
func be32(v int) []byte { return binary.BigEndian.AppendUint32(nil, uint32(v)) }

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func noise(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

func buildTGA(r *rand.Rand) []testCase {
	var out []testCase
	for _, imagetype := range []int{1, 2, 3, 9, 10, 11, 5} {
		for _, depth := range []int{1, 8, 15, 16, 24, 32} {
			for v := 0; v < 2; v++ {
				w, h := 1+r.Intn(9), 1+r.Intn(7)
				colormap := 0
				if imagetype&7 == 1 || r.Intn(6) == 0 {
					colormap = 1
				}
				idLen := r.Intn(3)
				mapLen := 1 + r.Intn(8)
				mapDepth := []int{15, 16, 24, 32}[r.Intn(4)]
				flags := []int{0, 0x10, 0x20, 0x30}[r.Intn(4)] | r.Intn(16)
				head := cat([]byte{byte(idLen), byte(colormap), byte(imagetype)}, le16(r.Intn(2)), le16(mapLen),
					[]byte{byte(mapDepth)}, le16(0), le16(0), le16(w), le16(h), []byte{byte(depth), byte(flags)})
				body := noise(r, idLen)
				if colormap == 1 {
					body = append(body, noise(r, mapLen*((mapDepth+7)/8))...)
				}
				bpp := (depth + 7) / 8
				if imagetype&8 != 0 {
					for n := 0; n < w*h; {
						run := 1 + r.Intn(5)
						if r.Intn(2) == 0 {
							body = append(body, byte(0x80|(run-1)))
							body = append(body, noise(r, bpp)...)
						} else {
							body = append(body, byte(run-1))
							body = append(body, noise(r, run*bpp)...)
						}
						n += run
					}
				} else {
					body = append(body, noise(r, w*h*bpp)...)
				}
				if r.Intn(2) == 0 {
					ext := len(head) + len(body)
					body = append(body, make([]byte, 494)...)
					body = append(body, byte(r.Intn(4)))
					body = append(body, cat(le32(ext), le32(0), []byte("TRUEVISION-XFILE.\x00"))...)
				}
				out = append(out, testCase{"TGA", fmt.Sprintf("go type%d depth%d", imagetype, depth), cat(head, body)})
			}
		}
	}
	return out
}

func buildSUN(r *rand.Rand) []testCase {
	var out []testCase
	for _, depth := range []int{1, 4, 8, 24, 32, 16} {
		for _, ft := range []int{0, 1, 2, 3, 6} {
			w, h := 1+r.Intn(8), 1+r.Intn(6)
			palLen := []int{0, 0, 6, 768, 1000}[r.Intn(5)]
			palType := 1
			if r.Intn(8) == 0 {
				palType = 2
			}
			head := cat(be32(0x59A66A95), be32(w), be32(h), be32(depth), be32(0), be32(ft), be32(palType), be32(palLen))
			stride := (w*depth + 15) / 16 * 2
			data := noise(r, palLen+stride*h)
			if ft == 2 {
				for i := palLen; i < len(data); i += 5 {
					data[i] = 0x80
				}
			}
			out = append(out, testCase{"SUN", fmt.Sprintf("go depth%d type%d", depth, ft), cat(head, data)})
		}
	}
	return out
}

func buildSGI(r *rand.Rand) []testCase {
	var out []testCase
	keys := [][3]int{{1, 1, 1}, {1, 2, 1}, {2, 1, 1}, {1, 3, 3}, {2, 3, 3}, {1, 3, 4}, {2, 3, 4}, {1, 3, 2}}
	for _, k := range keys {
		for _, comp := range []int{0, 1, 2} {
			w, h := 1+r.Intn(6), 1+r.Intn(5)
			z := k[2]
			head := make([]byte, 512)
			copy(head, cat(be16(474), []byte{byte(comp), byte(k[0])}, be16(k[1]), be16(w), be16(h), be16(z)))
			var body []byte
			if comp == 1 {
				tab := z * h
				start := make([]int, tab)
				length := make([]int, tab)
				var rows []byte
				base := 512 + 8*tab
				for i := 0; i < tab; i++ {
					var row []byte
					for x := 0; x < w; {
						n := 1 + r.Intn(w-x)
						if r.Intn(2) == 0 {
							row = append(row, byte(0x80|n))
							row = append(row, noise(r, n*k[0])...)
						} else {
							row = append(row, byte(n))
							row = append(row, noise(r, k[0])...)
						}
						x += n
					}
					row = append(row, make([]byte, k[0])...)
					start[i], length[i] = base+len(rows), len(row)/k[0]
					if r.Intn(10) == 0 {
						length[i] += 3
					}
					rows = append(rows, row...)
				}
				for _, s := range start {
					body = append(body, be32(s)...)
				}
				for _, l := range length {
					body = append(body, be32(l)...)
				}
				body = append(body, rows...)
			} else {
				body = noise(r, w*h*z*k[0])
			}
			out = append(out, testCase{"SGI", fmt.Sprintf("go %v comp%d", k, comp), cat(head, body)})
		}
	}
	return out
}

func buildQOI(r *rand.Rand) []testCase {
	var out []testCase
	for _, ch := range []byte{3, 4} {
		for v := 0; v < 4; v++ {
			w, h := 1+r.Intn(8), 1+r.Intn(6)
			var body []byte
			for i := 0; i < w*h; i++ {
				switch r.Intn(6) {
				case 0:
					body = append(body, 254)
					body = append(body, noise(r, 3)...)
				case 1:
					body = append(body, 255)
					body = append(body, noise(r, 4)...)
				case 2:
					body = append(body, byte(r.Intn(64)))
				case 3:
					body = append(body, byte(0x40|r.Intn(64)))
				case 4:
					body = append(body, byte(0x80|r.Intn(64)), byte(r.Intn(256)))
				default:
					body = append(body, byte(0xc0|r.Intn(3)))
				}
			}
			body = append(body, 0, 0, 0, 0, 0, 0, 0, 1)
			out = append(out, testCase{"QOI", fmt.Sprintf("go ch%d", ch), cat([]byte("qoif"), be32(w), be32(h), []byte{ch, 0}, body)})
		}
	}
	return out
}

func buildPCXPlanar(r *rand.Rand) []testCase {
	var out []testCase
	for _, planes := range []int{1, 2, 3, 4} {
		for _, bits := range []int{1, 8, 2} {
			w, h := 1+r.Intn(12), 1+r.Intn(6)
			stride := (w*bits + 7) / 8
			head := make([]byte, 128)
			copy(head, cat([]byte{10, 5, 1, byte(bits)}, le16(0), le16(0), le16(w-1), le16(h-1), le16(72), le16(72)))
			copy(head[16:64], noise(r, 48))
			head[65] = byte(planes)
			copy(head[66:], le16(stride))
			var body []byte
			for i := 0; i < planes*stride*h; i++ {
				if r.Intn(4) == 0 {
					body = append(body, byte(0xC0|(1+r.Intn(3))), byte(r.Intn(256)))
				} else {
					body = append(body, byte(r.Intn(0xC0)))
				}
			}
			if r.Intn(2) == 0 {
				body = append(body, 12)
				body = append(body, noise(r, 768)...)
			}
			out = append(out, testCase{"PCX", fmt.Sprintf("go planes%d bits%d", planes, bits), cat(head, body)})
		}
	}
	return out
}

func buildText(r *rand.Rand) []testCase {
	var out []testCase
	for v := 0; v < 6; v++ {
		w, h := 1+r.Intn(12), 1+r.Intn(5)
		var b strings.Builder
		fmt.Fprintf(&b, "#define im_width %d\n#define im_height %d\n", w, h)
		if v%2 == 1 {
			b.WriteString("#define im_x_hot 1\n#define im_y_hot 2\n")
		}
		b.WriteString("static char im_bits[] = {\n")
		for i := 0; i < (w+7)/8*h; i++ {
			fmt.Fprintf(&b, "0x%02x, ", r.Intn(256))
		}
		b.WriteString("};\n")
		out = append(out, testCase{"XBM", "go xbm", []byte(b.String())})
	}
	for v := 0; v < 10; v++ {
		w, h := 1+r.Intn(6), 1+r.Intn(4)
		bpp := 1 + r.Intn(2)
		colors := 1 + r.Intn(5)
		if v == 9 {
			colors = 260
			bpp = 2
		}
		var b strings.Builder
		b.WriteString("/* XPM */\nstatic char *im[] = {\n")
		fmt.Fprintf(&b, "\"%d %d %d %d\",\n", w, h, colors, bpp)
		keys := []string{}
		for i := 0; i < colors; i++ {
			key := fmt.Sprintf("%c%c", 'a'+i%26, 'A'+i/26%26)[:bpp]
			if bpp == 1 {
				key = string(rune('!' + i))
			}
			keys = append(keys, key)
			switch {
			case i == 0 && v%3 == 0:
				fmt.Fprintf(&b, "\"%s c None\",\n", key)
			case v == 7 && i == 1:
				fmt.Fprintf(&b, "\"%s c red\",\n", key)
			default:
				fmt.Fprintf(&b, "\"%s c #%06x\",\n", key, r.Intn(1<<24))
			}
		}
		if v%2 == 0 {
			b.WriteString("/* pixels */\n")
		}
		for y := 0; y < h; y++ {
			b.WriteString("\"")
			for x := 0; x < w; x++ {
				b.WriteString(keys[r.Intn(len(keys))])
			}
			b.WriteString("\",\n")
		}
		b.WriteString("};\n")
		out = append(out, testCase{"XPM", "go xpm", []byte(b.String())})
	}
	for v := 0; v < 6; v++ {
		w, h := 1+r.Intn(6), 1+r.Intn(4)
		text := fmt.Sprintf("width %d\nheight %d\npixel n8\n* comment\n\x0c", w, h)
		out = append(out, testCase{"IMT", "go imt", cat([]byte(text), noise(r, w*h+r.Intn(3)-1))})
	}
	imModes := []string{"L 8 image", "Greyscale image", "RGB image", "RGBA image", "B2 image", "L 32 F image", "RGB3 image", "LA image", "CMYK image", "foo image"}
	for v, m := range imModes {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		size := fmt.Sprintf("%d*%d", w, h)
		if v == 3 {
			size = "2.5*3"
		}
		header := fmt.Sprintf("Image type: %s\r\nName: x\r\nImage size (x*y): %s\r\n", m, size)
		if v%4 == 1 {
			header += "Lut: 1\r\n"
		}
		pad := make([]byte, 512-len(header))
		pad[len(pad)-1] = 0x1a
		body := noise(r, w*h*4)
		if v%4 == 1 {
			body = cat(noise(r, 768), body)
		}
		out = append(out, testCase{"IM", "go im " + m, cat([]byte(header), pad, body)})
	}
	for v := 0; v < 4; v++ {
		w, h := 2+r.Intn(4), 1+r.Intn(3)
		text := fmt.Sprintf("%%!PS-Adobe-3.0 EPSF-3.0\n%%%%BoundingBox: 0 0 %d %d\n%%%%EndComments\n%%ImageData: %d %d 8 %d 0 1 1 \"beginimage\"\n", w, h, w, h, 1+v)
		if v == 3 {
			text += "%%BeginBinary: -40\n"
		}
		out = append(out, testCase{"EPS", "go eps", cat([]byte(text), noise(r, 20))})
	}
	return out
}

func iptcFieldBytes(rec, ds int, data []byte) []byte {
	return cat([]byte{0x1c, byte(rec), byte(ds)}, be16(len(data)), data)
}

func buildIPTC(r *rand.Rand) []testCase {
	var out []testCase
	for v := 0; v < 8; v++ {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		layers := []int{1, 3, 4, 2}[v%4]
		comp := 0
		if layers > 1 {
			comp = 1
		}
		fields := [][]byte{
			iptcFieldBytes(3, 60, []byte{byte(layers), byte(comp)}),
			iptcFieldBytes(3, 20, be16(w)),
			iptcFieldBytes(3, 30, be16(h)),
			iptcFieldBytes(3, 120, []byte{1}),
		}
		if v%3 == 1 {
			fields = append(fields, iptcFieldBytes(3, 65, []byte{byte(1 + r.Intn(3))}))
		}
		if v == 6 {
			fields[3] = iptcFieldBytes(3, 120, []byte{5})
		}
		r.Shuffle(len(fields), func(i, j int) { fields[i], fields[j] = fields[j], fields[i] })
		fields = append(fields, iptcFieldBytes(8, 10, noise(r, w*h)))
		out = append(out, testCase{"IPTC", fmt.Sprintf("go iptc %d", v), cat(fields...)})
	}
	return out
}

func buildMisc(r *rand.Rand) []testCase {
	var out []testCase
	out = append(out,
		testCase{"PCD", "go pcd", cat(make([]byte, 2048), []byte("PCD_"), noise(r, 1536))},
		testCase{"MPEG", "go mpeg", cat([]byte{0, 0, 1, 0xb3}, noise(r, 8))},
		testCase{"BUFR", "go bufr", cat([]byte("BUFR"), noise(r, 30))},
		testCase{"GRIB", "go grib", cat([]byte("GRIB\x00\x00\x00\x01"), noise(r, 30))},
		testCase{"HDF5", "go hdf5", cat([]byte("\x89HDF\r\n\x1a\n"), noise(r, 30))},
	)
	for _, kind := range []int{1, 2, 4, 3} {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		fields := make([]int32, 64)
		fields[1], fields[8], fields[9], fields[10], fields[13], fields[14] = 4, int32(h), int32(w), int32(kind), 1, 0
		fields[33] = 256
		var head []byte
		for _, f := range fields {
			head = append(head, be32(int(f))...)
		}
		out = append(out, testCase{"MCIDAS", fmt.Sprintf("go mcidas %d", kind), cat(head, noise(r, w*h*kind))})
	}
	for v := 0; v < 3; v++ {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		head := make([]byte, 1024)
		copy(head, []byte{0x80, 0xe8, 0, 0})
		copy(head[416:], le16(h))
		copy(head[418:], le16(w))
		copy(head[424:], cat(le16(14), le16(2-v%2)))
		out = append(out, testCase{"PIXAR", "go pixar", cat(head, noise(r, w*h*3))})
	}
	for v := 0; v < 3; v++ {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		out = append(out, testCase{"XVTHUMB", "go xv", cat([]byte(fmt.Sprintf("P7 332\n#XVVERSION\n#END\n%d %d 255\n", w, h)), noise(r, w*h))})
	}
	for _, version := range []int{1, 2, 3} {
		for _, depth := range []int{1, 4, 3} {
			w, h := 1+r.Intn(5), 1+r.Intn(4)
			hs := 20 + 5
			if version == 2 {
				hs = 28 + 5
			}
			head := cat(le32(hs), le32(version), le32(w), le32(h), le32(depth))
			if version == 2 {
				head = cat(head, []byte("GIMP"), le32(10))
			}
			out = append(out, testCase{"GBR", fmt.Sprintf("go gbr v%d d%d", version, depth), cat(head, []byte("name\x00"), noise(r, w*h*depth))})
		}
	}
	for _, format := range []int{1, 0, 2} {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		data := noise(r, w*h*3)
		out = append(out, testCase{"FTEX", fmt.Sprintf("go ftex %d", format),
			cat([]byte("FTEX"), le32(1), le32(w), le32(h), le32(1), le32(1), le32(format), le32(32), le32(len(data)), data)})
	}
	for v := 0; v < 4; v++ {
		head := make([]byte, 128)
		copy(head[4:], le16(0xAF12))
		copy(head[6:], le16(1+v%2))
		copy(head[8:], cat(le16(4), le16(3)))
		chunk := cat(le32(40), le16(0xF1FA), le16(1), make([]byte, 8), le32(22), le16(11), le16(1), []byte{0, 2}, noise(r, 6))
		out = append(out, testCase{"FLI", "go fli", cat(head, chunk)})
	}
	for _, mode := range []int{3, 1, 2, 4, 9, 5} {
		for _, comp := range []int{0, 1, 2} {
			w, h := 1+r.Intn(5), 1+r.Intn(4)
			channels := 3
			head := cat([]byte("8BPS"), be16(1), make([]byte, 6), be16(channels), be32(h), be32(w), be16(8), be16(mode))
			body := cat(be32(0), be32(12), []byte("8BIM"), be16(1039), []byte{0, 0}, be32(0), be32(0))
			body = append(body, be16(comp)...)
			if comp == 1 {
				for i := 0; i < channels*h; i++ {
					body = append(body, be16(2)...)
				}
			}
			body = append(body, noise(r, w*h*channels)...)
			out = append(out, testCase{"PSD", fmt.Sprintf("go psd mode%d comp%d", mode, comp), cat(head, body)})
		}
	}
	for _, variant := range []string{"L", "LA", "P", "DXT1", "DX10", "RGB16", "bad"} {
		w, h := 1+r.Intn(5), 1+r.Intn(4)
		header := make([]byte, 120)
		copy(header[4:], cat(le32(h), le32(w)))
		var tail []byte
		switch variant {
		case "L":
			copy(header[72:], cat(le32(0x20000), le32(0), le32(8)))
			tail = noise(r, w*h)
		case "LA":
			copy(header[72:], cat(le32(0x20001), le32(0), le32(16)))
			tail = noise(r, w*h*2)
		case "P":
			copy(header[72:], cat(le32(0x20), le32(0), le32(8)))
			tail = noise(r, 1024+w*h)
		case "DXT1":
			copy(header[72:], cat(le32(4), []byte("DXT1"), le32(0)))
			tail = noise(r, 8*((w+3)/4)*((h+3)/4))
		case "DX10":
			copy(header[72:], cat(le32(4), []byte("DX10"), le32(0)))
			tail = cat(le32(28), noise(r, 16), noise(r, w*h*4))
		case "RGB16":
			copy(header[72:], cat(le32(0x41), le32(0), le32(16), le32(0x7c00), le32(0x3e0), le32(0x1f), le32(0x8000)))
			tail = noise(r, w*h*2)
		default:
			copy(header[72:], cat(le32(0), le32(0), le32(0)))
		}
		out = append(out, testCase{"DDS", "go dds " + variant, cat([]byte("DDS "), le32(124), header, tail)})
	}
	for _, v := range []string{"BLP1", "BLP2"} {
		out = append(out, testCase{"BLP", "go " + v, cat([]byte(v), le32(1), le32(8), le32(3), le32(2), le32(4), noise(r, 200))})
	}
	for v := 0; v < 4; v++ {
		w, h := 1+r.Intn(8), 1+r.Intn(8)
		dib := cat(le32(40), le32(w), le32(2*h), le16(1), le16([]int{24, 32, 8, 1}[v]), le32(0), le32(0), le32(0), le32(0), le32(0), le32(0))
		dib = append(dib, noise(r, 1024+w*h*8)...)
		cur := cat([]byte{0, 0, 2, 0}, le16(1), []byte{byte(w), byte(h), 0, 0}, le16(1), le16(1), le32(len(dib)), le32(22), dib)
		out = append(out, testCase{"CUR", "go cur", cur})
		ico := cat([]byte{0, 0, 1, 0}, le16(1), []byte{byte(w), byte(h), 0, 0}, le16(1), le16([]int{24, 32, 8, 1}[v]), le32(len(dib)), le32(22), dib)
		out = append(out, testCase{"ICO", "go ico", ico})
	}
	for v := 0; v < 3; v++ {
		body := cat([]byte("is32"), be32(8+16*16*3), noise(r, 16*16*3), []byte("s8mk"), be32(8+256), noise(r, 256))
		if v == 1 {
			body = cat([]byte("zzzz"), be32(12), noise(r, 4))
		}
		out = append(out, testCase{"ICNS", "go icns", cat([]byte("icns"), be32(8+len(body)), body)})
	}
	for _, brands := range [][]string{{"mif1", "heic"}, {"avif", "mif1"}, {"mif1", "avif"}, {"msf1", "avis"}, {"heic", "avif"}} {
		body := cat([]byte(brands[0]), le32(0))
		for _, b := range brands[1:] {
			body = append(body, []byte(b)...)
		}
		ftyp := cat(be32(8+len(body)), []byte("ftyp"), body)
		out = append(out, testCase{"AVIF", "go avif " + strings.Join(brands, "/"), cat(ftyp, noise(r, 40))})
	}
	for v := 0; v < 4; v++ {
		siz := cat([]byte{0xff, 0x4f, 0xff, 0x51}, be16(41), be16(0), be32(5+v), be32(4), be32(0), be32(0), be32(5), be32(4), be32(0), be32(0), be16(v%4+1))
		for c := 0; c < v%4+1; c++ {
			siz = append(siz, 7, 1, 1)
		}
		siz = cat(siz, []byte{0xff, 0x64}, be16(8), []byte{0, 1}, []byte("abcd"), []byte{0xff, 0xd9})
		out = append(out, testCase{"JPEG2000", "go j2k", siz})
	}
	for v := 0; v < 4; v++ {
		name := []string{"FITS", "WMF", "WMF", "SPIDER"}[v]
		switch v {
		case 0:
			var b strings.Builder
			for _, card := range []string{"SIMPLE  =                    T", "BITPIX  =                    8", "NAXIS   =                    2", "NAXIS1  =                    3", "NAXIS2  =                    2", "END"} {
				fmt.Fprintf(&b, "%-80s", card)
			}
			header := []byte(b.String())
			header = append(header, bytes.Repeat([]byte(" "), 2880-len(header))...)
			out = append(out, testCase{name, "go fits", cat(header, noise(r, 2880))})
		case 1:
			head := cat([]byte{0xd7, 0xcd, 0xc6, 0x9a, 0, 0}, le16(0), le16(0), le16(100), le16(80), le16(1440), le32(0), le16(0))
			head = cat(head, []byte{1, 0, 9, 0}, noise(r, 30))
			out = append(out, testCase{name, "go wmf placeable", head})
		case 2:
			head := cat(le32(1), le32(88), le32(0), le32(0), le32(100), le32(50), le32(0), le32(0), le32(2000), le32(1000), []byte(" EMF"), noise(r, 20))
			out = append(out, testCase{name, "go emf", head})
		case 3:
			out = append(out, testCase{name, "go spider zero", make([]byte, 200)})
		}
	}
	return out
}

func tiffEntry(tag, typ, count int, value []byte) []byte {
	v := make([]byte, 4)
	copy(v, value)
	return cat(le16(tag), le16(typ), le32(count), v)
}

func buildTIFF(r *rand.Rand) []testCase {
	var out []testCase
	for v := 0; v < 30; v++ {
		w, h := 1+r.Intn(6), 1+r.Intn(5)
		spp := []int{1, 3, 4, 1, 2}[v%5]
		photo := []int{1, 2, 2, 3, 1}[v%5]
		bits := 8
		var entries [][]byte
		add := func(tag, typ, count int, value []byte) { entries = append(entries, tiffEntry(tag, typ, count, value)) }
		add(256, 3, 1, le16(w))
		add(257, 4, 1, le32(h))
		bpsCount := spp
		switch r.Intn(4) {
		case 0:
			add(258, 3, 1, le16(bits))
		case 1:
			add(258, 1, 1, []byte{8})
		default:
			if bpsCount <= 2 {
				add(258, 3, bpsCount, cat(le16(8), le16(8)))
			} else {
				add(258, 3, bpsCount, le32(0))
			}
		}
		add(259, []int{3, 3, 2, 5}[r.Intn(4)], 1, le16([]int{1, 5, 1, 32773, 7}[r.Intn(5)]))
		add(262, 3, 1, le16(photo))
		add(277, 3, 1, le16(spp))
		if spp == 2 {
			add(338, 3, 1, le16(r.Intn(3)))
		}
		if photo == 3 {
			add(320, 3, 768, le32(0))
		}
		if r.Intn(3) > 0 {
			add(273, 4, 1, le32(0))
			add(278, 3, 1, le16(h))
			add(279, 4, 1, le32(w*h*spp))
		}
		if r.Intn(5) == 0 {
			add(274, 3, 1, le16(1+r.Intn(8)))
		}
		if r.Intn(5) == 0 {
			add(284, 3, 1, le16(2))
		}
		if r.Intn(6) == 0 {
			add(282, 2, 4, []byte("abc\x00"))
			add(296, 3, 1, le16(3))
		}
		if r.Intn(6) == 0 {
			add(339, 3, 1, le16(3))
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return binary.LittleEndian.Uint16(entries[i]) < binary.LittleEndian.Uint16(entries[j])
		})
		ifdOff := 8
		ifd := cat(le16(len(entries)))
		for _, e := range entries {
			ifd = append(ifd, e...)
		}
		ifd = append(ifd, le32(0)...)
		extra := make([]byte, 0)
		ifdLen := len(ifd)
		dataOff := ifdOff + ifdLen
		for i, e := range entries {
			tag := binary.LittleEndian.Uint16(e)
			typ := binary.LittleEndian.Uint16(e[2:])
			count := binary.LittleEndian.Uint32(e[4:])
			size := int(count) * int(tiffUnitSize[int64(typ)])
			pos := 2 + 12*i + 8
			if size > 4 {
				binary.LittleEndian.PutUint32(ifd[pos:], uint32(dataOff+len(extra)))
				extra = append(extra, noise(r, size)...)
			}
			if tag == 273 {
				binary.LittleEndian.PutUint32(ifd[pos:], uint32(dataOff+ifdLen))
			}
		}
		data := cat([]byte("II*\x00"), le32(ifdOff), ifd, extra, noise(r, w*h*spp+8))
		out = append(out, testCase{"TIFF", fmt.Sprintf("go tiff %d", v), data})
	}
	return out
}

// variants truncates, mutates and pads one base.
func variants(r *rand.Rand, base testCase) []testCase {
	out := []testCase{base}
	n := len(base.data)
	limit := min(n, 48)
	for cut := 0; cut < limit; cut += 1 + cut/8 {
		out = append(out, testCase{base.format, base.label + fmt.Sprintf(" cut%d", cut), base.data[:cut]})
	}
	if n > 2 {
		out = append(out, testCase{base.format, base.label + " cut-half", base.data[:n/2]})
		out = append(out, testCase{base.format, base.label + " cut-1", base.data[:n-1]})
	}
	for m := 0; m < 10 && n > 0; m++ {
		d := append([]byte(nil), base.data...)
		for k := 0; k < 1+r.Intn(2); k++ {
			i := r.Intn(min(n, 64))
			switch r.Intn(4) {
			case 0:
				d[i] = 0
			case 1:
				d[i] = 0xff
			default:
				d[i] = byte(r.Intn(256))
			}
		}
		out = append(out, testCase{base.format, base.label + fmt.Sprintf(" mut%d", m), d})
	}
	out = append(out, testCase{base.format, base.label + " pad", cat(base.data, noise(r, 1+r.Intn(64)))})
	return out
}
