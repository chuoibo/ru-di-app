package rag

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/jobs"
)

// NhatKyRetention is how long a rag_query_log row lives (design 04 §4,
// ADR-0040 §3): thirty days, the same as the engine's metrics rows.
const NhatKyRetention = 30 * 24 * time.Hour

// XoaNhatKy deletes query log rows past NhatKyRetention.
func XoaNhatKy(ctx context.Context, q Querier) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM rag_query_log WHERE at < clock_timestamp() - make_interval(secs => $1)`, NhatKyRetention.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DinhKy is the thirty-day purge of the query log as a periodic task
// (jobs.DinhKy). A database without the retrieval schema (`core migrate-rag`
// never ran there) is a pass with nothing to purge, not a failure.
func DinhKy() jobs.DinhKy {
	return jobs.DinhKy{Ten: "rag.xoa_nhat_ky", Nhip: 10 * time.Minute, Chay: func(ctx context.Context, pool *pgxpool.Pool) error {
		ok, err := Installed(ctx, pool)
		if err != nil || !ok {
			return err
		}
		_, err = XoaNhatKy(ctx, pool)
		return err
	}}
}
