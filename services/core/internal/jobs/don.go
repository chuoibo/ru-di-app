package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Execer is what Don needs: a pool, a connection or a transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Don deletes the outbox rows nobody will publish or read again: unpublished
// rows past their expiry (the job's sharing window closed, so it can no longer
// run) and published rows older than an hour. The relay runs it before every
// batch; the periodic task DinhKyDon runs it where no relay does, so a
// poll-only deployment does not grow the table without bound.
func Don(ctx context.Context, q Execer) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM job_outbox WHERE (published_at IS NULL AND expires_at IS NOT NULL AND expires_at<=clock_timestamp()) OR published_at<clock_timestamp()-interval '1 hour'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DinhKyDon is the outbox's own periodic task.
func DinhKyDon() DinhKy {
	return DinhKy{Ten: "jobs.don_outbox", Nhip: time.Minute, Chay: func(ctx context.Context, pool *pgxpool.Pool) error {
		_, err := Don(ctx, pool)
		return err
	}}
}
