package nap

import (
	"reflect"
	"testing"

	"mobile/services/core/internal/repo"
)

func TestDinhDangGia(t *testing.T) {
	i := func(v int64) *int64 { return &v }
	for _, c := range []struct {
		lo, hi *int64
		uoc    bool
		want   string
	}{
		{i(35000), i(60000), false, "35.000–60.000 đ/người"},
		{i(35000), i(35000), false, "35.000 đ/người"},
		{i(150000), nil, false, "150.000 đ/người"},
		{i(1250000), i(3000000), true, "khoảng 1.250.000–3.000.000 đ/người (ước)"},
		{i(0), i(0), false, "0 đ/người"},
		{i(999), nil, false, "999 đ/người"},
		{nil, i(60000), false, ""},
		{nil, nil, true, ""},
	} {
		if got := DinhDangGia(c.lo, c.hi, c.uoc); got != c.want {
			t.Errorf("%v %v %v: %q, want %q", c.lo, c.hi, c.uoc, got, c.want)
		}
	}
}

// The evidence fields of a place: every key from the row, «gia» only with a
// known price, chua_ro for what is unknown.
func TestTruongHienThi(t *testing.T) {
	lo, hi := int64(40000), int64(70000)
	dc, gio := "1 Đường Hoa", "Mo-Su 07:00-22:00"
	p := repo.Place{ID: "a", Name: "Quán A", Category: "cafe", DestinationID: "d-da-lat", Address: &dc,
		PriceMinVND: &lo, PriceMaxVND: &hi, OpenHours: &gio}
	h, _ := DungHoSo(p)
	got := TruongHienThi(p, h, true)
	want := map[string]string{"ten": "Quán A", "loai": "cafe", "diem_den": "d-da-lat", "dia_chi": dc, "gio": gio,
		"gia_min_vnd": "40000", "gia_max_vnd": "70000", "gia": "khoảng 40.000–70.000 đ/người (ước)"}
	if h.Lich == nil {
		want["chua_ro"] = "gio_chua_ro"
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields:\n got %v\nwant %v", got, want)
	}
	bare := repo.Place{ID: "b", Name: "Quán B", Category: "an_vat", DestinationID: "d-da-lat"}
	hb, _ := DungHoSo(bare)
	if got := TruongHienThi(bare, hb, true); got["chua_ro"] != "gio_chua_ro,gia_chua_ro" || got["gia"] != "" || got["dia_chi"] != "" {
		t.Fatalf("an unknown place: %v", got)
	}
}
