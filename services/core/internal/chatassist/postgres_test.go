package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/testdb"
)

type fixture struct {
	pool                                            *pgxpool.Pool
	handler                                         *Handler
	context, person, peer, member, token, peerToken string
	capabilityCalls                                 *atomic.Int64
}

func setup(t *testing.T, model http.HandlerFunc) fixture {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "chat_ai_test_" + strings.ReplaceAll(newID(), "-", "")
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE")
		if err != nil {
			t.Error(err)
		}
	})
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "account_sessions", "messages", "places", "destinations", "outings", "outing_stops", "person_interests"} {
		if _, err = pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	// The order `core migrate-chat` runs: the outbox first, since version 5's
	// trigger calls jobs_them.
	if err = jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	capabilityCalls := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/brain/v1/capabilities" {
			capabilityCalls.Add(1)
			reply(w, 200, map[string]any{"plan": map[string]bool{"available": true}})
			return
		}
		if model != nil {
			model(w, r)
			return
		}
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic inference fixture"}})
	}))
	t.Cleanup(server.Close)
	t.Setenv("MOBILE_BRAIN_URL", server.URL)
	t.Setenv("MOBILE_INTERNAL_TOKEN", "synthetic-internal-test-only")
	f := fixture{pool: pool, context: newID(), person: newID(), peer: newID(), member: newID(), token: "synthetic-" + newID(), peerToken: "synthetic-" + newID()}
	f.handler = New(pool, brain.Configured())
	f.capabilityCalls = capabilityCalls
	_, err = pool.Exec(ctx, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic caller'),($2,'Synthetic peer')`, f.person, f.peer)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES($1,'Synthetic job group','group',$2)`, f.context, f.person)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','admin','named'),($4,$2,$5,'active','member','named')`, f.member, f.context, f.person, newID(), f.peer)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct{ person, token string }{{f.person, f.token}, {f.peer, f.peerToken}} {
		_, err = pool.Exec(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()+interval '1 hour')`, newID(), s.person, auth.TokenDigest(s.token))
		if err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f fixture) request(method, path, token string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func (f fixture) route() string { return "/contexts/" + f.context + "/ai-invocations" }
func requireCode(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
}
func (f fixture) create(t *testing.T) Invocation {
	t.Helper()
	w := f.request("POST", f.route(), f.token, map[string]string{"logical_id": newID(), "command": "plan", "prompt": "Synthetic explicitly shared request"})
	requireCode(t, w, 202)
	var v Invocation
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestInvocationOnlyInputAndDurableIdempotency(t *testing.T) {
	var payload map[string]any
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic provider fixture"}})
	})
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','Synthetic unshared history sentinel')`, newID(), f.context, f.peer); err != nil {
		t.Fatal(err)
	}
	input := map[string]string{"logical_id": newID(), "command": "plan", "prompt": "Only this synthetic invocation is shared"}
	var first Invocation
	for i := 0; i < 2; i++ {
		w := f.request("POST", f.route(), f.token, input)
		status := 202
		if i > 0 {
			status = 200
		}
		requireCode(t, w, status)
		var v Invocation
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		if i == 0 {
			first = v
		} else if v.ID != first.ID {
			t.Fatal("retry created a new invocation")
		}
	}
	input["prompt"] = "Different synthetic bytes"
	requireCode(t, f.request("POST", f.route(), f.token, input), 409)
	ok, err := f.handler.ProcessOne(ctx)
	if err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	raw, _ := json.Marshal(payload)
	if bytes.Contains(raw, []byte("unshared history")) || !bytes.Contains(raw, []byte("Only this synthetic invocation")) {
		t.Fatal("inference input scope violated")
	}
	// ADR-0036 §2.3 and §5: the server lays the roster on top, one entry per
	// active member by display name. Never an account id.
	if got := payload["members"].([]any); len(got) != 2 {
		t.Fatalf("roster has %d entries, want the 2 active members", len(got))
	}
	if bytes.Contains(raw, []byte(f.peer)) || bytes.Contains(raw, []byte(f.person)) {
		t.Fatal("roster carries an account id")
	}
	w := f.request("GET", f.route()+"/"+first.ID, f.token, nil)
	requireCode(t, w, 200)
	var final Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &final)
	if final.Status != "succeeded" || final.MessageID == nil {
		t.Fatalf("not published: %#v", final)
	}
	if _, err = f.handler.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	var prompt *string
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("publish count=%d err=%v", count, err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT prompt FROM chat_ai_invocations WHERE id=$1`, first.ID).Scan(&prompt); err != nil || prompt != nil {
		t.Fatal("completed prompt retained")
	}
	requireCode(t, f.request("GET", f.route()+"/"+first.ID, f.peerToken, nil), 404)
	listing := f.request("GET", f.route(), f.token, nil)
	requireCode(t, listing, 200)
	if strings.Contains(listing.Body.String(), "prompt") || strings.Contains(listing.Body.String(), "digest") {
		t.Fatal("public listing exposes private inputs")
	}
}

func TestInvocationConcurrentReplayAndMembershipBarrier(t *testing.T) {
	f := setup(t, nil)
	input := map[string]string{"logical_id": newID(), "command": "plan", "prompt": "Synthetic race"}
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- f.request("POST", f.route(), f.token, input) }()
	}
	wg.Wait()
	close(responses)
	var id string
	created := 0
	for w := range responses {
		if w.Code != 202 && w.Code != 200 {
			t.Fatalf("race response %d: %s", w.Code, w.Body.String())
		}
		if w.Code == 202 {
			created++
		}
		var v Invocation
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		if id != "" && id != v.ID {
			t.Fatal("duplicate logical job")
		}
		id = v.ID
	}
	if created != 1 {
		t.Fatalf("created %d jobs", created)
	}
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE id=$1`, f.member); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.request("POST", f.route(), f.token, input), 403)
	if _, err := f.pool.Exec(ctx, `UPDATE memberships SET state='active',left_at=NULL WHERE id=$1`, f.member); err != nil {
		t.Fatal(err)
	}
	if _, err := f.handler.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	var prompt *string
	if err := f.pool.QueryRow(ctx, `SELECT status,prompt FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &prompt); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" || prompt != nil {
		t.Fatal("rejoin revived an old sharing grant")
	}
	requireCode(t, f.request("POST", f.route()+"/"+id+"/retry", f.token, nil), 409)
}

func TestInvocationRevocationWhileInferenceAndLeaseRecovery(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic result after revoke"}})
	})
	job := f.create(t)
	done := make(chan error, 1)
	go func() { _, err := f.handler.ProcessOne(context.Background()); done <- err }()
	<-entered
	if _, err := f.pool.Exec(context.Background(), `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE token_digest=$1`, auth.TokenDigest(f.token)); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var count int
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if count != 0 || status != "failed" {
		t.Fatalf("revoke published: count=%d status=%s", count, status)
	}
	requireCode(t, f.request("GET", f.route()+"/"+job.ID, f.token, nil), 401)
}

func TestInvocationCancellationAndExpiredLease(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	job := f.create(t)
	j, ok, err := f.handler.claim(ctx)
	if err != nil || !ok {
		t.Fatal(err)
	}
	requireCode(t, f.request("POST", f.route()+"/"+job.ID+"/cancel", f.token, nil), 200)
	if err = f.handler.publish(ctx, j, json.RawMessage(`{"kind":"text","payload":{"text":"Synthetic stale worker"}}`), nil); err != nil {
		t.Fatal(err)
	}
	another := f.create(t)
	if _, _, err = f.handler.claim(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, another.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err = f.handler.ProcessOne(ctx); err != nil || !ok {
		t.Fatalf("lease recovery: %v %v", ok, err)
	}
	var n int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("cancel/recovery count=%d err=%v", n, err)
	}
}

func TestPlanPromotionAtomicConcurrentAndRollback(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	source := newID()
	_, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,kind,card) VALUES($1,$2,'ai_card',$3)`, source, f.context, `{"kind":"itinerary","payload":{"title":"Synthetic plan","stops":[{"time_text":"18:30","place":{"id":"synthetic","name":"Synthetic place"}}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	input := promotionInput{Source: source, Title: "Synthetic confirmed plan", Starts: "2026-10-01", Ends: "2026-10-01", Headcount: 2, Budget: 100000, Stops: []planStop{{At: "18:30", Label: "Synthetic human-reviewed stop"}}}
	route := "/contexts/" + f.context + "/plan-promotions"
	bad := input
	bad.Stops = []planStop{{At: "tomorrow", Label: "Synthetic invalid time"}}
	requireCode(t, f.request("POST", route, f.token, bad), 422)
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM outings`).Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid promotion left an orphan outing")
	}
	responses := make(chan *httptest.ResponseRecorder, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- f.request("POST", route, f.token, input) }()
	}
	wg.Wait()
	close(responses)
	var first string
	created := 0
	for w := range responses {
		if w.Code != 200 && w.Code != 201 {
			t.Fatalf("promotion: %d %s", w.Code, w.Body.String())
		}
		if w.Code == 201 {
			created++
		}
		var v promotionResult
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		if first != "" && first != v.OutingID {
			t.Fatal("source promoted twice")
		}
		first = v.OutingID
		if v.Revision != 1 {
			t.Fatalf("missing atomic stops: %d", v.Revision)
		}
	}
	if created != 1 {
		t.Fatal("multiple promotion winners")
	}
	requireCode(t, f.request("GET", route+"/"+source, f.peerToken, nil), 200)
	input.Title = "Conflicting human revision"
	requireCode(t, f.request("POST", route, f.peerToken, input), 409)
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM outings`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate outing count=%d err=%v", count, err)
	}
}

func TestDeletedIdentityCannotInvokeReadOrPromote(t *testing.T) {
	f := setup(t, nil)
	job := f.create(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE people SET deleted_at=clock_timestamp() WHERE id=$1`, f.person); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.request("GET", f.route()+"/"+job.ID, f.token, nil), 401)
	requireCode(t, f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil), 401)
	requireCode(t, f.request("GET", "/contexts/"+f.context+"/plan-promotions/"+newID(), f.token, nil), 401)
	if _, err := f.handler.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("deleted identity published: %d %v", n, err)
	}
}

func TestUnauthenticatedCallsNeverReachInferenceCapability(t *testing.T) {
	f := setup(t, nil)
	input := map[string]string{"logical_id": newID(), "command": "plan", "prompt": "Synthetic unauthorized request"}
	requireCode(t, f.request("POST", f.route(), "synthetic-invalid-session", input), 401)
	requireCode(t, f.request("POST", f.route()+"/"+newID()+"/retry", "synthetic-invalid-session", nil), 401)
	if f.capabilityCalls.Load() != 0 {
		t.Fatal("unauthorized request reached inference service")
	}
}

func TestExpiredUnclaimedLeaseCannotDispatchOrPublish(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	job := f.create(t)
	j, ok, err := f.handler.claim(ctx)
	if err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.handler.prepare(ctx, j); err == nil {
		t.Fatal("expired worker retained dispatch permission")
	}
	if err = f.handler.publish(ctx, j, json.RawMessage(`{"kind":"text","payload":{"text":"Synthetic expired worker"}}`), nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired worker published: %d %v", count, err)
	}
	if ok, err = f.handler.ProcessOne(ctx); err != nil || !ok {
		t.Fatalf("fresh worker recovery: %v %v", ok, err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("recovery did not publish once: %d %v", count, err)
	}
}
