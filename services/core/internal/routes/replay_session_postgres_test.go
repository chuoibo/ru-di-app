//go:build postgres

package routes

import (
	"context"
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

// A generic write's stored answer is not handed back to a bearer whose session
// was revoked since (audit 2026-10-05, RS-02): through the real front door,
// idempotency and PostgreSQL, the replay is get_actor's 401, and nothing is
// written twice.
func TestAGenericReplayToARevokedSessionIsRefused(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	people := make([]string, 2)
	for i := range people {
		if err := pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Replay phiên (dữ liệu mẫu)') RETURNING id`).Scan(&people[i]); err != nil {
			t.Fatal(err)
		}
	}
	reporter, target := people[0], people[1]
	key := "replay-session-" + reporter
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE idempotency_key=$1`, key)
		_, _ = pool.Exec(ctx, `DELETE FROM reports WHERE reporter_id=$1`, reporter)
		_, _ = pool.Exec(ctx, `DELETE FROM account_sessions WHERE person_id=$1`, reporter)
		_, _ = pool.Exec(ctx, `DELETE FROM people WHERE id = ANY($1)`, people)
	})
	token := "synthetic-replay-session-" + reporter
	session, err := (repo.Repository{Q: pool}).CreateAccountSession(ctx, repo.AccountSessionInput{PersonID: reporter, TokenDigest: auth.TokenDigest(token), Now: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	env := endpoint.Env{Mode: endpoint.ModeProd, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now}
	h := coreWithEnv(t, env, idem.New(idem.NewPostgresStore(pool), idem.WithReplayGate(idem.SessionGate(repo.Sessions{Q: pool}, time.Now))))
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(`{"target_type":"person","target_id":"`+target+`","reason":"spam"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	first := send()
	if first.Code != 201 {
		t.Fatalf("first %d: %s", first.Code, first.Body.String())
	}
	if again := send(); again.Code != 201 || again.Header().Get(idem.ReplayHeaderName) != "true" || again.Body.String() != first.Body.String() {
		t.Fatalf("live replay %d: %s", again.Code, again.Body.String())
	}
	if _, err := pool.Exec(ctx, `UPDATE account_sessions SET revoked_at=now() WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	denied := send()
	want := `{"code":"authentication_required","detail":"Session is not valid"}`
	if denied.Code != 401 || denied.Body.String() != want || denied.Header().Get(idem.ReplayHeaderName) != "" {
		t.Fatalf("revoked replay %d %v: %s", denied.Code, denied.Header(), denied.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE reporter_id=$1`, reporter).Scan(&count); err != nil || count != 1 {
		t.Fatalf("reports written: %d %v", count, err)
	}
}
