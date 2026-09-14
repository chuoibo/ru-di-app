//go:build linux

package limit

import (
	"syscall"
	"unsafe"
)

const clockMonotonic = 1 // CLOCK_MONOTONIC

// Monotonic reads CLOCK_MONOTONIC, the clock behind Python's time.monotonic
// on Linux (time.get_clock_info in the pinned image says so).
//
// The absolute value matters, not just differences: AddressWindow buckets
// int(clock / window), so reading the same kernel clock puts Go's minute
// boundaries on the same instants as the Python container's on one host.
// Go's own monotonic reading is not exposed, hence the direct call.
func Monotonic() float64 {
	var ts syscall.Timespec
	_, _, errno := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME, clockMonotonic, uintptr(unsafe.Pointer(&ts)), 0)
	if errno != 0 {
		// clock_gettime(CLOCK_MONOTONIC) does not fail on Linux; Python
		// would raise here. Fall back rather than hand out zero forever.
		return fallbackMonotonic()
	}
	return secondsFromNanos(ts.Nano())
}
