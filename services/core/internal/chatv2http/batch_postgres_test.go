//go:build postgres

package chatv2http

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
)

func TestPostgresBatchHistorySessionAndMembershipIsolation(t *testing.T) {
	f := liveSetup(t)
	s := chatv2.NewStore(f.pool)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := s.Send(ctx, f.people[0].id, f.envelope(0)); err != nil {
			t.Fatal(err)
		}
	}
	liveExec(t, f.pool, `UPDATE chat_v2_members SET first_sequence=4 WHERE context_id=$1 AND device_id=$2`, f.conversation, f.people[1].device)
	recipients := []chatv2.Recipient{}
	for _, p := range f.people {
		recipients = append(recipients, chatv2.Recipient{ActorID: p.id, DeviceID: p.device, SessionDigest: auth.TokenDigest(p.token), Limit: 100})
	}
	recipients = append(recipients, recipients[0])
	recipients[3].SessionDigest = auth.TokenDigest(f.people[1].token)
	recipients = append(recipients, recipients[0])
	recipients[4].DeviceID = f.people[1].device
	check := func() []chatv2.Delivery {
		t.Helper()
		results, err := s.EventsBatch(ctx, f.conversation, recipients)
		if err != nil {
			t.Fatal(err)
		}
		return results
	}
	results := check()
	if len(results[0].Page.Events) != 5 || len(results[1].Page.Events) != 2 || results[1].Page.Events[0].Sequence != 4 {
		t.Fatal("history boundary was not per recipient")
	}
	for _, i := range []int{3, 4} {
		if !errors.Is(results[i].Err, chatv2.ErrForbidden) {
			t.Fatal("session/device impersonation accepted")
		}
	}
	liveExec(t, f.pool, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, f.people[0].id)
	results = check()
	if !errors.Is(results[0].Err, chatv2.ErrForbidden) || results[1].Err != nil {
		t.Fatal("session revocation was shared or cached")
	}
	liveExec(t, f.pool, `UPDATE memberships SET state='left',left_at=now() WHERE id=$1`, f.people[2].membership)
	liveExec(t, f.pool, `INSERT INTO memberships(id,person_id,context_id,state,role,origin)VALUES($1,$2,$3,'active','member','named')`, newID(), f.people[2].id, f.conversation)
	liveExec(t, f.pool, `UPDATE chat_v2_conversations SET ready=true,epoch=2 WHERE context_id=$1`, f.conversation)
	results = check()
	if !errors.Is(results[2].Err, chatv2.ErrForbidden) || results[1].Err != nil {
		t.Fatal("rejoined identity revived old device membership")
	}
	liveExec(t, f.pool, `UPDATE chat_v2_devices SET revoked_at=now() WHERE id=$1`, f.people[1].device)
	if _, err := s.EventsBatch(ctx, f.conversation, recipients); !errors.Is(err, chatv2.ErrNotReady) {
		t.Fatalf("rekey gate bypassed: %v", err)
	}
	liveExec(t, f.pool, `UPDATE chat_v2_conversations SET ready=true,epoch=3 WHERE context_id=$1`, f.conversation)
	results = check()
	if !errors.Is(results[1].Err, chatv2.ErrForbidden) {
		t.Fatal("revoked device authorized")
	}
}

func TestPostgresBatchByteBoundAndIndependentCursors(t *testing.T) {
	f := liveSetup(t)
	s := chatv2.NewStore(f.pool)
	ctx := context.Background()
	for i := 0; i < 150; i++ {
		if _, err := s.Send(ctx, f.people[0].id, f.envelope(0)); err != nil {
			t.Fatal(err)
		}
	}
	recipients := []chatv2.Recipient{}
	for i, p := range f.people {
		recipients = append(recipients, chatv2.Recipient{ActorID: p.id, DeviceID: p.device, SessionDigest: auth.TokenDigest(p.token), After: int64(i * 50), Limit: 80})
	}
	results, err := s.EventsBatch(ctx, f.conversation, recipients)
	if err != nil {
		t.Fatal(err)
	}
	for i, result := range results {
		if result.Err != nil || result.Page.Events[0].Sequence != int64(i*50+1) || result.Page.NextSequence != min(int64(i*50+80), 150) {
			t.Fatalf("wrong independent cursor %d", i)
		}
	}
}

// The gate pauses after authority has been read, before the ciphertext query.
// It observes a real pgx transaction and never replaces a SQL result.
type snapshotReadGate struct {
	reached, release chan struct{}
	held             atomic.Bool
}

func (g *snapshotReadGate) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.Contains(q.SQL, "FROM chat_v2_events WHERE context_id=$1 AND sequence>$2 ORDER BY sequence") && g.held.CompareAndSwap(false, true) {
		close(g.reached)
		select {
		case <-g.release:
		case <-ctx.Done():
		}
	}
	return ctx
}
func (*snapshotReadGate) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func snapshotStore(t *testing.T, f liveWorld) (*chatv2.Store, *snapshotReadGate) {
	t.Helper()
	g := &snapshotReadGate{reached: make(chan struct{}), release: make(chan struct{})}
	cfg := f.pool.Config().Copy()
	cfg.ConnConfig.Tracer = g
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return chatv2.NewStore(pool), g
}
func TestPostgresBatchRevocationUsesFreshSnapshot(t *testing.T) {
	f := liveSetup(t)
	writer := chatv2.NewStore(f.pool)
	s, gate := snapshotStore(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := writer.Send(ctx, f.people[0].id, f.envelope(0)); err != nil {
		t.Fatal(err)
	}
	p := f.people[0]
	recipients := []chatv2.Recipient{{ActorID: p.id, DeviceID: p.device, SessionDigest: auth.TokenDigest(p.token), Limit: 100}}
	type readResult struct {
		deliveries []chatv2.Delivery
		err        error
	}
	done := make(chan readResult, 1)
	go func() { rows, err := s.EventsBatch(ctx, f.conversation, recipients); done <- readResult{rows, err} }()
	select {
	case <-gate.reached:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Revocation does not wait for the read. A new event then commits while the
	// old read is still paused, exposing any accidental READ COMMITTED usage.
	if _, err := f.pool.Exec(ctx, `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1`, p.id); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Send(ctx, f.people[1].id, f.envelope(1)); err != nil {
		t.Fatal(err)
	}
	close(gate.release)
	select {
	case r := <-done:
		if r.err != nil || r.deliveries[0].Err != nil || len(r.deliveries[0].Page.Events) != 1 || r.deliveries[0].Page.NextSequence != 1 {
			t.Fatalf("snapshot crossed revoke/event boundary: %v", r.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	r, err := s.EventsBatch(ctx, f.conversation, recipients)
	if err != nil || !errors.Is(r[0].Err, chatv2.ErrForbidden) {
		t.Fatalf("fresh post-revoke snapshot admitted session: %v", err)
	}
}
func TestPostgresBatchExpiredSessionCannotUseOldSnapshot(t *testing.T) {
	f := liveSetup(t)
	s, gate := snapshotStore(t, f)
	writer := chatv2.NewStore(f.pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := writer.Send(ctx, f.people[0].id, f.envelope(0)); err != nil {
		t.Fatal(err)
	}
	p := f.people[0]
	if _, err := f.pool.Exec(ctx, `UPDATE account_sessions SET expires_at=clock_timestamp()+interval '200 milliseconds' WHERE person_id=$1`, p.id); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		r, err := s.EventsBatch(ctx, f.conversation, []chatv2.Recipient{{ActorID: p.id, DeviceID: p.device, SessionDigest: auth.TokenDigest(p.token), Limit: 100}})
		if err == nil && !errors.Is(r[0].Err, chatv2.ErrForbidden) {
			err = errors.New("expired session received a snapshot page")
		}
		done <- err
	}()
	select {
	case <-gate.reached:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	expired := false
	for !expired {
		if err := f.pool.QueryRow(ctx, `SELECT expires_at<=clock_timestamp() FROM account_sessions WHERE person_id=$1`, p.id).Scan(&expired); err != nil {
			t.Fatal(err)
		}
		if !expired {
			select {
			case <-time.After(5 * time.Millisecond):
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	close(gate.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
