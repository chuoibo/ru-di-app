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
	// lim, when set, is asked before every call (GioiHan).
	lim GioiHan

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

// WithGioiHan asks lim before every call, the first try and every retry. A
// refusal ends the call with ErrGioiHan before it is counted; a limiter that
// fails to answer lets the call through (fail open, design 02 §6).
func (d *Dem) WithGioiHan(lim GioiHan) *Dem { d.lim = lim; return d }

// Name is the wrapped model's name.
func (d *Dem) Name() string { return d.inner.Name() }

// GetGoogleLLMVariant is the wrapped model's backend, so ADK prepares each
// request for the backend that will really serve it. Dem deliberately has no
// Client method: ADK opens a live session straight from that client, and
// such a session would never pass the counter.
func (d *Dem) GetGoogleLLMVariant() genai.Backend {
	if v, ok := d.inner.(googleLLM); ok {
		return v.GetGoogleLLMVariant()
	}
	return genai.BackendUnspecified
}

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
//
// When the budget refuses a retry, the turn ends with the provider error the
// retry was for, not ErrHetNganSach: the provider failing is what happened,
// and the job's code says so (provider_unavailable with its 5xx or 429
// class, a transient failure) rather than telling the user an outage was an
// exhausted budget. A first call the budget refuses is still ErrHetNganSach.
func (d *Dem) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// The retryable provider error the next call would retry.
		var truoc error
		for lan := 0; ; lan++ {
			// The limiter before the counter: a call it refuses never
			// leaves the process, so it must not spend the turn's budget.
			if d.lim != nil {
				if ok, err := d.lim.Xin(ctx, d.inner.Name()); err == nil && !ok {
					yield(nil, ErrGioiHan)
					return
				}
			}
			if err := d.giuMot(ctx); err != nil {
				if truoc != nil && errors.Is(err, ErrHetNganSach) {
					err = truoc
				}
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
			truoc = last
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
