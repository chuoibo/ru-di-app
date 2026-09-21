//go:build postgres

package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/idem"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

func TestChatReplayRechecksMembershipThroughHTTP(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	var person string
	if err := pool.QueryRow(ctx, `INSERT INTO people (id,display_name) VALUES (gen_random_uuid(),'Replay (dữ liệu mẫu)') RETURNING id`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	store := repo.Repository{Q: pool}
	group, err := store.CreateContext(ctx, "Replay chat (dữ liệu mẫu)", person)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES (gen_random_uuid(),$1,$2,'active','admin','named')`, group.ID, person); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE idempotency_key LIKE $1`, "chat-replay-"+group.ID+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM account_sessions WHERE person_id=$1`, person)
		_, _ = pool.Exec(ctx, `DELETE FROM context_read_marks WHERE context_id=$1`, group.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE context_id=$1`, group.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM memberships WHERE context_id=$1`, group.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id=$1`, group.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM people WHERE id=$1`, person)
	})
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now}
	h := coreWithEnv(t, env, idem.New(idem.NewPostgresStore(pool)))
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Actor-ID", person)
		req.Header.Set("X-Actor-Roles", "member")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	path := "/contexts/" + group.ID + "/messages"
	body := `{"kind":"text","body":"Tin thử riêng tư (dữ liệu mẫu)"}`
	key := "chat-replay-" + group.ID
	first := request(http.MethodPost, path, body, key)
	if first.Code != 201 {
		t.Fatalf("first %d: %s", first.Code, first.Body.String())
	}
	replay := request(http.MethodPost, path, body, key)
	if replay.Code != 201 || replay.Header().Get(idem.ReplayHeaderName) != "true" || replay.Body.String() != first.Body.String() {
		t.Fatalf("authorized replay %d: %s", replay.Code, replay.Body.String())
	}
	var message string
	if err := pool.QueryRow(ctx, `SELECT id FROM messages WHERE context_id=$1`, group.ID).Scan(&message); err != nil {
		t.Fatal(err)
	}
	readPath := "/contexts/" + group.ID + "/read-mark"
	readBody := fmt.Sprintf(`{"message_id":%q}`, message)
	if read := request(http.MethodPut, readPath, readBody, key+"-read"); read.Code != 200 {
		t.Fatalf("read %d: %s", read.Code, read.Body.String())
	}
	if _, err := pool.Exec(ctx, `UPDATE memberships SET state='left',left_at=now() WHERE context_id=$1 AND person_id=$2`, group.ID, person); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ method, path, body, key string }{{http.MethodPost, path, body, key}, {http.MethodPut, readPath, readBody, key + "-read"}} {
		denied := request(test.method, test.path, test.body, test.key)
		if denied.Code != 403 || denied.Header().Get(idem.ReplayHeaderName) != "" || strings.Contains(denied.Body.String(), "Tin thử riêng tư") {
			t.Fatalf("revoked replay leaked %d: %s", denied.Code, denied.Body.String())
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE context_id=$1`, group.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("retry duplicated message: %d %v", count, err)
	}
	// Production session revocation must also pass through the endpoint on replay.
	if _, err := pool.Exec(ctx, `UPDATE memberships SET state='active',left_at=NULL WHERE context_id=$1 AND person_id=$2`, group.ID, person); err != nil {
		t.Fatal(err)
	}
	token := "synthetic-chat-session-" + group.ID
	session, err := store.CreateAccountSession(ctx, repo.AccountSessionInput{PersonID: person, TokenDigest: auth.TokenDigest(token), Now: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	env.Mode = endpoint.ModeProd
	prod := coreWithEnv(t, env, idem.New(idem.NewPostgresStore(pool)))
	prodRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key+"-session")
		w := httptest.NewRecorder()
		prod.ServeHTTP(w, req)
		return w
	}
	if w := prodRequest(); w.Code != 201 {
		t.Fatalf("production first %d: %s", w.Code, w.Body.String())
	}
	if _, err := pool.Exec(ctx, `UPDATE account_sessions SET revoked_at=now() WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if w := prodRequest(); w.Code != 401 || w.Header().Get(idem.ReplayHeaderName) != "" {
		t.Fatalf("revoked session replay %d: %s", w.Code, w.Body.String())
	}

}

func TestDirectChatReplayRechecksBlockedCounterpart(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	people := make([]string, 2)
	for i := range people {
		if err := pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Pair replay (dữ liệu mẫu)') RETURNING id`).Scan(&people[i]); err != nil {
			t.Fatal(err)
		}
	}
	if people[0] > people[1] {
		people[0], people[1] = people[1], people[0]
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	pair, err := (repo.Repository{Q: tx}).CreatePairContext(ctx, repo.PairContextInput{PairKey: people[0] + ":" + people[1], MemberIDs: people, CreatedByID: people[0], Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE idempotency_key=$1`, "chat-pair-replay-"+pair.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE context_id=$1`, pair.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM memberships WHERE context_id=$1`, pair.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id=$1`, pair.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM friend_requests WHERE requester_id=$1`, people[0])
		_, _ = pool.Exec(ctx, `DELETE FROM people WHERE id=ANY($1::uuid[])`, people)
	})
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now}
	h := coreWithEnv(t, env, idem.New(idem.NewPostgresStore(pool)))
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/contexts/"+pair.ID+"/messages", strings.NewReader(`{"kind":"text","body":"Tin thử riêng tư (dữ liệu mẫu)"}`))
		r.Header.Set("X-Actor-ID", people[0])
		r.Header.Set("X-Actor-Roles", "member")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "chat-pair-replay-"+pair.ID)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request(); w.Code != 201 {
		t.Fatalf("pair first %d: %s", w.Code, w.Body.String())
	}
	if w := request(); w.Code != 201 || w.Header().Get(idem.ReplayHeaderName) != "true" {
		t.Fatalf("pair replay %d: %s", w.Code, w.Body.String())
	}
	if _, err := pool.Exec(ctx, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES(gen_random_uuid(),$1,$2,'blocked',$2,now())`, people[0], people[1]); err != nil {
		t.Fatal(err)
	}
	if w := request(); w.Code != 409 || w.Header().Get(idem.ReplayHeaderName) != "" {
		t.Fatalf("blocked pair replay %d: %s", w.Code, w.Body.String())
	}
}
