package dockhoan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

func TestDocHoiLaiMotLanKhiCauTraLoiBiCat(t *testing.T) {
	loop := `{"is_expense": true, "title": "tiền nước ` + strings.Repeat("nhé hihi ", 60)
	ok := `{"is_expense": true, "title": "tiền nước", "amount_text": "300k"}`
	stub := llm.NewStub(llm.Buoc{Text: loop}, llm.Buoc{Text: ok})
	raw, err := Doc(context.Background(), motluot.Moi(stub, 1).Luot(3), "Tao trả 300k tiền nước")
	if err != nil || raw["amount_text"] != "300k" || stub.SoGoi() != 2 {
		t.Fatalf("%v %v after %d calls", raw, err, stub.SoGoi())
	}
	// Twice unreadable is unreadable; a third call is never made.
	stub = llm.NewStub(llm.Buoc{Text: loop}, llm.Buoc{Text: loop}, llm.Buoc{Text: ok})
	if _, err := Doc(context.Background(), motluot.Moi(stub, 1).Luot(3), "x"); !errors.Is(err, motluot.ErrKhongDocDuoc) || stub.SoGoi() != 2 {
		t.Fatalf("%v after %d calls", err, stub.SoGoi())
	}
	// A provider error is returned as it came, without a second call.
	stub = llm.NewStub(llm.Buoc{Loi: errors.New("upstream down")}, llm.Buoc{Text: ok})
	if _, err := Doc(context.Background(), motluot.Moi(stub, 1).Luot(3), "x"); err == nil || stub.SoGoi() != 1 {
		t.Fatalf("%v after %d calls", err, stub.SoGoi())
	}
}
