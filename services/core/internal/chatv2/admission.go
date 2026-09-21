package chatv2

import (
	"context"
	"errors"
	"sync"
)

const (
	maxWriteRooms     = 1024
	maxAdmittedWrites = 2048
	maxRoomWrites     = 256
)

var errWriteCapacity = errors.New("chat_v2_write_capacity")

type writeRoom struct {
	active chan struct{}
	refs   int
}

// writeAdmission queues contending writers before they acquire a database
// connection. PostgreSQL still owns authorization, sequencing and deduplication.
// A bounded local pipeline cannot exhaust the pool behind one room's row lock.
type writeAdmission struct {
	mu      sync.Mutex
	rooms   map[string]*writeRoom
	total   int
	active  chan struct{}
	perRoom int
}

func newWriteAdmission(poolSize int32) *writeAdmission {
	// Reserve capacity for authentication, catch-up and the persistent LISTEN
	// connection even when different conversations are writing concurrently.
	slots := max(1, min(8, int(poolSize)/2))
	// Two in-flight transactions can prepare the next authorized write while
	// PostgreSQL commits its predecessor. Small pools retain one per room so
	// a blocked room cannot consume all global writer capacity.
	return &writeAdmission{rooms: map[string]*writeRoom{}, active: make(chan struct{}, slots), perRoom: max(1, min(2, slots-1))}
}

func (a *writeAdmission) acquire(ctx context.Context, conversation string) (func(), error) {
	if !ValidID(conversation) {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	r := a.rooms[conversation]
	if a.total >= maxAdmittedWrites || (r == nil && len(a.rooms) >= maxWriteRooms) || (r != nil && r.refs >= maxRoomWrites) {
		a.mu.Unlock()
		return nil, errWriteCapacity
	}
	if r == nil {
		r = &writeRoom{active: make(chan struct{}, a.perRoom)}
		a.rooms[conversation] = r
	}
	r.refs++
	a.total++
	a.mu.Unlock()
	forget := func() {
		a.mu.Lock()
		r.refs--
		a.total--
		if r.refs == 0 {
			delete(a.rooms, conversation)
		}
		a.mu.Unlock()
	}
	select {
	case r.active <- struct{}{}:
	case <-ctx.Done():
		forget()
		return nil, ctx.Err()
	}
	select {
	case a.active <- struct{}{}:
	case <-ctx.Done():
		<-r.active
		forget()
		return nil, ctx.Err()
	}
	release := func() {
		<-a.active
		<-r.active
		forget()
	}
	// A ready slot and cancellation can win the same select. Never return an
	// already-cancelled queued call to a caller that could start its transaction.
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}
