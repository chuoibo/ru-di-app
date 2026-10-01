package nap

import (
	"testing"
	"time"
)

// The online budget is a sliding hour; a failing door backs off 30 s,
// doubling to 10 min, and a success starts it over.
func TestHanMuc(t *testing.T) {
	h := NewHanMuc(300)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if h.Con(t0) != 300 {
		t.Fatal("a fresh budget is not whole")
	}
	h.Tieu(t0, 250)
	h.Tieu(t0.Add(30*time.Minute), 40)
	if got := h.Con(t0.Add(59 * time.Minute)); got != 10 {
		t.Fatalf("within the hour: %d left", got)
	}
	if got := h.Con(t0.Add(61 * time.Minute)); got != 260 {
		t.Fatalf("after the first spend aged out: %d left", got)
	}
	h.Tieu(t0.Add(62*time.Minute), 400)
	if h.Con(t0.Add(62*time.Minute)) != 0 {
		t.Fatal("an overspent budget is not empty")
	}
	var got []time.Duration
	for i := 0; i < 7; i++ {
		got = append(got, h.Loi())
	}
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 10 * time.Minute, 10 * time.Minute}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backoff %d: %v, want %v", i, got[i], want[i])
		}
	}
	h.Tieu(t0, 0)
	if h.Loi() != 30*time.Second {
		t.Fatal("a success did not reset the backoff")
	}
	var none *HanMuc
	if none.Con(t0) < 1<<30 || none.Loi() != 30*time.Second {
		t.Fatal("no budget is not unlimited")
	}
	none.Tieu(t0, 5)
	if choLoi(1) != 15*time.Second || choLoi(2) != time.Minute {
		t.Fatal("document backoff")
	}
}

// The fingerprint moves with every stored attribute and nothing else.
func TestDauThuocTinh(t *testing.T) {
	base := Hang{ChunkID: "a", DocID: "a", Text: "x", DiemDen: "d", DiUng: []string{"tom"}, DiUngRo: true,
		AnKieng: []string{"chay"}, GiaMin: 1, GiaMax: 2, GiaRo: true, MoO: []int16{1}, GioRo: true, DanhMuc: []string{"cafe"},
		HienThi: map[string]string{"ten": "A", "dia_chi": "1 Đường Hoa"}}
	d0 := DauThuocTinh(base)
	same := base
	same.Text, same.ContentHash, same.Dense, same.KhiChat = "khác", "h", []float32{1}, []string{"yen_tinh"}
	if DauThuocTinh(same) != d0 {
		t.Fatal("a field the index does not filter on moved the fingerprint")
	}
	for name, edit := range map[string]func(*Hang){
		"destination": func(h *Hang) { h.DiemDen = "e" },
		"allergen":    func(h *Hang) { h.DiUng = []string{"tom", "sua"} },
		"certainty":   func(h *Hang) { h.DiUngRo = false },
		"diet":        func(h *Hang) { h.AnKieng = nil },
		"price":       func(h *Hang) { h.GiaMin = 3 },
		"price max":   func(h *Hang) { h.GiaMax = 3 },
		"price known": func(h *Hang) { h.GiaRo = false },
		"slots":       func(h *Hang) { h.MoO = []int16{2} },
		"hours known": func(h *Hang) { h.GioRo = false },
		"category":    func(h *Hang) { h.DanhMuc = []string{"cafe", "an_vat"} },
		"evidence":    func(h *Hang) { h.HienThi = map[string]string{"dia_chi": "2 Đường Khác"} },
	} {
		h := base
		edit(&h)
		if DauThuocTinh(h) == d0 {
			t.Errorf("%s did not move the fingerprint", name)
		}
	}
}
