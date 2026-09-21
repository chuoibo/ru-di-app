//go:build e2e

// Package chate2e drives the Go front door over real HTTP and WebSocket.
//
// Every other Go test in this repository reaches the code through a package
// boundary: httptest servers, in-process handlers, fake repositories. Those
// prove a function. They cannot prove that a request which left a phone
// arrives at the same answer, because they never cross the wire, the router,
// the session middleware, the idempotency layer or the proxy decision.
//
// This package is the black box. It knows only a base URL, a bearer token and
// the wire shapes. It never imports internal packages and never touches the
// database, so a case that passes here passes for a real client too.
//
// Fail-closed on purpose: a missing stack is a failure, not a skip. A suite
// that quietly skips reads exactly like a suite that passed.
package chate2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// otpRetryWait mirrors the login limiter: a 429 during sign-in is the limiter
// doing its job, so the harness waits it out instead of disabling it.
const otpRetryWait = 61 * time.Second

// User is one synthetic account from the seeded stack. The token is a real
// bearer session; it is never logged.
type User struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	PersonID string `json:"personId"`
	Phone    string `json:"phone"`
	Token    string `json:"token"`
}

// Stack is the running system under test, as the seeder described it.
type Stack struct {
	APIURL  string   `json:"apiUrl"`
	Users   []User   `json:"users"`
	GroupID string   `json:"groupId"`
	DMIds   []string `json:"dmIds"`
}

// Load reads the seeded session fixture. Both the path and the stack must
// exist: this suite measures a live system or it fails.
func Load(t *testing.T) *Stack {
	t.Helper()
	path := os.Getenv("CHAT_E2E_SESSIONS")
	if path == "" {
		t.Fatal("CHAT_E2E_SESSIONS chưa được đặt: bộ E2E này đo stack thật, không tự bịa dữ liệu")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("không đọc được %s: %v", path, err)
	}
	var stack Stack
	if err := json.Unmarshal(raw, &stack); err != nil {
		t.Fatalf("fixture phiên hỏng: %v", err)
	}
	if stack.APIURL == "" || len(stack.Users) < 4 || stack.GroupID == "" {
		t.Fatalf("fixture thiếu: cần apiUrl, groupId và ít nhất 4 người (có %d)", len(stack.Users))
	}
	if !strings.HasPrefix(stack.APIURL, "http://127.0.0.1:") &&
		!strings.HasPrefix(stack.APIURL, "http://localhost:") {
		t.Fatalf("từ chối chạy ngoài loopback: %s", stack.APIURL)
	}
	return &stack
}

// Client is one signed-in device. Two clients for the same person model two
// devices; that distinction matters for read marks and stream capacity.
type Client struct {
	t     *testing.T
	base  string
	token string
	http  *http.Client
}

// As returns a client for the user at the given seed index.
func (s *Stack) As(t *testing.T, index int) *Client {
	t.Helper()
	if index < 0 || index >= len(s.Users) {
		t.Fatalf("không có người thứ %d trong fixture (%d người)", index, len(s.Users))
	}
	return &Client{
		t:     t,
		base:  s.APIURL,
		token: s.Users[index].Token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Anonymous returns a client with no bearer, for the cases that must be 401.
func (s *Stack) Anonymous(t *testing.T) *Client {
	t.Helper()
	return &Client{t: t, base: s.APIURL, http: &http.Client{Timeout: 30 * time.Second}}
}

// WithToken returns a client carrying an arbitrary bearer, for revocation and
// forgery cases.
func (s *Stack) WithToken(t *testing.T, token string) *Client {
	t.Helper()
	return &Client{t: t, base: s.APIURL, token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// Response is a decoded answer. Body stays available for the cases that care
// about the exact bytes rather than the decoded shape.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	JSON   map[string]any
}

// Str reads a string field, failing the case when it is absent -- a test that
// silently compares "" to "" proves nothing.
func (r Response) Str(t *testing.T, key string) string {
	t.Helper()
	value, ok := r.JSON[key]
	if !ok {
		t.Fatalf("thiếu trường %q trong %s", key, r.trim())
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("trường %q không phải chuỗi: %T", key, value)
	}
	return text
}

// Num reads a numeric field as an int64.
func (r Response) Num(t *testing.T, key string) int64 {
	t.Helper()
	value, ok := r.JSON[key]
	if !ok {
		t.Fatalf("thiếu trường %q trong %s", key, r.trim())
	}
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("trường %q không phải số: %T", key, value)
	}
	return int64(number)
}

// List reads an array field.
func (r Response) List(t *testing.T, key string) []any {
	t.Helper()
	value, ok := r.JSON[key]
	if !ok {
		t.Fatalf("thiếu trường %q trong %s", key, r.trim())
	}
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("trường %q không phải mảng: %T", key, value)
	}
	return items
}

func (r Response) trim() string {
	if len(r.Body) > 400 {
		return string(r.Body[:400]) + "…"
	}
	return string(r.Body)
}

type options struct {
	idempotencyKey string
	headers        map[string]string
}

// Option adjusts one request.
type Option func(*options)

// Idem attaches an Idempotency-Key. Reusing the same key is how the replay
// cases are written, so the key is explicit rather than generated per call.
func Idem(key string) Option {
	return func(o *options) { o.idempotencyKey = key }
}

// Header sets one extra request header.
func Header(name, value string) Option {
	return func(o *options) {
		if o.headers == nil {
			o.headers = map[string]string{}
		}
		o.headers[name] = value
	}
}

// Do sends one request and decodes the answer. It never fails the test on a
// non-2xx: the status is the thing under test in most cases here.
func (c *Client) Do(method, path string, body any, opts ...Option) Response {
	c.t.Helper()
	settings := options{}
	for _, apply := range opts {
		apply(&settings)
	}
	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				c.t.Fatalf("không mã hoá được thân yêu cầu: %v", err)
			}
			reader = bytes.NewReader(encoded)
		}
		request, err := http.NewRequest(method, c.base+path, reader)
		if err != nil {
			c.t.Fatalf("yêu cầu hỏng: %v", err)
		}
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		if c.token != "" {
			request.Header.Set("Authorization", "Bearer "+c.token)
		}
		if settings.idempotencyKey != "" {
			request.Header.Set("Idempotency-Key", settings.idempotencyKey)
		}
		for name, value := range settings.headers {
			request.Header.Set(name, value)
		}
		response, err := c.http.Do(request)
		if err != nil {
			c.t.Fatalf("%s %s không gửi được: %v", method, path, err)
		}
		payload, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			c.t.Fatalf("%s %s không đọc được thân: %v", method, path, err)
		}
		// The login limiter is part of the product. Wait it out rather than
		// turning it off, which would make every auth case meaningless.
		if response.StatusCode == http.StatusTooManyRequests && attempt < 2 &&
			strings.Contains(path, "/auth/") {
			time.Sleep(otpRetryWait)
			continue
		}
		decoded := Response{Status: response.StatusCode, Header: response.Header, Body: payload}
		if len(payload) > 0 && strings.HasPrefix(strings.TrimSpace(string(payload)), "{") {
			_ = json.Unmarshal(payload, &decoded.JSON)
		}
		return decoded
	}
}

// Expect is Do plus a status assertion, for the steps that are setup rather
// than the thing being measured.
func (c *Client) Expect(status int, method, path string, body any, opts ...Option) Response {
	c.t.Helper()
	response := c.Do(method, path, body, opts...)
	if response.Status != status {
		c.t.Fatalf("%s %s: mong %d, nhận %d — %s", method, path, status, response.Status, response.trim())
	}
	return response
}

// Page is one change-feed page, on HTTP and on the socket alike.
type Page struct {
	ContextID    string   `json:"context_id"`
	Changes      []Change `json:"changes"`
	NextSequence int64    `json:"next_sequence"`
	Watermark    int64    `json:"watermark"`
	HasMore      bool     `json:"has_more"`
}

// Change is one pointer in the feed. The feed carries pointers, not content:
// a client learns that entity X moved to revision N and then hydrates it
// through the snapshot route. That is what keeps a revoked reader from being
// handed content by the socket itself.
type Change struct {
	Sequence int64  `json:"sequence"`
	Type     string `json:"type"`
	EntityID string `json:"entity_id"`
	Revision int64  `json:"revision"`
}

// Drain reads the change feed over HTTP until it is caught up and returns the
// head cursor plus everything seen after the given point.
func (c *Client) Drain(contextID string, after int64) (int64, []Change) {
	c.t.Helper()
	seen := []Change{}
	cursor := after
	for step := 0; step < 100; step++ {
		response := c.Expect(200, "GET",
			fmt.Sprintf("/contexts/%s/changes?after=%d&limit=100", contextID, cursor), nil)
		var page Page
		if err := json.Unmarshal(response.Body, &page); err != nil {
			c.t.Fatalf("trang thay đổi hỏng: %v", err)
		}
		seen = append(seen, page.Changes...)
		cursor = page.NextSequence
		if !page.HasMore {
			return cursor, seen
		}
	}
	c.t.Fatal("feed thay đổi không bao giờ bắt kịp sau 100 trang")
	return 0, nil
}

// Head returns the current feed cursor without collecting history.
func (c *Client) Head(contextID string) int64 {
	c.t.Helper()
	cursor, _ := c.Drain(contextID, 0)
	return cursor
}

// Stream is a live change socket. It speaks the server's contract exactly:
// authenticate, then acknowledge every page with the sequence the server
// named. An ack that does not match closes the connection, so the harness
// cannot accidentally pass by ignoring the protocol.
type Stream struct {
	t      *testing.T
	conn   *websocket.Conn
	cancel context.CancelFunc
	pages  chan Page
	closed chan struct{}
	errs   chan error
	once   sync.Once
}

// Stream opens the change socket positioned after the given sequence.
func (c *Client) Stream(contextID string, after int64) *Stream {
	c.t.Helper()
	target := strings.Replace(c.base, "http://", "ws://", 1)
	target = fmt.Sprintf("%s/contexts/%s/changes/stream?after=%d", target, contextID, after)
	ctx, cancel := context.WithCancel(context.Background())
	conn, _, err := websocket.Dial(ctx, target, nil)
	if err != nil {
		cancel()
		c.t.Fatalf("không mở được socket thay đổi: %v", err)
	}
	conn.SetReadLimit(1 << 20)
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "authenticate", "token": c.token}); err != nil {
		cancel()
		c.t.Fatalf("không gửi được khung xác thực: %v", err)
	}
	stream := &Stream{
		t: c.t, conn: conn, cancel: cancel,
		pages: make(chan Page, 64), closed: make(chan struct{}), errs: make(chan error, 1),
	}
	go stream.pump(ctx)
	return stream
}

func (s *Stream) pump(ctx context.Context) {
	defer close(s.closed)
	for {
		var page Page
		if err := wsjson.Read(ctx, s.conn, &page); err != nil {
			select {
			case s.errs <- err:
			default:
			}
			return
		}
		// Acknowledging exactly what the server named is mandatory; a wrong
		// sequence is a policy violation and the server hangs up.
		if err := wsjson.Write(ctx, s.conn, map[string]any{
			"type": "ack", "sequence": page.NextSequence,
		}); err != nil {
			select {
			case s.errs <- err:
			default:
			}
			return
		}
		select {
		case s.pages <- page:
		case <-ctx.Done():
			return
		}
	}
}

// Await waits for a page that satisfies the predicate, accumulating every
// change the socket delivered along the way.
func (s *Stream) Await(timeout time.Duration, satisfied func([]Change) bool) []Change {
	s.t.Helper()
	deadline := time.After(timeout)
	collected := []Change{}
	for {
		if satisfied(collected) {
			return collected
		}
		select {
		case page := <-s.pages:
			collected = append(collected, page.Changes...)
		case err := <-s.errs:
			s.t.Fatalf("socket đóng khi đang chờ: %v (đã nhận %d thay đổi)", err, len(collected))
		case <-deadline:
			if satisfied(collected) {
				return collected
			}
			s.t.Fatalf("hết %s mà điều kiện chưa đạt; đã nhận %d thay đổi", timeout, len(collected))
		}
	}
}

// AwaitClose asserts the server hung up, which is the expected outcome for a
// revoked session rather than a silent stall.
func (s *Stream) AwaitClose(timeout time.Duration) error {
	s.t.Helper()
	select {
	case err := <-s.errs:
		return err
	case <-time.After(timeout):
		s.t.Fatalf("socket vẫn mở sau %s, lẽ ra phải bị đóng", timeout)
		return nil
	}
}

// Close releases the socket.
func (s *Stream) Close() {
	s.once.Do(func() {
		_ = s.conn.Close(websocket.StatusNormalClosure, "")
		s.cancel()
		<-s.closed
	})
}

// Contains reports whether the entity appears anywhere in the changes.
func Contains(changes []Change, entityID string) bool {
	for _, change := range changes {
		if change.EntityID == entityID {
			return true
		}
	}
	return false
}

// CountFor returns how many changes named the entity.
func CountFor(changes []Change, entityID string) int {
	total := 0
	for _, change := range changes {
		if change.EntityID == entityID {
			total++
		}
	}
	return total
}
