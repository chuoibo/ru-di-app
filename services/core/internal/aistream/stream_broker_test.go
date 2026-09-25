//go:build broker

package aistream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// CORE_TEST_REDIS_URL names a disposable Redis. With CORE_REQUIRE_BROKER_TESTS=1
// a missing URL is a failure: a skipped broker test reads like a passing one.
func open(t *testing.T) *Stream {
	t.Helper()
	url := os.Getenv("CORE_TEST_REDIS_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_REDIS_URL is required")
		}
		t.Skip("CORE_TEST_REDIS_URL not set")
	}
	ns := fmt.Sprintf("t%d", time.Now().UnixNano())
	s, err := Open(url, ns)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBrokerTierReachesRedis(t *testing.T) {
	s := open(t)
	if err := s.client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
}

func TestAppendReadResumesExclusively(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01")
	var ids []string
	for i := 0; i < 5; i++ {
		id, err := s.Append(ctx, key, MaxLenInvocation, Delta, map[string]any{"p": 0, "text": fmt.Sprint(i)}, time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	all, err := s.Read(ctx, key, "", 100)
	if err != nil || len(all) != 5 {
		t.Fatalf("read %d %v", len(all), err)
	}
	rest, err := s.Read(ctx, key, ids[1], 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 3 || rest[0].ID != ids[2] {
		t.Fatalf("resume after %s returned %d events starting %v; want exactly the suffix", ids[1], len(rest), rest)
	}
}

func TestExpiryFollowsTheSharingWindowAndTheEnding(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd02")
	window := time.Now().Add(10 * time.Minute)
	if _, err := s.Append(ctx, key, MaxLenInvocation, Delta, map[string]any{"text": "a"}, window); err != nil {
		t.Fatal(err)
	}
	ttl := s.client.TTL(ctx, key).Val()
	if ttl <= 9*time.Minute || ttl > 10*time.Minute {
		t.Fatalf("TTL %v, want the sharing window", ttl)
	}
	if _, err := s.Append(ctx, key, MaxLenInvocation, Xong, map[string]any{"message_id": "x"}, window); err != nil {
		t.Fatal(err)
	}
	if ttl = s.client.TTL(ctx, key).Val(); ttl > AfterTerminal {
		t.Fatalf("TTL after the ending %v, want <= %v", ttl, AfterTerminal)
	}
	// A room is shared: one invocation ending must not shorten it.
	room, _ := s.Keys.Room("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddaa")
	if _, err := s.Append(ctx, room, MaxLenRoom, Xong, map[string]any{"message_id": "x"}, window); err != nil {
		t.Fatal(err)
	}
	if ttl = s.client.TTL(ctx, room).Val(); ttl <= AfterTerminal {
		t.Fatalf("a room key was shortened by one ending: %v", ttl)
	}
}

func TestMaxLenBoundsTheStream(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd03")
	for i := 0; i < 400; i++ {
		if _, err := s.Append(ctx, key, 100, Delta, map[string]any{"text": "x"}, time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	// MAXLEN ~ trims by whole macro nodes, so the bound is approximate.
	if n := s.client.XLen(ctx, key).Val(); n > 300 {
		t.Fatalf("stream holds %d entries, want it bounded near 100", n)
	}
}

// End to end: a worker process appends, another process's hub wakes, an SSE
// client receives every event in order and the stream closes at the ending.
func TestFollowDeliversInOrderAcrossProcesses(t *testing.T) {
	writer := open(t)
	reader, err := Open(os.Getenv("CORE_TEST_REDIS_URL"), strings.TrimPrefix(strings.TrimSuffix(writer.Keys.prefix, ":ai:"), "rudi:"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub()
	ready := make(chan struct{})
	go reader.Listen(ctx, hub, ready)
	<-ready
	key, _ := writer.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd04")
	opt := DefaultFollow()
	opt.Reconcile = time.Hour // only wakes may deliver: proves the hub works
	opt.MaxDuration = 5 * time.Second
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_ = Follow(r.Context(), w, reader, hub, key, ResumeFrom(r), opt, nil)
	}))
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < 50; i++ {
			_, _ = writer.Append(context.Background(), key, MaxLenInvocation, Delta, map[string]any{"p": 0, "text": fmt.Sprintf("t%02d", i)}, time.Now().Add(time.Minute))
		}
		_, _ = writer.Append(context.Background(), key, MaxLenInvocation, Xong, map[string]any{"message_id": "m"}, time.Now().Add(time.Minute))
	}()
	scanner := bufio.NewScanner(resp.Body)
	var got []string
	var lastID, midID string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "id: ") {
			lastID = strings.TrimPrefix(line, "id: ")
			if len(got) == 24 {
				midID = lastID
			}
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"text"`) {
			got = append(got, line)
		}
	}
	if len(got) != 50 {
		t.Fatalf("received %d deltas, want 50", len(got))
	}
	for i, line := range got {
		if !strings.Contains(line, fmt.Sprintf(`"t%02d"`, i)) {
			t.Fatalf("delta %d out of order: %s", i, line)
		}
	}
	// Resuming from the ending's id returns nothing and closes at once.
	req, _ := http.NewRequest("GET", server.URL, nil)
	req.Header.Set("Last-Event-ID", lastID)
	start := time.Now()
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("resuming after the ending waited %v instead of closing", took)
	}
	if strings.Contains(string(body), "data: ") {
		t.Fatalf("resuming after the ending replayed events: %q", body)
	}
	// Resuming in the middle returns exactly the rest.
	req.Header.Set("Last-Event-ID", midID)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if n := strings.Count(string(body), `"text"`); n != 25 {
		t.Fatalf("resume from delta 24 returned %d deltas, want 25", n)
	}
}

func TestFollowRevokesWhenAuthorizationEnds(t *testing.T) {
	s := open(t)
	hub := NewHub()
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd05")
	opt := DefaultFollow()
	opt.Reconcile = 50 * time.Millisecond
	var allowed atomic.Bool
	allowed.Store(true)
	rec := httptest.NewRecorder()
	go func() { time.Sleep(200 * time.Millisecond); allowed.Store(false) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := Follow(ctx, rec, s, hub, key, "", opt, func(context.Context) bool { return allowed.Load() }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), "event: thu_hoi") {
		t.Fatalf("revoked reader was not told: %q", rec.Body.String())
	}
}
