package routing

import "sync"

// slots is preview.py's _CAPACITY: four concurrent routing calls, acquired
// with blocking=False. Releasing is the caller's finally.
const slots = 4

var (
	capacityMu sync.Mutex
	inFlight   int
)

// TrySlot is _CAPACITY.acquire(blocking=False).
func TrySlot() bool {
	capacityMu.Lock()
	defer capacityMu.Unlock()
	if inFlight >= slots {
		return false
	}
	inFlight++
	return true
}

// ReleaseSlot is _CAPACITY.release().
func ReleaseSlot() {
	capacityMu.Lock()
	inFlight--
	capacityMu.Unlock()
}
