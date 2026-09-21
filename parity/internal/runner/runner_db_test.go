package runner

import (
	"testing"

	"mobile/parity/internal/compare"
	"mobile/parity/internal/dbsnap"
)

func inserted(texts ...string) *dbsnap.Change {
	change := &dbsnap.Change{Relations: []string{"contexts"}}
	if len(texts) == 0 {
		return change
	}
	rows := make([]dbsnap.Row, len(texts))
	for i, text := range texts {
		rows[i] = dbsnap.Row{Text: text}
	}
	change.Changed = []dbsnap.RelationChange{{Relation: "contexts", Keyed: true, Inserted: rows}}
	return change
}

func transcript(steps ...StepResult) *Run { return &Run{Steps: steps} }

var wire = compare.Exchange{Status: 201, Body: `{"id":"<uuid#1>"}`}

func TestAWriteMissingFromOneDatabaseIsADifferenceEvenWhenTheWireMatches(t *testing.T) {
	ref := transcript(StepResult{StepID: "create", Norm: wire, NormChange: inserted(`{"id":"<uuid#1>","display_name":"Nhóm"}`)})
	cand := transcript(StepResult{StepID: "create", Norm: wire, NormChange: inserted()})
	diffs := Diff(ref, cand)
	if len(diffs) != 1 || len(diffs[0].Differences) != 0 || len(diffs[0].Database) == 0 {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestIdenticalWritesAreEqual(t *testing.T) {
	row := `{"id":"<uuid#1>","display_name":"Nhóm"}`
	ref := transcript(StepResult{StepID: "create", Norm: wire, NormChange: inserted(row)})
	cand := transcript(StepResult{StepID: "create", Norm: wire, NormChange: inserted(row)})
	if diffs := Diff(ref, cand); len(diffs) != 0 {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestADatabaseLaneOnOneSideOnlyIsADifference(t *testing.T) {
	ref := transcript(StepResult{StepID: "create", Norm: wire, NormChange: inserted()})
	cand := transcript(StepResult{StepID: "create", Norm: wire})
	diffs := Diff(ref, cand)
	if len(diffs) != 1 || len(diffs[0].Database) != 1 || diffs[0].Database[0].Kind != DatabaseLaneMismatch {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestWithoutADatabaseLaneOnlyTheWireIsCompared(t *testing.T) {
	ref := transcript(StepResult{StepID: "read", Norm: wire})
	cand := transcript(StepResult{StepID: "read", Norm: wire})
	if diffs := Diff(ref, cand); len(diffs) != 0 {
		t.Fatalf("diffs = %+v", diffs)
	}
}
