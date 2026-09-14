package limit

import "time"

var processStart = time.Now()

// fallbackMonotonic is seconds since this process started, on Go's monotonic
// reading. Differences match CLOCK_MONOTONIC; the absolute value does not.
func fallbackMonotonic() float64 {
	return secondsFromNanos(int64(time.Since(processStart)))
}
