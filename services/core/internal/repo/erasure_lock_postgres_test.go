//go:build postgres

package repo

import (
	"context"
	"testing"
	"time"

	"mobile/services/core/internal/testdb"
)

// A reader may already hold the identity lock when account deletion starts.
// Erasure must wait before touching sessions, otherwise the two operations
// deadlock when that reader subsequently validates its session.
func TestErasureLocksIdentityBeforeSessionRevocation(t *testing.T) {
	p := testdb.Pool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var person, session string
	if err := p.QueryRow(ctx, `INSERT INTO people(id,display_name)VALUES(gen_random_uuid(),'Synthetic lock participant')RETURNING id::text`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = p.Exec(context.Background(), `DELETE FROM account_sessions WHERE person_id=$1`, person)
		_, _ = p.Exec(context.Background(), `DELETE FROM people WHERE id=$1`, person)
	})
	if err := p.QueryRow(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at)VALUES(gen_random_uuid(),$1,decode(md5(random()::text)||md5(random()::text),'hex'),'genesis',now()+interval '1 day')RETURNING id::text`, person).Scan(&session); err != nil {
		t.Fatal(err)
	}
	reader, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback(context.Background())
	if _, err = reader.Exec(ctx, `SELECT id FROM people WHERE id=$1 FOR SHARE`, person); err != nil {
		t.Fatal(err)
	}
	eraser, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer eraser.Rollback(context.Background())
	eraserPID := eraser.Conn().PgConn().PID()
	erased := make(chan error, 1)
	go func() { _, err := (Repository{Q: eraser}).ErasePerson(ctx, person, time.Now()); erased <- err }()
	// Observe the real blocked statement, instead of inferring it from a sleep.
	var waiting bool
	for !waiting {
		if err = p.QueryRow(ctx, `SELECT coalesce(wait_event_type='Lock',false) FROM pg_stat_activity WHERE pid=$1`, eraserPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if !waiting {
			select {
			case err = <-erased:
				t.Fatalf("erasure escaped identity lock: %v", err)
			case <-time.After(time.Millisecond):
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	sessionCtx, sessionCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer sessionCancel()
	if _, err = reader.Exec(sessionCtx, `SELECT id FROM account_sessions WHERE id=$1 FOR SHARE`, session); err != nil {
		t.Fatalf("erasure locked sessions before identity: %v", err)
	}
	if err = reader.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-erased:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var revoked bool
	if err = eraser.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM account_sessions WHERE id=$1`, session).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("erasure failed to revoke after reader released: %v", err)
	}
}
