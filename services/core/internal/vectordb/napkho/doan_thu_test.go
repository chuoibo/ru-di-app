package napkho

import (
	"context"
	"testing"

	"mobile/services/core/internal/rag/nap"
)

// doanThu is nap.DoanQuan for a test whose texts fit one chunk.
func doanThu(t testing.TB, h nap.HoSoQuan, tt nap.ThuocTinh, chunker string) []nap.Hang {
	t.Helper()
	rows, err := nap.DoanQuan(context.Background(), h, tt, chunker, nap.ChiaNguyen{})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
