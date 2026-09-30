//go:build postgres

package nap_test

import (
	"context"
	"strings"
	"testing"

	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

// vnlocal's attributes and categories reach every reader of enrichment:
// a vnlocal row wins over place_enrichments, is current whatever RuDi's
// hash says, a row outside the closed lists is left out, and categories
// read back as DanhMuc ids.
func TestLamGiauVaDanhMucTuVnlocal(t *testing.T) {
	ctx := context.Background()
	if err := ingest.Migrate(ctx, testdb.Pool(t)); err != nil {
		t.Fatal(err)
	}
	pool := naptest.Pool(t)
	for _, table := range []string{"place_lam_giau", "place_danh_muc"} {
		if _, err := pool.Exec(ctx, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"vnl-a", "vnl-b", "vnl-c"} {
		naptest.Chen(t, pool, repo.Place{ID: id, DestinationID: "d-tinh-79", Name: id, Category: "cafe", Source: "seed"})
	}
	var stale [32]byte
	stale[0] = 1
	// vnl-a: RuDi's enrichment, now stale; vnlocal re-ran it.
	if err := nap.GhiLamGiau(ctx, pool, []nap.LamGiau{{PlaceID: "vnl-a", NguonHash: stale, Model: "rudi", PromptVersion: nap.PromptVersion(),
		KetQua: nap.KetQuaLamGiau{DiUng: []string{"sua"}, DiUngRo: true, MonChinh: []string{"Cũ"}, TinCay: nap.TinCayCao}, Review: nap.ReviewAuto}}); err != nil {
		t.Fatal(err)
	}
	ins := func(id string, diUng []string, mon []string, tinCay string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO place_lam_giau(place_id,source_ref,di_ung,an_kieng,khi_chat,mon_chinh,chen_lenh,tin_cay,schema_version,checked_at,synced_at)
			VALUES($1,'plc_'||substr($1,5),$2,'{}','{}',$3,false,$4,'lam-giau@1',now(),now())`, id, diUng, mon, tinCay); err != nil {
			t.Fatal(err)
		}
	}
	ins("vnl-a", []string{"tom"}, []string{"Lẩu tôm"}, "cao")
	// vnl-b: an allergen outside the list (the DB CHECK does not hold di_ung;
	// the reader's checks must).
	ins("vnl-b", []string{"gluten"}, []string{"Bánh"}, "cao")
	if _, err := pool.Exec(ctx, `INSERT INTO place_danh_muc(place_id,source_ref,danh_muc,schema_version,checked_at,synced_at)
		VALUES('vnl-a','plc_a','{cafe,luu_tru}','danh-muc@1',now(),now()),('vnl-c','plc_c','{khong_ro}','danh-muc@1',now(),now())`); err != nil {
		t.Fatal(err)
	}

	enr, err := nap.DocLamGiau(ctx, pool, []string{"vnl-a", "vnl-b", "vnl-c"})
	if err != nil {
		t.Fatal(err)
	}
	a := enr["vnl-a"]
	if a == nil || !a.Ngoai || strings.Join(a.KetQua.DiUng, ",") != "tom" || strings.Join(a.KetQua.MonChinh, ",") != "Lẩu tôm" {
		t.Fatalf("vnl-a: %+v", a)
	}
	var h [32]byte
	if tt := nap.ApDung(a, h); !tt.Co || tt.Cu || tt.DiUngRo {
		t.Fatalf("vnl-a applied: %+v", tt)
	}
	if enr["vnl-b"] != nil || enr["vnl-c"] != nil {
		t.Fatalf("a refused or missing row produced an enrichment: %+v %+v", enr["vnl-b"], enr["vnl-c"])
	}
	dm, err := nap.DocDanhMuc(ctx, pool, []string{"vnl-a", "vnl-b", "vnl-c"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(dm["vnl-a"], ",") != "cafe,luu_tru" || strings.Join(dm["vnl-c"], ",") != "khong_ro" || dm["vnl-b"] != nil {
		t.Fatalf("categories: %v", dm)
	}
	// The build reads them: vnl-a counts as enriched, its row carries the
	// categories and the vnlocal dishes.
	n, _ := naptest.Nap(t, nap.NewKhoNho())
	var rep nap.BaoCaoDung
	docs, _, err := n.ChuanBiQuan(ctx, pool, &rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.HoSo.ID == "vnl-b" || d.HoSo.ID == "vnl-c" {
			t.Fatalf("%s has no current enrichment and is in the version", d.HoSo.ID)
		}
		if d.HoSo.ID != "vnl-a" {
			continue
		}
		if len(d.Rows) != 1 || strings.Join(d.Rows[0].DanhMuc, ",") != "cafe,luu_tru" || !strings.Contains(d.Rows[0].Text, "Món chính: Lẩu tôm") {
			t.Fatalf("vnl-a row: %+v", d.Rows)
		}
	}
	if rep.ThieuLamGiau != 2 {
		t.Fatalf("places left out waiting for enrichment = %d, want 2 (vnl-b refused, vnl-c none)", rep.ThieuLamGiau)
	}
}
