package hybrid

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/vectordb"
)

// A place indexed as several chunks comes back once, under its own id.
func TestChunksFoldToTheirPlace(t *testing.T) {
	th, k := moiThu(t)
	for _, c := range []struct{ id, text string }{{"lau1#ho_so", "lẩu nấm đồi thông"}, {"lau1#danh_gia", "lẩu nấm ngon"}} {
		v, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: c.text}})
		th.fake.Them(vectordb.HangDiaDiem{ID: c.id, DocID: "lau1", Dense: v[0], Text: c.text, ThuocTinh: th.fake.DiaDiem["lau1"].ThuocTinh, PhienBan: 3})
	}
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu nấm"})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range kq.BangChung {
		if strings.HasPrefix(b.ID, "lau1") {
			n++
			if b.ID != "lau1" {
				t.Fatalf("a chunk id reached the evidence: %s", b.ID)
			}
		}
	}
	if n != 1 {
		t.Fatalf("the place came back %d times: %v", n, idsOf(kq.BangChung))
	}
}

// ghiNhung records what it was asked to embed.
type ghiNhung struct {
	nhung.Stub
	texts []string
}

func (g *ghiNhung) Nhung(ctx context.Context, texts []string, tv nhung.TacVu) ([][]float32, error) {
	g.texts = append(g.texts, texts...)
	return g.Stub.Nhung(ctx, texts, tv)
}

// A query of any length is cut to MaxCauRune before it is embedded and
// before it reaches BM25.
func TestQueryIsCappedBeforeEmbedding(t *testing.T) {
	th, k := moiThu(t)
	g := &ghiNhung{}
	k.Nhung = g
	long := strings.Repeat("lẩu nấm ", 2000)
	if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: long}); err != nil {
		t.Fatal(err)
	}
	if len(g.texts) != 1 || utf8.RuneCountInString(g.texts[0]) > MaxCauRune {
		t.Fatalf("embedded %d runes", utf8.RuneCountInString(g.texts[0]))
	}
	if got := th.fake.Da[0].Thua.Text; utf8.RuneCountInString(got) > MaxCauRune {
		t.Fatalf("BM25 got %d runes", utf8.RuneCountInString(got))
	}
}

// Unknown hours or price: kept and flagged «chưa rõ» when no hard
// constraint asks about them; excluded when one does (docs/architecture/03
// §8.4).
func TestUnknownIsKeptFlaggedOrExcluded(t *testing.T) {
	th, k := moiThu(t)
	h := th.song["lau3"]
	h.GioRo, h.Gio = false, ""
	h.ThuocTinh.OSlots = nil
	th.song["lau3"] = h
	r := th.fake.DiaDiem["lau3"]
	r.ThuocTinh.OSlots = nil
	th.fake.Them(r)
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu gà", Cung: truyhoi.Cung{DiemDenID: "da-lat"}})
	if err != nil {
		t.Fatal(err)
	}
	flagged := false
	for _, b := range kq.BangChung {
		if b.ID == "lau3" {
			flagged = b.Truong["chua_ro"] == "gio_chua_ro"
		} else if b.Truong["chua_ro"] != "" {
			t.Fatalf("%s flagged unknown: %q", b.ID, b.Truong["chua_ro"])
		}
	}
	if !flagged {
		t.Fatalf("the place with unknown hours was not kept with its flag: %+v", kq.BangChung)
	}
	at := time.Date(2026, 9, 26, 19, 0, 0, 0, vectordb.ViTri)
	w := truyhoi.KhungMo{Tu: at, Den: at.Add(3 * time.Hour)}
	for _, c := range []truyhoi.Cung{{DiemDenID: "da-lat", MoLuc: &at}, {DiemDenID: "da-lat", MoTrong: &w}} {
		kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu gà", Cung: c})
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range kq.BangChung {
			if b.ID == "lau3" {
				t.Fatalf("unknown hours passed a hard time constraint %+v", c)
			}
		}
	}
}

// An open window reaches the index as the slots it touches.
func TestOpenWindowReachesTheIndex(t *testing.T) {
	th, k := moiThu(t)
	at := time.Date(2026, 9, 26, 18, 0, 0, 0, vectordb.ViTri) // a Saturday
	w := truyhoi.KhungMo{Tu: at, Den: at.Add(time.Hour)}
	if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{MoTrong: &w}}); err != nil {
		t.Fatal(err)
	}
	got := th.fake.Da[0].Loc.Slots
	s := vectordb.SlotTuan(at)
	if len(got) != 2 || got[0] != s || got[1] != s+1 {
		t.Fatalf("window slots %v, want [%d %d]", got, s, s+1)
	}
}

// The fusion weights the adapter was given reach the index.
func TestFusionWeightsReachTheIndex(t *testing.T) {
	th, k := moiThu(t)
	w := vectordb.TrongSo{Dense: 1, BM25: 0.1, BM25KhongDau: 1, MILCO: 1}
	k.TrongSo = &w
	if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu"}); err != nil {
		t.Fatal(err)
	}
	if got := th.fake.Da[0].TrongSo; got == nil || *got != w {
		t.Fatalf("weights %+v", got)
	}
}
