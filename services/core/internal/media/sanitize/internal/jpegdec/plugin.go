// Package jpegdec opens and decodes JPEG files exactly as Pillow 12.2.0
// does in the parity image: JpegImagePlugin and MpoImagePlugin for the
// header, ImageFile.load and JpegDecode.c for feeding, and a port of the
// libjpeg-turbo 3.1.4.1 decompressor (islow IDCT, fancy upsampling) for
// the pixels.
package jpegdec

import (
	"bytes"
	"encoding/binary"
	"errors"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Plugin is the JPEG entry of Image.OPEN.
var Plugin = pil.Plugin{Name: "JPEG", Accept: Accept, Open: Open}

// Accept is JpegImagePlugin._accept.
func Accept(prefix []byte) bool {
	return bytes.HasPrefix(prefix, []byte{0xFF, 0xD8, 0xFF})
}

// errTruncatedRead is the OSError ImageFile._safe_read raises; it escapes
// Image.open.
var errTruncatedRead = errors.New("OSError: Truncated File Read")

type segment struct {
	name string
	data []byte
}

// header is what JpegImageFile._open leaves on the instance.
type header struct {
	data []byte
	pos  int

	width, height int
	mode          string

	hasExif bool
	exif    []byte
	hasXMP  bool
	xmp     []byte
	hasDPI  bool
	hasMP   bool
	mp      []byte

	applist []segment
	icclist [][]byte
}

type opened struct {
	data   []byte
	width  int
	height int
	mode   string
	format string
	info   pil.Info
}

func (o *opened) Size() (int, int) { return o.width, o.height }

// Load is ImageFile.load for the JPEG (or MPO frame 0) tile.
func (o *opened) Load() (*pil.Image, error) {
	pix, err := decode(o.data, o.mode, o.width, o.height)
	if err != nil {
		return nil, err
	}
	return &pil.Image{
		Mode:   o.mode,
		Width:  o.width,
		Height: o.height,
		Pix:    pix,
		Info:   o.info,
		Format: o.format,
	}, nil
}

// Open is jpeg_factory: JpegImageFile(fp) and the MPO adoption.
func Open(data []byte) (pil.Opened, error) {
	h := &header{data: data}
	if err := h.open(); err != nil {
		return nil, err
	}
	if h.mode == "" || h.width <= 0 || h.height <= 0 {
		return nil, pil.Next("not identified by this driver")
	}
	format := "JPEG"
	switch h.classifyMP() {
	case mpIsMPO:
		format = "MPO"
	case mpStructError:
		return nil, pil.Next("struct.error unpacking the MP entries")
	}
	o := &opened{data: data, width: h.width, height: h.height, mode: h.mode, format: format}
	o.info.HasExif = h.hasExif
	o.info.Exif = h.exif
	o.info.HasXMP = h.hasXMP
	o.info.XMP = h.xmp
	return o, nil
}

func (h *header) read(n int) []byte {
	end := min(h.pos+n, len(h.data))
	if h.pos > end {
		return nil
	}
	b := h.data[h.pos:end]
	h.pos = end
	return b
}

// segmentData is `n = i16(fp.read(2)) - 2; s = ImageFile._safe_read(fp, n)`.
func (h *header) segmentData() ([]byte, error) {
	lb := h.read(2)
	if len(lb) < 2 {
		return nil, pil.Next("struct.error reading a segment length")
	}
	n := int(lb[0])<<8 | int(lb[1]) - 2
	if n <= 0 {
		return nil, nil
	}
	if h.pos+n > len(h.data) {
		h.pos = len(h.data)
		return nil, errTruncatedRead
	}
	s := h.data[h.pos : h.pos+n]
	h.pos += n
	return s, nil
}

// open is JpegImageFile._open, with the exception classes ImageFile.__init__
// turns into SyntaxError mapped to pil.NextError.
func (h *header) open() error {
	if !Accept(h.read(3)) {
		return pil.Next("not a JPEG file")
	}
	s := []byte{0xFF}
	for {
		if len(s) == 0 {
			return pil.Next("IndexError: end of data")
		}
		i := int(s[0])
		if i != 0xFF {
			s = h.read(1)
			continue
		}
		s = append([]byte{0xFF}, h.read(1)...)
		if len(s) < 2 {
			return pil.Next("struct.error reading a marker")
		}
		i = int(s[0])<<8 | int(s[1])
		switch {
		case i >= 0xFFC0 && i <= 0xFFFE:
			if err := h.handle(i); err != nil {
				return err
			}
			if i == 0xFFDA {
				return h.readDPIFromExif()
			}
			s = h.read(1)
		case i == 0xFFFF:
			s = []byte{0xFF}
		case i == 0xFF00:
			s = h.read(1)
		default:
			return pil.Next("no marker found")
		}
	}
}

func (h *header) handle(marker int) error {
	switch {
	case marker == 0xFFC8, marker >= 0xFFD0 && marker <= 0xFFD9, marker >= 0xFFF0 && marker <= 0xFFFD:
		return nil
	case marker == 0xFFC4, marker == 0xFFCC, marker == 0xFFDA, marker == 0xFFDC, marker == 0xFFDD, marker == 0xFFDF:
		_, err := h.segmentData()
		return err
	case marker == 0xFFDB:
		return h.dqt()
	case marker >= 0xFFE0 && marker <= 0xFFEF:
		return h.app(marker)
	case marker == 0xFFFE:
		s, err := h.segmentData()
		if err == nil {
			h.applist = append(h.applist, segment{name: "COM", data: s})
		}
		return err
	default:
		return h.sof(marker)
	}
}

var appNames = [16]string{
	"APP0", "APP1", "APP2", "APP3", "APP4", "APP5", "APP6", "APP7",
	"APP8", "APP9", "APP10", "APP11", "APP12", "APP13", "APP14", "APP15",
}

var xmpPrefix = []byte("http://ns.adobe.com/xap/1.0/\x00")

func (h *header) app(marker int) error {
	s, err := h.segmentData()
	if err != nil {
		return err
	}
	h.applist = append(h.applist, segment{name: appNames[marker&15], data: s})
	switch {
	case marker == 0xFFE0 && bytes.HasPrefix(s, []byte("JFIF")):
		if len(s) < 7 {
			return pil.Next("struct.error reading the JFIF version")
		}
		if len(s) >= 12 && (s[7] == 1 || s[7] == 2) {
			h.hasDPI = true
		}
	case marker == 0xFFE1 && bytes.HasPrefix(s, []byte("Exif\x00\x00")):
		if h.hasExif {
			h.exif = append(h.exif, s[6:]...)
		} else {
			h.exif = append([]byte(nil), s...)
			h.hasExif = true
		}
	case marker == 0xFFE1 && bytes.HasPrefix(s, xmpPrefix):
		h.xmp = s[bytes.IndexByte(s, 0)+1:]
		h.hasXMP = true
	case marker == 0xFFE2 && bytes.HasPrefix(s, []byte("FPXR\x00")):
	case marker == 0xFFE2 && bytes.HasPrefix(s, []byte("ICC_PROFILE\x00")):
		h.icclist = append(h.icclist, s)
	case marker == 0xFFED && bytes.HasPrefix(s, []byte("Photoshop 3.0\x00")):
		return photoshopResources(s)
	case marker == 0xFFEE && bytes.HasPrefix(s, []byte("Adobe")):
		if len(s) < 7 {
			return pil.Next("struct.error reading the Adobe version")
		}
	case marker == 0xFFE2 && bytes.HasPrefix(s, []byte("MPF\x00")):
		h.mp = s[4:]
		h.hasMP = true
	}
	return nil
}

// photoshopResources walks the 8BIM resources the way the APP13 handler
// does: a struct.error ends the walk quietly, but an IndexError reading a
// name length escapes.
func photoshopResources(s []byte) error {
	offset := 14
	for offset+4 <= len(s) && string(s[offset:offset+4]) == "8BIM" {
		offset += 4
		if offset+2 > len(s) {
			return nil
		}
		code := int(binary.BigEndian.Uint16(s[offset:]))
		offset += 2
		if offset >= len(s) {
			return pil.Next("IndexError reading a Photoshop resource name")
		}
		offset += 1 + int(s[offset])
		offset += offset & 1
		if offset+4 > len(s) {
			return nil
		}
		size := int(binary.BigEndian.Uint32(s[offset:]))
		offset += 4
		if code == 0x03ED {
			// ResolutionInfo unpacks 14 bytes; fewer raise struct.error.
			dataLen := 0
			if offset < len(s) {
				dataLen = min(size, len(s)-offset)
			}
			if dataLen < 14 {
				return nil
			}
		}
		offset += size
		offset += offset & 1
	}
	return nil
}

func (h *header) sof(marker int) error {
	s, err := h.segmentData()
	if err != nil {
		return err
	}
	if len(s) < 5 {
		return pil.Next("struct.error reading the frame size")
	}
	h.height = int(s[1])<<8 | int(s[2])
	h.width = int(s[3])<<8 | int(s[4])
	if s[0] != 8 {
		return pil.Next("cannot handle %d-bit layers", s[0])
	}
	if len(s) < 6 {
		return pil.Next("IndexError reading the layer count")
	}
	switch s[5] {
	case 1:
		h.mode = "L"
	case 3:
		h.mode = "RGB"
	case 4:
		h.mode = "CMYK"
	default:
		return pil.Next("cannot handle %d-layer images", s[5])
	}
	if len(h.icclist) > 0 {
		first := h.icclist[0]
		for _, p := range h.icclist[1:] {
			if bytes.Compare(p, first) < 0 {
				first = p
			}
		}
		if len(first) < 14 {
			return pil.Next("IndexError reading the ICC fragment count")
		}
		h.icclist = nil
	}
	for i := 6; i < len(s); i += 3 {
		if i+3 > len(s) {
			return pil.Next("IndexError reading a layer")
		}
	}
	return nil
}

func (h *header) dqt() error {
	s, err := h.segmentData()
	if err != nil {
		return err
	}
	for len(s) > 0 {
		precision := 1
		if s[0]/16 != 0 {
			precision = 2
		}
		qtLength := 1 + precision*64
		if len(s) < qtLength {
			return pil.Next("bad quantization table marker")
		}
		s = s[qtLength:]
	}
	return nil
}

// readDPIFromExif is JpegImageFile._read_dpi_from_exif: it reaches
// Image.getexif at open time, and only an IndexError escapes it.
func (h *header) readDPIFromExif() error {
	if h.hasDPI || !h.hasExif {
		return nil
	}
	data := h.exif
	for len(data) > 0 && bytes.HasPrefix(data, []byte("Exif\x00\x00")) {
		data = data[6:]
	}
	if len(data) == 0 {
		return nil
	}
	dir, ok := newIFD(data[:min(8, len(data))], data)
	if !ok || !dir.has(0x0128) || !dir.has(0x011A) {
		return nil
	}
	if dpiRaisesIndexError(dir.value(0x011A)) {
		return pil.Next("IndexError reading XResolution")
	}
	return nil
}

const (
	mpIsJPEG = iota
	mpIsMPO
	mpStructError
)

// classifyMP is jpeg_factory's use of _getmp: MPO when the MP index lists
// more than one image (and the file is not Ultra HDR), JPEG when _getmp
// returns None or raises SyntaxError, TypeError or IndexError, and a
// struct.error escaping the entry loop fails Image.open's JPEG attempt.
func (h *header) classifyMP() int {
	if !h.hasMP {
		return mpIsJPEG
	}
	data := h.mp
	head := data[:min(8, len(data))]
	var order binary.ByteOrder = binary.LittleEndian
	if bytes.HasPrefix(head, []byte("MM\x00\x2a")) {
		order = binary.BigEndian
	}
	dir, ok := newIFD(head, data)
	if !ok || !dir.has(0xB001) {
		return mpIsJPEG
	}
	quant := dir.value(0xB001)
	if !dir.has(0xB002) {
		return mpIsJPEG
	}
	if quant.kind != pyInt {
		return mpIsJPEG
	}
	if !quant.big && quant.Int <= 0 {
		return mpIsJPEG
	}
	raw := dir.value(0xB002)
	if raw.kind != pyBytes {
		return mpIsJPEG
	}
	count := quant.Int
	for entry := int64(0); quant.big || entry < count; entry++ {
		off := entry * 16
		if off+16 > int64(len(raw.b)) {
			return mpStructError
		}
		attribute := order.Uint32(raw.b[off:])
		if (attribute>>24)&7 != 0 {
			return mpIsJPEG
		}
	}
	if count > 1 {
		for _, seg := range h.applist {
			if seg.name == "APP1" && bytes.Contains(seg.data, []byte(` hdrgm:Version="`)) {
				return mpIsJPEG
			}
		}
		return mpIsMPO
	}
	return mpIsJPEG
}
