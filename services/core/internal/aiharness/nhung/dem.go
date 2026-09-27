package nhung

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrHetNganSach: the turn has used its MaxEmbedCallsPerTurn embedding
// requests; the retrieval that asked runs without its dense leg and says so
// (truyhoi.NoVector), it does not fail.
var ErrHetNganSach = errors.New("nhung: the turn's embedding budget is spent")

// Dem is one turn's embedder: every request it forwards is counted before it
// goes out, the request past the ceiling is refused with ErrHetNganSach, and
// a query text already embedded in this turn (same task, same NFC text) is
// answered from memory without a request. Build one per turn with
// llm.MaxEmbedCallsPerTurn; it is safe for concurrent use.
type Dem struct {
	inner Nhung
	max   int

	mu  sync.Mutex
	n   int
	nho map[string][]float32
}

// NewDem wraps inner for one turn with room for max requests.
func NewDem(inner Nhung, max int) *Dem {
	return &Dem{inner: inner, max: max, nho: map[string][]float32{}}
}

func (d *Dem) Model() string { return d.inner.Model() }
func (d *Dem) Dims() int     { return d.inner.Dims() }

// SoGoi is how many requests this turn made.
func (d *Dem) SoGoi() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return int64(d.n)
}

// soYeuCau is how many provider requests n inputs take.
func soYeuCau(n int) int { return (n + MaxBatch - 1) / MaxBatch }

func (d *Dem) giu(k int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.n+k > d.max {
		return ErrHetNganSach
	}
	d.n += k
	return nil
}

// Nhung answers texts already embedded this turn from memory and embeds the
// rest under the counter.
func (d *Dem) Nhung(ctx context.Context, texts []string, tv TacVu) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrRong
	}
	out := make([][]float32, len(texts))
	var thieu []string
	var viTri []int
	d.mu.Lock()
	for i, t := range texts {
		if v, ok := d.nho[string(tv)+"\x00"+ChuanNFC(t)]; ok {
			out[i] = v
			continue
		}
		thieu = append(thieu, t)
		viTri = append(viTri, i)
	}
	d.mu.Unlock()
	if len(thieu) == 0 {
		return out, nil
	}
	if err := d.giu(soYeuCau(len(thieu))); err != nil {
		return nil, err
	}
	vs, err := d.inner.Nhung(ctx, thieu, tv)
	if err != nil {
		return nil, err
	}
	if err := kiemSoVector(vs, len(thieu), d.inner.Dims()); err != nil {
		return nil, err
	}
	d.mu.Lock()
	for j, v := range vs {
		out[viTri[j]] = v
		d.nho[string(tv)+"\x00"+ChuanNFC(thieu[j])] = v
	}
	d.mu.Unlock()
	return out, nil
}

// NhungTaiLieu embeds documents under the counter (no memo: documents are
// embedded by the ingest, not by a turn).
func (d *Dem) NhungTaiLieu(ctx context.Context, docs []TaiLieuVao) ([][]float32, error) {
	if len(docs) == 0 {
		return nil, ErrRong
	}
	if err := d.giu(soYeuCau(len(docs))); err != nil {
		return nil, err
	}
	vs, err := d.inner.NhungTaiLieu(ctx, docs)
	if err != nil {
		return nil, err
	}
	if err := kiemSoVector(vs, len(docs), d.inner.Dims()); err != nil {
		return nil, err
	}
	return vs, nil
}

// ErrSoVector: an embedder answered a different number of vectors than it
// was asked for.
var ErrSoVector = errors.New("nhung: the embedder answered a different number of vectors")

// kiemSoVector refuses an answer with the wrong number of vectors (extra
// ones would index past the request, missing ones would leave a nil dense
// vector nobody flagged) or a vector of the wrong length. The caller then
// runs without its dense leg and says so (truyhoi.NoVector).
func kiemSoVector(vs [][]float32, n, dims int) error {
	if len(vs) != n {
		return fmt.Errorf("%w: %d for %d", ErrSoVector, len(vs), n)
	}
	for _, v := range vs {
		if len(v) != dims {
			return fmt.Errorf("%w: got %d, want %d", ErrSaiChieu, len(v), dims)
		}
	}
	return nil
}
