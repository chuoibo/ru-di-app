package rag

import _ "embed"

// vangDiaDiem is the place retrieval golden set (design 04 §8.3): wholly
// synthetic places and questions. One file, read by the lexical tests
// (vang_test.go) and embedded for the vector index's promotion gate
// (rag/nap), so the two indexes are measured on the same set, never on a
// copy.
//
//go:embed testdata/truy-hoi-dia-diem.json
var vangDiaDiem []byte

// VangDiaDiem returns the golden set's bytes.
func VangDiaDiem() []byte { return append([]byte(nil), vangDiaDiem...) }
