package hybrid

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/vectordb"
)

// daoNguoc reverses and records what it was given.
type daoNguoc struct {
	cau  []string
	vao  []int
	goi  int
	loi  error
	nhan func([]truyhoi.BangChung)
}

func (d *daoNguoc) XepLai(_ context.Context, cau string, bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error) {
	d.goi++
	d.cau = append(d.cau, cau)
	d.vao = append(d.vao, len(bc))
	if d.loi != nil {
		return nil, d.loi
	}
	out := append([]truyhoi.BangChung(nil), bc...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out[:min(n, len(out))], nil
}

// The shared Kho reranks with the reranker its turn carries (the engine's
// counted one), and says no_rerank when the turn carries none.
func TestTurnRerankerFromContext(t *testing.T) {
	_, k := moiThu(t)
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{DiemDenID: "da-lat"}}
	base, err := k.Tim(context.Background(), y)
	if err != nil || !has(base.Degraded, truyhoi.NoRerank) || len(base.BangChung) < 2 {
		t.Fatalf("no reranker: %v %v %v", idsOf(base.BangChung), base.Degraded, err)
	}
	d := &daoNguoc{}
	kq, err := k.Tim(truyhoi.VoiXepLai(context.Background(), d), y)
	if err != nil || has(kq.Degraded, truyhoi.NoRerank) || d.goi != 1 {
		t.Fatalf("turn reranker: %v %v %v, %d calls", idsOf(kq.BangChung), kq.Degraded, err, d.goi)
	}
	want := idsOf(base.BangChung)
	for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
		want[i], want[j] = want[j], want[i]
	}
	if !reflect.DeepEqual(idsOf(kq.BangChung), want) {
		t.Fatalf("not in the reranker's order: %v, want %v", idsOf(kq.BangChung), want)
	}
	// A failing turn reranker: the RRF order, flagged, the turn goes on.
	d.loi = errors.New("timeout")
	kq, err = k.Tim(truyhoi.VoiXepLai(context.Background(), d), y)
	if err != nil || !has(kq.Degraded, truyhoi.NoRerank) || !reflect.DeepEqual(idsOf(kq.BangChung), idsOf(base.BangChung)) {
		t.Fatalf("failed reranker: %v %v %v", idsOf(kq.BangChung), kq.Degraded, err)
	}
}

// A caller that reranks itself (the corrective loop) gets the retrieval
// order, no rerank call and no flag from the retriever.
func TestCallerRerankDefers(t *testing.T) {
	_, k := moiThu(t)
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{DiemDenID: "da-lat"}}
	base, _ := k.Tim(context.Background(), y)
	d := &daoNguoc{}
	for _, ctx := range []context.Context{
		truyhoi.HoanXepLai(truyhoi.VoiXepLai(context.Background(), d)),
		truyhoi.HoanXepLai(context.Background()),
	} {
		kq, err := k.Tim(ctx, y)
		if err != nil || d.goi != 0 || has(kq.Degraded, truyhoi.NoRerank) || !reflect.DeepEqual(idsOf(kq.BangChung), idsOf(base.BangChung)) {
			t.Fatalf("deferred: %v %v %v, %d calls", idsOf(kq.BangChung), kq.Degraded, err, d.goi)
		}
	}
}

// The reranker reads at most UngVienXepLai candidates in RRF order and
// scores against the diacritics-restored query.
func TestRerankPoolAndQuery(t *testing.T) {
	th, k := moiThu(t)
	all := []int16{}
	for s := int16(0); s < vectordb.SoSlotTuan; s++ {
		all = append(all, s)
	}
	for i := 0; i < truyhoi.UngVienXepLai+10; i++ {
		id := "x" + strconv.Itoa(i)
		text := "lẩu quán số " + strconv.Itoa(i)
		v, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: text}})
		tt := vectordb.ThuocTinh{DiemDen: "da-lat", OSlots: all, GiaMinVND: 1000}
		th.fake.Them(vectordb.HangDiaDiem{ID: id, Dense: v[0], Text: text, ThuocTinh: tt, PhienBan: 1})
		gia := int64(1000)
		th.song[id] = thuoctinh.Hang{ThuocTinh: tt, Ten: id, Loai: "an_uong", GiaMinVND: &gia, GiaRo: true, GioRo: true, Gio: "00:00 – 24:00"}
	}
	d := &daoNguoc{}
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lau", CauCoDau: "lẩu", K: 5}
	kq, err := k.Tim(truyhoi.VoiXepLai(context.Background(), d), y)
	if err != nil || len(kq.BangChung) != 5 {
		t.Fatalf("%v %v", idsOf(kq.BangChung), err)
	}
	if d.vao[0] != truyhoi.UngVienXepLai || d.cau[0] != "lẩu" {
		t.Fatalf("the reranker read %d candidates against %q", d.vao[0], d.cau[0])
	}
}

// The router's diacritics-restored form reaches both legs: the BM25 field
// folds marks (rd.v4), so it and the person's own unmarked spelling give the
// same terms, and the dense leg reads the restored words.
func TestRestoredQueryFormReachesBothLegs(t *testing.T) {
	th, k := moiThu(t)
	g := &ghiNhung{}
	k.Nhung = g
	if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "quan lau nam", CauCoDau: "quán lẩu nấm"}); err != nil {
		t.Fatal(err)
	}
	q := th.fake.Da[0].Thua
	if q == nil || q.Text != "quán lẩu nấm" || len(g.texts) != 1 || g.texts[0] != "quán lẩu nấm" {
		t.Fatalf("bm25 %+v, embedded %v", q, g.texts)
	}
	// One form only: both legs read it.
	if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "quán lẩu"}); err != nil {
		t.Fatal(err)
	}
	if q = th.fake.Da[1].Thua; q == nil || q.Text != "quán lẩu" || g.texts[1] != "quán lẩu" {
		t.Fatalf("single form: %+v, embedded %v", q, g.texts)
	}
}
