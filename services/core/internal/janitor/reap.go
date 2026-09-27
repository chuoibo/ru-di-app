package janitor

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ObjectStore is the part of photo storage the reaper uses.
type ObjectStore interface {
	Delete(key string) (bool, error)
}

// ReapReport counts one pass over pending_object_deletes.
type ReapReport struct {
	Deleted  int // files unlinked
	Missing  int // queued, already gone: the row is cleared
	Kept     int // still referenced by a photo row: the row is cleared, the file kept
	Failed   int // unlink failed: attempts and last_error recorded, retried later
	Disabled bool
}

// reapBatch bounds one pass.
const reapBatch = 500

// ReapObjects unlinks files queued in pending_object_deletes.
//
// The queue exists so a transaction that deletes photo rows never unlinks a
// file itself: a rollback cannot bring a file back. Something still has to
// empty it, or every purge leaves its files on disk for good.
//
// A key still named by a place_photos or uploaded_images row is never
// unlinked -- a queue row written in error must not break a picture on
// somebody's screen. The table belongs to the ingest migration; before it has
// run there is nothing to reap.
func ReapObjects(ctx context.Context, pool *pgxpool.Pool, store ObjectStore) (ReapReport, error) {
	var report ReapReport
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('pending_object_deletes') IS NOT NULL`).Scan(&exists); err != nil {
		return report, err
	}
	if !exists {
		report.Disabled = true
		return report, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT d.storage_key,
		       EXISTS (SELECT 1 FROM place_photos p WHERE p.storage_key = d.storage_key)
		    OR EXISTS (SELECT 1 FROM uploaded_images u WHERE u.storage_key = d.storage_key)
		  FROM pending_object_deletes d
		 ORDER BY d.attempts, d.requested_at
		 LIMIT $1`, reapBatch)
	if err != nil {
		return report, err
	}
	type item struct {
		key        string
		referenced bool
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.key, &it.referenced); err != nil {
			rows.Close()
			return report, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return report, err
	}

	for _, it := range items {
		if ctx.Err() != nil {
			break
		}
		if it.referenced {
			report.Kept++
		} else {
			deleted, err := store.Delete(it.key)
			if err != nil {
				report.Failed++
				if _, err := pool.Exec(ctx, `
					UPDATE pending_object_deletes
					   SET attempts = attempts + 1, last_error = left($2, 500)
					 WHERE storage_key = $1`, it.key, err.Error()); err != nil {
					return report, err
				}
				continue
			}
			if deleted {
				report.Deleted++
			} else {
				report.Missing++
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM pending_object_deletes WHERE storage_key = $1`, it.key); err != nil {
			return report, err
		}
	}
	return report, ctx.Err()
}
