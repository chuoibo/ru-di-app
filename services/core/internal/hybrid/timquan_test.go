package hybrid

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/vectordb"
)

// khoTimQuan is a store of cafés, restaurants and a bar with price bands
// and categories, the same on the index and on the live rows.
func khoTimQuan(t *testing.T) *Kho {
	t.Helper()
	fake := vectordb.MoiFake()
	song := map[string]thuoctinh.Hang{}
	add := func(id, text string, lo, hi int64, dm ...string) {
		tt := vectordb.ThuocTinh{DiemDen: "d-tinh-79", GiaMinVND: lo, GiaMaxVND: hi, DanhMuc: dm}
		v, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: text}})
		fake.Them(vectordb.HangDiaDiem{ID: id, Dense: v[0], Text: text, ThuocTinh: tt, GiaMaxVND: hi, PhienBan: 1})
		song[id] = thuoctinh.Hang{ThuocTinh: tt, Ten: id}
	}
	add("cafe_re", "quán cà phê sân vườn yên tĩnh", 25000, 40000, "cafe")
	add("cafe_dat", "quán cà phê rooftop view đẹp", 80000, 150000, "cafe")
	add("cafe_mo", "quán cà phê nhỏ", vectordb.GiaKhongRo, vectordb.GiaKhongRo, "cafe")
	add("bun", "quán bún bò cà phê sáng", 35000, 50000, "quan_an", "cafe")
	add("bar", "quán bar cocktail", 150000, 300000, "bar_nhau")
	add("chua_ro", "quán cà phê chưa phân loại", 30000, 50000, vectordb.KhongRo)
	return &Kho{Nhung: nhung.Stub{}, Index: fake, Thua: vectordb.BM25{}, TenDiaDiem: "rd_places",
		DocSong: DocSongHam(func(_ context.Context, ids []string) (map[string]thuoctinh.Hang, error) {
			out := map[string]thuoctinh.Hang{}
			for _, id := range ids {
				if h, ok := song[id]; ok {
					out[id] = h
				}
			}
			return out, nil
		})}
}

func timIDs(t *testing.T, k *Kho, v TimQuanVao) []string {
	t.Helper()
	kq, err := TimQuan(context.Background(), k, v)
	if err != nil {
		t.Fatal(err)
	}
	ids := idsOf(kq.BangChung)
	slices.Sort(ids)
	return ids
}

func TestTimQuanLocDanhMucVaGia(t *testing.T) {
	k := khoTimQuan(t)
	i64 := func(v int64) *int64 { return &v }
	for _, c := range []struct {
		name string
		v    TimQuanVao
		want []string
	}{
		{"category: cafés, the unclassified one out", TimQuanVao{Cau: "cà phê", DanhMuc: []string{"cafe"}},
			[]string{"bun", "cafe_dat", "cafe_mo", "cafe_re"}},
		{"UI group expands to its categories", TimQuanVao{Cau: "quán", NhomUI: "di-choi-dem"}, []string{"bar"}},
		{"band 30k-60k: overlap, unknown price out", TimQuanVao{Cau: "cà phê", DanhMuc: []string{"cafe"},
			GiaTuVND: i64(30000), GiaDenVND: i64(60000)}, []string{"bun", "cafe_re"}},
		{"floor only", TimQuanVao{Cau: "quán", GiaTuVND: i64(100000)}, []string{"bar", "cafe_dat"}},
		// No category asked: the unclassified place (cheapest 30k) stays.
		{"ceiling only", TimQuanVao{Cau: "quán", GiaDenVND: i64(30000)}, []string{"cafe_re", "chua_ro"}},
	} {
		if got := timIDs(t, k, c.v); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

var motLuc = time.Date(2026, 10, 1, 19, 0, 0, 0, vectordb.ViTri)

func TestTimQuanTuChoiBoLocSai(t *testing.T) {
	k := khoTimQuan(t)
	i64 := func(v int64) *int64 { return &v }
	for name, v := range map[string]TimQuanVao{
		"unknown UI group":        {Cau: "x", NhomUI: "quan-nuoc"},
		"unknown category":        {Cau: "x", DanhMuc: []string{"quan_nuoc"}},
		"floor above the ceiling": {Cau: "x", GiaTuVND: i64(90000), GiaDenVND: i64(50000)},
		"half a window":           {Cau: "x", MoTu: &motLuc},
	} {
		if _, err := TimQuan(context.Background(), k, v); err == nil {
			t.Errorf("%s: searched", name)
		}
	}
}

// The lexical fallback cannot hold a category or a price floor: a request
// with one is refused there, not answered wider.
func TestDuPhongKhongNoiBoLoc(t *testing.T) {
	phu := &demTim{}
	d := DuPhong{Chinh: loiKhongNhanh{}, Phu: phu}
	i64 := func(v int64) *int64 { return &v }
	for _, c := range []truyhoi.Cung{{DanhMuc: []string{"cafe"}}, {GiaTuVND: i64(1)}} {
		if _, err := d.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "x", Cung: c}); !errors.Is(err, ErrKhongNhanh) {
			t.Fatalf("%+v: %v", c, err)
		}
	}
	if phu.n != 0 {
		t.Fatalf("the fallback answered %d filtered requests", phu.n)
	}
	if _, err := d.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "x"}); err != nil || phu.n != 1 {
		t.Fatalf("an unfiltered request did not fall back: %v %d", err, phu.n)
	}
}

type loiKhongNhanh struct{}

func (loiKhongNhanh) Tim(context.Context, truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	return truyhoi.KetQuaTruyHoi{}, ErrKhongNhanh
}

type demTim struct{ n int }

func (d *demTim) Tim(context.Context, truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	d.n++
	return truyhoi.KetQuaTruyHoi{}, nil
}
