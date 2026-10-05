//go:build postgres

package push

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/testdb"
)

// Push on a real PostgreSQL: registration rules, the wake queue fed from the
// chat v2 log, the content-free payload, and revocation on sign-out and
// account deletion. Synthetic data only.

type world struct {
	store              Store
	room               string
	an, binh           string
	anToken, binhToken []byte
	binhSession        string
}

func id(t *testing.T, s Store) string {
	var v string
	if err := s.Pool.QueryRow(context.Background(), `SELECT gen_random_uuid()::text`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func setup(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	// A schema of its own: migrating chat v2 and push into the shared
	// database's public schema leaked their tables and triggers into every
	// other package's tests in the same run (chatlegacychange read this
	// package's chat_v2_conversations through a cached plan).
	base := testdb.Pool(t)
	var raw string
	if err := base.QueryRow(ctx, `SELECT replace(gen_random_uuid()::text,'-','')`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	schema := "push_test_" + raw[:20]
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests", "account_sessions"} {
		if _, err = pool.Exec(ctx, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
	}
	if err := chatv2.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	w := world{store: Store{Pool: pool}}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	w.an, w.binh, w.room = id(t, w.store), id(t, w.store), id(t, w.store)
	exec(`INSERT INTO people(id,display_name) VALUES($1,'An (dữ liệu mẫu)'),($2,'Bình (dữ liệu mẫu)')`, w.an, w.binh)
	exec(`INSERT INTO contexts(id,display_name,created_by_id,kind) VALUES($1,'Phòng thử',$2,'group')`, w.room, w.an)
	exec(`INSERT INTO memberships(id,context_id,person_id,state,role,origin,created_at) VALUES(gen_random_uuid(),$1,$2,'active','admin','named',now()),(gen_random_uuid(),$1,$3,'active','member','named',now())`, w.room, w.an, w.binh)
	session := func(person, name string) []byte {
		d := sha256.Sum256([]byte(name + w.room))
		exec(`INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES(gen_random_uuid(),$1,$2,'genesis',now()+interval '1 hour')`, person, d[:])
		return d[:]
	}
	w.anToken, w.binhToken = session(w.an, "an"), session(w.binh, "binh")
	_ = pool.QueryRow(ctx, `SELECT id::text FROM account_sessions WHERE token_digest=$1`, w.binhToken).Scan(&w.binhSession)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM push_outbox WHERE conversation_id=$1`, w.room)
		_, _ = pool.Exec(bg, `DELETE FROM push_devices WHERE person_id IN ($1,$2)`, w.an, w.binh)
		_, _ = pool.Exec(bg, `DELETE FROM push_chat_cursor WHERE conversation_id=$1`, w.room)
	})
	return w
}

func token(n string) string { return "ExponentPushToken[" + strings.Repeat(n, 22) + "]" }

func TestRegistrationRulesAndRevocation(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	inst := id(t, w.store)
	if _, err := w.store.Register(ctx, w.binhToken, Registration{InstallationID: inst, Platform: "android", Token: token("b")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Register(ctx, w.binhToken, Registration{InstallationID: inst, Platform: "android", Token: token("c")}); err != nil {
		t.Fatalf("the same installation moves to a new token: %v", err)
	}
	if _, err := w.store.Register(ctx, w.anToken, Registration{InstallationID: id(t, w.store), Platform: "ios", Token: token("c")}); err != ErrConflict {
		t.Fatalf("a token changed owner between installations: %v", err)
	}
	for _, bad := range []Registration{{InstallationID: "x", Platform: "android", Token: token("d")}, {InstallationID: id(t, w.store), Platform: "web", Token: token("d")}, {InstallationID: id(t, w.store), Platform: "ios", Token: "not-a-token"}} {
		if _, err := w.store.Register(ctx, w.anToken, bad); err != ErrInvalid {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if err := w.store.Unregister(ctx, w.anToken, inst); err != ErrForbidden {
		t.Fatalf("An unregistered Bình's installation: %v", err)
	}
	// Signing out (the session revoked) stops the device.
	if _, err := w.store.Pool.Exec(ctx, `UPDATE account_sessions SET revoked_at=now() WHERE id=$1`, w.binhSession); err != nil {
		t.Fatal(err)
	}
	var revoked bool
	_ = w.store.Pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM push_devices WHERE installation_id=$1`, inst).Scan(&revoked)
	if !revoked {
		t.Fatal("a revoked session's device still receives pushes")
	}
}

type recorder struct {
	mu   sync.Mutex
	sent []Message
	fail bool
}

func (r *recorder) Send(_ context.Context, m []Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return io.ErrUnexpectedEOF
	}
	r.sent = append(r.sent, m...)
	return nil
}

func TestAChatEventWakesTheOtherMembersWithoutContent(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	if _, err := w.store.Register(ctx, w.binhToken, Registration{InstallationID: id(t, w.store), Platform: "android", Token: token("e")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Register(ctx, w.anToken, Registration{InstallationID: id(t, w.store), Platform: "ios", Token: token("f")}); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		if _, err := w.store.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO chat_v2_conversations(context_id,epoch,ready,last_sequence) VALUES($1,1,true,2)`, w.room)
	secret := "Bình ơi, mật khẩu wifi là"
	exec(`INSERT INTO chat_v2_events(context_id,sequence,kind,actor_id,body) VALUES($1,1,'envelope',$2,$3),($1,2,'envelope',$2,$3)`, w.room, w.an, `{"ciphertext":"`+secret+`"}`)
	t.Cleanup(func() {
		_, _ = w.store.Pool.Exec(context.Background(), `DELETE FROM chat_v2_events WHERE context_id=$1`, w.room)
		_, _ = w.store.Pool.Exec(context.Background(), `DELETE FROM chat_v2_conversations WHERE context_id=$1`, w.room)
	})
	if _, err := w.store.Enqueue(ctx, 1000); err != nil {
		t.Fatal(err)
	}
	failing := &recorder{fail: true}
	if _, err := w.store.SendPending(ctx, failing, 100); err == nil {
		t.Fatal("a failed hand-off reported success")
	}
	// The failed wake is retried once its lease lapses.
	exec(`UPDATE push_outbox SET leased_until=now()-interval '1 second' WHERE conversation_id=$1`, w.room)
	r := &recorder{}
	if _, err := w.store.SendPending(ctx, r, 100); err != nil {
		t.Fatal(err)
	}
	if len(r.sent) != 1 || r.sent[0].To != token("e") || r.sent[0].Data["c"] != w.room || r.sent[0].Data["s"] != "2" {
		t.Fatalf("one coalesced wake to Bình alone: %+v", r.sent)
	}
	raw, _ := json.Marshal(r.sent)
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "An (") || r.sent[0].Body != WakeText {
		t.Fatalf("the push says more than a wake: %s", raw)
	}
	again := &recorder{}
	if _, err := w.store.Enqueue(ctx, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.SendPending(ctx, again, 100); err != nil || len(again.sent) != 0 {
		t.Fatalf("a sent wake went again: %+v %v", again.sent, err)
	}
	// Account deletion erases the person's devices and queue.
	exec(`UPDATE people SET deleted_at=now() WHERE id=$1`, w.binh)
	var left int
	_ = w.store.Pool.QueryRow(ctx, `SELECT count(*) FROM push_devices WHERE person_id=$1`, w.binh).Scan(&left)
	if left != 0 {
		t.Fatalf("a deleted account keeps %d push devices", left)
	}
}

func TestTheExpoSenderPostsOnlyTheWake(t *testing.T) {
	var got []Message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer synthetic-access" {
			t.Errorf("access token: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	sender := ExpoSender{URL: server.URL, AccessToken: "synthetic-access"}
	msg := Message{To: token("g"), Title: "Rủ Đi", Body: WakeText, Priority: "high", Sound: "default", Data: map[string]string{"t": "chat", "c": "room", "s": "1"}}
	if err := sender.Send(context.Background(), []Message{msg}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Data["t"] != "chat" || got[0].Body != WakeText {
		t.Fatalf("posted: %+v", got)
	}
}

// Security review 05/10: a live installation of another person is never taken
// over; a revoked one (signed out on this phone) is; a reinstalled app gets
// its token back from its revoked installation.
func TestAnInstallationIsNeverTakenFromItsLivePerson(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	inst := id(t, w.store)
	if _, err := w.store.Register(ctx, w.binhToken, Registration{InstallationID: inst, Platform: "android", Token: token("h")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Register(ctx, w.anToken, Registration{InstallationID: inst, Platform: "android", Token: token("i")}); err != ErrConflict {
		t.Fatalf("An took Bình's live installation: %v", err)
	}
	if err := w.store.Unregister(ctx, w.binhToken, inst); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Register(ctx, w.anToken, Registration{InstallationID: inst, Platform: "android", Token: token("i")}); err != nil {
		t.Fatalf("An signing in on the phone Bình signed out of: %v", err)
	}
	if err := w.store.Unregister(ctx, w.anToken, inst); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Register(ctx, w.anToken, Registration{InstallationID: id(t, w.store), Platform: "android", Token: token("i")}); err != nil {
		t.Fatalf("a reinstall gets its token back from the revoked installation: %v", err)
	}
}

// Security review 05/10: one wake whose hand-off fails does not fail the
// others, and an expired session's device is not woken.
func TestOneBadHandOffFailsOnlyItsWakeAndExpiredSessionsSleep(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := w.store.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.store.Register(ctx, w.binhToken, Registration{InstallationID: id(t, w.store), Platform: "android", Token: token("j")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Register(ctx, w.anToken, Registration{InstallationID: id(t, w.store), Platform: "ios", Token: token("k")}); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO push_outbox(id,person_id,conversation_id,sequence) VALUES(gen_random_uuid(),$1,$3,1),(gen_random_uuid(),$2,$3,1)`, w.an, w.binh, w.room)
	r := &picky{refuse: token("k")}
	if _, err := w.store.SendPending(ctx, r, 100); err == nil {
		t.Fatal("the refused hand-off was not reported")
	}
	if len(r.sent) != 1 || r.sent[0].To != token("j") {
		t.Fatalf("Bình's wake went despite An's refusal: %+v", r.sent)
	}
	exec(`DELETE FROM push_outbox WHERE conversation_id=$1`, w.room)
	exec(`UPDATE account_sessions SET created_at=now()-interval '2 hours', expires_at=now()-interval '1 minute' WHERE token_digest=$1`, w.binhToken)
	exec(`INSERT INTO push_outbox(id,person_id,conversation_id,sequence) VALUES(gen_random_uuid(),$1,$2,2)`, w.binh, w.room)
	quiet := &recorder{}
	if _, err := w.store.SendPending(ctx, quiet, 100); err != nil || len(quiet.sent) != 0 {
		t.Fatalf("an expired session's device was woken: %+v %v", quiet.sent, err)
	}
}

type picky struct {
	refuse string
	sent   []Message
}

func (p *picky) Send(_ context.Context, m []Message) error {
	for _, x := range m {
		if x.To == p.refuse {
			return io.ErrUnexpectedEOF
		}
	}
	p.sent = append(p.sent, m...)
	return nil
}

// Security review 05/10: the hand-off happens outside any transaction. While
// the push service hangs, no push_outbox row stays locked, and another worker
// claims nothing twice.
func TestAHangingPushServiceHoldsNoLock(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	if _, err := w.store.Register(ctx, w.binhToken, Registration{InstallationID: id(t, w.store), Platform: "android", Token: token("l")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Pool.Exec(ctx, `INSERT INTO push_outbox(id,person_id,conversation_id,sequence) VALUES(gen_random_uuid(),$1,$2,1)`, w.binh, w.room); err != nil {
		t.Fatal(err)
	}
	hang := &hanging{entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := w.store.SendPending(ctx, hang, 100); done <- err }()
	<-hang.entered
	locked := ctx
	tx, err := w.store.Pool.Begin(locked)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '300ms'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM push_outbox WHERE conversation_id=$1 FOR UPDATE`, w.room); err != nil {
		t.Fatalf("a row stayed locked during the hand-off: %v", err)
	}
	_ = tx.Rollback(context.Background())
	again := &recorder{}
	if _, err := w.store.SendPending(ctx, again, 100); err != nil || len(again.sent) != 0 {
		t.Fatalf("a leased wake was claimed twice: %+v %v", again.sent, err)
	}
	close(hang.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type hanging struct{ entered, release chan struct{} }

func (h *hanging) Send(ctx context.Context, _ []Message) error {
	close(h.entered)
	<-h.release
	return nil
}
