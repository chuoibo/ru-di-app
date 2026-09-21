//go:build postgres

package chatlegacychange

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
)

type peerReadBarrier struct {
	peer             string
	entered, release chan struct{}
	once             sync.Once
	unblockOnce      sync.Once
}

func (b *peerReadBarrier) unblock() { b.unblockOnce.Do(func() { close(b.release) }) }

func (b *peerReadBarrier) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(q.SQL, "SELECT id FROM people") && len(q.Args) > 0 && fmt.Sprint(q.Args[0]) == b.peer {
		b.once.Do(func() {
			close(b.entered)
			select {
			case <-b.release:
			case <-ctx.Done():
			}
		})
	}
	return ctx
}
func (*peerReadBarrier) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func pausedPair(t *testing.T) (world, Store, *peerReadBarrier) {
	t.Helper()
	w := setup(t)
	exec(t, w.pool, `UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, w.room, w.people[0]+":"+w.people[1])
	b := &peerReadBarrier{peer: w.people[1], entered: make(chan struct{}), release: make(chan struct{})}
	config := w.pool.Config().Copy()
	config.ConnConfig.Tracer = b
	pool, err := pgxpool.NewWithConfig(bg, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return w, Store{Pool: pool}, b
}

func TestPostgresSnapshotDoesNotDeadlockPeerErasureOrReadFutureData(t *testing.T) {
	w, reader, barrier := pausedPair(t)
	old := w.message(t)
	ctx, cancel := context.WithTimeout(bg, 5*time.Second)
	defer cancel()
	defer barrier.unblock()
	type answer struct {
		value Snapshot
		err   error
	}
	done := make(chan answer, 1)
	go func() { v, e := reader.Snapshot(ctx, w.headers[0], w.room, SnapshotRequest{}); done <- answer{v, e} }()
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal("reader did not establish snapshot")
	}
	// Account erasure locks the person before changing their memberships. A
	// reader that locks the peer membership first creates a real lock cycle.
	writeCtx, stopWrite := context.WithTimeout(ctx, time.Second)
	defer stopWrite()
	tx, err := w.pool.Begin(writeCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(bg)
	if _, err = tx.Exec(writeCtx, `UPDATE people SET deleted_at=clock_timestamp() WHERE id=$1`, w.people[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(writeCtx, `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE person_id=$1 AND context_id=$2`, w.people[1], w.room); err != nil {
		t.Fatalf("snapshot blocked peer erasure: %v", err)
	}
	if err = tx.Commit(writeCtx); err != nil {
		t.Fatal(err)
	}
	future := w.message(t)
	barrier.unblock()
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if len(got.value.Messages) != 1 || snapshotObject(t, got.value.Messages[0])["id"] != old.ID {
		t.Fatalf("snapshot crossed revocation boundary; future=%s", future.ID)
	}
	if _, err = w.store.Snapshot(ctx, w.headers[0], w.room, SnapshotRequest{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("fresh read accepted deleted peer: %v", err)
	}
}

func TestPostgresSnapshotRechecksSessionExpiryAtHandoff(t *testing.T) {
	w, reader, barrier := pausedPair(t)
	_ = w.message(t)
	exec(t, w.pool, `UPDATE account_sessions SET created_at=clock_timestamp()-interval '1 day',expires_at=clock_timestamp()+interval '500 milliseconds' WHERE token_digest=$1`, auth.TokenDigest(w.tokens[0]))
	ctx, cancel := context.WithTimeout(bg, 5*time.Second)
	defer cancel()
	defer barrier.unblock()
	done := make(chan error, 1)
	go func() { _, err := reader.Snapshot(ctx, w.headers[0], w.room, SnapshotRequest{}); done <- err }()
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal("reader did not establish snapshot")
	}
	for {
		var expired bool
		if err := w.pool.QueryRow(ctx, `SELECT expires_at<=clock_timestamp() FROM account_sessions WHERE token_digest=$1`, auth.TokenDigest(w.tokens[0])).Scan(&expired); err != nil {
			t.Fatal(err)
		}
		if expired {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	barrier.unblock()
	if err := <-done; !errors.Is(err, ErrAuthentication) {
		t.Fatalf("expired read handed off: %v", err)
	}
}
