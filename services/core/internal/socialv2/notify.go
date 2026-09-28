package socialv2

import (
	"context"
	"time"
)

func (h *Handler) subscribe(personID string) (chan struct{}, func()) {
	channel := make(chan struct{}, 1)
	h.mu.Lock()
	if h.watchers[personID] == nil {
		h.watchers[personID] = map[chan struct{}]struct{}{}
	}
	h.watchers[personID][channel] = struct{}{}
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		delete(h.watchers[personID], channel)
		if len(h.watchers[personID]) == 0 {
			delete(h.watchers, personID)
		}
		h.mu.Unlock()
	}
}

func (h *Handler) wake(personID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for channel := range h.watchers[personID] {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

// Run consumes PostgreSQL wake hints on one shared connection. The durable
// social_wall_changes table remains the source of truth after reconnects.
func (h *Handler) Run(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := h.pool.Acquire(ctx)
		if err == nil {
			_, err = conn.Exec(ctx, `LISTEN social_wall_changes`)
			if err == nil {
				for ctx.Err() == nil {
					message, receiveErr := conn.Conn().WaitForNotification(ctx)
					if receiveErr != nil {
						break
					}
					h.wake(message.Payload)
				}
			}
			conn.Release()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
