package jpegenc

// The sanitizer's encoder call is Image.save(format="JPEG", quality=88,
// optimize=True) with Pillow's default subsampling, which leaves libjpeg's
// jpeg_set_colorspace(JCS_YCbCr) sampling: 2x2 luma, 1x1 chroma. Each
// constant is on its own line so a mutant can flip exactly one of them.
const (
	sanitizeQuality  = 88
	sanitizeLumaH    = 2
	sanitizeLumaV    = 2
	sanitizeOptimize = true
)
