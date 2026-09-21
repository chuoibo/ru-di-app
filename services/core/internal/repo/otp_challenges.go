package repo

// The OTP challenge lifecycle of SqlAlchemyApiRepository (ADR-0016): issue one
// code for one telephone, read back what was issued recently, and spend it.
//
// Neither the number nor the code is in this file or in the table. What is
// stored is `phone_digest`, an HMAC of the canonical number under the server's
// identity key, and `code_digest`, an HMAC salted by the challenge's own id --
// both computed by internal/identity before they reach here. Every argument
// below is already a digest, so nothing a caller passes and nothing a test
// writes down is a telephone number.

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// OtpChallenge is OtpChallengeRecord.
type OtpChallenge struct {
	ID          string
	PhoneDigest []byte
	CodeDigest  []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Attempts    int64
	ConsumedAt  *time.Time
}

// otpChallengeColumns is `select(OtpChallenge)`: every mapped column in
// declaration order, unlabelled.
const otpChallengeColumns = `otp_challenges.id, otp_challenges.phone_digest, otp_challenges.code_digest,
	        otp_challenges.created_at, otp_challenges.expires_at, otp_challenges.attempts,
	        otp_challenges.consumed_at`

// otpChallengeColumnsByID is `session.get(OtpChallenge, id)`.
const otpChallengeColumnsByID = `otp_challenges.id AS otp_challenges_id,
	        otp_challenges.phone_digest AS otp_challenges_phone_digest,
	        otp_challenges.code_digest AS otp_challenges_code_digest,
	        otp_challenges.created_at AS otp_challenges_created_at,
	        otp_challenges.expires_at AS otp_challenges_expires_at,
	        otp_challenges.attempts AS otp_challenges_attempts,
	        otp_challenges.consumed_at AS otp_challenges_consumed_at`

var otpChallengeInsert = []insertColumn{{"id", "::UUID"}, {"phone_digest", ""}, {"code_digest", ""},
	{"created_at", "::TIMESTAMP WITH TIME ZONE"}, {"expires_at", "::TIMESTAMP WITH TIME ZONE"},
	{"attempts", "::INTEGER"}, {"consumed_at", "::TIMESTAMP WITH TIME ZONE"}}

func scanOtpChallenge(row scannable) (*OtpChallenge, error) {
	var c OtpChallenge
	err := row.Scan(&c.ID, &c.PhoneDigest, &c.CodeDigest, &c.CreatedAt, &c.ExpiresAt, &c.Attempts,
		&c.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt, c.ExpiresAt = c.CreatedAt.UTC(), c.ExpiresAt.UTC()
	c.ConsumedAt = utcOptional(c.ConsumedAt)
	return &c, nil
}

// OtpChallengeInput is create_otp_challenge's keyword arguments. The id is the
// caller's: the service mints it before the code, because the code's digest is
// salted with it.
type OtpChallengeInput struct {
	ChallengeID string
	PhoneDigest []byte
	CodeDigest  []byte
	ExpiresAt   time.Time
	Now         time.Time
}

// CreateOtpChallenge is create_otp_challenge: one INSERT of every mapped
// column. attempts is written 0 from the Python default rather than left to
// the server default, and consumed_at NULL.
func (r Repository) CreateOtpChallenge(ctx context.Context, in OtpChallengeInput) (OtpChallenge, error) {
	created, expires := pythonInstant(in.Now), pythonInstant(in.ExpiresAt)
	if _, err := r.Q.Exec(ctx, renderInsert("otp_challenges", otpChallengeInsert, 1),
		in.ChallengeID, in.PhoneDigest, in.CodeDigest, created, expires, 0, nil); err != nil {
		return OtpChallenge{}, err
	}
	return OtpChallenge{ID: in.ChallengeID, PhoneDigest: in.PhoneDigest, CodeDigest: in.CodeDigest,
		CreatedAt: created.UTC(), ExpiresAt: expires.UTC(), Attempts: 0}, nil
}

// RecentOtpChallenges is recent_otp_challenges: what was issued for one
// telephone since an instant, newest first.
//
// `created_at > since` is strict, so a challenge created at exactly the edge of
// the window is outside it. The service subtracts the window from its own clock
// to build `since`, so the boundary is the service's arithmetic and not the
// database's.
func (r Repository) RecentOtpChallenges(ctx context.Context, phoneDigest []byte, since time.Time) ([]OtpChallenge, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+otpChallengeColumns+`
		   FROM otp_challenges
		  WHERE otp_challenges.phone_digest = $1
		    AND otp_challenges.created_at > $2::TIMESTAMP WITH TIME ZONE
		  ORDER BY otp_challenges.created_at DESC`, phoneDigest, pythonInstant(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OtpChallenge{}
	for rows.Next() {
		challenge, err := scanOtpChallenge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *challenge)
	}
	return out, rows.Err()
}

// GetOtpChallenge is get_otp_challenge: the row by primary key, nil when there
// is none.
func (r Repository) GetOtpChallenge(ctx context.Context, challengeID string) (*OtpChallenge, error) {
	return scanOtpChallenge(r.Q.QueryRow(ctx,
		`SELECT `+otpChallengeColumnsByID+`
		   FROM otp_challenges
		  WHERE otp_challenges.id = $1::UUID`, challengeID))
}

// RecordOtpAttempt is record_otp_attempt: the row by primary key, then one
// UPDATE of the columns whose value changed, in table order.
//
// Two SQLAlchemy facts the statement log shows:
//   - writing the attempt count it already has is not a change, so a flush with
//     nothing else to do emits no UPDATE at all;
//   - `consumed_at` is only written when it was NULL, so a challenge spent
//     earlier keeps the instant it was spent at.
func (r Repository) RecordOtpAttempt(ctx context.Context, challengeID string, attempts int64, consumed bool,
	now time.Time) (*OtpChallenge, error) {
	challenge, err := r.GetOtpChallenge(ctx, challengeID)
	if err != nil || challenge == nil {
		return nil, err
	}
	var sets []string
	var args []any
	if challenge.Attempts != attempts {
		challenge.Attempts = attempts
		args = append(args, attempts)
		sets = append(sets, "attempts=$"+strconv.Itoa(len(args))+"::INTEGER")
	}
	if consumed && challenge.ConsumedAt == nil {
		at := pythonInstant(now)
		challenge.ConsumedAt = &at
		args = append(args, at)
		sets = append(sets, "consumed_at=$"+strconv.Itoa(len(args))+"::TIMESTAMP WITH TIME ZONE")
	}
	if len(sets) == 0 {
		return challenge, nil
	}
	args = append(args, challengeID)
	if err := r.execUpdate(ctx, `UPDATE otp_challenges SET `+joinComma(sets)+
		` WHERE otp_challenges.id = $`+strconv.Itoa(len(args))+`::UUID`, args...); err != nil {
		return nil, err
	}
	return challenge, nil
}
