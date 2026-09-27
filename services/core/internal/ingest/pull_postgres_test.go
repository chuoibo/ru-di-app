//go:build postgres

package ingest

import (
	"context"
	"testing"
	"time"
)

// fakeFeed serves rows in (synced_at, place_id) order, the way the source
// index does.
type fakeFeed struct{ rows []FeedRow }

func (f fakeFeed) Page(_ context.Context, syncedAt time.Time, placeID string, limit int) ([]FeedRow, error) {
	var out []FeedRow
	for _, row := range f.rows {
		after := row.SyncedAt.After(syncedAt) || (row.SyncedAt.Equal(syncedAt) && row.PlaceID > placeID)
		if after && len(out) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func feedDoc(t *testing.T, placeID string) []byte {
	t.Helper()
	line := validLine(t, func(r map[string]any) { r["place_id"] = placeID })
	return line
}

// TestPullLosesNoRowAtASharedSyncedAt. The feed writes ~200 rows per
// transaction with one `synced_at`; a page boundary inside such a group must
// not drop the rest of it. Five rows share one instant, pages of two.
func TestPullLosesNoRowAtASharedSyncedAt(t *testing.T) {
	pool, ctx := migratedPool(t)
	clean := func() {
		for _, sql := range []string{
			`DELETE FROM ingest_cursor WHERE source = 'vnlocal.places'`,
			`DELETE FROM ingest_place_raw WHERE batch_id LIKE 'pg-%'`,
			`DELETE FROM ingest_reject WHERE batch_id LIKE 'pg-%'`,
			`DELETE FROM ingest_batch WHERE id LIKE 'pg-%'`,
		} {
			if _, err := pool.Exec(context.Background(), sql); err != nil {
				t.Fatal(err)
			}
		}
	}
	clean()
	t.Cleanup(clean)
	at := time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC).Add(123456 * time.Microsecond)
	var rows []FeedRow
	for _, id := range []string{"plc_a", "plc_b", "plc_c", "plc_d", "plc_e"} {
		rows = append(rows, FeedRow{PlaceID: id, SyncedAt: at, Doc: feedDoc(t, id)})
	}
	rows = append(rows, FeedRow{PlaceID: "plc_f", SyncedAt: at.Add(time.Microsecond), Doc: feedDoc(t, "plc_f")})
	feed := fakeFeed{rows: rows}

	landed := 0
	for i := 0; i < 10; i++ {
		result, err := Pull(ctx, pool, feed, PullOptions{PageSize: 2, MaxRows: 2})
		if err != nil {
			t.Fatal(err)
		}
		if result.BatchID == "" {
			break
		}
		landed += result.Landed
	}
	if landed != len(rows) {
		t.Fatalf("landed %d of %d rows", landed, len(rows))
	}

	var keys int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT source_key) FROM ingest_place_raw
		WHERE batch_id LIKE 'pg-%'`).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	if keys != len(rows) {
		t.Errorf("distinct places landed = %d, want %d", keys, len(rows))
	}

	var cursorAt time.Time
	var cursorID string
	if err := pool.QueryRow(ctx, `SELECT synced_at, place_id FROM ingest_cursor WHERE source=$1`,
		FeedSource).Scan(&cursorAt, &cursorID); err != nil {
		t.Fatal(err)
	}
	if !cursorAt.Equal(at.Add(time.Microsecond)) || cursorID != "plc_f" {
		t.Errorf("cursor = (%s, %s), want the last row at microsecond precision", cursorAt, cursorID)
	}
}
