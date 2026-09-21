package chatv2http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"mobile/services/core/internal/chatv2"
)

// These probes cover transport boundaries, not encrypted payload semantics.
type securityStore struct {
	memoryStore
	eventCalls       atomic.Int64
	markCalls        atomic.Int64
	blockAfter       int64
	deadlineObserved chan bool
}

func (s *securityStore) Events(ctx context.Context, a, d, c string, after int64, limit int) (chatv2.Page, error) {
	calls := s.eventCalls.Add(1)
	if s.deadlineObserved != nil && calls > s.blockAfter {
		_, bounded := ctx.Deadline()
		s.deadlineObserved <- bounded
		if !bounded {
			return chatv2.Page{}, errors.New("missing operation deadline")
		}
		<-ctx.Done()
		return chatv2.Page{}, ctx.Err()
	}
	return s.memoryStore.Events(ctx, a, d, c, after, limit)
}
func (s *securityStore) Mark(context.Context, string, string, string, string, int64) (chatv2.Mark, error) {
	s.markCalls.Add(1)
	return chatv2.Mark{}, nil
}

func TestMutationReauthenticatesAfterBodyDecode(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, changeActor := range []bool{false, true} {
			name := method + "/revoked"
			if changeActor {
				name = method + "/changed_actor"
			}
			t.Run(name, func(t *testing.T) {
				var changed atomic.Bool
				authed := make(chan struct{}, 1)
				s := &securityStore{}
				h := New(Options{Experimental: true, Store: s, Authenticate: func(ctx context.Context, headers http.Header) (string, error) {
					if changed.Load() {
						if changeActor {
							return deviceID, nil
						}
						return "", ErrAuthentication
					}
					select {
					case authed <- struct{}{}:
					default:
					}
					return fakeAuth(ctx, headers)
				}})
				path := "/v2/chat/" + conversationID + "/events"
				var value any = envelope()
				if method == http.MethodPut {
					path = "/v2/chat/" + conversationID + "/marks"
					value = map[string]any{"device_id": deviceID, "kind": "read", "sequence": 1}
				}
				body, writer := io.Pipe()
				defer body.Close()
				defer writer.Close()
				r := httptest.NewRequest(method, path, body)
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Authorization", "Bearer synthetic-session")
				w := httptest.NewRecorder()
				done := make(chan struct{})
				go func() { h.ServeHTTP(w, r); close(done) }()
				select {
				case <-authed:
				case <-time.After(time.Second):
					t.Fatal("request did not authenticate")
				}
				changed.Store(true)
				if err := json.NewEncoder(writer).Encode(value); err != nil {
					t.Fatal(err)
				}
				writer.Close()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("handler remained blocked")
				}
				if w.Code != 401 || s.sends != 0 || s.markCalls.Load() != 0 {
					t.Fatalf("status=%d sends=%d marks=%d", w.Code, s.sends, s.markCalls.Load())
				}
			})
		}
	}
}

func TestConnectionOverflowDoesNotReadStore(t *testing.T) {
	s := &securityStore{}
	h := New(Options{Experimental: true, Store: s, Authenticate: fakeAuth, MaxConnections: 1, ReconcileInterval: time.Hour})
	server := httptest.NewServer(h)
	defer server.Close()
	first := dial(t, server.URL)
	defer first.CloseNow()
	readFrame(t, first)
	before := s.eventCalls.Load()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	unexpected, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v2/chat/"+conversationID+"/stream?device_id="+deviceID, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer synthetic-session"}}})
	if unexpected != nil {
		unexpected.CloseNow()
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err == nil || response == nil || response.StatusCode != 429 {
		t.Fatalf("expected429, response=%v err=%v", response, err)
	}
	if calls := s.eventCalls.Load(); calls != before {
		t.Fatalf("overflow accessed store: before=%d after=%d", before, calls)
	}
}

func TestHTTPStoreOperationDeadline(t *testing.T) {
	s := &securityStore{deadlineObserved: make(chan bool, 1)}
	h := New(Options{Experimental: true, Store: s, Authenticate: fakeAuth, OperationTimeout: 20 * time.Millisecond})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request(t, h, http.MethodGet, "/v2/chat/"+conversationID+"/events?device_id="+deviceID, nil)
	}()
	select {
	case w := <-done:
		if w.Code != 503 {
			t.Fatal(w.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP store operation failed to time out")
	}
	if !<-s.deadlineObserved {
		t.Fatal("store received unbounded context")
	}
}

func TestWebsocketStoreOperationDeadline(t *testing.T) {
	s := &securityStore{blockAfter: 1, deadlineObserved: make(chan bool, 1)}
	h := New(Options{Experimental: true, Store: s, Authenticate: fakeAuth, OperationTimeout: 20 * time.Millisecond, ReconcileInterval: 10 * time.Millisecond})
	server := httptest.NewServer(h)
	defer server.Close()
	c := dial(t, server.URL)
	readFrame(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("store timeout did not close stream: %v", err)
	}
	select {
	case bounded := <-s.deadlineObserved:
		if !bounded {
			t.Fatal("stream store received unbounded context")
		}
	default:
		t.Fatal("stream did not enter catch-up")
	}
}

func TestWebsocketAuthenticationOperationDeadline(t *testing.T) {
	var calls atomic.Int64
	bounded := make(chan bool, 1)
	h := New(Options{Experimental: true, Store: &memoryStore{}, OperationTimeout: 20 * time.Millisecond, ReconcileInterval: 10 * time.Millisecond, Authenticate: func(ctx context.Context, headers http.Header) (string, error) {
		if calls.Add(1) > 1 {
			_, hasDeadline := ctx.Deadline()
			bounded <- hasDeadline
			if !hasDeadline {
				return "", ErrAuthentication
			}
			<-ctx.Done()
			return "", ctx.Err()
		}
		return fakeAuth(ctx, headers)
	}})
	server := httptest.NewServer(h)
	defer server.Close()
	c := dial(t, server.URL)
	readFrame(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("auth timeout did not close stream: %v", err)
	}
	select {
	case hasDeadline := <-bounded:
		if !hasDeadline {
			t.Fatal("stream authentication received unbounded context")
		}
	default:
		t.Fatal("stream did not reauthenticate")
	}
}
