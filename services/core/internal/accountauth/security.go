package accountauth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) account(w http.ResponseWriter, r *http.Request) {
	person, _, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var username string
	var emailCipher []byte
	var hasPassword, hasGoogle, discoverable bool
	err = h.pool.QueryRow(r.Context(), `SELECT username,email_cipher,password_hash IS NOT NULL,google_subject IS NOT NULL,discoverable FROM managed_accounts WHERE person_id=$1`, person).Scan(&username, &emailCipher, &hasPassword, &hasGoogle, &discoverable)
	if err != nil {
		refuse(w, err)
		return
	}
	email := ""
	if emailCipher != nil {
		if err = h.cfg.Vault.open("email:"+person, emailCipher, &email); err != nil {
			refuse(w, err)
			return
		}
	}
	respond(w, 200, map[string]any{"username": username, "email": email, "has_password": hasPassword, "has_google": hasGoogle, "discoverable_by_username": discoverable})
}
func (h *Handler) reauth(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in struct {
		Password string       `json:"password"`
		Google   *googleInput `json:"google"`
	}
	if err = decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	// A stolen session must not become a password oracle: failures are capped
	// per person, and a correct proof is never slowed by a stranger's tries.
	budgets := []budget{{"reauth-fail", person, 10, 15 * time.Minute}, {"reauth-fail-day", person, 30, 24 * time.Hour}}
	if err = h.spent(r.Context(), budgets...); err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, false); err != nil {
		refuse(w, err)
		return
	}
	var encoded, issuer, subject string
	if err = tx.QueryRow(r.Context(), `SELECT coalesce(password_hash,''),coalesce(google_issuer,''),coalesce(google_subject,'') FROM managed_accounts WHERE person_id=$1`, person).Scan(&encoded, &issuer, &subject); err != nil {
		refuse(w, err)
		return
	}
	if in.Google != nil && in.Password == "" {
		p, e := h.googleProof(r.Context(), tx, *in.Google, "reauth")
		err = e
		if err == nil && (p.Person != person || p.Session != session || p.Google.Issuer != issuer || p.Google.Subject != subject) {
			err = problem(401, "credentials_invalid")
		}
	} else if in.Google == nil && len([]rune(in.Password)) <= 128 && encoded != "" {
		var ok bool
		ok, err = h.checkPassword(r.Context(), in.Password, encoded)
		if err == nil && !ok {
			err = problem(401, "credentials_invalid")
		}
	} else {
		err = problem(401, "credentials_invalid")
	}
	if err != nil {
		var e *Error
		if errors.As(err, &e) && e.Status == 401 {
			if failed := h.fail(r.Context(), budgets...); failed != nil {
				err = failed
			}
		}
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE account_sessions SET reauthenticated_at=clock_timestamp() WHERE id=$1`, session)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, map[string]int{"valid_for_seconds": 300})
}
func (h *Handler) rotate(ctx context.Context, tx pgx.Tx, person, via string) (map[string]any, error) {
	if _, err := tx.Exec(ctx, `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1 AND revoked_at IS NULL`, person); err != nil {
		return nil, err
	}
	return h.mintSession(ctx, tx, person, via, false)
}
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err = decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	password, err := Password(in.Password)
	if err != nil {
		refuse(w, err)
		return
	}
	hashed, err := h.hash(r.Context(), password)
	if err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, true); err != nil {
		refuse(w, err)
		return
	}
	var email bool
	if err = tx.QueryRow(r.Context(), `SELECT email_digest IS NOT NULL FROM managed_accounts WHERE person_id=$1`, person).Scan(&email); err != nil {
		refuse(w, err)
		return
	}
	if !email {
		refuse(w, problem(409, "verified_email_required"))
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE managed_accounts SET password_hash=$2 WHERE person_id=$1`, person, hashed); err != nil {
		refuse(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE account_challenges SET consumed_at=clock_timestamp() WHERE person_id=$1 AND kind='reset' AND consumed_at IS NULL`, person); err != nil {
		refuse(w, err)
		return
	}
	out, err := h.rotate(r.Context(), tx, person, "password")
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, out)
}
func (h *Handler) changeEmail(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if err = decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	email, err := Email(in.Email)
	if err != nil {
		refuse(w, err)
		return
	}
	if err = h.limit(r.Context(), "email-change", person, 5, time.Hour); err != nil {
		refuse(w, err)
		return
	}
	if err = h.issuing(r, email); err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, true); err != nil {
		refuse(w, err)
		return
	}
	// Whether another account holds this email is said only after its OTP
	// (verifyEmail), so the request cannot be used to test addresses.
	out, err := h.newChallenge(r.Context(), tx, "email", email, person, pending{Email: email, Person: person, Session: session}, true)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 202, out)
}
func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in proof
	if err = decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, true); err != nil {
		refuse(w, err)
		return
	}
	p, err := h.consume(r.Context(), tx, "email", in)
	if err != nil {
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	if p.Person != person || p.Session != session {
		refuse(w, commit(r.Context(), tx, problem(401, "challenge_invalid")))
		return
	}
	ciphertext, err := h.cfg.Vault.seal("email:"+person, p.Email)
	if err != nil {
		refuse(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE managed_accounts SET email_digest=$2,email_cipher=$3 WHERE person_id=$1`, person, h.cfg.Vault.mac("email", p.Email), ciphertext); err != nil {
		if conflict(err) != err {
			// Another account holds the address; told only now, after its OTP.
			err = problem(409, "email_unavailable")
		}
		refuse(w, err)
		return
	}
	// Recovery proofs for the previous email must not remain usable.
	if _, err = tx.Exec(r.Context(), `UPDATE account_challenges SET consumed_at=clock_timestamp() WHERE person_id=$1 AND kind IN ('reset','email') AND consumed_at IS NULL`, person); err != nil {
		refuse(w, err)
		return
	}
	var via string
	if err = tx.QueryRow(r.Context(), `SELECT CASE WHEN password_hash IS NULL THEN 'google' ELSE 'password' END FROM managed_accounts WHERE person_id=$1`, person).Scan(&via); err != nil {
		refuse(w, err)
		return
	}
	out, err := h.rotate(r.Context(), tx, person, via)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, out)
}
func (h *Handler) unlinkGoogle(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, true); err != nil {
		refuse(w, err)
		return
	}
	var local bool
	if err = tx.QueryRow(r.Context(), `SELECT password_hash IS NOT NULL AND email_digest IS NOT NULL FROM managed_accounts WHERE person_id=$1`, person).Scan(&local); err != nil {
		refuse(w, err)
		return
	}
	if !local {
		refuse(w, problem(409, "last_login_method"))
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE managed_accounts SET google_issuer=NULL,google_subject=NULL WHERE person_id=$1`, person); err != nil {
		refuse(w, err)
		return
	}
	out, err := h.rotate(r.Context(), tx, person, "password")
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, out)
}
func (h *Handler) logoutAll(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, false); err != nil {
		refuse(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1 AND revoked_at IS NULL`, person)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 204, nil)
}
func (h *Handler) discovery(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in struct {
		Discoverable *bool `json:"discoverable_by_username"`
	}
	if err = decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	if in.Discoverable == nil {
		refuse(w, problem(422, "discovery_required"))
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = h.lockAccount(r.Context(), tx, person, session, false); err != nil {
		refuse(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE managed_accounts SET discoverable=$2 WHERE person_id=$1`, person, *in.Discoverable)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, map[string]bool{"discoverable_by_username": *in.Discoverable})
}
func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	person, _, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in struct {
		Username string `json:"username"`
	}
	if err = decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	username, err := Username(in.Username)
	if err != nil {
		refuse(w, err)
		return
	}
	if err = h.limit(r.Context(), "lookup", person, 30, time.Minute); err != nil {
		refuse(w, err)
		return
	}
	var id, name string
	err = h.pool.QueryRow(r.Context(), `SELECT m.person_id::text,p.display_name FROM managed_accounts m JOIN people p ON p.id=m.person_id WHERE username=$1 AND discoverable AND p.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=$2 AND f.addressee_id=m.person_id) OR (f.addressee_id=$2 AND f.requester_id=m.person_id)))`, username, person).Scan(&id, &name)
	if err == pgx.ErrNoRows {
		err = problem(404, "person_not_found")
	}
	if err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, map[string]string{"person_id": id, "display_name": name, "username": username})
}
