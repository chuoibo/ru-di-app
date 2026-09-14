package httpclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIsSentAsWritten(t *testing.T) {
	var gotHost, gotPath, gotQuery, gotUA, gotAE string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotPath, gotQuery = r.Host, r.URL.EscapedPath(), r.URL.RawQuery
		gotUA, gotAE = r.UserAgent(), r.Header.Get("Accept-Encoding")
	}))
	defer server.Close()

	client, err := New(server.URL, "parity.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), Request{Method: "GET", Path: "/contexts/a%2Fb//c?limit=1&limit=2"}); err != nil {
		t.Fatal(err)
	}
	if gotHost != "parity.test" || gotPath != "/contexts/a%2Fb//c" || gotQuery != "limit=1&limit=2" {
		t.Fatalf("host=%q path=%q query=%q", gotHost, gotPath, gotQuery)
	}
	if gotUA != UserAgent || gotAE != "" {
		t.Fatalf("user-agent=%q accept-encoding=%q", gotUA, gotAE)
	}
}

func TestResponseIsReturnedAsSent(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(`{"a":1}`))
	_ = zw.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://"+r.Host+"/elsewhere")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz.Bytes())
	}))
	defer server.Close()
	client, _ := New(server.URL, "parity.test")

	resp, err := client.Do(context.Background(), Request{Method: "GET", Path: "/gz", Header: http.Header{"Accept-Encoding": {"gzip"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(resp.Body, gz.Bytes()) || resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("body or encoding altered: %q %q", resp.Body, resp.Header.Get("Content-Encoding"))
	}
	resp, err = client.Do(context.Background(), Request{Method: "GET", Path: "/redirect"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusTemporaryRedirect || resp.Header.Get("Location") != "http://parity.test/elsewhere" {
		t.Fatalf("redirect followed or rewritten: %d %q", resp.Status, resp.Header.Get("Location"))
	}
}

func TestNewRejectsBaseWithPath(t *testing.T) {
	if _, err := New("http://127.0.0.1:1/api", ""); err == nil {
		t.Fatal("base with path accepted")
	}
}
