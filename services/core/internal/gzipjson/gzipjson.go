// Package gzipjson compresses JSON responses for clients that ask for it.
//
// GET /places for TP.HCM is 3.4 MB of JSON, and phones on mobile data pay for
// every byte. Android's OkHttp and iOS's URLSession send `Accept-Encoding: gzip`
// on their own and inflate transparently, so the app needs no change.
//
// Only JSON, and only when asked: images are already compressed, streams must
// not be buffered by a compressor, and the parity harness turns compression
// off (DisableCompression), so what it compares is byte-for-byte unchanged.
package gzipjson

import (
	"bufio"
	"compress/gzip"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
)

var writers = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
	return w
}}

// Middleware wraps next.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || r.Header.Get("Upgrade") != "" ||
			!acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		cw := &writer{ResponseWriter: w}
		defer cw.finish()
		next.ServeHTTP(cw, r)
	})
}

func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(token), "gzip") {
			return !strings.Contains(strings.ReplaceAll(params, " ", ""), "q=0") ||
				strings.Contains(params, "q=0.")
		}
	}
	return false
}

type writer struct {
	http.ResponseWriter
	decided bool
	gz      *gzip.Writer
}

func (w *writer) decide(code int) {
	if w.decided {
		return
	}
	w.decided = true
	h := w.Header()
	h.Add("Vary", "Accept-Encoding")
	if code == http.StatusNoContent || code == http.StatusNotModified || code < 200 ||
		h.Get("Content-Encoding") != "" ||
		!strings.HasPrefix(strings.ToLower(h.Get("Content-Type")), "application/json") {
		return
	}
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	gz := writers.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)
	w.gz = gz
}

func (w *writer) WriteHeader(code int) {
	w.decide(code)
	w.ResponseWriter.WriteHeader(code)
}

func (w *writer) Write(p []byte) (int, error) {
	if !w.decided {
		w.WriteHeader(http.StatusOK)
	}
	if w.gz != nil {
		return w.gz.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

// Flush pushes what is compressed so far, for handlers that flush.
func (w *writer) Flush() {
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack keeps connection upgrades working through the wrapper.
func (w *writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("gzipjson: response does not support hijacking")
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *writer) finish() {
	if w.gz != nil {
		_ = w.gz.Close()
		w.gz.Reset(nil)
		writers.Put(w.gz)
		w.gz = nil
	}
}
