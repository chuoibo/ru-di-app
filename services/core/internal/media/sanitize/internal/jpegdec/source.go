package jpegdec

import "fmt"

// pillowBlock is ImageFile.MAXBLOCK: ImageFile.load hands the JPEG decoder
// the file in reads of this size.
const pillowBlock = 65536

// jpegError is a libjpeg ERREXIT, or Pillow's OSError for a file that ran
// out before the decoder was done. Either way Pillow's load raises.
type jpegError struct {
	code string
}

func (e *jpegError) Error() string { return "jpeg: " + e.code }

// errexit aborts decoding the way libjpeg's error_exit longjmps out.
func errexit(code string, args ...any) {
	panic(&jpegError{code: fmt.Sprintf(code, args...)})
}

// suspendSignal unwinds a marker read that found no more input while
// jpeg_finish_decompress was reading up to EOI; Pillow treats that
// suspension as a finished decode.
type suspendSignal struct{}

// source is Pillow's suspending jpeg source manager driven by
// ImageFile.load: the decoder sees the file in growing prefixes of
// pillowBlock bytes. A read past what Pillow has handed over makes Pillow
// read the next block; a read past the end of the file either suspends
// the finishing decoder or makes load raise "image file is truncated".
type source struct {
	data  []byte
	pos   int
	limit int
	// finishing is set inside jpeg_finish_decompress.
	finishing bool
	// strict is set while an arithmetic decoder, which cannot suspend,
	// reads a restart marker.
	strict bool
}

func newSource(data []byte) *source {
	return &source{data: data, limit: min(pillowBlock, len(data))}
}

// more hands the decoder Pillow's next block. It reports false when the
// file has nothing left.
func (s *source) more() bool {
	if s.limit >= len(s.data) {
		return false
	}
	s.limit = min(s.limit+pillowBlock, len(s.data))
	return true
}

// available is bytes_in_buffer for a reader positioned at pos.
func (s *source) available(pos int) int {
	if pos >= s.limit {
		return 0
	}
	return s.limit - pos
}

// ensure makes the byte at pos available, growing the handed-over prefix
// like repeated Pillow reads; when the file is exhausted it suspends a
// finishing decoder and fails any other.
func (s *source) ensure(pos int) {
	if pos < s.limit {
		return
	}
	if s.strict {
		errexit("JERR_CANT_SUSPEND")
	}
	if s.finishing {
		// jpeg_finish_decompress runs inside the decode call that read
		// the last row: running dry returns to Pillow, which is done.
		panic(suspendSignal{})
	}
	for pos >= s.limit {
		if !s.more() {
			errexit("image file is truncated")
		}
	}
}

// inputByte is INPUT_BYTE for the marker reader.
func (s *source) inputByte() int {
	s.ensure(s.pos)
	c := s.data[s.pos]
	s.pos++
	return int(c)
}

// input2Bytes is INPUT_2BYTES.
func (s *source) input2Bytes() int {
	hi := s.inputByte()
	return hi<<8 | s.inputByte()
}

// skip is Pillow's skip_input_data: skipping past the buffer carries the
// remainder into the next reads, so the position may run past limit.
func (s *source) skip(n int) {
	if n > 0 {
		s.pos += n
	}
}
