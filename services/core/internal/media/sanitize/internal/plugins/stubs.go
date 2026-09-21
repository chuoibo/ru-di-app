package plugins

import (
	"bytes"
	"fmt"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// BUFR, GRIB and HDF5 are StubImageFiles with no handler registered: open
// succeeds with a 1x1 "F" image and load raises OSError.
func stub(name string, headerLen int64, accept func([]byte) bool) pil.Plugin {
	return plugin(name, accept, func(f *file) (pil.Opened, error) {
		if !accept(f.read(headerLen)) {
			return nil, raise("SyntaxError", "not a %s file", name)
		}
		_ = f.seek(-headerLen, 1)
		return finish("F", 1, 1, loadFails(raise("OSError", "cannot find loader for this %s file", name)))
	})
}

func acceptBUFR(p []byte) bool { return hasPrefix(p, "BUFR", "ZCZC") }

func acceptGRIB(p []byte) bool { return len(p) >= 8 && hasPrefix(p, "GRIB") && p[7] == 1 }

func acceptHDF5(p []byte) bool { return hasPrefix(p, "\x89HDF\r\n\x1a\n") }

func acceptWMF(p []byte) bool { return hasPrefix(p, "\xd7\xcd\xc6\x9a\x00\x00", "\x01\x00\x00\x00") }

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// openWMF is WmfStubImageFile._open. No handler is registered on Linux
// (Image.core has no drawwmf), so load raises OSError.
func openWMF(f *file) (pil.Opened, error) {
	s := f.read(44)
	var w, h int64
	switch {
	case hasPrefix(s, "\xd7\xcd\xc6\x9a\x00\x00"):
		inch, err := u16le(s, 14)
		if err != nil {
			return nil, err
		}
		if inch == 0 {
			return nil, raise("ValueError", "Invalid inch")
		}
		x0, _ := s16le(s, 6)
		y0, _ := s16le(s, 8)
		x1, _ := s16le(s, 10)
		y1, _ := s16le(s, 12)
		w = floorDiv((x1-x0)*72, inch)
		h = floorDiv((y1-y0)*72, inch)
		if len(s) < 26 || string(s[22:26]) != "\x01\x00\t\x00" {
			return nil, raise("SyntaxError", "Unsupported WMF file format")
		}
	case hasPrefix(s, "\x01\x00\x00\x00") && len(s) >= 44 && string(s[40:44]) == " EMF":
		v := make([]int64, 8)
		for i := range v {
			v[i], _ = s32le(s, 8+4*i)
		}
		w, h = v[2]-v[0], v[3]-v[1]
		if v[6]-v[4] == 0 || v[7]-v[5] == 0 {
			return nil, raise("ZeroDivisionError", "float division by zero")
		}
	default:
		return nil, raise("SyntaxError", "Unsupported file format")
	}
	return finish("RGB", w, h, loadFails(raise("OSError", "cannot find loader for this WMF file")))
}

// openMPEG is MpegImageFile._open: a sequence header gives the size, and
// no tile is set, so load raises "cannot load this image".
func openMPEG(f *file) (pil.Opened, error) {
	b := f.read(7)
	if len(b) < 4 {
		return nil, indexErr()
	}
	if string(b[:4]) != "\x00\x00\x01\xb3" {
		return nil, raise("SyntaxError", "not an MPEG file")
	}
	if len(b) < 7 {
		return nil, indexErr()
	}
	w := (int64(b[4])<<8 | int64(b[5])) >> 4
	h := (int64(b[5])&15)<<8 | int64(b[6])
	return finish("RGB", w, h, loadFails(raise("OSError", "cannot load this image")))
}

func acceptEPS(p []byte) bool {
	if hasPrefix(p, "%!PS") {
		return true
	}
	v, err := u32le(p, 0)
	return err == nil && v == 0xC6D3D0C5
}

// isStrSpace is str.isspace for a latin-1 character.
func isStrSpace(c byte) bool {
	return isASCIISpace(c) || (c >= 0x1c && c <= 0x1f) || c == 0x85 || c == 0xa0
}

func strFields(s []byte) [][]byte {
	var out [][]byte
	start := -1
	for i, c := range s {
		if isStrSpace(c) {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func asciiFields(s []byte) [][]byte {
	var out [][]byte
	start := -1
	for i, c := range s {
		if isASCIISpace(c) {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// isWord is re's \w for a latin-1 str character.
func isWord(c byte) bool {
	switch {
	case c == '_', c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c == 0xaa, c == 0xb2, c == 0xb3, c == 0xb5, c == 0xb9, c == 0xba:
		return true
	case c >= 0xbc && c <= 0xbe, c >= 0xc0 && c <= 0xd6, c >= 0xd8 && c <= 0xf6, c >= 0xf8:
		return true
	}
	return false
}

// openEPS is EpsImageFile._open. Ghostscript is not installed in the image,
// so load always raises OSError.
func openEPS(f *file) (pil.Opened, error) {
	s := f.read(4)
	var offset int64
	if string(s) == "%!PS" {
		offset = 0
	} else {
		magic, err := u32le(s, 0)
		if err != nil {
			return nil, err
		}
		if magic != 0xC6D3D0C5 {
			return nil, raise("SyntaxError", "not an EPS file")
		}
		s = f.read(8)
		if offset, err = u32le(s, 0); err != nil {
			return nil, err
		}
		if _, err = u32le(s, 4); err != nil {
			return nil, err
		}
	}
	_ = f.seek(offset, 0)
	mode := "RGB"
	info := map[string]bool{}
	var bbox []int64
	bboxSet := false
	var imgW, imgH int64
	imgSet := false
	var arr [255]byte
	n := 0
	readingHeader, readingTrailer, trailerReached := true, false, false
	visited := map[string]bool{}

	checkRequired := func() error {
		if !info["PS-Adobe"] {
			return raise("SyntaxError", `EPS header missing "%%!PS-Adobe" comment`)
		}
		if !info["BoundingBox"] {
			return raise("SyntaxError", `EPS header missing "%%%%BoundingBox" comment`)
		}
		return nil
	}
	readComment := func(line []byte) bool {
		if !hasPrefix(line, "%%") {
			return false
		}
		colon := bytes.IndexByte(line[2:], ':')
		if colon < 0 {
			return false
		}
		k := string(line[2 : 2+colon])
		v := line[2+colon+1:]
		for len(v) > 0 && (v[0] == ' ' || v[0] == '\t') {
			v = v[1:]
		}
		info[k] = true
		if k == "BoundingBox" {
			if string(v) == "(atend)" {
				readingTrailer = true
			} else if !bboxSet || len(bbox) == 0 || (trailerReached && readingTrailer) {
				var parsed []int64
				ok := true
				for _, token := range strFields(v) {
					fv, good := pyFloat(token, true)
					if !good || fv != fv || fv > 1e308 || fv < -1e308 {
						ok = false
						break
					}
					if fv > float64(saturate) {
						fv = float64(saturate)
					} else if fv < -float64(saturate) {
						fv = -float64(saturate)
					}
					parsed = append(parsed, int64(fv))
				}
				if ok {
					bbox, bboxSet = parsed, true
				}
			}
		}
		return true
	}

loop:
	for {
		b := f.read(1)
		switch {
		case len(b) == 0:
			if n == 0 {
				if readingHeader {
					if err := checkRequired(); err != nil {
						return nil, err
					}
				}
				break loop
			}
		case b[0] == '\r' || b[0] == '\n':
			if n == 0 {
				continue
			}
		default:
			if n >= 255 {
				if arr[0] == '%' {
					return nil, raise("SyntaxError", "not an EPS file")
				}
				if readingHeader {
					if err := checkRequired(); err != nil {
						return nil, err
					}
					readingHeader = false
				}
				n = 0
			}
			arr[n] = b[0]
			n++
			continue
		}
		line := arr[:n]
		switch {
		case readingHeader:
			if arr[0] != '%' || string(arr[:13]) == "%%EndComments" {
				if err := checkRequired(); err != nil {
					return nil, err
				}
				readingHeader = false
				continue
			}
			if !readComment(line) {
				if len(line) >= 2 && line[0] == '%' && (line[1] == '%' || line[1] == '!' || isWord(line[1])) &&
					bytes.IndexByte(line[2:], ':') < 0 {
					k := string(line[2:])
					if strings.HasPrefix(k, "PS-Adobe") {
						info["PS-Adobe"] = true
					} else {
						info[k] = true
					}
				} else if line[0] != '%' {
					return nil, raise("OSError", "bad EPS header")
				}
			}
		case string(arr[:11]) == "%ImageData:":
			if imgSet {
				n = 0
				continue
			}
			values := asciiFields(arr[11:n])
			var nums [4]int64
			for i := 0; i < 4; i++ {
				if i >= len(values) {
					return nil, raise("ValueError", "not enough values to unpack")
				}
				v, ok := pyInt(values[i], false)
				if !ok {
					return nil, raise("ValueError", "invalid literal for int()")
				}
				nums[i] = v
			}
			switch nums[2] {
			case 1:
				mode = "1"
			case 8:
				m, ok := map[int64]string{1: "L", 2: "LAB", 3: "RGB", 4: "CMYK"}[nums[3]]
				if !ok {
					return nil, raise("KeyError", "%d", nums[3])
				}
				mode = m
			default:
				break loop
			}
			imgW, imgH, imgSet = nums[0], nums[1], true
		case string(arr[:5]) == "%%EOF":
			break loop
		case trailerReached && readingTrailer:
			readComment(line)
		case string(arr[:9]) == "%%Trailer":
			trailerReached = true
		case string(arr[:14]) == "%%BeginBinary:":
			count, ok := pyInt(arr[14:n], false)
			if !ok {
				return nil, raise("ValueError", "invalid literal for int()")
			}
			_ = f.seek(count, 1)
			// A negative count seeks back over lines already read; when the
			// same position comes round with the same state, Pillow never
			// returns.
			key := fmt.Sprintf("%d|%t%t%t%t%t|%d|%x", f.tell(), readingHeader, readingTrailer, trailerReached, imgSet, bboxSet, len(info), arr)
			if visited[key] {
				return nil, pil.Unsupported("EPS", "Pillow does not terminate on this %%%%BeginBinary loop")
			}
			visited[key] = true
		}
		n = 0
	}
	if !bboxSet || len(bbox) == 0 {
		return nil, raise("OSError", "cannot determine EPS bounding box")
	}
	w, h := imgW, imgH
	if !imgSet {
		if len(bbox) < 4 {
			return nil, indexErr()
		}
		w, h = bbox[2]-bbox[0], bbox[3]-bbox[1]
	}
	return finish(mode, w, h, loadFails(raise("OSError", "Unable to locate Ghostscript on paths")))
}
