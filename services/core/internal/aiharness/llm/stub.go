package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"sort"
	"sync"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Buoc is one scripted model reply.
type Buoc struct {
	// Text is a text answer.
	Text string
	// Goi, when set, makes the reply a function call instead.
	Goi *genai.FunctionCall
	// Loi, when set, makes the reply an error (a genai.APIError for a
	// provider failure).
	Loi error
	// Finish is the finish reason; empty means STOP.
	Finish genai.FinishReason
	// Usage is the reply's token accounting.
	Usage *genai.GenerateContentResponseUsageMetadata
	// Cho delays the reply, for deadline cases.
	Cho time.Duration
}

// Stub is a scripted model.LLM: each call takes the next Buoc and records the
// request it was given, in canonical form, before answering. It never opens
// a connection. Running past the script is an error, so a test that expected
// fewer calls is red rather than silently answered.
type Stub struct {
	mu   sync.Mutex
	kich []Buoc
	yeu  [][]byte
}

// NewStub scripts a stub.
func NewStub(kich ...Buoc) *Stub { return &Stub{kich: kich} }

// ErrHetKichBan is what a stub answers past its script.
var ErrHetKichBan = errors.New("llm: stub script exhausted")

// Name is the production model name, so requests read the same.
func (s *Stub) Name() string { return Model }

// SoGoi is how many calls the stub has answered.
func (s *Stub) SoGoi() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.yeu)
}

// YeuCau returns the canonical JSON of every request, in order.
func (s *Stub) YeuCau() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.yeu))
	copy(out, s.yeu)
	return out
}

// GenerateContent answers from the script.
func (s *Stub) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		canon, err := Canon(req)
		if err != nil {
			yield(nil, err)
			return
		}
		s.mu.Lock()
		i := len(s.yeu)
		s.yeu = append(s.yeu, canon)
		var b Buoc
		ok := i < len(s.kich)
		if ok {
			b = s.kich[i]
		}
		s.mu.Unlock()
		if !ok {
			yield(nil, ErrHetKichBan)
			return
		}
		if b.Cho > 0 {
			select {
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case <-time.After(b.Cho):
			}
		}
		if b.Loi != nil {
			yield(nil, b.Loi)
			return
		}
		finish := b.Finish
		if finish == "" {
			finish = genai.FinishReasonStop
		}
		part := &genai.Part{Text: b.Text}
		if b.Goi != nil {
			part = &genai.Part{FunctionCall: b.Goi}
		}
		yield(&model.LLMResponse{
			Content:       &genai.Content{Role: "model", Parts: []*genai.Part{part}},
			UsageMetadata: b.Usage,
			FinishReason:  finish,
			TurnComplete:  true,
		}, nil)
	}
}

// Canon is a request as a model sees it, as stable JSON: sorted keys, the
// system instruction, the contents, the generation config and the names of the
// declared tools. HTTP options (headers the transport adds) are left out:
// they are the wire, not the request. A prompt change shows up as a diff of
// the golden files built from this.
func Canon(req *model.LLMRequest) ([]byte, error) {
	out := map[string]any{"model": req.Model}
	contents, err := roundTrip(req.Contents)
	if err != nil {
		return nil, err
	}
	out["contents"] = contents
	if req.Config != nil {
		cfg := *req.Config
		cfg.HTTPOptions = nil
		c, err := roundTrip(&cfg)
		if err != nil {
			return nil, err
		}
		out["config"] = c
	}
	names := make([]string, 0, len(req.Tools))
	for name := range req.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	out["tools"] = names
	// No HTML escaping: the data blocks' '<' and '>' read as themselves.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// roundTrip turns a genai value into plain JSON values, so maps marshal with
// sorted keys.
func roundTrip(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	return out, json.Unmarshal(raw, &out)
}
