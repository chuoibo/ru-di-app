package chatv2

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func waitAdmitted(t *testing.T, a *writeAdmission, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		got := a.total
		a.mu.Unlock()
		if got == count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("admission did not reach the expected bounded count")
}

func TestWriteAdmissionSeparateRoomsAndCancellation(t *testing.T) {
	a := newWriteAdmission(4)
	room := id()
	release, err := a.acquire(context.Background(), room)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		release, err := a.acquire(ctx, room)
		if release != nil {
			release()
		}
		done <- err
	}()
	waitAdmitted(t, a, 2)
	other, err := a.acquire(context.Background(), id())
	if err != nil {
		t.Fatal("one queued room blocked another room")
	}
	other()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled queue acquired: %v", err)
	}
	release()
	if a.total != 0 || len(a.rooms) != 0 || len(a.active) != 0 {
		t.Fatal("cancelled admission retained resources")
	}
}

func TestWriteAdmissionBoundsRoomsAndTotalRequests(t *testing.T) {
	a := newWriteAdmission(2)
	room := id()
	first, err := a.acquire(context.Background(), room)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	queue := func(room string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := a.acquire(ctx, room)
			if release != nil {
				release()
				t.Error("queued request ran after cancellation")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("unexpected admission result: %v", err)
			}
		}()
	}
	rooms := []string{room}
	for i := 1; i < maxWriteRooms; i++ {
		rooms = append(rooms, id())
		queue(rooms[i])
	}
	waitAdmitted(t, a, maxWriteRooms)
	if _, err := a.acquire(ctx, id()); !errors.Is(err, errWriteCapacity) {
		t.Fatal("unbounded conversation map")
	}
	for _, room := range rooms {
		queue(room)
	}
	waitAdmitted(t, a, maxAdmittedWrites)
	if _, err := a.acquire(ctx, room); !errors.Is(err, errWriteCapacity) {
		t.Fatal("unbounded total requests")
	}
	if _, err := a.acquire(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid identifiers entered admission")
	}
	cancel()
	wg.Wait()
	first()
	if a.total != 0 || len(a.rooms) != 0 || len(a.active) != 0 {
		t.Fatal("cancelled queue retained map or active slots")
	}
}

func TestWriteAdmissionBoundsOneRoom(t *testing.T) {
	a := newWriteAdmission(4)
	room := id()
	first, err := a.acquire(context.Background(), room)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i := 1; i < maxRoomWrites; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, _ := a.acquire(ctx, room)
			if release != nil {
				release()
				t.Error("queued request ran after cancellation")
			}
		}()
	}
	waitAdmitted(t, a, maxRoomWrites)
	if _, err := a.acquire(ctx, room); !errors.Is(err, errWriteCapacity) {
		t.Fatal("unbounded room queue")
	}
	cancel()
	wg.Wait()
	first()
	if a.total != 0 || len(a.rooms) != 0 || len(a.active) != 0 {
		t.Fatal("room queue did not release memory")
	}
}
