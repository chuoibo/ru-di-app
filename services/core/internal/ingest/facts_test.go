package ingest

import (
	"testing"

	"mobile/services/core/internal/domain/giomo"
)

func TestGioOSM(t *testing.T) {
	cases := []struct {
		name, raw, want string
		ok              bool
	}{
		{"every day alike", `[{"thu":"mon","mo":"08:00","dong":"22:00","qua_dem":false},{"thu":"tue","mo":"08:00","dong":"22:00","qua_dem":false},{"thu":"wed","mo":"08:00","dong":"22:00","qua_dem":false},{"thu":"thu","mo":"08:00","dong":"22:00","qua_dem":false},{"thu":"fri","mo":"08:00","dong":"22:00","qua_dem":false},{"thu":"sat","mo":"08:00","dong":"22:00","qua_dem":false},{"thu":"sun","mo":"08:00","dong":"22:00","qua_dem":false}]`,
			"Mo-Su 08:00-22:00", true},
		{"sunday not listed is closed", `[{"thu":"mon","mo":"06:00","dong":"21:30"},{"thu":"tue","mo":"06:00","dong":"21:30"},{"thu":"wed","mo":"06:00","dong":"21:30"},{"thu":"thu","mo":"06:00","dong":"21:30"},{"thu":"fri","mo":"06:00","dong":"21:30"},{"thu":"sat","mo":"06:00","dong":"21:30"}]`,
			"Mo-Sa 06:00-21:30; Su off", true},
		{"two shifts, listed out of order", `[{"thu":"mon","mo":"17:00","dong":"22:00"},{"thu":"mon","mo":"06:00","dong":"10:00"}]`,
			"Mo 06:00-10:00,17:00-22:00; Tu-Su off", true},
		{"overnight", `[{"thu":"fri","mo":"18:00","dong":"02:00","qua_dem":true}]`,
			"Mo-Th off; Fr 18:00-02:00; Sa-Su off", true},
		{"midnight end", `[{"thu":"sat","mo":"10:00","dong":"24:00","qua_dem":false}]`,
			"Mo-Fr off; Sa 10:00-24:00; Su off", true},
		{"a repeated shift counts once", `[{"thu":"tue","mo":"07:00","dong":"11:00"},{"thu":"tue","mo":"07:00","dong":"11:00"}]`,
			"Mo off; Tu 07:00-11:00; We-Su off", true},
		{"empty is unknown, not closed", `[]`, "", false},
		{"qua_dem disagreeing with the times", `[{"thu":"mon","mo":"08:00","dong":"22:00","qua_dem":true}]`, "", false},
		{"unknown weekday", `[{"thu":"hol","mo":"08:00","dong":"22:00"}]`, "", false},
		{"malformed time", `[{"thu":"mon","mo":"8h","dong":"22:00"}]`, "", false},
		{"start at 24:00", `[{"thu":"mon","mo":"24:00","dong":"02:00","qua_dem":true}]`, "", false},
		{"not a list", `{"thu":"mon"}`, "", false},
	}
	for _, c := range cases {
		got, ok := GioOSM([]byte(c.raw))
		if got != c.want || ok != c.ok {
			t.Errorf("%s: GioOSM = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestGioOSMReadsBackInGiomo: the string is for giomo, so the schedule it
// reads must be the one fed -- Sunday closed stays closed, an overnight
// Friday is open at 01:00 on Saturday.
func TestGioOSMReadsBackInGiomo(t *testing.T) {
	s, ok := GioOSM([]byte(`[{"thu":"fri","mo":"18:00","dong":"02:00","qua_dem":true},{"thu":"mon","mo":"06:00","dong":"21:30"}]`))
	if !ok {
		t.Fatal("did not read")
	}
	l, ok := giomo.Doc(s)
	if !ok {
		t.Fatalf("giomo does not read %q", s)
	}
	at := func(day, h, m int) int { return day*24*60 + h*60 + m }
	for _, c := range []struct {
		m    int
		open bool
	}{
		{at(5, 1, 0), true},   // Saturday 01:00, from Friday night
		{at(5, 3, 0), false},  // Saturday 03:00
		{at(0, 7, 0), true},   // Monday 07:00
		{at(6, 12, 0), false}, // Sunday: not listed, closed
		{at(1, 12, 0), false}, // Tuesday: not listed, closed
	} {
		if l.MoLuc(c.m) != c.open {
			t.Errorf("minute %d: open=%v, want %v (%q)", c.m, !c.open, c.open, s)
		}
	}
}

func i64(v int64) *int64   { return &v }
func str(s string) *string { return &s }

func TestGiaMoiNguoi(t *testing.T) {
	cases := []struct {
		name   string
		row    FactRow
		lo, hi *int64
		uoc    bool
	}{
		{"per-person band first", FactRow{GiaNguoiMin: i64(35000), GiaNguoiMax: i64(60000), GiaNguoiCoSo: str("review_web"),
			GiaMin: i64(20000), GiaMax: i64(90000), GiaDonVi: str("moi_nguoi")}, i64(35000), i64(60000), false},
		{"estimated from dishes is marked", FactRow{GiaNguoiMin: i64(40000), GiaNguoiMax: i64(70000), GiaNguoiCoSo: str("uoc_tu_mon")},
			i64(40000), i64(70000), true},
		{"older band when already per person", FactRow{GiaMin: i64(50000), GiaMax: i64(150000), GiaDonVi: str("moi_ve")},
			i64(50000), i64(150000), false},
		{"per serving counts too", FactRow{GiaMin: i64(150000), GiaDonVi: str("moi_suat")}, i64(150000), nil, false},
		{"per dish is not per person", FactRow{GiaMin: i64(30000), GiaMax: i64(80000), GiaDonVi: str("moi_mon")}, nil, nil, false},
		{"per group is not per person", FactRow{GiaMin: i64(300000), GiaDonVi: str("moi_nhom")}, nil, nil, false},
		{"a missing bound stays missing", FactRow{GiaNguoiMin: i64(50000), GiaNguoiCoSo: str("review_web")}, i64(50000), nil, false},
		{"a free entry is a price", FactRow{GiaMin: i64(0), GiaMax: i64(0), GiaDonVi: str("moi_ve")}, i64(0), i64(0), false},
		{"inverted band is unknown", FactRow{GiaNguoiMin: i64(90000), GiaNguoiMax: i64(40000)}, nil, nil, false},
		{"nothing fed", FactRow{}, nil, nil, false},
	}
	eq := func(a, b *int64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
	for _, c := range cases {
		lo, hi, uoc := GiaMoiNguoi(c.row)
		if !eq(lo, c.lo) || !eq(hi, c.hi) || uoc != c.uoc {
			t.Errorf("%s: got (%v, %v, %v)", c.name, lo, hi, uoc)
		}
	}
}

func TestFactReject(t *testing.T) {
	ok := FactRow{PlaceID: "plc_abc", GioMoCua: []byte(`[]`), Menu: []byte(`[]`), HoatDong: []byte(`[]`)}
	if r := factReject(ok); r != "" {
		t.Fatalf("a clean row was refused: %s", r)
	}
	for _, c := range []struct {
		want string
		edit func(*FactRow)
	}{
		{RejectFactsPlaceID, func(r *FactRow) { r.PlaceID = "abc" }},
		{RejectFactsPlaceID, func(r *FactRow) { r.PlaceID = "plc_" }},
		{RejectFactsConHoatDong, func(r *FactRow) { r.ConHoatDong = str("chac_dong") }},
		{RejectFactsNotArray, func(r *FactRow) { r.Menu = []byte(`{}`) }},
		{RejectFactsNotArray, func(r *FactRow) { r.GioMoCua = []byte(`null`) }},
		{RejectFactsNegative, func(r *FactRow) { r.GiaNguoiMax = i64(-1) }},
	} {
		row := ok
		c.edit(&row)
		if got := factReject(row); got != c.want {
			t.Errorf("want %s, got %q", c.want, got)
		}
	}
}
