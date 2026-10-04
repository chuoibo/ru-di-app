//go:build postgres

package loimoi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
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

func exec(t *testing.T, q repo.Querier, sql string, a ...any) {
	t.Helper()
	if _, e := q.Exec(bg, sql, a...); e != nil {
		t.Fatal(e)
	}
}

// world: an invited binh in anh's group of two, chi in nobody's group, and a
// pair between anh and binh (a pair has no invitation step).
type world struct {
	pool                 *pgxpool.Pool
	h                    *Handler
	group, pair          string
	anh, binh, chi, dung string
	tokens               map[string]string
}

func setup(t *testing.T) world {
	t.Helper()
	base := testdb.Pool(t)
	schema := "loi_moi_" + strings.ReplaceAll(id(), "-", "")
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
	for _, table := range []string{"people", "contexts", "memberships", "account_sessions"} {
		exec(t, pool, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	w := world{pool: pool, h: New(pool, "prod"), anh: id(), binh: id(), chi: id(), dung: id(), group: id(), pair: id(), tokens: map[string]string{}}
	names := map[string]string{w.anh: "Synthetic Anh", w.binh: "Synthetic Binh", w.chi: "Synthetic Chi", w.dung: "Synthetic Dung"}
	for p, name := range names {
		token := "synthetic-" + id()
		w.tokens[p] = token
		exec(t, pool, `INSERT INTO people(id,display_name) VALUES($1,$2)`, p, name)
		exec(t, pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, id(), p, auth.TokenDigest(token))
	}
	exec(t, pool, `INSERT INTO contexts(id,display_name,created_by_id) VALUES($1,'Synthetic hội cuối tuần',$2)`, w.group, w.anh)
	exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,joined_at) VALUES($1,$2,$3,'active','admin',now())`, id(), w.group, w.anh)
	exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,joined_at) VALUES($1,$2,$3,'active','member',now())`, id(), w.group, w.dung)
	exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,invited_by_id) VALUES($1,$2,$3,'invited','member',$4)`, id(), w.group, w.binh, w.anh)
	exec(t, pool, `INSERT INTO contexts(id,display_name,created_by_id,kind,pair_key) VALUES($1,'',$2,'pair',$3)`, w.pair, w.anh, w.anh+":"+w.binh)
	exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,joined_at) VALUES($1,$2,$3,'active','member',now())`, id(), w.pair, w.anh)
	exec(t, pool, `INSERT INTO memberships(id,context_id,person_id,state,role,invited_by_id) VALUES($1,$2,$3,'invited','member',$4)`, id(), w.pair, w.binh, w.anh)
	return w
}

func (w world) call(t *testing.T, method, context, who string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/contexts/"+context+"/invitation", nil)
	r.Header.Set("Authorization", "Bearer "+w.tokens[who])
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, r)
	return rec
}

func code(rec *httptest.ResponseRecorder) string {
	var e apiError
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return e.Code
}

// QA UI-080: the invited person reads who invited them and how many are in,
// before answering; nobody else reads an invitation that is not theirs.
func TestPostgresInvitationNamesTheInviter(t *testing.T) {
	w := setup(t)
	rec := w.call(t, http.MethodGet, w.group, w.binh)
	if rec.Code != 200 {
		t.Fatalf("invited person: %d %s", rec.Code, rec.Body.String())
	}
	var inv Invitation
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatal(err)
	}
	if inv.InvitedBy == nil || inv.InvitedBy.DisplayName != "Synthetic Anh" || inv.InvitedBy.ID != w.anh {
		t.Fatalf("inviter: %+v", inv.InvitedBy)
	}
	if inv.MemberCount != 2 || inv.DisplayName != "Synthetic hội cuối tuần" || inv.ContextID != w.group {
		t.Fatalf("invitation: %+v", inv)
	}
	for _, who := range []string{w.anh, w.chi, w.dung} {
		if rec := w.call(t, http.MethodGet, w.group, who); rec.Code != 404 || code(rec) != "invitation_not_found" {
			t.Fatalf("not invited: %d %s", rec.Code, rec.Body.String())
		}
	}
	if rec := w.call(t, http.MethodGet, w.pair, w.binh); rec.Code != 404 {
		t.Fatalf("a pair has no invitation to read: %d", rec.Code)
	}
	// An inviter whose account is gone is not named.
	exec(t, w.pool, `UPDATE people SET deleted_at=now() WHERE id=$1`, w.anh)
	rec = w.call(t, http.MethodGet, w.group, w.binh)
	_ = json.Unmarshal(rec.Body.Bytes(), &inv)
	if rec.Code != 200 || inv.InvitedBy != nil {
		t.Fatalf("deleted inviter still named: %d %s", rec.Code, rec.Body.String())
	}
}

// QA UI-080: «Từ chối» closes only the person's own invitation, once; a later
// invitation to the same group can start again.
func TestPostgresInvitationDeclineClosesOnlyMine(t *testing.T) {
	w := setup(t)
	if rec := w.call(t, http.MethodDelete, w.group, w.chi); rec.Code != 404 {
		t.Fatalf("somebody not invited declined: %d", rec.Code)
	}
	if rec := w.call(t, http.MethodDelete, w.group, w.dung); rec.Code != 404 {
		t.Fatalf("an active member left through the invitation route: %d", rec.Code)
	}
	if rec := w.call(t, http.MethodDelete, w.pair, w.binh); rec.Code != 404 {
		t.Fatalf("a pair invitation was declined: %d", rec.Code)
	}
	if rec := w.call(t, http.MethodDelete, w.group, w.binh); rec.Code != 204 {
		t.Fatalf("decline: %d %s", rec.Code, rec.Body.String())
	}
	var state string
	var leftSet bool
	if err := w.pool.QueryRow(bg, `SELECT state, left_at IS NOT NULL FROM memberships WHERE context_id=$1 AND person_id=$2`, w.group, w.binh).Scan(&state, &leftSet); err != nil {
		t.Fatal(err)
	}
	if state != "left" || !leftSet {
		t.Fatalf("declined row: %s left_at %v", state, leftSet)
	}
	var active int
	if err := w.pool.QueryRow(bg, `SELECT count(*) FROM memberships WHERE context_id=$1 AND state='active'`, w.group).Scan(&active); err != nil || active != 2 {
		t.Fatalf("members after a decline: %d %v", active, err)
	}
	if rec := w.call(t, http.MethodDelete, w.group, w.binh); rec.Code != 404 {
		t.Fatalf("second decline: %d", rec.Code)
	}
	if rec := w.call(t, http.MethodGet, w.group, w.binh); rec.Code != 404 {
		t.Fatalf("declined invitation still readable: %d", rec.Code)
	}
	exec(t, w.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,invited_by_id) VALUES($1,$2,$3,'invited','member',$4)`, id(), w.group, w.binh, w.anh)
	if rec := w.call(t, http.MethodGet, w.group, w.binh); rec.Code != 200 {
		t.Fatalf("a new invitation after a decline: %d", rec.Code)
	}
}
