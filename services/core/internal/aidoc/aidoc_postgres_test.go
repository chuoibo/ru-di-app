//go:build postgres

package aidoc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/testdb"
)

// kho is a private schema with copies of the tables the adapters read (no
// foreign keys: LIKE does not copy them) and a pool whose search_path
// starts there, dropped when the test ends.
func kho(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	var b [8]byte
	_, _ = rand.Read(b[:])
	schema := "aidoc_test_" + hex.EncodeToString(b[:])
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"destinations", "places", "place_photos", "outings", "memberships"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []struct {
		id, ten                       string
		lat, lng, nam, tay, bac, dong float64
	}{
		{"da-lat", "Đà Lạt", 11.94, 108.44, 11.8, 108.3, 12.1, 108.6},
		{"vung-tau", "Vũng Tàu", 10.35, 107.08, 10.3, 107.0, 10.5, 107.2},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO destinations(id,name,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8,0)`,
			d.id, d.ten, d.lat, d.lng, d.nam, d.tay, d.bac, d.dong); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []struct{ id, dd, ten, loai, moTa string }{
		{"q-chay", "da-lat", "Quán Chay An Nhiên", "quan_an", "Quán chay yên tĩnh"},
		{"q-kem", "da-lat", "Kem Bơ Đậu Phộng", "quan_an", "Kem bơ rắc đậu phộng rang"},
		{"q-hai-san", "vung-tau", "Hải Sản Vũng Tàu", "quan_an", "Hải sản tươi ở Vũng Tàu"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,traits,description,source,price_min_vnd)
			VALUES($1,$2,$3,$4,'[]'::jsonb,11.9,108.4,'[]'::jsonb,$5,'curated',50000)`, p.id, p.dd, p.ten, p.loai, p.moTa); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func ids(bs []truyhoi.BangChung) map[string]bool {
	out := map[string]bool{}
	for _, b := range bs {
		out[b.ID] = true
	}
	return out
}

// A read transaction refuses writes, and the semaphore bounds how many
// reads hold a connection.
func TestChiDocChiDocVaGioiHan(t *testing.T) {
	pool := kho(t)
	ctx := context.Background()
	c := Moi(pool, 1)
	err := c.Doc(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO destinations(id,name,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES('x','x',0,0,0,0,0,0,0)`)
		return err
	})
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "25006" {
		t.Fatalf("a write in a read transaction: %v", err)
	}
	giu := make(chan struct{})
	vao := make(chan struct{})
	// Release the held read before the pool closes, even when the test
	// fails: a held connection would block pool.Close forever.
	var mo sync.Once
	tha := func() { mo.Do(func() { close(giu) }) }
	t.Cleanup(tha)
	go func() {
		_ = c.Doc(ctx, func(pgx.Tx) error { close(vao); <-giu; return nil })
	}()
	<-vao
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := c.Doc(short, func(pgx.Tx) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second read past the semaphore: %v", err)
	}
	tha()
}

// The retriever filters by what the MODEL extracted, never by the words of
// the query: a destination named in the text does not filter, an allergen
// named in the text does not filter, and the extracted ones always do.
func TestLexicalKhongDocChu(t *testing.T) {
	pool := kho(t)
	ctx := context.Background()
	l := Lexical{C: Moi(pool, 2)}
	tim := func(cau string, c truyhoi.Cung) map[string]bool {
		t.Helper()
		kq, err := l.Tim(ctx, truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: cau, Cung: c, K: truyhoi.MaxK})
		if err != nil {
			t.Fatal(err)
		}
		if len(kq.Degraded) != 1 || kq.Degraded[0] != truyhoi.LexicalOnly {
			t.Fatalf("flags %v", kq.Degraded)
		}
		for _, b := range kq.BangChung {
			if b.Nguon != truyhoi.Places || b.Truong["ten"] == "" {
				t.Fatalf("evidence %+v", b)
			}
		}
		return ids(kq.BangChung)
	}
	if got := tim("quán chay ở Vũng Tàu", truyhoi.Cung{}); !got["q-hai-san"] || !got["q-chay"] {
		t.Fatalf("the words filtered the destination: %v", got)
	}
	if got := tim("quán chay ở Vũng Tàu", truyhoi.Cung{DiemDenID: "da-lat"}); got["q-hai-san"] || !got["q-chay"] {
		t.Fatalf("the extracted destination did not filter: %v", got)
	}
	if got := tim("mình dị ứng đậu phộng", truyhoi.Cung{DiemDenID: "da-lat"}); !got["q-kem"] {
		t.Fatalf("the words filtered an allergen: %v", got)
	}
	if got := tim("mình dị ứng đậu phộng, thích quán chay", truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"dau_phong"}}); got["q-kem"] || !got["q-chay"] {
		t.Fatalf("the extracted allergen did not filter: %v", got)
	}
	nho := int64(10000)
	if got := tim("chay hải sản kem", truyhoi.Cung{NganSachVND: &nho}); len(got) != 0 {
		t.Fatalf("the budget did not filter: %v", got)
	}
	du := int64(60000)
	if got := tim("chay hải sản kem", truyhoi.Cung{NganSachVND: &du}); len(got) != 3 {
		t.Fatalf("a budget above every price filtered: %v", got)
	}
	if _, err := l.Tim(ctx, truyhoi.YeuCau{Nguon: truyhoi.Manual, Cau: "x"}); !errors.Is(err, truyhoi.ErrYeuCau) {
		t.Fatalf("a manual request: %v", err)
	}
}

// The open window reaches the live filter as a window: a place open
// 19:00–23:00 is kept for «tối nay» (18:00–22:00) and dropped for «at
// 18:00» (no-heuristics review 3: Go narrowed the window to its start).
func TestKhungMoTrenDuLieuThat(t *testing.T) {
	pool := kho(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO places(id,destination_id,name,category,kinds,lat,lng,traits,description,source,price_min_vnd,open_hours)
		VALUES('q-toi','da-lat','Quán Mở Tối','quan_an','[]'::jsonb,11.9,108.4,'[]'::jsonb,'Quán chay mở buổi tối','curated',50000,'19:00 – 23:00')`); err != nil {
		t.Fatal(err)
	}
	l := Lexical{C: Moi(pool, 2)}
	gio := time.FixedZone("ICT", 7*3600)
	tu := time.Date(2026, 9, 25, 18, 0, 0, 0, gio)
	tim := func(c truyhoi.Cung) map[string]bool {
		t.Helper()
		kq, err := l.Tim(ctx, truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "quán chay", Cung: c, K: truyhoi.MaxK})
		if err != nil {
			t.Fatal(err)
		}
		return ids(kq.BangChung)
	}
	if got := tim(truyhoi.Cung{DiemDenID: "da-lat", MoTrong: &truyhoi.KhungMo{Tu: tu, Den: tu.Add(4 * time.Hour)}}); !got["q-toi"] {
		t.Fatalf("the window dropped a place open inside it: %v", got)
	}
	if got := tim(truyhoi.Cung{DiemDenID: "da-lat", MoLuc: &tu}); got["q-toi"] {
		t.Fatalf("an instant kept a place closed at it: %v", got)
	}
	sang := time.Date(2026, 9, 25, 7, 0, 0, 0, gio)
	if got := tim(truyhoi.Cung{DiemDenID: "da-lat", MoTrong: &truyhoi.KhungMo{Tu: sang, Den: sang.Add(3 * time.Hour)}}); got["q-toi"] {
		t.Fatalf("a morning window kept an evening place: %v", got)
	}
}

func TestDocChoNhomCaNhan(t *testing.T) {
	pool := kho(t)
	ctx := context.Background()
	d := Doc{C: Moi(pool, 2)}
	bs, err := d.Quan(ctx, []string{"q-hai-san", "khong-co", "q-chay"})
	if err != nil || len(bs) != 2 || bs[0].ID != "q-hai-san" || bs[1].Truong["gia_min_vnd"] != "50000" {
		t.Fatalf("places %+v %v", bs, err)
	}
	ds, err := d.DiemDen(ctx)
	if err != nil || len(ds) != 2 {
		t.Fatalf("destinations %+v %v", ds, err)
	}
	kv, err := d.KhuVuc(ctx, "da-lat")
	if err != nil || len(kv) != 1 || kv[0].ID != "da-lat" {
		t.Fatalf("areas %+v %v", kv, err)
	}

	// Hex-letter UUIDs, so no fixture reads as a long number.
	nhom, khac, nguoi := "cccccccc-cccc-4ccc-8ccc-cccccccccccc", "dddddddd-dddd-4ddd-8ddd-dddddddddddd", "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	for i, o := range []struct {
		ctx, tu, den string
	}{{nhom, "2026-09-20", "2026-09-21"}, {nhom, "2026-10-01", "2026-10-02"}, {khac, "2026-10-05", "2026-10-05"}} {
		id := fmt.Sprintf("aaaaaaaa-aaaa-4aaa-8aaa-%012d", i)
		if _, err := pool.Exec(ctx, `INSERT INTO outings(id,context_id,created_by_id,title,starts_on,ends_on,headcount,budget_per_person_vnd)
			VALUES($1,$2,$3,$4,$5,$6,4,0)`, id, o.ctx, nguoi, fmt.Sprintf("Chuyến %d", i), o.tu, o.den); err != nil {
			t.Fatal(err)
		}
	}
	for i, m := range []struct{ ctx, state string }{{nhom, "active"}, {khac, "invited"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin,created_at)
			VALUES($1,$2,$3,$4,'member','named',now())`, fmt.Sprintf("bbbbbbbb-bbbb-4bbb-8bbb-%012d", i), m.ctx, nguoi, m.state); err != nil {
			t.Fatal(err)
		}
	}
	ngay := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	sap, err := d.ChuyenDi(ctx, nhom, ngay, true, 5)
	if err != nil || len(sap) != 1 || sap[0].Truong["tieu_de"] != "Chuyến 1" || sap[0].Truong["tu_ngay"] != "2026-10-01" {
		t.Fatalf("upcoming %+v %v", sap, err)
	}
	qua, err := d.ChuyenDi(ctx, nhom, ngay, false, 5)
	if err != nil || len(qua) != 1 || qua[0].Truong["tieu_de"] != "Chuyến 0" {
		t.Fatalf("past %+v %v", qua, err)
	}
	if n, err := d.SoThanhVien(ctx, nhom); err != nil || n != 1 {
		t.Fatalf("members %d %v", n, err)
	}
	// Only groups the person is an active member of.
	cua, err := d.ChuyenDiSapToi(ctx, nguoi, ngay, 5)
	if err != nil || len(cua) != 1 || cua[0].Truong["tieu_de"] != "Chuyến 1" {
		t.Fatalf("own %+v %v", cua, err)
	}
	if _, err := d.ChuyenDiSapToi(ctx, "not-a-uuid", ngay, 5); !errors.Is(err, ErrID) {
		t.Fatalf("a non-uuid identity: %v", err)
	}
}
