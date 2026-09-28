//go:build milvus

package vectordb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/milvus-io/milvus/client/v3/column"
	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/milvusclient"
)

// TestMoRongCapNhatRieng: a field that arrives after the schema (the feed's
// menu with prices, owner 2026-09-28) is upserted into FMoRong alone, by
// partial update: the row keeps its facet, price and vector, and a row that
// was not touched still holds {}.
func TestMoRongCapNhatRieng(t *testing.T) {
	m := ketThu(t)
	ctx := ctxThu(t, 3*time.Minute)
	rows := fxDiaDiem(11, 2)
	rows[0].ID, rows[1].ID = "mr-a", "mr-b"
	rows[0].Facet, rows[0].ChunkSo = "mon_an", 1
	rows[0].ThuocTinh.GoBo, rows[1].ThuocTinh.GoBo = false, false
	name, err := m.TaoPhienBan(ctx, KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.GhiDiaDiem(ctx, name, rows); err != nil {
		t.Fatal(err)
	}
	choThay(t, m, name, rows)
	menu := []byte(`{"menu":[{"ten":"Cà phê muối","gia_vnd":35000}]}`)
	if n, err := m.CapNhatMoRong(ctx, name, []string{"mr-a"}, menu); err != nil || n != 1 {
		t.Fatalf("partial update: %d %v", n, err)
	}
	if _, err := m.CapNhatMoRong(ctx, name, []string{"mr-a"}, []byte(`[1]`)); err == nil {
		t.Fatal("a JSON array was accepted as the dict")
	}
	rs, err := m.cli.Query(ctx, milvusclient.NewQueryOption(name).WithIDs(column.NewColumnVarChar(FID, []string{"mr-a", "mr-b"})).
		WithOutputFields(FID, FMoRong, FFacet, FChunkSo, FPriceMin).WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]map[string]any{}
	for i := 0; i < rs.ResultCount; i++ {
		id, _ := rs.GetColumn(FID).GetAsString(i)
		raw, err := rs.GetColumn(FMoRong).Get(i)
		if err != nil {
			t.Fatal(err)
		}
		var d map[string]any
		if err := json.Unmarshal(raw.([]byte), &d); err != nil {
			t.Fatalf("%s: mo_rong %q", id, raw)
		}
		facet, _ := rs.GetColumn(FFacet).GetAsString(i)
		so, _ := rs.GetColumn(FChunkSo).GetAsInt64(i)
		gia, _ := rs.GetColumn(FPriceMin).GetAsInt64(i)
		d["_facet"], d["_so"], d["_gia"] = facet, so, gia
		got[id] = d
	}
	a, b := got["mr-a"], got["mr-b"]
	if a == nil || b == nil {
		t.Fatalf("rows back: %v", got)
	}
	if _, ok := a["menu"]; !ok || a["_facet"] != "mon_an" || a["_so"] != int64(1) || a["_gia"] != rows[0].ThuocTinh.GiaMinVND {
		t.Fatalf("updated row: %v", a)
	}
	if len(b) != 3 {
		t.Fatalf("untouched row's mo_rong is not {}: %v", b)
	}
}
