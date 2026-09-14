// Package tap records which requests reach the candidate's Python.
//
// It sits between the candidate front door and its Python API. A step the
// front door answered in Go never passes through it, so its record says, step
// by step, whether Python served the answer. The harness reads the record on a
// separate control listener: nothing about the tap is visible on the wire being
// compared, and it knows nothing about the front door beyond being in front.
package tap

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Entry is one request that reached Python.
type Entry struct {
	Seq    int    `json:"seq"`
	Method string `json:"method"`
	Target string `json:"target"`
}

// Recorder holds every request forwarded so far.
type Recorder struct {
	mu      sync.Mutex
	entries []Entry
}

// forwardedHeaders are what the front door set for Python. ReverseProxy with
// Rewrite removes them from the outbound request, so they are copied back.
var forwardedHeaders = []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"}

// Proxy forwards every request to upstream as received and records it first.
// Keep-alive is off upstream: uvicorn closes a connection after an unhandled
// exception without saying so, and a reused connection would turn the next
// request into a 502 that Python never sent.
func (rec *Recorder) Proxy(upstream *url.URL) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.Out.Host = r.In.Host
			for _, name := range forwardedHeaders {
				if values, ok := r.In.Header[name]; ok {
					r.Out.Header[name] = values
				}
			}
		},
		Transport: &http.Transport{
			Proxy:              nil,
			DialContext:        (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			DisableCompression: true,
			DisableKeepAlives:  true,
		},
		FlushInterval: -1,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.entries = append(rec.entries, Entry{Seq: len(rec.entries) + 1, Method: r.Method, Target: r.RequestURI})
		rec.mu.Unlock()
		proxy.ServeHTTP(w, r)
	})
}

type page struct {
	Last    int     `json:"last"`
	Entries []Entry `json:"entries"`
}

// Control answers GET /entries?after=N with the entries whose sequence number
// is above N and the last sequence number recorded.
func (rec *Recorder) Control() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/entries" {
			http.NotFound(w, r)
			return
		}
		after, err := strconv.Atoi(r.URL.Query().Get("after"))
		if err != nil || after < 0 {
			http.Error(w, "after must be a non-negative integer", http.StatusBadRequest)
			return
		}
		rec.mu.Lock()
		out := page{Last: len(rec.entries), Entries: []Entry{}}
		if after < len(rec.entries) {
			out.Entries = append(out.Entries, rec.entries[after:]...)
		}
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}

// Client reads a tap's control listener.
type Client struct {
	base string
	http *http.Client
}

// NewClient returns a client for the control listener at base.
func NewClient(base string) (*Client, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("tap: control URL %q must be scheme://host:port", base)
	}
	return &Client{base: strings.TrimSuffix(base, "/"), http: &http.Client{Timeout: 5 * time.Second}}, nil
}

// Since returns the last sequence number and the entries after the given one.
func (c *Client) Since(ctx context.Context, after int) (int, []Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/entries?after="+strconv.Itoa(after), nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("tap: control answered %d", resp.StatusCode)
	}
	var out page
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, nil, err
	}
	return out.Last, out.Entries, nil
}

// Last returns the last sequence number recorded so far.
func (c *Client) Last(ctx context.Context) (int, error) {
	last, _, err := c.Since(ctx, 1<<30)
	return last, err
}
