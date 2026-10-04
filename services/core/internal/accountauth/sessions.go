package accountauth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/routes"
)

func (h *Handler) createAccount(ctx context.Context, tx pgx.Tx, p pending) (string, error) {
	person, err := newID()
	if err != nil {
		return "", err
	}
	name := p.Username
	if p.Google.Name != "" {
		name = p.Google.Name
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	if _, err = tx.Exec(ctx, `INSERT INTO people(id,display_name,discoverable_by_phone,notify_prefs,wall_comment_policy) VALUES($1,$2,false,'{}','readers')`, person, name); err != nil {
		return "", err
	}
	var emailDigest, emailCipher []byte
	if p.Email != "" {
		emailDigest = h.cfg.Vault.mac("email", p.Email)
		emailCipher, err = h.cfg.Vault.seal("email:"+person, p.Email)
		if err != nil {
			return "", err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO managed_accounts(person_id,username,email_digest,email_cipher,password_hash,google_issuer,google_subject) VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''))`, person, p.Username, emailDigest, emailCipher, p.PasswordHash, p.Google.Issuer, p.Google.Subject)
	return person, conflict(err)
}
func (h *Handler) mintSession(ctx context.Context, tx pgx.Tx, person, via string, isNew bool) (map[string]any, error) {
	var first bool
	if err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM account_sessions WHERE person_id=$1)`, person).Scan(&first); err != nil {
		return nil, err
	}
	isNew = isNew || first
	token, err := randomSecret()
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	var expires time.Time
	err = tx.QueryRow(ctx, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,created_at,expires_at,reauthenticated_at) VALUES($1,$2,$3,$4,clock_timestamp(),clock_timestamp()+interval '30 days',clock_timestamp()) RETURNING expires_at`, id, person, digest(token), via).Scan(&expires)
	if err != nil {
		return nil, err
	}
	// Bound active sessions; losing a response cannot grow the session table without limit.
	if _, err = tx.Exec(ctx, `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE id IN(SELECT id FROM account_sessions WHERE person_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC,id DESC OFFSET 20)`, person); err != nil {
		return nil, err
	}
	var name, username string
	if err = tx.QueryRow(ctx, `SELECT p.display_name,m.username FROM people p JOIN managed_accounts m ON m.person_id=p.id WHERE p.id=$1`, person).Scan(&name, &username); err != nil {
		return nil, err
	}
	contexts, err := routes.AccountContexts(ctx, tx, person)
	if err != nil {
		return nil, err
	}
	return map[string]any{"token": token, "person_id": person, "expires_at": expires, "issued_via": via, "is_new_person": isNew, "profile": map[string]string{"display_name": name, "username": username}, "contexts": contexts, "context_id": nil, "membership_state": nil, "membership_id": nil}, nil
}
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	username, err := Username(in.Username)
	if err != nil {
		refuse(w, problem(401, "credentials_invalid"))
		return
	}
	if len([]rune(in.Password)) > 128 {
		refuse(w, problem(401, "credentials_invalid"))
		return
	}
	ip := h.clientIP(r)
	// Attempts from one address bound the Argon2 work it can ask for; the
	// failure budgets below decide who may still guess at a username.
	if err = h.limit(r.Context(), "login-ip", ip, 60, time.Minute); err != nil {
		refuse(w, err)
		return
	}
	budgets, err := h.loginBudgets(r.Context(), username, ip)
	if err != nil {
		refuse(w, err)
		return
	}
	// Every attempt holds a unit until it proves itself; only a wrong
	// password keeps it. Whatever else ends the request gives it back.
	if err = h.reserve(r.Context(), budgets...); err != nil {
		refuse(w, err)
		return
	}
	failed := false
	defer func() {
		if !failed {
			h.refund(context.WithoutCancel(r.Context()), budgets...)
		}
	}()
	var person, encoded string
	err = h.pool.QueryRow(r.Context(), `SELECT m.person_id::text,coalesce(m.password_hash,'') FROM managed_accounts m JOIN people p ON p.id=m.person_id WHERE m.username=$1 AND p.deleted_at IS NULL`, username).Scan(&person, &encoded)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		refuse(w, err)
		return
	}
	// The same Argon2 work is performed for nonexistent and Google-only accounts.
	if encoded == "" {
		encoded = dummyPasswordHash
	}
	ok, err := h.checkPassword(r.Context(), in.Password, encoded)
	if err != nil {
		refuse(w, err)
		return
	}
	if !ok || person == "" {
		failed = true
		refuse(w, problem(401, "credentials_invalid"))
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = lockPerson(r.Context(), tx, person); err != nil {
		refuse(w, err)
		return
	}
	var current string
	err = tx.QueryRow(r.Context(), `SELECT password_hash FROM managed_accounts WHERE person_id=$1 FOR UPDATE`, person).Scan(&current)
	if err != nil {
		refuse(w, err)
		return
	}
	if current != encoded {
		refuse(w, problem(401, "credentials_invalid"))
		return
	}
	out, err := h.mintSession(r.Context(), tx, person, "password", false)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	h.forgive(r.Context(), budgets[0])
	// The owner's address is remembered, so a stranger spending the
	// username's day cannot keep them out from where they usually sign in.
	_, _ = h.cfg.Limits.Allow(r.Context(), h.rateKey("login-known", pairKey(username, ip)), 1<<30, 30*24*time.Hour)
	respond(w, 201, out)
}

func pairKey(username, ip string) string { return username + "\x00" + ip }

// Ten wrong passwords from one address pause that address for the username;
// a hundred from one address in an hour pause the address; a hundred from
// anywhere in a day pause the username, except from addresses its owner
// signed in from in the last thirty days, until a reset proves the owner.
func (h *Handler) loginBudgets(ctx context.Context, username, ip string) ([]budget, error) {
	budgets := []budget{
		{"login-fail", pairKey(username, ip), 10, 15 * time.Minute},
		{"login-fail-ip", ip, 100, time.Hour},
	}
	if h.cfg.Limits == nil {
		return nil, problem(503, "auth_temporarily_unavailable")
	}
	known, err := h.cfg.Limits.Count(ctx, h.rateKey("login-known", pairKey(username, ip)))
	if err != nil {
		return nil, problem(503, "auth_temporarily_unavailable")
	}
	if known == 0 {
		budgets = append(budgets, userLoginBudget(username))
	}
	return budgets, nil
}
func userLoginBudget(username string) budget {
	return budget{"login-fail-user", username, 100, 24 * time.Hour}
}

// A valid encoded hash with a random salt and a non-password digest costs the same as a real lookup.
const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
