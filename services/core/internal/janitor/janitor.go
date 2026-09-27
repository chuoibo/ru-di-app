// Package janitor removes rows whose only remaining purpose was to expire.
//
// Nothing else ever deletes them: every login leaves an OTP challenge and a
// session, every keyed write leaves an idempotency row, and each table grows
// with the number of people for as long as the product exists. The retention
// windows below are generous multiples of what the code reads back:
//
//   - otp_challenges: the send limit looks back 15 minutes (otp.DefaultWindowSeconds)
//   - account_sessions: a session is useless once expired or revoked; kept 30
//     days after that so a support question about a recent sign-in can still
//     be answered
//   - idempotency_keys: clients retry within seconds to minutes; a completed
//     key is kept 7 days, an unfinished one (a crashed request's claim) 1 day
//
// audit_events is deliberately absent: it is the money audit trail.
package janitor

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Retention windows.
const (
	OTPRetention         = 7 * 24 * time.Hour
	SessionRetention     = 30 * 24 * time.Hour
	IdempotencyRetention = 7 * 24 * time.Hour
	StaleClaimRetention  = 24 * time.Hour
	batchRows            = 5000
)

// Report counts what one pass removed, per table.
type Report struct {
	OTPChallenges   int64
	Sessions        int64
	IdempotencyKeys int64
}

type rule struct {
	count *int64
	sql   string
	args  func(now time.Time) []any
}

// Purge removes expired rows in batches of 5,000, each its own statement, so
// no pass holds a long lock on a table that login and every keyed write use.
func Purge(ctx context.Context, pool *pgxpool.Pool, now time.Time) (Report, error) {
	var report Report
	rules := []rule{
		{&report.OTPChallenges, `
			DELETE FROM otp_challenges WHERE id IN (
			  SELECT id FROM otp_challenges WHERE created_at < $1 LIMIT $2)`,
			func(now time.Time) []any { return []any{now.Add(-OTPRetention)} }},
		{&report.Sessions, `
			DELETE FROM account_sessions WHERE id IN (
			  SELECT id FROM account_sessions
			   WHERE expires_at < $1 OR revoked_at < $1 LIMIT $2)`,
			func(now time.Time) []any { return []any{now.Add(-SessionRetention)} }},
		{&report.IdempotencyKeys, `
			DELETE FROM idempotency_keys WHERE id IN (
			  SELECT id FROM idempotency_keys
			   WHERE completed_at < $1 OR (completed_at IS NULL AND created_at < $3) LIMIT $2)`,
			func(now time.Time) []any {
				return []any{now.Add(-IdempotencyRetention), nil, now.Add(-StaleClaimRetention)}
			}},
	}
	for _, r := range rules {
		for {
			args := r.args(now)
			if len(args) == 1 {
				args = append(args, batchRows)
			} else {
				args[1] = batchRows
			}
			tag, err := pool.Exec(ctx, r.sql, args...)
			if err != nil {
				return report, err
			}
			*r.count += tag.RowsAffected()
			if tag.RowsAffected() < batchRows || ctx.Err() != nil {
				break
			}
		}
	}
	return report, ctx.Err()
}
