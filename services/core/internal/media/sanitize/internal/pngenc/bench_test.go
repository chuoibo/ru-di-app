package pngenc

import "testing"

// BenchmarkPhotoRGBA encodes the 2000x1500 "photo" corpus image, the
// slowest content for deflate_slow's chain walk.
func BenchmarkPhotoRGBA(b *testing.B) {
	c := corpusCase{Content: "photo", W: 2000, H: 1500, Seed: 7}
	pix := c.pixels()
	b.SetBytes(int64(len(pix)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeRGBA(c.W, c.H, pix)
	}
}
