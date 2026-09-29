//go:build postgres

package ingest

import (
	"context"
	"testing"
	"time"
)

// fakeFactFeed serves web facts in (synced_at, place_id) order.
type fakeFactFeed struct{ rows []FactRow }

func (f fakeFactFeed) Page(_ context.Context, syncedAt time.Time, placeID string, limit int) ([]FactRow, error) {
	var out []FactRow
	for _, row := range f.rows {
		after := row.SyncedAt.After(syncedAt) || (row.SyncedAt.Equal(syncedAt) && row.PlaceID > placeID)
		if after && len(out) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func factRow(id string, at, until time.Time, edit func(*FactRow)) FactRow {
	r := FactRow{PlaceID: id, SyncedAt: at, SchemaVersion: "web-facts@2", TrangThai: "co_du_lieu",
		ConHoatDong: str("con_hoat_dong"), GioMoCua: []byte(`[]`), Menu: []byte(`[]`), HoatDong: []byte(`[]`),
		CheckedAt: at, HetHanAt: until}
	if edit != nil {
		edit(&r)
	}
	return r
}

const monSat = `[{"thu":"mon","mo":"06:00","dong":"21:30","qua_dem":false},{"thu":"tue","mo":"06:00","dong":"21:30","qua_dem":false},` +
	`{"thu":"wed","mo":"06:00","dong":"21:30","qua_dem":false},{"thu":"thu","mo":"06:00","dong":"21:30","qua_dem":false},` +
	`{"thu":"fri","mo":"06:00","dong":"21:30","qua_dem":false},{"thu":"sat","mo":"06:00","dong":"21:30","qua_dem":false}]`

// TestWebFactsReachTheCatalogue: one sync round lands the places and their
// web facts and copies hours and the per-person band onto the places; an
// expired fact reads as unknown; a place that arrives after its facts gets
// them in the round it lands; an older version of a fact never overwrites a
// newer one; the feed's audit columns are nowhere.
func TestWebFactsReachTheCatalogue(t *testing.T) {
	pool, ctx := migratedPool(t)
	ids := []string{"vnl-wfa", "vnl-wfb", "vnl-wfc", "vnl-wfd"}
	clean := func() {
		for _, sql := range []string{
			`DELETE FROM place_facts WHERE place_id = ANY($1)`,
			`DELETE FROM place_source_post WHERE place_id = ANY($1)`,
			`DELETE FROM places WHERE id = ANY($1)`,
		} {
			if _, err := pool.Exec(context.Background(), sql, ids); err != nil {
				t.Fatal(err)
			}
		}
		for _, sql := range []string{
			`DELETE FROM ingest_cursor WHERE source IN ('vnlocal.places','vnlocal.place_web_facts')`,
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
	if _, err := SeedProvinces(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedProvinceDestinations(ctx, pool); err != nil {
		t.Fatal(err)
	}

	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	until := at.Add(45 * 24 * time.Hour)
	places := fakeFeed{rows: placeRows(t, at, "plc_wfa", "plc_wfb", "plc_wfc")}
	facts := fakeFactFeed{rows: []FactRow{
		factRow("plc_wfa", at, until, func(r *FactRow) {
			r.GioMoCua = []byte(monSat)
			r.GiaNguoiMin, r.GiaNguoiMax, r.GiaNguoiCoSo = i64(35000), i64(60000), str("uoc_tu_mon")
			r.Menu = []byte(`[{"mon":"Phở bò","gia_vnd":55000,"ghi_chu":null}]`)
		}),
		// Expired: its hours and price must not reach the place.
		factRow("plc_wfb", at, at.Add(-time.Minute), func(r *FactRow) {
			r.GioMoCua = []byte(monSat)
			r.GiaMin, r.GiaMax, r.GiaDonVi = i64(20000), i64(40000), str("moi_nguoi")
		}),
		// Per dish: not a per-person price.
		factRow("plc_wfc", at, until, func(r *FactRow) {
			r.GiaMin, r.GiaMax, r.GiaDonVi = i64(30000), i64(80000), str("moi_mon")
		}),
		// Facts for a place the catalogue does not hold yet.
		factRow("plc_wfd", at, until, func(r *FactRow) { r.GioMoCua = []byte(monSat) }),
		factRow("plc_wfz", at, until, func(r *FactRow) { r.ConHoatDong = str("chac_la_dong") }),
	}}
	opt := SyncOptions{Pull: PullOptions{PageSize: 2, MaxRows: 2}, Facts: facts}

	report, err := SyncOnce(ctx, pool, places, nil, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	if report.Facts != 4 || report.Rejected[RejectFactsConHoatDong] != 1 {
		t.Fatalf("facts landed %d, rejected %v; want 4 and one unknown state", report.Facts, report.Rejected)
	}
	type got struct {
		hours  *string
		lo, hi *int64
	}
	read := func(id string) got {
		t.Helper()
		var g got
		if err := pool.QueryRow(ctx, `SELECT open_hours, price_min_vnd, price_max_vnd FROM places WHERE id=$1`, id).
			Scan(&g.hours, &g.lo, &g.hi); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return g
	}
	a := read("vnl-wfa")
	if a.hours == nil || *a.hours != "Mo-Sa 06:00-21:30; Su off" || a.lo == nil || *a.lo != 35000 || a.hi == nil || *a.hi != 60000 {
		t.Fatalf("vnl-wfa = %+v", a)
	}
	if b := read("vnl-wfb"); b.hours != nil || b.lo != nil || b.hi != nil {
		t.Fatalf("an expired fact reached the place: %+v", b)
	}
	if c := read("vnl-wfc"); c.lo != nil || c.hi != nil {
		t.Fatalf("a per-dish price became a per-person one: %+v", c)
	}
	var uoc bool
	var menu string
	if err := pool.QueryRow(ctx, `SELECT gia_uoc, menu::text FROM place_facts WHERE place_id='vnl-wfa'`).Scan(&uoc, &menu); err != nil {
		t.Fatal(err)
	}
	if !uoc || menu == "[]" {
		t.Fatalf("estimate flag %v, menu %s", uoc, menu)
	}
	if report.FactsApplied.HetHan != 1 {
		t.Fatalf("expired rows = %d, want 1", report.FactsApplied.HetHan)
	}

	// A quiet round writes nothing.
	again, err := SyncOnce(ctx, pool, places, nil, nil, opt)
	if err != nil || again.Facts != 0 || again.FactsApplied.Updated != 0 {
		t.Fatalf("quiet round: %+v, %v", again, err)
	}

	// The place arrives after its facts: hours in the same round.
	later := fakeFeed{rows: placeRows(t, at.Add(time.Second), "plc_wfd")}
	if _, err := SyncOnce(ctx, pool, later, nil, nil, opt); err != nil {
		t.Fatal(err)
	}
	if d := read("vnl-wfd"); d.hours == nil {
		t.Fatal("a place landed after its facts has no hours")
	}

	// An older version replayed from a reset cursor does not win.
	if _, err := pool.Exec(ctx, `DELETE FROM ingest_cursor WHERE source = $1`, FactsSource); err != nil {
		t.Fatal(err)
	}
	old := fakeFactFeed{rows: []FactRow{factRow("plc_wfa", at.Add(-time.Hour), until, nil)}}
	if _, err := PullFacts(ctx, pool, old, PullOptions{}); err != nil {
		t.Fatal(err)
	}
	if applied, err := ApplyFacts(ctx, pool, time.Now()); err != nil || applied.Updated != 0 {
		t.Fatalf("an older fact moved the place: %+v, %v", applied, err)
	}

	// Past het_han_at the hours and price go: unknown, not the last value.
	if applied, err := ApplyFacts(ctx, pool, until.Add(time.Minute)); err != nil || applied.Updated < 2 {
		t.Fatalf("expiry: %+v, %v", applied, err)
	}
	if a := read("vnl-wfa"); a.hours != nil || a.lo != nil {
		t.Fatalf("an expired fact stayed on the place: %+v", a)
	}
}

// placeRows are place-feed rows sharing one synced_at.
func placeRows(t *testing.T, at time.Time, ids ...string) []FeedRow {
	var out []FeedRow
	for _, id := range ids {
		out = append(out, FeedRow{PlaceID: id, SyncedAt: at, Doc: feedDoc(t, id)})
	}
	return out
}
