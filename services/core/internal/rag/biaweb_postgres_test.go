//go:build postgres

package rag

import (
	"context"
	"testing"
	"time"

	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/testdb"
)

// TestDongBoBiaWeb: the web's «permanently closed» takes a fed place out of
// search while the fact is unexpired, and gives it back when a later check
// no longer says so. A person's tombstone is never replaced or lifted.
func TestDongBoBiaWeb(t *testing.T) {
	ctx := context.Background()
	if err := ingest.Migrate(ctx, testdb.Pool(t)); err != nil {
		t.Fatal(err)
	}
	pool := kho(t)
	if _, err := pool.Exec(ctx, `CREATE TABLE place_facts (LIKE public.place_facts INCLUDING ALL)`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 3, 0, 0, 0, time.UTC)
	later, earlier := now.Add(45*24*time.Hour), now.Add(-time.Hour)
	for _, id := range []string{"vnl-a", "vnl-b", "vnl-c", "vnl-d"} {
		if _, err := pool.Exec(ctx, `INSERT INTO places(id,destination_id,name,category,source,source_ref) VALUES($1,'d-tinh-79',$1,'cafe','vnlocal','plc_'||substr($1,5))`, id); err != nil {
			t.Fatal(err)
		}
	}
	fact := func(id, state string, until time.Time) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO place_facts(place_id,source_ref,schema_version,trang_thai,con_hoat_dong,gio_mo_cua,menu,hoat_dong,checked_at,het_han_at,synced_at)
			VALUES($1,'plc_'||substr($1,5),'web-facts@2','co_du_lieu',$2,'[]','[]','[]',$3,$4,$3)
			ON CONFLICT (place_id) DO UPDATE SET con_hoat_dong=EXCLUDED.con_hoat_dong, het_han_at=EXCLUDED.het_han_at`,
			id, state, now.Add(-24*time.Hour), until); err != nil {
			t.Fatal(err)
		}
	}
	fact("vnl-a", "dong_vinh_vien", later)   // closed, current
	fact("vnl-b", "dong_vinh_vien", earlier) // closed, expired: unknown
	fact("vnl-c", "tam_dong", later)         // temporarily closed: stays searchable
	fact("vnl-d", "dong_vinh_vien", later)   // a person already took it down
	fact("vnl-e", "dong_vinh_vien", later)   // no catalogue row
	if err := Tombstone(ctx, pool, "vnl-d", "takedown"); err != nil {
		t.Fatal(err)
	}
	reasons := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		rows, err := pool.Query(ctx, `SELECT doc_id, reason FROM rag_tombstones WHERE corpus='place'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, r string
			if err := rows.Scan(&id, &r); err != nil {
				t.Fatal(err)
			}
			out[id] = r
		}
		return out
	}

	b, err := DongBoBiaWeb(ctx, pool, now)
	if err != nil || b.BoQua || b.Them != 1 || b.Go != 0 {
		t.Fatalf("first pass: %+v, %v", b, err)
	}
	got := reasons()
	if got["vnl-a"] != "web_closed" || got["vnl-d"] != "takedown" || len(got) != 2 {
		t.Fatalf("tombstones after the first pass: %v", got)
	}
	// Search reads it: the lexical path's tombstone set holds it.
	if ids, err := (Kho{Q: pool}).tombstoned(ctx); err != nil || !ids["vnl-a"] {
		t.Fatalf("retrieval does not see the web closure: %v, %v", ids, err)
	}
	if b, err := DongBoBiaWeb(ctx, pool, now); err != nil || b.Them != 0 || b.Go != 0 {
		t.Fatalf("a quiet pass changed something: %+v, %v", b, err)
	}

	// A later check no longer confirms the closure: the place comes back.
	// The person's takedown on vnl-d stays although its fact is unchanged.
	fact("vnl-a", "khong_ro", later)
	b, err = DongBoBiaWeb(ctx, pool, now)
	if err != nil || b.Them != 0 || b.Go != 1 {
		t.Fatalf("after the recheck: %+v, %v", b, err)
	}
	if got := reasons(); got["vnl-a"] != "" || got["vnl-d"] != "takedown" {
		t.Fatalf("tombstones after the recheck: %v", got)
	}

	// Expiry moves with the clock: closed again, then past het_han_at.
	fact("vnl-a", "dong_vinh_vien", later)
	if _, err := DongBoBiaWeb(ctx, pool, now); err != nil {
		t.Fatal(err)
	}
	if b, err := DongBoBiaWeb(ctx, pool, later.Add(time.Minute)); err != nil || b.Go != 1 {
		t.Fatalf("an expired closure still hides the place: %+v, %v", b, err)
	}
	if got := reasons(); got["vnl-a"] != "" || got["vnl-d"] != "takedown" {
		t.Fatalf("tombstones after expiry: %v", got)
	}
}
