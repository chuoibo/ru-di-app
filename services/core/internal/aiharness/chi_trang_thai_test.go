package aiharness

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/llm"
)

// The group worker's Sink while a turn runs (slice 12): the statuses pass,
// the text does not -- a group's answer reaches its readers from the posted
// card, after its commit (contract §4.1) -- and nothing is paced, since
// nobody reads what is dropped. The result still carries the whole verified
// text, which the card is built from. Identity: the same turn on a plain Sink
// hears the paced deltas and takes the pacing's time.
func TestChiTrangThaiKhongChuKhongNhip(t *testing.T) {
	chu := strings.TrimSpace(strings.Repeat("Tối thứ 7 cả nhóm ra bờ hồ đi dạo rồi ghé ăn lẩu nhé. ", 2))
	chay := func(boc func(Sink) Sink) (*ghi, Result, time.Duration) {
		stub := llm.NewStub(ruThang(), dung(false, chu), kiemDatN(2))
		e, err := New(WithModel(stub), WithLogger(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))),
			WithRetryWait(func(int) time.Duration { return 0 }), WithNhipPhat(40*time.Millisecond), WithNguon(moiTheGioi(t).nguon()))
		if err != nil {
			t.Fatal(err)
		}
		g := &ghi{}
		start := time.Now()
		res, err := e.RunNhom(context.Background(), luotNhomCoBan(), boc(g))
		if err != nil {
			t.Fatal(err)
		}
		return g, res, time.Since(start)
	}
	g, res, took := chay(func(s Sink) Sink { return ChiTrangThai{Sink: s} })
	if !g.chiTrangThai() || len(g.status) == 0 {
		t.Fatalf("the held Sink heard %s", g.bytes())
	}
	if res.Text != chu {
		t.Fatalf("result text %q, want the verified answer", res.Text)
	}
	g2, res2, took2 := chay(func(s Sink) Sink { return s })
	if strings.Join(g2.delta, "") != chu || res2.Text != chu {
		t.Fatalf("identity: deltas %q", strings.Join(g2.delta, ""))
	}
	if took2 < 3*40*time.Millisecond || took > took2/2 {
		t.Fatalf("held turn %v, paced turn %v: the held one must skip the pacing", took, took2)
	}
}
