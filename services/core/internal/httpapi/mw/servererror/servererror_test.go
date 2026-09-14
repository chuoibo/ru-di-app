package servererror

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/httpapi/mw/cors"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func rawPath(r *http.Request) string { return r.URL.Path }

// dial opens one keep-alive connection to a server running h.
func dial(t *testing.T, h http.Handler) (net.Conn, *bufio.Reader) {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	return conn, bufio.NewReader(conn)
}

func send(t *testing.T, conn net.Conn, br *bufio.Reader, path string, extra string) (*http.Response, []byte, error) {
	t.Helper()
	if _, err := io.WriteString(conn, "GET "+path+" HTTP/1.1\r\nHost: core.test\r\n"+extra+"\r\n"); err != nil {
		return nil, nil, err
	}
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body, err
}

func TestACrashBeforeTheResponseAnswersStarlettesDefaultAndClosesTheConnection(t *testing.T) {
	conn, br := dial(t, Middleware(quiet(), rawPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Route", "set before the crash")
		Raise(errors.New("boom"))
	})))
	resp, body, err := send(t, conn, br, "/contexts", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 || string(body) != "Internal Server Error" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	want := map[string]string{"Content-Type": "text/plain; charset=utf-8", "Content-Length": "21"}
	for name, value := range want {
		if resp.Header.Get(name) != value {
			t.Fatalf("%s = %q, want %q", name, resp.Header.Get(name), value)
		}
	}
	for _, name := range []string{"X-Route", "Cache-Control", "Connection"} {
		if _, present := resp.Header[name]; present {
			t.Fatalf("%s present in %v", name, resp.Header)
		}
	}
	if _, _, err := send(t, conn, br, "/contexts", ""); err == nil {
		t.Fatal("connection still served a second request after the crash")
	}
}

func TestGuestPathsCarryPrivacyHeadersOnACrash(t *testing.T) {
	for path, guest := range map[string]bool{"/g": true, "/g/abc": true, "/goals": false, "/": false} {
		conn, br := dial(t, Middleware(quiet(), rawPath, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("not an error value")
		})))
		resp, _, err := send(t, conn, br, path, "")
		if err != nil {
			t.Fatal(path, err)
		}
		if resp.StatusCode != 500 {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		got := resp.Header.Get("Cache-Control") == "no-store" &&
			resp.Header.Get("Referrer-Policy") == "no-referrer" &&
			resp.Header.Get("X-Robots-Tag") == "noindex, nofollow"
		if got != guest || (!guest && len(resp.Header.Values("Cache-Control")) != 0) {
			t.Fatalf("%s: guest headers %v, want %v (%v)", path, got, guest, resp.Header)
		}
	}
}

func TestACrashAfterTheResponseStartedTruncatesAndCloses(t *testing.T) {
	conn, br := dial(t, Middleware(quiet(), rawPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, "ten bytes!")
		w.(http.Flusher).Flush()
		Raise(errors.New("late"))
	})))
	resp, body, err := send(t, conn, br, "/stream", "")
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("resp %v err %v", resp, err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) || string(body) != "ten bytes!" {
		t.Fatalf("body %q err %v, want the started body cut short", body, err)
	}
}

func TestAnAbortedHandlerIsNotAnsweredWith500(t *testing.T) {
	conn, br := dial(t, Middleware(quiet(), rawPath, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})))
	if resp, _, err := send(t, conn, br, "/x", ""); err == nil {
		t.Fatalf("got a response %d for an aborted handler", resp.StatusCode)
	}
}

func TestHealthyRequestsKeepTheConnection(t *testing.T) {
	conn, br := dial(t, Middleware(quiet(), rawPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})))
	for i := 0; i < 2; i++ {
		resp, body, err := send(t, conn, br, "/healthy", "")
		if err != nil || resp.StatusCode != 200 || string(body) != "ok" {
			t.Fatalf("request %d: %v %q %v", i, resp, body, err)
		}
	}
}

func TestTheCrashAnswerGoesOutFromAboveCORS(t *testing.T) {
	policy := cors.New("http://allowed.test", true)
	crashes := true
	conn, br := dial(t, Middleware(quiet(), rawPath, policy.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if crashes {
			Raise(errors.New("boom"))
		}
		_, _ = io.WriteString(w, "ok")
	}))))
	resp, _, err := send(t, conn, br, "/contexts", "Origin: http://allowed.test\r\n")
	if err != nil || resp.StatusCode != 500 {
		t.Fatalf("%v %v", resp, err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("crash answer carries allow-origin %q", got)
	}

	// Control: the same stack without the crash does add it, so the check
	// above is not passing because the origin was refused.
	crashes = false
	conn2, br2 := dial(t, Middleware(quiet(), rawPath, policy.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))))
	resp, _, err = send(t, conn2, br2, "/contexts", "Origin: http://allowed.test\r\n")
	if err != nil || !strings.Contains(resp.Header.Get("Access-Control-Allow-Origin"), "allowed.test") {
		t.Fatalf("control lost allow-origin: %v %v", resp, err)
	}
}
