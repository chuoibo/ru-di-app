// Package dieuchinh is the pure arithmetic of an amendment to a collection
// batch that was already frozen or published (ADR-0056): the obligations the
// next batch version holds once one expense's allocations are replaced, which
// pair edges changed, who must accept, and how receipts already confirmed on
// a superseded obligation count toward its successor.
//
// Money stays integer đồng throughout and the allocations of the replacement
// are the allocator's (Σ = the expense total), checked by the caller exactly
// as a confirmation is; this package never invents an amount.
package dieuchinh

import (
	"errors"
	"math/big"
	"slices"

	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
)

// Source is one confirmed expense version a batch was frozen from: who paid,
// and every participant's allocation.
type Source struct {
	ExpenseVersionID string
	PaidByID         string
	Allocations      []ledger.Allocation
}

// Edge is one obligation of a batch version: what sender owes recipient.
type Edge struct {
	SenderID    string
	RecipientID string
	AmountVND   *big.Int
	// SourceExpenseVersionIDs are the versions behind the edge, in the order
	// the merge met them.
	SourceExpenseVersionIDs []string
}

// Change is one pair whose amount differs between two batch versions. A zero
// on either side is an edge that appears or disappears.
type Change struct {
	SenderID    string
	RecipientID string
	OldVND      *big.Int
	NewVND      *big.Int
}

// ErrNotInBatch is a replacement for a version the batch was not frozen from.
var ErrNotInBatch = errors.New("dieuchinh: the amended version is not a source of this batch")

// ErrNoObligations is an amendment after which nobody owes anybody: the batch
// would hold nothing, which is a cancellation, not an amendment.
var ErrNoObligations = errors.New("dieuchinh: the amended batch owes nothing")

// Obligations is FreezeBatch's arithmetic over sources: each version's
// allocations become edges to its payer, merged per pair, in source order.
func Obligations(sources []Source) ([]Edge, error) {
	var raw []ledger.Obligation
	for _, s := range sources {
		payer := s.PaidByID
		found, err := ledger.ObligationsFromAllocations(s.Allocations, &payer, s.ExpenseVersionID)
		if err != nil {
			return nil, err
		}
		raw = append(raw, found...)
	}
	merged, err := ledger.MergeObligations(raw)
	if err != nil {
		return nil, err
	}
	out := make([]Edge, len(merged))
	for i, m := range merged {
		out[i] = Edge{SenderID: m.SenderID, RecipientID: m.RecipientID, AmountVND: new(big.Int).Set(m.AmountVND),
			SourceExpenseVersionIDs: slices.Clone(m.SourceExpenseVersionIDs)}
	}
	return out, nil
}

// Amend is the next batch version's obligations: the batch's sources with the
// version being corrected replaced by its replacement, in the same place.
func Amend(sources []Source, replacedVersionID string, replacement Source) ([]Edge, error) {
	next := make([]Source, 0, len(sources))
	found := false
	for _, s := range sources {
		if s.ExpenseVersionID == replacedVersionID {
			next = append(next, replacement)
			found = true
			continue
		}
		next = append(next, s)
	}
	if !found {
		return nil, ErrNotInBatch
	}
	edges, err := Obligations(next)
	if err != nil {
		return nil, err
	}
	if len(edges) == 0 {
		return nil, ErrNoObligations
	}
	return edges, nil
}

// Diff is every pair whose amount differs from old to new, ordered by sender
// then recipient. An unchanged pair is not a change and needs nobody's yes.
func Diff(old, next []Edge) []Change {
	type key struct{ s, r string }
	amounts := map[key][2]*big.Int{}
	var keys []key
	add := func(edges []Edge, side int) {
		for _, e := range edges {
			k := key{e.SenderID, e.RecipientID}
			v, seen := amounts[k]
			if !seen {
				keys = append(keys, k)
				v = [2]*big.Int{new(big.Int), new(big.Int)}
			}
			v[side] = new(big.Int).Add(v[side], e.AmountVND)
			amounts[k] = v
		}
	}
	add(old, 0)
	add(next, 1)
	slices.SortFunc(keys, func(a, b key) int {
		if a.s != b.s {
			if a.s < b.s {
				return -1
			}
			return 1
		}
		if a.r < b.r {
			return -1
		}
		if a.r > b.r {
			return 1
		}
		return 0
	})
	var out []Change
	for _, k := range keys {
		v := amounts[k]
		if v[0].Cmp(v[1]) != 0 {
			out = append(out, Change{SenderID: k.s, RecipientID: k.r, OldVND: v[0], NewVND: v[1]})
		}
	}
	return out
}

// Affected is everyone on either end of a change, sorted, each once: the
// people who must accept before the amendment applies (spec invariant 5).
func Affected(changes []Change) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range changes {
		for _, p := range []string{c.SenderID, c.RecipientID} {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	slices.Sort(out)
	return out
}

// Decision is one affected person's answer.
type Decision struct {
	PersonID string
	Accept   bool
}

// Outcome is where an amendment stands given its affected people and the
// answers so far: "rejected" at the first no, "accepted" once every affected
// person said yes, else "proposed". Answers from people not affected count
// for nothing.
func Outcome(affected []string, decisions []Decision) string {
	yes := map[string]bool{}
	for _, d := range decisions {
		if !slices.Contains(affected, d.PersonID) {
			continue
		}
		if !d.Accept {
			return "rejected"
		}
		yes[d.PersonID] = true
	}
	for _, p := range affected {
		if !yes[p] {
			return "proposed"
		}
	}
	return "accepted"
}

// CarriedStatus is an obligation's status counting the receipts confirmed on
// every obligation it succeeds: money already received is never asked for
// again (spec L505), and more received than now owed is over_confirmed.
func CarriedStatus(declared money.VND, receiptsOnChain []money.VND) (string, error) {
	return ledger.ObligationStatus(declared, receiptsOnChain)
}
