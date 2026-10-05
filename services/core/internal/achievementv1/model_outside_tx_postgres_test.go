//go:build postgres

package achievementv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/testdb"
)

// Nếp's call runs after the read transaction committed (audit 2026-10-05,
// PER-ACHIEVE-01): with a pool of one connection, another query gets that
// connection while the model is still thinking. Before, the transaction held
// it for the whole call.
func TestPostgresTheModelCallHoldsNoConnection(t *testing.T) {
	ctx := context.Background()
	base := testdb.Pool(t)
	if err := Migrate(ctx, base); err != nil {
		t.Fatal(err)
	}
	config := base.Config().Copy()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var person string
	if err := base.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Thành tựu (dữ liệu mẫu)') RETURNING id`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	token := "synthetic-achievement-" + person
	if _, err := base.Exec(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES(gen_random_uuid(),$1,$2,'genesis',now()+interval '1 hour')`, person, auth.TokenDigest(token)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = base.Exec(context.Background(), `DELETE FROM account_sessions WHERE person_id=$1`, person)
		_, _ = base.Exec(context.Background(), `DELETE FROM people WHERE id=$1`, person)
	})
	may := motluot.Moi(llm.NewStub(llm.Buoc{Cho: 2 * time.Second, Text: `{"candidate_ids":[],"line":""}`}), 1)
	h := New(pool, "prod").WithAI(may)
	r := httptest.NewRequest("POST", "/me/achievement-suggestions", strings.NewReader(`{"consent":true}`))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); h.ServeHTTP(w, r) }()
	time.Sleep(500 * time.Millisecond)
	probe, cancel := context.WithTimeout(ctx, time.Second)
	_, err = pool.Exec(probe, `SELECT 1`)
	cancel()
	<-done
	if err != nil {
		t.Fatalf("the only connection was held through the model call: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("suggestions %d: %s", w.Code, w.Body.String())
	}
}
