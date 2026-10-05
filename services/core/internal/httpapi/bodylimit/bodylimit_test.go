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

func TestADeclaredOversizeBodyIsRefusedBeforeTheRoute(t *testing.T) {
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

// countingReader counts what the layer read of a body.
type countingReader struct {
	r    io.Reader
	read int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	return n, err
}

// A refused body is read to its end before the 413, as Python's _discard
// does: answering mid-upload made net/http close the connection under a
// client still sending, which saw a reset instead of the answer (parity
// w0/body-limit, 2026-10-05). Beyond DiscardBytes it stops reading.
func TestARefusedBodyIsDrainedBeforeTheAnswerUpToABound(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int64
		declared bool
		want     int64
	}{
		{"declared", 3 << 20, true, 3 << 20},
		{"undeclared", 3 << 20, false, 3 << 20},
		{"past the bound", DiscardBytes + 5<<20, true, DiscardBytes},
	} {
		body := &countingReader{r: io.LimitReader(zeros{}, tc.size)}
		r := httptest.NewRequest(http.MethodPost, "/expenses", body)
		r.ContentLength = -1
		if tc.declared {
			r.ContentLength = tc.size
		}
		w := httptest.NewRecorder()
		Wrap(1<<20, time.Second, http.NotFoundHandler()).ServeHTTP(w, r)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s: %d", tc.name, w.Code)
		}
		if body.read != tc.want {
			t.Fatalf("%s: read %d of %d before answering, want %d", tc.name, body.read, tc.size, tc.want)
		}
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// Over a real connection, a client sending 8 MiB to a 1 MiB route gets the
// 413, not a write error.
func TestAClientStillSendingGetsThe413(t *testing.T) {
	server := httptest.NewServer(Wrap(1<<20, 10*time.Second, http.NotFoundHandler()))
	defer server.Close()
	for i := 0; i < 5; i++ {
		resp, err := http.Post(server.URL+"/expenses", "application/json", io.LimitReader(zeros{}, 8<<20))
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge || string(got) != tooLarge {
			t.Fatalf("attempt %d: %d %q", i, resp.StatusCode, got)
		}
	}
}
