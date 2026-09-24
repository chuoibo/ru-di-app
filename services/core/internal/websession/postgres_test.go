//go:build postgres

package websession

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

func TestPostgresLookupAcceptsOnlyALiveSession(t *testing.T) {
	bg := context.Background()
	base := testdb.Pool(t)
	id := func() string {
		s, e := repo.NewUUID()
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	schema := "web_session_" + strings.ReplaceAll(id(), "-", "")
	must := func(q repo.Querier, sql string, a ...any) {
		t.Helper()
		if _, e := q.Exec(bg, sql, a...); e != nil {
			t.Fatal(e)
		}
	}
	must(base, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	t.Cleanup(func() { must(base, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, e := pgxpool.NewWithConfig(bg, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "account_sessions"} {
		must(pool, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	person, gone := id(), id()
	must(pool, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic web person')`, person)
	must(pool, `INSERT INTO people(id,display_name,deleted_at) VALUES($1,'Synthetic erased',now())`, gone)
	session := func(owner, extra string) string {
		token := "synthetic-" + strings.ReplaceAll(id(), "-", "")
		must(pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at`+extra+`) VALUES($1,$2,$3,'otp',now()+interval '1 day'`+map[string]string{"": ")", ",revoked_at": ",now())"}[extra], id(), owner, auth.TokenDigest(token))
		return token
	}
	live, revoked, erased := session(person, ""), session(person, ",revoked_at"), session(gone, "")
	expired := "synthetic-expired"
	must(pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,created_at,expires_at) VALUES($1,$2,$3,'otp',now()-interval '2 days',now()-interval '1 minute')`, id(), person, auth.TokenDigest(expired))

	store := Store{Pool: pool}
	got, err := store.Lookup(bg, live)
	if err != nil || got.PersonID != person || got.Profile == nil || got.Profile.DisplayName != "Synthetic web person" || got.IssuedVia != "otp" {
		t.Fatalf("live: %+v %v", got, err)
	}
	for name, token := range map[string]string{"revoked": revoked, "expired": expired, "erased person": erased, "unknown": "synthetic-none"} {
		if _, err := store.Lookup(bg, token); err != ErrAuthentication {
			t.Errorf("%s: %v, want ErrAuthentication", name, err)
		}
	}
}
