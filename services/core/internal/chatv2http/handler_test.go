package chatv2http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"mobile/services/core/internal/chatv2"
)

const personID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const deviceID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const conversationID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

// This fake tests transport behavior only; the PostgreSQL suite separately
// verifies the actual permission, transaction, and signature checks.
type memoryStore struct {
	mu     sync.Mutex
	events []chatv2.Event
	denied bool
	sends  int
}

func (s *memoryStore) Send(_ context.Context, _ string, e chatv2.Envelope) (chatv2.SendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sends++
	if s.denied {
		return chatv2.SendResult{}, chatv2.ErrForbidden
	}
	for _, v := range s.events {
		if v.Envelope != nil && v.Envelope.LogicalSendID == e.LogicalSendID {
			return chatv2.SendResult{Event: v, Replayed: true}, nil
		}
	}
	v := chatv2.Event{Sequence: int64(len(s.events) + 1), Kind: "envelope", ActorID: personID, Envelope: &e}
	s.events = append(s.events, v)
	return chatv2.SendResult{Event: v}, nil
}
func (s *memoryStore) Events(_ context.Context, _, _, _ string, after int64, limit int) (chatv2.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := chatv2.Page{Events: []chatv2.Event{}, NextSequence: after}
	if s.denied {
		return p, chatv2.ErrForbidden
	}
	for _, e := range s.events {
		if e.Sequence > after {
			if len(p.Events) == limit {
				p.HasMore = true
				break
			}
			p.Events = append(p.Events, e)
			p.NextSequence = e.Sequence
		}
	}
	return p, nil
}
func (s *memoryStore) Mark(_ context.Context, _, _, _, _ string, _ int64) (chatv2.Mark, error) {
	return chatv2.Mark{}, nil
}
func fakeAuth(_ context.Context, h http.Header) (string, error) {
	if h.Get("Authorization") != "Bearer synthetic-session" {
		return "", ErrAuthentication
	}
	return personID, nil
}
func envelope() chatv2.Envelope {
	return chatv2.Envelope{ConversationID: conversationID, DeviceID: deviceID, LogicalSendID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", Protocol: chatv2.Protocol, Epoch: 1, Ciphertext: []byte("opaque transport fixture; not MLS proof"), Signature: []byte("signature checked by store")}
}
func request(t *testing.T, h http.Handler, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if value != nil {
		if err := json.NewEncoder(&b).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Authorization", "Bearer synthetic-session")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDisabledTransportCannotBeEnabledByRequest(t *testing.T) {
	h := New(Options{Store: &memoryStore{}, Authenticate: fakeAuth})
	w := request(t, h, "POST", "/v2/chat/"+conversationID+"/events", envelope())
	if w.Code != 503 {
		t.Fatalf("status %d", w.Code)
	}
}
func TestStrictWireAndReplayAuthorization(t *testing.T) {
	s := &memoryStore{}
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true})
	path := "/v2/chat/" + conversationID + "/events"
	if w := request(t, h, "POST", path, envelope()); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, h, "POST", path, envelope()); w.Code != 200 {
		t.Fatal(w.Code)
	}
	s.denied = true
	if w := request(t, h, "POST", path, envelope()); w.Code != 403 {
		t.Fatalf("replay bypass %d", w.Code)
	}
	for _, q := range []string{"?access_token=secret", "?after=1&after=2", "?limit=0", "?after=-1"} {
		if w := request(t, h, "GET", path+q, nil); w.Code < 400 {
			t.Fatalf("accepted %q", q)
		}
	}
	bad := envelope()
	bad.ConversationID = deviceID
	if w := request(t, h, "POST", path, bad); w.Code != 422 {
		t.Fatalf("cross-context %d", w.Code)
	}
	if w := request(t, h, "POST", path, map[string]any{"body": "plaintext", "conversation_id": conversationID}); w.Code != 400 {
		t.Fatalf("plaintext %d", w.Code)
	}
}
func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http")+"/v2/chat/"+conversationID+"/stream?device_id="+deviceID, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer synthetic-session"}}, Subprotocols: []string{chatv2.Protocol}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}
func readFrame(t *testing.T, c *websocket.Conn) frame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var f frame
	if err := wsjson.Read(ctx, c, &f); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestWebsocketReceivesReconciledEventsAndResumes(t *testing.T) {
	s := &memoryStore{}
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true, ReconcileInterval: 10 * time.Millisecond})
	server := httptest.NewServer(h)
	defer server.Close()
	c := dial(t, server.URL)
	if f := readFrame(t, c); f.Type != "ready" {
		t.Fatal(f)
	}
	// Simulate a different replica committing without a local wakeup.
	if _, err := s.Send(context.Background(), personID, envelope()); err != nil {
		t.Fatal(err)
	}
	f := readFrame(t, c)
	if f.Page == nil || len(f.Page.Events) != 1 || f.Page.NextSequence != 1 {
		t.Fatal(f)
	}
	if err := wsjson.Write(context.Background(), c, ack{Type: "ack", Sequence: 1}); err != nil {
		t.Fatal(err)
	}
	c.CloseNow()
	w := request(t, h, "GET", "/v2/chat/"+conversationID+"/events?device_id="+deviceID+"&after=1", nil)
	var p chatv2.Page
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(p.Events) != 0 || p.NextSequence != 1 {
		t.Fatal(w.Code, p)
	}
}
func TestQuietWebsocketReauthenticates(t *testing.T) {
	var revoked atomic.Bool
	h := New(Options{Store: &memoryStore{}, Experimental: true, ReconcileInterval: 10 * time.Millisecond, Authenticate: func(ctx context.Context, h http.Header) (string, error) {
		if revoked.Load() {
			return "", ErrAuthentication
		}
		return fakeAuth(ctx, h)
	}})
	server := httptest.NewServer(h)
	defer server.Close()
	c := dial(t, server.URL)
	readFrame(t, c)
	revoked.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("revoked session remained open: %v", err)
	}
}
func TestWebsocketRejectsFutureAckAndCrossOrigin(t *testing.T) {
	s := &memoryStore{}
	_, _ = s.Send(context.Background(), personID, envelope())
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true})
	server := httptest.NewServer(h)
	defer server.Close()
	c := dial(t, server.URL)
	readFrame(t, c)
	readFrame(t, c)
	if err := wsjson.Write(context.Background(), c, ack{Type: "ack", Sequence: 99}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatal(err)
	}
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v2/chat/"+conversationID+"/stream?device_id="+deviceID, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer synthetic-session"}, "Origin": []string{"https://untrusted.invalid"}}})
	if err == nil {
		t.Fatal("accepted hostile origin")
	}
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func TestBackendErrorDoesNotExposeCredentials(t *testing.T) {
	h := New(Options{Store: &memoryStore{}, Experimental: true, Authenticate: func(context.Context, http.Header) (string, error) { return "", errors.New("postgres://private-secret") }})
	w := request(t, h, "GET", "/v2/chat/"+conversationID+"/events", nil)
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHubCoalescesAndUnsubscribes(t *testing.T) {
	h := NewHub()
	ch, stop := h.Subscribe(conversationID)
	for i := 0; i < 100; i++ {
		h.Wake(conversationID)
	}
	select {
	case <-ch:
	default:
		t.Fatal("missing wake")
	}
	select {
	case <-ch:
		t.Fatal("unbounded wake")
	default:
	}
	stop()
	stop()
	h.Wake(conversationID)
	if len(h.subscribers) != 0 {
		t.Fatal("subscriber leak")
	}
}
