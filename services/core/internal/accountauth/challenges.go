package accountauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type challengeReply struct {
	ID      string `json:"challenge_id"`
	Secret  string `json:"challenge_secret"`
	Expires int    `json:"expires_in_seconds"`
	Resend  int    `json:"resend_after_seconds"`
}
type proof struct {
	ID     string `json:"challenge_id"`
	Secret string `json:"challenge_secret"`
	Code   string `json:"code"`
}
type pending struct {
	Purpose      string
	Username     string
	Email        string
	PasswordHash string
	Person       string
	Google       GoogleClaims
	Session      string
	Nonce        string
}

func (h *Handler) newChallenge(ctx context.Context, tx pgx.Tx, kind, subject, person string, value pending, send bool) (challengeReply, error) {
	// Serialize issuance per kind+subject across replicas before inspecting history.
	key := fmt.Sprintf("%x", h.cfg.Vault.mac("challenge-lock", kind+subject))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return challengeReply{}, err
	}
	var recent, count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE created_at>clock_timestamp()-interval '60 seconds'),count(*) FROM account_challenges WHERE kind=$1 AND subject_digest=$2 AND created_at>clock_timestamp()-interval '15 minutes'`, kind, h.cfg.Vault.mac("challenge:"+kind, subject)).Scan(&recent, &count); err != nil {
		return challengeReply{}, err
	}
	if kind != "google" && kind != "google_register" && (recent > 0 || count >= 5) {
		return challengeReply{}, problem(429, "challenge_resend_limited")
	}
	id, err := newID()
	if err != nil {
		return challengeReply{}, err
	}
	secret, err := randomSecret()
	if err != nil {
		return challengeReply{}, err
	}
	payload, err := h.cfg.Vault.seal("challenge:"+id, value)
	if err != nil {
		return challengeReply{}, err
	}
	var codeHash []byte
	var code string
	if send || kind == "reset" {
		n, e := rand.Int(rand.Reader, big.NewInt(1000000))
		if e != nil {
			return challengeReply{}, e
		}
		code = fmt.Sprintf("%06d", n)
		codeHash = h.cfg.Vault.mac("otp:"+id, code)
	}
	if _, err = tx.Exec(ctx, `UPDATE account_challenges SET consumed_at=clock_timestamp() WHERE kind=$1 AND subject_digest=$2 AND consumed_at IS NULL`, kind, h.cfg.Vault.mac("challenge:"+kind, subject)); err != nil {
		return challengeReply{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO account_challenges(id,kind,subject_digest,binding_digest,code_digest,payload_cipher,person_id,expires_at) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,clock_timestamp()+interval '300 seconds')`, id, kind, h.cfg.Vault.mac("challenge:"+kind, subject), digest(secret), codeHash, payload, person)
	if err != nil {
		return challengeReply{}, err
	}
	if send {
		mailID, e := newID()
		if e != nil {
			return challengeReply{}, e
		}
		payload, e := h.cfg.Vault.seal("mail:"+mailID, mailPayload{value.Email, code, kind})
		if e != nil {
			return challengeReply{}, e
		}
		_, err = tx.Exec(ctx, `INSERT INTO account_mail_outbox(id,challenge_id,payload_cipher,expires_at) SELECT $1,id,$2,expires_at FROM account_challenges WHERE id=$3`, mailID, payload, id)
		if err != nil {
			return challengeReply{}, err
		}
	}
	return challengeReply{id, secret, 300, 60}, nil
}
func (h *Handler) consume(ctx context.Context, tx pgx.Tx, kind string, in proof) (pending, error) {
	var payload, binding, code []byte
	var consumed *time.Time
	var alive bool
	var attempts int
	// UUID parsing is performed by PostgreSQL; malformed values have the same public refusal.
	if !validID(in.ID) || len(in.Secret) != 43 {
		return pending{}, problem(401, "challenge_invalid")
	}
	err := tx.QueryRow(ctx, `SELECT payload_cipher,binding_digest,code_digest,consumed_at,expires_at>clock_timestamp(),attempts FROM account_challenges WHERE id=$1 AND kind=$2 FOR UPDATE`, in.ID, kind).Scan(&payload, &binding, &code, &consumed, &alive, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return pending{}, problem(401, "challenge_invalid")
	}
	if err != nil {
		return pending{}, err
	}
	if consumed != nil || !alive || attempts >= 5 || subtle.ConstantTimeCompare(binding, digest(in.Secret)) != 1 {
		return pending{}, problem(401, "challenge_invalid")
	}
	if code != nil && (len(in.Code) != 6 || subtle.ConstantTimeCompare(code, h.cfg.Vault.mac("otp:"+in.ID, in.Code)) != 1) {
		if _, err = tx.Exec(ctx, `UPDATE account_challenges SET attempts=attempts+1,consumed_at=CASE WHEN attempts=4 THEN clock_timestamp() ELSE NULL END WHERE id=$1`, in.ID); err != nil {
			return pending{}, err
		}
		return pending{}, problem(401, "code_invalid")
	}
	var value pending
	if err = h.cfg.Vault.open("challenge:"+in.ID, payload, &value); err != nil {
		return pending{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE account_challenges SET consumed_at=clock_timestamp() WHERE id=$1`, in.ID); err != nil {
		return pending{}, err
	}
	return value, nil
}
func validID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
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
	email, err := Email(in.Email)
	if err != nil {
		refuse(w, err)
		return
	}
	password, err := Password(in.Password)
	if err != nil {
		refuse(w, err)
		return
	}
	if err = h.limit(r.Context(), "mail", email, 5, 15*time.Minute); err != nil {
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
	var exists bool
	// Usernames are public. Email ownership is disclosed only after its OTP.
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM managed_accounts WHERE username=$1)`, username).Scan(&exists)
	if err != nil {
		refuse(w, err)
		return
	}
	if exists {
		refuse(w, problem(409, "account_already_exists"))
		return
	}
	out, err := h.newChallenge(r.Context(), tx, "register", email, "", pending{Username: username, Email: email, PasswordHash: hashed}, true)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 202, out)
}
func (h *Handler) verifyRegister(w http.ResponseWriter, r *http.Request) {
	var in proof
	if err := decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	p, err := h.consume(r.Context(), tx, "register", in)
	if err != nil {
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	person, err := h.createAccount(r.Context(), tx, p)
	if err != nil {
		refuse(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 201, map[string]any{"person_id": person, "username": p.Username, "verified": true})
}
func (h *Handler) requestReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := decode(w, r, &in); err != nil {
		refuse(w, err)
		return
	}
	email, err := Email(in.Email)
	if err != nil {
		refuse(w, err)
		return
	}
	if err = h.limit(r.Context(), "mail", email, 5, 15*time.Minute); err != nil {
		refuse(w, err)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		refuse(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var person string
	err = tx.QueryRow(r.Context(), `SELECT m.person_id::text FROM managed_accounts m JOIN people p ON p.id=m.person_id WHERE email_digest=$1 AND password_hash IS NOT NULL AND p.deleted_at IS NULL`, h.cfg.Vault.mac("email", email)).Scan(&person)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		refuse(w, err)
		return
	}
	// Always issue an indistinguishable challenge; unknown email never queues mail.
	known := person != ""
	out, err := h.newChallenge(r.Context(), tx, "reset", email, person, pending{Email: email, Person: person}, known)
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 202, out)
}
func (h *Handler) confirmReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		proof
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
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
	// Lock the account before its proofs, matching email rotation and erasure.
	// This read is only a lock-order hint; consume still checks the full proof.
	if !validID(in.ID) {
		refuse(w, problem(401, "challenge_invalid"))
		return
	}
	var candidate *string
	err = tx.QueryRow(r.Context(), `SELECT person_id::text FROM account_challenges WHERE id=$1 AND kind='reset'`, in.ID).Scan(&candidate)
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, problem(401, "challenge_invalid"))
		return
	}
	if err != nil {
		refuse(w, err)
		return
	}
	var current []byte
	if candidate != nil {
		if err = lockPerson(r.Context(), tx, *candidate); err != nil {
			refuse(w, err)
			return
		}
		err = tx.QueryRow(r.Context(), `SELECT email_digest FROM managed_accounts WHERE person_id=$1 FOR UPDATE`, *candidate).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			err = problem(401, "challenge_invalid")
		}
		if err != nil {
			refuse(w, err)
			return
		}
	}
	p, err := h.consume(r.Context(), tx, "reset", in.proof)
	if err != nil {
		refuse(w, commit(r.Context(), tx, err))
		return
	}
	if p.Person == "" || candidate == nil || p.Person != *candidate {
		refuse(w, commit(r.Context(), tx, problem(401, "challenge_invalid")))
		return
	}
	if subtle.ConstantTimeCompare(current, h.cfg.Vault.mac("email", p.Email)) != 1 {
		refuse(w, commit(r.Context(), tx, problem(401, "challenge_invalid")))
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE managed_accounts SET password_hash=$2 WHERE person_id=$1`, p.Person, hashed); err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1 AND revoked_at IS NULL`, p.Person)
	}
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	respond(w, 200, map[string]bool{"reset": true})
}
