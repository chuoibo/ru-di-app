package nap

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Semantic chunking (owner, 2026-09-28): a facet is never cut; one longer
// than NguongDoan runes is split where the meaning moves most between two
// consecutive sentences, and every piece carries the facet's context line
// (contextual retrieval, DoanQuan).
const (
	// NguongDoan is the longest chunk, in runes. A facet up to it is one
	// chunk and costs no embedding call here.
	NguongDoan = 2000
	// MaxManh bounds the chunks of one facet: the indexer deletes a
	// document by enumerating every id it can have. The feed's longest facet
	// (~2,600 runes) needs two.
	MaxManh = 4
	// PhanViCat: a break is a drop between consecutive sentences at or
	// above this percentile of the facet's drops.
	PhanViCat = 0.90
	// ManhToiThieu: no break at a meaning drop leaves a piece shorter than
	// this (runes); a size break still may.
	ManhToiThieu = NguongDoan / 4
)

// ErrQuaDai: a facet needs more than MaxManh chunks. The document goes to
// the dead-letter queue; nothing is ever cut to fit.
var ErrQuaDai = errors.New("nap: a facet needs more chunks than MaxManh")

// ChiaDoan splits one facet's text into chunks.
type ChiaDoan interface {
	Chia(ctx context.Context, text string) ([]string, error)
}

// ChiaNghia is the semantic chunker: sentences embedded by the pipeline's
// dense encoder, breaks at the largest cosine drops, pieces bounded by
// NguongDoan, each piece after the first opening with the previous piece's
// last sentence (one sentence of overlap).
type ChiaNghia struct {
	Nhung NhungTaiLieu
}

// Chia returns text as one chunk when it fits, else its semantic pieces.
func (c ChiaNghia) Chia(ctx context.Context, text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= NguongDoan {
		return []string{text}, nil
	}
	caus := tachCau(text)
	var drops []float64
	if len(caus) > 1 {
		if c.Nhung == nil {
			return nil, errors.New("nap: semantic chunking needs an encoder")
		}
		docs := make([]TaiLieu, len(caus))
		for i, s := range caus {
			docs[i] = TaiLieu{Chu: s}
		}
		vecs, err := c.Nhung.NhungTaiLieu(ctx, docs)
		if err != nil {
			return nil, err
		}
		if len(vecs) != len(caus) {
			return nil, fmt.Errorf("nap: %d sentence vectors for %d sentences", len(vecs), len(caus))
		}
		drops = make([]float64, len(caus)-1)
		for i := range drops {
			drops[i] = 1 - cosineDay(vecs[i], vecs[i+1])
		}
	}
	out := ghepManh(caus, drops)
	if len(out) > MaxManh {
		return nil, fmt.Errorf("%w: %d", ErrQuaDai, len(out))
	}
	return out, nil
}

// ghepManh packs sentences into pieces: a new piece starts when the next
// sentence would pass NguongDoan, or at a drop at or above the percentile
// above the median once the piece holds ManhToiThieu runes. drops[i] is the drop between
// sentence i and i+1 (nil: size breaks only).
func ghepManh(caus []string, drops []float64) []string {
	// A break needs a drop at or above the percentile AND above the median:
	// when most neighbours are alike the percentile is the common (tiny)
	// drop, and every sentence would look like a change of meaning.
	nguong, trungVi := math.Inf(1), math.Inf(1)
	if len(drops) > 0 {
		nguong, trungVi = phanVi(drops, PhanViCat), phanVi(drops, 0.5)
	}
	var out []string
	var cur []string
	size := func(ss []string) int { return utf8.RuneCountInString(strings.Join(ss, " ")) }
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, " "))
		}
	}
	for i, s := range caus {
		if len(cur) > 0 {
			tooBig := size(append(append([]string{}, cur...), s)) > NguongDoan
			meaning := i > 0 && drops != nil && drops[i-1] >= nguong && drops[i-1] > trungVi && size(cur) >= ManhToiThieu
			if tooBig || meaning {
				flush()
				last := cur[len(cur)-1]
				cur = nil
				if size([]string{last, s}) <= NguongDoan {
					cur = append(cur, last)
				}
			}
		}
		cur = append(cur, s)
	}
	flush()
	return out
}

// tachCau splits text into sentences: at line breaks, and after . ! ? …
// followed by a space. A sentence longer than NguongDoan is cut at word
// boundaries into pieces that fit (the only split not at a sentence end).
func tachCau(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		var b strings.Builder
		rs := []rune(line)
		for i, r := range rs {
			b.WriteRune(r)
			end := r == '.' || r == '!' || r == '?' || r == '…'
			if end && (i+1 == len(rs) || unicode.IsSpace(rs[i+1])) {
				out = appendCau(out, b.String())
				b.Reset()
			}
		}
		out = appendCau(out, b.String())
	}
	return out
}

func appendCau(out []string, s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for utf8.RuneCountInString(s) > NguongDoan {
		rs := []rune(s)
		cut := NguongDoan
		for j := NguongDoan; j > NguongDoan/2; j-- {
			if unicode.IsSpace(rs[j]) {
				cut = j
				break
			}
		}
		out = append(out, strings.TrimSpace(string(rs[:cut])))
		s = strings.TrimSpace(string(rs[cut:]))
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// cosineDay is the full cosine (kho.go's cosine is a dot product over
// vectors already unit length; a sentence vector need not be).
func cosineDay(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		if i >= len(b) {
			break
		}
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

// phanVi is the nearest-rank percentile p of xs.
func phanVi(xs []float64, p float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	i := int(math.Ceil(p*float64(len(s)))) - 1
	return s[max(0, min(i, len(s)-1))]
}

// ChiaNguyen keeps every facet whole: the chunker of a test or a caller that
// has no encoder and knows its texts fit. A text over NguongDoan is an
// error, never cut.
type ChiaNguyen struct{}

func (ChiaNguyen) Chia(_ context.Context, text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > NguongDoan {
		return nil, fmt.Errorf("%w: %d runes and no encoder", ErrQuaDai, utf8.RuneCountInString(text))
	}
	return []string{text}, nil
}
