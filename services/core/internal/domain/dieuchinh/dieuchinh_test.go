package dieuchinh

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
)

// Hand-computed vectors (ADR-0056). A paid a 300.000đ dinner split three ways
// (v1); B paid a 60.000đ taxi split with A (v2). The batch therefore holds
// B→A 100.000, C→A 100.000 and A→B 30.000: pairs are never netted.

func alloc(pairs ...any) []ledger.Allocation {
	var out []ledger.Allocation
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, ledger.Allocation{ParticipantID: pairs[i].(string), AmountVND: money.VND(pairs[i+1].(int))})
	}
	return out
}

var batch = []Source{
	{ExpenseVersionID: "v1", PaidByID: "A", Allocations: alloc("A", 100000, "B", 100000, "C", 100000)},
	{ExpenseVersionID: "v2", PaidByID: "B", Allocations: alloc("A", 30000, "B", 30000)},
}

func amounts(edges []Edge) map[string]int64 {
	out := map[string]int64{}
	for _, e := range edges {
		out[e.SenderID+"→"+e.RecipientID] = e.AmountVND.Int64()
	}
	return out
}

func changes(cs []Change) map[string][2]int64 {
	out := map[string][2]int64{}
	for _, c := range cs {
		out[c.SenderID+"→"+c.RecipientID] = [2]int64{c.OldVND.Int64(), c.NewVND.Int64()}
	}
	return out
}

func TestTheBatchBeforeAnyAmendment(t *testing.T) {
	edges, err := Obligations(batch)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"B→A": 100000, "C→A": 100000, "A→B": 30000}
	if got := amounts(edges); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAmendments(t *testing.T) {
	old, _ := Obligations(batch)
	for _, c := range []struct {
		name        string
		replacement Source
		edges       map[string]int64
		changes     map[string][2]int64
		affected    []string
	}{
		{"B pays more, C less", Source{"v1b", "A", alloc("A", 100000, "B", 150000, "C", 50000)},
			map[string]int64{"B→A": 150000, "C→A": 50000, "A→B": 30000},
			map[string][2]int64{"B→A": {100000, 150000}, "C→A": {100000, 50000}}, []string{"A", "B", "C"}},
		{"C leaves the dinner", Source{"v1b", "A", alloc("A", 150000, "B", 150000)},
			map[string]int64{"B→A": 150000, "A→B": 30000},
			map[string][2]int64{"B→A": {100000, 150000}, "C→A": {100000, 0}}, []string{"A", "B", "C"}},
		{"D joins the dinner", Source{"v1b", "A", alloc("A", 75000, "B", 75000, "C", 75000, "D", 75000)},
			map[string]int64{"B→A": 75000, "C→A": 75000, "D→A": 75000, "A→B": 30000},
			map[string][2]int64{"B→A": {100000, 75000}, "C→A": {100000, 75000}, "D→A": {0, 75000}}, []string{"A", "B", "C", "D"}},
		{"only C's share moves to B", Source{"v1b", "A", alloc("A", 100000, "B", 200000)},
			map[string]int64{"B→A": 200000, "A→B": 30000},
			map[string][2]int64{"B→A": {100000, 200000}, "C→A": {100000, 0}}, []string{"A", "B", "C"}},
		{"the same allocations again", Source{"v1b", "A", alloc("A", 100000, "B", 100000, "C", 100000)},
			map[string]int64{"B→A": 100000, "C→A": 100000, "A→B": 30000}, map[string][2]int64{}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			next, err := Amend(batch, "v1", c.replacement)
			if err != nil {
				t.Fatal(err)
			}
			if got := amounts(next); !reflect.DeepEqual(got, c.edges) {
				t.Fatalf("edges %v want %v", got, c.edges)
			}
			diff := Diff(old, next)
			if got := changes(diff); !reflect.DeepEqual(got, c.changes) {
				t.Fatalf("changes %v want %v", got, c.changes)
			}
			if got := Affected(diff); !reflect.DeepEqual(got, c.affected) {
				t.Fatalf("affected %v want %v", got, c.affected)
			}
			// Integer đồng and no money created: every edge of the next
			// version is a positive integer.
			for _, e := range next {
				if e.AmountVND.Sign() <= 0 {
					t.Fatalf("edge %v", e)
				}
			}
		})
	}
}

func TestAmendRefusals(t *testing.T) {
	if _, err := Amend(batch, "v9", Source{"x", "A", alloc("A", 1)}); !errors.Is(err, ErrNotInBatch) {
		t.Fatalf("a version outside the batch: %v", err)
	}
	single := batch[:1]
	if _, err := Amend(single, "v1", Source{"v1b", "A", alloc("A", 300000)}); !errors.Is(err, ErrNoObligations) {
		t.Fatalf("an amendment that leaves nothing owed: %v", err)
	}
	if _, err := Amend(batch, "v1", Source{"v1b", "A", alloc("A", 100000, "B", -1)}); err == nil {
		t.Fatal("a negative allocation was accepted")
	}
}

func TestOutcome(t *testing.T) {
	affected := []string{"A", "B", "C"}
	for _, c := range []struct {
		decisions []Decision
		want      string
	}{
		{nil, "proposed"},
		{[]Decision{{"A", true}, {"B", true}}, "proposed"},
		{[]Decision{{"A", true}, {"B", true}, {"C", true}}, "accepted"},
		{[]Decision{{"A", true}, {"B", false}, {"C", true}}, "rejected"},
		{[]Decision{{"Z", false}, {"A", true}, {"B", true}, {"C", true}}, "accepted"},
	} {
		if got := Outcome(affected, c.decisions); got != c.want {
			t.Errorf("%v: %s want %s", c.decisions, got, c.want)
		}
	}
}

// Money already received is carried to the successor and never asked for
// again: below what was received is over_confirmed, not a refund demand.
func TestCarriedStatus(t *testing.T) {
	for _, c := range []struct {
		declared int64
		received []money.VND
		want     string
	}{
		{150000, []money.VND{100000}, "partially_confirmed"},
		{50000, []money.VND{100000}, "over_confirmed"},
		{100000, []money.VND{60000, 40000}, "confirmed"},
		{100000, nil, "outstanding"},
	} {
		got, err := CarriedStatus(money.VND(c.declared), c.received)
		if err != nil || got != c.want {
			t.Errorf("%d over %v: %s %v want %s", c.declared, c.received, got, err, c.want)
		}
	}
	_ = big.NewInt
}
