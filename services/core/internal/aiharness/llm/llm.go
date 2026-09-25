// Package llm is the AI engine's only door to a model: the Gemini model built
// from Go (ADK's gemini constructor over google.golang.org/genai), a scripted
// stub for every test, and the per-turn counter that holds a turn to
// MaxModelCallsPerTurn, retries included (ADR-0037 §2.5).
//
// Nothing else in the engine builds a model client. Under `go test` a Gemini
// client can only be pointed at a loopback host, so no test, gate or parity
// run can reach the real provider (ADR-0037 §2.10, ADR-0034 §2.6).
package llm

import (
	"context"
	"errors"
	"iter"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// Model is the one model every text-producing step uses.
const Model = "gemini-3.5-flash-lite"

// MaxModelCallsPerTurn bounds the model calls of one turn, counting every
// retry. The eval reads it to budget real calls (design 01 §5).
const MaxModelCallsPerTurn = 8

// Environment variables GeminiFromEnv reads.
const (
	EnvAPIKey  = "GEMINI_API_KEY"
	EnvBaseURL = "MOBILE_GEMINI_BASE_URL"
)

// DefaultBaseURL is the Gemini API. It is set explicitly so no environment
// variable read inside genai (GOOGLE_GEMINI_BASE_URL) can move it.
const DefaultBaseURL = "https://generativelanguage.googleapis.com/"

var (
	// ErrNotConfigured: no API key.
	ErrNotConfigured = errors.New("llm: GEMINI_API_KEY is not set")
	// ErrHetNganSach: the turn has spent MaxModelCallsPerTurn.
	ErrHetNganSach = errors.New("llm: model call budget for this turn is spent")
	// ErrKhongUngVien: the provider answered with no candidate at all, which
	// is what Gemini does when it blocks the prompt itself
	// (promptFeedback.blockReason). ADK's non-streaming call drops the block
	// reason and reports a bare error, so the engine reads this as the
	// provider's safety refusal, not as the provider being down.
	ErrKhongUngVien = errors.New("llm: the provider returned no candidate (a blocked prompt)")
)

// adkKhongUngVien is the message of the error ADK's gemini model returns for
// a response with no candidates (google.golang.org/adk v1.7.0,
// model/gemini/gemini.go). TestGeminiChanCauHoi drives the real transport
// into it, so an ADK upgrade that changes the message is red there.
const adkKhongUngVien = "empty response"

// geminiModel is ADK's gemini model with that one error given its meaning.
// Embedding the interface hides every method ADK's concrete type has beyond
// model.LLM, so the two ADK looks for are forwarded: GetGoogleLLMVariant
// (without it ADK reads the backend as Unspecified, skips the Gemini-API-only
// request fixes and refuses live) and Client.
type geminiModel struct{ model.LLM }

// googleLLM is what ADK asks a model for its backend
// (internal/llminternal/googlellm.GoogleLLM).
type googleLLM interface {
	GetGoogleLLMVariant() genai.Backend
}

// GetGoogleLLMVariant forwards to ADK's model.
func (g geminiModel) GetGoogleLLMVariant() genai.Backend {
	if v, ok := g.LLM.(googleLLM); ok {
		return v.GetGoogleLLMVariant()
	}
	return genai.BackendUnspecified
}

// Client forwards to ADK's model.
func (g geminiModel) Client() *genai.Client {
	if c, ok := g.LLM.(interface{ Client() *genai.Client }); ok {
		return c.Client()
	}
	return nil
}

// GenerateContent forwards to ADK's model.
func (g geminiModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for resp, err := range g.LLM.GenerateContent(ctx, req, stream) {
			if err != nil && err.Error() == adkKhongUngVien {
				err = ErrKhongUngVien
			}
			if !yield(resp, err) {
				return
			}
		}
	}
}

// CheckBaseURL accepts an override only for a loopback host: the override
// exists to point the real genai transport at a local stub, never to send
// requests (and the key) anywhere else.
func CheckBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return errors.New("llm: " + EnvBaseURL + " must be an http(s) URL")
	}
	if !loopback(u.Hostname()) {
		return errors.New("llm: " + EnvBaseURL + " may only name a loopback host")
	}
	return nil
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// NewGemini builds the Gemini model. baseURL "" means the real API, which a
// test binary is refused.
func NewGemini(ctx context.Context, apiKey, baseURL string) (model.LLM, error) {
	if apiKey == "" {
		return nil, ErrNotConfigured
	}
	if baseURL != "" {
		if err := CheckBaseURL(baseURL); err != nil {
			return nil, err
		}
	} else {
		if testing.Testing() {
			return nil, errors.New("llm: a test binary may only reach a loopback Gemini")
		}
		baseURL = DefaultBaseURL
	}
	m, err := gemini.NewModel(ctx, Model, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
		// No HTTPRetryOptions: retries are this package's, so each one passes
		// the counter (dem.go).
		HTTPOptions: genai.HTTPOptions{BaseURL: baseURL},
		HTTPClient:  &http.Client{Timeout: 45 * time.Second},
	})
	if err != nil {
		return nil, err
	}
	return geminiModel{m}, nil
}

// GeminiFromEnv reads GEMINI_API_KEY and MOBILE_GEMINI_BASE_URL once.
func GeminiFromEnv(ctx context.Context, getenv func(string) string) (model.LLM, error) {
	return NewGemini(ctx, getenv(EnvAPIKey), getenv(EnvBaseURL))
}
