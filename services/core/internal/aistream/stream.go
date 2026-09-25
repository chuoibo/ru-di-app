package aistream

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Bounds on what one stream may hold in Redis.
const (
	// MaxLenInvocation bounds one invocation's stream; deltas are coalesced
	// upstream, so a long answer is well under this.
	MaxLenInvocation = 1024
	// MaxLenRoom bounds a room's stream, which interleaves its invocations.
	MaxLenRoom = 2048
	// AfterTerminal is how long a finished stream stays readable, so a client
	// that reconnects a moment late still gets the ending.
	AfterTerminal = 120 * time.Second
	// RoomWindow is the sharing window: no room entry outlives it.
	RoomWindow = 15 * time.Minute
)

// minID is the smallest stream id at or after t.
func minID(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) + "-0" }

// Stream is the Redis side: Append on the worker, Read and Listen on readers.
type Stream struct {
	client *redis.Client
	Keys   Keys
}

// Open connects with the same short timeouts as internal/chatbus. Errors never
// echo the URL, which may carry a password.
func Open(rawURL, namespace string) (*Stream, error) {
	keys, err := NewKeys(namespace)
	if err != nil {
		return nil, err
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, errors.New("aistream: invalid Redis configuration")
	}
	options.DialTimeout = 500 * time.Millisecond
	options.ReadTimeout = 500 * time.Millisecond
	options.WriteTimeout = 500 * time.Millisecond
	options.ContextTimeoutEnabled = true
	options.MaxRetries = -1
	options.PoolSize = 32
	options.DisableIdentity = true
	return &Stream{client: redis.NewClient(options), Keys: keys}, nil
}

// Close releases the connection pool.
func (s *Stream) Close() error { return s.client.Close() }

// Append adds one event to key, bounded to maxLen entries and expiring at
// expireAt (the sharing window's end), and wakes every process's readers. A
// terminal event shortens the key's life to AfterTerminal, never lengthens it.
// The caller decides which keys a job may write; for a v2 (E2EE) job that is
// never a room key.
func (s *Stream) Append(ctx context.Context, key string, maxLen int64, kind Kind, data any, expireAt time.Time) (string, error) {
	if !kind.Valid() {
		return "", ErrKind
	}
	if !s.Keys.Owns(key) {
		return "", ErrName
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	pipe := s.client.TxPipeline()
	add := pipe.XAdd(ctx, &redis.XAddArgs{Stream: key, MaxLen: maxLen, Approx: true, Values: []any{"e", string(kind), "j", string(raw)}})
	pipe.ExpireAt(ctx, key, expireAt)
	if s.Keys.isRoom(key) {
		// A room interleaves many invocations: one ending never shortens the
		// key, and entries older than the sharing window are trimmed even while
		// newer invocations keep refreshing the key's TTL.
		pipe.XTrimMinIDApprox(ctx, key, minID(time.Now().Add(-RoomWindow)), 0)
	} else if kind.Terminal() {
		if left := time.Until(expireAt); left > AfterTerminal {
			pipe.Expire(ctx, key, AfterTerminal)
		}
	}
	pipe.Publish(ctx, s.Keys.Wake(), strings.TrimPrefix(key, s.Keys.prefix))
	if _, err = pipe.Exec(ctx); err != nil {
		return "", err
	}
	return add.Val(), nil
}

// Read returns up to count events after the entry id `after`, exclusive; an
// empty after reads from the start.
func (s *Stream) Read(ctx context.Context, key, after string, count int64) ([]Event, error) {
	if !s.Keys.Owns(key) {
		return nil, ErrName
	}
	start := "-"
	if after != "" {
		if !ValidID(after) {
			return nil, errors.New("aistream: invalid resume position")
		}
		start = "(" + after
	}
	msgs, err := s.client.XRangeN(ctx, key, start, "+", count).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(msgs))
	for _, m := range msgs {
		kind, _ := m.Values["e"].(string)
		data, _ := m.Values["j"].(string)
		if !Kind(kind).Valid() || !json.Valid([]byte(data)) {
			continue
		}
		out = append(out, Event{ID: m.ID, Kind: Kind(kind), Data: json.RawMessage(data)})
	}
	return out, nil
}

// EndedAt reports whether the entry `id` of key is a terminal event: a client
// resuming from the ending has nothing left to wait for.
func (s *Stream) EndedAt(ctx context.Context, key, id string) (bool, error) {
	if !s.Keys.Owns(key) || !ValidID(id) {
		return false, ErrName
	}
	msgs, err := s.client.XRangeN(ctx, key, id, id, 1).Result()
	if err != nil || len(msgs) == 0 {
		return false, err
	}
	kind, _ := msgs[0].Values["e"].(string)
	return Kind(kind).Terminal(), nil
}

// Hub wakes this process's readers of a key when any process appends to it.
// Readers pull from Redis on wake; a slow client falls behind in Redis, not in
// this process's memory.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[string]map[chan struct{}]struct{}{}} }

// Subscribe returns a channel that receives (without blocking the hub) when
// key grows, and the function that unsubscribes it.
func (h *Hub) Subscribe(key string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = map[chan struct{}]struct{}{}
	}
	h.subs[key][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[key], ch)
		if len(h.subs[key]) == 0 {
			delete(h.subs, key)
		}
		h.mu.Unlock()
	}
}

// Wake signals every subscriber of key.
func (h *Hub) Wake(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[key] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Listen feeds the hub from the wake channel until ctx ends, reconnecting on
// failure. A missed wake is repaired by the readers' reconcile tick.
func (s *Stream) Listen(ctx context.Context, hub *Hub, ready chan<- struct{}) {
	announced := false
	for ctx.Err() == nil {
		sub := s.client.Subscribe(ctx, s.Keys.Wake())
		_, err := sub.Receive(ctx)
		if err == nil && !announced && ready != nil {
			close(ready)
			announced = true
		}
		for err == nil && ctx.Err() == nil {
			var m *redis.Message
			m, err = sub.ReceiveMessage(ctx)
			if err == nil {
				hub.Wake(s.Keys.prefix + m.Payload)
			}
		}
		_ = sub.Close()
		if ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}
