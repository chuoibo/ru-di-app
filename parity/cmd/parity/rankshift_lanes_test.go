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
