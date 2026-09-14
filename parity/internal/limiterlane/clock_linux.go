//go:build linux

package limiterlane

import (
	"syscall"
	"unsafe"
)

const clockMonotonic = 1 // CLOCK_MONOTONIC

// Monotonic reads CLOCK_MONOTONIC, the clock behind Python's time.monotonic
// on Linux and behind core's limiter windows. The harness is a black box and
// cannot import core, so the call is repeated here.
func Monotonic() (float64, error) {
	var ts syscall.Timespec
	_, _, errno := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME, clockMonotonic, uintptr(unsafe.Pointer(&ts)), 0)
	if errno != 0 {
		return 0, errno
	}
	return float64(ts.Sec) + float64(ts.Nsec)/1e9, nil
}
