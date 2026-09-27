package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FeedSource is the name the pull keeps its cursor under.
const FeedSource = "vnlocal.places"

// FeedRow is one row of the feed's own database: the place.v1 record and the
// pair the cursor is made of.
type FeedRow struct {
	PlaceID  string
	SyncedAt time.Time
	Doc      []byte
}

// Feed pages through the feed's database in cursor order.
type Feed interface {
	// Page returns up to limit rows strictly after (syncedAt, placeID), in
	// (synced_at, place_id) order.
	Page(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]FeedRow, error)
}

// PGFeed reads the feed's Postgres (read-only role, 30 s statement limit).
type PGFeed struct{ Pool *pgxpool.Pool }

// Page is keyset pagination on the pair, never on `synced_at` alone: the feed
// writes ~200 rows per transaction with one shared `synced_at`, and a
// `synced_at > $1` cursor drops the remainder of any batch a page cuts through.
func (f PGFeed) Page(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]FeedRow, error) {
	rows, err := f.Pool.Query(ctx, `
		SELECT place_id, synced_at, doc::text FROM places
		WHERE (synced_at, place_id) > ($1, $2)
		ORDER BY synced_at, place_id
		LIMIT $3`, syncedAt, placeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedRow
	for rows.Next() {
		var row FeedRow
		var doc string
		if err := rows.Scan(&row.PlaceID, &row.SyncedAt, &doc); err != nil {
			return nil, err
		}
		row.Doc = []byte(doc)
		out = append(out, row)
	}
	return out, rows.Err()
}

// cursorStart is before every row. Year 1 rather than -infinity so it
// round-trips through time.Time unchanged.
var cursorStart = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)

// PullOptions bounds one pull.
type PullOptions struct {
	PageSize int // rows per source query
	MaxRows  int // rows per landed batch
}

// Pull lands the next slice of the feed's database as one incremental batch.
//
// Rows, rejects and the moved cursor go in ONE transaction, so a crash leaves
// either all of it or none: the next pull resumes from the cursor that matches
// what is actually landed. An empty result (BatchID "") means caught up.
//
// Pulls are serialised by an advisory lock. Two pullers racing would each read
// the same cursor and land the same rows twice under different batch ids.
func Pull(ctx context.Context, pool *pgxpool.Pool, feed Feed, opt PullOptions) (LandResult, error) {
	if opt.PageSize <= 0 {
		opt.PageSize = 500
	}
	if opt.MaxRows <= 0 {
		opt.MaxRows = 2000
	}
	result := LandResult{
		Rejected:    map[string]int{},
		Drift:       map[string]int{},
		Overclaimed: map[string]int{},
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(815208)`); err != nil {
		return result, err
	}

	syncedAt, placeID := cursorStart, ""
	err = tx.QueryRow(ctx, `SELECT synced_at, place_id FROM ingest_cursor WHERE source = $1`,
		FeedSource).Scan(&syncedAt, &placeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}

	var rows []FeedRow
	for len(rows) < opt.MaxRows {
		limit := opt.PageSize
		if left := opt.MaxRows - len(rows); left < limit {
			limit = left
		}
		page, err := feed.Page(ctx, syncedAt, placeID, limit)
		if err != nil {
			return result, fmt.Errorf("read feed after (%s, %q): %w",
				syncedAt.Format(time.RFC3339Nano), placeID, err)
		}
		rows = append(rows, page...)
		if len(page) > 0 {
			last := page[len(page)-1]
			syncedAt, placeID = last.SyncedAt, last.PlaceID
		}
		if len(page) < limit {
			break
		}
	}
	if len(rows) == 0 {
		return result, nil
	}

	var seq int
	var previous *string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(max(dot_seq), 0) + 1,
		       (SELECT id FROM ingest_batch ORDER BY dot_seq DESC LIMIT 1)
		FROM ingest_batch`).Scan(&seq, &previous); err != nil {
		return result, err
	}
	batchID := fmt.Sprintf("pg-%06d-%s", seq, time.Now().UTC().Format("20060102T150405Z"))
	result.BatchID = batchID

	// The batch row describes what was pulled the way a file delivery
	// describes its file: a digest over the rows in order, and their size.
	digest := sha256.New()
	var bytes int64
	for _, row := range rows {
		digest.Write([]byte(LineDigest(row.Doc)))
		bytes += int64(len(row.Doc))
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ingest_batch (id, source, schema_version, dot_seq, dot_truoc,
		  kieu_dot, file_name, file_sha256, file_bytes, rows_declared,
		  updated_at_min, updated_at_max)
		VALUES ($1,'vnlocal','place.v1',$2,$3,'tang_dan',$4,$5,$6,$7,$8,$9)`,
		batchID, seq, previous, "pg:"+FeedSource,
		hex.EncodeToString(digest.Sum(nil)), bytes, len(rows),
		rows[0].SyncedAt, rows[len(rows)-1].SyncedAt); err != nil {
		return result, err
	}

	batch := &pgx.Batch{}
	for i, row := range rows {
		landLine(batch, &result, batchID, i+1, row.Doc)
	}
	if err := sendBatch(ctx, tx, batch); err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `UPDATE ingest_batch SET rows_landed=$2 WHERE id=$1`,
		batchID, result.Landed); err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ingest_cursor (source, synced_at, place_id, batch_id)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (source) DO UPDATE SET
		  synced_at = EXCLUDED.synced_at, place_id = EXCLUDED.place_id,
		  batch_id = EXCLUDED.batch_id, moved_at = clock_timestamp()`,
		FeedSource, syncedAt, placeID, batchID); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
