// Package limit ports the API's per-process rate limiters to Go.
//
// Python keeps two limiter classes, and a Go-served route must refuse on
// exactly the call Python refuses on, with the same bytes (ADR-0029 §2.4):
//
//   - ActorWindow is FixedWindowLimiter in
//     services/api/app/api/search_rate_limit.py: keyed by actor id, a window
//     opens at the first call that finds none open, a refusal leaves that
//     window where it is, and rolled windows are swept.
//   - AddressWindow is FixedWindowLimit in
//     services/api/app/api/routes/identity.py: keyed by source address,
//     windows are int(clock/window) buckets of the absolute clock, and the
//     whole table is dropped once it holds too many entries.
//
// Neither sets Retry-After or any other header. A refusal is the plain
// ApiProblem JSON body, written by WriteRefusal.
//
// Both are in memory and per process, like Python's: two replicas mean twice
// the ceiling and a restart forgives everyone. Parity preserves that.
package limit

import (
	"math"
	"net"
	"net/http"
	"sync"

	"mobile/services/core/internal/httpapi/problem"
)

// Decision is the outcome of one check. When Allowed is false, Problem is the
// ApiProblem the Python route raises, ready for WriteRefusal.
type Decision struct {
	Allowed bool
	Problem problem.Problem
}

// WriteRefusal writes the refusal exactly as Python's ApiProblem handler does:
// status 429, content-length and content-type application/json, and the
// compact {"code","detail"} body with non-ASCII left unescaped.
func WriteRefusal(w http.ResponseWriter, d Decision) error {
	return problem.WriteJSON(w, d.Problem)
}

// Caller is the address the per-address windows count against.
//
// Python reads request.client.host. Behind core, uvicorn runs with
// forwarded-allow-ips '*' and takes the first X-Forwarded-For entry, which
// core's proxy sets to exactly the host part of the socket peer
// (httputil.ProxyRequest.SetXForwarded), so for a Go-served route the same
// string is the host part of RemoteAddr. "unknown" mirrors Python's
// `if request.client else "unknown"` branch for a peer with no address.
func Caller(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return host
}

// ---------------------------------------------------------------------------
// ActorWindow: FixedWindowLimiter
// ---------------------------------------------------------------------------

// defaultMinSweepAt is search_rate_limit._MIN_SWEEP_AT: below this many
// tracked keys the map is not worth walking.
const defaultMinSweepAt = 1024

// ActorConfig is what FixedWindowLimiter takes at construction.
type ActorConfig struct {
	Limit         int
	WindowSeconds int
	Code          string
	Message       string
}

type actorEntry struct {
	openedAt float64
	used     int
}

// ActorWindow is a fixed window per key. It is safe for concurrent use.
type ActorWindow[K comparable] struct {
	limit      int
	window     float64
	refusal    problem.Problem
	clock      Clock
	minSweepAt int

	mu        sync.Mutex
	windows   map[K]actorEntry
	sweepAt   int
	lastSweep float64
}

// NewActorWindow builds the limiter. Like Python's constructor it reads the
// clock once, as the time of the last sweep.
func NewActorWindow[K comparable](cfg ActorConfig, clock Clock) *ActorWindow[K] {
	return newActorWindow[K](cfg, clock, defaultMinSweepAt)
}

func newActorWindow[K comparable](cfg ActorConfig, clock Clock, minSweepAt int) *ActorWindow[K] {
	l := &ActorWindow[K]{
		limit:      cfg.Limit,
		window:     float64(cfg.WindowSeconds),
		refusal:    problem.Problem{Status: http.StatusTooManyRequests, Code: cfg.Code, Detail: cfg.Message},
		clock:      clock,
		minSweepAt: minSweepAt,
		windows:    map[K]actorEntry{},
		sweepAt:    minSweepAt,
	}
	l.lastSweep = clock()
	return l
}

// Check admits one call or refuses it having spent nothing.
func (l *ActorWindow[K]) Check(key K) Decision {
	// Read before the lock, as Python does: under concurrency a caller can
	// hold an older `now` than the window it finds, and the arithmetic below
	// must see the same negative difference Python would.
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.windows[key]
	if !ok {
		entry = actorEntry{openedAt: now}
	}
	if now-entry.openedAt >= l.window {
		entry = actorEntry{openedAt: now}
	}
	if entry.used >= l.limit {
		// Written back unchanged: moving openedAt on a refusal would turn the
		// window into an indefinite ban under client retries. With a limit of
		// zero this also stores a fresh key without a sweep, as Python does.
		l.windows[key] = entry
		return Decision{Problem: l.refusal}
	}
	entry.used++
	l.windows[key] = entry
	if len(l.windows) >= l.sweepAt || now-l.lastSweep >= l.window {
		l.sweep(now)
	}
	return Decision{Allowed: true}
}

// Tracked reports how many keys are held, like FixedWindowLimiter.tracked().
func (l *ActorWindow[K]) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.windows)
}

// sweep drops windows that have rolled. Caller holds the lock. A new map, as
// Python's dict comprehension builds one, so memory is actually returned.
func (l *ActorWindow[K]) sweep(now float64) {
	kept := make(map[K]actorEntry, len(l.windows))
	for key, entry := range l.windows {
		if now-entry.openedAt < l.window {
			kept[key] = entry
		}
	}
	l.windows = kept
	l.sweepAt = max(l.minSweepAt, len(kept)*2)
	l.lastSweep = now
}

// ---------------------------------------------------------------------------
// AddressWindow: FixedWindowLimit
// ---------------------------------------------------------------------------

// defaultMaxTracked is identity._MAX_TRACKED.
const defaultMaxTracked = 10_000

// AddressConfig is what FixedWindowLimit takes, plus the ApiProblem its
// route raises on a refusal (the class returns a bool; each route words the
// 429 itself).
type AddressConfig struct {
	Limit         int
	WindowSeconds float64
	Code          string
	Detail        string
}

type addressKey struct {
	caller string
	// int(clock() / window) in Python. Held as the truncated float: equal
	// Python ints are exactly equal truncated floats, and a float cannot
	// overflow where an int64 conversion could.
	bucket float64
}

// AddressWindow counts calls per (caller, bucket). It is safe for concurrent
// use; Python's needs no lock only because every caller is an async handler
// on one event loop, which the mutex here stands in for.
type AddressWindow struct {
	limit      int
	window     float64
	refusal    problem.Problem
	clock      Clock
	maxTracked int

	mu   sync.Mutex
	seen map[addressKey]int
}

// NewAddressWindow builds the limiter. It does not read the clock.
//
// A zero or NaN window panics: Python would raise ZeroDivisionError on every
// call, and no shipped configuration has one.
func NewAddressWindow(cfg AddressConfig, clock Clock) *AddressWindow {
	return newAddressWindow(cfg, clock, defaultMaxTracked)
}

func newAddressWindow(cfg AddressConfig, clock Clock, maxTracked int) *AddressWindow {
	if cfg.WindowSeconds == 0 || math.IsNaN(cfg.WindowSeconds) {
		panic("limit: address window must be a non-zero number of seconds")
	}
	return &AddressWindow{
		limit:      cfg.Limit,
		window:     cfg.WindowSeconds,
		refusal:    problem.Problem{Status: http.StatusTooManyRequests, Code: cfg.Code, Detail: cfg.Detail},
		clock:      clock,
		maxTracked: maxTracked,
		seen:       map[addressKey]int{},
	}
}

// Check admits one call or refuses it. A refusal stores nothing.
func (l *AddressWindow) Check(caller string) Decision {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Python's int() truncates toward zero; so does Trunc.
	bucket := math.Trunc(l.clock() / l.window)
	// Checked before the lookup and on every call, admitted or not; stale
	// buckets count toward the bound because nothing else removes them.
	if len(l.seen) > l.maxTracked {
		l.seen = map[addressKey]int{}
	}
	key := addressKey{caller: caller, bucket: bucket}
	used := l.seen[key]
	if used >= l.limit {
		return Decision{Problem: l.refusal}
	}
	l.seen[key] = used + 1
	return Decision{Allowed: true}
}

// Allow is Check reduced to the bool FixedWindowLimit.allow returns.
func (l *AddressWindow) Allow(caller string) bool {
	return l.Check(caller).Allowed
}

// Tracked reports how many (caller, bucket) entries are held.
func (l *AddressWindow) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.seen)
}
