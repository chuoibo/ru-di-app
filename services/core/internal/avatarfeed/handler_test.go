package avatarfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"mobile/services/core/internal/auth"
)

const (
	alice = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	bob   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	carol = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

// fake is a Backend over three people: alice and bob share a group, carol is
// alone. Tokens are the person's name.
type fake struct {
	mu       sync.Mutex
	versions map[string]*string
	revoked  map[string]bool
}

func str(s string) *string { return &s }

func newFake() *fake {
	return &fake{versions: map[string]*string{alice: str("v-alice-1"), bob: nil, carol: str("v-carol-1")}, revoked: map[string]bool{}}
}

var byToken = map[string]string{"alice": alice, "bob": bob, "carol": carol}

func (f *fake) Actor(_ context.Context, digest []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for token, id := range byToken {
		if string(auth.TokenDigest(token)) == string(digest) && !f.revoked[token] {
			return id, nil
		}
	}
	return "", ErrAuthentication
}
func (f *fake) Authenticate(ctx context.Context, h http.Header) (string, error) {
	token, problem := auth.BearerToken(h)
	if problem != nil {
		return "", ErrAuthentication
	}
	return f.Actor(ctx, auth.TokenDigest(token))
}
func sees(viewer, subject string) bool {
	return viewer == subject || (viewer != carol && subject != carol)
}
func (f *fake) Versions(_ context.Context, actor string, ids []string) (map[string]*string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]*string{}
	for _, id := range ids {
		if v, known := f.versions[id]; known && sees(actor, id) {
			out[id] = v
		}
	}
	return out, nil
}
func (f *fake) Audience(_ context.Context, subject string) ([]string, *string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var viewers []string
	for _, v := range []string{alice, bob, carol} {
		if sees(v, subject) {
			viewers = append(viewers, v)
		}
	}
	return viewers, f.versions[subject], nil
}

const room = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"

func (f *fake) ContextMembers(_ context.Context, context string) ([]string, error) {
	if context == room {
		return []string{alice, bob}, nil
	}
	return nil, nil
}

func TestMembershipChangeTellsTheRoomAndThePersonToResync(t *testing.T) {
	h := New(newFake(), nil, context.Background(), nil)
	chans := map[string]chan Event{}
	for _, p := range []string{alice, bob, carol} {
		c, _ := h.register(p)
		chans[p] = c
	}
	if err := h.dispatch(context.Background(), "m:"+room+":"+carol); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{alice, bob, carol} {
		select {
		case e := <-chans[p]:
			if e.Type != "ready" {
				t.Errorf("%s got %+v, want ready", p, e)
			}
		default:
			t.Errorf("%s was not told to resync", p)
		}
	}
	for _, bad := range []string{"m:" + room, "m:x:" + carol, "x:" + room + ":" + carol, "hello"} {
		_ = h.dispatch(context.Background(), bad)
	}
	for p, c := range chans {
		select {
		case e := <-c:
			t.Errorf("%s got %+v from a malformed payload", p, e)
		default:
		}
	}
}

func TestParseIDs(t *testing.T) {
	ids, ok := ParseIDs("ids=" + strings.ToUpper(alice) + "," + bob + "," + alice)
	if !ok || len(ids) != 2 || ids[0] != alice || ids[1] != bob {
		t.Fatalf("got %v %v, want lowercased, deduplicated, in order", ids, ok)
	}
	many := make([]string, MaxIDs+1)
	for i := range many {
		many[i] = fmt.Sprintf("%08x-aaaa-4aaa-8aaa-aaaaaaaaaaaa", i)
	}
	if ids, ok := ParseIDs("ids=" + strings.Join(many[:MaxIDs], ",")); !ok || len(ids) != MaxIDs {
		t.Fatalf("exactly MaxIDs must pass, got %d %v", len(ids), ok)
	}
	for _, bad := range []string{"", "ids=", "ids=nope", "ids=" + alice + "&x=1", "ids=" + alice + "&ids=" + bob, "ids=" + strings.Join(many, ",")} {
		if _, ok := ParseIDs(bad); ok {
			t.Errorf("ParseIDs(%.60q) accepted", bad)
		}
	}
}

func TestMatchesOnlyTheTwoPaths(t *testing.T) {
	for path, want := range map[string]bool{"/people/avatars": true, "/people/avatars/stream": true, "/people/" + alice + "/avatar": false, "/people/avatars/x": false, "/people/me": false} {
		if Matches(path) != want {
			t.Errorf("Matches(%q) = %v", path, !want)
		}
	}
}

func get(t *testing.T, h http.Handler, query, token string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest("GET", "/people/avatars?"+query, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: a version answer must never be served stale", w.Header().Get("Cache-Control"))
	}
	return w.Code, body
}

func TestVersionsAnswerOnlyWhatTheReaderMaySee(t *testing.T) {
	h := New(newFake(), nil, context.Background(), nil)
	code, body := get(t, h, "ids="+alice+","+bob+","+carol, "bob")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	got := body["avatars"].(map[string]any)
	if got[alice] != "v-alice-1" {
		t.Errorf("alice = %v, want her version", got[alice])
	}
	if v, present := got[bob]; !present || v != nil {
		t.Errorf("bob = %v (present %v), want an explicit null: no avatar", v, present)
	}
	if _, present := got[carol]; present {
		t.Errorf("carol shares no group with bob and must be absent, got %v", got[carol])
	}
	if code, _ := get(t, h, "ids="+alice, ""); code != 401 {
		t.Errorf("no bearer: %d, want 401", code)
	}
	if code, _ := get(t, h, "ids=x", "bob"); code != 422 {
		t.Errorf("bad ids: %d, want 422", code)
	}
}

type client struct {
	conn *websocket.Conn
	t    *testing.T
}

func dial(t *testing.T, url, token string) client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, strings.Replace(url, "http", "ws", 1)+"/people/avatars/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	if err = wsjson.Write(ctx, conn, map[string]string{"type": "authenticate", "token": token}); err != nil {
		t.Fatal(err)
	}
	c := client{conn, t}
	if e := c.next(); e.Type != "ready" {
		t.Fatalf("first frame %+v, want ready", e)
	}
	return c
}
func (c client) next() Event {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var e Event
	if err := wsjson.Read(ctx, c.conn, &e); err != nil {
		c.t.Fatalf("read: %v", err)
	}
	return e
}
func (c client) nothingWithin(d time.Duration) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	var e Event
	if err := wsjson.Read(ctx, c.conn, &e); err == nil {
		c.t.Fatalf("unexpected frame %+v", e)
	}
}

func waitRegistered(t *testing.T, h *Handler, n int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		h.mu.Lock()
		total := 0
		for _, set := range h.conns {
			total += len(set)
		}
		h.mu.Unlock()
		if total == n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("connections never reached %d", n)
}

func TestStreamPushesTheNewVersionOnlyToTheAudience(t *testing.T) {
	f := newFake()
	h := New(f, nil, context.Background(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()
	a, b, c := dial(t, srv.URL, "alice"), dial(t, srv.URL, "bob"), dial(t, srv.URL, "carol")
	waitRegistered(t, h, 3)

	f.mu.Lock()
	f.versions[alice] = str("v-alice-2")
	f.mu.Unlock()
	if err := h.Broadcast(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	for name, cl := range map[string]client{"alice": a, "bob": b} {
		e := cl.next()
		if e.Type != "avatar" || e.PersonID != alice || e.AvatarID == nil || *e.AvatarID != "v-alice-2" {
			t.Errorf("%s got %+v, want alice's new version", name, e)
		}
	}
	c.nothingWithin(300 * time.Millisecond)
}

func TestStreamRefusesABadTokenAndClosesARevokedOne(t *testing.T) {
	f := newFake()
	h := New(f, nil, context.Background(), nil)
	h.Revalidate = 50 * time.Millisecond
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, strings.Replace(srv.URL, "http", "ws", 1)+"/people/avatars/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = wsjson.Write(ctx, conn, map[string]string{"type": "authenticate", "token": "mallory"})
	var e Event
	if err = wsjson.Read(ctx, conn, &e); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("unknown token: frame %+v err %v, want a policy close", e, err)
	}

	b := dial(t, srv.URL, "bob")
	f.mu.Lock()
	f.revoked["bob"] = true
	f.mu.Unlock()
	rctx, rcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer rcancel()
	if err = wsjson.Read(rctx, b.conn, &e); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("revoked session kept streaming: frame %+v err %v", e, err)
	}
}

func TestSlowConnectionIsClosedNotSilentlySkipped(t *testing.T) {
	h := New(newFake(), nil, context.Background(), nil)
	events, ok := h.register(bob)
	if !ok {
		t.Fatal("register")
	}
	for i := 0; i < cap(events)+1; i++ {
		h.deliver([]string{bob}, Event{Type: "avatar", PersonID: alice})
	}
	n := 0
	for range events {
		n++
	}
	if n != cap(events) {
		t.Fatalf("drained %d, want the %d buffered then a closed channel", n, cap(events))
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.conns[bob]) != 0 {
		t.Fatal("the dropped connection is still registered")
	}
}

func TestOriginRule(t *testing.T) {
	h := New(newFake(), nil, context.Background(), nil)
	for origin, want := range map[string]bool{"": true, "http://localhost:8081": true, "http://127.0.0.1:8177": true, "https://evil.example": false, "http://localhost/x": false} {
		r := httptest.NewRequest("GET", "http://api.example/people/avatars/stream", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if h.OriginAllowed(r) != want {
			t.Errorf("origin %q allowed = %v", origin, !want)
		}
	}
}
