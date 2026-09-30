package ingest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/domain/tuvung"
)

// Cursor sources of the two AI passes vnlocal runs for RuDi.
const (
	DanhMucSource = "vnlocal.place_danh_muc"
	LamGiauSource = "vnlocal.place_lam_giau"
)

// Reasons a row of either pass is refused. The cursor still moves past it:
// vnlocal re-runs a place whose input changes, and the next version lands
// then.
const (
	RejectAIPlaceID = "ai_place_id_la"
	RejectDanhMuc   = "danh_muc_ngoai_danh_sach"
	RejectLamGiau   = "lam_giau_ngoai_danh_sach"
)

// DanhMucRow is one row of the feed's place_danh_muc (danh-muc@1), without
// the model's prose (ly_do).
type DanhMucRow struct {
	PlaceID       string
	SyncedAt      time.Time
	DanhMuc       []string
	Model         *string
	PromptVersion *string
	SchemaVersion string
	CheckedAt     time.Time
}

// LamGiauRow is one row of the feed's place_lam_giau (lam-giau@1), without
// vnlocal's input hash.
type LamGiauRow struct {
	PlaceID       string
	SyncedAt      time.Time
	DiUng         []string
	AnKieng       []string
	KhiChat       []string
	MonChinh      []string
	ChenLenh      bool
	TinCay        string
	Model         *string
	PromptVersion *string
	SchemaVersion string
	CheckedAt     time.Time
}

// AIFeed pages through the two passes' tables in cursor order.
type AIFeed interface {
	DanhMuc(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]DanhMucRow, error)
	LamGiau(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]LamGiauRow, error)
}

// PGAIFeed reads them from the feed's Postgres (role rudi_app, read only).
type PGAIFeed struct{ Pool *pgxpool.Pool }

// DanhMuc is keyset pagination on (synced_at, place_id), as PGFeed.Page.
func (f PGAIFeed) DanhMuc(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]DanhMucRow, error) {
	rows, err := f.Pool.Query(ctx, `
		SELECT place_id, synced_at, danh_muc, model, prompt_version, schema_version, checked_at
		FROM place_danh_muc
		WHERE (synced_at, place_id) > ($1, $2)
		ORDER BY synced_at, place_id
		LIMIT $3`, syncedAt, placeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DanhMucRow
	for rows.Next() {
		var r DanhMucRow
		if err := rows.Scan(&r.PlaceID, &r.SyncedAt, &r.DanhMuc, &r.Model, &r.PromptVersion, &r.SchemaVersion, &r.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LamGiau is keyset pagination on (synced_at, place_id), as PGFeed.Page.
func (f PGAIFeed) LamGiau(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]LamGiauRow, error) {
	rows, err := f.Pool.Query(ctx, `
		SELECT place_id, synced_at, di_ung, an_kieng, khi_chat, mon_chinh, chen_lenh, tin_cay,
		       model, prompt_version, schema_version, checked_at
		FROM place_lam_giau
		WHERE (synced_at, place_id) > ($1, $2)
		ORDER BY synced_at, place_id
		LIMIT $3`, syncedAt, placeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LamGiauRow
	for rows.Next() {
		var r LamGiauRow
		if err := rows.Scan(&r.PlaceID, &r.SyncedAt, &r.DiUng, &r.AnKieng, &r.KhiChat, &r.MonChinh, &r.ChenLenh, &r.TinCay,
			&r.Model, &r.PromptVersion, &r.SchemaVersion, &r.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// maDong reports whether list is a set of ids of v (no repeats, at most
// max; max 0 = no bound), or exactly [khong_ro] when khongRo allows it.
func maDong(list []string, v *tuvung.TuVung, max int, khongRo bool) bool {
	if khongRo && len(list) == 1 && list[0] == tuvung.KhongRo {
		return true
	}
	if max > 0 && len(list) > max {
		return false
	}
	seen := map[string]bool{}
	for _, id := range list {
		if seen[id] || !slices.Contains(v.IDs(), id) {
			return false
		}
		seen[id] = true
	}
	return true
}

func feedID(id string) bool { return len(id) > len("plc_") && id[:len("plc_")] == "plc_" }

// danhMucReject says why a category row cannot be kept, or "": 1 to 10
// DanhMuc ids without repeats, or exactly [khong_ro].
func danhMucReject(r DanhMucRow) string {
	switch {
	case !feedID(r.PlaceID):
		return RejectAIPlaceID
	case len(r.DanhMuc) == 0 || !maDong(r.DanhMuc, tuvung.DanhMuc, 10, true):
		return RejectDanhMuc
	}
	return ""
}

// lamGiauReject says why an attribute row cannot be kept, or "": every list
// inside its closed vocabulary (khong_ro alone where the prompt allows it),
// at most 4 moods and 5 dishes, a known confidence. The dish strings' own
// safety is checked where they are read (rag/nap, as a model answer).
func lamGiauReject(r LamGiauRow) string {
	switch {
	case !feedID(r.PlaceID):
		return RejectAIPlaceID
	case !maDong(r.DiUng, tuvung.DiUng, 0, true),
		!maDong(r.AnKieng, tuvung.AnKieng, 0, true),
		!maDong(r.KhiChat, tuvung.KhiChat, 4, false),
		len(r.MonChinh) > 5,
		r.TinCay != "cao" && r.TinCay != "vua" && r.TinCay != "thap":
		return RejectLamGiau
	}
	return ""
}

// AIResult is what one pull of either pass did.
type AIResult struct {
	Landed   int
	Changed  []string // catalogue ids whose row was written
	Rejected map[string]int
	CaughtUp bool
}

// keoKeyset lands the next slice of one pass: reads pages after the stored
// cursor, upserts the rows it keeps, moves the cursor, all in one
// transaction under the source's own advisory lock.
func keoKeyset[T any](ctx context.Context, pool *pgxpool.Pool, source string, lock int64, opt PullOptions,
	page func(context.Context, time.Time, string, int) ([]T, error),
	key func(T) (time.Time, string),
	queue func(*pgx.Batch, T, *AIResult),
) (AIResult, error) {
	if opt.PageSize <= 0 {
		opt.PageSize = 500
	}
	if opt.MaxRows <= 0 {
		opt.MaxRows = 2000
	}
	result := AIResult{Rejected: map[string]int{}}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lock); err != nil {
		return result, err
	}
	syncedAt, placeID := cursorStart, ""
	err = tx.QueryRow(ctx, `SELECT synced_at, place_id FROM ingest_cursor WHERE source = $1`, source).Scan(&syncedAt, &placeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	var rows []T
	for len(rows) < opt.MaxRows {
		limit := min(opt.PageSize, opt.MaxRows-len(rows))
		got, err := page(ctx, syncedAt, placeID, limit)
		if err != nil {
			return result, fmt.Errorf("read %s after (%s, %q): %w", source, syncedAt.Format(time.RFC3339Nano), placeID, err)
		}
		rows = append(rows, got...)
		if len(got) > 0 {
			syncedAt, placeID = key(got[len(got)-1])
		}
		if len(got) < limit {
			result.CaughtUp = true
			break
		}
	}
	if len(rows) == 0 {
		return result, nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		queue(batch, r, &result)
	}
	if batch.Len() > 0 {
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return result, err
		}
	}
	// A changed row marks its place for the index (the rag_dirty trigger
	// on places): the index stores the categories and the attributes.
	if len(result.Changed) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE places SET updated_at = clock_timestamp()
			WHERE id = ANY($1) AND source = 'vnlocal'`, result.Changed); err != nil {
			return result, err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ingest_cursor (source, synced_at, place_id, batch_id)
		VALUES ($1,$2,$3,NULL)
		ON CONFLICT (source) DO UPDATE SET
		  synced_at = EXCLUDED.synced_at, place_id = EXCLUDED.place_id,
		  batch_id = NULL, moved_at = clock_timestamp()`,
		source, syncedAt, placeID); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

// PullDanhMuc lands the next slice of the categories.
func PullDanhMuc(ctx context.Context, pool *pgxpool.Pool, feed AIFeed, opt PullOptions) (AIResult, error) {
	return keoKeyset(ctx, pool, DanhMucSource, 815210, opt, feed.DanhMuc,
		func(r DanhMucRow) (time.Time, string) { return r.SyncedAt, r.PlaceID },
		func(b *pgx.Batch, r DanhMucRow, res *AIResult) {
			if reason := danhMucReject(r); reason != "" {
				res.Rejected[reason]++
				return
			}
			id := PlaceID(r.PlaceID)
			b.Queue(`
				INSERT INTO place_danh_muc (place_id, source_ref, danh_muc, model, prompt_version, schema_version, checked_at, synced_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT (place_id) DO UPDATE SET
				  danh_muc = EXCLUDED.danh_muc, model = EXCLUDED.model, prompt_version = EXCLUDED.prompt_version,
				  schema_version = EXCLUDED.schema_version, checked_at = EXCLUDED.checked_at,
				  synced_at = EXCLUDED.synced_at, landed_at = clock_timestamp()
				WHERE place_danh_muc.synced_at <= EXCLUDED.synced_at`,
				id, r.PlaceID, r.DanhMuc, r.Model, r.PromptVersion, r.SchemaVersion, r.CheckedAt, r.SyncedAt)
			res.Landed++
			res.Changed = append(res.Changed, id)
		})
}

// PullLamGiau lands the next slice of the search attributes.
func PullLamGiau(ctx context.Context, pool *pgxpool.Pool, feed AIFeed, opt PullOptions) (AIResult, error) {
	return keoKeyset(ctx, pool, LamGiauSource, 815211, opt, feed.LamGiau,
		func(r LamGiauRow) (time.Time, string) { return r.SyncedAt, r.PlaceID },
		func(b *pgx.Batch, r LamGiauRow, res *AIResult) {
			if reason := lamGiauReject(r); reason != "" {
				res.Rejected[reason]++
				return
			}
			id := PlaceID(r.PlaceID)
			b.Queue(`
				INSERT INTO place_lam_giau (place_id, source_ref, di_ung, an_kieng, khi_chat, mon_chinh, chen_lenh, tin_cay,
				  model, prompt_version, schema_version, checked_at, synced_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
				ON CONFLICT (place_id) DO UPDATE SET
				  di_ung = EXCLUDED.di_ung, an_kieng = EXCLUDED.an_kieng, khi_chat = EXCLUDED.khi_chat,
				  mon_chinh = EXCLUDED.mon_chinh, chen_lenh = EXCLUDED.chen_lenh, tin_cay = EXCLUDED.tin_cay,
				  model = EXCLUDED.model, prompt_version = EXCLUDED.prompt_version,
				  schema_version = EXCLUDED.schema_version, checked_at = EXCLUDED.checked_at,
				  synced_at = EXCLUDED.synced_at, landed_at = clock_timestamp()
				WHERE place_lam_giau.synced_at <= EXCLUDED.synced_at`,
				id, r.PlaceID, nonNilList(r.DiUng), nonNilList(r.AnKieng), nonNilList(r.KhiChat), nonNilList(r.MonChinh),
				r.ChenLenh, r.TinCay, r.Model, r.PromptVersion, r.SchemaVersion, r.CheckedAt, r.SyncedAt)
			res.Landed++
			res.Changed = append(res.Changed, id)
		})
}

func nonNilList(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
