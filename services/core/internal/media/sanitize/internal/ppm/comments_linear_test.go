package ppm

import (
	"bytes"
	"math/rand"
	"testing"
)

// oldIgnoreComments is the removed quadratic version, kept as the reference
// the linear pass must agree with.
func oldIgnoreComments(block []byte) ([]byte, bool) {
	spans := false
	for {
		start := bytes.IndexByte(block, '#')
		if start == -1 {
			break
		}
		end := commentEnd(block, start)
		if end != -1 {
			joined := make([]byte, 0, len(block))
			joined = append(joined, block[:start]...)
			block = append(joined, block[end+1:]...)
		} else {
			block = block[:start]
			spans = true
			break
		}
	}
	return block, spans
}

func TestCommentStrippingMatchesTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	alphabet := []byte("0123 #\n\r9")
	for range 5000 {
		block := make([]byte, rng.Intn(60))
		for i := range block {
			block[i] = alphabet[rng.Intn(len(alphabet))]
		}
		want, wantSpans := oldIgnoreComments(append([]byte(nil), block...))
		p := &plainDecoder{}
		got := p.ignoreComments(append([]byte(nil), block...))
		if !bytes.Equal(got, want) || p.commentSpans != wantSpans {
			t.Fatalf("%q: got %q/%v want %q/%v", block, got, p.commentSpans, want, wantSpans)
		}
	}
}

// A block of many short comments costs one pass and one buffer (audit
// 2026-10-05, CODEC-03): the reference copied the rest of the block per
// comment, and searched for a missing '\r' to the end of it each time.
func TestManyCommentsCostOneBuffer(t *testing.T) {
	block := bytes.Repeat([]byte("#c\n1 "), 200000)
	allocs := testing.AllocsPerRun(3, func() {
		p := &plainDecoder{}
		if got := p.ignoreComments(block); len(got) != 2*200000 {
			t.Fatalf("kept %d bytes", len(got))
		}
	})
	// A constant, not one per comment: the reference made 200,000. The exact
	// figure (the output buffer, the decoder, a closure) depends on what the
	// compiler keeps on the stack under each build's flags.
	if allocs > 8 {
		t.Fatalf("%v allocations for one block of 200000 comments", allocs)
	}
}
