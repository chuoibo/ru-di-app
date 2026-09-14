//go:build !linux

package limiterlane

import "errors"

// Monotonic is unavailable off Linux: the lane relies on the stacks reading
// the same CLOCK_MONOTONIC as the harness.
func Monotonic() (float64, error) {
	return 0, errors.New("the limiter lane needs Linux, where the stacks and the harness share CLOCK_MONOTONIC")
}
