//go:build postgres

package thuoctinh_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/thuoctinh/thuoctinhtest"
	"mobile/services/core/internal/vectordb"
)

// The re-check reads the tables the ingest writes, with the ingest's rule:
// a reviewed enrichment's allergens and diets, the live price and hours;
// an unreviewed «no allergen» is unknown; a stale enrichment (the row
// changed since) loses its certainty; a tombstone removes the place; an id
// with no row is absent.
func TestDocDocBangNapGhi(t *testing.T) {
	ctx := context.Background()
	pool := thuoctinhtest.Kho(t)
	gio := "10:00 – 22:00"
	gia := int64(120000)
	thuoctinhtest.ThemNoi(t, pool, "p1", "d-da-lat", "Lẩu tôm", &gia, &gio)
	thuoctinhtest.ThemNoi(t, pool, "p2", "d-da-lat", "Phở", nil, nil)
	thuoctinhtest.ThemNoi(t, pool, "p3", "d-da-lat", "Bún chả", &gia, &gio)
	thuoctinhtest.ThemNoi(t, pool, "p4", "d-da-lat", "Cơm", &gia, &gio)
	thuoctinhtest.LamGiau(t, pool, "p1", []string{"tom"}, []string{"thuan_chay"}, true)
	thuoctinhtest.LamGiau(t, pool, "p2", []string{}, nil, false) // «no allergen», nobody reviewed
	thuoctinhtest.LamGiau(t, pool, "p3", []string{}, nil, true)
	thuoctinhtest.LamGiau(t, pool, "p4", []string{}, nil, true)
	if _, err := pool.Exec(ctx, `UPDATE places SET description='Cơm, nay thêm lẩu cua' WHERE id='p4'`); err != nil {
		t.Fatal(err)
	}
	if err := rag.Tombstone(ctx, pool, "p3", "takedown"); err != nil {
		t.Fatal(err)
	}
	rows, err := thuoctinh.Doc(ctx, pool, []string{"p1", "p2", "p3", "p4", "p-gone"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rows["p-gone"]; ok || len(rows) != 4 {
		t.Fatalf("closed world broken: %v", rows)
	}
	p1 := rows["p1"]
	if !slices.Equal(p1.ThuocTinh.DiUng, []string{"tom"}) || !slices.Equal(p1.ThuocTinh.AnKieng, []string{"chay", "thuan_chay"}) ||
		p1.ThuocTinh.GiaMinVND != 120000 || p1.ThuocTinh.DiemDen != "d-da-lat" || len(p1.ThuocTinh.OSlots) == 0 || p1.ThuocTinh.GoBo ||
		!p1.GioRo || !p1.GiaRo || p1.Gio != gio || p1.Truong()["chua_ro"] != "" {
		t.Fatalf("p1 read back as %+v", p1)
	}
	if want := vectordb.SlotTuan(time.Date(2026, 9, 26, 19, 0, 0, 0, vectordb.ViTri)); !slices.Contains(p1.ThuocTinh.OSlots, want) {
		t.Fatalf("p1 is not open at 19:00 by its live hours: %v", p1.ThuocTinh.OSlots)
	}
	p2 := rows["p2"]
	if !slices.Equal(p2.ThuocTinh.DiUng, []string{vectordb.KhongRo}) || p2.ThuocTinh.GiaMinVND != vectordb.GiaKhongRo ||
		len(p2.ThuocTinh.OSlots) != 0 || p2.Truong()["chua_ro"] != "gio_chua_ro,gia_chua_ro" {
		t.Fatalf("p2 (unreviewed «no allergen», no price, no hours) read back as %+v", p2)
	}
	if !rows["p3"].ThuocTinh.GoBo {
		t.Fatal("a tombstoned place reads as live")
	}
	if p4 := rows["p4"].ThuocTinh; !slices.Equal(p4.DiUng, []string{vectordb.KhongRo}) {
		t.Fatalf("a stale enrichment kept its certainty: %+v", p4)
	}
	tom := vectordb.LocCung{DiUng: []string{"hai_san", "tom"}}
	for id, want := range map[string]bool{"p1": false, "p2": false, "p3": false, "p4": false} {
		if ok, _ := tom.Dat(rows[id].ThuocTinh); ok != want {
			t.Errorf("%s under a shrimp allergy: %v", id, ok)
		}
	}
	sua := vectordb.LocCung{DiUng: []string{"sua"}}
	if ok, _ := sua.Dat(rows["p1"].ThuocTinh); !ok {
		t.Error("the reviewed place with shrimp only fails a milk allergy")
	}
}
