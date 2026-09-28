package nhung

import (
	"context"
	"errors"
	"testing"
)

// One turn, one budget: calls made through a shared TheoLuot embedder with
// the turn's context take from the same counter the router uses, and the
// call past MaxEmbedCallsPerTurn never reaches the provider.
func TestTheoLuotDemMotNganSachChoCaLuot(t *testing.T) {
	inner := &demGoi{}
	e := TheoLuot{Inner: inner}
	d := MoiDemLuot()
	ctx := VoiDemLuot(context.Background(), d)
	if err := d.Giu(); err != nil { // the router's example choice
		t.Fatal(err)
	}
	if _, err := e.Nhung(ctx, []string{"quán lẩu"}, CauHoi); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Nhung(ctx, []string{"quán nướng"}, CauHoi); !errors.Is(err, ErrHetLuotNhung) {
		t.Fatalf("a call past the turn's budget: %v", err)
	}
	if inner.n != 1 || d.ConLai() != 0 {
		t.Fatalf("provider saw %d calls, %d left", inner.n, d.ConLai())
	}
	// Another turn has its own budget; no turn (offline) counts nothing.
	if _, err := e.Nhung(VoiDemLuot(context.Background(), MoiDemLuot()), []string{"a"}, CauHoi); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Nhung(context.Background(), []string{"a"}, CauHoi); err != nil {
		t.Fatal(err)
	}
	if _, err := (TheoLuot{Inner: lechSo{}}).Nhung(context.Background(), []string{"a", "b"}, CauHoi); err == nil {
		t.Fatal("a wrong vector count accepted")
	}
}
