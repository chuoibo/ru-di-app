// Package brain is the Go client for the Python brain seam (ADR-0029 §2.7).
// Auth, the database and the limiter stay here. The model step is a POST
// under /internal/brain/v1, gated by X-Internal-Token, and is never reached
// through the public front door.
package brain

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"mobile/services/core/internal/pyjson"
)

const (
	envBrainURL       = "MOBILE_BRAIN_URL"
	envPythonUpstream = "MOBILE_PYTHON_UPSTREAM"
	envToken          = "MOBILE_INTERNAL_TOKEN"
	tokenHeader       = "X-Internal-Token"
	timeout           = 60 * time.Second
	maxResponseBytes  = 2 << 20
)

var (
	errRedirect = errors.New("brain: redirect refused")
	errConfig   = errors.New("brain: invalid configuration")
)

// Error is a closed brain refusal: HTTP status plus a code, nothing else.
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return e.Code }

// Client posts JSON to the brain. A nil *Client is Configured() finding no
// URL or no token, which is an answer of its own.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// Configured reads MOBILE_BRAIN_URL (else MOBILE_PYTHON_UPSTREAM) and
// MOBILE_INTERNAL_TOKEN on every call, the way routing.Configured rereads
// Valhalla. A missing or invalid pair is nil, not a process crash: public
// Python still owns the route until LIVE-GO.
func Configured() *Client {
	base := os.Getenv(envBrainURL)
	if base == "" {
		base = os.Getenv(envPythonUpstream)
	}
	token := strings.TrimSpace(os.Getenv(envToken))
	if base == "" || token == "" {
		return nil
	}
	client, err := newClient(base, token)
	if err != nil {
		return nil
	}
	return client
}

func newClient(baseURL, token string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, errConfig
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errConfig
	}
	if parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errConfig
	}
	transport := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) { return nil, nil }}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errRedirect
			},
		},
	}, nil
}

// PostJSON posts body to /internal/brain/v1/{action}. A non-2xx answer with
// {"code":...} is *Error; anything else is a transport or decode failure.
func (c *Client) PostJSON(action string, body pyjson.Value) (pyjson.Value, error) {
	return c.PostJSONContext(context.Background(), action, body)
}

// PostJSONContext lets the job worker cancel an inference without leaking its input.
func (c *Client) PostJSONContext(ctx context.Context, action string, body pyjson.Value) (pyjson.Value, error) {
	if c == nil {
		return nil, &Error{Status: 502, Code: "brain_unavailable"}
	}
	encoded, err := pyjson.Dumps(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/brain/v1/"+action, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tokenHeader, c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Status: 502, Code: "brain_unavailable"}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, &Error{Status: 502, Code: "brain_unavailable"}
	}
	if len(raw) > maxResponseBytes {
		return nil, &Error{Status: 502, Code: "brain_unavailable"}
	}
	if len(raw) == 0 {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return pyjson.Null{}, nil
		}
		return nil, &Error{Status: resp.StatusCode, Code: "brain_unavailable"}
	}
	value, err := pyjson.Loads(raw)
	if err != nil {
		return nil, &Error{Status: 502, Code: "brain_unavailable"}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return value, nil
	}
	code := "brain_unavailable"
	if obj, ok := value.(*pyjson.OrderedMap); ok {
		if v, found := obj.Get("code"); found {
			if s, ok := v.(pyjson.String); ok && string(s) != "" {
				code = string(s)
			}
		}
	}
	return nil, &Error{Status: resp.StatusCode, Code: code}
}

// ImageBody is the JSON the brain's image routes accept.
func ImageBody(content []byte, contentType string) *pyjson.OrderedMap {
	body := pyjson.NewOrderedMap()
	body.Set("image", pyjson.String(base64.StdEncoding.EncodeToString(content)))
	body.Set("content_type", pyjson.String(contentType))
	return body
}

// AsObject is the JSON object a successful brain call returned.
func AsObject(value pyjson.Value) (*pyjson.OrderedMap, error) {
	obj, ok := value.(*pyjson.OrderedMap)
	if !ok {
		return nil, fmt.Errorf("brain: answer is %T, not an object", value)
	}
	return obj, nil
}
