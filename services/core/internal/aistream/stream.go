package aistream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// Stream is the Redis side: the job writers on the worker, Read and Listen
// on readers. It is the only code that writes a stream (XADD): the gate in
// internal/aigate holds every other package to that.
type Stream struct {
	client *redis.Client
	Keys   Keys
	// song is whether the wake subscription is up (Listen).
	song atomic.Bool
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

// Song reports whether this process hears the wake channel right now: the
// readers of this process are fed. A Redis that is down reads false within
// the Listen loop's next attempt.
func (s *Stream) Song() bool { return s.song.Load() }

// Entry is one event to append.
type Entry struct {
	Kind Kind
	Data any
}

// Append adds one event to key: AppendBatch with a single entry and no
// invocation field.
func (s *Stream) Append(ctx context.Context, key string, maxLen int64, kind Kind, data any, expireAt time.Time) (string, error) {
	ids, err := s.AppendBatch(ctx, key, maxLen, "", []Entry{{Kind: kind, Data: data}}, expireAt)
	if err != nil {
		return "", err
	}
	return ids[0], nil
}

// AppendBatch adds events to key in one pipeline, bounded to maxLen entries,
// and wakes every process's readers once.
//
// An invocation key expires at expireAt (the sharing window's end), and a
// terminal event shortens its life to AfterTerminal, never lengthens it. A
// room key interleaves many invocations: each entry carries inv, the id of
// the invocation it belongs to, so a requester can read only theirs; the key
// lives RoomWindow past its last write, and entries older than RoomWindow are
// trimmed even while newer invocations keep refreshing it. The caller decides
// which keys a job may write; for a v2 (E2EE) job that is never a room key.
func (s *Stream) AppendBatch(ctx context.Context, key string, maxLen int64, inv string, entries []Entry, expireAt time.Time) ([]string, error) {
	if !s.Keys.Owns(key) || (inv != "" && !idPattern.MatchString(inv)) {
		return nil, ErrName
	}
	if len(entries) == 0 {
		return nil, errors.New("aistream: nothing to append")
	}
	room := s.Keys.isRoom(key)
	pipe := s.client.TxPipeline()
	adds := make([]*redis.StringCmd, 0, len(entries))
	terminal := false
	for _, e := range entries {
		if !e.Kind.Valid() {
			return nil, ErrKind
		}
		raw, err := json.Marshal(e.Data)
		if err != nil {
			return nil, err
		}
		values := []any{"e", string(e.Kind), "j", string(raw)}
		if inv != "" {
			values = append(values, "inv", inv)
		}
		adds = append(adds, pipe.XAdd(ctx, &redis.XAddArgs{Stream: key, MaxLen: maxLen, Approx: true, Values: values}))
		terminal = terminal || e.Kind.Terminal()
	}
	if room {
		pipe.Expire(ctx, key, RoomWindow)
		pipe.XTrimMinIDApprox(ctx, key, minID(time.Now().Add(-RoomWindow)), 0)
	} else {
		pipe.ExpireAt(ctx, key, expireAt)
		if terminal {
			if left := time.Until(expireAt); left > AfterTerminal {
				pipe.Expire(ctx, key, AfterTerminal)
			}
		}
	}
	pipe.Publish(ctx, s.Keys.Wake(), strings.TrimPrefix(key, s.Keys.prefix))
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	ids := make([]string, len(adds))
	for i, a := range adds {
		ids[i] = a.Val()
	}
	return ids, nil
}

// Read returns up to count entries after the entry id `after`, exclusive; an
// empty after reads from the start. With inv set, only that invocation's
// entries of a room key are returned; the others are still read past, so the
// last id read is returned too, for the next read to start after.
func (s *Stream) Read(ctx context.Context, key, after string, count int64) ([]Event, error) {
	events, _, _, err := s.read(ctx, key, after, "", count)
	return events, err
}

// read is Read keeping only inv's entries ("" for all). last is the id of the
// last entry read, kept or not, and full says the read filled its count, so
// there may be more after last.
func (s *Stream) read(ctx context.Context, key, after, inv string, count int64) (out []Event, last string, full bool, err error) {
	if !s.Keys.Owns(key) {
		return nil, after, false, ErrName
	}
	start := "-"
	if after != "" {
		if !ValidID(after) {
			return nil, after, false, errors.New("aistream: invalid resume position")
		}
		start = "(" + after
	}
	msgs, err := s.client.XRangeN(ctx, key, start, "+", count).Result()
	if err != nil {
		return nil, after, false, err
	}
	last = after
	out = make([]Event, 0, len(msgs))
	for _, m := range msgs {
		last = m.ID
		e, ok := event(m)
		if !ok || (inv != "" && e.Inv != inv) {
			continue
		}
		out = append(out, e)
	}
	return out, last, int64(len(msgs)) >= count, nil
}

func event(m redis.XMessage) (Event, bool) {
	kind, _ := m.Values["e"].(string)
	data, _ := m.Values["j"].(string)
	inv, _ := m.Values["inv"].(string)
	if !Kind(kind).Valid() || !json.Valid([]byte(data)) {
		return Event{}, false
	}
	return Event{ID: m.ID, Kind: Kind(kind), Data: json.RawMessage(data), Inv: inv}, true
}

// EndedAt reports whether the entry `id` of key is a terminal event of inv
// ("" for any): a client resuming from the ending has nothing left to wait
// for.
func (s *Stream) EndedAt(ctx context.Context, key, id, inv string) (bool, error) {
	if !s.Keys.Owns(key) || !ValidID(id) {
		return false, ErrName
	}
	msgs, err := s.client.XRangeN(ctx, key, id, id, 1).Result()
	if err != nil || len(msgs) == 0 {
		return false, err
	}
	e, ok := event(msgs[0])
	return ok && e.Kind.Terminal() && (inv == "" || e.Inv == inv), nil
}

// MoLuot counts one SSE opening by person against perMinute, in Redis under a
// digest of the id (no person id is kept in Redis). It reports whether the
// opening is within the limit; a Redis that cannot answer lets it through
// (fail open, design 02 §5.2) and says so with the error. The count and its
// one-minute life are set in one script, atomically: a counter left with no
// TTL would never be evicted under volatile-ttl and refuse that person for
// good (review of slice 11, finding 9); a key found without one gets it.
func (s *Stream) MoLuot(ctx context.Context, person string, perMinute int64) (bool, error) {
	n, err := demMo.Run(ctx, s.client, []string{s.Keys.GioiHanMo(person)}, int64(time.Minute/time.Second)).Int64()
	if err != nil {
		return true, err
	}
	return n <= perMinute, nil
}

// demMo is INCR, then EXPIRE whenever the key has no TTL, in one script.
var demMo = redis.NewScript(`local n = redis.call('INCR', KEYS[1])
if redis.call('TTL', KEYS[1]) < 0 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
return n`)

// bamNguoi is the digest a person id is kept under in Redis.
func bamNguoi(person string) string {
	sum := sha256.Sum256([]byte("rl:sse:" + person))
	return hex.EncodeToString(sum[:16])
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
// failure. A missed wake is repaired by the readers' reconcile tick. Song
// reports true while the subscription is up.
func (s *Stream) Listen(ctx context.Context, hub *Hub, ready chan<- struct{}) {
	announced := false
	defer s.song.Store(false)
	for ctx.Err() == nil {
		sub := s.client.Subscribe(ctx, s.Keys.Wake())
		_, err := sub.Receive(ctx)
		if err == nil {
			s.song.Store(true)
			if !announced && ready != nil {
				close(ready)
				announced = true
			}
		}
		for err == nil && ctx.Err() == nil {
			var m *redis.Message
			m, err = sub.ReceiveMessage(ctx)
			if err == nil {
				hub.Wake(s.Keys.prefix + m.Payload)
			}
		}
		s.song.Store(false)
		_ = sub.Close()
		if ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}
