package rag

import (
	"reflect"
	"testing"

	"mobile/services/core/internal/domain/giomo"
)

// RRF arithmetic, by hand: k = 60, ⌊10⁹/(60+rank)⌋, summed, ties by id.
func TestFuseIsIntegerRRF(t *testing.T) {
	if diemRRF(1) != 16393442 || diemRRF(2) != 16129032 || diemRRF(50) != 9090909 {
		t.Fatalf("contributions %d %d %d", diemRRF(1), diemRRF(2), diemRRF(50))
	}
	got := fuse(
		[]hang{{"a", 1}, {"b", 2}, {"c", 3}},
		[]hang{{"c", 1}, {"a", 2}},
		[]hang{{"d", 1}, {"b", 1}},
	)
	want := []Hit{
		{ID: "b", Diem: 16129032 + 16393442},
		{ID: "a", Diem: 16393442 + 16129032},
		{ID: "c", Diem: 15873015 + 16393442},
		{ID: "d", Diem: 16393442},
	}
	// a and b tie exactly; the id decides.
	want[0], want[1] = want[1], want[0]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fuse = %+v\nwant   %+v", got, want)
	}
	if fuse() == nil || len(fuse()) != 0 {
		t.Fatal("no lists is no hits")
	}
}

func TestXepDiemIsDenseAndCut(t *testing.T) {
	got := xepDiem(map[string]float64{"x": 2, "y": 2, "z": 1, "w": 3}, 3)
	want := []hang{{"w", 1}, {"x", 2}, {"y", 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("xepDiem = %+v", got)
	}
}

func TestThuatTruyVanDropsStopWordsAndSlotSpans(t *testing.T) {
	y := YeuCau{Cau: "Quán lẩu nấm nào ở Đà Lạt ngon?", Bo: [][]string{{"da", "lat"}}}
	got := thuatTruyVan(y)
	want := []string{"lau", "nam", "ngon", "lau_nam"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("terms %q, want %q", got, want)
	}
	if q := tsQuery(got); q != "'lau' | 'nam' | 'ngon' | 'launam'" {
		t.Fatalf("tsquery %q", q)
	}
	if tsQuery(thuatTruyVan(YeuCau{Cau: "ở đâu vậy?"})) != "'vay'" {
		t.Fatal("stop words leaked")
	}
	if tsQuery(nil) != "" {
		t.Fatal("empty query")
	}
}

func TestDocCauReadsSlotsFromWords(t *testing.T) {
	dests := []DiemDen{
		{ID: "d-da-lat", Ten: "Đà Lạt", Nam: 11.88, Tay: 108.38, Bac: 12, Dong: 108.52},
		{ID: "d-tphcm", Ten: "TP. Hồ Chí Minh", Nam: 10.68, Tay: 106.6, Bac: 10.88, Dong: 106.82},
	}
	y, dd := DocCau("Tìm quán cà phê yên tĩnh ở Sài Gòn, bạn mình dị ứng sữa tươi, ăn chay", dests)
	if dd.ID != "d-tphcm" || y.DiemDen != "d-tphcm" || dd.Nguon != NguonTen {
		t.Fatalf("destination %+v", dd)
	}
	if !reflect.DeepEqual(y.DiUng, []string{"sua"}) || !reflect.DeepEqual(y.AnKieng, []string{"chay"}) ||
		!reflect.DeepEqual(y.LoaiCho, []string{"cafe"}) || !reflect.DeepEqual(y.KhiChat, []string{"yen_tinh"}) {
		t.Fatalf("slots %+v", y)
	}
	if !reflect.DeepEqual(thuatTruyVan(y)[:3], []string{"ca", "phe", "yen"}) {
		t.Fatalf("terms %q", thuatTruyVan(y))
	}
}

var dests = []DiemDen{
	{ID: "d-da-lat", Ten: "Đà Lạt", Nam: 11.88, Tay: 108.38, Bac: 12, Dong: 108.52},
	{ID: "d-tphcm", Ten: "TP. Hồ Chí Minh", Nam: 10.68, Tay: 106.6, Bac: 10.88, Dong: 106.82},
	{ID: "d-hoi-an", Ten: "Hội An", Nam: 15.84, Tay: 108.28, Bac: 15.92, Dong: 108.38},
}

// ResolveDestination never defaults: with nothing that names a destination
// it says so, however many destinations there are and whichever sorts first.
func TestResolveDestinationNeverDefaults(t *testing.T) {
	cases := []struct {
		ten   string
		g     GoiY
		id    string
		nguon string
		nhieu bool
	}{
		{"không gì cả: hỏi lại, không lấy Đà Lạt", GoiY{Cau: "tối nay đi đâu ăn lẩu"}, "", "", false},
		{"câu rỗng", GoiY{}, "", "", false},
		{"vùng Quận 1 nằm trong hộp TP.HCM", GoiY{KhuVuc: "hcm-quan-1", Cau: "ở Đà Lạt"}, "d-tphcm", NguonKhuVuc, false},
		{"vùng lạ bị bỏ qua, xuống tên", GoiY{KhuVuc: "khong-co", Cau: "đi Hội An"}, "d-hoi-an", NguonTen, false},
		{"tên có dấu", GoiY{Cau: "quán ngon ở Đà Lạt"}, "d-da-lat", NguonTen, false},
		{"tên không dấu", GoiY{Cau: "quan ngon o da lat"}, "d-da-lat", NguonTen, false},
		{"tên gọi khác", GoiY{Cau: "ăn gì ở Sài Gòn"}, "d-tphcm", NguonTen, false},
		{"bỏ tiền tố TP.", GoiY{Cau: "cà phê Hồ Chí Minh"}, "d-tphcm", NguonTen, false},
		{"nhãn vùng trong câu", GoiY{Cau: "lẩu ở Quận 3"}, "d-tphcm", NguonTen, false},
		{"«hỏi ăn» có dấu không phải Hội An", GoiY{Cau: "mình hỏi ăn gì ngon"}, "", "", false},
		{"«hoi an» không dấu thì đoán là Hội An", GoiY{Cau: "hoi an co gi ngon"}, "d-hoi-an", NguonTen, false},
		{"hai điểm đến thì không chọn", GoiY{Cau: "Đà Lạt hay Hội An"}, "", "", true},
		{"phiếu Nếp đa số", GoiY{TuPhieu: []string{"d-hoi-an", "d-hoi-an", "d-da-lat"}}, "d-hoi-an", NguonPhieu, false},
		{"phiếu hoà thì hỏi lại", GoiY{TuPhieu: []string{"d-hoi-an", "d-da-lat"}}, "", "", false},
		// Canary 2's unit half: a group whose stops are in HCM gets HCM.
		{"chặng của nhóm ở TP.HCM", GoiY{Cau: "tối nay ăn gì", TuChang: []string{"d-tphcm", "d-tphcm"}}, "d-tphcm", NguonChang, false},
		{"chặng lạ bị bỏ", GoiY{TuChang: []string{"d-khong-co"}}, "", "", false},
		{"tên thắng chặng", GoiY{Cau: "đi Đà Lạt", TuChang: []string{"d-tphcm"}}, "d-da-lat", NguonTen, false},
	}
	for _, c := range cases {
		got := ResolveDestination(dests, c.g)
		if got.ID != c.id || got.Nguon != c.nguon || got.NhieuDiemDen != c.nhieu {
			t.Errorf("%s: %+v", c.ten, got)
		}
		if (got.ID == "") != (got.Thieu == "khu_vuc") {
			t.Errorf("%s: unresolved must ask for khu_vuc, got %+v", c.ten, got)
		}
	}
}

func TestRangBuocGoesLastAndDropsUnknownHoursOnceThreeAreOpen(t *testing.T) {
	at := 600
	r := rangBuocCua(YeuCau{Luc: &at})
	// Two places known to be open (a, and u2 whose price is what is
	// unknown): the hours-unknown u1 stays, after both.
	hits := []Hit{{ID: "u1", Co: []string{CoGioChuaRo}}, {ID: "a"}, {ID: "u2", Co: []string{CoGiaChuaRo}}}
	got := r.xepChuaRo(append([]Hit(nil), hits...))
	if len(got) != 3 || got[0].ID != "a" || got[1].ID != "u1" || got[2].ID != "u2" {
		t.Fatalf("order %+v", got)
	}
	// A third known-open place: u1 goes.
	got = r.xepChuaRo(append(hits, Hit{ID: "b"}))
	for _, h := range got {
		if h.ID == "u1" {
			t.Fatalf("hours-unknown place kept with three known open: %+v", got)
		}
	}
	// Known hours, closed at the time: out.
	l, _ := giomo.Doc("18:00 – 22:00")
	if ok, _ := r.dat(HoSo{Lich: &l}); ok {
		t.Fatal("a closed place passed")
	}
}
