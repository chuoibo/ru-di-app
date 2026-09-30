//go:build postgres

package ingest

import (
	"context"
	"testing"
	"time"
)

type fakeAI struct {
	dm []DanhMucRow
	lg []LamGiauRow
}

func (f fakeAI) DanhMuc(_ context.Context, at time.Time, id string, limit int) ([]DanhMucRow, error) {
	var out []DanhMucRow
	for _, r := range f.dm {
		if (r.SyncedAt.After(at) || (r.SyncedAt.Equal(at) && r.PlaceID > id)) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f fakeAI) LamGiau(_ context.Context, at time.Time, id string, limit int) ([]LamGiauRow, error) {
	var out []LamGiauRow
	for _, r := range f.lg {
		if (r.SyncedAt.After(at) || (r.SyncedAt.Equal(at) && r.PlaceID > id)) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

// TestAIPassesLandAndMarkThePlace: categories and attributes land by keyset
// across pages, a refused row is counted and skipped, a landed row marks its
// place for the index (updated_at moves, which fires rag_dirty), and an
// older version replayed never overwrites a newer one.
func TestAIPassesLandAndMarkThePlace(t *testing.T) {
	pool, ctx := migratedPool(t)
	ids := []string{"vnl-aia", "vnl-aib"}
	clean := func() {
		for _, sql := range []string{`DELETE FROM place_danh_muc WHERE place_id = ANY($1)`, `DELETE FROM place_lam_giau WHERE place_id = ANY($1)`,
			`DELETE FROM place_source_post WHERE place_id = ANY($1)`, `DELETE FROM places WHERE id = ANY($1)`} {
			if _, err := pool.Exec(context.Background(), sql, ids); err != nil {
				t.Fatal(err)
			}
		}
		for _, sql := range []string{`DELETE FROM ingest_cursor WHERE source IN ('vnlocal.places','vnlocal.place_danh_muc','vnlocal.place_lam_giau')`,
			`DELETE FROM ingest_place_raw WHERE batch_id LIKE 'pg-%'`, `DELETE FROM ingest_reject WHERE batch_id LIKE 'pg-%'`,
			`DELETE FROM ingest_batch WHERE id LIKE 'pg-%'`} {
			if _, err := pool.Exec(context.Background(), sql); err != nil {
				t.Fatal(err)
			}
		}
	}
	clean()
	t.Cleanup(clean)
	if _, err := SeedProvinces(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedProvinceDestinations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	if _, err := SyncOnce(ctx, pool, fakeFeed{rows: placeRows(t, at, "plc_aia", "plc_aib")}, nil, nil, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	var before time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM places WHERE id='vnl-aia'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	feed := fakeAI{
		dm: []DanhMucRow{
			{PlaceID: "plc_aia", SyncedAt: at, DanhMuc: []string{"cafe", "luu_tru"}, SchemaVersion: "danh-muc@1", CheckedAt: at},
			{PlaceID: "plc_aib", SyncedAt: at, DanhMuc: []string{"quan_nuoc"}, SchemaVersion: "danh-muc@1", CheckedAt: at},
		},
		lg: []LamGiauRow{
			{PlaceID: "plc_aia", SyncedAt: at, DiUng: []string{"tom"}, AnKieng: []string{}, KhiChat: []string{"yen_tinh"},
				MonChinh: []string{"Lẩu tôm"}, TinCay: "cao", SchemaVersion: "lam-giau@1", CheckedAt: at},
		},
	}
	report, err := SyncOnce(ctx, pool, fakeFeed{}, nil, nil, SyncOptions{Pull: PullOptions{PageSize: 1, MaxRows: 1}, AI: feed})
	if err != nil {
		t.Fatal(err)
	}
	if report.DanhMuc != 1 || report.LamGiau != 1 || report.Rejected[RejectDanhMuc] != 1 {
		t.Fatalf("landed %d categories, %d attributes, rejected %v", report.DanhMuc, report.LamGiau, report.Rejected)
	}
	var dm []string
	var mon []string
	if err := pool.QueryRow(ctx, `SELECT d.danh_muc, l.mon_chinh FROM place_danh_muc d JOIN place_lam_giau l USING (place_id) WHERE place_id='vnl-aia'`).
		Scan(&dm, &mon); err != nil || len(dm) != 2 || len(mon) != 1 {
		t.Fatalf("rows: %v %v %v", dm, mon, err)
	}
	var after time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM places WHERE id='vnl-aia'`).Scan(&after); err != nil || !after.After(before) {
		t.Fatalf("the place was not marked for the index: %s -> %s %v", before, after, err)
	}
	// An older version, replayed from a reset cursor, does not win.
	if _, err := pool.Exec(ctx, `DELETE FROM ingest_cursor WHERE source = $1`, DanhMucSource); err != nil {
		t.Fatal(err)
	}
	old := fakeAI{dm: []DanhMucRow{{PlaceID: "plc_aia", SyncedAt: at.Add(-time.Hour), DanhMuc: []string{"quan_an"}, SchemaVersion: "danh-muc@1", CheckedAt: at}}}
	if _, err := PullDanhMuc(ctx, pool, old, PullOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT danh_muc FROM place_danh_muc WHERE place_id='vnl-aia'`).Scan(&dm); err != nil || len(dm) != 2 {
		t.Fatalf("an older category row overwrote a newer one: %v %v", dm, err)
	}
}
