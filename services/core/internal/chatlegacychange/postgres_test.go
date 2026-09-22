//go:build postgres

package chatlegacychange

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

var bg = context.Background()

func id() string {
	s, e := repo.NewUUID()
	if e != nil {
		panic(e)
	}
	return s
}

type world struct {
	pool           *pgxpool.Pool
	store          Store
	room           string
	people, tokens []string
	headers        []http.Header
}

func exec(t *testing.T, q repo.Querier, sql string, a ...any) {
	t.Helper()
	if _, e := q.Exec(bg, sql, a...); e != nil {
		t.Fatal(e)
	}
}
func setup(t *testing.T) world {
	t.Helper()
	base := testdb.Pool(t)
	schema := "legacy_feed_" + strings.ReplaceAll(id(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	exec(t, base, "CREATE SCHEMA "+quoted)
	t.Cleanup(func() { exec(t, base, "DROP SCHEMA "+quoted+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, e := pgxpool.NewWithConfig(bg, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests", "account_sessions", "messages", "message_reactions", "votes", "vote_options", "vote_ballots"} {
		exec(t, pool, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	// LIKE copies the unique definition but PostgreSQL chooses a fresh name.
	// Keep the production name used to identify a true idempotent reaction.
	var reactionUnique string
	if e = pool.QueryRow(bg, `SELECT conname FROM pg_constraint WHERE conrelid='message_reactions'::regclass AND contype='u'`).Scan(&reactionUnique); e != nil {
		t.Fatal(e)
	}
	if reactionUnique != "uq_message_reactions_one_per_kind" {
		exec(t, pool, "ALTER TABLE message_reactions RENAME CONSTRAINT "+pgx.Identifier{reactionUnique}.Sanitize()+" TO uq_message_reactions_one_per_kind")
	}
	if e = Migrate(bg, pool); e != nil {
		t.Fatal(e)
	}
	w := world{pool: pool, store: Store{Pool: pool}, room: id()}
	for i := 0; i < 3; i++ {
		person, token := id(), "synthetic-"+id()
		w.people = append(w.people, person)
		w.tokens = append(w.tokens, token)
		w.headers = append(w.headers, http.Header{"Authorization": []string{"Bearer " + token}})
		exec(t, pool, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic feed participant')`, person)
		exec(t, pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, id(), person, auth.TokenDigest(token))
	}
	exec(t, pool, `INSERT INTO contexts(id,display_name,created_by_id) VALUES($1,'Synthetic feed room',$2)`, w.room, w.people[0])
	for _, person := range w.people[:2] {
		exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','member','named')`, id(), w.room, person)
	}
	return w
}
func (w world) write(t *testing.T, fn func(repo.Repository)) {
	t.Helper()
	u := db.NewUnit(w.pool)
	defer u.Rollback(bg)
	call := &endpoint.Call{Request: httptest.NewRequest("POST", "/contexts/"+w.room+"/messages", nil), Unit: u, Actor: &auth.Actor{ID: w.people[0]}, Scope: dispatch.Scope{Params: map[string]string{"context_id": w.room}}}
	if e := BeforeWrite(bg, call); e != nil {
		t.Fatal(e)
	}
	tx, e := u.Tx(bg)
	if e != nil {
		t.Fatal(e)
	}
	fn(repo.Repository{Q: tx})
	if e = u.Commit(bg); e != nil {
		t.Fatal(e)
	}
}
func (w world) message(t *testing.T) repo.Message {
	var m repo.Message
	w.write(t, func(r repo.Repository) {
		body := "Synthetic hello"
		var e error
		m, e = r.CreateMessage(bg, repo.MessageInput{ContextID: w.room, AuthorID: &w.people[0], Kind: "text", Body: &body, Now: time.Now()})
		if e != nil {
			t.Fatal(e)
		}
	})
	return m
}
func snapshotObject(t *testing.T, b json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if e := json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestPostgresCaptureAndSnapshotReactionDeletePoll(t *testing.T) {
	w := setup(t)
	m := w.message(t)
	before, e := w.store.Changes(bg, w.headers[1], w.room, 0, 100)
	if e != nil || len(before.Changes) != 1 {
		t.Fatalf("initial %+v %v", before, e)
	}
	w.write(t, func(r repo.Repository) {
		if _, e := r.AddReaction(bg, m.ID, w.people[0], "heart", time.Now()); e != nil {
			t.Fatal(e)
		}
	})
	changed, e := w.store.Changes(bg, w.headers[1], w.room, before.NextSequence, 100)
	if e != nil || len(changed.Changes) != 1 || changed.Changes[0].EntityID != m.ID {
		t.Fatalf("reaction %+v %v", changed, e)
	}
	snap, e := w.store.Snapshot(bg, w.headers[1], w.room, SnapshotRequest{MessageIDs: []string{m.ID}})
	if e != nil {
		t.Fatal(e)
	}
	obj := snapshotObject(t, snap.Messages[0])
	reactions := obj["reactions"].([]any)
	if len(reactions) != 1 || reactions[0].(map[string]any)["mine"] != false {
		t.Fatalf("peer reaction %s", snap.Messages[0])
	}
	own, e := w.store.Snapshot(bg, w.headers[0], w.room, SnapshotRequest{MessageIDs: []string{m.ID}})
	if e != nil {
		t.Fatal(e)
	}
	if snapshotObject(t, own.Messages[0])["reactions"].([]any)[0].(map[string]any)["mine"] != true {
		t.Fatal("lost reader-specific mine")
	}
	var v repo.Vote
	w.write(t, func(r repo.Repository) {
		var e error
		v, e = r.CreateVote(bg, repo.VoteInput{ContextID: w.room, CreatedByID: w.people[0], Question: "Synthetic choice", Options: []repo.VoteOptionInput{{Label: "A"}, {Label: "B"}}, Now: time.Now()})
		if e != nil {
			t.Fatal(e)
		}
		if _, _, e = r.UpsertBallot(bg, v.ID, v.Options[1].ID, w.people[1], time.Now()); e != nil {
			t.Fatal(e)
		}
	})
	snap, e = w.store.Snapshot(bg, w.headers[1], w.room, SnapshotRequest{VoteIDs: []string{v.ID}})
	if e != nil {
		t.Fatal(e)
	}
	vote := snapshotObject(t, snap.Votes[0])
	if vote["total_ballots"] != float64(1) || vote["my_option_id"] != v.Options[1].ID {
		t.Fatalf("tally %s", snap.Votes[0])
	}
	w.write(t, func(r repo.Repository) {
		if _, e = r.SoftDeleteMessage(bg, m.ID, time.Now()); e != nil {
			t.Fatal(e)
		}
		if _, e = r.CloseVote(bg, v.ID, w.people[0], time.Now()); e != nil {
			t.Fatal(e)
		}
	})
	snap, e = w.store.Snapshot(bg, w.headers[1], w.room, SnapshotRequest{MessageIDs: []string{m.ID}, VoteIDs: []string{v.ID}})
	if e != nil {
		t.Fatal(e)
	}
	obj = snapshotObject(t, snap.Messages[0])
	if obj["kind"] != "deleted" || obj["body"] != nil || len(obj["reactions"].([]any)) != 0 {
		t.Fatalf("tombstone %s", snap.Messages[0])
	}
	if snapshotObject(t, snap.Votes[0])["is_closed"] != true {
		t.Fatal("poll close stale")
	}
	var events, outbox int
	_ = w.pool.QueryRow(bg, `SELECT count(*) FROM chat_legacy_changes`).Scan(&events)
	_ = w.pool.QueryRow(bg, `SELECT count(*) FROM chat_legacy_change_outbox`).Scan(&outbox)
	if events != outbox {
		t.Fatalf("events=%d outbox=%d", events, outbox)
	}
}
func TestPostgresRollbackReplayAndMigrationRoundTrip(t *testing.T) {
	w := setup(t)
	m := w.message(t)
	tx, e := w.pool.Begin(bg)
	if e != nil {
		t.Fatal(e)
	}
	r := repo.Repository{Q: tx}
	if _, e = r.AddReaction(bg, m.ID, w.people[1], "heart", time.Now()); e != nil {
		t.Fatal(e)
	}
	_ = tx.Rollback(bg)
	p, e := w.store.Changes(bg, w.headers[0], w.room, 0, 100)
	if e != nil || len(p.Changes) != 1 {
		t.Fatalf("rollback leaked %+v %v", p, e)
	}
	w.write(t, func(r repo.Repository) {
		if _, e = r.AddReaction(bg, m.ID, w.people[1], "heart", time.Now()); e != nil {
			t.Fatal(e)
		}
	})
	w.write(t, func(r repo.Repository) {
		added, e := r.AddReaction(bg, m.ID, w.people[1], "heart", time.Now())
		if e != nil || added {
			t.Fatalf("replay %v %v", added, e)
		}
	})
	p, e = w.store.Changes(bg, w.headers[0], w.room, 0, 100)
	if e != nil || len(p.Changes) != 2 || p.Watermark != 2 {
		t.Fatalf("duplicate effect %+v %v", p, e)
	}
	if e = Migrate(bg, w.pool); e != nil {
		t.Fatal(e)
	}
	if e = Unmigrate(bg, w.pool); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(bg, w.pool); e != nil {
		t.Fatal(e)
	}
	snap, e := w.store.Snapshot(bg, w.headers[0], w.room, SnapshotRequest{})
	if e != nil || len(snap.Messages) != 1 || snap.Watermark != 0 {
		t.Fatalf("round trip %+v %v", snap, e)
	}
}
func TestPostgresSnapshotACLAndCrossContextIDs(t *testing.T) {
	w := setup(t)
	m := w.message(t)
	if _, e := w.store.Snapshot(bg, w.headers[2], w.room, SnapshotRequest{MessageIDs: []string{m.ID}}); !errors.Is(e, ErrForbidden) {
		t.Fatalf("outsider %v", e)
	}
	other := id()
	exec(t, w.pool, `INSERT INTO contexts(id,display_name,created_by_id) VALUES($1,'Synthetic other room',$2)`, other, w.people[0])
	foreign := id()
	exec(t, w.pool, `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','Synthetic private')`, foreign, other, w.people[0])
	snap, e := w.store.Snapshot(bg, w.headers[1], w.room, SnapshotRequest{MessageIDs: []string{foreign}})
	if e != nil || len(snap.Messages) != 0 {
		t.Fatalf("cross context leak %+v %v", snap, e)
	}
	exec(t, w.pool, `UPDATE memberships SET state='left',left_at=now() WHERE context_id=$1 AND person_id=$2`, w.room, w.people[1])
	if _, e = w.store.Changes(bg, w.headers[1], w.room, 0, 100); !errors.Is(e, ErrForbidden) {
		t.Fatalf("removed member %v", e)
	}
	exec(t, w.pool, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, w.people[0])
	if _, e = w.store.Changes(bg, w.headers[0], w.room, 0, 100); !errors.Is(e, ErrAuthentication) {
		t.Fatalf("revoked session %v", e)
	}
}
func TestPostgresConcurrentSequenceHasNoCommitGap(t *testing.T) {
	w := setup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, e := w.pool.Begin(bg)
			if e != nil {
				errs <- e
				return
			}
			defer tx.Rollback(bg)
			r := repo.Repository{Q: tx}
			body := "Synthetic burst"
			_, e = r.CreateMessage(bg, repo.MessageInput{ContextID: w.room, AuthorID: &w.people[0], Kind: "text", Body: &body, Now: time.Now()})
			if e == nil {
				e = tx.Commit(bg)
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	p, e := w.store.Changes(bg, w.headers[1], w.room, 0, 100)
	if e != nil || len(p.Changes) != 20 {
		t.Fatalf("burst %+v %v", p, e)
	}
	for i, c := range p.Changes {
		if c.Sequence != int64(i+1) {
			t.Fatalf("gap %+v", p)
		}
	}
}
func TestPostgresWebsocketFirstAuthResumeAndLiveRevocation(t *testing.T) {
	w := setup(t)
	ctx, cancel := context.WithCancel(bg)
	defer cancel()
	h := New(w.store, ctx, nil)
	h.ReconcileInterval = 20 * time.Millisecond
	h.AuthTimeout = 100 * time.Millisecond
	server := httptest.NewServer(cors.New("", false).Middleware(h))
	defer server.Close()
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/contexts/" + w.room + "/changes/stream?after=0"
	dialCtx, stop := context.WithTimeout(bg, 3*time.Second)
	defer stop()
	c, _, e := websocket.Dial(dialCtx, address, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://localhost:8177"}}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	if e = wsjson.Write(dialCtx, c, map[string]string{"type": "authenticate", "token": w.tokens[1]}); e != nil {
		t.Fatal(e)
	}
	var page Page
	if e = wsjson.Read(dialCtx, c, &page); e != nil || page.NextSequence != 0 {
		t.Fatalf("bootstrap %+v %v", page, e)
	}
	if e = wsjson.Write(dialCtx, c, map[string]any{"type": "ack", "sequence": page.NextSequence}); e != nil {
		t.Fatal(e)
	}
	_ = w.message(t)
	if e = wsjson.Read(dialCtx, c, &page); e != nil || len(page.Changes) != 1 {
		t.Fatalf("live %+v %v", page, e)
	}
	cursor := page.NextSequence
	_ = wsjson.Write(dialCtx, c, map[string]any{"type": "ack", "sequence": cursor})
	c.CloseNow()
	_ = w.message(t)
	c, _, e = websocket.Dial(dialCtx, strings.Replace(address, "after=0", fmt.Sprintf("after=%d", cursor), 1), &websocket.DialOptions{HTTPHeader: w.headers[1]})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	if e = wsjson.Read(dialCtx, c, &page); e != nil || len(page.Changes) != 1 || page.NextSequence != cursor+1 {
		t.Fatalf("resume %+v %v", page, e)
	}
	_ = wsjson.Write(dialCtx, c, map[string]any{"type": "ack", "sequence": page.NextSequence})
	exec(t, w.pool, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, w.people[1])
	if e = wsjson.Read(dialCtx, c, &page); e == nil {
		t.Fatal("session revocation left websocket active")
	}
	unauth, _, e := websocket.Dial(dialCtx, address, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer unauth.CloseNow()
	if e = wsjson.Read(dialCtx, unauth, &page); e == nil {
		t.Fatal("received content before authentication")
	}
}

func TestPostgresMixedWritersUseHeadBeforeEntityLocks(t *testing.T) {
	w := setup(t)
	a, b := w.message(t), w.message(t)
	errs := make(chan error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, ids := range [][]string{{a.ID, b.ID}, {b.ID, a.ID}} {
		ids := ids
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(bg, 3*time.Second)
			defer cancel()
			u := db.NewUnit(w.pool)
			defer u.Rollback(ctx)
			call := &endpoint.Call{Request: httptest.NewRequest("POST", "/contexts/"+w.room+"/messages", nil), Unit: u, Actor: &auth.Actor{ID: w.people[0]}, Scope: dispatch.Scope{Params: map[string]string{"context_id": w.room}}}
			if e := BeforeWrite(ctx, call); e != nil {
				errs <- e
				return
			}
			tx, e := u.Tx(ctx)
			if e != nil {
				errs <- e
				return
			}
			r := repo.Repository{Q: tx}
			for _, message := range ids {
				if _, e = r.AddReaction(ctx, message, w.people[0], "heart", time.Now()); e != nil {
					errs <- e
					return
				}
			}
			// The bot creates its poll and card in the same transaction as a user send.
			v, e := r.CreateVote(ctx, repo.VoteInput{ContextID: w.room, CreatedByID: w.people[0], Question: "Synthetic mixed writer", Options: []repo.VoteOptionInput{{Label: "A"}, {Label: "B"}}, Now: time.Now()})
			if e != nil {
				errs <- e
				return
			}
			card, _ := json.Marshal(map[string]any{"kind": "poll", "payload": map[string]any{"vote_id": v.ID}})
			if _, e = r.CreateMessage(ctx, repo.MessageInput{ContextID: w.room, Kind: "ai_card", Card: card, Now: time.Now()}); e != nil {
				errs <- e
				return
			}
			errs <- u.Commit(ctx)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	snap, e := w.store.Snapshot(bg, w.headers[1], w.room, SnapshotRequest{})
	if e != nil || len(snap.Messages) != 4 || len(snap.Votes) != 2 {
		t.Fatalf("mixed snapshot %+v %v", snap, e)
	}
}

func TestPostgresUncommittedCounterNeverSkipsCommittedChange(t *testing.T) {
	w := setup(t)
	tx, e := w.pool.Begin(bg)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(bg)
	r := repo.Repository{Q: tx}
	body := "Synthetic not committed"
	if _, e = r.CreateMessage(bg, repo.MessageInput{ContextID: w.room, AuthorID: &w.people[0], Kind: "text", Body: &body, Now: time.Now()}); e != nil {
		t.Fatal(e)
	}
	entered, finished := make(chan struct{}), make(chan error, 1)
	go func() {
		other, e := w.pool.Begin(bg)
		if e != nil {
			finished <- e
			return
		}
		defer other.Rollback(bg)
		close(entered)
		r := repo.Repository{Q: other}
		body := "Synthetic committed later"
		_, e = r.CreateMessage(bg, repo.MessageInput{ContextID: w.room, AuthorID: &w.people[0], Kind: "text", Body: &body, Now: time.Now()})
		if e == nil {
			e = other.Commit(bg)
		}
		finished <- e
	}()
	<-entered
	p, e := w.store.Changes(bg, w.headers[1], w.room, 0, 100)
	if e != nil || p.Watermark != 0 || len(p.Changes) != 0 {
		t.Fatalf("uncommitted leak %+v %v", p, e)
	}
	_ = tx.Rollback(bg)
	select {
	case e = <-finished:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("writer remained blocked after rollback")
	}
	p, e = w.store.Changes(bg, w.headers[1], w.room, 0, 100)
	if e != nil || p.Watermark != 1 || len(p.Changes) != 1 {
		t.Fatalf("rolled back gap %+v %v", p, e)
	}
}

func TestPostgresIdleDisconnectReleasesAccountCapacity(t *testing.T) {
	w := setup(t)
	ctx, cancel := context.WithCancel(bg)
	defer cancel()
	h := New(w.store, ctx, nil)
	h.ReconcileInterval = 20 * time.Millisecond
	server := httptest.NewServer(h)
	defer server.Close()
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/contexts/" + w.room + "/changes/stream"
	for i := 0; i < 8; i++ {
		op, stop := context.WithTimeout(bg, time.Second)
		c, _, e := websocket.Dial(op, address, &websocket.DialOptions{HTTPHeader: w.headers[1]})
		if e != nil {
			stop()
			t.Fatal(e)
		}
		var page Page
		e = wsjson.Read(op, c, &page)
		if e != nil {
			stop()
			t.Fatal(e)
		}
		e = wsjson.Write(op, c, map[string]any{"type": "ack", "sequence": page.NextSequence})
		if e != nil {
			stop()
			t.Fatal(e)
		}
		c.CloseNow()
		stop()
		deadline := time.Now().Add(time.Second)
		for {
			h.mu.Lock()
			active := h.actors[w.people[1]]
			h.mu.Unlock()
			if active == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("quiet closed websocket leaked actor slot")
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func TestPostgresPairBlockAndDeletedPeerRefuseSnapshots(t *testing.T) {
	w := setup(t)
	m := w.message(t)
	exec(t, w.pool, `UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, w.room, w.people[0]+":"+w.people[1])
	if _, e := w.store.Snapshot(bg, w.headers[0], w.room, SnapshotRequest{MessageIDs: []string{m.ID}}); e != nil {
		t.Fatal(e)
	}
	edge := id()
	exec(t, w.pool, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES($1,$2,$3,'blocked',$2,now())`, edge, w.people[0], w.people[1])
	for _, headers := range w.headers[:2] {
		if _, e := w.store.Snapshot(bg, headers, w.room, SnapshotRequest{}); !errors.Is(e, ErrForbidden) {
			t.Fatalf("block must be symmetric: %v", e)
		}
	}
	exec(t, w.pool, `DELETE FROM friend_requests WHERE id=$1`, edge)
	exec(t, w.pool, `UPDATE people SET deleted_at=now() WHERE id=$1`, w.people[1])
	if _, e := w.store.Changes(bg, w.headers[0], w.room, 0, 100); !errors.Is(e, ErrForbidden) {
		t.Fatalf("deleted peer admitted: %v", e)
	}
}

func TestPostgresOutboxFailureCannotCommitBusinessMutation(t *testing.T) {
	w := setup(t)
	m := w.message(t)
	exec(t, w.pool, `ALTER TABLE chat_legacy_change_outbox ADD CONSTRAINT synthetic_outbox_failure CHECK(false) NOT VALID`)
	tx, e := w.pool.Begin(bg)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(bg)
	r := repo.Repository{Q: tx}
	if _, e = r.AddReaction(bg, m.ID, w.people[1], "heart", time.Now()); e == nil {
		t.Fatal("outbox failure was ignored")
	}
	_ = tx.Rollback(bg)
	page, e := w.store.Changes(bg, w.headers[0], w.room, 0, 100)
	if e != nil || page.Watermark != 1 {
		t.Fatalf("partial commit %+v %v", page, e)
	}
	snapshot, e := w.store.Snapshot(bg, w.headers[0], w.room, SnapshotRequest{MessageIDs: []string{m.ID}})
	if e != nil {
		t.Fatal(e)
	}
	if len(snapshotObject(t, snapshot.Messages[0])["reactions"].([]any)) != 0 {
		t.Fatal("reaction survived rejected outbox")
	}
}
