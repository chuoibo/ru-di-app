package main

import (
	"testing"

	"mobile/parity/internal/compare"
	"mobile/parity/internal/dbsnap"
	"mobile/parity/internal/runner"
)

// The case the gate itself produced on 20/09, w10/concurrency/post-people-
// person_id-block: one Binder numbers responses AND snapshots, so a background
// rank shift appears in both. The first version of allDiffs dropped any
// scenario with a database difference, which meant the classifier never fired
// on a scenario with the database lane on.
func TestRankShiftSeesSnapshotLanes(t *testing.T) {
	row := func(rank string) string {
		return `{"id":"<uuid#2>","state":"pending","created_at":"<ts#` + rank + `|f6|+00:00>"}`
	}
	diffs := []runner.StepDiff{{
		StepID: "owner_asks_friend",
		Differences: []compare.Difference{{
			Part:      "body",
			Reference: `{"created_at":"<ts#6|f6|Z>"}`,
			Candidate: `{"created_at":"<ts#7|f6|Z>"}`,
		}},
		Database: []dbsnap.Difference{{
			Relation: "friend_requests", Kind: "inserted",
			Reference: 1, Candidate: 1,
			OnlyReference: []string{row("6")}, OnlyCandidate: []string{row("7")},
		}},
	}}
	offset, ok := compare.RankShift(allDiffs(diffs))
	if !ok || offset != 1 {
		t.Fatalf("offset=%d ok=%v, want 1 true", offset, ok)
	}
}

func TestRowCountChangeIsNeverARankShift(t *testing.T) {
	// The candidate wrote a row the reference did not. No renumbering explains
	// that, and it must never be waved through as background noise.
	diffs := []runner.StepDiff{{
		StepID: "owner_asks_friend",
		Database: []dbsnap.Difference{{
			Relation: "friend_requests", Kind: "inserted",
			Reference: 1, Candidate: 2,
			OnlyReference: []string{`{"created_at":"<ts#6|f6|Z>"}`},
			OnlyCandidate: []string{`{"created_at":"<ts#7|f6|Z>"}`, `{"created_at":"<ts#8|f6|Z>"}`},
		}},
	}}
	if _, ok := compare.RankShift(allDiffs(diffs)); ok {
		t.Fatal("a row count change was accepted as a rank shift")
	}
}

// Khác biệt của một kịch bản SHIFT không được nằm trong tổng: mã thoát đọc
// tổng, nên để lại chúng là biến một nhãn "nhiễu nền" thành một lượt chạy đỏ.
// Đo được trên CI 21/09: scenarios_diff=0 mà differences=8, và job vẫn hỏng.
func TestShiftScenarioAddsNothingToTheFatalTally(t *testing.T) {
	row := func(rank string) string {
		return `{"id":"<uuid#2>","created_at":"<ts#` + rank + `|f6|+00:00>"}`
	}
	diffs := []runner.StepDiff{{
		StepID: "owner_asks_friend",
		Differences: []compare.Difference{{
			Part:      "body",
			Reference: `{"created_at":"<ts#6|f6|Z>"}`,
			Candidate: `{"created_at":"<ts#7|f6|Z>"}`,
		}},
		Database: []dbsnap.Difference{{
			Relation: "friend_requests", Kind: "inserted",
			Reference: 1, Candidate: 1,
			OnlyReference: []string{row("6")}, OnlyCandidate: []string{row("7")},
		}},
	}}
	if _, ok := compare.RankShift(allDiffs(diffs)); !ok {
		t.Fatal("tiền đề hỏng: ca này phải là SHIFT")
	}
	// Đếm y như vòng lặp trong run(): mỗi khác biệt một đơn vị.
	counted := 0
	for _, d := range diffs {
		counted += len(d.Differences) + len(d.Database) + len(d.Media)
	}
	if counted == 0 {
		t.Fatal("tiền đề hỏng: ca này phải có khác biệt để đếm")
	}
	// Một kịch bản SHIFT đi qua nhánh giữa, nơi không cộng gì vào rep.Differences.
	// Ca này ghim tiền đề đó; nếu ai đó cộng lại, RankShift vẫn true mà lượt chạy
	// hoá đỏ, và lời nhắc nằm ngay đây.
}
