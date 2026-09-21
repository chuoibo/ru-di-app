package authsteps

import (
	"errors"
	"math/big"
	"time"

	"mobile/services/core/internal/domain/otp"
)

// otpWindow and otpCodeTTL are OTP_LIMITS["window_seconds"] and
// OTP_LIMITS["code_ttl_seconds"], which is app.domain.otp.DEFAULT_LIMITS: the
// service reads the table rather than restating the numbers.
const (
	otpWindow  = otp.DefaultWindowSeconds * time.Second
	otpCodeTTL = otp.DefaultCodeTTLSeconds * time.Second
)

// otpPhone is _otp_phone: the canonical number and the signing key, or the
// identity route's own 422 and 503.
//
// `phone` is what the hand-parsed body carried, so it is any JSON value; only
// a str passes, which is `isinstance(phone, str)`. The order is the whole of
// what the caller can learn: the shape of the field, then the shape of the
// number, then whether this host can derive ids at all.
func otpPhone(id Identity, phone any) (string, []byte, error) {
	raw, ok := phone.(string)
	if !ok {
		return "", nil, refusal(422, "phone_required", "Thiếu trường phone, và phải là chuỗi.")
	}
	canonical, ok := id.CanonicalMobile(raw)
	if !ok {
		return "", nil, refusal(422, "phone_not_mobile", "Chưa đúng dạng số di động Việt Nam.")
	}
	key, err := id.ReadKey()
	if err != nil {
		var missing *KeyMissing
		if errors.As(err, &missing) {
			return "", nil, refusal(
				503,
				"identity_key_missing",
				"Máy chủ chưa cấu hình khoá danh tính nên chưa đăng nhập được.",
			)
		}
		return "", nil, err
	}
	return canonical, key, nil
}

// RequestOTP is request_otp (POST /auth/otp/request): issue one code for one
// telephone. The number is never stored and never logged -- what the challenge
// row holds is its digest.
//
// With debugCode set (which the process only allows beside the log sender) every
// challenge carries that code, so the comparison path on verify is one path and
// not two.
func RequestOTP(
	s Store,
	id Identity,
	mint Secrets,
	sender SMSSender,
	phone any,
	debugCode *string,
	now time.Time,
) (OtpRequestView, error) {
	canonical, key, err := otpPhone(id, phone)
	if err != nil {
		return OtpRequestView{}, err
	}
	digest, err := id.PhoneDigest(canonical, key)
	if err != nil {
		return OtpRequestView{}, err
	}
	since, err := before(now, otpWindow)
	if err != nil {
		return OtpRequestView{}, err
	}
	recent, err := s.RecentOtpChallenges(digest, since)
	if err != nil {
		return OtpRequestView{}, err
	}
	issued := make([]time.Time, len(recent))
	for i, row := range recent {
		issued[i] = row.CreatedAt
	}
	plan, err := otp.PlanRequest(issued, now, nil)
	if err != nil {
		return OtpRequestView{}, err
	}
	if !plan.Allowed {
		wait := plan.RetryAfterSeconds.String()
		if plan.Reason == "resend_too_soon" {
			return OtpRequestView{}, refusal(
				429, "otp_resend_too_soon", "Mã vừa được gửi. Gửi lại sau "+wait+" giây.",
			)
		}
		return OtpRequestView{}, refusal(
			429, "otp_too_many_requests", "Số này đã nhận quá nhiều mã. Thử lại sau "+wait+" giây.",
		)
	}
	challengeID, err := mint.NewUUID()
	if err != nil {
		return OtpRequestView{}, err
	}
	code := ""
	if debugCode != nil {
		code = *debugCode
	} else if code, err = otp.GenerateCode(drawFrom(mint)); err != nil {
		return OtpRequestView{}, err
	}
	codeDigest, err := id.CodeDigest(challengeID, code, key)
	if err != nil {
		return OtpRequestView{}, err
	}
	expiresAt, err := after(now, otpCodeTTL)
	if err != nil {
		return OtpRequestView{}, err
	}
	record, err := s.CreateOtpChallenge(challengeID, digest, codeDigest, expiresAt, now)
	if err != nil {
		return OtpRequestView{}, err
	}
	if err := sender.SendOTP(canonical, code, record.ID); err != nil {
		var undelivered *DeliveryError
		if !errors.As(err, &undelivered) {
			return OtpRequestView{}, err
		}
		// The challenge is spent rather than left live: a code nobody received
		// is a code only a guesser can use.
		if _, err := s.RecordOtpAttempt(record.ID, 0, true, now); err != nil {
			return OtpRequestView{}, err
		}
		return OtpRequestView{}, refusal(
			503, "sms_unavailable", "Chưa gửi được tin nhắn lúc này, thử lại sau.",
		)
	}
	return OtpRequestView{
		ChallengeID:        record.ID,
		ExpiresInSeconds:   otp.DefaultCodeTTLSeconds,
		ResendAfterSeconds: otp.DefaultResendCooldownSeconds,
	}, nil
}

// VerifyOTP is verify_otp (POST /auth/otp/verify): spend a code for a session.
//
// Every dead-challenge refusal is one 404, because telling a guesser that a
// code WAS once real is worth more to them than to the person who simply has
// to ask for a new one. A challenge issued to another number is, to this
// caller, no challenge: the phone digests are compared before anything else is
// read off the row.
//
// The attempt is recorded before the refusal is raised, so a wrong code burns
// a try whether the caller reads the answer or not.
func VerifyOTP(
	s Store,
	id Identity,
	mint Secrets,
	challengeID string,
	phone, code any,
	now time.Time,
) (SessionView, error) {
	canonical, key, err := otpPhone(id, phone)
	if err != nil {
		return SessionView{}, err
	}
	typed, ok := code.(string)
	if !ok || strip(typed) == "" {
		return SessionView{}, refusal(422, "code_required", "Thiếu mã xác minh.")
	}
	typed = strip(typed)
	digest, err := id.PhoneDigest(canonical, key)
	if err != nil {
		return SessionView{}, err
	}
	challenge, err := s.GetOtpChallenge(challengeID)
	if err != nil {
		return SessionView{}, err
	}
	if challenge != nil && !compareDigest(challenge.PhoneDigest, digest) {
		challenge = nil
	}
	matches := false
	if challenge != nil {
		// Salted by the id of the row that was found, not by the id the caller
		// sent: the same code on two challenges stores two unrelated digests.
		typedDigest, err := id.CodeDigest(challenge.ID, typed, key)
		if err != nil {
			return SessionView{}, err
		}
		matches = compareDigest(challenge.CodeDigest, typedDigest)
	}
	var state *otp.Challenge
	if challenge != nil {
		state = &otp.Challenge{
			ExpiresAt:  challenge.ExpiresAt,
			Attempts:   bigInt(challenge.Attempts),
			ConsumedAt: challenge.ConsumedAt,
		}
	}
	plan := otp.PlanVerify(state, now, matches, nil)
	switch plan.Outcome {
	case "not_found", "consumed", "expired", "burned_already":
		return SessionView{}, refusal(
			404, "otp_challenge_not_found", "Mã không còn hiệu lực. Hãy yêu cầu mã mới.",
		)
	}
	if challenge == nil {
		return SessionView{}, &Invariant{Reason: "a live outcome with no challenge"}
	}
	spent := plan.Outcome == "ok" || plan.Outcome == "burned"
	// `attempts` is a stored integer column, and the only path that adds to it
	// is one the ceiling above has already bounded.
	if _, err := s.RecordOtpAttempt(challenge.ID, plan.Attempts.Int64(), spent, now); err != nil {
		return SessionView{}, err
	}
	if plan.Outcome == "burned" {
		return SessionView{}, refusal(
			429, "otp_too_many_attempts", "Sai mã quá nhiều lần. Hãy yêu cầu mã mới.",
		)
	}
	if plan.Outcome == "wrong_code" {
		return SessionView{}, refusal(
			422, "otp_code_invalid", "Mã chưa đúng. Còn "+plan.AttemptsLeft.String()+" lần thử.",
		)
	}

	// Who this number belongs to is `account_identities`, not the derived id
	// (ADR-0023 §2.2.2). The derivation only decides what id a NEW account
	// gets; a number whose previous account was deleted comes back as a new
	// person, because reviving the anonymised row would hand the next holder
	// of that number somebody else's groups and ledger.
	subject := hexOf(digest)
	bound, err := s.GetAccountIdentity("phone", subject)
	if err != nil {
		return SessionView{}, err
	}
	personID := ""
	isNew := false
	if bound != nil {
		personID = bound.PersonID
	} else {
		if personID, err = id.PersonID(canonical, key); err != nil {
			return SessionView{}, err
		}
		person, err := s.GetPerson(personID)
		if err != nil {
			return SessionView{}, err
		}
		if person != nil && person.DeletedAt != nil {
			if personID, err = mint.NewUUID(); err != nil {
				return SessionView{}, err
			}
			person = nil
		}
		isNew = person == nil
		if isNew {
			if _, err := s.CreatePerson(personID, NewPersonName); err != nil {
				// Two verifies raced on a brand-new number; the row exists.
				if !isConflict(err) {
					return SessionView{}, err
				}
				isNew = false
			}
		}
	}
	if _, err := s.UpsertAccountIdentity(personID, "phone", subject, now); err != nil {
		return SessionView{}, err
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
	record, err := s.CreateAccountSession(personID, tokenDigest, nil, expiresAt, now, "otp")
	if err != nil {
		return SessionView{}, err
	}
	return sessionResponse(s, raw, record, personID, nil, nil, nil, isNew)
}

// drawFrom is `secrets.randbelow` as generate_code wants it. The bound is
// always 10**CODE_LENGTH, so narrowing it for the seam loses nothing.
func drawFrom(mint Secrets) func(*big.Int) (*big.Int, error) {
	return func(bound *big.Int) (*big.Int, error) {
		drawn, err := mint.RandomBelow(bound.Int64())
		if err != nil {
			return nil, err
		}
		return big.NewInt(drawn), nil
	}
}
