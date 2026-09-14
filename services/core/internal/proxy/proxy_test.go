package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type seen struct {
	host, rawPath, rawQuery, xff, forwarded, idem string
	body                                          []byte
}

func newPair(t *testing.T, upstreamHandler func(http.ResponseWriter, *http.Request, *seen)) (*httptest.Server, *seen) {
	t.Helper()
	got := &seen{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.host = r.Host
		got.rawPath = r.URL.EscapedPath()
		got.rawQuery = r.URL.RawQuery
		got.xff = r.Header.Get("X-Forwarded-For")
		got.forwarded = r.Header.Get("Forwarded")
		got.idem = r.Header.Get("Idempotency-Key")
		got.body, _ = io.ReadAll(r.Body)
		upstreamHandler(w, r, got)
	}))
	t.Cleanup(upstream.Close)
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(New(target, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(front.Close)
	return front, got
}

// rawClient neither decompresses nor follows redirects, like the parity harness.
func rawClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DisableCompression: true, Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestRequestReachesPythonUnchangedExceptCallerAddress(t *testing.T) {
	front, got := newPair(t, func(w http.ResponseWriter, _ *http.Request, _ *seen) {
		w.WriteHeader(http.StatusCreated)
	})
	body := []byte(`{"display_name":"Team Đà Lạt"}`)
	req, _ := http.NewRequest(http.MethodPost, front.URL+"/contexts/a%2Fb//c?limit=1&limit=2", bytes.NewReader(body))
	req.Host = "parity.test"
	req.Header.Set("Idempotency-Key", "ctx-1")
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	req.Header.Set("Forwarded", "for=6.6.6.6")
	resp, err := rawClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got.host != "parity.test" {
		t.Errorf("Host = %q, want the client's", got.host)
	}
	if got.rawPath != "/contexts/a%2Fb//c" {
		t.Errorf("path = %q, want encoded slash and double slash kept", got.rawPath)
	}
	if got.rawQuery != "limit=1&limit=2" {
		t.Errorf("query = %q", got.rawQuery)
	}
	if got.xff != "127.0.0.1" {
		t.Errorf("X-Forwarded-For = %q, want only the socket peer", got.xff)
	}
	if got.forwarded != "" {
		t.Errorf("Forwarded = %q, want stripped", got.forwarded)
	}
	if got.idem != "ctx-1" {
		t.Errorf("Idempotency-Key = %q", got.idem)
	}
	if !bytes.Equal(got.body, body) {
		t.Errorf("body = %q", got.body)
	}
}

func TestResponseReachesClientUnchanged(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(`{"status":"ok"}`))
	_ = zw.Close()
	compressed := gz.Bytes()

	front, _ := newPair(t, func(w http.ResponseWriter, _ *http.Request, _ *seen) {
		h := w.Header()
		h["date"] = []string{"Mon, 14 Sep 2026 10:00:00 GMT"}
		h["server"] = []string{"uvicorn"}
		h.Set("Content-Type", "application/json")
		h.Set("Content-Encoding", "gzip")
		h.Set("Idempotency-Replayed", "true")
		h.Add("Vary", "Origin")
		h.Add("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(compressed)
	})
	req, _ := http.NewRequest(http.MethodGet, front.URL+"/healthz", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := rawClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	gotBody, _ := io.ReadAll(resp.Body)

	if !bytes.Equal(gotBody, compressed) {
		t.Errorf("body was altered (decompressed?): %q", gotBody)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %q", resp.Header.Get("Content-Encoding"))
	}
	if resp.Header.Get("Date") != "Mon, 14 Sep 2026 10:00:00 GMT" {
		t.Errorf("Date = %q, want Python's", resp.Header.Get("Date"))
	}
	if resp.Header.Get("Server") != "uvicorn" {
		t.Errorf("Server = %q", resp.Header.Get("Server"))
	}
	if got := resp.Header.Values("Vary"); len(got) != 2 {
		t.Errorf("Vary = %q, want both values, not merged or dropped", got)
	}
}

func TestRedirectIsPassedThroughNotFollowed(t *testing.T) {
	front, _ := newPair(t, func(w http.ResponseWriter, r *http.Request, _ *seen) {
		w.Header().Set("Location", "http://"+r.Host+"/contexts")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	req, _ := http.NewRequest(http.MethodGet, front.URL+"/contexts/", nil)
	req.Host = "parity.test"
	resp, err := rawClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "http://parity.test/contexts" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestUpstreamDownIsBadGateway(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:1")
	front := httptest.NewServer(New(target, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer front.Close()
	resp, err := rawClient().Get(front.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLargeStreamedBodyArrivesWhole(t *testing.T) {
	front, got := newPair(t, func(w http.ResponseWriter, _ *http.Request, _ *seen) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	})
	payload := strings.Repeat("x", 10*1024*1024+1)
	resp, err := rawClient().Post(front.URL+"/people/me/photos", "multipart/form-data; boundary=b", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(got.body) != len(payload) {
		t.Fatalf("upstream got %d bytes, want %d", len(got.body), len(payload))
	}
}
