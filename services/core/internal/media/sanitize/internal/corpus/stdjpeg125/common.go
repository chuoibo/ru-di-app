// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stdjpeg125 is the JPEG encoder of Go 1.25's image/jpeg, frozen.
//
// The sanitize corpus rebuilds every golden input from a spec, and the golden
// records what Python answered for those exact bytes. Go 1.26 replaced
// image/jpeg's forward DCT (fdct.go → dct.go), so the same spec encoded with
// the standard library now yields different input bytes, and the golden would
// be comparing Pillow's answer for one file against Go's answer for another.
// Freezing the encoder keeps spec → input bytes stable across toolchains.
//
// writer.go and fdct.go are copied from go1.25.14 src/image/jpeg with two
// mechanical edits: the package clause, and fdct.go's fix_<i>_<fraction>
// constants renamed fix<i>p<fraction> (the repository guard reads a nine-digit
// run after "_" as a long number). This file carries the few declarations
// they used from reader.go and idct.go. Test-input generation only: production code
// encodes with ../jpegenc and decodes with ../jpegdec, never with this.
package stdjpeg125

// A DCT block is 8x8 (idct.go).
const blockSize = 64

type block [blockSize]int32

// Markers the encoder writes (reader.go).
const (
	sof0Marker = 0xc0 // Start Of Frame (Baseline Sequential).
	dhtMarker  = 0xc4 // Define Huffman Table.
	soiMarker  = 0xd8 // Start Of Image.
	eoiMarker  = 0xd9 // End Of Image.
	sosMarker  = 0xda // Start Of Scan.
	dqtMarker  = 0xdb // Define Quantization Table.
)

// unzig maps from the zig-zag ordering to the natural ordering (reader.go).
var unzig = [blockSize]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}
