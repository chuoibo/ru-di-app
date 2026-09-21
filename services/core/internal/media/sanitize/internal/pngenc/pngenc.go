// Package pngenc writes RGBA images exactly as Pillow 12.2.0 does for
// Image.frombytes("RGBA", size, pix).save(buf, "PNG") in the pinned parity
// image. Pillow there calls the system zlib 1.3.1 at run time (see the
// zlib131 package), not the zlib-ng its build headers name.
package pngenc

import (
	"encoding/binary"
	"hash/crc32"

	"mobile/services/core/internal/media/sanitize/internal/pngenc/zlib131"
)

var signature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// EncodeRGBA returns the PNG Pillow writes for width x height RGBA pixels
// (4 bytes per pixel, row-major). It panics if pix is too short.
func EncodeRGBA(width, height int, pix []byte) []byte {
	if width <= 0 || height <= 0 || len(pix) < 4*width*height {
		panic("pngenc: bad image dimensions or pixel buffer")
	}
	out := make([]byte, 0, len(signature)+64+len(pix)/2)
	out = append(out, signature...)

	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(height))
	ihdr[8] = 8  // bit depth
	ihdr[9] = 6  // colour type RGBA
	ihdr[10] = 0 // compression
	ihdr[11] = 0 // filter category
	ihdr[12] = 0 // interlace
	out = putChunk(out, "IHDR", ihdr[:])

	size := bufsize(width)
	enc := newZipEncoder(width, height, pix)
	buf := make([]byte, size)
	for {
		n, done := enc.encode(buf)
		// ImageFile._encode_tile writes every encode() result, then stops
		// once the encoder reports an error code (here: the end).
		out = putChunk(out, "IDAT", buf[:n])
		if done {
			break
		}
	}
	return putChunk(out, "IEND", nil)
}

// putChunk is PngImagePlugin.putchunk.
func putChunk(out []byte, cid string, data []byte) []byte {
	var head [8]byte
	binary.BigEndian.PutUint32(head[0:], uint32(len(data)))
	copy(head[4:], cid)
	out = append(out, head[:]...)
	out = append(out, data...)
	crc := crc32.Update(crc32.ChecksumIEEE([]byte(cid)), crc32.IEEETable, data)
	return binary.BigEndian.AppendUint32(out, crc)
}

// zipEncoder is ImagingZipEncode (ZipEncode.c) in ZIP_PNG mode for an
// RGBA image with the packer RGBA -> RGBA (32 bits per pixel).
type zipEncoder struct {
	pix           []byte
	width, height int
	bytes         int // state->bytes = (bits * xsize + 7) / 8
	bpp           int
	y             int
	state         int

	buffer, previous, prior, up, average, paeth []byte

	strm zlib131.Stream
}

func newZipEncoder(width, height int, pix []byte) *zipEncoder {
	const bits = 32
	return &zipEncoder{
		pix:    pix,
		width:  width,
		height: height,
		bytes:  (bits*width + 7) / 8,
		bpp:    (bits + 7) / 8,
	}
}

func (e *zipEncoder) init() {
	n := e.bytes + 1
	e.buffer = make([]byte, n)
	e.previous = make([]byte, n)
	e.prior = make([]byte, n)
	e.up = make([]byte, n)
	e.average = make([]byte, n)
	e.paeth = make([]byte, n)
	e.buffer[0] = 0
	e.prior[0] = 1
	e.up[0] = 2
	e.average[0] = 3
	e.paeth[0] = 4

	level := compressLevel
	if optimize {
		level = 9
	}
	if err := e.strm.DeflateInit2(level, windowBits, memLevel, strategy); err != zlib131.OK {
		panic("pngenc: deflateInit2 refused the Pillow parameters")
	}
	e.state = 1
}

// encode is one ImagingZipEncode call with a buffer of len(buf) bytes. It
// returns how many bytes it wrote and whether the stream is complete
// (IMAGING_CODEC_END).
func (e *zipEncoder) encode(buf []byte) (int, bool) {
	if e.state == 0 {
		e.init()
	}
	e.strm.NextOut = buf
	written := func() int { return len(buf) - len(e.strm.NextOut) }

	if len(e.strm.NextIn) > 0 {
		if err := e.strm.Deflate(zlib131.NoFlush); err < 0 {
			panic("pngenc: deflate failed on leftover input")
		}
	}

	if e.state == 1 {
		for len(e.strm.NextOut) > 0 {
			if e.y >= e.height {
				e.state = 2
				break
			}
			row := e.pix[e.y*e.width*4 : (e.y+1)*e.width*4]
			copy(e.buffer[1:], row)
			e.y++

			output := e.filter()
			e.strm.NextIn = output[:e.bytes+1]
			if err := e.strm.Deflate(zlib131.NoFlush); err < 0 {
				panic("pngenc: deflate failed")
			}
			e.buffer, e.previous = e.previous, e.buffer
		}
		if len(e.strm.NextOut) == 0 {
			return written(), false
		}
	}

	// state 2: end of image data, flush the compressor
	for len(e.strm.NextOut) > 0 {
		err := e.strm.Deflate(zlib131.Finish)
		if err == zlib131.StreamEnd {
			return written(), true
		}
		if err < 0 {
			panic("pngenc: deflate failed at finish")
		}
	}
	return written(), false
}

func distance(v byte) int32 {
	if v < 128 {
		return int32(v)
	}
	return 256 - int32(v)
}

// filter picks the line filter with the least total distance from zero,
// as ZipEncode.c does. Sums are C ints.
func (e *zipEncoder) filter() []byte {
	buffer, previous := e.buffer, e.previous
	n := e.bytes
	bpp := e.bpp
	output := buffer

	var sum int32
	for i := 1; i <= n; i++ {
		sum += distance(buffer[i])
	}

	if filterUp && sum > 0 {
		up := e.up
		var s int32
		for i := 1; i <= n; i++ {
			v := buffer[i] - previous[i]
			up[i] = v
			s += distance(v)
		}
		if s < sum {
			output = up
			sum = s
		}
	}

	if filterPrior && sum > 0 {
		prior := e.prior
		var s int32
		i := 1
		for ; i <= bpp && i <= n; i++ {
			v := buffer[i]
			prior[i] = v
			s += distance(v)
		}
		for ; i <= n; i++ {
			v := buffer[i] - buffer[i-bpp]
			prior[i] = v
			s += distance(v)
		}
		if s < sum {
			output = prior
			sum = s
		}
	}

	if filterAverage && sum > 0 {
		average := e.average
		var s int32
		i := 1
		for ; i <= bpp && i <= n; i++ {
			v := buffer[i] - previous[i]/2
			average[i] = v
			s += distance(v)
		}
		for ; i <= n; i++ {
			v := buffer[i] - byte((int(buffer[i-bpp])+int(previous[i]))/2)
			average[i] = v
			s += distance(v)
		}
		if s < sum {
			output = average
			sum = s
		}
	}

	if filterPaeth && sum > 0 {
		paeth := e.paeth
		var s int32
		i := 1
		for ; i <= bpp && i <= n; i++ {
			v := buffer[i] - previous[i]
			paeth[i] = v
			s += distance(v)
		}
		for ; i <= n; i++ {
			a := int(buffer[i-bpp])
			b := int(previous[i])
			c := int(previous[i-bpp])
			pa := abs(b - c)
			pb := abs(a - c)
			pc := abs(a + b - 2*c)
			var pred int
			switch {
			case pa <= pb && pa <= pc:
				pred = a
			case pb <= pc:
				pred = b
			default:
				pred = c
			}
			v := buffer[i] - byte(pred)
			paeth[i] = v
			s += distance(v)
		}
		if s < sum {
			output = paeth
		}
	}
	return output
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
