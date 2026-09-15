package pngenc

import "mobile/services/core/internal/media/sanitize/internal/pngenc/zlib131"

// What Image.save(buf, "PNG") passes for a fresh RGBA image in Pillow
// 12.2.0: PngImagePlugin._apply_encoderinfo leaves optimize False,
// compress_level -1 and compress_type -1; ZipEncode.c then calls
// deflateInit2(level, Z_DEFLATED, 15, 9, Z_FILTERED) for ZIP_PNG.
const (
	compressLevel = zlib131.DefaultCompression
	windowBits    = 15
	memLevel      = 9
	strategy      = zlib131.Filtered
)

// optimize is encoderinfo["optimize"]: it would add the Average filter and
// raise the level to Z_BEST_COMPRESSION (not ported).
const optimize = false

// Filters ZipEncode.c tries for ZIP_PNG lines, in its order: None, Up,
// Sub ("prior"), Average (only with optimize), Paeth.
const (
	filterUp      = true
	filterPrior   = true
	filterAverage = optimize
	filterPaeth   = true
)

// maxBlock is ImageFile.MAXBLOCK.
const maxBlock = 65536

// bufsize is ImageFile._save's max(MAXBLOCK, bufsize, im.size[0] * 4) with
// the PNG plugin's bufsize of 0: every encoder.encode(bufsize) call, and so
// every IDAT chunk but the last, is this long.
func bufsize(width int) int {
	if 4*width > maxBlock {
		return 4 * width
	}
	return maxBlock
}
