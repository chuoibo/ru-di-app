package llm

import (
	"context"
	"errors"
	"iter"
	"sync"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Dem is one turn's model: every GenerateContent it forwards, the first try
// and every retry alike, is counted before it goes out, and the call past the
// ceiling is refused with ErrHetNganSach instead of made.
//
// Retries are made here, not in genai: two, on 429 and 503, before anything
// has been yielded (S1 never streams, so that is always). Each retry passes
// the counter like any other call, so the ceiling is the real number of
// requests that left the process.
type Dem struct {
	inner model.LLM
	max   int
	// giu, when set, holds one call on the job's durable counter before the
	// call is made (design 01 §2, GiuLuot); nil counts in memory only.
	giu func(context.Context) error
	// cho is the wait before retry n (1-based).
	cho func(n int) time.Duration

	mu sync.Mutex
	n  int
}

// NewDem wraps inner for one turn with room for max calls.
func NewDem(inner model.LLM, max int, giu func(context.Context) error) *Dem {
	return &Dem{inner: inner, max: max, giu: giu, cho: func(n int) time.Duration {
		if n <= 1 {
			return 300 * time.Millisecond
		}
		return 1200 * time.Millisecond
	}}
}

// WithWait replaces the retry waits (tests pass zero).
func (d *Dem) WithWait(cho func(n int) time.Duration) *Dem { d.cho = cho; return d }

// Name is the wrapped model's name.
func (d *Dem) Name() string { return d.inner.Name() }

// SoGoi is how many calls have gone out, retries included.
func (d *Dem) SoGoi() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

// ConLai is how many calls are left.
func (d *Dem) ConLai() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.max - d.n
}

func (d *Dem) giuMot(ctx context.Context) error {
	d.mu.Lock()
	if d.n >= d.max {
		d.mu.Unlock()
		return ErrHetNganSach
	}
	d.n++
	d.mu.Unlock()
	if d.giu != nil {
		return d.giu(ctx)
	}
	return nil
}

const maxRetry = 2

// GenerateContent forwards to the wrapped model under the counter.
func (d *Dem) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for lan := 0; ; lan++ {
			if err := d.giuMot(ctx); err != nil {
				yield(nil, err)
				return
			}
			yielded := false
			var last error
			for resp, err := range d.inner.GenerateContent(ctx, req, stream) {
				if err != nil && !yielded && lan < maxRetry && retryable(err) {
					last = err
					break
				}
				yielded = true
				if !yield(resp, err) {
					return
				}
				if err != nil {
					return
				}
			}
			if last == nil {
				return
			}
			select {
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case <-time.After(d.cho(lan + 1)):
			}
		}
	}
}

func retryable(err error) bool {
	var v genai.APIError
	if errors.As(err, &v) {
		return v.Code == 429 || v.Code == 503
	}
	var p *genai.APIError
	if errors.As(err, &p) && p != nil {
		return p.Code == 429 || p.Code == 503
	}
	return false
}
