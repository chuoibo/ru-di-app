// Package jpegenc is a pure Go port of the libjpeg-turbo 3.1.4.1 baseline
// compressor as Pillow 12.2.0 drives it (libImaging/JpegEncode.c,
// PIL/JpegImagePlugin.py).
//
// EncodeRGB reproduces, byte for byte, what the upload sanitizer's
// Image.save(format="JPEG", quality=88, optimize=True) writes, including the
// cases where Pillow raises instead. Encode exposes the same compressor with
// other settings so tests can build JPEG inputs; only the sanitizer
// configuration is held to Pillow's bytes.
//
// The port follows the C data flow: the prep controller's color and
// bottom-edge padding (jcprepct.c), the downsamplers (jcsample.c), the full
// coefficient buffer with its dummy blocks (jccoefct.c compress_first_pass),
// the islow forward DCT (jfdctint.c), reciprocal quantization (jcdctmgr.c),
// Huffman statistics and optimal tables (jchuff.c) and the marker writer
// (jcmarker.c).
package jpegenc

import (
	"errors"
	"fmt"
)

const (
	dctSize        = 8
	maxDimension   = 65500 // JPEG_MAX_DIMENSION
	maxSampFactor  = 4     // MAX_SAMP_FACTOR
	maxBlocksInMCU = 10    // C_MAX_BLOCKS_IN_MCU
	maxCompsInScan = 4     // MAX_COMPS_IN_SCAN
)

// ColorSpace selects the input layout and the JPEG color space.
type ColorSpace int

const (
	// Gray takes 1 byte per pixel and writes a JFIF grayscale file.
	Gray ColorSpace = iota + 1
	// YCbCr takes RGB, 3 bytes per pixel, and writes JFIF YCbCr.
	YCbCr
	// CMYK takes 4 bytes per pixel and writes Adobe APP14, transform 0.
	CMYK
)

// Sampling is one component's horizontal and vertical sampling factor.
type Sampling struct{ H, V int }

// Segment is a marker segment written right after SOI and the JFIF or
// Adobe marker, like Pillow's jpeg_write_marker calls: Marker is the second
// marker byte (0xE0..0xEF for APPn, 0xFE for COM).
type Segment struct {
	Marker byte
	Data   []byte
}

// Options configures Encode.
type Options struct {
	Width, Height int
	Color         ColorSpace
	// Sampling overrides jpeg_set_colorspace's factors, one per component.
	Sampling []Sampling
	// Quality is jpeg_set_quality's quality, with force_baseline.
	Quality int
	// Optimize is optimize_coding: two passes and optimal Huffman tables.
	Optimize bool
	// RestartInterval is restart_interval in MCUs (0 for none).
	RestartInterval int
	// InvertCMYK stores 255-x for every CMYK sample, as Photoshop does.
	InvertCMYK bool
	Segments   []Segment
	// BufferSize is Pillow's encoder buffer (ImageFile._save's bufsize).
	// With Optimize every byte is written inside jpeg_finish_compress,
	// where a full buffer cannot suspend: an output of BufferSize bytes or
	// more fails. Zero means unlimited.
	BufferSize int
}

// EncodeError is a failure Pillow's JPEG save raises as a Python exception.
type EncodeError struct {
	// Code names the libjpeg error (JERR_...) or Pillow's own check.
	Code string
	// PyType and PyMessage are the exception Image.save raises.
	PyType    string
	PyMessage string
}

func (e *EncodeError) Error() string {
	return fmt.Sprintf("jpegenc: %s (Python %s: %s)", e.Code, e.PyType, e.PyMessage)
}

// libjpegMessage is what ImageFile._get_oserror builds for
// IMAGING_CODEC_BROKEN, the status JpegEncode.c's error handler sets.
const libjpegMessage = "broken data stream when writing image file"

func libjpegError(code string) error {
	return &EncodeError{Code: code, PyType: "OSError", PyMessage: libjpegMessage}
}

// IsEncodeError reports whether err is an *EncodeError.
func IsEncodeError(err error) bool {
	var encodeErr *EncodeError
	return errors.As(err, &encodeErr)
}

// EncodeRGB is the sanitizer's call: Image.frombytes("RGB", (width, height),
// pix).save(buf, "JPEG", quality=88, optimize=True).
func EncodeRGB(width, height int, pix []byte) ([]byte, error) {
	return Encode(SanitizerOptions(width, height), pix)
}

// SanitizerOptions is the configuration EncodeRGB uses.
func SanitizerOptions(width, height int) Options {
	return Options{
		Width:      width,
		Height:     height,
		Color:      YCbCr,
		Sampling:   []Sampling{{sanitizeLumaH, sanitizeLumaV}, {1, 1}, {1, 1}},
		Quality:    sanitizeQuality,
		Optimize:   sanitizeOptimize,
		BufferSize: PillowBufferSize(width, height, sanitizeQuality, sanitizeOptimize),
	}
}

// PillowBufferSize is the bufsize ImageFile._save hands the JPEG encoder
// for an image without exif, extra markers or progressive mode:
// JpegImagePlugin._save picks width*height for optimize with quality below
// 95 (twice that at 95 and above), 5 otherwise, and ImageFile._save raises
// it to max(MAXBLOCK, bufsize, width*4).
func PillowBufferSize(width, height, quality int, optimize bool) int {
	bufsize := 5
	if optimize {
		if quality >= 95 || quality == -1 {
			bufsize = 2 * width * height
		} else {
			bufsize = width * height
		}
	}
	return max(65536, bufsize, width*4)
}

type downsampleMethod int

const (
	fullsizeDownsample downsampleMethod = iota
	h2v1Downsample
	h2v2Downsample
	intDownsample
)

type component struct {
	id, h, v, tbl  int
	widthInBlocks  int
	heightInBlocks int
	method         downsampleMethod

	// stride is the main buffer width, widthInBlocks*8.
	stride int
	// colorStride is the prep controller's color buffer width.
	colorStride int

	// coef is the whole-image coefficient buffer, blocksWide by blocksHigh
	// blocks of 64 coefficients.
	coef       []int16
	blocksWide int
	blocksHigh int
}

type encoder struct {
	opts          Options
	comps         []component
	maxH, maxV    int
	totalIMCURows int
	quant         [2][64]uint16
	div           [2]divisors
	mcusPerRow    int
	mcuRows       int
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

func roundUp(a, b int) int { return ceilDiv(a, b) * b }

func newEncoder(opts Options) (*encoder, error) {
	if opts.Width <= 0 || opts.Height <= 0 {
		// encode.c _setimage refuses before libjpeg is set up.
		return nil, &EncodeError{Code: "empty image", PyType: "ValueError", PyMessage: "cannot write empty image"}
	}
	e := &encoder{opts: opts}
	switch opts.Color {
	case Gray:
		e.comps = []component{{id: 1, h: 1, v: 1, tbl: 0}}
	case YCbCr:
		e.comps = []component{{id: 1, h: 2, v: 2, tbl: 0}, {id: 2, h: 1, v: 1, tbl: 1}, {id: 3, h: 1, v: 1, tbl: 1}}
	case CMYK:
		e.comps = []component{{id: 'C', h: 1, v: 1}, {id: 'M', h: 1, v: 1}, {id: 'Y', h: 1, v: 1}, {id: 'K', h: 1, v: 1}}
	default:
		return nil, fmt.Errorf("jpegenc: unknown color space %d", opts.Color)
	}
	if opts.Sampling != nil {
		if len(opts.Sampling) != len(e.comps) {
			return nil, fmt.Errorf("jpegenc: %d sampling factors for %d components", len(opts.Sampling), len(e.comps))
		}
		for ci, s := range opts.Sampling {
			e.comps[ci].h, e.comps[ci].v = s.H, s.V
		}
	}
	scale := qualityScaling(opts.Quality)
	e.quant[0] = scaleQuantTable(&stdLuminanceQuant, scale)
	e.quant[1] = scaleQuantTable(&stdChrominanceQuant, scale)

	// jcmaster.c initial_setup.
	if opts.Height > maxDimension || opts.Width > maxDimension {
		return nil, libjpegError("JERR_IMAGE_TOO_BIG")
	}
	e.maxH, e.maxV = 1, 1
	for _, c := range e.comps {
		if c.h <= 0 || c.h > maxSampFactor || c.v <= 0 || c.v > maxSampFactor {
			return nil, libjpegError("JERR_BAD_SAMPLING")
		}
		e.maxH = max(e.maxH, c.h)
		e.maxV = max(e.maxV, c.v)
	}
	for ci := range e.comps {
		c := &e.comps[ci]
		c.widthInBlocks = ceilDiv(opts.Width*c.h, e.maxH*dctSize)
		c.heightInBlocks = ceilDiv(opts.Height*c.v, e.maxV*dctSize)
		c.stride = c.widthInBlocks * dctSize
		c.colorStride = c.widthInBlocks * dctSize * e.maxH / c.h
		c.blocksWide = roundUp(c.widthInBlocks, c.h)
		c.blocksHigh = roundUp(c.heightInBlocks, c.v)
	}
	e.totalIMCURows = ceilDiv(opts.Height, e.maxV*dctSize)

	// jcsample.c jinit_downsampler.
	for ci := range e.comps {
		c := &e.comps[ci]
		switch {
		case c.h == e.maxH && c.v == e.maxV:
			c.method = fullsizeDownsample
		case c.h*2 == e.maxH && c.v == e.maxV:
			c.method = h2v1Downsample
		case c.h*2 == e.maxH && c.v*2 == e.maxV:
			c.method = h2v2Downsample
		case e.maxH%c.h == 0 && e.maxV%c.v == 0:
			c.method = intDownsample
		default:
			return nil, libjpegError("JERR_FRACT_SAMPLE_NOTIMPL")
		}
	}

	// jcmaster.c per_scan_setup: one scan holding every component.
	if len(e.comps) == 1 {
		c := &e.comps[0]
		e.mcusPerRow = c.widthInBlocks
		e.mcuRows = c.heightInBlocks
	} else {
		if len(e.comps) > maxCompsInScan {
			return nil, libjpegError("JERR_COMPONENT_COUNT")
		}
		e.mcusPerRow = ceilDiv(opts.Width, e.maxH*dctSize)
		e.mcuRows = ceilDiv(opts.Height, e.maxV*dctSize)
		blocks := 0
		for _, c := range e.comps {
			blocks += c.h * c.v
		}
		if blocks > maxBlocksInMCU {
			return nil, libjpegError("JERR_BAD_MCU_SIZE")
		}
	}
	for tbl := range e.quant {
		e.div[tbl] = newDivisors(&e.quant[tbl])
	}
	return e, nil
}

// Encode compresses pix (row-major, 1, 3 or 4 bytes per pixel for Gray,
// YCbCr and CMYK) with opts.
func Encode(opts Options, pix []byte) ([]byte, error) {
	e, err := newEncoder(opts)
	if err != nil {
		return nil, err
	}
	if want := opts.Width * opts.Height * len(e.comps); len(pix) != want {
		return nil, fmt.Errorf("jpegenc: %d pixel bytes, want %d", len(pix), want)
	}
	for _, segment := range opts.Segments {
		if len(segment.Data) > 65533 {
			return nil, libjpegError("JERR_BAD_LENGTH")
		}
	}

	e.firstPass(pix)

	dcTables, acTables := [2]*huffTable{}, [2]*huffTable{}
	dcTables[0] = newHuffTable(bitsDCLuminance, valDCLuminance)
	acTables[0] = newHuffTable(bitsACLuminance, valACLuminance)
	dcTables[1] = newHuffTable(bitsDCChrominance, valDCChrominance)
	acTables[1] = newHuffTable(bitsACChrominance, valACChrominance)
	if opts.Optimize {
		if err := e.optimizeTables(&dcTables, &acTables); err != nil {
			return nil, err
		}
	}
	var dcDerived, acDerived [2]*derivedTable
	for _, c := range e.comps {
		if dcDerived[c.tbl] == nil {
			if dcDerived[c.tbl], err = makeDerived(dcTables[c.tbl], true); err != nil {
				return nil, err
			}
			if acDerived[c.tbl], err = makeDerived(acTables[c.tbl], false); err != nil {
				return nil, err
			}
		}
	}

	out := make([]byte, 0, 1024+opts.Width*opts.Height/2)
	out = e.writeFileHeader(out)
	for _, segment := range opts.Segments {
		out = append(out, 0xFF, segment.Marker, byte((len(segment.Data)+2)>>8), byte(len(segment.Data)+2))
		out = append(out, segment.Data...)
	}
	out = e.writeFrameHeader(out)
	out = e.writeScanHeader(out, &dcTables, &acTables)
	out = e.encodeScan(out, &dcDerived, &acDerived)
	out = append(out, 0xFF, 0xD9)
	if opts.Optimize && opts.BufferSize > 0 && len(out) >= opts.BufferSize {
		return nil, libjpegError("JERR_CANT_SUSPEND")
	}
	return out, nil
}
