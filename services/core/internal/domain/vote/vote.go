// Package vote ports app.domain.vote (F17): counting ballots without choosing
// a winner when the group is tied.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image.
package vote

import (
	"cmp"
	"slices"
)

// Codes VoteError carries.
const (
	CodeNoOptions         = "NO_OPTIONS"
	CodeDuplicatePosition = "DUPLICATE_POSITION"
	CodeDuplicateBallot   = "DUPLICATE_BALLOT"
	CodeUnknownOption     = "UNKNOWN_OPTION"
)

// VoteError is Python's VoteError: `str(exc)` is the code.
type VoteError struct {
	Code string
}

func (e *VoteError) Error() string { return e.Code }

func refuse(code string) error { return &VoteError{Code: code} }

// Option is one option dict: its id and position.
type Option struct {
	ID       string
	Position int
}

// Ballot is one ballot dict.
type Ballot struct {
	VoterID  string
	OptionID string
}

// Count is one entry of the counts dict.
type Count struct {
	OptionID string
	Count    int
}

// Result is the dict tally returns. Counts keeps the dict's order: each id
// once, where it first appears in position order. DecidedOptionID is nil
// unless exactly one option leads.
type Result struct {
	TotalBallots     int
	Counts           []Count
	LeadingOptionIDs []string
	IsTie            bool
	DecidedOptionID  *string
}

// CountOf is `result["counts"][option_id]`; ok is false for an id the counts
// do not hold.
func (r Result) CountOf(optionID string) (int, bool) {
	for _, count := range r.Counts {
		if count.OptionID == optionID {
			return count.Count, true
		}
	}
	return 0, false
}

// Tally is tally. Options are ordered by a stable sort on position. Two options
// sharing an id but not a position are not refused: the id is counted once and
// is listed among the leaders once per option, so a single leading id can be a
// tie with itself and decide nothing.
func Tally(options []Option, ballots []Ballot) (Result, error) {
	if len(options) == 0 {
		return Result{}, refuse(CodeNoOptions)
	}
	ordered := slices.Clone(options)
	slices.SortStableFunc(ordered, func(a, b Option) int { return cmp.Compare(a.Position, b.Position) })
	positions := map[int]bool{}
	for _, option := range ordered {
		if positions[option.Position] {
			return Result{}, refuse(CodeDuplicatePosition)
		}
		positions[option.Position] = true
	}

	counts := map[string]int{}
	var keys []string
	for _, option := range ordered {
		if _, seen := counts[option.ID]; !seen {
			counts[option.ID] = 0
			keys = append(keys, option.ID)
		}
	}
	voters := map[string]bool{}
	for _, ballot := range ballots {
		if voters[ballot.VoterID] {
			return Result{}, refuse(CodeDuplicateBallot)
		}
		voters[ballot.VoterID] = true
		if _, known := counts[ballot.OptionID]; !known {
			return Result{}, refuse(CodeUnknownOption)
		}
		counts[ballot.OptionID]++
	}

	leading := []string{}
	if len(ballots) > 0 {
		highest := 0
		for i, key := range keys {
			if i == 0 || counts[key] > highest {
				highest = counts[key]
			}
		}
		for _, option := range ordered {
			if counts[option.ID] == highest {
				leading = append(leading, option.ID)
			}
		}
	}
	result := Result{
		TotalBallots:     len(ballots),
		Counts:           make([]Count, len(keys)),
		LeadingOptionIDs: leading,
		IsTie:            len(leading) > 1,
	}
	for i, key := range keys {
		result.Counts[i] = Count{OptionID: key, Count: counts[key]}
	}
	if len(leading) == 1 {
		decided := leading[0]
		result.DecidedOptionID = &decided
	}
	return result, nil
}
