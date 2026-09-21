// Package httpclient sends a request exactly as a scenario wrote it and
// returns the response exactly as the server sent it.
//
// Go's default client is helpful in ways that hide differences: it gunzips and
// drops Content-Encoding, follows redirects, adds User-Agent and cleans nothing
// but might re-encode paths. Every one of those is turned off here.
package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// UserAgent is sent unless a scenario sets its own.
const UserAgent = "parity/1"

// Request is one scenario step's request after template rendering.
type Request struct {
	Method string
	Path   string // path and query, exactly as sent
	Header http.Header
	Body   []byte // nil means no body
}

// Response is what came back, unmodified.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Client talks to one stack.
type Client struct {
	base string
	host string
	http *http.Client
}

// New returns a client for base (scheme://host:port) that sends host as the
// Host header, so absolute redirects are comparable across stacks.
func New(base, host string) (*Client, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("httpclient: base %q must be scheme://host:port", base)
	}
	return &Client{
		base: strings.TrimSuffix(base, "/"),
		host: host,
		http: &http.Client{
			Transport: newTransport(false),
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: 90 * time.Second,
		},
	}, nil
}

// Do sends req and reads the whole response.
func (c *Client) Do(ctx context.Context, req Request) (Response, error) {
	if !strings.HasPrefix(req.Path, "/") {
		return Response{}, fmt.Errorf("httpclient: path %q must start with /", req.Path)
	}
	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, c.base+req.Path, body)
	if err != nil {
		return Response{}, fmt.Errorf("httpclient: %w", err)
	}
	for name, values := range req.Header {
		httpReq.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", UserAgent)
	}
	if c.host != "" {
		httpReq.Host = c.host
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("httpclient: %s %s: %w", req.Method, req.Path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("httpclient: reading %s %s: %w", req.Method, req.Path, err)
	}
	return Response{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: data}, nil
}

// newTransport builds the harness transport with keep-alive off. uvicorn closes
// a connection right after answering an unhandled exception with 500, without a
// Connection: close header (run_asgi calls transport.close()). A pooled client
// can write its next request onto that connection, and Go does not retry a PUT
// or POST, so the harness would report EOF for something that is not a
// difference between the stacks. keepAlive exists so a test can show that.
func newTransport(keepAlive bool) *http.Transport {
	return &http.Transport{
		Proxy:              nil,
		DisableCompression: true,
		ForceAttemptHTTP2:  false,
		DisableKeepAlives:  !keepAlive,
	}
}
