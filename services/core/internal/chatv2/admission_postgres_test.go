package chatv2

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresQueuedWritersDoNotStarveOtherConversation(t *testing.T) {
	for _, size := range []int32{4, 15} {
		t.Run(fmt.Sprint(size), func(t *testing.T) { queuedWritersDoNotStarveOtherConversation(t, size) })
	}
}

func queuedWritersDoNotStarveOtherConversation(t *testing.T, poolSize int32) {
	f := setup(t, "group")
	ctx := context.Background()
	other, member := id(), id()
	mustExec(t, f.pool, `INSERT INTO contexts(id,display_name,created_by_id,kind) VALUES($1,'Synthetic independent room',$2,'group')`, other, f.actor)
	mustExec(t, f.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','admin','named')`, member, other, f.actor)
	mustExec(t, f.pool, `INSERT INTO chat_v2_conversations(context_id,epoch,ready) VALUES($1,1,true)`, other)
	mustExec(t, f.pool, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence) VALUES($1,$2,$3,1)`, other, f.device, member)
	cfg := f.pool.Config().Copy()
	cfg.MaxConns = poolSize
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := NewStore(pool)
	blocker, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err = blocker.Exec(ctx, `SELECT context_id FROM chat_v2_conversations WHERE context_id=$1 FOR UPDATE`, f.conversation); err != nil {
		t.Fatal(err)
	}
	queued, cancel := context.WithTimeout(ctx, 10*time.Second)
	var wg sync.WaitGroup
	defer func() { cancel(); _ = blocker.Rollback(ctx); wg.Wait() }()
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		envelope := f.envelope()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Send(queued, f.actor, envelope)
			results <- err
		}()
	}
	waitAdmitted(t, s.writes, 16)
	// A room blocked on a real PostgreSQL row lock must not occupy every
	// connection. A different room can read and write within its own deadline.
	independent, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	if _, err = s.Events(independent, f.actor, f.device, other, 0, 100); err != nil {
		t.Fatalf("blocked writers starved independent reader: %v", err)
	}
	envelope := f.envelope()
	envelope.ConversationID = other
	envelope = sign(f.key, envelope)
	if _, err = s.Send(independent, f.actor, envelope); err != nil {
		t.Fatalf("blocked writers starved independent writer: %v", err)
	}
	cancel()
	wg.Wait()
	for i := 0; i < 16; i++ {
		if err := <-results; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled queued write returned %v", err)
		}
	}
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_events WHERE context_id=$1`, f.conversation).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cancelled writes committed later: count=%d err=%v", count, err)
	}
}

func TestPostgresQueuedWriteRechecksRevocation(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	release := holdAdmissionRoom(t, f.store.writes, f.conversation)
	queued, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := f.store.Send(queued, f.actor, f.envelope()); done <- err }()
	waitAdmitted(t, f.store.writes, f.store.writes.perRoom+1)
	mustExec(t, f.pool, `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE id=$1`, f.membership)
	release()
	if err := <-done; !errors.Is(err, ErrForbidden) {
		t.Fatalf("queued write used an old membership grant: %v", err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("revoked queued write persisted: count=%d err=%v", count, err)
	}
}

func TestPostgresQueuedMutationsRecheckSession(t *testing.T) {
	for _, revoke := range []string{"revoked_at=clock_timestamp()", "expires_at=clock_timestamp()-interval '1 second'"} {
		t.Run(revoke, func(t *testing.T) {
			f := setup(t, "group")
			ctx := context.Background()
			mustExec(t, f.pool, `CREATE TABLE account_sessions (LIKE public.account_sessions INCLUDING ALL)`)
			digest := sha256.Sum256([]byte("synthetic queued mutation session"))
			mustExec(t, f.pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,created_at,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()-interval '1 day',clock_timestamp()+interval '1 hour')`, id(), f.actor, digest[:])
			if _, err := f.store.SendSession(ctx, f.actor, digest[:], f.envelope()); err != nil {
				t.Fatal(err)
			}
			release := holdAdmissionRoom(t, f.store.writes, f.conversation)
			queued, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			done := make(chan error, 2)
			go func() { _, err := f.store.SendSession(queued, f.actor, digest[:], f.envelope()); done <- err }()
			go func() {
				_, err := f.store.MarkSession(queued, f.actor, digest[:], f.device, f.conversation, "read", 1)
				done <- err
			}()
			waitAdmitted(t, f.store.writes, f.store.writes.perRoom+2)
			mustExec(t, f.pool, `UPDATE account_sessions SET `+revoke+` WHERE token_digest=$1`, digest[:])
			release()
			for i := 0; i < 2; i++ {
				if err := <-done; !errors.Is(err, ErrForbidden) {
					t.Fatalf("queued mutation retained an old bearer grant: %v", err)
				}
			}
			var events, marks int
			if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM chat_v2_events),(SELECT count(*) FROM chat_v2_marks)`).Scan(&events, &marks); err != nil || events != 1 || marks != 0 {
				t.Fatalf("revoked mutation persisted: events=%d marks=%d err=%v", events, marks, err)
			}
		})
	}
}

func holdAdmissionRoom(t *testing.T, admission *writeAdmission, room string) func() {
	t.Helper()
	releases := []func(){}
	for i := 0; i < admission.perRoom; i++ {
		release, err := admission.acquire(context.Background(), room)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	return func() {
		for _, release := range releases {
			release()
		}
	}
}
