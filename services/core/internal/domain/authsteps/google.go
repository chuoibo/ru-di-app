package authsteps

import (
	"errors"
	"math/big"
	"time"
)

func bigInt(value int64) *big.Int { return big.NewInt(value) }

// LoginWithGoogle is login_with_google (POST /auth/google): a Google ID token
// exchanged for a session (ADR-0016).
//
// Order is the whole of the security argument. A host with no client ids
// refuses before it reads the token (503); a token the verifier does not vouch
// for is one 401 whatever the reason it gave; and the `sub` is looked up in
// `account_identities` and nowhere else -- a first `sub` is a NEW person even
// if a person with the same e-mail address exists, because the claims do not
// carry the address to make that choice with.
func LoginWithGoogle(
	s Store,
	mint Secrets,
	verifier GoogleVerifier,
	idToken any,
	now time.Time,
) (SessionView, error) {
	if verifier == nil {
		return SessionView{}, refusal(
			503, "google_not_configured", "Máy chủ chưa cấu hình đăng nhập Google.",
		)
	}
	typed, ok := idToken.(string)
	if !ok || strip(typed) == "" {
		return SessionView{}, refusal(422, "id_token_required", "Thiếu id_token.")
	}
	claims, err := verifier.Verify(strip(typed))
	if err != nil {
		var broken *GoogleTokenInvalid
		if !errors.As(err, &broken) {
			return SessionView{}, err
		}
		return SessionView{}, refusal(
			401, "google_token_invalid", "Google không xác nhận lượt đăng nhập này. Thử lại.",
		)
	}
	existing, err := s.GetAccountIdentity("google", claims.Subject)
	if err != nil {
		return SessionView{}, err
	}
	personID := ""
	isNew := false
	if existing == nil {
		if personID, err = mint.NewUUID(); err != nil {
			return SessionView{}, err
		}
		isNew = true
		displayName := NewPersonName
		if claims.DisplayName != nil && *claims.DisplayName != "" {
			displayName = *claims.DisplayName
		}
		if _, raced := s.CreatePersonWithIdentity(personID, displayName, "google", claims.Subject, now); raced != nil {
			if !isConflict(raced) {
				return SessionView{}, raced
			}
			// Two first logins raced on one `sub`. The repository rolled the
			// loser's person row back with the failed binding, so re-read the
			// winner and sign in as that person. No winner means the conflict
			// was about something else, and Python re-raises the original.
			won, err := s.GetAccountIdentity("google", claims.Subject)
			if err != nil {
				return SessionView{}, err
			}
			if won == nil {
				return SessionView{}, raced
			}
			personID = won.PersonID
			isNew = false
		}
	} else {
		personID = existing.PersonID
		isNew = false
		if _, err := s.UpsertAccountIdentity(personID, "google", claims.Subject, now); err != nil {
			return SessionView{}, err
		}
	}
	raw, err := mint.NewSessionToken()
	if err != nil {
		return SessionView{}, err
	}
	tokenDigest := mint.TokenDigest(raw)
	expiresAt, err := after(now, AccountSessionTTL)
	if err != nil {
		return SessionView{}, err
	}
	record, err := s.CreateAccountSession(personID, tokenDigest, nil, expiresAt, now, "google")
	if err != nil {
		return SessionView{}, err
	}
	return sessionResponse(s, raw, record, personID, nil, nil, nil, isNew)
}
