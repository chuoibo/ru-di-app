package hieu

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The router writes each query twice: self-contained in the person's
// spelling, and with diacritics restored. The second form is optional,
// never blank, bounded, and dropped when it equals the first.
func TestTruyVanHaiDang(t *testing.T) {
	sua := func(q string) string {
		return strings.Replace(hopLe, `"truy_van":[{"nguon":"places","cau":"quán cà phê yên tĩnh ở Đà Lạt"}]`, `"truy_van":[`+q+`]`, 1)
	}
	kq, err := Doc([]byte(sua(`{"nguon":"places","cau":"quan cafe yen tinh o da lat","cau_co_dau":"quán cà phê yên tĩnh ở Đà Lạt"}`)), vaoMau(obs.BotNhom))
	if err != nil || !reflect.DeepEqual(kq.TruyVan, []TruyVan{{Nguon: truyhoi.Places, Cau: "quan cafe yen tinh o da lat", CauCoDau: "quán cà phê yên tĩnh ở Đà Lạt"}}) ||
		kq.TruyVan[0].CoDau() != "quán cà phê yên tĩnh ở Đà Lạt" {
		t.Fatalf("%+v %v", kq.TruyVan, err)
	}
	kq, err = Doc([]byte(sua(`{"nguon":"places","cau":"quán lẩu","cau_co_dau":"quán lẩu"}`)), vaoMau(obs.BotNhom))
	if err != nil || kq.TruyVan[0].CauCoDau != "" || kq.TruyVan[0].CoDau() != "quán lẩu" {
		t.Fatalf("same form: %+v %v", kq.TruyVan, err)
	}
	for name, q := range map[string]string{
		"blank second form":    `{"nguon":"places","cau":"quan lau","cau_co_dau":""}`,
		"second form too long": `{"nguon":"places","cau":"quan lau","cau_co_dau":"` + strings.Repeat("ẩ", MaxChuTruyVan+1) + `"}`,
		"unknown query field":  `{"nguon":"places","cau":"quan lau","cau_khong_dau":"quan lau"}`,
	} {
		if _, err := Doc([]byte(sua(q)), vaoMau(obs.BotNhom)); !errors.Is(err, ErrCauTruc) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The request is laid out for the implicit cache: the closed lists first,
// the clock and the message last.
func TestNoiDungTinhTruocDongSau(t *testing.T) {
	v := vaoNep()
	stub := llm.NewStub(buoc(hopLeNep)...)
	if _, err := Moi().Hieu(context.Background(), v, demMoi(stub, 8)); err != nil {
		t.Fatal(err)
	}
	req := string(stub.YeuCau()[0])
	req = req[strings.Index(req, `"contents"`):]
	diemDen := strings.Index(req, `nguon=\"danh_sach_diem_den\"`)
	nganHan := strings.Index(req, `nguon=\"ngan_han\"`)
	mayChu := strings.Index(req, `nguon=\"may_chu\"`)
	cauHoi := strings.Index(req, `nguon=\"cau_hoi\"`)
	if diemDen < 0 || nganHan < 0 || mayChu < 0 || cauHoi < 0 || !(diemDen < nganHan && nganHan < mayChu && mayChu < cauHoi) {
		t.Fatalf("block order: danh_sach_diem_den %d, ngan_han %d, may_chu %d, cau_hoi %d", diemDen, nganHan, mayChu, cauHoi)
	}
	// The system instruction is the bot's static text: the same bytes for
	// any turn.
	v2 := vaoNep()
	v2.Cau = "một câu khác hẳn"
	stub2 := llm.NewStub(buoc(hopLeNep)...)
	if _, err := Moi().Hieu(context.Background(), v2, demMoi(stub2, 8)); err != nil {
		t.Fatal(err)
	}
	he := func(r []byte) string {
		s := string(r)
		return s[strings.Index(s, `"systemInstruction"`):strings.Index(s, `"thinkingConfig"`)]
	}
	if he(stub.YeuCau()[0]) != he(stub2.YeuCau()[0]) {
		t.Fatal("the router's system instruction changes with the turn")
	}
}
