package chatv2http

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"mobile/services/core/internal/chatv2"
)

// BatchStore must authenticate sessions inside the same transaction as ACLs
// and ciphertext reads. It is selected explicitly only for session auth.
type BatchStore interface {
	EventsBatch(context.Context, string, []chatv2.Recipient) ([]chatv2.Delivery, error)
}

var errDispatchCapacity = errors.New("dispatch_capacity")

const maxEncodedDispatchBytes = 64 << 20

type sharedFrame struct {
	data  []byte
	refs  atomic.Int64
	owner *dispatcher
}

func (f *sharedFrame) release() {
	if f != nil && f.refs.Add(-1) == 0 {
		f.owner.bytes.Add(-int64(len(f.data)))
	}
}

type dispatched struct {
	page  chatv2.Page
	frame *sharedFrame
	err   error
}
type dispatchRequest struct {
	ctx       context.Context
	recipient chatv2.Recipient
	reply     chan dispatched
	isolated  bool
}
type dispatchRoom struct{ incoming chan *dispatchRequest }
type dispatcher struct {
	handler *Handler
	store   BatchStore
	mu      sync.Mutex
	rooms   map[string]*dispatchRoom
	bytes   atomic.Int64
	slots   chan struct{}
}

func newDispatcher(h *Handler, s BatchStore) *dispatcher {
	return &dispatcher{handler: h, store: s, rooms: map[string]*dispatchRoom{}, slots: make(chan struct{}, 8)}
}

func (d *dispatcher) next(ctx context.Context, conversation string, r chatv2.Recipient) (dispatched, error) {
	req := &dispatchRequest{ctx: ctx, recipient: r, reply: make(chan dispatched, 1)}
	d.mu.Lock()
	room := d.rooms[conversation]
	if room == nil {
		room = &dispatchRoom{incoming: make(chan *dispatchRequest, d.handler.options.MaxConnections)}
		d.rooms[conversation] = room
		go d.run(conversation, room)
	}
	select {
	case room.incoming <- req:
		d.mu.Unlock()
	default:
		d.mu.Unlock()
		return dispatched{}, errDispatchCapacity
	}
	select {
	case result, ok := <-req.reply:
		if !ok {
			return dispatched{}, context.Canceled
		}
		return result, nil
	case <-ctx.Done():
		// A reply and cancellation may race. Returning ownership through this
		// goroutine ensures every reserved frame byte is eventually released.
		go func() {
			if result, ok := <-req.reply; ok {
				result.frame.release()
			}
		}()
		return dispatched{}, ctx.Err()
	}
}

func (d *dispatcher) run(conversation string, room *dispatchRoom) {
	wake, unsubscribe := d.handler.hub.Subscribe(conversation)
	defer unsubscribe()
	ticker := time.NewTicker(d.handler.options.ReconcileInterval)
	defer ticker.Stop()
	pending := []*dispatchRequest{}
	scheduled := false
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	schedule := func() {
		if !scheduled {
			// A short coalescing window amortizes socket syscalls and ACKs. It is
			// bounded well below the realtime budget, even for a lone message.
			timer.Reset(20 * time.Millisecond)
			scheduled = true
		}
	}
	idle := time.Now()
	seen := int64(-1)
	dirty := true
	closeRequest := func(r *dispatchRequest) { close(r.reply) }
	for {
		select {
		case r := <-room.incoming:
			pending = append(pending, r)
			// A caught-up ACK does not need another identical ACL read. Wakeups
			// and reconciliation still validate every waiting socket.
			if dirty || r.recipient.After < seen || r.isolated {
				schedule()
			}
		case <-wake:
			dirty = true
			if len(pending) > 0 {
				schedule()
			}
		case <-ticker.C:
			dirty = true
			if len(pending) > 0 {
				schedule()
			} else if time.Since(idle) > 5*time.Second {
				d.mu.Lock()
				if len(room.incoming) == 0 {
					delete(d.rooms, conversation)
					d.mu.Unlock()
					return
				}
				d.mu.Unlock()
			}
		case <-d.handler.options.Context.Done():
			d.mu.Lock()
			delete(d.rooms, conversation)
			for len(room.incoming) > 0 {
				pending = append(pending, <-room.incoming)
			}
			d.mu.Unlock()
			for _, r := range pending {
				closeRequest(r)
			}
			return
		case <-timer.C:
			scheduled = false
			dirty = false
			for len(room.incoming) > 0 {
				pending = append(pending, <-room.incoming)
			}
			alive := pending[:0]
			for _, r := range pending {
				if r.ctx.Err() != nil {
					closeRequest(r)
				} else {
					alive = append(alive, r)
				}
			}
			pending = alive
			if len(pending) == 0 {
				idle = time.Now()
				continue
			}
			// Catch-up groups are separated by cursor distance. A device far behind
			// never chooses the event window for current readers. Each pass services
			// every group once; neither old nor live cursors can starve the other.
			sort.Slice(pending, func(i, j int) bool { return pending[i].recipient.After > pending[j].recipient.After })
			again := make([]*dispatchRequest, 0, len(pending))
			for start := 0; start < len(pending); {
				end := start + 1
				for end < len(pending) && end-start < chatv2.MaxDispatchRecipients && pending[start].isolated == pending[end].isolated && pending[start].recipient.After-pending[end].recipient.After < chatv2.MaxPageSize && (!pending[start].isolated || pending[start].recipient.After == pending[end].recipient.After) {
					end++
				}
				retry, more, high := d.deliver(conversation, pending[start:end])
				seen = max(seen, high)
				again = append(again, retry...)
				if more {
					schedule()
				}
				start = end
			}
			pending = again
			if len(pending) == 0 {
				idle = time.Now()
			}
		}
	}
}

func (d *dispatcher) deliver(conversation string, requests []*dispatchRequest) ([]*dispatchRequest, bool, int64) {
	recipients := make([]chatv2.Recipient, len(requests))
	for i, r := range requests {
		recipients[i] = r.recipient
	}
	ctx, cancel := context.WithTimeout(d.handler.options.Context, d.handler.options.OperationTimeout)
	var results []chatv2.Delivery
	var err error
	select {
	case d.slots <- struct{}{}:
		defer func() { <-d.slots }()
		results, err = d.store.EventsBatch(ctx, conversation, recipients)
	case <-ctx.Done():
		err = ctx.Err()
	}
	cancel()
	type pageKey struct {
		First, Last int64
		More        bool
	}
	cache := map[pageKey]*sharedFrame{}
	// One temporary reference retains each encoding while recipients acquire
	// ownership. A slow network writer only holds its shared bounded page.
	defer func() {
		for _, f := range cache {
			f.release()
		}
	}()
	retry := []*dispatchRequest{}
	more := false
	high := int64(0)
	for i, r := range requests {
		if r.ctx.Err() != nil {
			close(r.reply)
			continue
		}
		if err != nil {
			r.reply <- dispatched{err: err}
			continue
		}
		result := results[i]
		high = max(high, max(result.HighWatermark, result.Page.NextSequence))
		if result.Err != nil {
			r.reply <- dispatched{err: result.Err}
			continue
		}
		if len(result.Page.Events) == 0 {
			r.isolated = result.Page.HasMore
			retry = append(retry, r)
			more = more || result.Page.HasMore
			continue
		}
		// Pages from this one immutable window with equal boundaries have exactly
		// equal ciphertext and metadata, regardless of the authenticated device.
		key := pageKey{result.Page.Events[0].Sequence, result.Page.NextSequence, result.Page.HasMore}
		f := cache[key]
		if f == nil {
			data, e := json.Marshal(frame{Type: "events", Page: &result.Page})
			if e != nil {
				r.reply <- dispatched{err: e}
				continue
			}
			used := d.bytes.Add(int64(len(data)))
			if used > maxEncodedDispatchBytes {
				d.bytes.Add(-int64(len(data)))
				r.reply <- dispatched{err: errDispatchCapacity}
				continue
			}
			f = &sharedFrame{data: data, owner: d}
			f.refs.Store(1)
			cache[key] = f
		}
		f.refs.Add(1)
		r.reply <- dispatched{page: result.Page, frame: f}
	}
	return retry, more, high
}
