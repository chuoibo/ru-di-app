package accountauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
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
	codeBearing := send || kind == "reset"
	// The day-long refusal comes first: it is the one a waiting person
	// must hear, not «wait a minute» followed by it.
	if codeBearing {
		if err := h.guessesLeft(ctx, tx, kind, h.cfg.Vault.mac("challenge:"+kind, subject)); err != nil {
			return challengeReply{}, err
		}
	}
	if kind != "google" && kind != "google_register" {
		if recent > 0 {
			return challengeReply{}, problem(429, "challenge_resend_limited")
		}
		if count >= 5 {
			return challengeReply{}, problem(429, "challenge_quota_reached")
		}
	}
	if codeBearing {
		if err := h.mailOpen(ctx, kind); err != nil {
			return challengeReply{}, err
		}
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
	if codeBearing {
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
	var payload, binding, code, subject []byte
	var consumed *time.Time
	var alive bool
	var attempts int
	// UUID parsing is performed by PostgreSQL; malformed values have the same public refusal.
	if !validID(in.ID) || len(in.Secret) != 43 {
		return pending{}, problem(401, "challenge_invalid")
	}
	err := tx.QueryRow(ctx, `SELECT payload_cipher,binding_digest,code_digest,subject_digest,consumed_at,expires_at>clock_timestamp(),attempts FROM account_challenges WHERE id=$1 AND kind=$2 FOR UPDATE`, in.ID, kind).Scan(&payload, &binding, &code, &subject, &consumed, &alive, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return pending{}, problem(401, "challenge_invalid")
	}
	if err != nil {
		return pending{}, err
	}
	if consumed != nil || !alive || attempts >= 5 || subtle.ConstantTimeCompare(binding, digest(in.Secret)) != 1 {
		return pending{}, problem(401, "challenge_invalid")
	}
	if code != nil {
		if err = h.guessesLeft(ctx, tx, kind, subject); err != nil {
			return pending{}, err
		}
	}
	if code != nil && (len(in.Code) != 6 || subtle.ConstantTimeCompare(code, h.cfg.Vault.mac("otp:"+in.ID, in.Code)) != 1) {
		if _, err = tx.Exec(ctx, `UPDATE account_challenges SET attempts=attempts+1,consumed_at=CASE WHEN attempts=4 THEN clock_timestamp() ELSE NULL END WHERE id=$1`, in.ID); err != nil {
			return pending{}, err
		}
		if err = h.fail(ctx, codeBudget(kind, subject, requestIP(ctx))); err != nil {
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

// Five wrong codes end one challenge, and a resend starts a fresh one, so the
// subject (an email) carries caps across challenges, a day long: ten wrong
// codes from one address, thirty from everywhere. A stranger must spread over
// three addresses to keep an owner from a code for a day; a guesser gets
// thirty tries in a million. Challenges are kept a day past expiry.
const subjectFailureCap = 30
const subjectFailures = `SELECT coalesce(sum(attempts),0) FROM account_challenges WHERE kind=$1 AND subject_digest=$2 AND created_at>clock_timestamp()-interval '24 hours'`

func codeBudget(kind string, subject []byte, ip string) budget {
	return budget{"otp-fail", kind + ":" + hex.EncodeToString(subject) + ":" + ip, 10, 24 * time.Hour}
}

// guessesLeft refuses a code, or a new one, once the subject is spent from
// this address or from all of them; subject is the stored digest.
func (h *Handler) guessesLeft(ctx context.Context, tx pgx.Tx, kind string, subject []byte) error {
	var failures int
	if err := tx.QueryRow(ctx, subjectFailures, kind, subject).Scan(&failures); err != nil {
		return err
	}
	if failures >= subjectFailureCap {
		return problem(429, "challenge_attempts_exhausted")
	}
	if err := h.spent(ctx, codeBudget(kind, subject, requestIP(ctx))); err != nil {
		var e *Error
		if errors.As(err, &e) && e.Status == 429 {
			return problem(429, "challenge_attempts_exhausted")
		}
		return err
	}
	return nil
}

// issuing charges the budgets an OTP mail spends: the address's own and the
// asking client's, so one client cannot use up the day's mail for everyone.
func (h *Handler) issuing(r *http.Request, email string) error {
	ip := h.clientIP(r)
	if err := h.limit(r.Context(), "mail-ip", ip, 20, time.Hour); err != nil {
		return err
	}
	if err := h.limit(r.Context(), "mail-ip-day", ip, 50, 24*time.Hour); err != nil {
		return err
	}
	return h.limit(r.Context(), "mail", email, 5, 15*time.Minute)
}

// precheck refuses a proof that cannot match before any Argon2 work is spent
// on the password that came with it; consume still decides under lock.
func (h *Handler) precheck(ctx context.Context, kind string, in proof) error {
	if !validID(in.ID) || len(in.Secret) != 43 {
		return problem(401, "challenge_invalid")
	}
	var binding []byte
	var usable bool
	err := h.pool.QueryRow(ctx, `SELECT binding_digest,consumed_at IS NULL AND expires_at>clock_timestamp() AND attempts<5 FROM account_challenges WHERE id=$1 AND kind=$2`, in.ID, kind).Scan(&binding, &usable)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(401, "challenge_invalid")
	}
	if err != nil {
		return err
	}
	if !usable || subtle.ConstantTimeCompare(binding, digest(in.Secret)) != 1 {
		return problem(401, "challenge_invalid")
	}
	return nil
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
	var exists bool
	// Usernames are public (ADR-0055), so a taken one is said at once and
	// spends no mail budget. Email ownership is disclosed only after its OTP.
	err = h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM managed_accounts WHERE username=$1) OR EXISTS(SELECT 1 FROM retired_usernames WHERE digest=sha256(convert_to($1,'UTF8')))`, username).Scan(&exists)
	if err != nil {
		refuse(w, err)
		return
	}
	if exists {
		refuse(w, problem(409, "account_already_exists"))
		return
	}
	if err = h.issuing(r, email); err != nil {
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
	if err = h.precheck(r.Context(), "reset", in.proof); err != nil {
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
	var username string
	if err = tx.QueryRow(r.Context(), `UPDATE managed_accounts SET password_hash=$2 WHERE person_id=$1 RETURNING username`, p.Person, hashed).Scan(&username); err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1 AND revoked_at IS NULL`, p.Person)
	}
	if err = commit(r.Context(), tx, err); err != nil {
		refuse(w, err)
		return
	}
	// The owner proved the mailbox; strangers' wrong passwords stop pausing them.
	h.forgive(r.Context(), userLoginBudget(username))
	respond(w, 200, map[string]bool{"reset": true})
}
