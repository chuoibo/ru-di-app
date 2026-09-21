package jpegdec

import (
	"bytes"
	"encoding/binary"
	"math"
)

// This file reproduces the part of TiffImagePlugin.ImageFileDirectory_v2
// (Pillow 12.2.0) that JpegImagePlugin reaches while opening a file: the
// MP index behind jpeg_factory's MPO decision, and IFD0 of the Exif block
// that _read_dpi_from_exif reads. Only tags whose TagInfo length is 1 are
// queried, so a value is always the first element Pillow unpacks.

// tiffPrefixes is TiffImagePlugin.PREFIXES.
var tiffPrefixes = [][]byte{
	[]byte("MM\x00\x2a"),
	[]byte("II\x2a\x00"),
	[]byte("MM\x2a\x00"),
	[]byte("II\x00\x2a"),
	[]byte("MM\x00\x2b"),
	[]byte("II\x2b\x00"),
}

// tiffUnitSize is the unit size of every type in _load_dispatch.
var tiffUnitSize = map[uint16]int{
	1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4, 16: 8,
}

// ifd is a loaded directory: raw tag data and types, as _tagdata and
// tagtype hold them.
type ifd struct {
	order binary.ByteOrder
	data  map[uint16][]byte
	types map[uint16]uint16
}

// newIFD is ImageFileDirectory_v2(head) followed by fp.seek(next) and
// load(fp) over buf. ok is false when the constructor raises (SyntaxError
// for a bad prefix, struct.error for a short head, which includes every
// BigTIFF head because it is read from an 8-byte header).
func newIFD(head, buf []byte) (dir *ifd, ok bool) {
	accepted := false
	for _, prefix := range tiffPrefixes {
		if bytes.HasPrefix(head, prefix) {
			accepted = true
			break
		}
	}
	if !accepted {
		return nil, false
	}
	var order binary.ByteOrder = binary.LittleEndian
	if head[0] == 'M' {
		order = binary.BigEndian
	}
	if head[2] == 43 || len(head) != 8 {
		return nil, false
	}
	next := int64(order.Uint32(head[4:8]))
	dir = &ifd{order: order, data: map[uint16][]byte{}, types: map[uint16]uint16{}}
	dir.load(buf, next)
	return dir, true
}

// load is ImageFileDirectory_v2.load: an OSError (a short read) ends the
// directory quietly, keeping the tags read so far.
func (d *ifd) load(buf []byte, pos int64) {
	read := func(n int64) ([]byte, bool) {
		if pos < 0 || pos+n > int64(len(buf)) {
			return nil, false
		}
		out := buf[pos : pos+n]
		pos += n
		return out, true
	}
	raw, ok := read(2)
	if !ok {
		return
	}
	count := int(d.order.Uint16(raw))
	for i := 0; i < count; i++ {
		entry, ok := read(12)
		if !ok {
			return
		}
		tag := d.order.Uint16(entry[0:2])
		typ := d.order.Uint16(entry[2:4])
		n := int64(d.order.Uint32(entry[4:8]))
		unit, known := tiffUnitSize[typ]
		if !known {
			continue
		}
		size := n * int64(unit)
		var value []byte
		if size > 4 {
			offset := int64(d.order.Uint32(entry[8:12]))
			if offset+size > int64(len(buf)) {
				// _safe_read raises OSError: load stops here.
				return
			}
			value = buf[offset : offset+size]
		} else {
			value = entry[8 : 8+size]
		}
		if len(value) == 0 {
			continue
		}
		d.data[tag] = value
		d.types[tag] = typ
	}
}

func (d *ifd) has(tag uint16) bool {
	_, ok := d.data[tag]
	return ok
}

// pyKind is the Python type of a tag value.
type pyKind int

const (
	pyInt pyKind = iota + 1
	pyFloat
	pyRational
	pyBytes
	pyStr
)

// pyValue is a tag value as Exif.__getitem__ returns it for a TagInfo of
// length 1.
type pyValue struct {
	kind pyKind
	// Int holds pyInt values; a LONG8 above the int64 range sets big.
	Int  int64
	big  bool
	f    float64
	num  int64
	den  int64
	b    []byte
	text []byte // latin-1 bytes of a str; one byte per character
}

// value returns the first element Pillow unpacks for tag.
func (d *ifd) value(tag uint16) pyValue {
	data := d.data[tag]
	o := d.order
	switch d.types[tag] {
	case 1, 7:
		return pyValue{kind: pyBytes, b: data}
	case 2:
		if bytes.HasSuffix(data, []byte{0}) {
			data = data[:len(data)-1]
		}
		return pyValue{kind: pyStr, text: data}
	case 3:
		return pyValue{kind: pyInt, Int: int64(o.Uint16(data))}
	case 4, 13:
		return pyValue{kind: pyInt, Int: int64(o.Uint32(data))}
	case 6:
		return pyValue{kind: pyInt, Int: int64(int8(data[0]))}
	case 8:
		return pyValue{kind: pyInt, Int: int64(int16(o.Uint16(data)))}
	case 9:
		return pyValue{kind: pyInt, Int: int64(int32(o.Uint32(data)))}
	case 16:
		v := o.Uint64(data)
		return pyValue{kind: pyInt, Int: int64(v), big: v > math.MaxInt64}
	case 11:
		return pyValue{kind: pyFloat, f: float64(math.Float32frombits(o.Uint32(data)))}
	case 12:
		return pyValue{kind: pyFloat, f: math.Float64frombits(o.Uint64(data))}
	case 5:
		return pyValue{kind: pyRational, num: int64(o.Uint32(data)), den: int64(o.Uint32(data[4:]))}
	case 10:
		return pyValue{kind: pyRational, num: int64(int32(o.Uint32(data))), den: int64(int32(o.Uint32(data[4:])))}
	}
	return pyValue{}
}

// dpiRaisesIndexError reports whether _read_dpi_from_exif lets an
// IndexError escape for this XResolution value: float(x[0]) / x[1] on a
// one-byte bytes value, on an empty str, or on a one-digit str (float()
// of a single character only succeeds for an ASCII digit). Every other
// failure there is caught and the dpi falls back to 72.
func dpiRaisesIndexError(v pyValue) bool {
	switch v.kind {
	case pyBytes:
		return len(v.b) == 1
	case pyStr:
		if len(v.text) == 0 {
			return true
		}
		return len(v.text) == 1 && v.text[0] >= '0' && v.text[0] <= '9'
	}
	return false
}
