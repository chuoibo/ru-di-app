package canary

import (
	"bufio"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func upstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://"+r.Host+"/there")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Vary", "Origin")
		cursor := base64.RawURLEncoding.EncodeToString([]byte("2026-09-14T10:00:00.120000+00:00|aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"))
		_, _ = w.Write([]byte(`{"id":"a","score":1.0,"created_at":"2026-09-14T10:00:00.123456Z","cursor":"` + cursor + `","path":"/g/` + strings.Repeat("Ab3_", 11)[:43] + `"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func fetch(t *testing.T, base, path string) (*http.Response, string) {
	t.Helper()
	client := &http.Client{
		Transport:     &http.Transport{DisableCompression: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

// headerValues counts names and values, so a duplicated value shows.
func headerValues(h http.Header) int {
	n := 0
	for _, values := range h {
		n += 1 + len(values)
	}
	return n
}

func TestEveryModeChangesWhatItClaims(t *testing.T) {
	target, _ := url.Parse(upstream(t).URL)
	baseline, baseBody := fetch(t, target.String(), "/json")

	for _, mode := range Modes() {
		t.Run(mode.Name, func(t *testing.T) {
			var applied atomic.Int64
			front := httptest.NewServer(Proxy(target, mode, &applied))
			defer front.Close()
			path := "/json"
			if mode.Name == "redirect-followed" {
				path = "/redirect"
			}
			resp, body := fetch(t, front.URL, path)
			changed := resp.StatusCode != baseline.StatusCode || body != baseBody ||
				headerValues(resp.Header) != headerValues(baseline.Header) ||
				resp.Header.Get("Content-Encoding") != ""
			if mode.Name == "redirect-followed" {
				changed = resp.StatusCode == http.StatusOK && resp.Header.Get("Location") == ""
			}
			if mode.Name == "identity" {
				if changed || applied.Load() != 0 {
					t.Fatalf("identity changed the response: %d %q", resp.StatusCode, body)
				}
				return
			}
			if !changed || applied.Load() == 0 {
				t.Fatalf("mode did not damage the response (applied=%d): %d %q", applied.Load(), resp.StatusCode, body)
			}
		})
	}
}

// crashingUpstream answers 500 and then drops the connection, as uvicorn does
// after an unhandled exception, without a Connection: close header.
func crashingUpstream(t *testing.T) *url.URL {
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
	target, _ := url.Parse("http://" + listener.Addr().String())
	return target
}

func postStatus(t *testing.T, base string) int {
	t.Helper()
	resp, err := http.Post(base+"/reports", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestAPythonCrashNeverTurnsTheNextIdentityRequestInto502(t *testing.T) {
	target := crashingUpstream(t)
	identity := Modes()[0]
	if identity.Name != "identity" {
		t.Fatalf("Modes()[0] = %q, want identity", identity.Name)
	}
	var applied atomic.Int64

	reusing := Proxy(target, identity, &applied).(*httputil.ReverseProxy)
	reusing.Transport = newTransport(true)
	withKeepAlive := httptest.NewServer(reusing)
	defer withKeepAlive.Close()
	if first, second := postStatus(t, withKeepAlive.URL), postStatus(t, withKeepAlive.URL); first != 500 || second != http.StatusBadGateway {
		t.Fatalf("reproduction changed: got %d then %d, expected 500 then 502", first, second)
	}

	fixed := httptest.NewServer(Proxy(target, identity, &applied))
	defer fixed.Close()
	for i := 0; i < 3; i++ {
		if code := postStatus(t, fixed.URL); code != 500 {
			t.Fatalf("request %d through identity: %d, want Python's 500", i, code)
		}
	}
}
