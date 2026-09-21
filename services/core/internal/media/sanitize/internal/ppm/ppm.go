// Package ppm reproduces Pillow 12.2.0's PpmImagePlugin: the header tokens
// with Python int()/float() parsing, the raw decoder for binary data, and
// the Python PpmPlainDecoder and PpmDecoder, each followed by set_as_raw.
package ppm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Plugin is the PPM entry of Image.OPEN.
var Plugin = pil.Plugin{Name: "PPM", Accept: Accept, Open: Open}

const (
	safeBlock    = 1 << 20
	decoderBlock = 1 << 16
	intMax       = 1<<31 - 1
)

// Accept is PpmImagePlugin._accept.
func Accept(prefix []byte) bool {
	return len(prefix) >= 2 && prefix[0] == 'P' && bytes.IndexByte([]byte("0123456fy"), prefix[1]) >= 0
}

var modes = map[string]string{
	"P1": "1", "P2": "L", "P3": "RGB", "P4": "1", "P5": "L", "P6": "RGB",
	"P0CMYK": "CMYK", "Pf": "F", "PyP": "P", "PyRGBA": "RGBA", "PyCMYK": "CMYK",
}

// Error is an exception open or load raised after the plugin accepted the
// file.
type Error struct{ Msg string }

func (e *Error) Error() string { return "PPM: " + e.Msg }

func fail(format string, args ...any) error { return &Error{Msg: fmt.Sprintf(format, args...)} }

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

type file struct {
	data []byte
	pos  int

	mode    string
	w, h    int64
	decoder string // "raw", "ppm_plain", "ppm"
	rawmode string
	maxval  int64
	ystep   int
	offset  int
}

func (f *file) readByte() (byte, bool) {
	if f.pos >= len(f.data) {
		return 0, false
	}
	c := f.data[f.pos]
	f.pos++
	return c, true
}

func (f *file) readToken() ([]byte, error) {
	var token []byte
	for len(token) <= 10 {
		c, ok := f.readByte()
		if !ok {
			break
		}
		if isWhitespace(c) {
			if len(token) == 0 {
				continue
			}
			break
		}
		if c == '#' {
			for {
				d, ok := f.readByte()
				if !ok || d == '\r' || d == '\n' {
					break
				}
			}
			continue
		}
		token = append(token, c)
	}
	if len(token) == 0 {
		return nil, fail("ValueError: Reached EOF while reading header")
	}
	if len(token) > 10 {
		return nil, fail("ValueError: Token too long in file header")
	}
	return token, nil
}

// digitsWithUnderscores strips Python's single underscores between digits,
// or reports that the digit string is malformed.
func digitsWithUnderscores(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '_' && i > 0 && i+1 < len(s) && s[i-1] >= '0' && s[i-1] <= '9' && s[i+1] >= '0' && s[i+1] <= '9':
		default:
			return "", false
		}
	}
	return b.String(), true
}

// pyInt is int(bytes) for tokens of at most 10 bytes.
func pyInt(token []byte) (int64, error) {
	s := strings.Trim(string(token), " \t\n\v\f\r")
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	digits, ok := digitsWithUnderscores(s)
	if !ok {
		return 0, fail("ValueError: invalid literal for int(): %q", token)
	}
	v, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fail("ValueError: invalid literal for int(): %q", token)
	}
	if neg {
		v = -v
	}
	return v, nil
}

// pyFloat is float(bytes) for tokens of at most 10 bytes.
func pyFloat(token []byte) (float64, error) {
	s := strings.Trim(string(token), " \t\n\v\f\r")
	body := s
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body = body[1:]
	}
	switch strings.ToLower(body) {
	case "inf", "infinity", "nan":
		v, _ := strconv.ParseFloat(s, 64)
		return v, nil
	}
	var clean strings.Builder
	sawDigit, sawExp := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			sawDigit = true
			clean.WriteByte(c)
		case c == '_':
			if i == 0 || i+1 >= len(s) || s[i-1] < '0' || s[i-1] > '9' || s[i+1] < '0' || s[i+1] > '9' {
				return 0, fail("ValueError: could not convert string to float")
			}
		case c == '+' || c == '-':
			if i != 0 && (s[i-1] != 'e' && s[i-1] != 'E') {
				return 0, fail("ValueError: could not convert string to float")
			}
			clean.WriteByte(c)
		case c == '.':
			clean.WriteByte(c)
		case c == 'e' || c == 'E':
			if sawExp || !sawDigit {
				return 0, fail("ValueError: could not convert string to float")
			}
			sawExp = true
			clean.WriteByte(c)
		default:
			return 0, fail("ValueError: could not convert string to float")
		}
	}
	v, err := strconv.ParseFloat(clean.String(), 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			return v, nil
		}
		return 0, fail("ValueError: could not convert string to float")
	}
	return v, nil
}

// Open is PpmImageFile._open.
func Open(data []byte) (pil.Opened, error) {
	f := &file{data: data, ystep: 1}
	var magic []byte
	for i := 0; i < 6; i++ {
		c, ok := f.readByte()
		if !ok || isWhitespace(c) {
			break
		}
		magic = append(magic, c)
	}
	mode, ok := modes[string(magic)]
	if !ok {
		return nil, pil.Next("not a PPM file")
	}
	f.mode = mode
	token, err := f.readToken()
	if err != nil {
		return nil, err
	}
	if f.w, err = pyInt(token); err != nil {
		return nil, err
	}
	if token, err = f.readToken(); err != nil {
		return nil, err
	}
	if f.h, err = pyInt(token); err != nil {
		return nil, err
	}
	magicStr := string(magic)
	f.decoder = "raw"
	if magicStr == "P1" || magicStr == "P2" || magicStr == "P3" {
		f.decoder = "ppm_plain"
	}
	switch mode {
	case "1":
		f.rawmode = "1;I"
	case "F":
		if token, err = f.readToken(); err != nil {
			return nil, err
		}
		scale, err := pyFloat(token)
		if err != nil {
			return nil, err
		}
		if scale == 0 || math.IsInf(scale, 0) || math.IsNaN(scale) {
			return nil, fail("ValueError: scale must be finite and non-zero")
		}
		f.rawmode = "F;32BF"
		if scale < 0 {
			f.rawmode = "F;32F"
		}
		f.ystep = -1
	default:
		if token, err = f.readToken(); err != nil {
			return nil, err
		}
		maxval, err := pyInt(token)
		if err != nil {
			return nil, err
		}
		if maxval <= 0 || maxval >= 65536 {
			return nil, fail("ValueError: maxval must be greater than 0 and less than 65536")
		}
		f.maxval = maxval
		if maxval > 255 && mode == "L" {
			f.mode = "I"
		}
		f.rawmode = mode
		if f.decoder != "ppm_plain" {
			if maxval == 65535 && mode == "L" {
				f.rawmode = "I;16B"
			} else if maxval != 255 {
				f.decoder = "ppm"
			}
		}
	}
	f.offset = f.pos
	// ImageFile.__init__: a non-positive size is "not identified by this
	// driver" (SyntaxError), so Image.open tries the next plugin.
	if f.w <= 0 || f.h <= 0 {
		return nil, pil.Next("PPM: not identified by this driver")
	}
	return f, nil
}

// Size is im.size after open (Python ints; a token has at most 10 bytes).
func (f *file) Size() (int, int) { return int(f.w), int(f.h) }

// pyFields is bytes.split() with no argument: ASCII whitespace only, unlike
// bytes.Fields, which also splits at encoded Unicode spaces.
func pyFields(block []byte) [][]byte {
	var fields [][]byte
	start := -1
	for i, c := range block {
		if isWhitespace(c) {
			if start >= 0 {
				fields = append(fields, block[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, block[start:])
	}
	return fields
}

// rawUnpack returns bits per pixel and a writer into the pil model for the
// rawmodes the PPM plugin can hand to the raw decoder.
func rawUnpack(mode, rawmode string) (int, func(dst, src []byte, n int), bool) {
	switch mode + "/" + rawmode {
	case "1/1;I":
		return 1, func(dst, src []byte, n int) {
			for i := 0; i < n; i++ {
				dst[i] = 255
				if src[i/8]>>(7-uint(i%8))&1 != 0 {
					dst[i] = 0
				}
			}
		}, true
	case "1/1;8":
		return 8, func(dst, src []byte, n int) {
			for i := 0; i < n; i++ {
				dst[i] = 0
				if src[i] > 0 {
					dst[i] = 255
				}
			}
		}, true
	case "L/L", "P/P":
		return 8, func(dst, src []byte, n int) { copy(dst[:n], src[:n]) }, true
	case "RGB/RGB":
		return 24, func(dst, src []byte, n int) { copy(dst[:3*n], src[:3*n]) }, true
	case "CMYK/CMYK", "RGBA/RGBA", "I/I;32", "F/F;32F":
		return 32, func(dst, src []byte, n int) { copy(dst[:4*n], src[:4*n]) }, true
	case "I/I;16B":
		return 16, func(dst, src []byte, n int) {
			for i := 0; i < n; i++ {
				binary.LittleEndian.PutUint32(dst[4*i:], uint32(src[2*i])<<8|uint32(src[2*i+1]))
			}
		}, true
	case "F/F;32BF":
		return 32, func(dst, src []byte, n int) {
			for i := 0; i < n; i++ {
				dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = src[4*i+3], src[4*i+2], src[4*i+1], src[4*i]
			}
		}, true
	}
	return 0, nil, false
}

// rawDecoder is ImagingRawDecode with stride 0.
type rawDecoder struct {
	pix    []byte
	stride int
	w, h   int
	bytes  int
	unpack func(dst, src []byte, n int)
	y      int
	ystep  int
	start  bool
}

func (d *rawDecoder) decode(buf []byte) int {
	if !d.start {
		d.start = true
		if d.ystep < 0 {
			d.y = d.h - 1
		}
	}
	used := 0
	for {
		if len(buf)-used < d.bytes {
			return used
		}
		d.unpack(d.pix[d.y*d.w*d.stride:], buf[used:], d.w)
		used += d.bytes
		d.y += d.ystep
		if d.y < 0 || d.y >= d.h {
			return -1
		}
	}
}

func (f *file) newImage() (*pil.Image, error) {
	if pil.BytesPerPixel(f.mode) == 0 {
		return nil, fail("ValueError: unrecognized image mode")
	}
	if f.w > intMax || f.h > intMax || f.w < -intMax-1 || f.h < -intMax-1 {
		return nil, fail("OverflowError: size")
	}
	if f.w < 0 || f.h < 0 {
		return nil, fail("ValueError: height and width must be > 0")
	}
	if f.w > intMax/4-1 {
		return nil, fail("MemoryError: line size")
	}
	stride := pil.BytesPerPixel(f.mode)
	if f.w*f.h*int64(stride) > 1<<34 {
		return nil, fail("MemoryError: image")
	}
	return &pil.Image{
		Mode: f.mode, Width: int(f.w), Height: int(f.h),
		Pix: make([]byte, int(f.w)*int(f.h)*stride), Format: "PPM",
	}, nil
}

// setAsRaw is PyDecoder.set_as_raw into a fresh raw decoder.
func setAsRaw(img *pil.Image, data []byte, rawmode string) error {
	bits, unpack, ok := rawUnpack(img.Mode, rawmode)
	if !ok {
		return fail("ValueError: unknown raw mode")
	}
	d := &rawDecoder{pix: img.Pix, stride: pil.BytesPerPixel(img.Mode), w: img.Width, h: img.Height,
		bytes: (img.Width*bits + 7) / 8, unpack: unpack, ystep: 1}
	if d.decode(data) >= 0 {
		return fail("ValueError: not enough image data")
	}
	return nil
}

// Load is ImageFile.load for the PPM tile.
func (f *file) Load() (*pil.Image, error) {
	img, err := f.newImage()
	if err != nil {
		return nil, err
	}
	if img.Width <= 0 || img.Height <= 0 {
		return nil, fail("ValueError: tile cannot extend outside image")
	}
	switch f.decoder {
	case "raw":
		bits, unpack, ok := rawUnpack(f.mode, f.rawmode)
		if !ok {
			return nil, fail("ValueError: unknown raw mode for given image mode")
		}
		if img.Width > intMax/bits-7 {
			return nil, fail("MemoryError: decoder buffer")
		}
		d := &rawDecoder{pix: img.Pix, stride: pil.BytesPerPixel(f.mode), w: img.Width, h: img.Height,
			bytes: (img.Width*bits + 7) / 8, unpack: unpack, ystep: f.ystep}
		pos := f.offset
		var pending []byte
		for {
			end := pos + decoderBlock
			if end > len(f.data) {
				end = len(f.data)
			}
			if pos >= len(f.data) {
				return nil, fail("OSError: image file is truncated")
			}
			s := f.data[pos:end]
			pos = end
			pending = append(pending, s...)
			n := d.decode(pending)
			if n < 0 {
				break
			}
			pending = append([]byte(nil), pending[n:]...)
		}
		return img, nil
	case "ppm_plain":
		p := &plainDecoder{data: f.data, pos: f.offset}
		if f.mode == "1" {
			data, err := p.bitonal(img.Width * img.Height)
			if err != nil {
				return nil, err
			}
			return img, setAsRaw(img, data, "1;8")
		}
		data, err := p.blocks(f.mode, img.Width*img.Height, f.maxval)
		if err != nil {
			return nil, err
		}
		rawmode := f.mode
		if f.mode == "I" {
			rawmode = "I;32"
		}
		return img, setAsRaw(img, data, rawmode)
	default:
		data := binaryDecode(f.data[min(f.offset, len(f.data)):], f.mode, img.Width*img.Height, f.maxval)
		rawmode := f.mode
		if f.mode == "I" {
			rawmode = "I;32"
		}
		return img, setAsRaw(img, data, rawmode)
	}
}

func bandsOf(mode string) int {
	switch mode {
	case "RGB":
		return 3
	case "RGBA", "CMYK":
		return 4
	}
	return 1
}

// binaryDecode is PpmDecoder.decode before set_as_raw.
func binaryDecode(data []byte, mode string, pixels int, maxval int64) []byte {
	inBytes := 1
	if maxval >= 256 {
		inBytes = 2
	}
	outBytes, outMax := 1, 255.0
	if mode == "I" {
		outBytes, outMax = 4, 65535
	}
	bands := bandsOf(mode)
	want := pixels * bands * outBytes
	out := make([]byte, 0, min(want, len(data)*outBytes+8))
	pos := 0
	for len(out) < want {
		if len(data)-pos < inBytes*bands {
			break
		}
		for b := 0; b < bands; b++ {
			var v int64
			if inBytes == 1 {
				v = int64(data[pos+b])
			} else {
				v = int64(data[pos+2*b])<<8 | int64(data[pos+2*b+1])
			}
			scaled := math.RoundToEven(float64(v) / float64(maxval) * outMax)
			if scaled > outMax {
				scaled = outMax
			}
			if mode == "I" {
				out = binary.LittleEndian.AppendUint32(out, uint32(int32(scaled)))
			} else {
				out = append(out, byte(scaled))
			}
		}
		pos += inBytes * bands
	}
	return out
}

// plainDecoder is PpmPlainDecoder reading the file from the tile offset.
type plainDecoder struct {
	data         []byte
	pos          int
	commentSpans bool
}

func (p *plainDecoder) readBlock() []byte {
	if p.pos >= len(p.data) {
		return nil
	}
	end := p.pos + safeBlock
	if end > len(p.data) {
		end = len(p.data)
	}
	out := p.data[p.pos:end]
	p.pos = end
	return out
}

func findFrom(block []byte, c byte, start int) int {
	if start > len(block) {
		return -1
	}
	i := bytes.IndexByte(block[start:], c)
	if i < 0 {
		return -1
	}
	return i + start
}

// commentEnd is _find_comment_end, including its min/max quirk.
func commentEnd(block []byte, start int) int {
	a := findFrom(block, '\n', start)
	b := findFrom(block, '\r', start)
	if a*b > 0 {
		return min(a, b)
	}
	return max(a, b)
}

func (p *plainDecoder) ignoreComments(block []byte) []byte {
	if p.commentSpans {
		for len(block) > 0 {
			end := commentEnd(block, 0)
			if end != -1 {
				block = block[end+1:]
				break
			}
			block = p.readBlock()
		}
	}
	p.commentSpans = false
	for {
		start := bytes.IndexByte(block, '#')
		if start == -1 {
			break
		}
		end := commentEnd(block, start)
		if end != -1 {
			joined := make([]byte, 0, len(block))
			joined = append(joined, block[:start]...)
			block = append(joined, block[end+1:]...)
		} else {
			block = block[:start]
			p.commentSpans = true
			break
		}
	}
	return block
}

func (p *plainDecoder) bitonal(total int) ([]byte, error) {
	var data []byte
	for len(data) != total {
		block := p.readBlock()
		if len(block) == 0 {
			break
		}
		block = p.ignoreComments(block)
		tokens := bytes.Join(pyFields(block), nil)
		for _, c := range tokens {
			if c != '0' && c != '1' {
				return nil, fail("ValueError: Invalid token for this mode")
			}
		}
		data = append(data, tokens...)
		if len(data) > total {
			data = data[:total]
		}
	}
	out := make([]byte, len(data))
	for i, c := range data {
		if c == '0' {
			out[i] = 0xff
		}
	}
	return out, nil
}

func (p *plainDecoder) blocks(mode string, pixels int, maxval int64) ([]byte, error) {
	const maxLen = 10
	outBytes, outMax := 1, 255.0
	if mode == "I" {
		outBytes, outMax = 4, 65535
	}
	total := pixels * bandsOf(mode) * outBytes
	var data, half []byte
	for len(data) != total {
		block := p.readBlock()
		if len(block) == 0 {
			if len(half) == 0 {
				break
			}
			block = []byte(" ")
		}
		block = p.ignoreComments(block)
		if len(half) > 0 {
			block = append(append([]byte(nil), half...), block...)
			half = nil
		}
		tokens := pyFields(block)
		if len(block) > 0 && !isWhitespace(block[len(block)-1]) {
			half = tokens[len(tokens)-1]
			tokens = tokens[:len(tokens)-1]
			if len(half) > maxLen {
				return nil, fail("ValueError: Token too long found in data")
			}
		}
		for _, token := range tokens {
			if len(token) > maxLen {
				return nil, fail("ValueError: Token too long found in data")
			}
			v, err := pyInt(token)
			if err != nil {
				return nil, err
			}
			if v < 0 {
				return nil, fail("ValueError: Channel value is negative")
			}
			if v > maxval {
				return nil, fail("ValueError: Channel value too large for this mode")
			}
			scaled := math.RoundToEven(float64(v) / float64(maxval) * outMax)
			if mode == "I" {
				data = binary.LittleEndian.AppendUint32(data, uint32(int32(scaled)))
			} else {
				data = append(data, byte(scaled))
			}
			if len(data) == total {
				break
			}
		}
	}
	return data, nil
}
