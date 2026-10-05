//go:build postgres

package accountauth

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPostgresRecoveryEnumerationResendAndEmailRotation(t *testing.T) {
	h := authWorld(t)
	person, token := signup(t, h, "recovery_a")
	_, other := signup(t, h, "recovery_b")
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, known := call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": "recovery_a@example.test"})
	requireCode(t, status, 202, known)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, unknown := call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": "missing@example.test"})
	requireCode(t, status, 202, unknown)
	for i, c := range []map[string]any{known, unknown} {
		code := "999999"
		if i == 0 {
			if code == mailCode(t, h, c["challenge_id"].(string)) {
				code = "000000"
			}
		}
		status, out := call(t, h, "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": c["challenge_id"].(string), "challenge_secret": c["challenge_secret"].(string), "code": code, "password": "replacement synthetic recovery password"})
		requireCode(t, status, 401, out)
		if out["code"] != "code_invalid" {
			t.Fatalf("reset existence leaked: %v", out["code"])
		}
	}
	var unknownMails int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM account_mail_outbox WHERE challenge_id=$1`, unknown["challenge_id"]).Scan(&unknownMails); err != nil || unknownMails != 0 {
		t.Fatal("unknown recipient queued mail", err)
	}
	old := proof{known["challenge_id"].(string), known["challenge_secret"].(string), mailCode(t, h, known["challenge_id"].(string))}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, out := call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": "recovery_a@example.test"})
	requireCode(t, status, 429, out)
	if _, err := h.pool.Exec(context.Background(), `UPDATE account_challenges SET created_at=clock_timestamp()-interval '61 seconds' WHERE id=$1`, old.ID); err != nil {
		t.Fatal(err)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, out = call(t, h, "/auth/password/reset/request", "POST", "", map[string]string{"email": "recovery_a@example.test"})
	requireCode(t, status, 202, out)
	status, out = call(t, h, "/auth/password/reset/confirm", "POST", "", map[string]string{"challenge_id": old.ID, "challenge_secret": old.Secret, "code": old.Code, "password": "replacement synthetic recovery password"})
	requireCode(t, status, 401, out)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, email := call(t, h, "/people/me/account/email", "POST", token, map[string]string{"email": "replacement@example.test"})
	requireCode(t, status, 202, email)
	newEmail := proof{email["challenge_id"].(string), email["challenge_secret"].(string), mailCode(t, h, email["challenge_id"].(string))}
	status, out = call(t, h, "/people/me/account/email/verify", "POST", other, newEmail)
	requireCode(t, status, 401, out)
	// A valid proof used by another account is spent, never transferred.
	status, out = call(t, h, "/people/me/account/email/verify", "POST", token, newEmail)
	requireCode(t, status, 401, out)
	if _, err := h.pool.Exec(context.Background(), `UPDATE account_challenges SET created_at=clock_timestamp()-interval '61 seconds' WHERE id=$1`, newEmail.ID); err != nil {
		t.Fatal(err)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, email = call(t, h, "/people/me/account/email", "POST", token, map[string]string{"email": "replacement@example.test"})
	requireCode(t, status, 202, email)
	newEmail = proof{email["challenge_id"].(string), email["challenge_secret"].(string), mailCode(t, h, email["challenge_id"].(string))}
	status, out = call(t, h, "/people/me/account/email/verify", "POST", token, newEmail)
	requireCode(t, status, 200, out)
	if out["person_id"] != person {
		t.Fatal("email changed person")
	}
	fresh := out["token"].(string)
	status, out = call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, status, 401, out)
	status, out = call(t, h, "/people/me/account", "GET", fresh, nil)
	requireCode(t, status, 200, out)
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	if out["email"] != "replacement@example.test" {
		t.Fatal("email not updated")
	}
}

func TestPostgresGoogleLinkReauthUnlinkAndLogoutAll(t *testing.T) {
	h := authWorld(t)
	person, token := signup(t, h, "google_link_a")
	_, other := signup(t, h, "google_link_b")
	freshProof := func(purpose, actor string) googleInput {
		status, c := call(t, h, "/auth/google/challenge", "POST", actor, map[string]string{"purpose": purpose})
		requireCode(t, status, 201, c)
		h.cfg.Google = testGoogle{GoogleClaims{Issuer: "https://accounts.google.com", Subject: "synthetic-link-sub", Nonce: c["nonce"].(string)}}
		return googleInput{proof: proof{ID: c["challenge_id"].(string), Secret: c["challenge_secret"].(string)}, IDToken: "synthetic"}
	}
	h.cfg.Google = testGoogle{}
	in := freshProof("link", token)
	status, out := call(t, h, "/people/me/account/google", "POST", other, in)
	requireCode(t, status, 401, out)
	in = freshProof("link", token)
	status, out = call(t, h, "/people/me/account/google", "POST", token, in)
	requireCode(t, status, 200, out)
	if out["person_id"] != person {
		t.Fatal("link moved identity")
	}
	token = out["token"].(string)
	status, out = call(t, h, "/people/me/account/google", "POST", token, in)
	requireCode(t, status, 401, out)
	in = freshProof("reauth", token)
	status, out = call(t, h, "/people/me/account/reauth", "POST", token, map[string]any{"google": in})
	requireCode(t, status, 200, out)
	status, out = call(t, h, "/people/me/account/google", "DELETE", token, nil)
	requireCode(t, status, 200, out)
	fresh := out["token"].(string)
	status, out = call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, status, 401, out)
	status, out = call(t, h, "/sessions/all", "DELETE", fresh, nil)
	requireCode(t, status, 204, out)
	status, out = call(t, h, "/people/me/account", "GET", fresh, nil)
	requireCode(t, status, 401, out)
}

type recordingSender struct {
	mu    sync.Mutex
	calls int
	fail  bool
}

func (s *recordingSender) Send(_ context.Context, p mailPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if p.Email == "" || len(p.Code) != 6 {
		return errors.New("invalid synthetic mail")
	}
	if s.fail {
		return errors.New("synthetic delivery failure")
	}
	return nil
}
func TestPostgresMailRetryClaimsAndSpentProofSuppression(t *testing.T) {
	h := authWorld(t)
	sender := &recordingSender{fail: true}
	h.cfg.Sender = sender
	var logs bytes.Buffer
	h.cfg.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, c := call(t, h, "/auth/register", "POST", "", map[string]string{"username": "mail_a", "email": "mail_a@example.test", "password": "synthetic mail account credential"})
	requireCode(t, status, 202, c)
	h.deliverOne(context.Background())
	if !strings.Contains(logs.String(), "delivery") || strings.Contains(logs.String(), "mail_a") || strings.Contains(logs.String(), mailCode(t, h, c["challenge_id"].(string))) {
		t.Fatal("mail failure logging leaked a secret or hid the failure")
	}
	var attempts int
	var done *time.Time
	var retry time.Time
	if err := h.pool.QueryRow(context.Background(), `SELECT attempts,done_at,next_attempt_at FROM account_mail_outbox WHERE challenge_id=$1`, c["challenge_id"]).Scan(&attempts, &done, &retry); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || done != nil || !retry.After(time.Now()) {
		t.Fatal("retry not bounded/scheduled")
	}
	sender.fail = false
	if _, err := h.pool.Exec(context.Background(), `UPDATE account_mail_outbox SET next_attempt_at=clock_timestamp()`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); h.deliverOne(context.Background()) }()
	}
	wg.Wait()
	var remaining int
	if err := h.pool.QueryRow(context.Background(), `SELECT attempts,done_at,octet_length(payload_cipher) FROM account_mail_outbox WHERE challenge_id=$1`, c["challenge_id"]).Scan(&attempts, &done, &remaining); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || done == nil || remaining != 0 || sender.calls != 2 {
		t.Fatalf("delivery was not exclusively claimed: attempts=%d calls=%d", attempts, sender.calls)
	}
	// repo-guard: allow=email reason=synthetic-reserved-test-domain
	status, c = call(t, h, "/auth/register", "POST", "", map[string]string{"username": "mail_b", "email": "mail_b@example.test", "password": "synthetic mail account credential"})
	requireCode(t, status, 202, c)
	p := proof{c["challenge_id"].(string), c["challenge_secret"].(string), mailCode(t, h, c["challenge_id"].(string))}
	status, out := call(t, h, "/auth/register/verify", "POST", "", p)
	requireCode(t, status, 201, out)
	h.deliverOne(context.Background())
	if sender.calls != 2 {
		t.Fatal("spent OTP mailed")
	}
}

func TestPostgresAccountErasureAndDiscoveryCompatibility(t *testing.T) {
	h := authWorld(t)
	person, token := signup(t, h, "erase_a")
	var oldFlag bool
	if err := h.pool.QueryRow(context.Background(), `SELECT discoverable_by_phone FROM people WHERE id=$1`, person).Scan(&oldFlag); err != nil || !oldFlag {
		t.Fatal("social discovery bridge", err)
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE people SET discoverable_by_phone=false WHERE id=$1`, person); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(context.Background(), `SELECT discoverable_by_phone FROM people WHERE id=$1`, person).Scan(&oldFlag); err != nil || !oldFlag {
		t.Fatal("legacy writer changed managed discovery", err)
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE people SET deleted_at=clock_timestamp() WHERE id=$1`, person); err != nil {
		t.Fatal(err)
	}
	status, out := call(t, h, "/people/me/account", "GET", token, nil)
	requireCode(t, status, 401, out)
	var credentials int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM managed_accounts WHERE person_id=$1`, person).Scan(&credentials); err != nil || credentials != 0 {
		t.Fatal("erasure kept credentials", err)
	}
}
