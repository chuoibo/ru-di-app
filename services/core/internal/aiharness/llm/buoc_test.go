package llm

import (
	"context"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// Worst case per path: no path's steps may add up past
// MaxModelCallsPerTurn, every path keeps its verifier and its router uncut,
// and the grader (the sufficiency judgement) is the first cut wherever it
// runs. Red if a constant or a table row grows a path past the ceiling.
func TestKeHoachTrongTran(t *testing.T) {
	if len(KeHoach) != 5 {
		t.Fatalf("%d paths", len(KeHoach))
	}
	for d, buocs := range KeHoach {
		if n := ToiDaDuong(d); n > MaxModelCallsPerTurn || n <= 0 {
			t.Errorf("%s: worst case %d calls, ceiling %d", d, n, MaxModelCallsPerTurn)
		}
		coKiem, coRouter := false, false
		for i, b := range buocs {
			if b.ToiDa < 1 {
				t.Errorf("%s: step %s at most %d", d, b.Loai, b.ToiDa)
			}
			if _, ok := mucNghi[b.Loai]; !ok {
				t.Errorf("%s: step %s has no thinking level", d, b.Loai)
			}
			switch b.Loai {
			case BuocKiem, BuocKiemLai, BuocRouter, BuocTraLoi, BuocAgentTraLoi:
				if b.Loai != BuocKiemLai && b.Cat != 0 {
					t.Errorf("%s: %s is cut (rank %d); a released text is never unverified", d, b.Loai, b.Cat)
				}
			case BuocCham:
				if b.Cat != 1 {
					t.Errorf("%s: the grader is cut at rank %d, not first", d, b.Cat)
				}
			}
			coKiem = coKiem || b.Loai == BuocKiem
			coRouter = coRouter || (b.Loai == BuocRouter && i == 0)
		}
		if !coKiem || !coRouter {
			t.Errorf("%s: verifier %v, router first %v", d, coKiem, coRouter)
		}
	}
	// The table follows the engine's own constants.
	for d, steps := range map[Duong]int{DuongTacTuNep: MaxStepsNep, DuongTacTuNhom: MaxStepsNhom} {
		n := 0
		for _, b := range KeHoach[d] {
			if b.Loai == BuocAgentKeHoach || b.Loai == BuocAgentTraLoi {
				n += b.ToiDa
			}
		}
		if n != steps {
			t.Errorf("%s: %d agent steps in the plan, %d in the engine", d, n, steps)
		}
	}
	if SauBuoc(DuongTruyHoi, BuocCham) != 5 || SauBuoc(DuongTruyHoi, BuocRouter) != ToiDaDuong(DuongTruyHoi) {
		t.Fatalf("SauBuoc: %d, %d", SauBuoc(DuongTruyHoi, BuocCham), SauBuoc(DuongTruyHoi, BuocRouter))
	}
}

// Every step has an explicit level and none carries a token budget.
func TestMucNghiTuongMinh(t *testing.T) {
	for b, l := range mucNghi {
		c := CauHinhNghi(b)
		if c.ThinkingBudget != nil || c.ThinkingLevel != l || l == "" || l == genai.ThinkingLevelUnspecified {
			t.Errorf("%s: %+v", b, c)
		}
	}
	if MucNghi(BuocAgentKeHoach) != genai.ThinkingLevelLow {
		t.Error("agent planning steps think at LOW")
	}
	for _, b := range []LoaiGoi{BuocRouter, BuocCham, BuocTraLoi, BuocKiem, BuocAgentTraLoi} {
		if MucNghi(b) != genai.ThinkingLevelMinimal {
			t.Errorf("%s thinks at %s", b, MucNghi(b))
		}
	}
}

// The counter sums every call's usage, the implicit cache's share included.
func TestDemCongToken(t *testing.T) {
	s := NewStub(
		Buoc{Text: "a", Usage: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 1000, CachedContentTokenCount: 768, CandidatesTokenCount: 20, ThoughtsTokenCount: 3}},
		Buoc{Text: "b", Usage: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 500, CachedContentTokenCount: 0, CandidatesTokenCount: 10}},
		Buoc{Text: "c"},
	)
	d := NewDem(s, 3, nil)
	for i := 0; i < 3; i++ {
		for _, err := range d.GenerateContent(context.Background(), &model.LLMRequest{Model: Model}, false) {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := d.Token(); got != (Token{In: 1500, Out: 30, Cache: 768, Thought: 3}) {
		t.Fatalf("%+v", got)
	}
}
