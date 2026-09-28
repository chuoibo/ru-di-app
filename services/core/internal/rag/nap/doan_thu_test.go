package nap

import (
	"context"
	"testing"
)

// doanThu is DoanQuan for a test whose texts fit one chunk: no encoder, no
// context to carry, an error fails the test.
func doanThu(t testing.TB, h HoSoQuan, tt ThuocTinh, chunker string) []Hang {
	t.Helper()
	rows, err := DoanQuan(context.Background(), h, tt, chunker, ChiaNguyen{})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
