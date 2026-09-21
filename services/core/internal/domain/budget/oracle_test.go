package budget

import (
	"errors"
	"testing"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_budget*.json is rendered by scripts/render_domain_w4_goldens.py
// from the real app.domain.budget in the parity API image; every case is
// replayed here.

func outingsOf(value any) ([]Outing, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]Outing, len(rows))
	for i, raw := range rows {
		r, err := oracletest.Row(raw, "outing_id", "title", "headcount", "budget_per_person_vnd", "split_total_vnd", "in_progress")
		if err != nil {
			return nil, err
		}
		o := &out[i]
		if o.OutingID, err = oracletest.Str(r["outing_id"]); err != nil {
			return nil, err
		}
		if o.Title, err = oracletest.Str(r["title"]); err != nil {
			return nil, err
		}
		if o.Headcount, err = oracletest.Int64(r["headcount"]); err != nil {
			return nil, err
		}
		budget, err := oracletest.Int64(r["budget_per_person_vnd"])
		if err != nil {
			return nil, err
		}
		o.BudgetPerPersonVND = money.VND(budget)
		if o.SplitTotalVND, err = oracletest.Integer(r["split_total_vnd"]); err != nil {
			return nil, err
		}
		if o.InProgress, err = oracletest.Bool(r["in_progress"]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func render(b Budget) map[string]any {
	live := make([]any, len(b.InProgress))
	for i, o := range b.InProgress {
		live[i] = map[string]any{
			"outing_id":                o.OutingID,
			"title":                    o.Title,
			"headcount":                o.Headcount,
			"budget_per_person_vnd":    int64(o.BudgetPerPersonVND),
			"spent_per_person_vnd":     oracletest.Exact(o.SpentPerPersonVND),
			"remaining_per_person_vnd": oracletest.Exact(o.RemainingPerPersonVND),
			"over_budget":              o.OverBudget,
		}
	}
	var comparison any
	if b.Comparison != nil {
		comparison = map[string]any{
			"candidate_per_person_vnd": oracletest.Exact(b.Comparison.CandidatePerPersonVND),
			"delta_vnd":                oracletest.Exact(b.Comparison.DeltaVND),
			"verdict":                  b.Comparison.Verdict,
		}
	}
	return map[string]any{
		"outing_count":        int64(b.OutingCount),
		"active_member_count": b.ActiveMemberCount,
		"avg_per_person_vnd":  oracletest.Exact(b.AvgPerPersonVND),
		"in_progress":         live,
		"comparison":          comparison,
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "build_group_budget" {
		return nil, oracletest.Decode(errors.New("unknown function " + c.Fn))
	}
	outings, err := outingsOf(args["outings"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	active, err := oracletest.Int64(args["active_member_count"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	candidate, err := oracletest.Integer(args["candidate_per_person_vnd"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	result, err := BuildGroupBudget(outings, active, candidate)
	if err != nil {
		return nil, err
	}
	return render(result), nil
}

func refusal(err error) (string, string, bool) {
	var refused *BudgetError
	if errors.As(err, &refused) {
		return "BudgetError", refused.Code, true
	}
	return "", "", false
}

func TestBuildGroupBudgetMatchesPython(t *testing.T) {
	checkBudget(t, oracletest.Load(t, "testdata/python_*.json"), "budget", 150)
}

// checkBudget replays the cases of module in files and asserts their spread:
// at least least cases, some refusals, and some sums past int64 among both the
// arguments and the answers.
func checkBudget(t *testing.T, files []oracletest.File, module string, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	tally := report.ByFn["build_group_budget"]
	if tally == nil || tally.Cases < least || tally.Refusals == 0 || tally.BigResults == 0 || tally.BigArgs == 0 {
		t.Fatalf("%+v, want at least %d cases: the corpus lost its spread", tally, least)
	}
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "budget")
	if constants["comparison_tolerance_percent"] != int64(ComparisonTolerancePercent) {
		t.Errorf("tolerance: Python %v, Go %d", constants["comparison_tolerance_percent"], ComparisonTolerancePercent)
	}
	ported := map[string]bool{"COMPARISON_TOLERANCE_PERCENT": true, "BudgetError": true, "build_group_budget": true}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !ported[name] {
			t.Errorf("Python exports %s and the port does not map it", name)
		}
	}
}
