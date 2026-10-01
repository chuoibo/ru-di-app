//go:build postgres

package jobs

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"mobile/services/core/internal/testdb"
)

// Nghe runs a pass when it starts listening, one pass for a burst of
// notifications, passes back to back while one asks for another, a pass on
// the safety poll, and listens again (with a pass) after its connection is
// killed.
func TestNgheChayTheoThongBao(t *testing.T) {
	pool := testdb.Pool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	kenh := fmt.Sprintf("nghe_test_%d", time.Now().UnixNano())
	var luot, lai atomic.Int32
	ran := make(chan struct{}, 64)
	ng := Nghe{Pool: pool, Kenh: kenh, ToiDa: time.Hour, Gop: 300 * time.Millisecond,
		Chay: func(context.Context) (bool, error) {
			luot.Add(1)
			ran <- struct{}{}
			return lai.Add(-1) >= 0, nil
		}}
	done := make(chan struct{})
	go func() { defer close(done); ng.Run(ctx) }()
	cho := func(what string) {
		t.Helper()
		select {
		case <-ran:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: no pass", what)
		}
	}
	yen := func(what string, d time.Duration) {
		t.Helper()
		select {
		case <-ran:
			t.Fatalf("%s: an extra pass", what)
		case <-time.After(d):
		}
	}
	cho("on listening")
	yen("idle", 200*time.Millisecond)

	for i := 0; i < 5; i++ {
		if _, err := pool.Exec(ctx, `SELECT pg_notify($1, 'x')`, kenh); err != nil {
			t.Fatal(err)
		}
	}
	cho("a burst")
	yen("a burst gathered", 600*time.Millisecond)

	lai.Store(2)
	if _, err := pool.Exec(ctx, `SELECT pg_notify($1, 'x')`, kenh); err != nil {
		t.Fatal(err)
	}
	cho("a backlog, first")
	cho("a backlog, second")
	cho("a backlog, third")
	yen("a backlog drained", 400*time.Millisecond)

	var killed int
	if err := pool.QueryRow(ctx, `SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity
		WHERE query = $1 AND pid <> pg_backend_pid()`, `LISTEN "`+kenh+`"`).Scan(&killed); err != nil || killed != 1 {
		t.Fatalf("killing the listener: %d, %v", killed, err)
	}
	cho("listening again")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not end with its context")
	}

	// The safety poll runs a pass with no notification at all.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	ng.Kenh, ng.ToiDa = kenh+"_poll", 300*time.Millisecond
	lai.Store(0)
	go ng.Run(ctx2)
	cho("on listening, poll")
	cho("the safety poll")
}
