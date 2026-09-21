// Package proxy forwards every request the Go core does not own to the Python
// API. The forwarding must be invisible: status, headers and body bytes reach
// the client exactly as Python wrote them, and Python sees the request exactly
// as the client sent it, apart from the caller's address.
package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// New returns a handler that forwards to upstream, which must be a bare origin.
func New(upstream *url.URL, logger *slog.Logger) http.Handler {
	transport := newTransport(false)
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			// SetURL points Host at the upstream. Starlette builds absolute
			// 307 Locations from Host, so the client's value must survive.
			r.Out.Host = r.In.Host
			// Rewrite has already dropped inbound X-Forwarded-*. Forwarded is
			// not covered by that, and no header a client sends may choose the
			// address Python's per-IP limiters count against.
			r.Out.Header.Del("Forwarded")
			// X-Forwarded-For becomes exactly the socket peer.
			r.SetXForwarded()
		},
		Transport: transport,
		// Stream as bytes arrive, so a slow body is never held in memory.
		FlushInterval: -1,
		ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			if errors.Is(err, context.Canceled) {
				// The client went away; there is nobody to answer.
				return
			}
			logger.Error("python upstream failed",
				"method", req.Method, "path", req.URL.Path, "error", err.Error())
			w.WriteHeader(http.StatusBadGateway)
		},
	}
}

// newTransport builds the upstream transport. Keep-alive to Python is off:
// uvicorn closes a connection right after answering an unhandled exception
// with 500, without a Connection: close header. A pooled connection would take
// the next request, Go would not retry a POST or PUT, and the client would get
// a 502 that Python never sent. One TCP connect per proxied request on the
// private network is the price; the client's own connection to core stays
// kept alive. keepAlive exists so a test can show the failure.
func newTransport(keepAlive bool) *http.Transport {
	return &http.Transport{
		// The upstream lives on the private backend network; an HTTP_PROXY in
		// the environment must never reroute it.
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// With compression enabled the transport adds its own Accept-Encoding
		// and transparently gunzips, deleting Content-Encoding and
		// Content-Length on the way. The client's header must pass through
		// untouched instead.
		DisableCompression: true,
		DisableKeepAlives:  !keepAlive,
		// No response header timeout: model-backed routes legitimately take
		// tens of seconds, and Python owns those deadlines.
	}
}
