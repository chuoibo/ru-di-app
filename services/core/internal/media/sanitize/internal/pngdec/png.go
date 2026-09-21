// Package pngdec reproduces how Pillow 12.2.0's PngImagePlugin opens and
// loads a PNG: the Python chunk stream with its exception classes, the
// ImageFile.load loop with PngImageFile.load_read/load_end, ZipDecode.c on
// top of the linked zlib-ng inflate, and the Unpack.c rawmodes.
package pngdec

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"unicode/utf8"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Plugin is the PNG entry of Image.OPEN.
var Plugin = pil.Plugin{Name: "PNG", Accept: Accept, Open: Open}

var magic = []byte("\x89PNG\r\n\x1a\n")

const (
	safeBlock     = 1 << 20 // ImageFile.SAFEBLOCK
	maxTextChunk  = safeBlock
	maxTextMemory = 64 * maxTextChunk
	decoderBlock  = 1 << 16 // ImageFile.MAXBLOCK, the decoder read size
	intMax        = 1<<31 - 1
)

// Accept is PngImagePlugin._accept.
func Accept(prefix []byte) bool { return bytes.HasPrefix(prefix, magic) }

// pyExc names the Python exception class a step raised.
type pyExc int

const (
	excNone pyExc = iota
	excEOF
	excAttribute
	excSyntax
	excStruct
	excIndex
	excType
	excValue
	excOS
	excOverflow
	excMemory
)

func (e pyExc) next() bool {
	return e == excSyntax || e == excStruct || e == excIndex || e == excType
}

// pyError carries a Python exception out of the emulation.
type pyError struct {
	exc pyExc
	msg string
}

func (e *pyError) Error() string { return e.msg }

func raise(exc pyExc, format string, args ...any) *pyError {
	return &pyError{exc: exc, msg: fmt.Sprintf(format, args...)}
}

// openError maps an exception escaping PngImageFile._open to what
// Image.open does with it.
func openError(err *pyError) error {
	if err.exc.next() {
		return pil.Next("PNG: %s", err.msg)
	}
	return err
}

// pyKind is the Python type of an info value.
type pyKind int

const (
	pyInt pyKind = iota + 1
	pyBool
	pyFloat
	pyBytes
	pyStr
	pyTuple
)

type pyValue struct {
	kind  pyKind
	i     int64
	f     float64
	b     []byte
	s     string
	tuple []int64
}

func (v pyValue) truthy() bool {
	switch v.kind {
	case pyInt, pyBool:
		return v.i != 0
	case pyFloat:
		return v.f != 0
	case pyBytes:
		return len(v.b) > 0
	case pyStr:
		return v.s != ""
	case pyTuple:
		return len(v.tuple) > 0
	}
	return false
}

type chunkRef struct {
	cid    []byte
	pos    int
	length int
}

type tile struct {
	extents pyValue
	offset  int
	rawmode string
}

// pngFile is PngImageFile together with its PngStream.
type pngFile struct {
	data []byte
	pos  int

	queue []chunkRef

	info        map[string]pyValue
	width       int64
	height      int64
	mode        string
	rawmode     string
	hasRawmode  bool
	tile        *tile
	palette     []byte
	hasPalette  bool
	nFrames     int64
	hasNFrames  bool
	seqNum      int64
	hasSeqNum   bool
	textMemory  int64
	prepareIDAT int
	animated    bool
}

// read is fp.read(n) on a BytesIO.
func (p *pngFile) read(n int) []byte {
	if n < 0 {
		n = 0
	}
	end := p.pos + n
	if end > len(p.data) || end < p.pos {
		end = len(p.data)
	}
	if p.pos > len(p.data) {
		return nil
	}
	out := p.data[p.pos:end]
	p.pos = end
	return out
}

// safeRead is ImageFile._safe_read.
func (p *pngFile) safeRead(size int) ([]byte, *pyError) {
	if size <= 0 {
		return nil, nil
	}
	data := p.read(size)
	if len(data) < size {
		return nil, raise(excOS, "Truncated File Read")
	}
	return data, nil
}

func i32(s []byte, off int) (int64, *pyError) {
	if off < 0 || len(s) < off+4 {
		return 0, raise(excStruct, "unpack_from requires a buffer of at least %d bytes", off+4)
	}
	return int64(uint32(s[off])<<24 | uint32(s[off+1])<<16 | uint32(s[off+2])<<8 | uint32(s[off+3])), nil
}

func i16(s []byte, off int) (int64, *pyError) {
	if off < 0 || len(s) < off+2 {
		return 0, raise(excStruct, "unpack_from requires a buffer of at least %d bytes", off+2)
	}
	return int64(uint16(s[off])<<8 | uint16(s[off+1])), nil
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isCID is re.compile(rb"\w\w\w\w").match.
func isCID(cid []byte) bool {
	if len(cid) < 4 {
		return false
	}
	for _, c := range cid[:4] {
		if !isWordByte(c) {
			return false
		}
	}
	return true
}

// readChunk is ChunkStream.read.
func (p *pngFile) readChunk() (chunkRef, *pyError) {
	if n := len(p.queue); n > 0 {
		ref := p.queue[n-1]
		p.queue = p.queue[:n-1]
		p.pos = ref.pos
		if !isCID(ref.cid) {
			return ref, raise(excSyntax, "broken PNG file (chunk %q)", ref.cid)
		}
		return ref, nil
	}
	s := p.read(8)
	var cid []byte
	if len(s) > 4 {
		cid = s[4:]
	}
	pos := p.pos
	length, err := i32(s, 0)
	if err != nil {
		return chunkRef{}, err
	}
	if !isCID(cid) {
		return chunkRef{}, raise(excSyntax, "broken PNG file (chunk %q)", cid)
	}
	return chunkRef{cid: cid, pos: pos, length: int(length)}, nil
}

// checkCRC is ChunkStream.crc.
func (p *pngFile) checkCRC(cid, data []byte) *pyError {
	crc := crc32.Update(crc32.ChecksumIEEE(cid), crc32.IEEETable, data)
	stored, err := i32(p.read(4), 0)
	if err != nil {
		return raise(excSyntax, "broken PNG file (incomplete checksum in %q)", cid)
	}
	if int64(crc) != stored {
		return raise(excSyntax, "broken PNG file (bad header checksum in %q)", cid)
	}
	return nil
}

// latin1 decodes latin-1 bytes into a Go (UTF-8) string.
func latin1(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

func (p *pngFile) checkTextMemory(n int64) *pyError {
	p.textMemory += n
	if p.textMemory > maxTextMemory {
		return raise(excValue, "Too much memory used in text chunks")
	}
	return nil
}

var pngModes = map[[2]byte][2]string{
	{1, 0}:  {"1", "1"},
	{2, 0}:  {"L", "L;2"},
	{4, 0}:  {"L", "L;4"},
	{8, 0}:  {"L", "L"},
	{16, 0}: {"I;16", "I;16B"},
	{8, 2}:  {"RGB", "RGB"},
	{16, 2}: {"RGB", "RGB;16B"},
	{1, 3}:  {"P", "P;1"},
	{2, 3}:  {"P", "P;2"},
	{4, 3}:  {"P", "P;4"},
	{8, 3}:  {"P", "P"},
	{8, 4}:  {"LA", "LA"},
	{16, 4}: {"RGBA", "LA;16B"},
	{8, 6}:  {"RGBA", "RGBA"},
	{16, 6}: {"RGBA", "RGBA;16B"},
}

// simplePalette is _simple_palette.match: ^\xff*\x00\xff*$ where $ also
// matches before a final newline.
func simplePalette(s []byte) bool {
	i := 0
	for i < len(s) && s[i] == 0xff {
		i++
	}
	if i >= len(s) || s[i] != 0 {
		return false
	}
	i++
	for i < len(s) && s[i] == 0xff {
		i++
	}
	return i == len(s) || i == len(s)-1 && s[i] == '\n'
}

// call is ChunkStream.call: the chunk handler's result, or the exception it
// raised (excAttribute for a chunk type PngStream has no handler for).
func (p *pngFile) call(cid []byte, pos, length int) ([]byte, *pyError) {
	switch string(cid) {
	case "iCCP":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		i := bytes.IndexByte(s, 0)
		if i+1 >= len(s) {
			return nil, raise(excIndex, "index out of range")
		}
		if s[i+1] != 0 {
			return nil, raise(excSyntax, "Unknown compression method %d in iCCP chunk", s[i+1])
		}
		if _, tooLarge, _ := pyDecompress(s[i+2:], maxTextChunk); tooLarge {
			return nil, raise(excValue, "Decompressed data too large")
		}
		return s, nil
	case "IHDR":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if length < 13 {
			return nil, raise(excValue, "Truncated IHDR chunk")
		}
		w, _ := i32(s, 0)
		h, _ := i32(s, 4)
		p.width, p.height = w, h
		if modes, ok := pngModes[[2]byte{s[8], s[9]}]; ok {
			p.mode, p.rawmode, p.hasRawmode = modes[0], modes[1], true
		}
		if s[12] != 0 {
			p.info["interlace"] = pyValue{kind: pyInt, i: 1}
		}
		if s[11] != 0 {
			return nil, raise(excSyntax, "unknown filter category")
		}
		return s, nil
	case "IDAT":
		return nil, p.chunkIDAT(pos, length)
	case "IEND":
		return nil, raise(excEOF, "end of PNG image")
	case "PLTE":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if p.mode == "P" {
			p.palette, p.hasPalette = s, true
		}
		return s, nil
	case "tRNS":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		switch p.mode {
		case "P":
			if simplePalette(s) {
				p.info["transparency"] = pyValue{kind: pyInt, i: int64(bytes.IndexByte(s, 0))}
			} else {
				p.info["transparency"] = pyValue{kind: pyBytes, b: s}
			}
		case "1":
			v, err := i16(s, 0)
			if err != nil {
				return nil, err
			}
			if v != 0 {
				v = 255
			}
			p.info["transparency"] = pyValue{kind: pyInt, i: v}
		case "L", "I;16":
			v, err := i16(s, 0)
			if err != nil {
				return nil, err
			}
			p.info["transparency"] = pyValue{kind: pyInt, i: v}
		case "RGB":
			r, err := i16(s, 0)
			if err != nil {
				return nil, err
			}
			g, err := i16(s, 2)
			if err != nil {
				return nil, err
			}
			b, err := i16(s, 4)
			if err != nil {
				return nil, err
			}
			p.info["transparency"] = pyValue{kind: pyTuple, tuple: []int64{r, g, b}}
		}
		return s, nil
	case "gAMA":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if _, err := i32(s, 0); err != nil {
			return nil, err
		}
		return s, nil
	case "cHRM":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if len(s)%4 != 0 {
			return nil, raise(excStruct, "unpack requires a buffer of %d bytes", len(s)/4*4)
		}
		return s, nil
	case "sRGB":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if length < 1 {
			return nil, raise(excValue, "Truncated sRGB chunk")
		}
		return s, nil
	case "pHYs":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if length < 9 {
			return nil, raise(excValue, "Truncated pHYs chunk")
		}
		return s, nil
	case "tEXt":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		k, v := s, []byte(nil)
		if i := bytes.IndexByte(s, 0); i >= 0 {
			k, v = s[:i], s[i+1:]
		}
		if len(k) > 0 {
			key := latin1(k)
			if string(k) == "exif" {
				p.info[key] = pyValue{kind: pyBytes, b: v}
			} else {
				p.info[key] = pyValue{kind: pyStr, s: latin1(v)}
			}
			if err := p.checkTextMemory(int64(len(v))); err != nil {
				return nil, err
			}
		}
		return s, nil
	case "zTXt":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		k, v := s, []byte(nil)
		if i := bytes.IndexByte(s, 0); i >= 0 {
			k, v = s[:i], s[i+1:]
		}
		method := byte(0)
		if len(v) > 0 {
			method = v[0]
		}
		if method != 0 {
			return nil, raise(excSyntax, "Unknown compression method %d in zTXt chunk", method)
		}
		var text []byte
		if len(v) > 0 {
			out, tooLarge, zerr := pyDecompress(v[1:], maxTextChunk)
			switch {
			case tooLarge:
				return nil, raise(excValue, "Decompressed data too large")
			case zerr:
				text = nil
			default:
				text = out
			}
		}
		if len(k) > 0 {
			p.info[latin1(k)] = pyValue{kind: pyStr, s: latin1(text)}
			if err := p.checkTextMemory(int64(len(text))); err != nil {
				return nil, err
			}
		}
		return s, nil
	case "iTXt":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		i := bytes.IndexByte(s, 0)
		if i < 0 {
			return s, nil
		}
		k, r := s[:i], s[i+1:]
		if len(r) < 2 {
			return s, nil
		}
		cf, cm, r := r[0], r[1], r[2:]
		j := bytes.IndexByte(r, 0)
		if j < 0 {
			return s, nil
		}
		lang, rest := r[:j], r[j+1:]
		j = bytes.IndexByte(rest, 0)
		if j < 0 {
			return s, nil
		}
		tk, v := rest[:j], rest[j+1:]
		if cf != 0 {
			if cm != 0 {
				return s, nil
			}
			out, tooLarge, zerr := pyDecompress(v, maxTextChunk)
			if tooLarge {
				return nil, raise(excValue, "Decompressed data too large")
			}
			if zerr {
				return s, nil
			}
			v = out
		}
		if string(k) == "XML:com.adobe.xmp" {
			p.info["xmp"] = pyValue{kind: pyBytes, b: v}
		}
		if !utf8.Valid(lang) || !utf8.Valid(tk) || !utf8.Valid(v) {
			return s, nil
		}
		p.info[latin1(k)] = pyValue{kind: pyStr, s: string(v)}
		if err := p.checkTextMemory(int64(utf8.RuneCount(v))); err != nil {
			return nil, err
		}
		return s, nil
	case "eXIf":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		p.info["exif"] = pyValue{kind: pyBytes, b: append([]byte("Exif\x00\x00"), s...)}
		return s, nil
	case "acTL":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if length < 8 {
			return nil, raise(excValue, "APNG contains truncated acTL chunk")
		}
		if p.hasNFrames {
			p.hasNFrames = false
			return s, nil
		}
		n, _ := i32(s, 0)
		if n == 0 || n > 0x80000000 {
			return s, nil
		}
		p.nFrames, p.hasNFrames = n, true
		loop, _ := i32(s, 4)
		p.info["loop"] = pyValue{kind: pyInt, i: loop}
		return s, nil
	case "fcTL":
		s, err := p.safeRead(length)
		if err != nil {
			return nil, err
		}
		if length < 26 {
			return nil, raise(excValue, "APNG contains truncated fcTL chunk")
		}
		seq, _ := i32(s, 0)
		if !p.hasSeqNum && seq != 0 || p.hasSeqNum && p.seqNum != seq-1 {
			return nil, raise(excSyntax, "APNG contains frame sequence errors")
		}
		p.seqNum, p.hasSeqNum = seq, true
		w, _ := i32(s, 4)
		h, _ := i32(s, 8)
		px, _ := i32(s, 12)
		py, _ := i32(s, 16)
		if px+w > p.width || py+h > p.height {
			return nil, raise(excSyntax, "APNG contains invalid frames")
		}
		p.info["bbox"] = pyValue{kind: pyTuple, tuple: []int64{px, py, px + w, py + h}}
		p.info["disposal"] = pyValue{kind: pyInt, i: int64(s[24])}
		p.info["blend"] = pyValue{kind: pyInt, i: int64(s[25])}
		return s, nil
	case "fdAT":
		if length < 4 {
			return nil, raise(excValue, "APNG contains truncated fDAT chunk")
		}
		s, err := p.safeRead(4)
		if err != nil {
			return nil, err
		}
		seq, _ := i32(s, 0)
		if !p.hasSeqNum || p.seqNum != seq-1 {
			return nil, raise(excSyntax, "APNG contains frame sequence errors")
		}
		p.seqNum = seq
		return nil, p.chunkIDAT(pos+4, length-4)
	}
	return nil, raise(excAttribute, "no handler for %q", cid)
}

// chunkIDAT is PngStream.chunk_IDAT; it always raises.
func (p *pngFile) chunkIDAT(pos, length int) *pyError {
	if bbox, ok := p.info["bbox"]; ok {
		if !p.hasRawmode {
			return raise(excAttribute, "im_rawmode")
		}
		p.tile = &tile{extents: bbox, offset: pos, rawmode: p.rawmode}
	} else {
		if p.hasNFrames {
			p.info["default_image"] = pyValue{kind: pyBool, i: 1}
		}
		if !p.hasRawmode {
			return raise(excAttribute, "im_rawmode")
		}
		p.tile = &tile{
			extents: pyValue{kind: pyTuple, tuple: []int64{0, 0, p.width, p.height}},
			offset:  pos,
			rawmode: p.rawmode,
		}
	}
	return raise(excEOF, "image data found")
}

// Open is Image.open's PNG factory: PngImageFile._open.
func Open(data []byte) (pil.Opened, error) {
	if !Accept(data) {
		return nil, pil.Next("not a PNG file")
	}
	p := &pngFile{data: data, pos: 8, info: map[string]pyValue{}}
	var last chunkRef
	for {
		ref, err := p.readChunk()
		if err != nil {
			return nil, openError(err)
		}
		last = ref
		s, err := p.call(ref.cid, ref.pos, ref.length)
		if err != nil {
			if err.exc == excEOF {
				break
			}
			if err.exc != excAttribute {
				return nil, openError(err)
			}
			if s, err = p.safeRead(ref.length); err != nil {
				return nil, openError(err)
			}
		}
		if err := p.checkCRC(ref.cid, s); err != nil {
			return nil, openError(err)
		}
	}
	// ImageFile.__init__: a plugin that leaves no mode or a non-positive
	// size is "not identified by this driver" (SyntaxError).
	if p.mode == "" || p.width <= 0 || p.height <= 0 {
		return nil, pil.Next("PNG: not identified by this driver")
	}
	if string(last.cid) == "fdAT" {
		p.prepareIDAT = last.length - 4
	} else {
		p.prepareIDAT = last.length
	}
	nFrames := int64(1)
	if p.hasNFrames {
		nFrames = p.nFrames
		if p.info["default_image"].truthy() {
			nFrames++
		}
		if err := p.seekFirstFrame(); err != nil {
			return nil, err
		}
	}
	p.animated = nFrames > 1
	return p, nil
}

// seekFirstFrame is the part of PngImageFile._seek(0) that can raise:
// preparing OP_BACKGROUND disposal allocates a frame and crops it.
func (p *pngFile) seekFirstFrame() error {
	disposal, ok := p.info["disposal"]
	if !ok || disposal.kind != pyInt || (disposal.i != 1 && disposal.i != 2) {
		return nil
	}
	if err := coreNewCheck(p.mode, p.width, p.height); err != nil {
		return err
	}
	bbox, ok := p.info["bbox"]
	if !ok || !bbox.truthy() {
		return raise(excAttribute, "dispose_extent")
	}
	if bbox.kind != pyTuple {
		return pil.Next("PNG: round() on a text bbox")
	}
	w := bbox.tuple[2] - bbox.tuple[0]
	h := bbox.tuple[3] - bbox.tuple[1]
	if w < 0 {
		w = -w
	}
	if h < 0 {
		h = -h
	}
	if w > 1<<40 || h > 1<<40 {
		return &pil.BombError{Pixels: w}
	}
	return pil.CheckBomb(int(w), int(h))
}

// coreNewCheck is the failure set of Image.core.new / core.fill for a mode
// and size: an unknown mode, a size that does not fit a C int, or a line
// size overflow.
func coreNewCheck(mode string, w, h int64) error {
	if pil.BytesPerPixel(mode) == 0 {
		return raise(excValue, "unrecognized image mode %q", mode)
	}
	if w > intMax || h > intMax {
		return raise(excOverflow, "signed integer is greater than maximum")
	}
	if w > intMax/4-1 {
		return raise(excMemory, "line size overflow")
	}
	return nil
}

// Size is im.size after open.
func (p *pngFile) Size() (int, int) { return int(p.width), int(p.height) }

var errNoTile = errors.New("cannot load this image")
