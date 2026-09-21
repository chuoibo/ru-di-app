// Package exif reproduces Image.getexif and ImageOps.exif_transpose of
// Pillow 12.2.0 as sanitize_image calls them: IFD0 parsed with
// TiffImagePlugin.ImageFileDirectory_v2 semantics, the XMP fallback, the
// orientation value's Python type against the transpose table, and
// Image.transpose on the pixels.
package exif

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"strings"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Error is an exception getexif or exif_transpose raised; the sanitizer
// turns it into not_an_image.
type Error struct{ Msg string }

func (e *Error) Error() string { return "exif: " + e.Msg }

func fail(format string, args ...any) error { return &Error{Msg: fmt.Sprintf(format, args...)} }

const orientationTag = 0x0112

// valueKind is the Python type an IFD tag value decodes to.
type valueKind int

const (
	kindInt valueKind = iota + 1
	kindFloat
	kindRational
	kindBytes
	kindStr
)

// value is the first element of a decoded tag, as Exif.__getitem__ returns
// it after ImageFileDirectory_v2._setitem kept one value (Orientation has
// spec length 1) and Exif._fixup.
type value struct {
	kind     valueKind
	i        int64
	f        float64
	num, den int64
}

// method returns the key of ImageOps.exif_transpose's table the value
// selects (2..8), or 0. A dict lookup needs equal hashes and ==: floats
// and IFDRational (a Fraction) equal to an int hash like it; an
// IFDRational with denominator 0 is nan, which matches nothing.
func (v value) method() int {
	var k int64
	switch v.kind {
	case kindInt:
		k = v.i
	case kindFloat:
		if v.f != math.Trunc(v.f) || v.f < 2 || v.f > 8 {
			return 0
		}
		k = int64(v.f)
	case kindRational:
		if v.den == 0 || v.num%v.den != 0 {
			return 0
		}
		k = v.num / v.den
	default:
		return 0
	}
	if k >= 2 && k <= 8 {
		return int(k)
	}
	return 0
}

// ifdEntry is one stored tag of ImageFileDirectory_v2: its type and data.
type ifdEntry struct {
	typ  uint16
	data []byte
}

var unitSize = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4, 16: 8}

// reader is an io.BytesIO over the EXIF block or the TIFF file.
type reader struct {
	data []byte
	pos  int64
}

func (r *reader) read(n int64) []byte {
	if r.pos >= int64(len(r.data)) || n <= 0 {
		if r.pos < int64(len(r.data)) {
			return nil
		}
		return nil
	}
	end := r.pos + n
	if end > int64(len(r.data)) {
		end = int64(len(r.data))
	}
	out := r.data[r.pos:end]
	r.pos = end
	return out
}

// loadIFD is ImageFileDirectory_v2.load: it stops silently (a warning) at
// the first short read, keeping the tags stored so far.
func loadIFD(r *reader, order binary.ByteOrder, bigTIFF bool) map[uint16]ifdEntry {
	tags := map[uint16]ifdEntry{}
	var count uint64
	if bigTIFF {
		b := r.read(8)
		if len(b) != 8 {
			return tags
		}
		count = order.Uint64(b)
	} else {
		b := r.read(2)
		if len(b) != 2 {
			return tags
		}
		count = uint64(order.Uint16(b))
	}
	entrySize, inline := int64(12), 4
	if bigTIFF {
		entrySize, inline = 20, 8
	}
	for i := uint64(0); i < count; i++ {
		entry := r.read(entrySize)
		if int64(len(entry)) != entrySize {
			return tags
		}
		tag := order.Uint16(entry[0:])
		typ := order.Uint16(entry[2:])
		var n uint64
		var field []byte
		if bigTIFF {
			n, field = order.Uint64(entry[4:]), entry[12:20]
		} else {
			n, field = uint64(order.Uint32(entry[4:])), entry[8:12]
		}
		unit, ok := unitSize[typ]
		if !ok {
			continue
		}
		size := n * uint64(unit)
		var data []byte
		if size > uint64(inline) {
			var offset uint64
			if bigTIFF {
				offset = order.Uint64(field)
			} else {
				offset = uint64(order.Uint32(field))
			}
			here := r.pos
			if offset > 1<<62 || size > 1<<62 {
				return tags
			}
			r.pos = int64(offset)
			data = r.read(int64(size))
			if uint64(len(data)) < size {
				return tags
			}
			r.pos = here
		} else {
			data = field[:size]
		}
		if len(data) == 0 {
			continue
		}
		tags[tag] = ifdEntry{typ: typ, data: data}
	}
	return tags
}

func decodeValue(e ifdEntry, order binary.ByteOrder) value {
	d := e.data
	switch e.typ {
	case 1, 7:
		return value{kind: kindBytes}
	case 2:
		return value{kind: kindStr}
	case 3:
		return value{kind: kindInt, i: int64(order.Uint16(d))}
	case 4, 13:
		return value{kind: kindInt, i: int64(order.Uint32(d))}
	case 6:
		return value{kind: kindInt, i: int64(int8(d[0]))}
	case 8:
		return value{kind: kindInt, i: int64(int16(order.Uint16(d)))}
	case 9:
		return value{kind: kindInt, i: int64(int32(order.Uint32(d)))}
	case 16:
		u := order.Uint64(d)
		if u > math.MaxInt64 {
			return value{kind: kindInt, i: -1}
		}
		return value{kind: kindInt, i: int64(u)}
	case 5:
		return value{kind: kindRational, num: int64(order.Uint32(d)), den: int64(order.Uint32(d[4:]))}
	case 10:
		return value{kind: kindRational, num: int64(int32(order.Uint32(d))), den: int64(int32(order.Uint32(d[4:])))}
	case 11:
		return value{kind: kindFloat, f: float64(math.Float32frombits(order.Uint32(d)))}
	case 12:
		return value{kind: kindFloat, f: math.Float64frombits(order.Uint64(d))}
	}
	return value{}
}

var tiffPrefixes = [][]byte{
	[]byte("MM\x00\x2a"), []byte("II\x2a\x00"), []byte("MM\x2a\x00"),
	[]byte("II\x00\x2a"), []byte("MM\x00\x2b"), []byte("II\x2b\x00"),
}

func acceptTIFF(head []byte) bool {
	for _, prefix := range tiffPrefixes {
		if bytes.HasPrefix(head, prefix) {
			return true
		}
	}
	return false
}

// exifState is what getexif leaves behind that exif_transpose reads.
type exifState struct {
	present bool
	value   value
}

// loadBlock is Exif.load on bytes: nil tags for an empty block.
func loadBlock(data []byte) (map[uint16]ifdEntry, binary.ByteOrder, error) {
	for len(data) > 0 && bytes.HasPrefix(data, []byte("Exif\x00\x00")) {
		data = data[6:]
	}
	if len(data) == 0 {
		return nil, nil, nil
	}
	r := &reader{data: data}
	head := r.read(8)
	if !acceptTIFF(head) {
		return nil, nil, fail("SyntaxError: not a TIFF file (header %q not valid)", head)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if head[0] == 'M' {
		order = binary.BigEndian
	}
	if head[2] == 43 {
		return nil, nil, fail("struct.error: BigTIFF header in an 8-byte head")
	}
	if len(head) != 8 {
		return nil, nil, fail("struct.error: unpack requires a buffer of 4 bytes")
	}
	r.pos = int64(order.Uint32(head[4:]))
	return loadIFD(r, order, false), order, nil
}

// fromHex is bytes.fromhex on a str.
func fromHex(s string) ([]byte, error) {
	out := make([]byte, 0, len(s)/2)
	i := 0
	isSpace := func(c byte) bool { return c == ' ' || c >= '\t' && c <= '\r' }
	hexVal := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10
		}
		return -1
	}
	for {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			return out, nil
		}
		if i+1 >= len(s) {
			return nil, fail("ValueError: non-hexadecimal number found in fromhex() arg")
		}
		top, bot := hexVal(s[i]), hexVal(s[i+1])
		if top < 0 || bot < 0 {
			return nil, fail("ValueError: non-hexadecimal number found in fromhex() arg")
		}
		out = append(out, byte(top<<4|bot))
		i += 2
	}
}

var (
	xmpStr   = regexp.MustCompile(`tiff:Orientation(="|>)([0-9])`)
	xmpBytes = regexp.MustCompile(`tiff:Orientation(="|>)([0-9])`)
)

// getexif is Image.getexif for the orientation tag.
func getexif(info pil.Info) (exifState, error) {
	var tags map[uint16]ifdEntry
	var order binary.ByteOrder
	switch {
	case info.HasExif:
		var err error
		if tags, order, err = loadBlock(info.Exif); err != nil {
			return exifState{}, err
		}
	case info.HasRawProfileExif:
		lines := strings.Split(info.RawProfileExif, "\n")
		joined := ""
		if len(lines) > 3 {
			joined = strings.Join(lines[3:], "")
		}
		data, err := fromHex(joined)
		if err != nil {
			return exifState{}, err
		}
		if tags, order, err = loadBlock(data); err != nil {
			return exifState{}, err
		}
	case info.TIFF != nil:
		src := info.TIFF
		order = binary.LittleEndian
		if src.BigEndian {
			order = binary.BigEndian
		}
		r := &reader{data: src.File, pos: src.Offset}
		tags = loadIFD(r, order, src.BigTIFF)
	}
	if e, ok := tags[orientationTag]; ok {
		return exifState{present: true, value: decodeValue(e, order)}, nil
	}
	var match [][]byte
	if info.HasAdobeXMP && info.AdobeXMP != "" {
		if m := xmpStr.FindStringSubmatch(info.AdobeXMP); m != nil {
			match = [][]byte{nil, nil, []byte(m[2])}
		}
	} else if info.HasXMP && len(info.XMP) > 0 {
		match = xmpBytes.FindSubmatch(info.XMP)
	}
	if match != nil {
		return exifState{present: true, value: value{kind: kindInt, i: int64(match[2][0] - '0')}}, nil
	}
	return exifState{}, nil
}

// Orientation is the key of exif_transpose's method table that
// image.getexif().get(Orientation, 1) selects: 2..8, or 0 when the image
// is left as it is.
func Orientation(info pil.Info) (int, error) {
	state, err := getexif(info)
	if err != nil || !state.present {
		return 0, err
	}
	return state.value.method(), nil
}

// OrientationTagPresent reports ExifTags.Base.Orientation in the EXIF data
// getexif loads before its XMP fallback, or the exception loading raises.
func OrientationTagPresent(info pil.Info) (bool, error) {
	info.HasXMP, info.HasAdobeXMP = false, false
	state, err := getexif(info)
	return state.present, err
}

// Transpose is ImageOps.exif_transpose(image) after image.load(): a copy,
// or the image transposed by its orientation.
func Transpose(img *pil.Image) (*pil.Image, error) {
	state, err := getexif(img.Info)
	if err != nil {
		return nil, err
	}
	op := -1
	if state.present {
		switch state.value.method() {
		case 2:
			op = FlipLeftRight
		case 3:
			op = Rotate180
		case 4:
			op = FlipTopBottom
		case 5:
			op = TransposeOp
		case 6:
			op = Rotate270
		case 7:
			op = Transverse
		case 8:
			op = Rotate90
		}
	}
	if op < 0 {
		out := *img
		return &out, nil
	}
	transposed := Apply(img, op)
	// transposed_image.getexif() reads the same info keys (no tag_v2), and
	// when it finds the orientation, deleting it rewrites info["exif"] (or
	// the raw profile) with Exif.tobytes(), which can raise.
	info := img.Info
	info.TIFF = nil
	second, err := getexif(info)
	if err != nil {
		return nil, err
	}
	if second.present && (info.HasExif || info.HasRawProfileExif) {
		if err := tobytesError(info); err != nil {
			return nil, err
		}
	}
	return transposed, nil
}

// Image.Transpose operation numbers.
const (
	FlipLeftRight = 0
	FlipTopBottom = 1
	Rotate90      = 2
	Rotate180     = 3
	Rotate270     = 4
	TransposeOp   = 5
	Transverse    = 6
)

// Apply is Image.transpose(op): whole pixels move, the palette and info are
// copied.
func Apply(img *pil.Image, op int) *pil.Image {
	w, h := img.Width, img.Height
	stride := pil.BytesPerPixel(img.Mode)
	out := *img
	if op == Rotate90 || op == Rotate270 || op == TransposeOp || op == Transverse {
		out.Width, out.Height = h, w
	}
	pix := make([]byte, len(img.Pix))
	ow := out.Width
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var ox, oy int
			switch op {
			case FlipLeftRight:
				ox, oy = w-1-x, y
			case FlipTopBottom:
				ox, oy = x, h-1-y
			case Rotate90:
				ox, oy = y, w-1-x
			case Rotate180:
				ox, oy = w-1-x, h-1-y
			case Rotate270:
				ox, oy = h-1-y, x
			case TransposeOp:
				ox, oy = y, x
			case Transverse:
				ox, oy = h-1-y, w-1-x
			}
			src := (y*w + x) * stride
			dst := (oy*ow + ox) * stride
			copy(pix[dst:dst+stride], img.Pix[src:src+stride])
		}
	}
	out.Pix = pix
	return &out
}
