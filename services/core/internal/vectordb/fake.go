package vectordb

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/promptsafety"
)

// Fake is an in-memory TimKiem for unit tests of the retrieval adapter. It
// filters with LocCung.Dat (the Go reading of the same rule the Milvus
// expression states), ranks each leg Milvus would search (cosine; shared
// folded terms) and fuses them with the same weighted
// RRF (HopRRF). It is evidence about the adapter's orchestration only,
// never about Milvus: the live tier (-tags milvus) is.
type Fake struct {
	mu      sync.Mutex
	DiaDiem map[string]HangDiaDiem
	// Loi, when set, is what every Tim answers (a Milvus outage).
	Loi error
	// Da records every request.
	Da []YeuCauTim
}

// MoiFake is an empty fake.
func MoiFake() *Fake { return &Fake{DiaDiem: map[string]HangDiaDiem{}} }

var _ TimKiem = (*Fake)(nil)

// Them adds or replaces place rows.
func (f *Fake) Them(rows ...HangDiaDiem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range rows {
		f.DiaDiem[r.ID] = r
	}
}

// TuKhongDau are the terms FText's analyzer yields (standard tokenizer,
// lowercase, asciifolding: marks and đ folded).
func TuKhongDau(text string) []string {
	return strings.FieldsFunc(promptsafety.Fold(nhung.ChuanNFC(text)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

type fakeDiem struct {
	id string
	d  float64
}

// Tim filters, ranks each leg and fuses by weighted RRF.
func (f *Fake) Tim(_ context.Context, y YeuCauTim) ([]Trung, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Da = append(f.Da, y)
	if f.Loi != nil {
		return nil, f.Loi
	}
	if y.K <= 0 || y.K > truyhoi.MaxK {
		return nil, fmt.Errorf("vectordb: k %d", y.K)
	}
	if y.Kho != KhoDiaDiem {
		return nil, fmt.Errorf("vectordb: fake holds places only")
	}
	legs, err := y.Nhanhs()
	if err != nil {
		return nil, err
	}
	shared := func(q, d []string) float64 {
		set := map[string]bool{}
		for _, t := range q {
			set[t] = true
		}
		n := 0
		for _, t := range d {
			if set[t] {
				n++
			}
		}
		return float64(n)
	}
	var ranked [][]Trung
	var w []float64
	for _, l := range legs {
		var l2 []fakeDiem
		for id, r := range f.DiaDiem {
			if ok, _ := y.Loc.Dat(r.thuocTinh()); !ok {
				continue
			}
			var d float64
			switch l.Truong {
			case FDense:
				d = nhung.Cosine(y.Dense, r.Dense)
			case FSparse:
				d = shared(TuKhongDau(y.Thua.Text), TuKhongDau(r.Text))
			default:
				continue
			}
			if l.Truong != FDense && d == 0 {
				continue
			}
			l2 = append(l2, fakeDiem{id, d})
		}
		sort.Slice(l2, func(i, j int) bool {
			if l2[i].d != l2[j].d {
				return l2[i].d > l2[j].d
			}
			return l2[i].id < l2[j].id
		})
		if len(l2) > y.ungVien() {
			l2 = l2[:y.ungVien()]
		}
		leg := make([]Trung, len(l2))
		for i, s := range l2 {
			r := f.DiaDiem[s.id]
			leg[i] = Trung{ID: s.id, DocID: r.Doc(), PhienBan: r.PhienBan, HienThi: hienThiTu(r.MoRong)}
		}
		ranked = append(ranked, leg)
		w = append(w, l.TrongSo)
	}
	return HopRRF(ranked, w, RRFK, y.K), nil
}
