package accountauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type googleInput struct {
	proof
	IDToken string `json:"id_token"`
}

func (h *Handler) googleChallenge(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Google == nil {
		refuse(w, problem(503, "google_unavailable"))
		return
	}
	var in struct {
		Purpose string `json:"purpose"`
	}
	if err := decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	if in.Purpose == "" {
		in.Purpose = "login"
	}
	if in.Purpose != "login" && in.Purpose != "reauth" && in.Purpose != "link" {
		refuse(w, problem(422, "purpose_invalid"))
		return
	}
	p := pending{Purpose: in.Purpose}
	var err error
	if in.Purpose == "login" {
		// Anonymous challenges are rows; one client cannot fill the table.
		if err = h.limit(r.Context(), "google-challenge-ip", h.clientIP(r), 30, time.Minute); err != nil {
			refuse(w, err)
			return
		}
	} else {
		p.Person, p.Session, err = h.actor(r)
		if err != nil {
			refuse(w, err)
			return
		}
	}
	nonce, err := randomSecret()
	if err != nil {
		refuse(w, err)
		return
	}
	p.Nonce = nonce
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if p.Person != "" {
		if err = h.lockAccount(r.Context(), tx, p.Person, p.Session, in.Purpose == "link"); err != nil {
			refuse(w, err)
			return
		}
	}
	out, err := h.newChallenge(r.Context(), tx, "google", nonce, p.Person, p, false)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 201, map[string]any{"challenge_id": out.ID, "challenge_secret": out.Secret, "nonce": nonce, "expires_in_seconds": 300})
}
func (h *Handler) googleProof(ctx context.Context, tx pgx.Tx, in googleInput, purpose string) (pending, error) {
	if h.cfg.Google == nil {
		return pending{}, problem(503, "google_unavailable")
	}
	if len(in.IDToken) == 0 || len(in.IDToken) > 12000 {
		return pending{}, problem(401, "google_token_invalid")
	}
	claims, err := h.cfg.Google.Verify(ctx, in.IDToken)
	if err != nil {
		return pending{}, err
	}
	p, err := h.consume(ctx, tx, "google", in.proof)
	if err != nil {
		return pending{}, err
	}
	if p.Purpose != purpose || claims.Nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(p.Nonce)) != 1 {
		return pending{}, problem(401, "google_nonce_invalid")
	}
	p.Google = claims
	return p, nil
}
func (h *Handler) googleLogin(w http.ResponseWriter, r *http.Request) {
	var in googleInput
	if err := decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	if err := h.limit(r.Context(), "google-ip", h.clientIP(r), 30, time.Minute); err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	p, err := h.googleProof(r.Context(), tx, in, "login")
	if err != nil {
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	var person string
	err = tx.QueryRow(r.Context(), `SELECT m.person_id::text FROM managed_accounts m JOIN people p ON p.id=m.person_id WHERE google_issuer=$1 AND google_subject=$2 AND p.deleted_at IS NULL`, p.Google.Issuer, p.Google.Subject).Scan(&person)
	if errors.Is(err, pgx.ErrNoRows) {
		// Matching email never selects another person's account. A username completes a new identity.
		p.Person = ""
		p.Session = ""
		p.Email = ""
		if p.Google.EmailVerified {
			p.Email, _ = Email(p.Google.Email)
		}
		out, e := h.newChallenge(r.Context(), tx, "google_register", p.Google.Issuer+":"+p.Google.Subject, "", p, false)
		if e = commit(r.Context(), tx, e); e != nil {
			refuse(w, e)
			return
		}
		respond(w, 200, map[string]any{"registration_required": true, "challenge_id": out.ID, "challenge_secret": out.Secret, "expires_in_seconds": 300})
		return
	}
	if err != nil {
		refuse(w, err)
		return
	}
	if err = lockPerson(r.Context(), tx, person); err != nil {
		refuse(w, err)
		return
	}
	// Linking or unlinking may have raced the unlocked lookup above.
	var stillLinked bool
	err = tx.QueryRow(r.Context(), `SELECT true FROM managed_accounts WHERE person_id=$1 AND google_issuer=$2 AND google_subject=$3 FOR UPDATE`, person, p.Google.Issuer, p.Google.Subject).Scan(&stillLinked)
	if errors.Is(err, pgx.ErrNoRows) {
		err = problem(401, "credentials_invalid")
	}
	if err != nil {
		refuse(w, err)
		return
	}
	out, err := h.mintSession(r.Context(), tx, person, "google", false)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 201, out)
}
func (h *Handler) googleRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		proof
		Username string `json:"username"`
	}
	if err := decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	username, err := Username(in.Username)
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
	p, err := h.consume(r.Context(), tx, "google_register", in.proof)
	if err != nil {
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	p.Username = username
	// A Google account sharing another account's email remains separate and has no recovery email yet.
	if p.Email != "" {
		var used bool
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM managed_accounts WHERE email_digest=$1)`, h.cfg.Vault.mac("email", p.Email)).Scan(&used); err != nil {
			refuse(w, err)
			return
		}
		if used {
			p.Email = ""
		}
	}
	person, err := h.createAccount(r.Context(), tx, p)
	if err != nil {
		refuse(w, err)
		return
	}
	out, err := h.mintSession(r.Context(), tx, person, "google", true)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 201, out)
}
func (h *Handler) linkGoogle(w http.ResponseWriter, r *http.Request) {
	person, session, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var in googleInput
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
	p, err := h.googleProof(r.Context(), tx, in, "link")
	if err != nil {
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	if p.Person != person || p.Session != session {
		refuse(w, commit(r.Context(), tx, problem(401, "challenge_invalid")))
		return
	}
	var old *string
	if err = tx.QueryRow(r.Context(), `SELECT google_subject FROM managed_accounts WHERE person_id=$1`, person).Scan(&old); err != nil {
		refuse(w, err)
		return
	}
	if old != nil {
		refuse(w, commit(r.Context(), tx, problem(409, "google_already_linked")))
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE managed_accounts SET google_issuer=$2,google_subject=$3 WHERE person_id=$1`, person, p.Google.Issuer, p.Google.Subject); err != nil {
		refuse(w, conflict(err))
		return
	}
	out, err := h.rotate(r.Context(), tx, person, "google")
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, out)
}
