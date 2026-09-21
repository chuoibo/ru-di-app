package jpegdec

import (
	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// decode is what ImageFile.load does for a JPEG tile: JpegDecode.c driving
// jpeg_read_header, jpeg_start_decompress, jpeg_read_scanlines and
// jpeg_finish_decompress over the file fed in 64 KiB blocks. mode is the
// mode JpegImagePlugin chose from the SOF layer count; width and height
// are the size it read.
func decode(data []byte, mode string, width, height int) (pix []byte, err error) {
	d := &decoder{src: newSource(data)}
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case *jpegError:
				pix, err = nil, v
			case *unsupportedSignal:
				pix, err = nil, pil.Unsupported("JPEG", "%s", v.reason)
			default:
				panic(r)
			}
		}
	}()
	d.readHeader()
	// Pillow's rawmode picks the output colour space: L, RGBX (JCS
	// extensions), or CMYK for "CMYK;I".
	switch mode {
	case "L":
		d.outColorSpace = csGrayscale
	case "RGB":
		d.outColorSpace = csExtRGBX
	case "CMYK":
		d.outColorSpace = csCMYK
	default:
		d.jpegColorSpace = csUnknown
		d.outColorSpace = csUnknown
	}
	ups := d.startDecompress()
	if d.imageWidth != width || d.imageHeight != height {
		// Pillow would write rows of its own size from libjpeg's buffer;
		// the two sizes come from the same SOF whenever libjpeg accepts it.
		errexit("size mismatch between the Python and libjpeg headers")
	}
	d.allocPlanes()
	if d.hasMultipleScans {
		d.outputMultiScan()
	} else {
		d.decodeSingleScan()
	}
	pix = d.convertOutput(mode, ups)
	d.finishDecompress()
	return pix, nil
}

// readHeader is Pillow's jpeg_read_header loop: a tables-only datastream
// (EOI before any SOS) is dropped with jpeg_abort, keeping its tables.
func (d *decoder) readHeader() {
	for {
		d.resetInputController()
		if d.consumeMarkers() == reachedSOS {
			d.defaultDecompressParms()
			return
		}
	}
}

// startDecompress is jpeg_start_decompress: master_selection, then for a
// multi-scan file the absorption of every scan up to EOI.
func (d *decoder) startDecompress() []upsampler {
	d.checkColorDeconverter()
	ups := d.selectUpsamplers()
	if d.lossless {
		panic(&unsupportedSignal{reason: "lossless JPEG (SOF3/SOF11), which libjpeg-turbo decodes"})
	}
	if d.dataPrecision != 8 {
		panic(&unsupportedSignal{reason: "12-bit JPEG"})
	}
	if d.progressive {
		d.coefBits = make([][64]int, 2*d.numComponents)
		for ci := 0; ci < d.numComponents; ci++ {
			for i := range d.coefBits[ci] {
				d.coefBits[ci][i] = -1
			}
		}
	} else if !d.arith {
		d.stdHuffTables()
	}
	if d.hasMultipleScans {
		d.allocCoefArrays()
	}
	d.startInputPass()
	d.lastGoodIMCURow = 0
	if d.hasMultipleScans {
		for {
			var r int
			if d.scanActive {
				r = d.consumeScanData()
			} else {
				r = d.consumeMarkers()
			}
			if r == reachedEOI {
				break
			}
		}
	}
	d.outputScanNumber = d.inputScanNumber
	// jinit_inverse_dct's islow multiplier tables, filled at the output
	// pass for every component whose quantization table was latched.
	for ci := range d.comps {
		c := &d.comps[ci]
		if c.quant == nil {
			continue
		}
		for i := range c.dctTable {
			c.dctTable[i] = int16(c.quant[i])
		}
	}
	return ups
}

// finishDecompress is jpeg_finish_decompress: it reads markers up to EOI
// from what Pillow has handed over; running dry ends the decode.
func (d *decoder) finishDecompress() {
	d.src.finishing = true
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(suspendSignal); ok {
				return
			}
			panic(r)
		}
	}()
	for !d.eoiReached {
		d.consumeMarkers()
	}
}
