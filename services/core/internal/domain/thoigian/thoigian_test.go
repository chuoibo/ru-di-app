package thoigian

import (
	"testing"
	"time"
)

// The fixed instant every case is asked at: Friday 25/09/2026 14:05 in
// Vietnam, 07:05 UTC. Written as UTC on purpose: arithmetic that forgot the
// wall clock would read the UTC date and hour.
var luc = time.Date(2026, 9, 25, 7, 5, 0, 0, time.UTC)

func TestDongBayGio(t *testing.T) {
	want := "Bây giờ: Thứ Sáu 25/09/2026 14:05 (Asia/Ho_Chi_Minh, 2026-09-25T14:05:00+07:00)"
	if got := DongBayGio(luc); got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// 23:30 UTC on the 25th is already the 26th in Vietnam.
	if got := DongBayGio(time.Date(2026, 9, 25, 17, 30, 0, 0, time.UTC)); got != "Bây giờ: Thứ Bảy 26/09/2026 00:30 (Asia/Ho_Chi_Minh, 2026-09-26T00:30:00+07:00)" {
		t.Fatalf("qua nửa đêm: %s", got)
	}
}

func TestLich(t *testing.T) {
	d := Ngay{2026, 9, 25}
	for n, want := range map[int]string{0: "Thứ Sáu 25/09/2026", 1: "Thứ Bảy 26/09/2026", 6: "Thứ Năm 01/10/2026", -25: "Thứ Hai 31/08/2026", 98: "Thứ Sáu 01/01/2027"} {
		if got := d.Cong(n).String(); got != want {
			t.Errorf("Cong(%d) = %s, muốn %s", n, got, want)
		}
	}
	if (Ngay{2026, 9, 27}).Thu() != 6 || (Ngay{2026, 9, 28}).Thu() != 0 {
		t.Fatal("thứ trong tuần lệch")
	}
}
