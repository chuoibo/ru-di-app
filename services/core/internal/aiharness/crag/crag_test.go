package crag

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/truyhoi"
)

func TestDocTuChoi(t *testing.T) {
	ok := []string{
		`{"ket_luan":"du","rang_buoc_thieu":[]}`,
		`{"ket_luan":"thieu","rang_buoc_thieu":["khi_chat"],"noi_long":["khi_chat","khu_vuc"]}`,
		`{"ket_luan":"mau_thuan","rang_buoc_thieu":["di_ung"],"viet_lai":"quán không có tôm"}`,
	}
	for _, s := range ok {
		if _, err := Doc([]byte(s)); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	bad := map[string]string{
		"relax a hard constraint": `{"ket_luan":"thieu","rang_buoc_thieu":["di_ung"],"noi_long":["di_ung"]}`,
		"relax the destination":   `{"ket_luan":"thieu","rang_buoc_thieu":[],"noi_long":["diem_den"]}`,
		"relax the budget":        `{"ket_luan":"thieu","rang_buoc_thieu":[],"noi_long":["ngan_sach"]}`,
		"unknown verdict":         `{"ket_luan":"ok","rang_buoc_thieu":[]}`,
		"unknown constraint":      `{"ket_luan":"thieu","rang_buoc_thieu":["gia"]}`,
		"both moves":              `{"ket_luan":"thieu","rang_buoc_thieu":[],"noi_long":["khi_chat"],"viet_lai":"x"}`,
		"sufficient with a move":  `{"ket_luan":"du","rang_buoc_thieu":[],"viet_lai":"x"}`,
		"missing list":            `{"ket_luan":"thieu"}`,
		"unknown field":           `{"ket_luan":"du","rang_buoc_thieu":[],"ly_do":"x"}`,
		"rewrite too long":        `{"ket_luan":"thieu","rang_buoc_thieu":[],"viet_lai":"` + strings.Repeat("a", MaxVietLai+1) + `"}`,
	}
	for name, s := range bad {
		if _, err := Doc([]byte(s)); !errors.Is(err, ErrCauTruc) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
}

func TestSuaYeuCauKhongDungRangBuocCung(t *testing.T) {
	n := int64(200000)
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "cafe",
		Cung: truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"tom"}, NganSachVND: &n},
		Mem:  truyhoi.Mem{LoaiCho: []string{"cafe"}, KhiChat: []string{"yen_tinh"}, KhuVuc: "ho-xuan-huong"}}
	d, _ := Doc([]byte(`{"ket_luan":"thieu","rang_buoc_thieu":["khi_chat"],"noi_long":["khi_chat","khu_vuc"]}`))
	got, ok, err := SuaYeuCau(y, d, 1)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if !reflect.DeepEqual(got.Cung, y.Cung) || got.Mem.KhiChat != nil || got.Mem.KhuVuc != "" || !reflect.DeepEqual(got.Mem.LoaiCho, []string{"cafe"}) || got.Cau != "cafe" {
		t.Fatalf("%+v", got)
	}
	if _, _, err := SuaYeuCau(y, d, 2); !errors.Is(err, ErrHetVong) {
		t.Fatal("a second corrective round ran")
	}
	w, _ := Doc([]byte(`{"ket_luan":"thieu","rang_buoc_thieu":[],"viet_lai":"cafe yên tĩnh gần hồ"}`))
	if got, ok, _ := SuaYeuCau(y, w, 1); !ok || got.Cau != "cafe yên tĩnh gần hồ" || !reflect.DeepEqual(got.Mem, y.Mem) {
		t.Fatalf("%+v", got)
	}
	du, _ := Doc([]byte(`{"ket_luan":"du","rang_buoc_thieu":[]}`))
	if _, ok, err := SuaYeuCau(y, du, 1); ok || err != nil {
		t.Fatal("a sufficient verdict ran a round")
	}
	if _, err := (truyhoi.Mem{}).Bo(truyhoi.RBDiUng); err == nil {
		t.Fatal("Mem.Bo dropped a hard constraint name")
	}
}
