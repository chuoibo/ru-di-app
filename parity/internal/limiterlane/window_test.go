package limiterlane

import (
	"strings"
	"testing"
	"time"
)

type fakeClock struct{ now float64 }

func (c *fakeClock) window() Window {
	return Window{Seconds: 60, Now: func() float64 { return c.now }, Sleep: func(d time.Duration) { c.now += d.Seconds() }}
}

func TestStartWaitsForTheNextBoundaryAndAMargin(t *testing.T) {
	for _, from := range []float64{125.3, 120, 179.99} {
		c := &fakeClock{now: from}
		if got := c.window().Start(); got != 3 {
			t.Errorf("from %g: window %d, want 3", from, got)
		}
		if want := 180 + Margin.Seconds(); c.now < want-1e-9 || c.now > want+1e-9 {
			t.Errorf("from %g: started at %g, want %g", from, c.now, want)
		}
	}
}

func TestCheckRefusesAScenarioThatReachedTheNextWindow(t *testing.T) {
	c := &fakeClock{now: 100}
	w := c.window()
	started := w.Start()
	c.now += 59
	if err := w.Check(started); err != nil {
		t.Fatalf("inside the window: %v", err)
	}
	c.now += 1
	if err := w.Check(started); err == nil || !strings.Contains(err.Error(), "outlasted") {
		t.Fatalf("next window accepted: %v", err)
	}
}

func TestMonotonicMovesForward(t *testing.T) {
	first, err := Monotonic()
	if err != nil {
		t.Skip(err)
	}
	time.Sleep(time.Millisecond)
	if second, _ := Monotonic(); second <= first {
		t.Fatalf("monotonic went from %g to %g", first, second)
	}
}
