//go:build postgres

package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/idem"
	"mobile/services/core/internal/testdb"
)

// completeFails is the PostgreSQL store with a completion that never lands on
// its own transaction: a crash, a dropped connection or a full disk after the
// business commit.
type completeFails struct{ *idem.PostgresStore }

func (completeFails) Complete(context.Context, string, string, idem.StoredResponse) error {
	return errors.New("synthetic: completion lost")
}

// The work and the key's answer commit together (audit 2026-10-05, RS-06):
// even when the separate completion would have failed, the key is settled and
// a retry replays instead of writing a second row.
func TestTheAnswerIsRecordedInTheWorksOwnTransaction(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	people := make([]string, 2)
	for i := range people {
		if err := pool.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Khoá cùng giao dịch (dữ liệu mẫu)') RETURNING id`).Scan(&people[i]); err != nil {
			t.Fatal(err)
		}
	}
	reporter, target := people[0], people[1]
	key := "idem-in-tx-" + reporter
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE idempotency_key=$1`, key)
		_, _ = pool.Exec(ctx, `DELETE FROM reports WHERE reporter_id=$1`, reporter)
		_, _ = pool.Exec(ctx, `DELETE FROM people WHERE id = ANY($1)`, people)
	})
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now}
	h := coreWithEnv(t, env, idem.New(completeFails{idem.NewPostgresStore(pool)}))
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(`{"target_type":"person","target_id":"`+target+`","reason":"spam"}`))
		req.Header.Set("X-Actor-ID", reporter)
		req.Header.Set("X-Actor-Roles", "member")
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
	var status int
	var body []byte
	if err := pool.QueryRow(ctx, `SELECT response_status, response_body FROM idempotency_keys WHERE idempotency_key=$1 AND completed_at IS NOT NULL`, key).Scan(&status, &body); err != nil {
		t.Fatalf("the key was not settled with the work: %v", err)
	}
	if status != 201 || string(body) != first.Body.String() {
		t.Fatalf("stored %d %q, answered %q", status, body, first.Body.String())
	}
	again := send()
	if again.Code != 201 || again.Header().Get(idem.ReplayHeaderName) != "true" || again.Body.String() != first.Body.String() {
		t.Fatalf("retry %d: %s", again.Code, again.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE reporter_id=$1`, reporter).Scan(&count); err != nil || count != 1 {
		t.Fatalf("reports written: %d %v", count, err)
	}
}
