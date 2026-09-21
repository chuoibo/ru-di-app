package chatv2

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCatchupReadersShareConversationWithoutWeakeningRevocation(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	if _, err := f.store.Send(ctx, f.actor, f.envelope()); err != nil {
		t.Fatal(err)
	}
	held, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Rollback(ctx)
	if _, err = authorizeWithLock(ctx, held, f.actor, f.device, f.conversation, false); err != nil {
		t.Fatal(err)
	}
	// A second real catch-up must finish while the first read boundary is held.
	// The former exclusive conversation lock fails this bounded operation.
	readCtx, cancel := context.WithTimeout(ctx, time.Second)
	page, err := f.store.Events(readCtx, f.other, f.otherDevice, f.conversation, 0, 10)
	cancel()
	if err != nil || len(page.Events) != 1 {
		t.Fatalf("parallel catch-up blocked: page=%+v err=%v", page, err)
	}
	revoke, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer revoke.Rollback(ctx)
	var pid int32
	if err = revoke.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := revoke.Exec(ctx, `UPDATE chat_v2_devices SET revoked_at=now() WHERE id=$1`, f.device)
		if err == nil {
			err = revoke.Commit(ctx)
		}
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		if err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("revocation crossed active read transaction: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("revocation never reached its lock boundary")
	}
	if err = held.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("revocation did not finish after read commit")
	}
	if _, err = f.store.Events(ctx, f.actor, f.device, f.conversation, 0, 10); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked reader accepted: %v", err)
	}
	if _, err = f.store.Events(ctx, f.other, f.otherDevice, f.conversation, 0, 10); !errors.Is(err, ErrNotReady) {
		t.Fatalf("other reader accepted invalidated epoch: %v", err)
	}
}
