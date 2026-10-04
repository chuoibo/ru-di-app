//go:build postgres

package accountauth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mobile/services/core/internal/auth"
)

// memLimiter counts like the Redis limiter, in one process, for tests that
// assert which budget a request spends. Production never falls back to it.
type memLimiter struct {
	mu sync.Mutex
	n  map[string]int
}

func newMemLimiter() *memLimiter { return &memLimiter{n: map[string]int{}} }
func (m *memLimiter) Allow(_ context.Context, key string, max int, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n[key]++
	return m.n[key] <= max, nil
}
func (m *memLimiter) Count(_ context.Context, key string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n[key], nil
}
func (m *memLimiter) Undo(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n[key] > 0 {
		m.n[key]--
	}
	return nil
}
func (m *memLimiter) Clear(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.n, key)
	return nil
}

func callFrom(t *testing.T, h *Handler, ip, path, method, token string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.RemoteAddr = ip + ":40000"
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

func TestPostgresLoginFailuresPauseTheGuesserNotTheOwner(t *testing.T) {
	h := authWorld(t)
	h.cfg.Limits = newMemLimiter()
	signup(t, h, "synthetic_victim")
	right := map[string]string{"username": "synthetic_victim", "password": "synthetic long credential synthetic_victim"}
	wrong := map[string]string{"username": "synthetic_victim", "password": "not the synthetic credential"}
	for range 10 {
		s, out := callFrom(t, h, "192.0.2.66", "/auth/login", "POST", "", wrong)
		requireCode(t, s, 401, out)
	}
	// The guessing address is paused for this username, even with the right password.
	s, out := callFrom(t, h, "192.0.2.66", "/auth/login", "POST", "", right)
	requireCode(t, s, 429, out)
	// The owner elsewhere is not.
	s, out = callFrom(t, h, "192.0.2.10", "/auth/login", "POST", "", right)
	requireCode(t, s, 201, out)
	// A hundred failures from many addresses in a day pause the username itself.
	for i := range 9 {
		for range 10 {
			s, out = callFrom(t, h, "203.0.113."+string(rune('1'+i)), "/auth/login", "POST", "", wrong)
			requireCode(t, s, 401, out)
		}
	}
	s, out = callFrom(t, h, "192.0.2.20", "/auth/login", "POST", "", right)
	requireCode(t, s, 429, out)
	// Except from where the owner signed in before.
	s, out = callFrom(t, h, "192.0.2.10", "/auth/login", "POST", "", right)
	requireCode(t, s, 201, out)
	// Proving the mailbox lifts that pause everywhere.
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out = callFrom(t, h, "192.0.2.20", "/auth/password/reset/request", "POST", "", map[string]string{"email": "synthetic_victim@example.test"})
	requireCode(t, s, 202, out)
	reset := map[string]string{"challenge_id": out["challenge_id"].(string), "challenge_secret": out["challenge_secret"].(string), "code": mailCode(t, h, out["challenge_id"].(string)), "password": "replacement synthetic victim credential"}
	s, out = callFrom(t, h, "192.0.2.20", "/auth/password/reset/confirm", "POST", "", reset)
	requireCode(t, s, 200, out)
	s, out = callFrom(t, h, "192.0.2.20", "/auth/login", "POST", "", map[string]string{"username": "synthetic_victim", "password": reset["password"]})
	requireCode(t, s, 201, out)
}

func TestPostgresConcurrentGuessesCannotOutrunTheBudget(t *testing.T) {
	h := authWorld(t)
	h.cfg.Limits = newMemLimiter()
	h.cfg.HashSlots = 16
	h.hashes = make(chan struct{}, 16)
	signup(t, h, "synthetic_burst")
	var mu sync.Mutex
	codes := map[int]int{}
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := callFrom(t, h, "192.0.2.66", "/auth/login", "POST", "", map[string]string{"username": "synthetic_burst", "password": "not the synthetic credential"})
			mu.Lock()
			codes[s]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	// Checked before counted, all thirty would have reached Argon2.
	if codes[401] != 10 || codes[429] != 20 {
		t.Fatalf("burst outcomes %v", codes)
	}
}

func TestPostgresWrongCodesAreCappedPerEmailAcrossResends(t *testing.T) {
	h := authWorld(t)
	signup(t, h, "synthetic_codes")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	email := map[string]string{"email": "synthetic_codes@example.test"}
	round := func() map[string]any {
		t.Helper()
		if _, e := h.pool.Exec(context.Background(), `UPDATE account_challenges SET created_at=created_at-interval '61 seconds' WHERE kind='reset'`); e != nil {
			t.Fatal(e)
		}
		s, out := call(t, h, "/auth/password/reset/request", "POST", "", email)
		requireCode(t, s, 202, out)
		return out
	}
	guessWrong := func(c map[string]any, times int) {
		t.Helper()
		code := "999999"
		if code == mailCode(t, h, c["challenge_id"].(string)) {
			code = "000000"
		}
		for range times {
			s, out := call(t, h, "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": c["challenge_id"].(string), "challenge_secret": c["challenge_secret"].(string), "code": code, "password": "replacement synthetic codes credential"})
			requireCode(t, s, 401, out)
		}
	}
	first := round()
	guessWrong(first, 5)
	second := round()
	// Racing challenges' failures (several alive at once, from elsewhere)
	// count too: with them the email has spent its thirty a day, and even
	// the right code is refused.
	if _, e := h.pool.Exec(context.Background(), `INSERT INTO account_challenges(id,kind,subject_digest,binding_digest,code_digest,payload_cipher,person_id,created_at,expires_at,attempts,consumed_at) SELECT gen_random_uuid(),kind,subject_digest,binding_digest,code_digest,payload_cipher,person_id,created_at,expires_at,5,consumed_at FROM account_challenges, generate_series(1,5) WHERE id=$1`, first["challenge_id"]); e != nil {
		t.Fatal(e)
	}
	s, out := call(t, h, "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": second["challenge_id"].(string), "challenge_secret": second["challenge_secret"].(string), "code": mailCode(t, h, second["challenge_id"].(string)), "password": "replacement synthetic codes credential"})
	requireCode(t, s, 429, out)
	if out["code"] != "challenge_attempts_exhausted" {
		t.Fatalf("code %v", out["code"])
	}
	// And no new code is issued for that email within the day.
	if _, e := h.pool.Exec(context.Background(), `UPDATE account_challenges SET created_at=created_at-interval '61 seconds' WHERE kind='reset'`); e != nil {
		t.Fatal(e)
	}
	s, out = call(t, h, "/auth/password/reset/request", "POST", "", email)
	requireCode(t, s, 429, out)
	if out["code"] != "challenge_attempts_exhausted" {
		t.Fatalf("code %v", out["code"])
	}
}

func TestPostgresEmailChangeSaysNothingBeforeItsCode(t *testing.T) {
	h := authWorld(t)
	_, token := signup(t, h, "synthetic_mover")
	signup(t, h, "synthetic_holder")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out := call(t, h, "/people/me/account/email", "POST", token, map[string]string{"email": "synthetic_holder@example.test"})
	requireCode(t, s, 202, out)
	p := proof{out["challenge_id"].(string), out["challenge_secret"].(string), mailCode(t, h, out["challenge_id"].(string))}
	s, out = call(t, h, "/people/me/account/email/verify", "POST", token, p)
	requireCode(t, s, 409, out)
	if out["code"] != "email_unavailable" {
		t.Fatalf("code %v", out["code"])
	}
	s, out = call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, s, 200, out)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	if out["email"] != "synthetic_mover@example.test" {
		t.Fatal("email moved to an address another account holds")
	}
}

func TestPostgresResetGarbageSpendsNoPasswordHash(t *testing.T) {
	h := authWorld(t)
	// Every hash slot busy: a request that reached Argon2 would wait and fail 503.
	for range cap(h.hashes) {
		h.hashes <- struct{}{}
	}
	start := time.Now()
	s, out := call(t, h, "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "challenge_secret": strings.Repeat("A", 43), "code": "123456", "password": "garbage synthetic credential"})
	requireCode(t, s, 401, out)
	if time.Since(start) > hashWait/2 {
		t.Fatal("garbage reset waited for a hash slot")
	}
}

func TestPostgresMailQuotaRefusesCodesAlike(t *testing.T) {
	h := authWorld(t)
	limits := newMemLimiter()
	h.cfg.Limits = limits
	h.cfg.MailPerDay = 1
	signup(t, h, "synthetic_quota")
	if _, e := limits.Allow(context.Background(), h.rateKey("smtp-day", mailDay()), 1, time.Hour); e != nil {
		t.Fatal(e)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	for _, email := range []string{"synthetic_quota@example.test", "nobody@example.test"} {
		s, out := call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": email})
		requireCode(t, s, 503, out)
		if out["code"] != "mail_unavailable" {
			t.Fatalf("code %v", out["code"])
		}
	}
}

func TestPostgresErasureRetiresUsernameAndAddressability(t *testing.T) {
	h := authWorld(t)
	person, _ := signup(t, h, "synthetic_gone")
	// The same single UPDATE repo.ErasePerson issues.
	if _, e := h.pool.Exec(context.Background(), `UPDATE people SET display_name='Người đã rời', discoverable_by_phone=false, deleted_at=clock_timestamp() WHERE id=$1`, person); e != nil {
		t.Fatal(e)
	}
	var addressable bool
	if e := h.pool.QueryRow(context.Background(), `SELECT discoverable_by_phone FROM people WHERE id=$1`, person).Scan(&addressable); e != nil || addressable {
		t.Fatal("erased person stayed addressable", e)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out := call(t, h, "/auth/register", "POST", "", map[string]string{"username": "Synthetic_Gone", "email": "newcomer@example.test", "password": "newcomer synthetic credential"})
	requireCode(t, s, 409, out)
	// Even a proof issued before the check cannot claim it.
	if _, e := h.pool.Exec(context.Background(), `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'x')`); e != nil {
		t.Fatal(e)
	}
	_, e := h.pool.Exec(context.Background(), `INSERT INTO managed_accounts(person_id,username,google_issuer,google_subject) SELECT id,'synthetic_gone','https://accounts.google.com','synthetic-late' FROM people WHERE display_name='x'`)
	if e == nil || conflict(e) == e {
		t.Fatal("retired username reissued", e)
	}
}

func TestPostgresAccountSchemaUpgradesFromVersionOne(t *testing.T) {
	pool := authSchema(t)
	ctx := context.Background()
	// The prelaunch stack: version 1 only.
	if _, e := pool.Exec(ctx, `CREATE TABLE managed_auth_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, schemaV1); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO managed_auth_migrations VALUES(1,$1)`, schemaDigest(schemaV1)); e != nil {
		t.Fatal(e)
	}
	if e := CheckSchema(ctx, pool); e == nil || !strings.Contains(e.Error(), "version 2") {
		t.Fatal("serving on a version 1 schema", e)
	}
	if e := Migrate(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if e := CheckSchema(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `UPDATE managed_auth_migrations SET digest='edited' WHERE version=2`); e != nil {
		t.Fatal(e)
	}
	if e := Migrate(ctx, pool); e == nil {
		t.Fatal("edited version accepted")
	}
}

func TestPostgresLiveSessionWithoutManagedAccountIsNotSignedOut(t *testing.T) {
	h := authWorld(t)
	ctx := context.Background()
	person, _ := newID()
	session, _ := newID()
	token, _ := randomSecret()
	if _, e := h.pool.Exec(ctx, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic operator')`, person); e != nil {
		t.Fatal(e)
	}
	if _, e := h.pool.Exec(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,created_at,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp(),clock_timestamp()+interval '1 day')`, session, person, auth.TokenDigest(token)); e != nil {
		t.Fatal(e)
	}
	// 401 tells a client its session is gone; this one is alive elsewhere.
	s, out := call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, s, 403, out)
	if out["code"] != "managed_account_required" {
		t.Fatalf("code %v", out["code"])
	}
	s, out = call(t, h, "/people/me/account", "GET", "not-a-live-token-"+token, nil)
	requireCode(t, s, 401, out)
}

func TestPostgresWrongCodesFromOneAddressPauseOnlyThatAddress(t *testing.T) {
	h := authWorld(t)
	h.cfg.Limits = newMemLimiter()
	signup(t, h, "synthetic_pair")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	email := map[string]string{"email": "synthetic_pair@example.test"}
	request := func(ip string, want int) map[string]any {
		t.Helper()
		if _, e := h.pool.Exec(context.Background(), `UPDATE account_challenges SET created_at=created_at-interval '61 seconds' WHERE kind='reset'`); e != nil {
			t.Fatal(e)
		}
		s, out := callFrom(t, h, ip, "/auth/password/reset/request", "POST", "", email)
		requireCode(t, s, want, out)
		return out
	}
	for range 2 {
		c := request("192.0.2.66", 202)
		code := "999999"
		if code == mailCode(t, h, c["challenge_id"].(string)) {
			code = "000000"
		}
		for range 5 {
			s, out := callFrom(t, h, "192.0.2.66", "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": c["challenge_id"].(string), "challenge_secret": c["challenge_secret"].(string), "code": code, "password": "replacement synthetic pair credential"})
			requireCode(t, s, 401, out)
		}
	}
	out := request("192.0.2.66", 429)
	if out["code"] != "challenge_attempts_exhausted" {
		t.Fatalf("code %v", out["code"])
	}
	// The owner, elsewhere, still gets a code and it works.
	c := request("192.0.2.10", 202)
	s, out := callFrom(t, h, "192.0.2.10", "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": c["challenge_id"].(string), "challenge_secret": c["challenge_secret"].(string), "code": mailCode(t, h, c["challenge_id"].(string)), "password": "replacement synthetic pair credential"})
	requireCode(t, s, 200, out)
}

func TestPostgresRegistrationsLeaveMailForRecovery(t *testing.T) {
	h := authWorld(t)
	limits := newMemLimiter()
	h.cfg.Limits = limits
	h.cfg.MailPerDay = 5
	signup(t, h, "synthetic_reserve")
	for range 4 {
		if _, e := limits.Allow(context.Background(), h.rateKey("smtp-day", mailDay()), 5, time.Hour); e != nil {
			t.Fatal(e)
		}
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out := call(t, h, "/auth/register", "POST", "", map[string]string{"username": "synthetic_late", "email": "late@example.test", "password": "late synthetic credential"})
	requireCode(t, s, 503, out)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	s, out = call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": "synthetic_reserve@example.test"})
	requireCode(t, s, 202, out)
}
