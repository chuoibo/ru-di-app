// Package thoigian is calendar arithmetic on Vietnam's wall clock: the
// civil date of an instant, adding days, the day of the week, and the one
// «now» line a model is told. Pure: an instant in, dates and a line out. It
// never reads a clock of its own; the caller passes the instant the question
// was stored with, so a job retried an hour later still has the «now» it
// was asked at.
//
// It reads no words. The keyword reader that used to resolve «tối nay»,
// «mai», «thứ 6 tới» from the question was removed by the owner's rule of
// 2026-09-25: the router (aiharness/hieu) resolves relative dates to ISO
// values from this «now» line and a calendar laid out with these functions,
// and Go checks their form and lays them on the calendar.
package thoigian

import (
	"fmt"
	"time"

	"mobile/services/core/internal/domain/pairpaper"
)

// Ngay is a civil date on Vietnam's wall clock.
type Ngay struct {
	Nam, Thang, Ngay int
}

// Now returns the wall-clock reading of luc in Vietnam.
func Now(luc time.Time) time.Time { return pairpaper.Local(luc) }

func ngayCua(t time.Time) Ngay { return Ngay{t.Year(), int(t.Month()), t.Day()} }

// Cong adds n days.
func (d Ngay) Cong(n int) Ngay {
	return ngayCua(time.Date(d.Nam, time.Month(d.Thang), d.Ngay+n, 12, 0, 0, 0, time.UTC))
}

// Thu is the day of the week, Monday = 0 … Sunday = 6.
func (d Ngay) Thu() int {
	return (int(time.Date(d.Nam, time.Month(d.Thang), d.Ngay, 12, 0, 0, 0, time.UTC).Weekday()) + 6) % 7
}

var tenThu = [7]string{"Thứ Hai", "Thứ Ba", "Thứ Tư", "Thứ Năm", "Thứ Sáu", "Thứ Bảy", "Chủ Nhật"}

// String is «Thứ Sáu 25/09/2026».
func (d Ngay) String() string {
	return fmt.Sprintf("%s %02d/%02d/%04d", tenThu[d.Thu()], d.Ngay, d.Thang, d.Nam)
}

// DongBayGio is the one line that tells a model what «now» is, in both a form
// it reads well and the RFC 3339 form an evaluator can hold it to:
// «Bây giờ: Thứ Sáu 25/09/2026 14:05 (Asia/Ho_Chi_Minh, 2026-09-25T14:05:00+07:00)».
func DongBayGio(luc time.Time) string {
	here := Now(luc)
	return fmt.Sprintf("Bây giờ: %s %02d:%02d (Asia/Ho_Chi_Minh, %s)", ngayCua(here), here.Hour(), here.Minute(), here.Format(time.RFC3339))
}
