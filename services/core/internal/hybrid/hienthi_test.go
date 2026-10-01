package hybrid

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/vectordb"
)

// ADR-0051: a hit carrying its evidence fields is answered from the index,
// with no read of Postgres; only a hit without them is read live, and only
// it.
func TestBangChungTuChiMuc(t *testing.T) {
	fake := vectordb.MoiFake()
	all := []int16{}
	for s := int16(0); s < vectordb.SoSlotTuan; s++ {
		all = append(all, s)
	}
	add := func(id, text string, f map[string]string) {
		v, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: text}})
		var mr []byte
		if f != nil {
			mr, _ = json.Marshal(map[string]any{vectordb.MoRongDau: "x", vectordb.MoRongHienThi: f})
		}
		fake.Them(vectordb.HangDiaDiem{ID: id, Dense: v[0], Text: text, MoRong: mr, PhienBan: 2,
			ThuocTinh: vectordb.ThuocTinh{DiemDen: "da-lat", OSlots: all, GiaMinVND: 50000}})
	}
	fa := map[string]string{"ten": "Lẩu Nấm A", "gia": "50.000 đ/người", "diem_den": "da-lat"}
	add("a", "quán lẩu nấm đồi thông", fa)
	add("b", "lẩu gà lá é", map[string]string{"ten": "Lẩu Gà B"})
	add("c", "lẩu bò nhúng giấm", nil) // written before the fields existed
	var read [][]string
	gia := int64(60000)
	k := &Kho{Nhung: nhung.Stub{}, Index: fake, Thua: vectordb.BM25{}, TenDiaDiem: "rd_places",
		DocSong: DocSongHam(func(_ context.Context, ids []string) (map[string]thuoctinh.Hang, error) {
			read = append(read, slices.Clone(ids))
			return map[string]thuoctinh.Hang{"c": {ThuocTinh: vectordb.ThuocTinh{DiemDen: "da-lat", OSlots: all, GiaMinVND: gia},
				Ten: "Lẩu Bò C", GiaMinVND: &gia, GiaRo: true, GioRo: true}}, nil
		})}
	kq, err := k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{DiemDenID: "da-lat"}})
	if err != nil || len(kq.BangChung) != 3 {
		t.Fatalf("%+v %v", kq.BangChung, err)
	}
	if len(read) != 1 || !slices.Equal(read[0], []string{"c"}) {
		t.Fatalf("Postgres was read for %v; only the hit without fields may be", read)
	}
	by := map[string]map[string]string{}
	for _, b := range kq.BangChung {
		by[b.ID] = b.Truong
	}
	if by["a"]["ten"] != "Lẩu Nấm A" || by["a"]["gia"] != "50.000 đ/người" || by["b"]["ten"] != "Lẩu Gà B" ||
		by["c"]["ten"] != "Lẩu Bò C" || by["c"]["gia"] != "60.000 đ/người" {
		t.Fatalf("evidence: %v", by)
	}
	if k.DocSongLuot.Load() != 1 {
		t.Fatalf("live reads counted %d", k.DocSongLuot.Load())
	}
	// No live reader: a hit without fields is dropped, never shown unread.
	k.DocSong = nil
	kq, err = k.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu", Cung: truyhoi.Cung{DiemDenID: "da-lat"}})
	if err != nil || len(kq.BangChung) != 2 {
		t.Fatalf("without a live reader: %+v %v", kq.BangChung, err)
	}
}
