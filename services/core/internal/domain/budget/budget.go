// Package budget is the Go port of app/domain/budget.py: F34 group budget
// awareness for GET /contexts/{context_id}/budget.
//
// The money laws hold as in Python (ADR-0029 §2.5). A stored amount (an
// outing's budget per person) is money.VND. The candidate from the query is a
// *big.Int: routes/budget.py admits any ASCII digit string with no ceiling
// and the response echoes it, so the Go answer must carry every digit Python
// prints. A derived sum is a *big.Int and never narrowed: an outing's
// split total is a SQL SUM of confirmed allocations, and every figure built
// from it (spent, remaining, the historical average, the delta) stays exact.
// Every division is Python's floor division, done with big.Int.Div, whose
// Euclidean quotient is the floor for the positive divisors this code reaches;
// both operands are refused when negative before any division, so no
// negative dividend is reachable. The ten percent band is compared in scaled
// integers, never as a ratio.
//
// A malformed fact is refused with INVALID_BUDGET_INPUT and never coerced.
// Go's types already refuse what Python refuses for a bool, a float or a
// missing field; what remains is a negative number, a title that is blank
// under str.strip(), and a split total that is not an int (nil).
//
// testdata/python_*.json is rendered from the real module by
// scripts/render_domain_w4_goldens.py and replayed by oracle_test.go.
package budget

import (
	"math/big"

	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/domain/money"
)

// ComparisonTolerancePercent is COMPARISON_TOLERANCE_PERCENT.
const ComparisonTolerancePercent = 10

// percentScale is _PERCENT_SCALE.
const percentScale = 100

// CodeInvalidBudgetInput is the one code *BudgetError carries.
const CodeInvalidBudgetInput = "INVALID_BUDGET_INPUT"

// The verdicts of a comparison.
const (
	VerdictUsual   = "nhu-thuong"
	VerdictCheaper = "re-hon"
	VerdictDearer  = "cao-hon"
)

// BudgetError mirrors BudgetError: Error returns the code.
type BudgetError struct {
	Code string
}

func (e *BudgetError) Error() string { return e.Code }

func invalid() error { return &BudgetError{Code: CodeInvalidBudgetInput} }

// Outing is one outing dict the service builds from group_recap.
// SplitTotalVND nil stands where Python would be handed something that is
// not an int.
type Outing struct {
	OutingID           string
	Title              string
	Headcount          int64
	BudgetPerPersonVND money.VND
	SplitTotalVND      *big.Int
	InProgress         bool
}

// OutingView is one entry of in_progress.
type OutingView struct {
	OutingID              string
	Title                 string
	Headcount             int64
	BudgetPerPersonVND    money.VND
	SpentPerPersonVND     *big.Int
	RemainingPerPersonVND *big.Int
	OverBudget            bool
}

// Comparison is the comparison dict.
type Comparison struct {
	CandidatePerPersonVND *big.Int
	DeltaVND              *big.Int
	Verdict               string
}

// Budget is build_group_budget's dict. AvgPerPersonVND and Comparison nil are
// None.
type Budget struct {
	OutingCount       int
	ActiveMemberCount int64
	AvgPerPersonVND   *big.Int
	InProgress        []OutingView
	Comparison        *Comparison
}

// readOuting is _read_outing's checks, in its order: title, then headcount,
// budget and split total. in_progress is a bool by type.
func readOuting(raw Outing) (Outing, error) {
	title := contexts.Strip(raw.Title)
	if title == "" {
		return Outing{}, invalid()
	}
	if raw.Headcount < 0 || raw.BudgetPerPersonVND < 0 || raw.SplitTotalVND == nil || raw.SplitTotalVND.Sign() < 0 {
		return Outing{}, invalid()
	}
	raw.Title = title
	return raw, nil
}

// BuildGroupBudget is build_group_budget: the finished outings' average
// spend per person, each outing still in progress with its spend against its
// budget, and the candidate compared with the average. candidate nil is None;
// it is exact, of any size.
// Checks run in Python's order: the member count, the candidate, then each
// outing in turn.
func BuildGroupBudget(outings []Outing, activeMemberCount int64, candidate *big.Int) (Budget, error) {
	if activeMemberCount < 0 {
		return Budget{}, invalid()
	}
	if candidate != nil && candidate.Sign() < 0 {
		return Budget{}, invalid()
	}

	finishedTotal := new(big.Int)
	finishedHeadcount := new(big.Int)
	result := Budget{ActiveMemberCount: activeMemberCount, InProgress: []OutingView{}}
	for _, raw := range outings {
		outing, err := readOuting(raw)
		if err != nil {
			return Budget{}, err
		}
		if !outing.InProgress {
			result.OutingCount++
			finishedTotal.Add(finishedTotal, outing.SplitTotalVND)
			finishedHeadcount.Add(finishedHeadcount, big.NewInt(outing.Headcount))
			continue
		}
		spent := new(big.Int)
		if outing.Headcount != 0 {
			spent.Div(outing.SplitTotalVND, big.NewInt(outing.Headcount))
		}
		remaining := new(big.Int).Sub(big.NewInt(int64(outing.BudgetPerPersonVND)), spent)
		result.InProgress = append(result.InProgress, OutingView{
			OutingID:              outing.OutingID,
			Title:                 outing.Title,
			Headcount:             outing.Headcount,
			BudgetPerPersonVND:    outing.BudgetPerPersonVND,
			SpentPerPersonVND:     spent,
			RemainingPerPersonVND: remaining,
			OverBudget:            remaining.Sign() < 0,
		})
	}

	if finishedHeadcount.Sign() != 0 {
		result.AvgPerPersonVND = new(big.Int).Div(finishedTotal, finishedHeadcount)
	}
	if candidate != nil && result.AvgPerPersonVND != nil {
		result.Comparison = compare(candidate, result.AvgPerPersonVND)
	}
	return result, nil
}

// compare is _comparison: inside the band, inclusive, is usual; outside it
// the sign of the delta decides.
func compare(candidate, average *big.Int) *Comparison {
	delta := new(big.Int).Sub(candidate, average)
	tolerance := new(big.Int).Mul(average, big.NewInt(ComparisonTolerancePercent))
	scaled := new(big.Int).Mul(new(big.Int).Abs(delta), big.NewInt(percentScale))
	verdict := VerdictDearer
	switch {
	case scaled.Cmp(tolerance) <= 0:
		verdict = VerdictUsual
	case delta.Sign() < 0:
		verdict = VerdictCheaper
	}
	return &Comparison{CandidatePerPersonVND: new(big.Int).Set(candidate), DeltaVND: delta, Verdict: verdict}
}
