//go:build milvus

package vectordb

import (
	"testing"
	"time"
)

// choThay waits until a vector search sees rows just written: count(*) at
// Strong sees a fresh upsert at once, but a vector search on the growing
// segment returned nothing for about 200 ms after it (measured on v3.0.2,
// three rounds: 0 hits on the first try, found on the second, 184–208 ms),
// whatever the consistency level. A test that asserts on a search right
// after a write waits here; the product does not need to (the index is a
// derived copy, a lag of a fraction of a second is a lag).
func choThay(t *testing.T, m *Milvus, name string, rows []HangDiaDiem) {
	t.Helper()
	ctx := ctxThu(t, 30*time.Second)
	probe := rows
	if len(probe) > 5 {
		probe = []HangDiaDiem{rows[0], rows[len(rows)/4], rows[len(rows)/2], rows[3*len(rows)/4], rows[len(rows)-1]}
	}
	for _, r := range probe {
		for i := 0; ; i++ {
			got, err := m.Tim(ctx, YeuCauTim{Ten: name, Kho: KhoDiaDiem, Dense: r.Dense, K: 50,
				Thua: &ThuaTruyVan{Loai: ThuaBM25, Text: r.Text}})
			if err != nil {
				t.Fatal(err)
			}
			seen := r.ThuocTinh.GoBo // a tombstoned row is filtered out by design
			for _, h := range got {
				if h.ID == r.ID {
					seen = true
				}
			}
			if seen {
				break
			}
			if i == 100 {
				t.Fatalf("%s never became searchable", r.ID)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
