package chatv2

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Clock returns the event database's current time under the same live
// session, device and room permission locks as a history read. It never
// appends an event or changes a sequence, receipt, outbox or schema.
func (s *Store) Clock(ctx context.Context, actor string, digest []byte, device, conversation string) (time.Time, error) {
	if !ValidID(actor) || !ValidID(device) || !ValidID(conversation) || len(digest) != 32 {
		return time.Time{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = authorizeSessionWithLock(ctx, tx, actor, device, conversation, lockShare, digest); err != nil {
		return time.Time{}, err
	}
	// A permission lock may have waited past the session's expiry. Evaluate
	// expiry at the actual sample, not at transaction start or on the Go host.
	// Materializing the volatile clock expression gives both uses one instant.
	var now time.Time
	err = tx.QueryRow(ctx, `WITH sample AS MATERIALIZED (SELECT clock_timestamp() AS server_now)
SELECT sample.server_now FROM sample JOIN account_sessions s
 ON s.token_digest=$1 AND s.person_id=$2 AND s.revoked_at IS NULL AND s.expires_at>sample.server_now`, digest, actor).Scan(&now)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrForbidden
	}
	if err != nil {
		return time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}
	return now, nil
}
