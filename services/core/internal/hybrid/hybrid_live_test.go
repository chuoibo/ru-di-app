//go:build milvus

package hybrid_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/hybrid"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/rerank"
	"mobile/services/core/internal/testmilvus"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/thuoctinh/thuoctinhtest"
	"mobile/services/core/internal/vectordb"
)

var (
	diemDen = []string{"d-da-lat", "d-tphcm"}
	amTiet  = []string{"quán", "lẩu", "nướng", "chay", "bún", "đồi", "thông", "sân", "vườn", "yên", "tĩnh", "hải", "sản", "trà", "sữa", "hồ"}
)

type noi struct {
	id, diemDen, text string
	gia               *int64
	gio               *string
	// diUng nil: the enrichment answered khong_ro (allergens unknown).
	diUng, anKieng []string
}

// sinh draws n invented places with their catalogue rows and enrichments.
func sinh(seed uint64, n int) []noi {
	r := rand.New(rand.NewPCG(seed, seed+1))
	out := make([]noi, n)
	for i := range out {
		var w []string
		for j := 0; j < 4+r.IntN(4); j++ {
			w = append(w, amTiet[r.IntN(len(amTiet))])
		}
		x := noi{id: fmt.Sprintf("h%03d", i), diemDen: diemDen[r.IntN(len(diemDen))], text: strings.Join(w, " ")}
		switch r.IntN(4) {
		case 0:
		case 1:
			x.diUng = []string{}
		default:
			ids := tuvung.DiUng.IDs()
			x.diUng = []string{ids[r.IntN(len(ids))]}
		}
		if r.IntN(3) == 0 {
			x.anKieng = []string{"chay"}
		}
		if r.IntN(6) != 0 {
			g := fmt.Sprintf("%02d:00 – %02d:00", 6+r.IntN(5), 18+r.IntN(5))
			x.gio = &g
		}
		if r.IntN(5) != 0 {
			g := int64(20+r.IntN(30)) * 5000
			x.gia = &g
		}
		out[i] = x
	}
	return out
}

// dung writes the catalogue and the reviewed enrichments to PostgreSQL
// through the ingest's writer, indexes each place with the attributes the
// re-check reads for it, and returns the adapter over both.
func dung(t *testing.T, ns []noi) (*hybrid.Kho, *vectordb.Milvus, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool := thuoctinhtest.Kho(t)
	m := testmilvus.Ket(t)
	name, err := m.TaoPhienBan(ctx, vectordb.KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(ns))
	for i, n := range ns {
		thuoctinhtest.ThemNoi(t, pool, n.id, n.diemDen, n.text, n.gia, n.gio)
		thuoctinhtest.LamGiau(t, pool, n.id, n.diUng, n.anKieng, true)
		ids[i] = n.id
	}
	live, err := thuoctinh.Doc(ctx, pool, ids)
	if err != nil {
		t.Fatal(err)
	}
	var rows []vectordb.HangDiaDiem
	for _, n := range ns {
		d, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: n.text}})
		rows = append(rows, vectordb.HangDiaDiem{ID: n.id + "#ho_so", DocID: n.id, Dense: d[0], Text: n.text,
			ThuocTinh: live[n.id].ThuocTinh, PhienBan: 1})
	}
	if err := m.GhiDiaDiem(ctx, name, rows); err != nil {
		t.Fatal(err)
	}
	// A fresh upsert is searchable ~200 ms later (vectordb choThay).
	last := rows[len(rows)-1]
	for i := 0; ; i++ {
		got, err := m.Tim(ctx, vectordb.YeuCauTim{Ten: name, Kho: vectordb.KhoDiaDiem, Dense: last.Dense, K: 5})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) > 0 && got[0].ID == last.ID {
			break
		}
		if i == 100 {
			t.Fatal("the index never became searchable")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := m.NangCap(ctx, vectordb.KhoDiaDiem, 1); err != nil {
		t.Fatal(err)
	}
	alias := m.Alias(vectordb.KhoDiaDiem)
	k := &hybrid.Kho{
		Nhung: nhung.Stub{}, Index: m, Thua: vectordb.BM25{}, DocSong: hybrid.DocSongHam(thuoctinh.DocTu(pool)), TenDiaDiem: alias,
		BiLoai: func(ctx context.Context, l vectordb.LocCung) (map[truyhoi.RangBuoc]int, error) {
			return m.DemBiLoai(ctx, alias, l)
		},
	}
	return k, m, pool
}

// End to end, through Milvus and PostgreSQL, with the index made stale on
// purpose (allergens added, places deleted, tombstoned, repriced after the
// index was built): no item the adapter returns breaks a hard constraint by
// its live rows, and the stale hits Milvus returned were dropped and counted.
func TestHybridDauCuoiKhongViPham(t *testing.T) {
	ns := sinh(5, 160)
	k, _, pool := dung(t, ns)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var gone []string
	for i, n := range ns[:40] {
		switch i % 4 {
		case 0: // the enrichment found shrimp since, and a person approved it
			thuoctinhtest.LamGiau(t, pool, n.id, []string{"tom"}, n.anKieng, true)
		case 1: // taken down
			if err := rag.Tombstone(ctx, pool, n.id, "takedown"); err != nil {
				t.Fatal(err)
			}
		case 2: // gone from the catalogue
			if _, err := pool.Exec(ctx, `DELETE FROM places WHERE id=$1`, n.id); err != nil {
				t.Fatal(err)
			}
			gone = append(gone, n.id)
		case 3: // repriced above every budget the test draws (the enrichment
			// goes stale with it: its allergens stop being certain)
			if _, err := pool.Exec(ctx, `UPDATE places SET price_min_vnd=900000 WHERE id=$1`, n.id); err != nil {
				t.Fatal(err)
			}
		}
	}
	r := rand.New(rand.NewPCG(9, 10))
	items, nonEmpty := 0, 0
	for i := 0; i < 60; i++ {
		var c truyhoi.Cung
		if r.IntN(2) == 0 {
			c.DiemDenID = diemDen[r.IntN(2)]
		}
		if r.IntN(2) == 0 {
			c.DiUng = []string{"tom"}
		}
		if r.IntN(4) == 0 {
			c.AnKieng = []string{"chay"}
		}
		if r.IntN(2) == 0 {
			at := time.Date(2026, 9, 21+r.IntN(7), 8+r.IntN(12), r.IntN(60), 0, 0, vectordb.ViTri)
			c.MoLuc = &at
		}
		if r.IntN(2) == 0 {
			ns := int64(20+r.IntN(30)) * 5000
			c.NganSachVND = &ns
		}
		q := amTiet[r.IntN(len(amTiet))] + " " + amTiet[r.IntN(len(amTiet))]
		k.Nhung = nhung.NewDem(nhung.Stub{}, llm.MaxEmbedCallsPerTurn)
		kq, err := k.Tim(ctx, truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: q, Cung: c, K: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(kq.BangChung) > 0 {
			nonEmpty++
		}
		l, _ := vectordb.TuCung(c)
		ids := make([]string, len(kq.BangChung))
		for j, b := range kq.BangChung {
			ids[j] = b.ID
		}
		live, err := thuoctinh.Doc(ctx, pool, ids)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			items++
			row, ok := live[id]
			if !ok || slices.Contains(gone, id) {
				t.Fatalf("set %d: %s is not in the catalogue and was returned", i, id)
			}
			if ok, rb := l.Dat(row.ThuocTinh); !ok {
				t.Fatalf("set %d: %s breaks %q by its live rows", i, id, rb)
			}
		}
		if len(kq.Degraded) != 1 || kq.Degraded[0] != truyhoi.NoRerank {
			t.Fatalf("set %d: degraded %v (only the absent reranker expected)", i, kq.Degraded)
		}
	}
	if items < 150 || nonEmpty < 30 || k.KiemLaiLoai.Load() == 0 {
		t.Fatalf("%d items over %d non-empty retrievals, %d re-check drops: too little was measured", items, nonEmpty, k.KiemLaiLoai.Load())
	}
	t.Logf("60 retrievals, %d items checked against live rows, 0 violations; the re-check dropped %d stale hits", items, k.KiemLaiLoai.Load())
}

// The full path through a served reranker (the loopback stand-in): one
// retrieval, reordered by the served scores, the result a subset of the
// candidates, no NoRerank.
func TestHybridQuaRerankThat(t *testing.T) {
	base, served := rerankGia(t)
	k, _, _ := dung(t, sinh(6, 40))
	q, err := rerank.Moi(base, "", 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	k.Rerank = rerank.NewDem(q, llm.MaxRerankCallsPerTurn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "quán lẩu yên tĩnh", K: 4}
	kq, err := k.Tim(ctx, y)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Degraded) != 0 || len(kq.BangChung) != 4 {
		t.Fatalf("degraded %v, %d items", kq.Degraded, len(kq.BangChung))
	}
	for i := 1; i < len(kq.BangChung); i++ {
		if kq.BangChung[i].DiemXepLai > kq.BangChung[i-1].DiemXepLai {
			t.Fatalf("not in the reranker's order: %+v", kq.BangChung)
		}
	}
	if st := q.ThongKe(); st.Goi != 1 || st.Loi != 0 || served.Load() != 1 {
		t.Fatalf("reranker counters %+v, served %d", st, served.Load())
	}
}
