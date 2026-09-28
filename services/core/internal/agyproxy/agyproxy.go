// Package agyproxy is a plain-HTTP client for the vnlocal machine's agy-proxy:
// the LAN gateway that speaks the OpenAI and Gemini wire formats in front of a
// shared pool of Gemini seats (vnlocal HANDOFF-KET-NOI.md §5).
//
// It covers the proxy's own endpoints (/v1/agy/run, /v1/agy/agent), the
// OpenAI-compatible chat completion, and the two probes (/health,
// /v1/models). There is no SDK underneath: every call is one request whose
// body this file builds, so the wire shape is readable here.
//
// The client never retries. A 429 comes back as *Error with RetryAfter set, so
// the caller decides whether a retry fits its own budget.
//
// Under `go test` the client only accepts a loopback base URL, so no test can
// spend the shared quota or send the token off the machine.
package agyproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Environment variables FromEnv reads.
const (
	EnvURL = "AGY_PROXY_URL" // http://<VNLOCAL_HOST>:20131
	EnvKey = "AGY_PROXY_KEY" // client token, must be in the proxy's allowlist
)

// Models the proxy serves (HANDOFF-KET-NOI.md §5.1).
const (
	ModelLow    = "gemini-3.8-flash-low"
	ModelMedium = "gemini-3.8-flash-medium"
	ModelHigh   = "gemini-3.8-flash-high"
)

// DefaultTimeout matches the proxy's own 180 s ceiling per run, plus slack
// for the queue in front of it.
const DefaultTimeout = 200 * time.Second

// maxResponseBytes caps what one answer may read into memory.
const maxResponseBytes = 16 << 20

// ErrNotConfigured means AGY_PROXY_URL or AGY_PROXY_KEY is missing.
var ErrNotConfigured = errors.New("agyproxy: " + EnvURL + " and " + EnvKey + " are required")

// Client talks to one agy-proxy with one token.
type Client struct {
	BaseURL string // scheme://host:port, no path
	Token   string
	HTTP    *http.Client
}

// New checks the base URL and builds a client.
func New(baseURL, token string) (*Client, error) {
	if baseURL == "" || token == "" {
		return nil, ErrNotConfigured
	}
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, errors.New("agyproxy: " + EnvURL + " must be an http(s) URL without credentials")
	}
	if testing.Testing() && !loopback(u.Hostname()) {
		return nil, errors.New("agyproxy: a test binary may only reach a loopback proxy")
	}
	return &Client{
		BaseURL: strings.TrimRight(u.Scheme+"://"+u.Host, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: DefaultTimeout},
	}, nil
}

// FromEnv reads AGY_PROXY_URL and AGY_PROXY_KEY.
func FromEnv(getenv func(string) string) (*Client, error) {
	return New(getenv(EnvURL), getenv(EnvKey))
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Error is a non-2xx answer from the proxy.
type Error struct {
	Status     int
	Detail     string        // the proxy's `detail`, or the raw body
	RunID      string        // `detail.run_id` on a 502: quote it to the machine's operator
	RetryAfter time.Duration // set on 429 / 503 when the proxy sent Retry-After
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("agyproxy: HTTP %d", e.Status)
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	if e.RunID != "" {
		msg += " (run_id " + e.RunID + ")"
	}
	return msg
}

// Unauthorized: the token is wrong, revoked or not in the allowlist.
func (e *Error) Unauthorized() bool { return e.Status == http.StatusUnauthorized }

// Busy: no free seat or the proxy is restarting; wait RetryAfter.
func (e *Error) Busy() bool {
	return e.Status == http.StatusTooManyRequests || e.Status == http.StatusServiceUnavailable
}

// Health calls GET /health, which needs no token.
func (c *Client) Health(ctx context.Context) error {
	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.do(ctx, http.MethodGet, "/health", nil, &out, false); err != nil {
		return err
	}
	if !out.OK {
		return errors.New("agyproxy: /health answered ok=false")
	}
	return nil
}

// Models lists the model ids (GET /v1/models, OpenAI shape). It is also the
// cheapest way to check the token: a key outside the allowlist gets 401.
func (c *Client) Models(ctx context.Context) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/models", nil, &out, true); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// RunRequest is one turn on POST /v1/agy/run.
//
// GoogleSearch and JSONSchema may be combined (the proxy searches, then
// structures that answer in a second pass).
type RunRequest struct {
	Model             string          `json:"model"`
	SystemInstruction string          `json:"system_instruction,omitempty"`
	Input             string          `json:"input"`
	GoogleSearch      bool            `json:"google_search,omitempty"`
	JSONSchema        json.RawMessage `json:"json_schema,omitempty"`
}

// Grounding lists what a Google Search turn looked up and cited. An empty
// Queries means the model answered from memory.
type Grounding struct {
	Queries []string `json:"queries"`
	Sources []struct {
		Title string `json:"title"`
		URI   string `json:"uri"`
	} `json:"sources"`
}

// Usage is the token count the proxy reports.
type Usage map[string]any

// RunResult is the answer of /v1/agy/run.
type RunResult struct {
	Text         string          `json:"text"`
	JSON         json.RawMessage `json:"json"`
	Grounding    *Grounding      `json:"grounding"`
	Usage        Usage           `json:"usage"`
	RunID        string          `json:"run_id"`
	FinishReason string          `json:"finish_reason"`
}

// Run asks one question.
func (c *Client) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	var out RunResult
	err := c.do(ctx, http.MethodPost, "/v1/agy/run", req, &out, true)
	return out, err
}

// Tool is a function the model may ask the caller to run.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema of the arguments
}

// AgentRequest is one step on POST /v1/agy/agent. The first step may carry
// Input alone; later steps carry the whole history in Contents (Gemini
// shape), which Continue builds.
type AgentRequest struct {
	Model             string            `json:"model"`
	SystemInstruction string            `json:"system_instruction,omitempty"`
	Input             string            `json:"input,omitempty"`
	Contents          []json.RawMessage `json:"contents,omitempty"`
	Tools             []Tool            `json:"tools,omitempty"`
}

// FunctionCall is the model asking for one tool run.
type FunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// AgentStep is the answer of one agent step. Done=false means FunctionCalls
// must be run and their results sent back with Continue.
type AgentStep struct {
	Text          string            `json:"text"`
	FunctionCalls []FunctionCall    `json:"function_calls"`
	Parts         []json.RawMessage `json:"parts"`
	Done          bool              `json:"done"`
	Usage         Usage             `json:"usage"`
	RunID         string            `json:"run_id"`
}

// Agent runs one step.
func (c *Client) Agent(ctx context.Context, req AgentRequest) (AgentStep, error) {
	var out AgentStep
	err := c.do(ctx, http.MethodPost, "/v1/agy/agent", req, &out, true)
	return out, err
}

// FunctionResult is what the caller's tool returned for one FunctionCall.
type FunctionResult struct {
	Name     string
	Response map[string]any
}

// Continue builds the next step's request: the history so far, the model's
// parts exactly as returned (they carry Gemini's thought signatures, so none
// may be dropped or rewritten), then the tool results as one user turn.
func Continue(prev AgentRequest, step AgentStep, results []FunctionResult) (AgentRequest, error) {
	next := prev
	next.Contents = append([]json.RawMessage(nil), prev.Contents...)
	if prev.Input != "" {
		first, err := json.Marshal(content{Role: "user", Parts: []any{map[string]string{"text": prev.Input}}})
		if err != nil {
			return AgentRequest{}, err
		}
		next.Contents = append(next.Contents, first)
		next.Input = ""
	}
	parts := make([]any, len(step.Parts))
	for i, p := range step.Parts {
		parts[i] = p
	}
	model, err := json.Marshal(content{Role: "model", Parts: parts})
	if err != nil {
		return AgentRequest{}, err
	}
	answers := make([]any, len(results))
	for i, r := range results {
		answers[i] = map[string]any{"functionResponse": map[string]any{"name": r.Name, "response": r.Response}}
	}
	user, err := json.Marshal(content{Role: "user", Parts: answers})
	if err != nil {
		return AgentRequest{}, err
	}
	next.Contents = append(next.Contents, model, user)
	return next, nil
}

type content struct {
	Role  string `json:"role"`
	Parts []any  `json:"parts"`
}

// Message is one OpenAI chat message with text content.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// ChatRequest is an OpenAI chat completion. ResponseFormat is passed through
// as is, e.g. {"type":"json_schema","json_schema":{"name":"x","schema":{…}}}.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"` // includes thinking tokens; too small returns "" with finish "length"
	Seed           *int            `json:"seed,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
	WebSearch      bool            `json:"-"`
}

// ChatResult is the first choice of a chat completion.
type ChatResult struct {
	Content      string
	FinishReason string
	Usage        Usage
	Grounding    *Grounding // set when WebSearch was on (`vnlocal.grounding`)
}

// Chat calls POST /v1/chat/completions.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResult, error) {
	type wire struct {
		ChatRequest
		WebSearchOptions *struct{} `json:"web_search_options,omitempty"`
	}
	body := wire{ChatRequest: req}
	if req.WebSearch {
		body.WebSearchOptions = &struct{}{}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage   Usage `json:"usage"`
		Vnlocal struct {
			Grounding *Grounding `json:"grounding"`
		} `json:"vnlocal"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/chat/completions", body, &out, true); err != nil {
		return ChatResult{}, err
	}
	if len(out.Choices) == 0 {
		return ChatResult{}, errors.New("agyproxy: chat completion has no choice")
	}
	return ChatResult{
		Content:      out.Choices[0].Message.Content,
		FinishReason: out.Choices[0].FinishReason,
		Usage:        out.Usage,
		Grounding:    out.Vnlocal.Grounding,
	}, nil
}

func (c *Client) do(ctx context.Context, method, path string, in, out any, auth bool) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return newError(resp, raw)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("agyproxy: %s %s: undecodable answer: %w", method, path, err)
	}
	return nil
}

func newError(resp *http.Response, raw []byte) *Error {
	e := &Error{Status: resp.StatusCode}
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s >= 0 {
		e.RetryAfter = time.Duration(s) * time.Second
	}
	// `detail` is a string for most errors and an object carrying run_id on 502.
	var body struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(raw, &body) == nil && len(body.Detail) > 0 {
		var text string
		if json.Unmarshal(body.Detail, &text) == nil {
			e.Detail = text
			return e
		}
		var obj struct {
			RunID   string `json:"run_id"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if json.Unmarshal(body.Detail, &obj) == nil {
			e.RunID = obj.RunID
			e.Detail = obj.Message
			if e.Detail == "" {
				e.Detail = obj.Error
			}
		}
		if e.Detail == "" {
			e.Detail = string(body.Detail)
		}
		return e
	}
	e.Detail = strings.TrimSpace(string(raw))
	if len(e.Detail) > 500 {
		e.Detail = e.Detail[:500]
	}
	return e
}
