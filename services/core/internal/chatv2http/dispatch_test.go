package chatv2http

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mobile/services/core/internal/chatv2"
)

type batchProbe struct {
	memoryStore
	calls  atomic.Int64
	denied atomic.Bool
}

func (s *batchProbe) EventsBatch(ctx context.Context, _ string, rs []chatv2.Recipient) ([]chatv2.Delivery, error) {
	s.calls.Add(1)
	out := make([]chatv2.Delivery, len(rs))
	for i, r := range rs {
		if s.denied.Load() {
			out[i].Err = chatv2.ErrForbidden
			continue
		}
		if r.After == 0 {
			out[i].Page = chatv2.Page{Events: []chatv2.Event{{Sequence: 1, Kind: "mark", Mark: &chatv2.Mark{DeviceID: deviceID}}}, NextSequence: 1}
		}
	}
	return out, nil
}
func TestDispatcherCoalescesAndReleasesSharedFrames(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &batchProbe{}
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true, BatchSessions: true, Context: ctx, ReconcileInterval: time.Hour})
	start := make(chan struct{})
	results := make(chan dispatched, 200)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r, err := h.dispatch.next(ctx, conversationID, chatv2.Recipient{ActorID: personID, DeviceID: deviceID, Limit: 100})
			if err != nil {
				results <- dispatched{err: err}
				return
			}
			results <- r
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	frames := map[*sharedFrame]bool{}
	for r := range results {
		if r.err != nil || r.frame == nil {
			t.Fatal("dispatch failed", r.err)
		}
		frames[r.frame] = true
		r.frame.release()
	}
	if s.calls.Load() >= 20 || len(frames) >= 20 {
		t.Fatalf("failed to coalesce: calls=%d encodings=%d", s.calls.Load(), len(frames))
	}
	if used := h.dispatch.bytes.Load(); used != 0 {
		t.Fatalf("retained frame bytes: %d", used)
	}
}
func TestDispatcherQuietReconciliationAndMemoryBound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &batchProbe{}
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true, BatchSessions: true, Context: ctx, ReconcileInterval: 15 * time.Millisecond})
	h.dispatch.bytes.Store(maxEncodedDispatchBytes)
	r, err := h.dispatch.next(ctx, conversationID, chatv2.Recipient{After: 0, Limit: 100})
	if err != nil || !errors.Is(r.err, errDispatchCapacity) {
		t.Fatal("memory cap was not enforced", err, r.err)
	}
	h.dispatch.bytes.Store(0)
	go func() { time.Sleep(30 * time.Millisecond); s.denied.Store(true) }()
	r, err = h.dispatch.next(ctx, conversationID, chatv2.Recipient{After: 1, Limit: 100})
	if err != nil || !errors.Is(r.err, chatv2.ErrForbidden) {
		t.Fatal("quiet permission change not reconciled", err, r.err)
	}
}

func TestDispatcherCaughtUpAckWaitsForWakeWithoutCachingGrants(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &batchProbe{}
	h := New(Options{Store: s, Authenticate: fakeAuth, Experimental: true, BatchSessions: true, Context: ctx, ReconcileInterval: time.Hour})
	first, err := h.dispatch.next(ctx, conversationID, chatv2.Recipient{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	first.frame.release()
	calls := s.calls.Load()
	done := make(chan dispatched, 1)
	go func() {
		r, e := h.dispatch.next(ctx, conversationID, chatv2.Recipient{After: 1, Limit: 100})
		if e != nil {
			r.err = e
		}
		done <- r
	}()
	time.Sleep(40 * time.Millisecond)
	if s.calls.Load() != calls {
		t.Fatal("caught-up ACK triggered a redundant durable read")
	}
	s.denied.Store(true)
	h.Wake(conversationID)
	select {
	case r := <-done:
		if !errors.Is(r.err, chatv2.ErrForbidden) {
			t.Fatal("wake reused cached grant", r.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
