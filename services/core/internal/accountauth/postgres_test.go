//go:build postgres

package accountauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

type testLimit struct{}

func (testLimit) Allow(context.Context, string, int, time.Duration) (bool, error) { return true, nil }

type testGoogle struct{ claims GoogleClaims }

func (g testGoogle) Verify(context.Context, string) (GoogleClaims, error) { return g.claims, nil }
func authWorld(t *testing.T) *Handler {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	id, _ := newID()
	schema := "accounts_" + strings.ReplaceAll(id, "-", "")
	if _, e := base.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, e := base.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		if e != nil {
			t.Error(e)
		}
	})
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "account_identities", "account_sessions", "contexts", "memberships", "friend_requests"} {
		if _, e = pool.Exec(ctx, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)"); e != nil {
			t.Fatal(e)
		}
	}
	if e = Migrate(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, pool); e != nil {
		t.Fatal("migration repeat", e)
	}
	if e = CheckSchema(ctx, pool); e != nil {
		t.Fatal(e)
	}
	return New(pool, Config{Vault: testVault(t), Limits: testLimit{}, HashSlots: 4})
}
func call(t *testing.T, h *Handler, path, method, token string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}
func requireCode(t *testing.T, got, want int, out map[string]any) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d want %d code %v", got, want, out["code"])
	}
}
func mailCode(t *testing.T, h *Handler, challenge string) string {
	t.Helper()
	var id string
	var cipher []byte
	if e := h.pool.QueryRow(context.Background(), `SELECT id::text,payload_cipher FROM account_mail_outbox WHERE challenge_id=$1`, challenge).Scan(&id, &cipher); e != nil {
		t.Fatal(e)
	}
	var p mailPayload
	if e := h.cfg.Vault.open("mail:"+id, cipher, &p); e != nil {
		t.Fatal(e)
	}
	return p.Code
}
func signup(t *testing.T, h *Handler, username string) (string, string) {
	t.Helper()
	password := "synthetic long credential " + username
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out := call(t, h, "/auth/register", "POST", "", map[string]string{"username": username, "email": username + "@example.test", "password": password})
	requireCode(t, s, 202, out)
	p := proof{out["challenge_id"].(string), out["challenge_secret"].(string), mailCode(t, h, out["challenge_id"].(string))}
	s, out = call(t, h, "/auth/register/verify", "POST", "", p)
	requireCode(t, s, 201, out)
	s, out = call(t, h, "/auth/login", "POST", "", map[string]string{"username": username, "password": password})
	requireCode(t, s, 201, out)
	return out["person_id"].(string), out["token"].(string)
}
func TestPostgresAccountProofResetAndSessionRevocation(t *testing.T) {
	h := authWorld(t)
	person, token := signup(t, h, "synthetic_a")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out := call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": "synthetic_a@example.test"})
	requireCode(t, s, 202, out)
	in := map[string]string{"challenge_id": out["challenge_id"].(string), "challenge_secret": out["challenge_secret"].(string), "code": mailCode(t, h, out["challenge_id"].(string)), "password": "new synthetic credential for a"}
	wrong := map[string]string{}
	for k, v := range in {
		wrong[k] = v
	}
	wrong["challenge_secret"] = strings.Repeat("X", 43)
	s, out = call(t, h, "/auth/password/reset/confirm", "POST", "", wrong)
	requireCode(t, s, 401, out)
	s, out = call(t, h, "/auth/password/reset/confirm", "POST", "", in)
	requireCode(t, s, 200, out)
	s, out = call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, s, 401, out)
	s, out = call(t, h, "/auth/password/reset/confirm", "POST", "", in)
	requireCode(t, s, 401, out)
	s, out = call(t, h, "/auth/login", "POST", "", map[string]string{"username": "synthetic_a", "password": in["password"]})
	requireCode(t, s, 201, out)
	if out["person_id"] != person {
		t.Fatal("reset moved identity")
	}
	// Challenge payload and outbox must not contain the email, code or password in plaintext.
	var raw []byte
	if e := h.pool.QueryRow(context.Background(), `SELECT payload_cipher FROM account_challenges WHERE id=$1`, in["challenge_id"]).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	for _, secret := range []string{in["code"], "synthetic_a@example.test", in["password"]} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("unencrypted secret")
		}
	}
}
func TestPostgresOTPAttemptBudgetExpiryAndDuplicateRace(t *testing.T) {
	h := authWorld(t)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out := call(t, h, "/auth/register", "POST", "", map[string]string{"username": "synthetic_race", "email": "race@example.test", "password": "long synthetic credential race"})
	requireCode(t, s, 202, out)
	p := proof{out["challenge_id"].(string), out["challenge_secret"].(string), mailCode(t, h, out["challenge_id"].(string))}
	var mu sync.Mutex
	success := 0
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, out := call(t, h, "/auth/register/verify", "POST", "", p)
			if s != 201 && s != 401 {
				t.Errorf("race status %d code %v", s, out["code"])
			}
			if s == 201 {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("created %d accounts", success)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out = call(t, h, "/auth/register", "POST", "", map[string]string{"username": "synthetic_budget", "email": "budget@example.test", "password": "long synthetic credential budget"})
	requireCode(t, s, 202, out)
	p = proof{out["challenge_id"].(string), out["challenge_secret"].(string), mailCode(t, h, out["challenge_id"].(string))}
	wrong := p
	wrong.Code = "999999"
	if wrong.Code == p.Code {
		wrong.Code = "000000"
	}
	for range 5 {
		s, out = call(t, h, "/auth/register/verify", "POST", "", wrong)
		requireCode(t, s, 401, out)
	}
	s, out = call(t, h, "/auth/register/verify", "POST", "", p)
	requireCode(t, s, 401, out)
	var attempts int
	if e := h.pool.QueryRow(context.Background(), `SELECT attempts FROM account_challenges WHERE id=$1`, p.ID).Scan(&attempts); e != nil || attempts != 5 {
		t.Fatal("attempt budget not persisted", e, attempts)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out = call(t, h, "/auth/register", "POST", "", map[string]string{"username": "synthetic_expiry", "email": "expiry@example.test", "password": "long synthetic credential expiry"})
	requireCode(t, s, 202, out)
	p = proof{out["challenge_id"].(string), out["challenge_secret"].(string), mailCode(t, h, out["challenge_id"].(string))}
	_, e := h.pool.Exec(context.Background(), `UPDATE account_challenges SET created_at=clock_timestamp()-interval '10 minutes',expires_at=clock_timestamp()-interval '5 minutes' WHERE id=$1`, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	s, out = call(t, h, "/auth/register/verify", "POST", "", p)
	requireCode(t, s, 401, out)
}
func TestPostgresGoogleNonceNoEmailMergeAndSafeLink(t *testing.T) {
	h := authWorld(t)
	local, localToken := signup(t, h, "synthetic_local")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	h.cfg.Google = testGoogle{GoogleClaims{Issuer: "https://accounts.google.com", Subject: "synthetic-sub", Email: "synthetic_local@example.test", EmailVerified: true}}
	s, c := call(t, h, "/auth/google/challenge", "POST", "", map[string]string{"purpose": "login"})
	requireCode(t, s, 201, c)
	in := googleInput{proof: proof{ID: c["challenge_id"].(string), Secret: c["challenge_secret"].(string)}, IDToken: "synthetic"}
	s, out := call(t, h, "/auth/google", "POST", "", in)
	requireCode(t, s, 401, out)
	s, c = call(t, h, "/auth/google/challenge", "POST", "", map[string]string{"purpose": "login"})
	requireCode(t, s, 201, c)
	g := h.cfg.Google.(testGoogle)
	g.claims.Nonce = c["nonce"].(string)
	h.cfg.Google = g
	in = googleInput{proof: proof{ID: c["challenge_id"].(string), Secret: c["challenge_secret"].(string)}, IDToken: "synthetic"}
	s, out = call(t, h, "/auth/google", "POST", "", in)
	requireCode(t, s, 200, out)
	register := map[string]string{"challenge_id": out["challenge_id"].(string), "challenge_secret": out["challenge_secret"].(string), "username": "synthetic_google"}
	s, out = call(t, h, "/auth/google/register", "POST", "", register)
	requireCode(t, s, 201, out)
	googlePerson := out["person_id"].(string)
	if googlePerson == local {
		t.Fatal("matching email merged people")
	}
	s, c = call(t, h, "/auth/google/challenge", "POST", localToken, map[string]string{"purpose": "link"})
	requireCode(t, s, 201, c)
	g.claims.Nonce = c["nonce"].(string)
	h.cfg.Google = g
	in = googleInput{proof: proof{ID: c["challenge_id"].(string), Secret: c["challenge_secret"].(string)}, IDToken: "synthetic"}
	s, out = call(t, h, "/people/me/account/google", "POST", localToken, in)
	requireCode(t, s, 409, out)
	var owner string
	if e := h.pool.QueryRow(context.Background(), `SELECT person_id::text FROM managed_accounts WHERE google_subject=$1`, g.claims.Subject).Scan(&owner); e != nil || owner != googlePerson {
		t.Fatal("identity stolen", e)
	}
}
func TestPostgresUsernameDiscoveryAndRecentAuth(t *testing.T) {
	h := authWorld(t)
	_, token := signup(t, h, "synthetic_find")
	_, other := signup(t, h, "synthetic_other")
	s, out := call(t, h, "/friends/lookup", "POST", other, map[string]string{"username": "@SYNTHETIC_FIND"})
	requireCode(t, s, 200, out)
	s, out = call(t, h, "/people/me/account/discovery", "PUT", token, map[string]bool{"discoverable_by_username": false})
	requireCode(t, s, 200, out)
	s, out = call(t, h, "/friends/lookup", "POST", other, map[string]string{"username": "synthetic_find"})
	requireCode(t, s, 404, out)
	if _, e := h.pool.Exec(context.Background(), `UPDATE account_sessions SET reauthenticated_at=clock_timestamp()-interval '10 minutes'`); e != nil {
		t.Fatal(e)
	}
	s, out = call(t, h, "/people/me/account/password", "PUT", token, map[string]string{"password": "new synthetic credential change"})
	requireCode(t, s, 403, out)
	s, out = call(t, h, "/people/me/account/reauth", "POST", token, map[string]string{"password": "synthetic long credential synthetic_find"})
	requireCode(t, s, 200, out)
	s, out = call(t, h, "/people/me/account/password", "PUT", token, map[string]string{"password": "new synthetic credential change"})
	requireCode(t, s, 200, out)
	s, out = call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, s, 401, out)
	_ = fmt.Sprintf("%v", out["code"])
}
