//go:build postgres

package repo

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/testdb"
)

// Fixture ids use hex letters only (the repo guard refuses long digit runs).
const (
	livePerson    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	erasedPerson  = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	activeContext = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	invitedCtx    = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	leftContext   = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
)

func exec(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", strings.Fields(sql)[0]+" "+strings.Fields(sql)[2], err)
	}
}

// fixtures writes the rows through the migrated schema's own constraints: the
// session CHECKs (genesis carries no invite id, expiry after creation) and the
// membership CHECK that ties state 'left' to left_at.
func fixtures(t *testing.T, tx pgx.Tx) {
	t.Helper()
	exec(t, tx, `INSERT INTO people (id, display_name) VALUES ($1, 'Người sống'), ($2, 'Người đã xoá')`, livePerson, erasedPerson)
	exec(t, tx, `UPDATE people SET deleted_at = now() WHERE id = $1`, erasedPerson)
	for _, ctx := range []string{activeContext, invitedCtx, leftContext} {
		exec(t, tx, `INSERT INTO contexts (id, display_name, created_by_id) VALUES ($1, 'Nhóm', $2)`, ctx, livePerson)
	}
	exec(t, tx, `INSERT INTO memberships (id, context_id, person_id, state, joined_at)
	             VALUES (gen_random_uuid(), $1, $2, 'active', now())`, activeContext, livePerson)
	exec(t, tx, `INSERT INTO memberships (id, context_id, person_id, state)
	             VALUES (gen_random_uuid(), $1, $2, 'invited')`, invitedCtx, livePerson)
	exec(t, tx, `INSERT INTO memberships (id, context_id, person_id, state, joined_at, left_at)
	             VALUES (gen_random_uuid(), $1, $2, 'left', now(), now())`, leftContext, livePerson)
	for person, token := range map[string]string{livePerson: "live-token", erasedPerson: "erased-token"} {
		exec(t, tx, `INSERT INTO account_sessions (id, person_id, token_digest, issued_via, expires_at)
		             VALUES (gen_random_uuid(), $1, $2, 'genesis', now() + interval '1 day')`,
			person, auth.TokenDigest(token))
	}
	exec(t, tx, `INSERT INTO account_sessions (id, person_id, token_digest, issued_via, created_at, expires_at, revoked_at)
	             VALUES (gen_random_uuid(), $1, $2, 'genesis', now() - interval '2 day', now() + interval '1 day', now())`,
		livePerson, auth.TokenDigest("revoked-token"))
}

func TestSessionByDigestReadsTheRow(t *testing.T) {
	tx := testdb.Tx(t)
	fixtures(t, tx)
	store := Sessions{Q: tx}

	record, err := store.SessionByDigest(context.Background(), auth.TokenDigest("live-token"))
	if err != nil || record == nil {
		t.Fatalf("record=%+v err=%v", record, err)
	}
	if record.PersonID != livePerson || record.RevokedAt != nil || !record.ExpiresAt.After(time.Now()) {
		t.Fatalf("record = %+v", record)
	}
	missing, err := store.SessionByDigest(context.Background(), auth.TokenDigest("never-issued"))
	if err != nil || missing != nil {
		t.Fatalf("unknown digest: record=%+v err=%v", missing, err)
	}
	revoked, err := store.SessionByDigest(context.Background(), auth.TokenDigest("revoked-token"))
	if err != nil || revoked == nil || revoked.RevokedAt == nil {
		t.Fatalf("revoked session: %+v %v", revoked, err)
	}
}

func TestGrantsFollowTheRoster(t *testing.T) {
	tx := testdb.Tx(t)
	fixtures(t, tx)
	store := Sessions{Q: tx}

	grants, err := store.Grants(context.Background(), livePerson)
	if err != nil {
		t.Fatal(err)
	}
	if !grants.PersonExists || strings.Join(grants.Contexts, ",") != activeContext {
		t.Fatalf("grants = %+v", grants)
	}
	if strings.Join(grants.Roles, ",") != "advancer,creditor,former_member,member,recipient,sender" {
		t.Fatalf("roles = %v", grants.Roles)
	}
	for _, id := range []string{erasedPerson, "ffffffff-ffff-4fff-8fff-ffffffffffff"} {
		g, err := store.Grants(context.Background(), id)
		if err != nil || g.PersonExists {
			t.Fatalf("person %s: grants=%+v err=%v", id, g, err)
		}
	}
}

func TestProdActorEndToEndOnTheMigratedSchema(t *testing.T) {
	tx := testdb.Tx(t)
	fixtures(t, tx)
	store := Sessions{Q: tx}
	now := time.Now().UTC()

	actor, problem, err := auth.ProdActor(context.Background(),
		http.Header{"Authorization": {"Bearer live-token"}}, store, now)
	if err != nil || problem != nil || actor.ID != livePerson {
		t.Fatalf("live: actor=%+v problem=%+v err=%v", actor, problem, err)
	}
	for _, token := range []string{"erased-token", "revoked-token", "never-issued"} {
		actor, problem, err := auth.ProdActor(context.Background(),
			http.Header{"Authorization": {"Bearer " + token}}, store, now)
		if err != nil || actor != nil || problem == nil || problem.Detail != "Session is not valid" {
			t.Fatalf("%s: actor=%+v problem=%+v err=%v", token, actor, problem, err)
		}
	}
}
