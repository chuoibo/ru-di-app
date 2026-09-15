// Package pil models the part of a Pillow 12.2.0 image that the upload
// sanitizer reads after decoding: mode, size, samples, palette and the info
// keys that ImageOps.exif_transpose and the alpha check look at.
//
// Decoders in sibling packages build an *Image the way the matching Pillow
// plugin would leave it after load(); the sanitizer then transposes,
// converts and re-encodes it.
package pil

import (
	"errors"
	"fmt"
)

// Image is a decoded Pillow image.
//
// Pix is row-major, BytesPerPixel(Mode) bytes per pixel:
//
//	"1", "L", "P"                       1 byte ("1" stores 0 or 255, as Pillow does)
//	"LA", "La", "PA"                    2 bytes: value, alpha
//	"RGB", "YCbCr", "LAB", "HSV"        3 bytes
//	"RGBA", "RGBa", "RGBX", "CMYK"      4 bytes
//	"I;16", "I;16L", "I;16B", "I;16N"   2 bytes: the sample value, little-endian
//	"I"                                 4 bytes: int32, little-endian
//	"F"                                 4 bytes: float32 IEEE bits, little-endian
type Image struct {
	Mode          string
	Width, Height int
	Pix           []byte

	// PaletteMode ("RGB" or "RGBA") and Palette describe the palette of a
	// "P" or "PA" image as the plugin set it (ImagePalette mode and bytes,
	// len(PaletteMode) bytes per entry, no padding added).
	PaletteMode string
	Palette     []byte

	Info Info

	// Format is Pillow's format name ("JPEG", "PNG", ...), for diagnostics.
	Format string
}

// Info mirrors the Pillow info keys the sanitizer reads.
type Info struct {
	// Transparency is info["transparency"], nil when the key is absent.
	Transparency *Transparency

	HasExif bool
	Exif    []byte // info["exif"] (bytes)

	HasRawProfileExif bool
	RawProfileExif    string // info["Raw profile type exif"] (str)

	HasXMP bool
	XMP    []byte // info["xmp"] (bytes)

	HasAdobeXMP bool
	AdobeXMP    string // info["XML:com.adobe.xmp"] (str)

	// TIFF is set for images whose getexif reads the file itself through
	// tag_v2 (TiffImageFile) instead of an info key.
	TIFF *TIFFSource
}

// TIFFSource locates IFD0 of a TIFF file for Image.getexif.
type TIFFSource struct {
	File      []byte
	Offset    int64
	BigEndian bool
	BigTIFF   bool
}

// TransparencyKind is the Python type of info["transparency"].
type TransparencyKind int

const (
	// TransparencyInt is an int (a palette index or a gray level).
	TransparencyInt TransparencyKind = iota + 1
	// TransparencyBytes is bytes (per-palette-entry alpha).
	TransparencyBytes
	// TransparencyRGB is an (r, g, b) tuple.
	TransparencyRGB
)

// Transparency is the value of info["transparency"].
type Transparency struct {
	Kind  TransparencyKind
	Int   int
	Bytes []byte
	RGB   [3]int
}

// BytesPerPixel is the Pix stride of a mode, 0 for a mode this model lacks.
func BytesPerPixel(mode string) int {
	switch mode {
	case "1", "L", "P":
		return 1
	case "LA", "La", "PA", "I;16", "I;16L", "I;16B", "I;16N":
		return 2
	case "RGB", "YCbCr", "LAB", "HSV":
		return 3
	case "RGBA", "RGBa", "RGBX", "CMYK", "I", "F":
		return 4
	}
	return 0
}

// Bands is Image.getbands() for a mode.
func Bands(mode string) []string {
	switch mode {
	case "1":
		return []string{"1"}
	case "L":
		return []string{"L"}
	case "P":
		return []string{"P"}
	case "LA":
		return []string{"L", "A"}
	case "La":
		return []string{"L", "a"}
	case "PA":
		return []string{"P", "A"}
	case "RGB":
		return []string{"R", "G", "B"}
	case "RGBA":
		return []string{"R", "G", "B", "A"}
	case "RGBa":
		return []string{"R", "G", "B", "a"}
	case "RGBX":
		return []string{"R", "G", "B", "X"}
	case "CMYK":
		return []string{"C", "M", "Y", "K"}
	case "YCbCr":
		return []string{"Y", "Cb", "Cr"}
	case "LAB":
		return []string{"L", "A", "B"}
	case "HSV":
		return []string{"H", "S", "V"}
	case "I", "I;16", "I;16L", "I;16B", "I;16N":
		return []string{"I"}
	case "F":
		return []string{"F"}
	}
	return nil
}

// HasAlphaBand reports "A" in getbands(), the first half of the
// sanitizer's alpha check.
func HasAlphaBand(mode string) bool {
	for _, band := range Bands(mode) {
		if band == "A" {
			return true
		}
	}
	return false
}

// MaxImagePixels is PIL.Image.MAX_IMAGE_PIXELS in 12.2.0:
// int(1024 * 1024 * 1024 // 4 // 3).
const MaxImagePixels = 1024 * 1024 * 1024 / 4 / 3

// NextError is an exception Image.open's _open_core catches (SyntaxError,
// IndexError, TypeError, struct.error) before trying the next plugin.
// ImageFile.__init__ also turns KeyError and EOFError from _open into
// SyntaxError, and raises SyntaxError for an empty mode or a size <= 0.
type NextError struct{ Msg string }

func (e *NextError) Error() string { return "next plugin: " + e.Msg }

// Next returns a *NextError.
func Next(format string, args ...any) error {
	return &NextError{Msg: fmt.Sprintf(format, args...)}
}

// IsNext reports whether err lets Image.open try the next plugin.
func IsNext(err error) bool {
	var next *NextError
	return errors.As(err, &next)
}

// UnsupportedError marks an input Pillow 12.2.0 would decode that this port
// does not reproduce. It is a reported divergence, never a guess at pixels.
type UnsupportedError struct {
	Format string
	Reason string
}

func (e *UnsupportedError) Error() string {
	return "unsupported " + e.Format + ": " + e.Reason
}

// Unsupported returns an *UnsupportedError.
func Unsupported(format, reason string, args ...any) error {
	return &UnsupportedError{Format: format, Reason: fmt.Sprintf(reason, args...)}
}

// BombError is DecompressionBombError, or a DecompressionBombWarning that
// the sanitizer's warning filter raised as an exception.
type BombError struct {
	Pixels  int64
	Warning bool
}

func (e *BombError) Error() string {
	if e.Warning {
		return fmt.Sprintf("DecompressionBombWarning: %d pixels", e.Pixels)
	}
	return fmt.Sprintf("DecompressionBombError: %d pixels", e.Pixels)
}

// CheckBomb is Image._decompression_bomb_check with the sanitizer's
// warnings.simplefilter("error", DecompressionBombWarning).
func CheckBomb(width, height int) error {
	w, h := int64(width), int64(height)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	pixels := w * h
	if pixels > 2*MaxImagePixels {
		return &BombError{Pixels: pixels}
	}
	if pixels > MaxImagePixels {
		return &BombError{Pixels: pixels, Warning: true}
	}
	return nil
}
