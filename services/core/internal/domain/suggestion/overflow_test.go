package suggestion

import (
	"errors"
	"math"
	"testing"
)

// Each trip fits int64 and is a valid amount, but their sum does not: the
// summary refuses instead of wrapping to a negative total (audit 2026-10-05,
// PER-AI-MONEY-02).
func TestHistoryTotalThatWouldWrapIsRefused(t *testing.T) {
	trips := []Trip{{Title: "a", SplitTotalVND: math.MaxInt64, Headcount: 2}, {Title: "b", SplitTotalVND: 1, Headcount: 2}}
	_, err := SummariseHistory(trips, nil)
	var refusal *Error
	if !errors.As(err, &refusal) || refusal.Code != "suggestion_history_total_overflow" {
		t.Fatalf("got %v", err)
	}
	h, err := SummariseHistory([]Trip{{Title: "a", SplitTotalVND: math.MaxInt64 - 1, Headcount: 2}, {Title: "b", SplitTotalVND: 1, Headcount: 2}}, nil)
	if err != nil || h.SplitTotalVND != math.MaxInt64 || *h.AvgPerPersonVND != math.MaxInt64/4 {
		t.Fatalf("an exact sum at the limit: %+v %v", h, err)
	}
}
