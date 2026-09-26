package limit

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// handClock is a hand-wound clock, the Go twin of tests/api's Clock.
type handClock struct {
	mu    sync.Mutex
	now   float64
	reads int
}

func (c *handClock) read() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reads++
	return c.now
}

func (c *handClock) set(t float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

type step struct {
	at   float64
	key  string
	want bool
}

func actorConfig(limit, window int) ActorConfig {
	return ActorConfig{Limit: limit, WindowSeconds: window, Code: "c", Message: "m"}
}

func runActor(t *testing.T, l *ActorWindow[string], clock *handClock, steps []step) {
	t.Helper()
	for i, s := range steps {
		clock.set(s.at)
		got := l.Check(s.key)
		if got.Allowed != s.want {
			t.Fatalf("step %d (t=%v key=%s): allowed=%v, want %v", i, s.at, s.key, got.Allowed, s.want)
		}
		if !got.Allowed && (got.Problem.Status != 429 || got.Problem.Code != "c" || got.Problem.Detail != "m") {
			t.Fatalf("step %d: refusal %+v", i, got.Problem)
		}
	}
}

func TestActorWindowEdgesAreInclusiveAndRefusalsDoNotSlide(t *testing.T) {
	clock := &handClock{}
	l := NewActorWindow[string](actorConfig(2, 60), clock.read)
	runActor(t, l, clock, []step{
		{0, "a", true},
		{0, "a", true},
		{0, "a", false},
		{30, "a", false},
		{59.999, "a", false}, // just under the edge
		{60, "a", true},      // at the edge: now - opened_at >= window rolls
		{60.001, "a", true},  // the new window opened at 60, not at a refusal
		{60.002, "a", false},
		{119.999, "a", false},
		{120.001, "a", true}, // just over the edge
	})
}

func TestActorWindowFloatSubtractionNotDecimal(t *testing.T) {
	// 1060.1 - 1000.1 is a hair under 60.0 in binary, so the window has not
	// rolled although the decimal difference is exactly sixty; 60.1 - 0.1 is
	// exactly 60.0 and has. Python does float arithmetic, so must Go.
	clock := &handClock{now: 1000.1}
	l := NewActorWindow[string](actorConfig(1, 60), clock.read)
	runActor(t, l, clock, []step{
		{1000.1, "a", true},
		{1060.1, "a", false},
		{1060.2, "a", true},
	})
	clock = &handClock{now: 0.1}
	l = NewActorWindow[string](actorConfig(1, 60), clock.read)
	runActor(t, l, clock, []step{
		{0.1, "a", true},
		{60.1, "a", true},
	})
}

func TestActorWindowKeysAreIsolated(t *testing.T) {
	clock := &handClock{}
	l := NewActorWindow[string](actorConfig(1, 60), clock.read)
	runActor(t, l, clock, []step{
		{0, "a", true},
		{0, "a", false},
		{0, "b", true},
		{1, "b", false},
		{1, "c", true},
		{60, "a", true},
		{60, "b", true},  // b opened at 0 as well, and rolled with a
		{60, "c", false}, // c opened at 1: its own window, 59 s old
	})
}

func TestActorWindowBackwardsClockKeepsWindow(t *testing.T) {
	// A caller that read the clock before another caller opened the window
	// sees a negative difference; the window must neither roll nor break.
	clock := &handClock{}
	l := NewActorWindow[string](actorConfig(1, 60), clock.read)
	runActor(t, l, clock, []step{
		{10, "a", true},
		{9.5, "a", false},
		{-100, "a", false},
		{70, "a", true},
	})
}

func TestActorWindowZeroLimitStoresRefusedKeysWithoutSweeping(t *testing.T) {
	clock := &handClock{}
	l := newActorWindow[string](actorConfig(0, 60), clock.read, 2)
	for i, key := range []string{"a", "b", "c", "d", "e"} {
		clock.set(float64(i * 100))
		if l.Check(key).Allowed {
			t.Fatalf("limit 0 admitted %s", key)
		}
	}
	if got := l.Tracked(); got != 5 {
		t.Fatalf("tracked=%d, want 5: a refusal writes the entry back and never sweeps", got)
	}
}

func TestActorWindowSweepBySizeDoublesThreshold(t *testing.T) {
	clock := &handClock{}
	l := newActorWindow[string](actorConfig(3, 60), clock.read, 4)
	for _, key := range []string{"a", "b", "c", "d"} {
		l.Check(key)
	}
	// The fourth key reached the threshold: a sweep ran, dropped nothing
	// (nothing has rolled) and doubled the threshold from what survived.
	if l.Tracked() != 4 || l.sweepAt != 8 || l.lastSweep != 0 {
		t.Fatalf("tracked=%d sweepAt=%d lastSweep=%v", l.Tracked(), l.sweepAt, l.lastSweep)
	}
	for _, key := range []string{"e", "f", "g"} {
		l.Check(key)
	}
	if l.Tracked() != 7 {
		t.Fatalf("tracked=%d before the doubled threshold, want 7", l.Tracked())
	}
	clock.set(59.999)
	l.Check("h") // eight keys, threshold eight: sweeps, but nothing has rolled
	if l.Tracked() != 8 || l.sweepAt != 16 {
		t.Fatalf("tracked=%d sweepAt=%d", l.Tracked(), l.sweepAt)
	}
}

func TestActorWindowSweepByTimeUsesConstructionClock(t *testing.T) {
	build := func(constructedAt float64) *ActorWindow[string] {
		clock := &handClock{now: constructedAt}
		l := newActorWindow[string](actorConfig(3, 60), clock.read, 1024)
		for _, s := range []step{{0, "a", true}, {59, "b", true}, {60, "c", true}} {
			clock.set(s.at)
			l.Check(s.key)
		}
		return l
	}
	// Built at 0: the call at 60 is one window after the last sweep, so it
	// sweeps and drops a (opened at 0, 60 - 0 is not < 60).
	if got := build(0).Tracked(); got != 2 {
		t.Fatalf("built at 0: tracked=%d, want 2", got)
	}
	// Built at 10: 60 - 10 < 60, no sweep, a stays resident.
	if got := build(10).Tracked(); got != 3 {
		t.Fatalf("built at 10: tracked=%d, want 3", got)
	}
}

func TestActorWindowRefusalNeverSweeps(t *testing.T) {
	clock := &handClock{}
	l := newActorWindow[string](actorConfig(1, 60), clock.read, 1024)
	runActor(t, l, clock, []step{
		{0, "a", true},
		{0, "b", true},
		{59, "d", true},
		// A window of time since the last sweep, and a and b have rolled, but
		// this call is refused (d opened at 59) and a refusal never sweeps.
		{61, "d", false},
	})
	if l.Tracked() != 3 {
		t.Fatalf("after refusal at 61: tracked=%d, want 3", l.Tracked())
	}
	runActor(t, l, clock, []step{{61, "c", true}})
	if l.Tracked() != 2 {
		t.Fatalf("after admitted call at 61: tracked=%d, want 2 (d and c)", l.Tracked())
	}
}

func TestActorWindowConcurrentCallersAdmitExactlyTheLimit(t *testing.T) {
	clock := &handClock{}
	l := newActorWindow[string](actorConfig(30, 60), clock.read, 8)
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if l.Check("same").Allowed {
					admitted.Add(1)
				}
				// Other keys churn the map and trigger sweeps under -race.
				l.Check(string(rune('a' + (g+i)%26)))
			}
		}(g)
	}
	wg.Wait()
	if admitted.Load() != 30 {
		t.Fatalf("admitted %d of 1280 calls on one key, want exactly 30", admitted.Load())
	}
}

func addressConfig(limit int, window float64) AddressConfig {
	return AddressConfig{Limit: limit, WindowSeconds: window, Code: "rate_limited", Detail: "d"}
}

func runAddress(t *testing.T, l *AddressWindow, clock *handClock, steps []step) {
	t.Helper()
	for i, s := range steps {
		clock.set(s.at)
		got := l.Check(s.key)
		if got.Allowed != s.want {
			t.Fatalf("step %d (t=%v key=%s): allowed=%v, want %v", i, s.at, s.key, got.Allowed, s.want)
		}
		if !got.Allowed && (got.Problem.Status != 429 || got.Problem.Code != "rate_limited" || got.Problem.Detail != "d") {
			t.Fatalf("step %d: refusal %+v", i, got.Problem)
		}
	}
}

func TestAddressWindowBucketsAreAbsolute(t *testing.T) {
	clock := &handClock{}
	l := NewAddressWindow(addressConfig(1, 60), clock.read)
	runAddress(t, l, clock, []step{
		{0, "10.0.0.1", true},
		{59.999, "10.0.0.1", false}, // just under the edge: still bucket 0
		{60, "10.0.0.1", true},      // at the edge: int(60 / 60) is bucket 1
		{60.001, "10.0.0.1", false},
		{119.999, "10.0.0.1", false},
		{120.001, "10.0.0.1", true}, // just over the next edge
		// Buckets belong to the clock, not to the first call: 0.2 s apart
		// across an edge is two fresh windows.
		{179.9, "10.0.0.2", true},
		{180.1, "10.0.0.2", true},
	})
}

func TestAddressWindowTruncatesTowardZero(t *testing.T) {
	clock := &handClock{}
	l := NewAddressWindow(addressConfig(1, 0.5), clock.read)
	runAddress(t, l, clock, []step{
		{0.49, "a", true},
		{0.5, "a", true}, // bucket 1: ceil would have put 0.49 there already
		{0.999, "a", false},
		// int(-0.25 / 0.5) is 0 in Python, not floor's -1.
		{-0.25, "b", true},
		{0.25, "b", false},
	})
}

func TestAddressWindowRefusalStoresNothingAndStaleBucketsStay(t *testing.T) {
	clock := &handClock{}
	l := NewAddressWindow(addressConfig(0, 60), clock.read)
	if l.Allow("a") || l.Tracked() != 0 {
		t.Fatalf("limit 0: tracked=%d", l.Tracked())
	}
	l = NewAddressWindow(addressConfig(1, 60), clock.read)
	for _, at := range []float64{0, 60, 120, 180} {
		clock.set(at)
		l.Allow("a")
	}
	if l.Tracked() != 4 {
		t.Fatalf("tracked=%d, want 4: nothing but the bound removes an old bucket", l.Tracked())
	}
}

func TestAddressWindowClearsAboveBoundAndForgetsCounts(t *testing.T) {
	clock := &handClock{}
	l := newAddressWindow(addressConfig(1, 60), clock.read, 3)
	runAddress(t, l, clock, []step{
		{0, "c0", true},
		{0, "c1", true},
		{0, "c2", true},
		{0, "c3", true}, // len 3 is not > 3: stored, len 4
		{0, "c0", true}, // len 4 > 3: cleared first, so c0 counts from zero
	})
	if l.Tracked() != 1 {
		t.Fatalf("tracked=%d after clear, want 1", l.Tracked())
	}
	runAddress(t, l, clock, []step{{0, "c0", false}})

	// The bound is checked before the lookup, so a call that would have been
	// refused is admitted once the table is over the bound.
	l = newAddressWindow(addressConfig(1, 60), clock.read, 1)
	runAddress(t, l, clock, []step{
		{0, "a", true},
		{0, "b", true}, // len 1 is not > 1: stored, len 2
		{0, "b", true}, // len 2 > 1: cleared, b's spent slot forgotten
		{0, "b", false},
	})
}

func TestAddressWindowZeroWindowPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("zero window did not panic")
		}
	}()
	NewAddressWindow(addressConfig(1, 0), func() float64 { return 0 })
}

func TestAddressWindowConcurrentCallersAdmitExactlyTheLimit(t *testing.T) {
	clock := &handClock{}
	l := newAddressWindow(addressConfig(20, 60), clock.read, 100)
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if l.Allow("192.0.2.7") {
					admitted.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != 20 {
		t.Fatalf("admitted %d, want exactly 20", admitted.Load())
	}
}

func TestCallerIsWhatCoreForwardsToPython(t *testing.T) {
	for _, remote := range []string{"192.0.2.1:1234", "[::1]:80", "[fe80::1%eth0]:443", "10.1.2.3:0"} {
		r := httptest.NewRequest(http.MethodPost, "/auth/otp/request", nil)
		r.RemoteAddr = remote
		out := r.Clone(r.Context())
		out.Header = http.Header{}
		pr := &httputil.ProxyRequest{In: r, Out: out}
		pr.SetXForwarded()
		// uvicorn with forwarded-allow-ips '*' takes the first entry, stripped.
		forwarded := out.Header.Get("X-Forwarded-For")
		if got := Caller(r); got != forwarded {
			t.Fatalf("%s: Caller=%q, core forwards %q", remote, got, forwarded)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "not-an-address"
	if got := Caller(r); got != "unknown" {
		t.Fatalf("Caller=%q", got)
	}
}

func TestSecondsFromNanosMatchesCPython(t *testing.T) {
	var big int64 = (1 << 47) + 7 // runtime division, not constant folding
	cases := []struct {
		ns   int64
		want float64
	}{
		{0, 0},
		{5_000_000_000, 5},
		{1, 1e-9},
		{1_500_000_000, 1.5},
		{-2_000_000_000, -2},
		{big, float64(big) / 1e9},
	}
	for _, c := range cases {
		if got := secondsFromNanos(c.ns); got != c.want {
			t.Fatalf("secondsFromNanos(%d)=%v, want %v", c.ns, got, c.want)
		}
	}
}

func TestMonotonicAdvancesWithTime(t *testing.T) {
	first := Monotonic()
	previous := first
	for i := 0; i < 1000; i++ {
		now := Monotonic()
		if now < previous {
			t.Fatalf("monotonic went backwards: %v after %v", now, previous)
		}
		previous = now
	}
	time.Sleep(20 * time.Millisecond)
	if elapsed := Monotonic() - first; elapsed < 0.019 || elapsed > 5 {
		t.Fatalf("20ms sleep measured as %vs", elapsed)
	}
}

func TestNewSetBuildsDistinctWindowsReadingTheClockOncePerActorWindow(t *testing.T) {
	clock := &handClock{}
	set := NewSet(clock.read)
	if clock.reads != 9 {
		t.Fatalf("clock read %d times at construction, want 9 (one per actor window)", clock.reads)
	}
	// One object per door, never one object with two names: a shared window
	// lets one feature's burst disable its neighbour.
	actor, _ := windowsByState(set)
	seen := map[*ActorWindow[string]]string{}
	for state, window := range actor {
		if other, ok := seen[window]; ok {
			t.Fatalf("%s and %s share one window", state, other)
		}
		seen[window] = state
	}
}
