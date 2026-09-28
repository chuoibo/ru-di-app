//go:build postgres || broker

package chatlegacychange

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

// The live fixture both tiers share: the Postgres tier (this package's
// feed) and the broker tier (the room's `ai` frames, phong_broker_test.go).

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
