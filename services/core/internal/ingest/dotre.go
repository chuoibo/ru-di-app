package ingest

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// nguonDoTre are the feed tables a round reads, by their cursor's source.
var nguonDoTre = []struct{ source, table string }{
	{FeedSource, "places"},
	{FactsSource, "place_web_facts"},
	{DanhMucSource, "place_danh_muc"},
	{LamGiauSource, "place_lam_giau"},
}

// GhiDoTre records, after a successful round ending at at, each source's
// cursor and the feed's newest synced_at (one indexed max per table), so
// the freshness SLO can tell a stalled ingest from a quiet feed. A feed
// table this database's role cannot read yet is skipped, not an error.
func GhiDoTre(ctx context.Context, pool *pgxpool.Pool, feed *pgxpool.Pool, at time.Time) error {
	for _, n := range nguonDoTre {
		var moi *time.Time
		if err := feed.QueryRow(ctx, `SELECT max(synced_at) FROM `+pgx.Identifier{n.table}.Sanitize()).Scan(&moi); err != nil {
			continue
		}
		var conTro *time.Time
		err := pool.QueryRow(ctx, `SELECT synced_at FROM ingest_cursor WHERE source = $1`, n.source).Scan(&conTro)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// How far the feed's newest row is past the cursor; a source never
		// pulled has no cursor and no lag to speak of.
		tre := 0.0
		if moi != nil && conTro != nil && moi.After(*conTro) {
			tre = moi.Sub(*conTro).Seconds()
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO ingest_do_tre (source, vong_at, con_tro, nguon_moi_nhat, tre_giay) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (source) DO UPDATE SET vong_at = EXCLUDED.vong_at, con_tro = EXCLUDED.con_tro,
			  nguon_moi_nhat = EXCLUDED.nguon_moi_nhat, tre_giay = EXCLUDED.tre_giay`,
			n.source, at, conTro, moi, max(tre, 0)); err != nil {
			return err
		}
	}
	return nil
}
