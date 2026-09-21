// Package imageopen reproduces PIL.Image.open's plugin loop (Pillow 12.2.0)
// for the upload sanitizer: the Image.ID order the API process builds, the
// accept test on the 16-byte prefix, the exceptions _open_core catches
// before trying the next plugin, and the decompression bomb check.
package imageopen

import (
	"errors"
	"fmt"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Order is Image.ID in the API process once Image.open has run: nothing
// registers a plugin at import time, so the first open calls preinit (BMP,
// DIB, GIF, JPEG, PPM, PNG) and, when those fail, init, which imports the
// rest in PIL._plugins order; a plugin that imports another registers that
// one first (DCX brings PCX, MPO brings TIFF). FPX and MIC are absent
// because olefile is not installed. Measured in mobile-parity-api:7bf58e3d.
//
// A fresh process tries the preinit plugins, then the ones init added; a
// process that already ran init tries the whole list at once. Both visit
// the same sequence.
var Order = []string{
	"BMP", "DIB", "GIF", "JPEG", "PPM", "PNG",
	"AVIF", "BLP", "BUFR", "CUR", "PCX", "DCX", "DDS", "EPS", "FITS", "FLI",
	"FTEX", "GBR", "GRIB", "HDF5", "JPEG2000", "ICNS", "ICO", "IM", "IMT",
	"IPTC", "MCIDAS", "MPEG", "TIFF", "MSP", "PCD", "PIXAR", "PSD", "QOI",
	"SGI", "SPIDER", "SUN", "TGA", "WEBP", "WMF", "XBM", "XPM", "XVTHUMB",
}

// ErrUnidentified is UnidentifiedImageError: no plugin opened the bytes.
var ErrUnidentified = errors.New("cannot identify image file")

// Open runs Image.open(io.BytesIO(raw)) over plugins, which must hold an
// entry for every name in Order. It returns the name of the plugin that
// opened the bytes. The error is ErrUnidentified, a *pil.BombError, or the
// error of the plugin whose exception escaped Image.open.
func Open(raw []byte, plugins map[string]pil.Plugin) (string, pil.Opened, error) {
	prefix := raw
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	for _, name := range Order {
		plugin, ok := plugins[name]
		if !ok {
			return "", nil, fmt.Errorf("imageopen: no plugin registered for %s", name)
		}
		if plugin.Accept != nil && !plugin.Accept(prefix) {
			continue
		}
		opened, err := plugin.Open(raw)
		if err != nil {
			if pil.IsNext(err) {
				continue
			}
			return name, nil, err
		}
		width, height := opened.Size()
		if width <= 0 || height <= 0 {
			// ImageFile.__init__: "not identified by this driver" is a
			// SyntaxError, raised before _open_core's bomb check.
			continue
		}
		if err := pil.CheckBomb(width, height); err != nil {
			return name, nil, err
		}
		return name, opened, nil
	}
	return "", nil, ErrUnidentified
}
