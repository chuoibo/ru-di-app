//go:build !linux

package limit

// Monotonic is seconds since process start off Linux, where no Python
// container shares this kernel's clock anyway.
func Monotonic() float64 {
	return fallbackMonotonic()
}
