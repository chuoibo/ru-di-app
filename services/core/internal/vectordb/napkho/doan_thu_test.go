package napkho

import (
	"testing"

	"mobile/services/core/internal/rag/nap"
)

// doanThu is nap.DoanQuan for a test: an error fails the test.
func doanThu(t testing.TB, h nap.HoSoQuan, tt nap.ThuocTinh, chunker string) []nap.Hang {
	t.Helper()
	rows, err := nap.DoanQuan(h, tt, chunker)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
