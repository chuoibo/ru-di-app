package httpclient

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

// dropsAfterAnswering answers one request per connection, then holds the
// connection without reading and closes it: uvicorn after an unhandled 500.
func dropsAfterAnswering(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, req.Body)
				_, _ = io.WriteString(conn, "HTTP/1.1 500 Internal Server Error\r\n"+
					"Content-Type: text/plain; charset=utf-8\r\nContent-Length: 21\r\n\r\nInternal Server Error")
				time.Sleep(300 * time.Millisecond)
			}(conn)
		}
	}()
	return "http://" + listener.Addr().String()
}

func TestAConnectionTheServerDropsIsNeverReused(t *testing.T) {
	base := dropsAfterAnswering(t)
	put := Request{Method: "PUT", Path: "/people/x", Body: []byte(`{"display_name":"a"}`)}

	// The reproduction: with keep-alive the second PUT lands on the connection
	// the server is dropping, and Go reports EOF.
	reusing, _ := New(base, "parity.test")
	reusing.http.Transport = newTransport(true)
	if _, err := reusing.Do(context.Background(), put); err != nil {
		t.Fatal(err)
	}
	if _, err := reusing.Do(context.Background(), put); err == nil {
		t.Fatal("a keep-alive client survived the dropped connection; this test no longer reproduces the failure")
	}

	client, _ := New(base, "parity.test")
	for i := 0; i < 3; i++ {
		resp, err := client.Do(context.Background(), put)
		if err != nil || resp.Status != 500 {
			t.Fatalf("request %d: status %d, err %v", i, resp.Status, err)
		}
	}
}
