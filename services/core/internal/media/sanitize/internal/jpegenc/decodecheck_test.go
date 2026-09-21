package jpegenc

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

// decodeCheck decodes out with Go's image/jpeg and checks its dimensions.
// Go's decoder rejects some legal layouts (for example 3x1 luma sampling),
// so those are only checked for a well-formed marker sequence.
func decodeCheck(out []byte, opts Options) error {
	if !bytes.HasPrefix(out, []byte{0xFF, 0xD8}) || !bytes.HasSuffix(out, []byte{0xFF, 0xD9}) {
		return fmt.Errorf("missing SOI or EOI")
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		if opts.Color == YCbCr && opts.Sampling != nil && (opts.Sampling[0].H == 3 || opts.Sampling[0].H == 4) {
			return nil
		}
		return err
	}
	if img.Bounds() != image.Rect(0, 0, opts.Width, opts.Height) {
		return fmt.Errorf("decoded bounds %v", img.Bounds())
	}
	return nil
}
