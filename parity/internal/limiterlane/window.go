// Package limiterlane runs scenarios that spend an in-memory rate limiter.
//
// Python's FixedWindowLimit and core's AddressWindow count per caller per
// window, the window being int(CLOCK_MONOTONIC / seconds). Every stack runs on
// this host, so both sides' window boundaries fall on the same instants. A
// scenario starts just after a boundary, runs on the reference and then on the
// candidate, and must end before the next boundary: each side then meets its
// limiter with nothing counted in that window, and the step that is refused is
// the same on both whatever ran before.
package limiterlane

import (
	"fmt"
	"math"
	"time"
)

// Margin is how long after a boundary a scenario starts. The harness and the
// stacks read one kernel clock, but a request sent on the boundary itself
// could be counted on either side of it.
const Margin = 250 * time.Millisecond

// Window places scenarios in limiter windows.
type Window struct {
	Seconds float64
	Now     func() float64 // CLOCK_MONOTONIC in seconds
	Sleep   func(time.Duration)
}

// Bucket is Python's window number, int(now / seconds).
func (w Window) Bucket() int64 { return int64(math.Floor(w.Now() / w.Seconds)) }

// Start sleeps until Margin after the next boundary and returns that window.
// It always waits for a new one: the current window may hold what the
// previous scenario spent.
func (w Window) Start() int64 {
	now := w.Now()
	next := (math.Floor(now/w.Seconds) + 1) * w.Seconds
	w.Sleep(time.Duration((next-now)*float64(time.Second)) + Margin)
	return w.Bucket()
}

// Check fails when the scenario reached the next window: from there on the
// two sides' counts no longer start from the same place.
func (w Window) Check(started int64) error {
	if now := w.Bucket(); now != started {
		return fmt.Errorf("the scenario outlasted one %g s limiter window (started in window %d, ended in %d)", w.Seconds, started, now)
	}
	return nil
}
