package ingest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/media/storage"
)

// SyncOptions configures one round.
type SyncOptions struct {
	Pull       PullOptions
	PerPlace   int // photographs kept per place
	MaxBatches int // pulls per round, so one round cannot run unbounded
	Photos     bool
}

// SyncReport is what one round did.
type SyncReport struct {
	Pulled    int // rows landed
	Batches   int
	Inserted  int
	Updated   int
	Stale     int
	Rejected  map[string]int
	Photos    int
	PhotoSkip map[string]int
	CaughtUp  bool
}

func (r SyncReport) String() string {
	return fmt.Sprintf("kéo %d dòng / %d đợt · thêm %d · cập nhật %d · cũ %d · từ chối %v · ảnh mới %d · bỏ ảnh %v · bắt kịp %v",
		r.Pulled, r.Batches, r.Inserted, r.Updated, r.Stale, r.Rejected, r.Photos, r.PhotoSkip, r.CaughtUp)
}

// SyncOnce brings the catalogue up to date with the feed: pull until caught up,
// apply every landed batch that has not been applied, then import photographs
// for every applied batch whose photo pass has not completed.
//
// Each step reads its work from the database rather than from the previous
// step's return value. A process killed between steps therefore loses nothing:
// the next round finds the landed-but-unapplied batch, or the applied batch
// without photos, and finishes it.
func SyncOnce(ctx context.Context, pool *pgxpool.Pool, feed Feed, frames FrameSource,
	store *storage.PhotoStorage, opt SyncOptions) (SyncReport, error) {
	report := SyncReport{Rejected: map[string]int{}, PhotoSkip: map[string]int{}}
	if opt.MaxBatches <= 0 {
		opt.MaxBatches = 50
	}

	for i := 0; i < opt.MaxBatches; i++ {
		landed, err := Pull(ctx, pool, feed, opt.Pull)
		if err != nil {
			return report, fmt.Errorf("pull: %w", err)
		}
		if landed.BatchID == "" {
			report.CaughtUp = true
			break
		}
		report.Batches++
		report.Pulled += landed.Landed
		for code, n := range landed.Rejected {
			report.Rejected[code] += n
		}
	}

	pending, err := batchIDs(ctx, pool, `
		SELECT id FROM ingest_batch WHERE applied_at IS NULL ORDER BY dot_seq`)
	if err != nil {
		return report, err
	}
	for _, id := range pending {
		applied, err := Apply(ctx, pool, id)
		if err != nil {
			return report, fmt.Errorf("apply %s: %w", id, err)
		}
		report.Inserted += applied.Inserted
		report.Updated += applied.Updated
		report.Stale += applied.Stale
		for code, n := range applied.Rejected {
			report.Rejected[code] += n
		}
	}

	if !opt.Photos {
		return report, nil
	}
	unphotographed, err := batchIDs(ctx, pool, `
		SELECT id FROM ingest_batch
		WHERE applied_at IS NOT NULL AND photos_at IS NULL ORDER BY dot_seq`)
	if err != nil {
		return report, err
	}
	for _, id := range unphotographed {
		result, err := ImportPhotos(ctx, pool, store, PhotoOptions{
			BatchID: id, Source: frames, PerPlace: opt.PerPlace})
		if err != nil {
			return report, fmt.Errorf("photos %s: %w", id, err)
		}
		report.Photos += result.Written
		for reason, n := range result.Skipped {
			report.PhotoSkip[reason] += n
		}
		if _, err := pool.Exec(ctx,
			`UPDATE ingest_batch SET photos_at = clock_timestamp() WHERE id = $1`, id); err != nil {
			return report, err
		}
	}
	return report, nil
}

func batchIDs(ctx context.Context, pool *pgxpool.Pool, sql string) ([]string, error) {
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
