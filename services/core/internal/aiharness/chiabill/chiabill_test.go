package chiabill

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/allocator"
)

func vaoThu() Vao {
	return Vao{Tin: []Tin{
		{BiDanh: "t1", Ten: "Lan", Chu: "Mình trả lẩu 850k hôm qua"},
		{BiDanh: "t2", Ten: "Minh", Chu: "Karaoke hết 1tr2 nha, mình trả luôn"},
	}, LoiNho: "chia bill, taxi 120.000đ mình trả"}
}

// A float or an exponent is refused, never rounded; an alias outside the
// reading, a missing or unknown field, a number in a string, an empty object
// are refused (ErrCauTruc). Moved here from aiharness (review of slices
// 9/11, finding 2.2: the package had no test file of its own).
func TestDocTuChoiCauTruc(t *testing.T) {
	v := vaoThu()
	for _, bad := range []string{
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000.5}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":8.5e5}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":-850000}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":0}]}`,
		fmt.Sprintf(`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":%d}]}`, int64(allocator.MaxAmountVND)+1),
		`{"khoan":[{"tin":"t9","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000}]}`,
		`{"khoan":[{"tin":"t1","so_tien_goc":"850k","so_tien_vnd":850000}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_vnd":850000}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000,"nguoi_tra":"Minh"}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":"850000"}]}`,
		`{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"` + strings.Repeat("8", MaxChuSoTienGoc+1) + `","so_tien_vnd":850000}]}`,
		`{}`,
	} {
		if _, err := Doc([]byte(bad), v); !errors.Is(err, ErrCauTruc) {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

// The amount must be supported by the message it names, structurally: the
// quote is a substring of that message (cutting no number) and its digits
// spell the amount up to a power of ten. Anything else refuses the reading
// with ErrSoTienKhongKhop, never ErrCauTruc (the engine asks back).
func TestDocSoTienPhaiCoTrongTin(t *testing.T) {
	v := vaoThu()
	for _, c := range []struct {
		ten, raw string
	}{
		{"so_bia", `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":9990000}]}`},
		{"trich_khong_co", `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"999k","so_tien_vnd":999000}]}`},
		{"trich_tin_khac", `{"khoan":[{"tin":"t2","tieu_de":"Karaoke","so_tien_goc":"850k","so_tien_vnd":850000}]}`},
		{"cat_giua_so", `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"50k","so_tien_vnd":50000}]}`},
		{"khong_chu_so", `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"lẩu","so_tien_vnd":850000}]}`},
		{"nho_hon_chu_so", `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":85}]}`},
		{"rong", `{"khoan":[{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"","so_tien_vnd":850000}]}`},
		{"loi_nho_bia", `{"khoan":[{"tin":"loi_nho","tieu_de":"taxi","so_tien_goc":"120.000đ","so_tien_vnd":150000}]}`},
	} {
		if _, err := Doc([]byte(c.raw), v); !errors.Is(err, ErrSoTienKhongKhop) || errors.Is(err, ErrCauTruc) {
			t.Errorf("%s: %v", c.ten, err)
		}
	}
	// Identity: the message's own amounts, in its own words.
	ks, err := Doc([]byte(`{"khoan":[
		{"tin":"t1","tieu_de":"lẩu","so_tien_goc":"850k","so_tien_vnd":850000},
		{"tin":"t2","tieu_de":"Karaoke","so_tien_goc":"1tr2","so_tien_vnd":1200000},
		{"tin":"loi_nho","tieu_de":"taxi","so_tien_goc":"120.000đ","so_tien_vnd":120000}]}`), v)
	if err != nil || len(ks) != 3 || ks[0].SoTienGoc != "850k" || ks[1].SoTienGoc != "1tr2" || ks[2].SoTienVND != 120000 {
		t.Fatalf("%v %+v", err, ks)
	}
}

// A title is kept only as a run of WHOLE words of its message, taken from the
// message; a part of a word, a paraphrase or an added word is dropped and the
// item kept (the reviewer's mutant G13 accepted any substring and returned
// the model's string).
func TestDocTieuDeLaTuTronVen(t *testing.T) {
	v := vaoThu()
	for _, c := range []struct {
		tieuDe, muon string
	}{
		{"lẩu", "lẩu"},
		{"trả lẩu", "trả lẩu"},
		{"trả  lẩu", "trả lẩu"},
		{"ả lẩ", ""},
		{"lẩ", ""},
		{"Lẩu", ""},
		{"lẩu hải sản", ""},
		{"lẩu 850k hôm", "lẩu 850k hôm"},
		{"", ""},
	} {
		raw := `{"khoan":[{"tin":"t1","tieu_de":"` + c.tieuDe + `","so_tien_goc":"850k","so_tien_vnd":850000}]}`
		ks, err := Doc([]byte(raw), v)
		if err != nil || len(ks) != 1 {
			t.Fatalf("%q: %v", c.tieuDe, err)
		}
		if ks[0].TieuDe != c.muon || ks[0].TieuDeBo != (c.muon == "") {
			t.Errorf("title %q: kept %q (dropped %v), want %q", c.tieuDe, ks[0].TieuDe, ks[0].TieuDeBo, c.muon)
		}
	}
}

// The verifier's output is read strictly: every item judged exactly once,
// only the closed verdicts, nothing else; one unsupported item fails the
// whole draft.
func TestDocKiem(t *testing.T) {
	for _, bad := range []string{
		`{"khoan":[{"so":1,"ket":"ho_tro"}]}`,
		`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":1,"ket":"ho_tro"}]}`,
		`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":3,"ket":"ho_tro"}]}`,
		`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":2,"ket":"co_the"}]}`,
		`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":2}]}`,
		`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":2,"ket":"ho_tro","ly_do":"x"}]}`,
		`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":2,"ket":"ho_tro"}],"dat":true}`,
		`{}`,
		`[]`,
	} {
		if _, err := DocKiem([]byte(bad), 2); !errors.Is(err, ErrCauTruc) {
			t.Errorf("%s: %v", bad, err)
		}
	}
	if dat, err := DocKiem([]byte(`{"khoan":[{"so":2,"ket":"ho_tro"},{"so":1,"ket":"ho_tro"}]}`), 2); err != nil || !dat {
		t.Fatalf("all supported: %v %v", dat, err)
	}
	if dat, err := DocKiem([]byte(`{"khoan":[{"so":1,"ket":"ho_tro"},{"so":2,"ket":"khong_ho_tro"}]}`), 2); err != nil || dat {
		t.Fatalf("one unsupported: %v %v", dat, err)
	}
}

// The verifier sees each item beside only the message it names, its writer
// and our formatting of the amount; a reading's alias outside it is refused.
func TestNoiDungKiem(t *testing.T) {
	v := vaoThu()
	ks := []Khoan{{Tin: "t2", TieuDe: "Karaoke", SoTienVND: 1200000, SoTienGoc: "1tr2"}, {Tin: BiDanhLoiNho, TieuDe: "taxi", SoTienVND: 120000, SoTienGoc: "120.000đ"}}
	body, err := NoiDungKiem(v, ks, "Tú")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1.200.000đ", "120.000đ", "Minh", "Tú", "Karaoke", `nguon="khoan_can_kiem"`} {
		if !strings.Contains(body, want) {
			t.Errorf("verifier body lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "850k") || strings.Contains(body, "Lan") {
		t.Errorf("the verifier saw a message no item names:\n%s", body)
	}
	if _, err := NoiDungKiem(v, []Khoan{{Tin: "t9", SoTienVND: 1}}, "Tú"); !errors.Is(err, ErrCauTruc) {
		t.Fatalf("an alias outside the reading: %v", err)
	}
	if _, err := NoiDungKiem(v, nil, "Tú"); !errors.Is(err, ErrCauTruc) {
		t.Fatalf("nothing to verify: %v", err)
	}
	if DongTien(1250000) != "1.250.000đ" || DongTien(7) != "7đ" {
		t.Fatal("DongTien")
	}
}
