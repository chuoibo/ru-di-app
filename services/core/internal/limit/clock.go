package limit

// Clock returns seconds as a float64, the way Python's time.monotonic does.
// Tests inject a hand-wound one; production uses Monotonic.
type Clock func() float64

// secondsFromNanos converts a nanosecond reading exactly as CPython 3.12's
// _PyTime_AsSecondsDouble: whole seconds divide as integers, anything else
// converts to double first and then divides by 1e9.
func secondsFromNanos(ns int64) float64 {
	const perSecond = 1_000_000_000
	if ns%perSecond == 0 {
		return float64(ns / perSecond)
	}
	return float64(ns) / 1e9
}
