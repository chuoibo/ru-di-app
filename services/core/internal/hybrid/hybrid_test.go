package hybrid

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/vectordb"
)

// A small catalogue in the fake index, with the live rows the re-check
// reads kept separately so a test can make them disagree (a stale index).
type thu struct {
	fake *vectordb.Fake
	song map[string]thuoctinh.Hang
	docs int
}

func moiThu(t *testing.T) (*thu, *Kho) {
	t.Helper()
	th := &thu{fake: vectordb.MoiFake(), song: map[string]thuoctinh.Hang{}}
	add := func(id, text string, tt vectordb.ThuocTinh) {
		v, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: text}})
		th.fake.Them(vectordb.HangDiaDiem{ID: id, Dense: v[0], Text: text, ThuocTinh: tt, PhienBan: 3})
		gia := tt.GiaMinVND
		th.song[id] = thuoctinh.Hang{ThuocTinh: tt, Ten: "Quán " + id, Loai: "an_uong", GiaMinVND: &gia, GiaRo: true, GioRo: true, Gio: "00:00 – 24:00"}
	}
	all := []int16{}
	for s := int16(0); s < vectordb.SoSlotTuan; s++ {
		all = append(all, s)
	}
	add("lau1", "quán lẩu nấm đồi thông", vectordb.ThuocTinh{DiemDen: "da-lat", OSlots: all, GiaMinVND: 100000})
	add("lau2", "lẩu hải sản ven hồ", vectordb.ThuocTinh{DiemDen: "da-lat", DiUng: []string{"tom"}, OSlots: all, GiaMinVND: 150000})
	add("lau3", "lẩu gà lá é", vectordb.ThuocTinh{DiemDen: "da-lat", OSlots: all, GiaMinVND: 90000})
	add("sg1", "lẩu thái quận ba", vectordb.ThuocTinh{DiemDen: "sai-gon", OSlots: all, GiaMinVND: 120000})
	k := &Kho{
		Nhung: nhung.Stub{}, Index: th.fake, Thua: vectordb.BM25{}, TenDiaDiem: "rd_places",
		DocSong: DocSongHam(func(_ context.Context, ids []string) (map[string]thuoctinh.Hang, error) {
			th.docs++
			out := map[string]thuoctinh.Hang{}
			for _, id := range ids {
				if h, ok := th.song[id]; ok {
					out[id] = h
				}
			}
			return out, nil
		}),
	}
	return th, k
}

func idsOf(bc []truyhoi.BangChung) []string {
	var s []string
	for _, b := range bc {
		s = append(s, b.ID)
	}
	return s
}

func TestHardConstraintsReachTheIndexUnchanged(t *testing.T) {
	th, k := moiThu(t)
	ns := int64(200000)
	at := time.Date(2026, 9, 26, 19, 0, 0, 0, vectordb.ViTri)
	c := truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"hai_san"}, MoLuc: &at, NganSachVND: &ns}
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: c})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := vectordb.TuCung(c)
	if !reflect.DeepEqual(th.fake.Da[0].Loc, want) {
		t.Fatalf("the index saw %+v, want %+v", th.fake.Da[0].Loc, want)
	}
	for _, id := range idsOf(kq.BangChung) {
		if id == "lau2" || id == "sg1" {
			t.Fatalf("%s breaks a hard constraint", id)
		}
	}
	// A shortfall returns fewer items, never a wider filter.
	if len(kq.BangChung) != 2 || len(th.fake.Da) != 1 {
		t.Fatalf("%v after %d searches", idsOf(kq.BangChung), len(th.fake.Da))
	}
	if kq.BangChung[0].PhienBanChiMuc != "v3" || kq.BangChung[0].Truong["ten"] == "" {
		t.Fatalf("evidence %+v", kq.BangChung[0])
	}
}

// The index is stale: it thinks lau1 has no allergen, the live row says it
// has shrimp now. The re-check drops it and says why.
func TestStaleIndexIsCaughtByTheRecheck(t *testing.T) {
	th, k := moiThu(t)
	h := th.song["lau1"]
	h.ThuocTinh.DiUng = []string{"tom"}
	th.song["lau1"] = h
	delete(th.song, "lau3") // gone from the catalogue since the index was built
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"tom"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.BangChung) != 0 {
		t.Fatalf("stale hits shown: %v", idsOf(kq.BangChung))
	}
	if kq.BiLoai[truyhoi.RBDiUng] != 1 || k.KiemLaiLoai.Load() != 2 {
		t.Fatalf("BiLoai %v, re-check drops %d", kq.BiLoai, k.KiemLaiLoai.Load())
	}
}

func TestRecheckErrorFailsClosed(t *testing.T) {
	_, k := moiThu(t)
	k.DocSong = DocSongHam(func(context.Context, []string) (map[string]thuoctinh.Hang, error) {
		return nil, errors.New("connection reset")
	})
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu"})
	if err == nil || len(kq.BangChung) != 0 {
		t.Fatal("hits shown without their re-check")
	}
}

type loiThua struct{}

func (loiThua) TruyVan(context.Context, string) (vectordb.ThuaTruyVan, error) {
	return vectordb.ThuaTruyVan{}, errors.New("sparse leg down")
}

func TestDegradedLegsAreReported(t *testing.T) {
	th, k := moiThu(t)
	// The turn's embedding budget is spent: sparse only, flagged.
	k.Nhung = nhung.NewDem(nhung.Stub{}, 0)
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu nấm"})
	if err != nil {
		t.Fatal(err)
	}
	if !has(kq.Degraded, truyhoi.NoVector) || !has(kq.Degraded, truyhoi.NoRerank) || th.fake.Da[0].Dense != nil || len(kq.BangChung) == 0 {
		t.Fatalf("degraded %v, dense sent %v, %d items", kq.Degraded, th.fake.Da[0].Dense != nil, len(kq.BangChung))
	}
	// The sparse encoder is down: dense only, flagged.
	_, k2 := moiThu(t)
	k2.Thua = loiThua{}
	kq, err = k2.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu nấm"})
	if err != nil || !has(kq.Degraded, truyhoi.NoSparse) || has(kq.Degraded, truyhoi.NoVector) {
		t.Fatalf("%v %v", kq.Degraded, err)
	}
	// Both down: an error the caller can fall back from.
	k2.Nhung = nhung.NewDem(nhung.Stub{}, 0)
	if _, err := k2.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu"}); !errors.Is(err, ErrKhongNhanh) {
		t.Fatal(err)
	}
	// Milvus down: an error, not an empty answer.
	_, k3 := moiThu(t)
	k3.Index.(*vectordb.Fake).Loi = errors.New("unavailable")
	if _, err := k3.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu"}); err == nil {
		t.Fatal("an index outage read as no results")
	}
}

func has(l []truyhoi.CoSuyGiam, f truyhoi.CoSuyGiam) bool {
	for _, x := range l {
		if x == f {
			return true
		}
	}
	return false
}

type rerankFn func(bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error)

func (f rerankFn) XepLai(_ context.Context, _ string, bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error) {
	return f(bc, n)
}

func TestRerankerOrdersNeverAdds(t *testing.T) {
	_, k := moiThu(t)
	k.Rerank = rerankFn(func(bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error) {
		out := append([]truyhoi.BangChung(nil), bc...)
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return out[:min(n, len(out))], nil
	})
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{DiemDenID: "da-lat"}, K: 2}
	kq, err := k.Tim(context.Background(), y)
	if err != nil || len(kq.BangChung) != 2 || has(kq.Degraded, truyhoi.NoRerank) {
		t.Fatalf("%v %v %v", idsOf(kq.BangChung), kq.Degraded, err)
	}
	// A failing reranker: the RRF order, flagged.
	_, k2 := moiThu(t)
	base, _ := k2.Tim(context.Background(), y)
	k2.Rerank = rerankFn(func(bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error) {
		return nil, errors.New("timeout")
	})
	kq, err = k2.Tim(context.Background(), y)
	if err != nil || !has(kq.Degraded, truyhoi.NoRerank) || !reflect.DeepEqual(idsOf(kq.BangChung), idsOf(base.BangChung)) {
		t.Fatalf("%v vs %v (%v)", idsOf(kq.BangChung), idsOf(base.BangChung), kq.Degraded)
	}
	// A reranker that invents an item is refused.
	k2.Rerank = rerankFn(func(bc []truyhoi.BangChung, n int) ([]truyhoi.BangChung, error) {
		return append(bc[:1:1], truyhoi.BangChung{ID: "invented"}), nil
	})
	if _, err := k2.Tim(context.Background(), y); err == nil {
		t.Fatal("an invented evidence item passed")
	}
}

func TestRefusesWhatItDoesNotServe(t *testing.T) {
	th, k := moiThu(t)
	for _, n := range []truyhoi.Nguon{truyhoi.Memory, truyhoi.GroupHistory} {
		if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: n, Cau: "x"}); !errors.Is(err, ErrNguon) {
			t.Fatalf("%s: %v", n, err)
		}
	}
	if _, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "x", Cung: truyhoi.Cung{DiUng: []string{"peanut"}}}); !errors.Is(err, vectordb.ErrLoc) {
		t.Fatalf("an unknown allergen id: %v", err)
	}
	if len(th.fake.Da) != 0 {
		t.Fatal("the index was searched for a refused request")
	}
}

// Soft preferences join the ranking text as their labels; they never reach
// the filter.
func TestSoftPreferencesRankOnly(t *testing.T) {
	th, k := moiThu(t)
	loai := tuvung.LoaiCho.IDs()[0]
	_, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Mem: truyhoi.Mem{LoaiCho: []string{loai}}})
	if err != nil {
		t.Fatal(err)
	}
	got := th.fake.Da[0]
	if !strings.Contains(got.Thua.Text, tuvung.LoaiCho.Nhan(loai)) {
		t.Fatalf("the soft preference did not reach the ranking text: %q", got.Thua.Text)
	}
	if !reflect.DeepEqual(got.Loc, vectordb.LocCung{}) {
		t.Fatalf("a soft preference became a filter: %+v", got.Loc)
	}
}
