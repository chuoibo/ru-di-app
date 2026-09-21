package compare

import "testing"

func d(part, ref, cand string) Difference {
	return Difference{Part: part, Reference: ref, Candidate: cand}
}

func TestRankShiftAcceptsOneConstantOffset(t *testing.T) {
	got, ok := RankShift([]Difference{
		d("body", `{"a":"<ts#3|f6|Z>","b":"<ts#4|f6|Z>"}`, `{"a":"<ts#2|f6|Z>","b":"<ts#3|f6|Z>"}`),
		d("body", `{"c":"<ts#5|f6|Z>"}`, `{"c":"<ts#4|f6|Z>"}`),
	})
	if !ok || got != -1 {
		t.Fatalf("offset=%d ok=%v, want -1 true", got, ok)
	}
}

func TestRankShiftRejectsMixedOffsets(t *testing.T) {
	// Two different offsets is not one background collision; it is a real
	// difference in how the two sides ordered their moments.
	if _, ok := RankShift([]Difference{
		d("body", `{"a":"<ts#3|f6|Z>","b":"<ts#9|f6|Z>"}`, `{"a":"<ts#2|f6|Z>","b":"<ts#5|f6|Z>"}`),
	}); ok {
		t.Fatal("mixed offsets accepted")
	}
}

func TestRankShiftRejectsShapeChange(t *testing.T) {
	// Same rank, different zone spelling: never noise.
	if _, ok := RankShift([]Difference{
		d("body", `{"a":"<ts#3|f6|Z>"}`, `{"a":"<ts#2|f6|+00:00>"}`),
	}); ok {
		t.Fatal("shape change accepted")
	}
}

func TestRankShiftRejectsAnythingElseDiffering(t *testing.T) {
	// A status difference alongside a clean shift must still be a difference.
	if _, ok := RankShift([]Difference{
		d("body", `{"a":"<ts#3|f6|Z>"}`, `{"a":"<ts#2|f6|Z>"}`),
		d("status", "201", "200"),
	}); ok {
		t.Fatal("non-rank difference accepted")
	}
}

func TestRankShiftRejectsClockCollapse(t *testing.T) {
	// The bug TestClockSourceChangeCollapsesRanks guards: the port stamped two
	// columns from one clock read, so two ranks became one. The ranks do not
	// move by a constant, and this must never be waved through as noise.
	if _, ok := RankShift([]Difference{
		d("body", `{"created_at":"<ts#1|f6|Z>","accepted_at":"<ts#2|f6|Z>"}`,
			`{"created_at":"<ts#1|f6|Z>","accepted_at":"<ts#1|f6|Z>"}`),
	}); ok {
		t.Fatal("clock collapse accepted as rank shift")
	}
}

func TestRankShiftRejectsEmptyAndNoRanks(t *testing.T) {
	if _, ok := RankShift(nil); ok {
		t.Fatal("empty accepted")
	}
	if _, ok := RankShift([]Difference{d("status", "500", "502")}); ok {
		t.Fatal("difference with no ranks accepted")
	}
}
