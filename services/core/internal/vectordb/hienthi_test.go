package vectordb

import (
	"context"
	"reflect"
	"testing"
)

// The evidence fields ride with a hit through the fusion; a row without
// them, or with a dict that does not parse, gives none.
func TestHienThiQuaHopRRF(t *testing.T) {
	f := map[string]string{"ten": "Quán A", "gia": "35.000 đ/người"}
	legs := [][]Trung{{{ID: "a", DocID: "a", HienThi: f}, {ID: "b", DocID: "b"}}, {{ID: "b", DocID: "b"}, {ID: "a", DocID: "a", HienThi: f}}}
	out := HopRRF(legs, []float64{1, 1}, RRFK, 10)
	for _, h := range out {
		if h.ID == "a" && !reflect.DeepEqual(h.HienThi, f) {
			t.Fatalf("a lost its fields: %+v", h)
		}
		if h.ID == "b" && h.HienThi != nil {
			t.Fatalf("b gained fields: %+v", h)
		}
	}
	for _, raw := range []string{``, `{}`, `{"dau":"x"}`, `{"hien_thi":{}}`, `{"hien_thi":"x"}`, `not json`} {
		if got := hienThiTu([]byte(raw)); got != nil {
			t.Errorf("%q gave %v", raw, got)
		}
	}
	if got := hienThiTu([]byte(`{"dau":"x","hien_thi":{"ten":"A"}}`)); got["ten"] != "A" {
		t.Fatalf("parsed %v", got)
	}
}

func TestFakeTraHienThi(t *testing.T) {
	fk := MoiFake()
	fk.Them(HangDiaDiem{ID: "a", Text: "quán lẩu nấm", Dense: []float32{1}, MoRong: []byte(`{"hien_thi":{"ten":"Lẩu A"}}`)})
	hits, err := fk.Tim(context.Background(), YeuCauTim{Ten: "rd_places", Kho: KhoDiaDiem, K: 5, Thua: &ThuaTruyVan{Text: "lẩu"}})
	if err != nil || len(hits) != 1 || hits[0].HienThi["ten"] != "Lẩu A" {
		t.Fatalf("%+v %v", hits, err)
	}
}
