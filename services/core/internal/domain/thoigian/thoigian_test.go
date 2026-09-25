package thoigian

import (
	"strings"
	"testing"
	"time"
)

// The fixed instant every case is asked at: Friday 25/09/2026 14:05 in
// Vietnam, 07:05 UTC. Written as UTC on purpose: a resolver that forgot the
// wall clock would read the UTC date and hour.
var luc = time.Date(2026, 9, 25, 7, 5, 0, 0, time.UTC)

func ict(y, m, d, h, mi int) time.Time {
	return time.Date(y, time.Month(m), d, h, mi, 0, 0, time.FixedZone("ICT", 7*3600)).UTC()
}

func mot(t *testing.T, text string, at time.Time) Moc {
	t.Helper()
	got := Giai(text, at)
	if len(got) != 1 {
		t.Fatalf("%q: %d mốc, muốn 1: %+v", text, len(got), got)
	}
	return got[0]
}

func TestDongBayGio(t *testing.T) {
	want := "Bây giờ: Thứ Sáu 25/09/2026 14:05 (Asia/Ho_Chi_Minh, 2026-09-25T14:05:00+07:00)"
	if got := DongBayGio(luc); got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestNgayTuongDoi(t *testing.T) {
	for _, c := range []struct {
		text, cum, mota string
		moHo            bool
	}{
		{"hôm nay đi đâu", "hôm nay", "Thứ Sáu 25/09/2026", false},
		{"Tối nay rảnh không", "Tối nay", "Thứ Sáu 25/09/2026, buổi tối (18:00–23:00)", false},
		{"mai đi cà phê nhé", "mai", "Thứ Bảy 26/09/2026", false},
		{"ngày mai mấy giờ", "ngày mai", "Thứ Bảy 26/09/2026", false},
		{"sáng mai ăn phở", "sáng mai", "Thứ Bảy 26/09/2026, buổi sáng (06:00–11:00)", false},
		{"mốt đi được không", "mốt", "Chủ Nhật 27/09/2026", false},
		{"ngày kia nhé", "ngày kia", "Chủ Nhật 27/09/2026", false},
		{"thứ 6 này đi đâu", "thứ 6 này", "Thứ Sáu 25/09/2026", false},
		{"thứ sáu tới", "thứ sáu tới", "Thứ Sáu 02/10/2026", false},
		{"thứ 7 tới đi Vũng Tàu", "thứ 7 tới", "Thứ Bảy 26/09/2026 (chưa chắc: cũng có thể là Thứ Bảy 03/10/2026)", true},
		{"thứ hai tuần sau", "thứ hai tuần sau", "Thứ Hai 28/09/2026", false},
		{"thứ 3 sau", "thứ 3 sau", "Thứ Ba 29/09/2026", false},
		{"thứ 2 này", "thứ 2 này", "Thứ Hai 28/09/2026 (chưa chắc: cũng có thể là Thứ Hai 21/09/2026)", true},
		{"chủ nhật này", "chủ nhật này", "Chủ Nhật 27/09/2026", false},
		{"CN đi bơi", "CN", "Chủ Nhật 27/09/2026", false},
		{"t7 đi", "t7", "Thứ Bảy 26/09/2026", false},
		{"thứ 6 đi", "thứ 6", "Thứ Sáu 25/09/2026 (chưa chắc: cũng có thể là Thứ Sáu 02/10/2026)", true},
		{"thứ tư", "thứ tư", "Thứ Tư 30/09/2026", false},
		{"cuối tuần đi đâu", "cuối tuần", "Thứ Bảy 26/09/2026 đến Chủ Nhật 27/09/2026", false},
		{"cuối tuần sau", "cuối tuần sau", "Thứ Bảy 03/10/2026 đến Chủ Nhật 04/10/2026", false},
		{"cuối tuần tới", "cuối tuần tới", "Thứ Bảy 03/10/2026 đến Chủ Nhật 04/10/2026 (chưa chắc: cũng có thể là Thứ Bảy 26/09/2026)", true},
		{"tuần sau họp", "tuần sau", "Thứ Hai 28/09/2026 đến Chủ Nhật 04/10/2026", false},
		{"tuần tới", "tuần tới", "Thứ Hai 28/09/2026 đến Chủ Nhật 04/10/2026", false},
		{"ngày 3/10 đi", "ngày 3/10", "Thứ Bảy 03/10/2026", false},
		{"25/12/2026 nhé", "25/12/2026", "Thứ Sáu 25/12/2026", false},
		{"20/9 đi rồi", "20/9", "Chủ Nhật 20/09/2026 (chưa chắc: cũng có thể là Thứ Hai 20/09/2027)", true},
	} {
		m := mot(t, c.text, luc)
		if m.Cum != c.cum || m.MoTa() != c.mota || m.MoHo != c.moHo {
			t.Errorf("%q:\n got cụm=%q «%s» mơ hồ=%v\nwant cụm=%q «%s» mơ hồ=%v", c.text, m.Cum, m.MoTa(), m.MoHo, c.cum, c.mota, c.moHo)
		}
	}
}

func TestGio(t *testing.T) {
	for _, c := range []struct{ text, mota string }{
		{"7h tối", "19:00"},
		{"7h30 tối nhé", "19:30"},
		{"19h30", "19:30"},
		{"19h", "19:00"},
		{"19:30 nha", "19:30"},
		{"8 giờ tối", "20:00"},
		{"7 giờ rưỡi sáng", "07:30"},
		{"3h chiều", "15:00"},
		{"1h trưa", "13:00"},
		{"12h đêm", "00:00"},
		{"tối 8h", "20:00"},
		{"7h", "07:00 (chưa chắc: cũng có thể là 19:00)"},
		{"7h rưỡi", "07:30 (chưa chắc: cũng có thể là 19:30)"},
	} {
		m := mot(t, c.text, luc)
		if m.Loai != LoaiGio || m.MoTa() != c.mota {
			t.Errorf("%q: %s «%s», muốn «%s»", c.text, m.Loai, m.MoTa(), c.mota)
		}
	}
	// A day with a part of the day lends it to the time right after.
	got := Giai("tối nay 7h đi ăn", luc)
	if len(got) != 2 || got[1].MoTa() != "19:00" {
		t.Fatalf("tối nay 7h: %+v", got)
	}
}

// «tối nay» at 23:30 is still tonight, today's date; the UTC date and the
// Vietnamese date agree here, the part of the day is what must not slip.
func TestToiNayLuc2330(t *testing.T) {
	m := mot(t, "tối nay còn quán nào mở", ict(2026, 9, 25, 23, 30))
	if m.Tu != (Ngay{2026, 9, 25}) || m.Buoi != BuoiToi || m.MoHo {
		t.Fatalf("tối nay lúc 23:30: %+v", m)
	}
}

// «mai» at 00:30 on Saturday: the calendar says Sunday, a person who has not
// slept yet may mean Saturday. The date comes from Vietnam's clock: at 00:30
// ICT it is still Friday in UTC, and a UTC resolver would answer Saturday.
func TestMaiLuc0030(t *testing.T) {
	m := mot(t, "mai đi đâu", ict(2026, 9, 26, 0, 30))
	if m.Tu != (Ngay{2026, 9, 27}) || !m.MoHo || m.Khac == nil || *m.Khac != (Ngay{2026, 9, 26}) {
		t.Fatalf("mai lúc 00:30: %+v (khác %+v)", m, m.Khac)
	}
	if !strings.Contains(m.MoTa(), "chưa chắc") {
		t.Fatalf("mô tả không nói chưa chắc: %s", m.MoTa())
	}
	// Past 05:00 the same word reads one way only.
	if m := mot(t, "mai đi đâu", ict(2026, 9, 26, 7, 0)); m.MoHo || m.Tu != (Ngay{2026, 9, 27}) {
		t.Fatalf("mai lúc 07:00: %+v", m)
	}
}

// Words that look like dates and are not.
func TestKhongPhaiNgay(t *testing.T) {
	for _, text := range []string{
		"mai mốt rảnh thì đi",
		"đi với Mai nhé",
		"hoa mai nở rồi",
		"ngân sách 50k một người",
		"nhóm 2 người",
		"32/13 là gì",
		"31/9",
		"ngày nay ai cũng bận",
		"thứ nhất là rẻ",
		"7 tiếng đồng hồ",
	} {
		if got := Giai(text, luc); len(got) != 0 {
			t.Errorf("%q: thấy %+v", text, got)
		}
	}
}

func TestNhieuMoc(t *testing.T) {
	got := Giai("thứ 7 này 7h tối hay chủ nhật tuần sau 9h sáng?", luc)
	var parts []string
	for _, m := range got {
		parts = append(parts, m.Cum+"="+m.MoTa())
	}
	want := "thứ 7 này=Thứ Bảy 26/09/2026|7h tối=19:00|chủ nhật tuần sau=Chủ Nhật 04/10/2026|9h sáng=09:00"
	if strings.Join(parts, "|") != want {
		t.Fatalf("\n got %s\nwant %s", strings.Join(parts, "|"), want)
	}
}
