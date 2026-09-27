package rag

import (
	"slices"
	"testing"

	"mobile/services/core/internal/ingest"
)

// curatedVN are the curated destinations of services/api/app/places/
// destinations_vn.py, the rows a deployed catalogue holds beside the
// province destinations `rudi-ingest migrate` seeds.
var curatedVN = []DiemDen{
	{ID: "d-da-lat", Ten: "Đà Lạt", Lat: 11.9404, Lng: 108.4583, Nam: 11.88, Tay: 108.38, Bac: 12.0, Dong: 108.52},
	{ID: "d-tphcm", Ten: "TP. Hồ Chí Minh", Lat: 10.7769, Lng: 106.7009, Nam: 10.72, Tay: 106.65, Bac: 10.83, Dong: 106.75},
	{ID: "d-ha-noi", Ten: "Hà Nội", Lat: 21.0285, Lng: 105.8542, Nam: 20.98, Tay: 105.81, Bac: 21.07, Dong: 105.9},
	{ID: "d-da-nang", Ten: "Đà Nẵng", Lat: 16.0544, Lng: 108.2022, Nam: 16.0, Tay: 108.15, Bac: 16.12, Dong: 108.28},
	{ID: "d-hoi-an", Ten: "Hội An", Lat: 15.8801, Lng: 108.338, Nam: 15.85, Tay: 108.3, Bac: 15.92, Dong: 108.39},
	{ID: "d-nha-trang", Ten: "Nha Trang", Lat: 12.2388, Lng: 109.1967, Nam: 12.19, Tay: 109.16, Bac: 12.29, Dong: 109.24},
	{ID: "d-hue", Ten: "Huế", Lat: 16.4637, Lng: 107.5909, Nam: 16.42, Tay: 107.54, Bac: 16.51, Dong: 107.64},
	{ID: "d-sa-pa", Ten: "Sa Pa", Lat: 22.3364, Lng: 103.8438, Nam: 22.29, Tay: 103.8, Bac: 22.38, Dong: 103.89},
	{ID: "d-phu-quoc", Ten: "Phú Quốc", Lat: 10.227, Lng: 103.964, Nam: 10.15, Tay: 103.9, Bac: 10.35, Dong: 104.06},
	{ID: "d-vung-tau", Ten: "Vũng Tàu", Lat: 10.346, Lng: 107.0843, Nam: 10.31, Tay: 107.05, Bac: 10.4, Dong: 107.13},
	{ID: "d-can-tho", Ten: "Cần Thơ", Lat: 10.0452, Lng: 105.7469, Nam: 10.01, Tay: 105.71, Bac: 10.08, Dong: 105.8},
	{ID: "d-ha-long", Ten: "Hạ Long", Lat: 20.9515, Lng: 107.0748, Nam: 20.91, Tay: 107.02, Bac: 21.0, Dong: 107.14},
	{ID: "d-quy-nhon", Ten: "Quy Nhơn", Lat: 13.7829, Lng: 109.2196, Nam: 13.74, Tay: 109.18, Bac: 13.82, Dong: 109.27},
	{ID: "d-ninh-binh", Ten: "Ninh Bình", Lat: 20.2506, Lng: 105.9745, Nam: 20.18, Tay: 105.9, Bac: 20.32, Dong: 106.03},
	{ID: "d-mui-ne", Ten: "Mũi Né", Lat: 10.933, Lng: 108.287, Nam: 10.9, Tay: 108.22, Bac: 10.97, Dong: 108.35},
}

// withProvinces is the curated destinations and every province destination,
// as ingest.SeedProvinceDestinations writes them.
func withProvinces() []DiemDen {
	out := slices.Clone(curatedVN)
	for _, p := range ingest.ProvinceBoxes {
		out = append(out, DiemDen{ID: ingest.ProvinceDestinationID(p.Code), Ten: p.Name, Tinh: p.Name,
			Lat: p.Lat, Lng: p.Lng, Nam: p.South, Tay: p.West, Bac: p.North, Dong: p.East})
	}
	return out
}

// Every curated destination lies in the province of the post-July-2025
// structure it belongs to -- including the merged ones (Vũng Tàu in TP.HCM,
// Hội An in Đà Nẵng, Quy Nhơn in Gia Lai, Mũi Né in Lâm Đồng, Phú Quốc in
// An Giang) and the ones a second, larger box also holds (Khánh Hòa's box
// reaches Đà Lạt, Huế's reaches Đà Nẵng).
func TestTinhCuaMoiDiemDenCuaTuyenChon(t *testing.T) {
	dests := withProvinces()
	want := map[string]int16{
		"d-da-lat": 68, "d-tphcm": 79, "d-ha-noi": 1, "d-da-nang": 48, "d-hoi-an": 48,
		"d-nha-trang": 56, "d-hue": 46, "d-sa-pa": 15, "d-phu-quoc": 91, "d-vung-tau": 79,
		"d-can-tho": 92, "d-ha-long": 22, "d-quy-nhon": 52, "d-ninh-binh": 37, "d-mui-ne": 68,
	}
	for _, d := range curatedVN {
		if got := TinhCua(dests, d.ID); got != ingest.ProvinceDestinationID(want[d.ID]) {
			t.Errorf("TinhCua(%s) = %q, want d-tinh-%d", d.ID, got, want[d.ID])
		}
	}
	if got := TinhCua(dests, "d-tinh-68"); got != "" {
		t.Errorf("a province lies in no province: %q", got)
	}
	// Curated destinations only: nothing to relate.
	if got := TinhCua(curatedVN, "d-da-lat"); got != "" {
		t.Errorf("no province destinations, yet %q", got)
	}
}

// With the province destinations present, the words still name the curated
// city (the verifier's four cases, which resolved to "" before), and a
// curated city's scope takes the province rows inside its box.
func TestResolveDestinationVoiDiemDenTinh(t *testing.T) {
	for _, dests := range [][]DiemDen{curatedVN, withProvinces()} {
		for _, c := range []struct{ cau, khuVuc, want string }{
			{"lẩu Hà Nội", "", "d-ha-noi"},
			{"cafe Đà Nẵng", "", "d-da-nang"},
			{"quán ở Hồ Chí Minh", "", "d-tphcm"},
			{"", "hcm-quan-1", "d-tphcm"},
			{"quán ngon ở Đà Lạt", "", "d-da-lat"},
		} {
			got := ResolveDestination(dests, GoiY{Cau: c.cau, KhuVuc: c.khuVuc})
			if got.ID != c.want {
				t.Errorf("%d destinations, %q/%q: %+v, want %s", len(dests), c.cau, c.khuVuc, got, c.want)
			}
		}
	}
	dests := withProvinces()
	// A province named alone is that province.
	if got := ResolveDestination(dests, GoiY{Cau: "đi Lâm Đồng chơi"}); got.ID != "d-tinh-68" {
		t.Errorf("Lâm Đồng: %+v", got)
	}
	// Two cities are still two.
	if got := ResolveDestination(dests, GoiY{Cau: "Hà Nội hay Đà Nẵng"}); got.ID != "" || !got.NhieuDiemDen {
		t.Errorf("two cities: %+v", got)
	}
}

func TestPhamViCua(t *testing.T) {
	dests := withProvinces()
	dl := PhamViCua(dests, "d-da-lat")
	if !slices.Equal(dl.Tron, []string{"d-da-lat"}) || !slices.Contains(dl.Tinh, "d-tinh-68") || dl.Hop == nil {
		t.Fatalf("Đà Lạt's scope: %+v", dl)
	}
	for _, c := range []struct {
		dd       string
		lat, lng float64
		co       bool
		want     bool
	}{
		{"d-da-lat", 0, 0, false, true},          // filed under the city, no coordinates
		{"d-tinh-68", 11.94, 108.45, true, true}, // ingested, inside Đà Lạt
		{"d-tinh-68", 11.55, 107.8, true, false}, // ingested, Bảo Lộc: Lâm Đồng, not Đà Lạt
		{"d-tinh-68", 0, 0, false, false},        // ingested, no coordinates
		{"d-tinh-56", 11.94, 108.45, true, true}, // filed under the overlapping box: still inside Đà Lạt
		{"d-tphcm", 11.94, 108.45, true, false},  // another curated city never
	} {
		if got := dl.Chua(c.dd, c.lat, c.lng, c.co); got != c.want {
			t.Errorf("Đà Lạt scope, %s at %v,%v (%v): %v", c.dd, c.lat, c.lng, c.co, got)
		}
	}
	ld := PhamViCua(dests, "d-tinh-68")
	if !slices.Equal(ld.Tron, []string{"d-tinh-68", "d-da-lat", "d-mui-ne"}) || ld.Hop != nil {
		t.Fatalf("Lâm Đồng's scope: %+v", ld)
	}
	if !ld.Chua("d-da-lat", 0, 0, false) || ld.Chua("d-tinh-56", 11.94, 108.45, true) {
		t.Fatal("Lâm Đồng takes Đà Lạt's rows and no other province's")
	}
	// Without province destinations a curated scope is the id alone, as before.
	if pv := PhamViCua(curatedVN, "d-da-lat"); !slices.Equal(pv.Tron, []string{"d-da-lat"}) || pv.Hop != nil || len(pv.Tinh) != 0 {
		t.Fatalf("curated only: %+v", pv)
	}
	if pv := PhamViCua(dests, ""); !pv.Chua("d-tinh-1", 0, 0, false) {
		t.Fatal("no destination is every destination")
	}
	if pv := PhamViCua(dests, "d-khong-co"); pv.Chua("d-da-lat", 11.94, 108.45, true) {
		t.Fatal("an unknown destination took another's rows")
	}
}
