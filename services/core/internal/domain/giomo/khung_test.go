package giomo

import (
	"reflect"
	"testing"
)

func TestKhungSplitsAcrossTheWeekEnd(t *testing.T) {
	cases := []struct {
		tu, den int
		want    []Khoang
		sql     string
	}{
		{600, 720, []Khoang{{600, 720}}, "{[600,720)}"},
		{PhutTuan - 60, PhutTuan + 30, []Khoang{{0, 30}, {PhutTuan - 60, PhutTuan}}, "{[0,30),[10020,10080)}"},
		{PhutTuan - 60, PhutTuan, []Khoang{{PhutTuan - 60, PhutTuan}}, "{[10020,10080)}"},
		{0, PhutTuan, []Khoang{{0, PhutTuan}}, "{[0,10080)}"},
		{300, 300, nil, "{}"},
	}
	for _, c := range cases {
		if got := Khung(c.tu, c.den); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Khung(%d,%d) = %v, want %v", c.tu, c.den, got, c.want)
		}
		if got := KhungSQL(c.tu, c.den); got != c.sql {
			t.Errorf("KhungSQL(%d,%d) = %s, want %s", c.tu, c.den, got, c.sql)
		}
	}
}

// open_within: open at some minute of the window, not for all of it.
func TestMoTrongIsOverlap(t *testing.T) {
	dem, ok := Doc("18:00 – 01:00")
	if !ok {
		t.Fatal("not read")
	}
	sang, _ := Doc("07:00 - 11:00")
	day := 24 * 60
	cases := []struct {
		ten     string
		l       Lich
		tu, den int
		want    bool
	}{
		{"khung chạm đầu ca tối", dem, 17*60 + 30, 18*60 + 30, true},
		{"khung kết thúc đúng lúc mở thì chưa mở", dem, 17 * 60, 18 * 60, false},
		{"sau nửa đêm vẫn trong ca hôm trước", dem, day + 30, day + 45, true},
		{"đêm Chủ nhật sang sáng thứ Hai", dem, 6*day + 23*60, 7*day + 30, true},
		{"quán sáng không mở buổi tối", sang, 19 * 60, 22 * 60, false},
		{"khung rỗng", sang, 8 * 60, 8 * 60, false},
	}
	for _, c := range cases {
		if got := c.l.MoTrong(c.tu, c.den); got != c.want {
			t.Errorf("%s: MoTrong(%d,%d) = %v, want %v", c.ten, c.tu, c.den, got, c.want)
		}
	}
}
