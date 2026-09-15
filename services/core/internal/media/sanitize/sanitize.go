// Package sanitize re-encodes uploaded images so embedded phone metadata
// never reaches storage. It is the Go port of services/api/app/media/images.py
// (sanitize_image) and reproduces Pillow 12.2.0 as pinned in
// mobile-parity-api:7bf58e3d: the same refusals in the same order, the same
// pixels, and the same output bytes for the paths the port covers.
//
// Paths the port does not reproduce return a *pil.UnsupportedError (see
// UnsupportedError); they are reported divergences, never approximations.
package sanitize

import (
	"errors"
	"fmt"

	"mobile/services/core/internal/media/sanitize/internal/convert"
	"mobile/services/core/internal/media/sanitize/internal/exif"
	"mobile/services/core/internal/media/sanitize/internal/imageopen"
	"mobile/services/core/internal/media/sanitize/internal/jpegenc"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/pngenc"
)

// Limits of app/media/images.py.
const (
	MaxUploadBytes = 10 * 1024 * 1024
	MaxPixels      = 50_000_000
)

// Refusal codes of app/media/images.py.
const (
	CodeImageTooLarge           = "image_too_large"
	CodeImageDimensionsTooLarge = "image_dimensions_too_large"
	CodeNotAnImage              = "not_an_image"
)

var (
	detailTooLarge   = fmt.Sprintf("Image exceeds the %d-byte upload limit.", MaxUploadBytes)
	detailDimensions = fmt.Sprintf("Image exceeds the %d-pixel limit.", MaxPixels)
	detailNotAnImage = "The uploaded bytes could not be decoded as a complete image."
)

// Sanitized is SanitizedImage: the re-encoded bytes and what the upload
// response reports about them.
type Sanitized struct {
	Data        []byte
	ContentType string
	Width       int
	Height      int
}

// Rejected is ImageRejected: a refusal with Python's code and detail.
type Rejected struct {
	Code   string
	Detail string
}

func (r *Rejected) Error() string { return r.Code + ": " + r.Detail }

// EncodeError is an exception Pillow's save raised. sanitize_image does not
// catch it, so Python answers it as an unhandled error, not as a refusal.
type EncodeError struct{ Err error }

func (e *EncodeError) Error() string { return "encoding sanitized image: " + e.Err.Error() }

func (e *EncodeError) Unwrap() error { return e.Err }

// UnsupportedError marks input Pillow decodes that this port does not.
type UnsupportedError = pil.UnsupportedError

// InternalError is a panic inside the port. Python has no counterpart; it
// is surfaced instead of being folded into not_an_image so a parity run
// counts it.
type InternalError struct{ Panic any }

func (e *InternalError) Error() string { return fmt.Sprintf("sanitize panicked: %v", e.Panic) }

func refuse(code, detail string) error { return &Rejected{Code: code, Detail: detail} }

// Sanitize is sanitize_image. The error is a *Rejected, an *EncodeError, an
// *UnsupportedError or an *InternalError.
func Sanitize(raw []byte) (result Sanitized, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result, err = Sanitized{}, &InternalError{Panic: recovered}
		}
	}()

	if len(raw) > MaxUploadBytes {
		return Sanitized{}, refuse(CodeImageTooLarge, detailTooLarge)
	}
	mode, width, height, pix, err := decode(raw)
	if err != nil {
		return Sanitized{}, err
	}
	if mode == "RGBA" {
		return Sanitized{
			Data:        pngenc.EncodeRGBA(width, height, pix),
			ContentType: "image/png",
			Width:       width,
			Height:      height,
		}, nil
	}
	data, err := jpegenc.EncodeRGB(width, height, pix)
	if err != nil {
		return Sanitized{}, &EncodeError{Err: err}
	}
	return Sanitized{Data: data, ContentType: "image/jpeg", Width: width, Height: height}, nil
}

// decode is the try block of sanitize_image: open, the pixel limit, load,
// exif_transpose, the alpha check and the conversion to RGB or RGBA.
func decode(raw []byte) (mode string, width, height int, pix []byte, err error) {
	_, opened, err := imageopen.Open(raw, registry)
	if err != nil {
		return "", 0, 0, nil, classify(err)
	}
	w, h := opened.Size()
	if int64(w)*int64(h) > MaxPixels {
		return "", 0, 0, nil, refuse(CodeImageDimensionsTooLarge, detailDimensions)
	}
	loaded, err := opened.Load()
	if err != nil {
		return "", 0, 0, nil, classify(err)
	}
	transposed, err := exif.Transpose(loaded)
	if err != nil {
		return "", 0, 0, nil, classify(err)
	}
	mode, pix, err = convert.ForEncoder(transposed)
	if err != nil {
		return "", 0, 0, nil, classify(err)
	}
	return mode, transposed.Width, transposed.Height, pix, nil
}

// classify maps an exception inside the try block to what sanitize_image
// raises: a decompression bomb is image_dimensions_too_large, anything else
// not_an_image. Unsupported input passes through as a divergence.
func classify(err error) error {
	var rejected *Rejected
	if errors.As(err, &rejected) {
		return err
	}
	var bomb *pil.BombError
	if errors.As(err, &bomb) {
		return refuse(CodeImageDimensionsTooLarge, detailDimensions)
	}
	var unsupported *pil.UnsupportedError
	if errors.As(err, &unsupported) {
		return err
	}
	return refuse(CodeNotAnImage, detailNotAnImage)
}
