package zlib131

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"
)

type rng uint64

func (r *rng) next() uint64 {
	*r += 0x9e3779b97f4a7c15
	z := uint64(*r)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// deflateChunked compresses data feeding inChunk bytes and offering
// outChunk bytes of output per deflate() call, like a C caller would.
func deflateChunked(t *testing.T, data []byte, inChunk, outChunk int) []byte {
	t.Helper()
	var z Stream
	if rc := z.DeflateInit2(DefaultCompression, 15, 9, Filtered); rc != OK {
		t.Fatalf("init: %d", rc)
	}
	var out []byte
	buf := make([]byte, outChunk)
	for len(data) > 0 || len(z.NextIn) > 0 {
		if len(z.NextIn) == 0 {
			n := inChunk
			if n > len(data) {
				n = len(data)
			}
			z.NextIn = data[:n]
			data = data[n:]
		}
		z.NextOut = buf
		if rc := z.Deflate(NoFlush); rc < 0 {
			t.Fatalf("deflate: %d", rc)
		}
		out = append(out, buf[:outChunk-len(z.NextOut)]...)
	}
	for {
		z.NextOut = buf
		rc := z.Deflate(Finish)
		out = append(out, buf[:outChunk-len(z.NextOut)]...)
		if rc == StreamEnd {
			return out
		}
		if rc < 0 {
			t.Fatalf("finish: %d", rc)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	r := rng(7)
	inputs := map[string][]byte{"empty": nil}
	noise := make([]byte, 300000)
	for i := range noise {
		noise[i] = byte(r.next())
	}
	inputs["noise"] = noise
	text := make([]byte, 400000)
	for i := range text {
		text[i] = "abcab cabbage\n"[r.next()%14]
	}
	inputs["text"] = text
	runs := make([]byte, 250000)
	for i := range runs {
		runs[i] = byte(i / 3000)
	}
	inputs["runs"] = runs
	mixed := append(append([]byte{}, text[:90000]...), noise[:120000]...)
	mixed = append(mixed, runs...)
	inputs["mixed"] = mixed

	for name, data := range inputs {
		for _, chunks := range [][2]int{{1 << 30, 1 << 20}, {1, 7}, {261, 65536}, {4001, 97}, {65536, 16384}} {
			compressed := deflateChunked(t, data, chunks[0], chunks[1])
			reader, err := zlib.NewReader(bytes.NewReader(compressed))
			if err != nil {
				t.Fatalf("%s %v: %v", name, chunks, err)
			}
			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("%s %v: %v", name, chunks, err)
			}
			if !bytes.Equal(got, data) {
				t.Fatalf("%s %v: round trip differs", name, chunks)
			}
		}
	}
}

func TestStaticTables(t *testing.T) {
	// Spot values of trees_tbl.h in zlib-ng 2.3.3.
	checks := []struct {
		name      string
		got, want int
	}{
		{"static_ltree[0].Code", int(staticLtree[0].fc), 12},
		{"static_ltree[0].Len", int(staticLtree[0].dl), 8},
		{"static_ltree[143].Code", int(staticLtree[143].fc), 253},
		{"static_ltree[144].Code", int(staticLtree[144].fc), 19},
		{"static_ltree[144].Len", int(staticLtree[144].dl), 9},
		{"static_ltree[256].Len", int(staticLtree[256].dl), 7},
		{"static_ltree[287].Len", int(staticLtree[287].dl), 8},
		{"static_dtree[1].Code", int(staticDtree[1].fc), 16},
		{"base_length[27]", baseLength[27], 224},
		{"base_length[28]", baseLength[28], 0},
		{"base_dist[29]", baseDist[29], 24576},
		{"length_code[255]", int(lengthCode[255]), 28},
		{"dist_code[511]", int(distCode[511]), 29},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
