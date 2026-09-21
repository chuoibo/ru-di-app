package chatv2

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

type fixture struct {
	pool                                                        *pgxpool.Pool
	store                                                       *Store
	actor, other, conversation, device, otherDevice, membership string
	key                                                         ed25519.PrivateKey
}

func id() string {
	var v [16]byte
	_, _ = rand.Read(v[:])
	s := hex.EncodeToString(v[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func setup(t *testing.T, kind string) fixture {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "chatv2_test_" + hex.EncodeToString([]byte(id()))[:20]
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	config.MaxConns = 30
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests"} {
		if _, err = pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{pool: pool, store: NewStore(pool), actor: id(), other: id(), conversation: id(), device: id(), otherDevice: id(), membership: id(), key: key}
	sql := `INSERT INTO people(id,display_name) VALUES($1,'Synthetic A'),($2,'Synthetic B')`
	if _, err = pool.Exec(ctx, sql, f.actor, f.other); err != nil {
		t.Fatal(err)
	}
	var pair any
	if kind == "pair" {
		pair = f.actor + ":" + f.other
	}
	if _, err = pool.Exec(ctx, `INSERT INTO contexts(id,display_name,created_by_id,kind,pair_key) VALUES($1,'Synthetic lab',$2,$3,$4)`, f.conversation, f.actor, kind, pair); err != nil {
		t.Fatal(err)
	}
	otherMembership := id()
	if _, err = pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','admin','named'),($4,$2,$5,'active','member','named')`, f.membership, f.conversation, f.actor, otherMembership, f.other); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO chat_v2_devices(id,person_id,signing_key) VALUES($1,$2,$3),($4,$5,$3)`, f.device, f.actor, []byte(pub), f.otherDevice, f.other); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO chat_v2_conversations(context_id,epoch,ready) VALUES($1,1,true)`, f.conversation); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence) VALUES($1,$2,$3,1),($1,$4,$5,1)`, f.conversation, f.device, f.membership, f.otherDevice, otherMembership); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f fixture) envelope() Envelope {
	e := Envelope{ConversationID: f.conversation, DeviceID: f.device, LogicalSendID: id(), Protocol: Protocol, Epoch: 1, Ciphertext: []byte("synthetic opaque test payload; not MLS evidence")}
	b, _ := SigningBytes(e)
	e.Signature = ed25519.Sign(f.key, b)
	return e
}
func sign(key ed25519.PrivateKey, e Envelope) Envelope {
	b, _ := SigningBytes(e)
	e.Signature = ed25519.Sign(key, b)
	return e
}
func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestSigningBindsEnvelope(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	e := Envelope{ConversationID: id(), DeviceID: id(), LogicalSendID: id(), Protocol: Protocol, Epoch: 1, Ciphertext: []byte{1, 2, 3}}
	e = sign(key, e)
	original, _ := SigningBytes(e)
	for name, mutate := range map[string]func(*Envelope){"conversation": func(v *Envelope) { v.ConversationID = id() }, "device": func(v *Envelope) { v.DeviceID = id() }, "logical": func(v *Envelope) { v.LogicalSendID = id() }, "epoch": func(v *Envelope) { v.Epoch++ }, "ciphertext": func(v *Envelope) { v.Ciphertext = []byte{4} }} {
		t.Run(name, func(t *testing.T) {
			changed := e
			mutate(&changed)
			b, err := SigningBytes(changed)
			if err != nil {
				t.Fatal(err)
			}
			if ed25519.Verify(key.Public().(ed25519.PublicKey), b, e.Signature) {
				t.Fatal("signature accepted mutation")
			}
		})
	}
	if !ed25519.Verify(key.Public().(ed25519.PublicKey), original, e.Signature) {
		t.Fatal("original signature rejected")
	}
}
func TestEnvelopeBounds(t *testing.T) {
	e := Envelope{ConversationID: id(), DeviceID: id(), LogicalSendID: id(), Protocol: Protocol, Epoch: 1, Ciphertext: []byte{1}}
	cases := []func(*Envelope){func(v *Envelope) { v.Epoch = 0 }, func(v *Envelope) { v.Protocol = "plaintext" }, func(v *Envelope) { v.Ciphertext = nil }, func(v *Envelope) { v.Ciphertext = make([]byte, MaxCiphertext+1) }, func(v *Envelope) { v.DeviceID = "bad" }}
	for _, mutate := range cases {
		v := e
		mutate(&v)
		if _, err := SigningBytes(v); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected invalid: %v", err)
		}
	}
}

func TestSendConcurrentReplayConflictAndCatchup(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	e := f.envelope()
	const n = 20
	var wg sync.WaitGroup
	results := make(chan SendResult, n)
	errs := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() { defer wg.Done(); result, err := f.store.Send(ctx, f.actor, e); results <- result; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	created := 0
	for result := range results {
		if result.Event.Sequence != 1 {
			t.Fatal(result)
		}
		if !result.Replayed {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created=%d", created)
	}
	var events, outbox, sends int
	if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM chat_v2_events),(SELECT count(*) FROM chat_v2_outbox),(SELECT count(*) FROM chat_v2_sends)`).Scan(&events, &outbox, &sends); err != nil {
		t.Fatal(err)
	}
	if events != 1 || outbox != 1 || sends != 1 {
		t.Fatalf("events=%d outbox=%d sends=%d", events, outbox, sends)
	}
	changed := e
	changed.Ciphertext = []byte("different")
	changed = sign(f.key, changed)
	if _, err := f.store.Send(ctx, f.actor, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	for range 3 {
		if _, err := f.store.Send(ctx, f.actor, f.envelope()); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.store.Events(ctx, f.other, f.otherDevice, f.conversation, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || !page.HasMore || page.NextSequence != 2 {
		t.Fatal(page)
	}
	page, err = f.store.Events(ctx, f.other, f.otherDevice, f.conversation, page.NextSequence, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.HasMore || page.NextSequence != 4 {
		t.Fatal(page)
	}
}

func TestConcurrentUniqueSendsOrdered(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	const n = 24
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.store.Send(ctx, f.actor, f.envelope()); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := f.store.Events(ctx, f.actor, f.device, f.conversation, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Events) != n {
		t.Fatal(len(p.Events))
	}
	for i, e := range p.Events {
		if e.Sequence != int64(i+1) {
			t.Fatal(e.Sequence)
		}
	}
}

func TestAuthorizationBeforeReplayAndRekey(t *testing.T) {
	for _, mode := range []string{"membership", "device", "account", "block", "other_member"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t, "pair")
			ctx := context.Background()
			e := f.envelope()
			if _, err := f.store.Send(ctx, f.actor, e); err != nil {
				t.Fatal(err)
			}
			want := ErrForbidden
			switch mode {
			case "membership":
				mustExec(t, f.pool, `UPDATE memberships SET state='left',left_at=now() WHERE id=$1`, f.membership)
			case "device":
				mustExec(t, f.pool, `UPDATE chat_v2_devices SET revoked_at=now() WHERE id=$1`, f.device)
			case "account":
				mustExec(t, f.pool, `UPDATE people SET deleted_at=now() WHERE id=$1`, f.actor)
			case "block":
				mustExec(t, f.pool, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_at,decided_by_id) VALUES($1,$2,$3,'blocked',now(),$2)`, id(), f.other, f.actor)
			case "other_member":
				mustExec(t, f.pool, `UPDATE memberships SET state='left',left_at=now() WHERE context_id=$1 AND person_id=$2`, f.conversation, f.other)
				want = ErrNotReady
			}
			if _, err := f.store.Send(ctx, f.actor, e); !errors.Is(err, want) {
				t.Fatalf("replay: %v want %v", err, want)
			}
			if _, err := f.store.Events(ctx, f.actor, f.device, f.conversation, 0, 10); !errors.Is(err, want) {
				t.Fatalf("read: %v", err)
			}
			if _, err := f.store.Mark(ctx, f.actor, f.device, f.conversation, "read", 1); !errors.Is(err, want) {
				t.Fatalf("mark: %v", err)
			}
		})
	}
}

func TestEpochForgeryAndHistoryBoundary(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	e := f.envelope()
	changed := e
	changed.Ciphertext = []byte("forged")
	if _, err := f.store.Send(ctx, f.actor, changed); !errors.Is(err, ErrForbidden) {
		t.Fatalf("forged: %v", err)
	}
	if _, err := f.store.Send(ctx, f.other, e); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong device owner: %v", err)
	}
	if _, err := f.store.Send(ctx, f.actor, e); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.pool, `UPDATE chat_v2_conversations SET epoch=2 WHERE context_id=$1`, f.conversation)
	if _, err := f.store.Send(ctx, f.actor, f.envelope()); !errors.Is(err, ErrEpoch) {
		t.Fatalf("stale epoch: %v", err)
	}
	if replay, err := f.store.Send(ctx, f.actor, e); err != nil || !replay.Replayed {
		t.Fatalf("prior acknowledged replay: %v", err)
	}
	fresh := f.envelope()
	fresh.Epoch = 2
	fresh = sign(f.key, fresh)
	if _, err := f.store.Send(ctx, f.actor, fresh); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.pool, `UPDATE chat_v2_members SET first_sequence=2 WHERE context_id=$1 AND device_id=$2`, f.conversation, f.otherDevice)
	page, err := f.store.Events(ctx, f.other, f.otherDevice, f.conversation, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Sequence != 2 {
		t.Fatal(page)
	}
}

func TestMonotonicMarksAndBounds(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	for range 20 {
		if _, err := f.store.Send(ctx, f.actor, f.envelope()); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func(seq int64) {
			defer wg.Done()
			_, err := f.store.Mark(ctx, f.other, f.otherDevice, f.conversation, "read", seq)
			errs <- err
		}(int64(i + 1))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	mark, err := f.store.Mark(ctx, f.other, f.otherDevice, f.conversation, "delivered", 1)
	if err != nil {
		t.Fatal(err)
	}
	if mark.Read != 20 || mark.Delivered != 20 {
		t.Fatal(mark)
	}
	if _, err = f.store.Mark(ctx, f.other, f.otherDevice, f.conversation, "read", 1000); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, limit := range []int{0, 101} {
		if _, err = f.store.Events(ctx, f.actor, f.device, f.conversation, 0, limit); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err = f.store.Events(ctx, f.actor, f.device, f.conversation, 1000, 10); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestOutboxFailureRollsBackSend(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	mustExec(t, f.pool, `ALTER TABLE chat_v2_outbox ADD CONSTRAINT synthetic_failure CHECK(sequence<0)`)
	e := f.envelope()
	if _, err := f.store.Send(ctx, f.actor, e); err == nil {
		t.Fatal("failure injection did not fail")
	}
	var count int
	var seq int64
	if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM chat_v2_events),last_sequence FROM chat_v2_conversations WHERE context_id=$1`, f.conversation).Scan(&count, &seq); err != nil {
		t.Fatal(err)
	}
	if count != 0 || seq != 0 {
		t.Fatalf("partial commit count=%d seq=%d", count, seq)
	}
	mustExec(t, f.pool, `ALTER TABLE chat_v2_outbox DROP CONSTRAINT synthetic_failure`)
	result, err := f.store.Send(ctx, f.actor, e)
	if err != nil || result.Event.Sequence != 1 {
		t.Fatalf("retry=%+v err=%v", result, err)
	}
}

func TestCatchupDoesNotSkipUncommittedSequence(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = authorize(ctx, tx, f.actor, f.device, f.conversation); err != nil {
		t.Fatal(err)
	}
	if _, err = appendEvent(ctx, tx, f.conversation, f.actor, "envelope", f.envelope()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		page, err := f.store.Events(ctx, f.other, f.otherDevice, f.conversation, 0, 10)
		if err == nil && (len(page.Events) != 1 || page.NextSequence != 1) {
			err = fmt.Errorf("skipped pending event: %+v", page)
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("reader bypassed uncommitted sequence: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not resume")
	}
}

func TestRejoinCannotReviveOldDeviceMembership(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	e := f.envelope()
	if _, err := f.store.Send(ctx, f.actor, e); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.pool, `UPDATE memberships SET state='left',left_at=now() WHERE id=$1`, f.membership)
	mustExec(t, f.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','member','named')`, id(), f.conversation, f.actor)
	// Even a completed new epoch cannot revive the departed membership binding.
	mustExec(t, f.pool, `UPDATE chat_v2_conversations SET ready=true,epoch=2 WHERE context_id=$1`, f.conversation)
	if _, err := f.store.Send(ctx, f.actor, e); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := f.store.Events(ctx, f.actor, f.device, f.conversation, 0, 10); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}

func TestRevocationWaitsForAuthorizedTransaction(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = authorize(ctx, tx, f.actor, f.device, f.conversation); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		_, err := f.pool.Exec(ctx, `UPDATE memberships SET state='left',left_at=now() WHERE id=$1`, f.membership)
		done <- err
	}()
	<-started
	select {
	case err = <-done:
		t.Fatalf("revocation bypassed authorization lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revocation did not resume")
	}
	if _, err = f.store.Send(ctx, f.actor, f.envelope()); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}

func TestMigrationDetectsDrift(t *testing.T) {
	f := setup(t, "group")
	mustExec(t, f.pool, `UPDATE chat_v2_schema_migrations SET digest='tampered' WHERE version=1`)
	if err := Migrate(context.Background(), f.pool); err == nil {
		t.Fatal("checksum drift accepted")
	}
}

func TestInvalidatedGateStaysClosed(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	mustExec(t, f.pool, `UPDATE chat_v2_conversations SET ready=false WHERE context_id=$1`, f.conversation)
	if _, err := f.store.Send(ctx, f.actor, f.envelope()); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	if _, err := f.store.Events(ctx, f.actor, f.device, f.conversation, 0, 10); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}

func TestCatchupBoundsBytesWithoutSkippingLargeEnvelopes(t *testing.T) {
	f := setup(t, "group")
	ctx := context.Background()
	for range 2 {
		e := f.envelope()
		e.Ciphertext = make([]byte, MaxCiphertext)
		e = sign(f.key, e)
		if _, err := f.store.Send(ctx, f.actor, e); err != nil {
			t.Fatal(err)
		}
	}
	// A small third event must remain reachable after both large pages.
	if _, err := f.store.Send(ctx, f.actor, f.envelope()); err != nil {
		t.Fatal(err)
	}
	page, err := f.store.Events(ctx, f.other, f.otherDevice, f.conversation, 0, MaxPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || !page.HasMore || page.NextSequence != 1 {
		t.Fatalf("first page: %+v", page)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaxPageBytes {
		t.Fatalf("first page exceeds budget: %d", len(encoded))
	}
	next, err := f.store.Events(ctx, f.other, f.otherDevice, f.conversation, page.NextSequence, MaxPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Events) != 2 || next.HasMore || next.NextSequence != 3 || next.Events[0].Sequence != 2 || next.Events[1].Sequence != 3 {
		t.Fatalf("second page sequences/count: %+v", next)
	}
	encoded, err = json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaxPageBytes {
		t.Fatalf("second page exceeds budget: %d", len(encoded))
	}
	// Caller-selected count limits still apply independently of the byte cap.
	single, err := f.store.Events(ctx, f.other, f.otherDevice, f.conversation, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(single.Events) != 1 || !single.HasMore || single.NextSequence != 2 {
		t.Fatal(single.NextSequence, single.HasMore, len(single.Events))
	}
}
