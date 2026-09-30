package nap

import (
	"testing"
)

// doanThu is DoanQuan for a test: an error fails the test.
func doanThu(t testing.TB, h HoSoQuan, tt ThuocTinh, chunker string) []Hang {
	t.Helper()
	rows, err := DoanQuan(h, tt, chunker)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
