// Package motluot runs the one-shot model calls that sit outside a chat
// turn: a public route's structured step (reading a bill, a transfer
// screenshot, an expense in a message; a suggestion card, a reel, a reason
// per place) and a job's (a diary draft, a post's moderation). These steps
// used to be the Python brain's (/internal/brain/v1/*); ADR-0052 moved them
// here, on the same model door as the chat engine (llm.GeminiFromEnv:
// agy-proxy when AGY_PROXY_URL is set).
//
// A process builds one May at startup and shares it. A nil *May is a
// process with no model configured: every call answers ErrChuaCauHinh, and
// each feature maps that to the closed refusal its route has always given
// without a key.
package motluot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
)

// ErrChuaCauHinh: this process has no model.
var ErrChuaCauHinh = errors.New("motluot: no model is configured")

// ErrBan: every seat of this process stayed taken past the call's wait.
var ErrBan = errors.New("motluot: every model seat is busy")

// SongSongServe and SongSongWork are the seats a serve and a work process
// take by default. agy-proxy runs at most 8 requests per client token; the
// chat engine in `core work` holds its own, so the two stay under that
// together (ADR-0052).
const (
	SongSongServe = 4
	SongSongWork  = 2
)

// choGhe is how long a call waits for a seat before it is refused.
const choGhe = 20 * time.Second

// May is the process's model for one-shot calls.
type May struct {
	m   model.LLM
	ghe chan struct{}
	lim llm.GioiHan
	cho func(n int) time.Duration
}

// Moi wraps m with song concurrent seats.
func Moi(m model.LLM, song int) *May {
	if song < 1 {
		song = 1
	}
	return &May{m: m, ghe: make(chan struct{}, song)}
}

// WithGioiHan asks lim before every call, retries included (the
// cross-process MOBILE_MODEL_RPM limiter).
func (y *May) WithGioiHan(lim llm.GioiHan) *May {
	if y != nil {
		y.lim = lim
	}
	return y
}

// WithWait replaces the retry waits (tests pass zero).
func (y *May) WithWait(cho func(n int) time.Duration) *May {
	if y != nil {
		y.cho = cho
	}
	return y
}

// TuEnv builds the process's model from the environment, or nil when no
// model is configured at all (neither AGY_PROXY_URL nor GEMINI_API_KEY):
// a keyless stack still starts and its AI routes refuse as they always
// have. A half-configured one (a URL without its key, a bad base URL) is an
// error, so the process refuses to start instead of failing every call.
func TuEnv(ctx context.Context, getenv func(string) string, song int) (*May, error) {
	if strings.TrimSpace(getenv(llm.EnvAgyURL)) == "" && strings.TrimSpace(getenv(llm.EnvAPIKey)) == "" {
		return nil, nil
	}
	m, err := llm.GeminiFromEnv(ctx, getenv)
	if err != nil {
		return nil, err
	}
	return Moi(m, song), nil
}

// CoMay says whether this process has a model.
func (y *May) CoMay() bool { return y != nil }

// Luot is one feature's call budget: up to tran requests leave the process,
// the first try and every retry alike (llm.Dem).
type Luot struct {
	y   *May
	dem *llm.Dem
}

// Luot opens a budget of tran requests. On a nil May it still returns a
// Luot, whose every call answers ErrChuaCauHinh.
func (y *May) Luot(tran int) *Luot {
	if y == nil {
		return &Luot{}
	}
	dem := llm.NewDem(y.m, tran, nil)
	if y.lim != nil {
		dem = dem.WithGioiHan(y.lim)
	}
	if y.cho != nil {
		dem = dem.WithWait(y.cho)
	}
	return &Luot{y: y, dem: dem}
}

// Goi makes one structured call and returns the answer's text (thoughts
// left out). It holds one of the process's seats while the call runs.
func (l *Luot) Goi(ctx context.Context, req *model.LLMRequest) (string, error) {
	if l == nil || l.y == nil {
		return "", ErrChuaCauHinh
	}
	wait, cancel := context.WithTimeout(ctx, choGhe)
	defer cancel()
	select {
	case l.y.ghe <- struct{}{}:
	case <-wait.Done():
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrBan
	}
	defer func() { <-l.y.ghe }()
	return cautruc.Goi(ctx, l.dem, req)
}

// SoGoi is how many requests this budget has let out, retries included.
func (l *Luot) SoGoi() int {
	if l == nil || l.dem == nil {
		return 0
	}
	return l.dem.SoGoi()
}

// Token is this budget's usage so far.
func (l *Luot) Token() llm.Token {
	if l == nil || l.dem == nil {
		return llm.Token{}
	}
	return l.dem.Token()
}

// ErrKhongDocDuoc: the model's answer is not one JSON object.
var ErrKhongDocDuoc = errors.New("motluot: the answer is not a JSON object")

// DocDoiTuong decodes a structured answer: exactly one JSON object, numbers
// kept as json.Number so nothing is rounded before the domain reads them.
func DocDoiTuong(text string) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(BoRao(text))))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil || out == nil || dec.More() {
		return nil, ErrKhongDocDuoc
	}
	return out, nil
}

// BoRao is text without one Markdown code fence around it ("```json" … "```").
// The proxy does not always hold the model to the JSON MIME type, and a
// fenced object is still the one object asked for; anything else is left as
// it came, for the decoder to refuse.
func BoRao(text string) string {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "```") || !strings.HasSuffix(t, "```") || len(t) < 6 {
		return text
	}
	t = strings.TrimSuffix(t, "```")
	first, rest, ok := strings.Cut(t, "\n")
	if !ok {
		return text
	}
	if lang := strings.TrimSpace(strings.TrimPrefix(first, "```")); lang != "" && !strings.EqualFold(lang, "json") {
		return text
	}
	return rest
}
