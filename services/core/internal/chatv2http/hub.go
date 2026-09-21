package chatv2http

import "sync"

// Hub carries hints, not messages. A capacity-one channel coalesces bursts
// without ever letting a slow device block a committed write.
type Hub struct {
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]struct{}
}

func NewHub() *Hub { return &Hub{subscribers: map[string]map[chan struct{}]struct{}{}} }
func (h *Hub) Subscribe(id string) (<-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan struct{}, 1)
	if h.subscribers[id] == nil {
		h.subscribers[id] = map[chan struct{}]struct{}{}
	}
	h.subscribers[id][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subscribers[id], ch)
		if len(h.subscribers[id]) == 0 {
			delete(h.subscribers, id)
		}
	}
}
func (h *Hub) Wake(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers[id] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
