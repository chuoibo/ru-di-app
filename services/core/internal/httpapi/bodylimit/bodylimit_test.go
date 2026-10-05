package bodylimit

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const tooLarge = `{"code": "request_body_too_large", "detail": "The request body is larger than this endpoint accepts."}`

func echo(seen *[]byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*seen = b
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestADeclaredOversizeBodyIsRefusedUnread(t *testing.T) {
	var seen []byte
	h := Wrap(8, time.Second, echo(&seen))
	r := httptest.NewRequest(http.MethodPost, "/expenses", strings.NewReader("abcdefghi"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge || w.Body.String() != tooLarge {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Content-Length") != strconv.Itoa(len(tooLarge)) {
		t.Fatalf("headers %v", w.Header())
	}
	if seen != nil {
		t.Fatal("the route ran")
	}
}

func TestAnUndeclaredOversizeBodyIsRefusedAtTheCap(t *testing.T) {
	var seen []byte
	h := Wrap(8, time.Second, echo(&seen))
	r := httptest.NewRequest(http.MethodPost, "/expenses", io.MultiReader(strings.NewReader("12345"), strings.NewReader("6789")))
	r.ContentLength = -1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge || seen != nil {
		t.Fatalf("%d, route saw %q", w.Code, seen)
	}
}

func TestABodyAtTheCapReachesTheRouteWhole(t *testing.T) {
	var seen []byte
	h := Wrap(8, time.Second, echo(&seen))
	r := httptest.NewRequest(http.MethodPost, "/expenses", strings.NewReader("12345678"))
	r.ContentLength = -1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || string(seen) != "12345678" {
		t.Fatalf("%d %q", w.Code, seen)
	}
}

func TestUploadRoutesTakeAnImageOneByteOverTheSanitizerCap(t *testing.T) {
	if For("POST /people/me/photos") <= 10*1024*1024+1 {
		t.Fatal("an image one byte too large must still reach the route for its own answer")
	}
	if For("POST /expenses") != DefaultBytes || DefaultBytes != 1<<20 {
		t.Fatal("a JSON route keeps the 1 MiB cap")
	}
}

// A real connection that sends half a body and stalls is cut once the budget
// is spent (net/http closes a connection whose read timed out, so no answer
// can go out on it), and the route never runs.
func TestASlowBodyRunsOutOfTime(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(Wrap(1<<20, 200*time.Millisecond, echo(&seen)))
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("POST /expenses HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n{\"half\":"))
	started := time.Now()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var buf bytes.Buffer
	_, err = io.Copy(&buf, conn)
	if err != nil || time.Since(started) > 2*time.Second {
		t.Fatalf("connection held for %s (%v)", time.Since(started), err)
	}
	if strings.HasPrefix(buf.String(), "HTTP/1.1 2") || seen != nil {
		t.Fatalf("the route ran: %q", buf.String())
	}
}
