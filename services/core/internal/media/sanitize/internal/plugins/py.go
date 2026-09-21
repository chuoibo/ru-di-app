// Package plugins reproduces the Pillow 12.2.0 open plugins the sanitizer's
// Image.open walks after BMP, DIB, GIF, JPEG, PPM and PNG: their accept
// tests, what their ImageFile.__init__ and _open raise on arbitrary bytes,
// the size they report, and, for the simple raw and RLE formats, the pixels
// load() produces. Formats whose decoding needs a codec library this port
// does not carry answer load with a *pil.UnsupportedError.
package plugins

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// exc is a Python exception raised inside plugin code.
type exc struct {
	Type string
	Msg  string
}

func (e *exc) Error() string { return e.Type + ": " + e.Msg }

func raise(typ, format string, args ...any) error {
	return &exc{Type: typ, Msg: fmt.Sprintf(format, args...)}
}

func structErr() error { return raise("struct.error", "unpack requires a buffer") }

func indexErr() error { return raise("IndexError", "index out of range") }

// excType is the Python exception name of err, "" for Go-only errors.
func excType(err error) string {
	var e *exc
	if errors.As(err, &e) {
		return e.Type
	}
	return ""
}

// openError maps an exception leaving a plugin's constructor to what
// _open_core does with it: ImageFile.__init__ turns IndexError, TypeError,
// KeyError, EOFError and struct.error into SyntaxError, and _open_core
// catches SyntaxError, IndexError, TypeError and struct.error.
func openError(err error) error {
	switch excType(err) {
	case "SyntaxError", "IndexError", "TypeError", "KeyError", "EOFError", "struct.error":
		return pil.Next("%v", err)
	}
	return err
}

// file is io.BytesIO over the upload.
type file struct {
	data []byte
	pos  int64
}

func (f *file) size() int64 { return int64(len(f.data)) }

// read is BytesIO.read(n); a negative n reads to the end.
func (f *file) read(n int64) []byte {
	end := f.size()
	if f.pos >= end {
		return nil
	}
	if n >= 0 && n < end-f.pos {
		end = f.pos + n
	}
	out := f.data[f.pos:end]
	f.pos = end
	return out
}

func (f *file) readline() []byte {
	if f.pos >= f.size() {
		return nil
	}
	rest := f.data[f.pos:]
	n := len(rest)
	for i, c := range rest {
		if c == '\n' {
			n = i + 1
			break
		}
	}
	f.pos += int64(n)
	return rest[:n]
}

func (f *file) tell() int64 { return f.pos }

// seek is BytesIO.seek: a negative absolute position raises ValueError, a
// relative one below zero clamps to zero.
func (f *file) seek(offset int64, whence int) error {
	pos := offset
	switch whence {
	case 0:
		if offset < 0 {
			return raise("ValueError", "negative seek value %d", offset)
		}
	case 1:
		pos = f.pos + offset
	case 2:
		pos = f.size() + offset
	}
	if pos < 0 {
		pos = 0
	}
	f.pos = pos
	return nil
}

// Binary field readers with Python's failure classes: an index past the end
// is IndexError, a short struct unpack is struct.error.

func at(b []byte, i int) (int64, error) {
	if i < 0 || i >= len(b) {
		return 0, indexErr()
	}
	return int64(b[i]), nil
}

func need(b []byte, o, n int) error {
	if o < 0 || len(b) < o+n {
		return structErr()
	}
	return nil
}

func u16le(b []byte, o int) (int64, error) {
	if err := need(b, o, 2); err != nil {
		return 0, err
	}
	return int64(b[o]) | int64(b[o+1])<<8, nil
}

func u16be(b []byte, o int) (int64, error) {
	if err := need(b, o, 2); err != nil {
		return 0, err
	}
	return int64(b[o])<<8 | int64(b[o+1]), nil
}

func s16le(b []byte, o int) (int64, error) {
	v, err := u16le(b, o)
	return int64(int16(v)), err
}

func s16be(b []byte, o int) (int64, error) {
	v, err := u16be(b, o)
	return int64(int16(v)), err
}

func u32le(b []byte, o int) (int64, error) {
	if err := need(b, o, 4); err != nil {
		return 0, err
	}
	return int64(b[o]) | int64(b[o+1])<<8 | int64(b[o+2])<<16 | int64(b[o+3])<<24, nil
}

func u32be(b []byte, o int) (int64, error) {
	if err := need(b, o, 4); err != nil {
		return 0, err
	}
	return int64(b[o])<<24 | int64(b[o+1])<<16 | int64(b[o+2])<<8 | int64(b[o+3]), nil
}

func s32le(b []byte, o int) (int64, error) {
	v, err := u32le(b, o)
	return int64(int32(v)), err
}

func s32be(b []byte, o int) (int64, error) {
	v, err := u32be(b, o)
	return int64(int32(v)), err
}

// Python numbers parsed from text.

const saturate = int64(1) << 62

func isASCIISpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

// normalize applies the str-only step of int() and float(): non-ASCII
// whitespace becomes a space, other non-ASCII characters cannot parse
// (latin-1 has no decimal digits beyond ASCII).
func normalize(s []byte, isStr bool) ([]byte, bool) {
	out := make([]byte, 0, len(s))
	for _, c := range s {
		if c < 0x80 {
			out = append(out, c)
			continue
		}
		if !isStr {
			return nil, false
		}
		if c == 0x85 || c == 0xa0 {
			out = append(out, ' ')
			continue
		}
		return nil, false
	}
	return out, true
}

func trimSpace(s []byte) []byte {
	for len(s) > 0 && isASCIISpace(s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && isASCIISpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// digitsWithUnderscores checks Python's rule that underscores only
// separate digits, and returns the digits.
func digitsWithUnderscores(s []byte) (string, bool) {
	if len(s) == 0 {
		return "", false
	}
	var b strings.Builder
	for i, c := range s {
		switch {
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '_':
			if i == 0 || i == len(s)-1 || s[i-1] == '_' || s[i+1] < '0' || s[i+1] > '9' {
				return "", false
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// pyInt is int(s) in base 10 (bytes, or a latin-1 str when isStr). ok is
// false where Python raises ValueError; values beyond 2**62 saturate.
func pyInt(s []byte, isStr bool) (int64, bool) {
	v, ok := pyBigInt(s, isStr)
	if !ok {
		return 0, false
	}
	return clampBig(v), true
}

func pyBigInt(s []byte, isStr bool) (*big.Int, bool) {
	s, ok := normalize(s, isStr)
	if !ok {
		return nil, false
	}
	s = trimSpace(s)
	neg := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	digits, ok := digitsWithUnderscores(s)
	if !ok {
		return nil, false
	}
	v, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, false
	}
	if neg {
		v.Neg(v)
	}
	return v, true
}

func clampBig(v *big.Int) int64 {
	if v.Cmp(big.NewInt(saturate)) > 0 {
		return saturate
	}
	if v.Cmp(big.NewInt(-saturate)) < 0 {
		return -saturate
	}
	return v.Int64()
}

// pyFloat is float(s). ok is false where Python raises ValueError.
func pyFloat(s []byte, isStr bool) (float64, bool) {
	s, ok := normalize(s, isStr)
	if !ok {
		return 0, false
	}
	s = trimSpace(s)
	body := s
	sign := 1.0
	if len(body) > 0 && (body[0] == '+' || body[0] == '-') {
		if body[0] == '-' {
			sign = -1
		}
		body = body[1:]
	}
	switch strings.ToLower(string(body)) {
	case "inf", "infinity":
		return sign * math.Inf(1), true
	case "nan":
		return math.NaN(), true
	}
	// Grammar: digits [. [digits]] | . digits, then [e [sign] digits]; each
	// digit run may hold single underscores between digits.
	i := 0
	var clean strings.Builder
	run := func() bool {
		start := i
		for i < len(body) && (body[i] >= '0' && body[i] <= '9' || body[i] == '_') {
			i++
		}
		if i == start {
			return false
		}
		digits, ok := digitsWithUnderscores(body[start:i])
		if !ok {
			return false
		}
		clean.WriteString(digits)
		return true
	}
	intPart := run()
	if !intPart && i < len(body) && body[i] == '_' {
		return 0, false
	}
	frac := false
	if i < len(body) && body[i] == '.' {
		clean.WriteByte('.')
		i++
		frac = run()
	}
	if !intPart && !frac {
		return 0, false
	}
	if i < len(body) && (body[i] == 'e' || body[i] == 'E') {
		clean.WriteByte('e')
		i++
		if i < len(body) && (body[i] == '+' || body[i] == '-') {
			clean.WriteByte(body[i])
			i++
		}
		if !run() {
			return 0, false
		}
	}
	if i != len(body) {
		return 0, false
	}
	v, err := strconv.ParseFloat(clean.String(), 64)
	if err != nil {
		var numErr *strconv.NumError
		if !errors.As(err, &numErr) || numErr.Err != strconv.ErrRange {
			return 0, false
		}
	}
	return sign * v, true
}

// opened is a successful Image.open.
type opened struct {
	mode string
	w, h int
	load func() (*pil.Image, error)
}

func (o *opened) Size() (int, int) { return o.w, o.h }

func (o *opened) Load() (*pil.Image, error) { return o.load() }

// bombCheck is Image._decompression_bomb_check for sizes that may not fit
// an int64 product.
func bombCheck(w, h int64) error {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	limit := int64(pil.MaxImagePixels)
	if w > 2*limit || h > 2*limit {
		return &pil.BombError{Pixels: math.MaxInt64}
	}
	return pil.CheckBomb(int(w), int(h))
}

// finish is the rest of ImageFile.__init__ (an empty mode or a size <= 0 is
// "not identified by this driver") followed by _open_core's bomb check.
func finish(mode string, w, h int64, load func() (*pil.Image, error)) (pil.Opened, error) {
	if mode == "" || w <= 0 || h <= 0 {
		return nil, pil.Next("not identified by this driver")
	}
	if err := bombCheck(w, h); err != nil {
		return nil, err
	}
	return &opened{mode: mode, w: int(w), h: int(h), load: load}, nil
}

func loadFails(err error) func() (*pil.Image, error) {
	return func() (*pil.Image, error) { return nil, err }
}

func unsupported(format, reason string) func() (*pil.Image, error) {
	return loadFails(pil.Unsupported(format, "%s", reason))
}

// plugin assembles a pil.Plugin whose open errors follow _open_core.
func plugin(name string, accept func([]byte) bool, open func(f *file) (pil.Opened, error)) pil.Plugin {
	return pil.Plugin{
		Name:   name,
		Accept: accept,
		Open: func(data []byte) (pil.Opened, error) {
			o, err := open(&file{data: data})
			if err != nil {
				return nil, openError(err)
			}
			return o, nil
		},
	}
}

func hasPrefix(b []byte, prefixes ...string) bool {
	for _, p := range prefixes {
		if len(b) >= len(p) && string(b[:len(p)]) == p {
			return true
		}
	}
	return false
}
