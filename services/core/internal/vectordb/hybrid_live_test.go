//go:build milvus

package vectordb

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/domain/tuvung"
)

// khaiNiem is a test embedder standing in for a semantic model on a tiny
// synonym fixture: each word of a concept pair maps to the concept's
// direction, every other word (the names) to nothing. It is the dense leg's
// strength in miniature (synonyms meet) and its weakness (names vanish);
// BM25 has the opposite pair. Test data, not a rule of the product.
var khaiNiem = map[string]int{"lau": 1, "hotpot": 1, "nuong": 2, "grill": 2, "kem": 3, "icecream": 3, "tra": 4, "tea": 4}

func nhungKhaiNiem(text string) []float32 {
	v := make([]float32, nhung.Dims)
	for _, s := range tuvung.AmTiet(text) {
		if c, ok := khaiNiem[s]; ok {
			v[c*7] += 1
		}
	}
	if n, err := nhung.ChuanHoa(v); err == nil {
		return n
	}
	v[0] = 1
	return v
}

// Hybrid beats dense-only on the synonym fixture: queries «<synonym B>
// <name>» whose document says «<synonym A> <name>», and pure synonym
// queries whose documents all use the other word. Dense-only cannot tell
// three same-concept documents apart by name (at most one of the three name
// queries of a concept can come first), BM25-only cannot reach a synonym;
// the RRF of both gets both.
func TestHybridHonDenseChiMot(t *testing.T) {
	m := ketThu(t)
	ctx := ctxThu(t, 3*time.Minute)
	name, err := m.TaoPhienBan(ctx, KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	type khai struct{ docWord, queryWord string }
	concepts := []khai{{"hotpot", "lẩu"}, {"grill", "nướng"}, {"icecream", "kem"}, {"tea", "trà"}}
	names := [][]string{{"Minh Châu", "Bà Tư", "Gia Hân"}, {"Thu Hà", "Bảo Long", "Kim Anh"}, {"Hải Đăng", "Ngọc Lan", "Phúc An"}, {"Tâm Như", "Quốc Việt", "Hồng Nhung"}}
	var rows []HangDiaDiem
	type q struct {
		text string
		want []string
		top  int
	}
	var queries []q
	for ci, c := range concepts {
		var ids []string
		for ni, n := range names[ci] {
			id := fmt.Sprintf("c%dn%d", ci, ni)
			ids = append(ids, id)
			text := c.docWord + " " + n
			rows = append(rows, HangDiaDiem{ID: id, Dense: nhungKhaiNiem(text), Text: text, PhienBan: 1,
				ThuocTinh: ThuocTinh{DiemDen: "da-lat", GiaMinVND: GiaKhongRo}})
			queries = append(queries, q{c.queryWord + " " + n, []string{id}, 1})
		}
		queries = append(queries, q{c.queryWord, ids, 3})
	}
	for i, filler := range []string{"quán ven hồ yên tĩnh", "sân vườn rộng", "view đồi thông", "nhạc sống cuối tuần", "chỗ gửi xe rộng", "bàn ngoài trời"} {
		rows = append(rows, HangDiaDiem{ID: fmt.Sprintf("x%d", i), Dense: nhungKhaiNiem(filler), Text: filler, PhienBan: 1,
			ThuocTinh: ThuocTinh{DiemDen: "da-lat", GiaMinVND: GiaKhongRo}})
	}
	if err := m.GhiDiaDiem(ctx, name, rows); err != nil {
		t.Fatal(err)
	}
	choThay(t, m, name, rows)
	score := func(dense, sparse bool) float64 {
		total := 0.0
		for _, qq := range queries {
			y := YeuCauTim{Ten: name, Kho: KhoDiaDiem, K: qq.top}
			if dense {
				y.Dense = nhungKhaiNiem(qq.text)
			}
			if sparse {
				y.Thua = &ThuaTruyVan{Text: qq.text}
			}
			got, err := m.Tim(ctx, y)
			if err != nil {
				t.Fatal(err)
			}
			hit := 0
			for _, h := range got {
				if slices.Contains(qq.want, h.ID) {
					hit++
				}
			}
			total += float64(hit) / float64(len(qq.want))
		}
		return total / float64(len(queries))
	}
	hybrid, dense, bm25 := score(true, true), score(true, false), score(false, true)
	t.Logf("%d queries: hybrid %.3f, dense-only %.3f, BM25-only %.3f", len(queries), hybrid, dense, bm25)
	if !(hybrid > dense) || hybrid < 0.95 || dense > 0.5 {
		t.Fatalf("hybrid %.3f does not beat dense-only %.3f as the fixture requires", hybrid, dense)
	}
	if hybrid < bm25 {
		t.Fatalf("hybrid %.3f below BM25-only %.3f", hybrid, bm25)
	}
}
