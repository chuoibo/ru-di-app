// Package ledger is the Go port of the part of app/domain/ledger.py that
// GET /contexts/{context_id}/balances reaches, and of the block of
// ApiService.get_context_balances that runs it (ContextBalances).
//
// The three money laws hold here exactly as in Python (ADR-0029 §2.5):
//
//  1. Amounts are whole đồng. Nothing in this package divides, rounds or
//     touches a float.
//  2. Nothing is lost between obligations, balances and transfers: the
//     balances net to zero and the transfers clear exactly the balances.
//  3. Balances are recomputed from the ledger rows on every call; no function
//     here accepts or returns a cached balance.
//
// ADR-0029 §2.4 makes Python the reference byte for byte, including the order
// in which refusals are found. testdata/python_*.json is rendered from the
// real module (and, for ContextBalances, from the real service method over a
// stub repository) by scripts/render_domain_w3_goldens.py; oracle_test.go
// replays every case and reads the hand-computed settlement corpus in
// services/api/tests/domain/golden_settlement in place.
//
// # Stored amounts and derived sums
//
// A stored amount (one confirmed allocation row, one obligation built from
// it) is money.VND, int64, as its BIGINT column is. A derived sum is a
// *big.Int: a merged pair total, a pair's confirmed receipts (a SQL SUM), a
// netted balance, a proposed transfer. Python's int never overflows, and these
// sums leave int64 over HTTP: POST /obligations/{id}/confirm-receipt takes any
// positive BIGINT, so two confirmations on one obligation already sum past
// 2**63 - 1, and 9,223,373 confirmed allocations of the allocator's 10**12
// ceiling put one balance past it. Go answers what Python answers in both
// cases; a caller writes a *big.Int with pyjson.NewBigInt. No *big.Int this
// package returns shares memory with an argument.
//
// A nil *big.Int stands where Python would be handed something that is not an
// int, and is refused with AMOUNT_NOT_INTEGER exactly where Python refuses it.
//
// # Where Go and Python differ
//
// SettlementPlan refuses an exact partition over maxExactPeople people with
// *RangeError, where Python would exhaust memory; the service always passes
// DefaultExactLimit (15). ContextBalances refuses a person id that is not a
// uuid the same way, where Python's uuid.UUID raises; the repository only
// reads uuid columns.
//
// confirmed_total, obligation_status and settlement_suggestions serve the batch
// and obligation routes, not W3, and are not ported here.
package ledger

import (
	"math/big"
	"slices"
	"strings"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/pyuuid"
)

// Codes carried by *LedgerError, identical to LedgerError.code.
const (
	CodeAmountNotInteger       = "AMOUNT_NOT_INTEGER"
	CodeNegativeAmount         = "NEGATIVE_AMOUNT"
	CodeNonPositiveAmount      = "NON_POSITIVE_AMOUNT"
	CodeNoAdvancer             = "NO_ADVANCER"
	CodeSelfObligation         = "SELF_OBLIGATION"
	CodeBalancesDoNotNetToZero = "BALANCES_DO_NOT_NET_TO_ZERO"
)

// DefaultExactLimit is settlement_plan's exact_limit default: above this many
// people with a non-zero balance the plan is greedy and not proven minimal.
const DefaultExactLimit = 15

// maxExactPeople bounds the exact partition's 2**n tables. Python has no such
// bound and would run out of memory well before it; see the package comment.
const maxExactPeople = 20

// TransferKind marks every proposed transfer as a draft, never an obligation.
const TransferKind = "offset_proposal_draft"

// ConflictStatus and ConflictDetail are what get_context_balances answers
// when a *LedgerError comes back: ApiProblem(409, exc.code, ConflictDetail).
// The problem code is the LedgerError code itself, upper case as it is.
const (
	ConflictStatus = 409
	ConflictDetail = "Confirmed ledger events cannot be balanced"
)

// LedgerError mirrors LedgerError: Error returns the code, as str(exc) does.
type LedgerError struct {
	Code string
}

func (e *LedgerError) Error() string { return e.Code }

// RangeError is Go-only: an input Python would take but this port refuses.
// See the package comment for the two cases. The service answers it as a crash.
type RangeError struct {
	Reason string
}

func (e *RangeError) Error() string { return "ledger: " + e.Reason }

func refuse(code string) error { return &LedgerError{Code: code} }

// RequireVND is require_vnd for a stored amount: NEGATIVE_AMOUNT below zero,
// NON_POSITIVE_AMOUNT for zero when positive is set.
func RequireVND(value money.VND, positive bool) (money.VND, error) {
	switch money.Violation(value, false, positive) {
	case money.Negative:
		return 0, refuse(CodeNegativeAmount)
	case money.NonPositive:
		return 0, refuse(CodeNonPositiveAmount)
	}
	return value, nil
}

// requireSum is require_vnd for a derived sum; nil is not an int.
func requireSum(value *big.Int, positive bool) error {
	switch {
	case value == nil:
		return refuse(CodeAmountNotInteger)
	case value.Sign() < 0:
		return refuse(CodeNegativeAmount)
	case positive && value.Sign() == 0:
		return refuse(CodeNonPositiveAmount)
	}
	return nil
}

// Allocation is one entry of a confirmed allocation map, participant -> đồng.
type Allocation struct {
	ParticipantID string
	AmountVND     money.VND
}

// Obligation is one directed edge from obligations_from_allocations.
type Obligation struct {
	SenderID               string
	RecipientID            string
	AmountVND              money.VND
	SourceExpenseVersionID string
}

// MergedObligation is one edge of merge_obligations: the pair's exact total
// and every source version, sorted and without repeats.
type MergedObligation struct {
	SenderID                string
	RecipientID             string
	AmountVND               *big.Int
	SourceExpenseVersionIDs []string
}

// Pair keys a confirmed receipt: what sender paid recipient.
type Pair struct {
	SenderID    string
	RecipientID string
}

// Balance is one netted position: positive means the group owes this person.
type Balance struct {
	PersonID string
	NetVND   *big.Int
}

// Transfer is one proposed transfer of a settlement plan.
type Transfer struct {
	Kind        string
	SenderID    string
	RecipientID string
	AmountVND   *big.Int
}

// Plan is settlement_plan's dict.
type Plan struct {
	Transfers     []Transfer
	TransferCount int
	ProvenMinimal bool
	PersonCount   int
}

// asDict gives a list of allocations the semantics of the Python dict the
// service builds from allocation rows: one entry per participant, at the
// position of its first row, holding the amount of its last row.
func asDict(allocations []Allocation) []Allocation {
	index := make(map[string]int, len(allocations))
	out := make([]Allocation, 0, len(allocations))
	for _, allocation := range allocations {
		if at, seen := index[allocation.ParticipantID]; seen {
			out[at].AmountVND = allocation.AmountVND
			continue
		}
		index[allocation.ParticipantID] = len(out)
		out = append(out, allocation)
	}
	return out
}

// ObligationsFromAllocations is obligations_from_allocations. advancerID nil
// is None and refused with NO_ADVANCER before any amount is read; every
// amount, the advancer's included, is validated before any edge is built.
func ObligationsFromAllocations(allocations []Allocation, advancerID *string, expenseVersionID string) ([]Obligation, error) {
	if advancerID == nil {
		return nil, refuse(CodeNoAdvancer)
	}
	entries := asDict(allocations)
	for _, entry := range entries {
		if _, err := RequireVND(entry.AmountVND, false); err != nil {
			return nil, err
		}
	}
	obligations := []Obligation{}
	for _, entry := range entries {
		if entry.ParticipantID == *advancerID || entry.AmountVND == 0 {
			continue
		}
		obligations = append(obligations, Obligation{
			SenderID:               entry.ParticipantID,
			RecipientID:            *advancerID,
			AmountVND:              entry.AmountVND,
			SourceExpenseVersionID: expenseVersionID,
		})
	}
	return obligations, nil
}

// comparePairs orders pairs as Python's (sender.encode(), recipient.encode()).
func comparePairs(a, b Pair) int {
	if c := strings.Compare(a.SenderID, b.SenderID); c != 0 {
		return c
	}
	return strings.Compare(a.RecipientID, b.RecipientID)
}

// MergeObligations is merge_obligations: obligations sharing a (sender,
// recipient) pair summed, never across pairs, ordered by the UTF-8 bytes of
// sender then recipient.
func MergeObligations(obligations []Obligation) ([]MergedObligation, error) {
	type slot struct {
		total   *big.Int
		sources []string
	}
	slots := map[Pair]*slot{}
	var pairs []Pair
	for _, obligation := range obligations {
		if obligation.SenderID == obligation.RecipientID {
			return nil, refuse(CodeSelfObligation)
		}
		amount, err := RequireVND(obligation.AmountVND, true)
		if err != nil {
			return nil, err
		}
		pair := Pair{SenderID: obligation.SenderID, RecipientID: obligation.RecipientID}
		current := slots[pair]
		if current == nil {
			current = &slot{total: new(big.Int)}
			slots[pair] = current
			pairs = append(pairs, pair)
		}
		current.total.Add(current.total, big.NewInt(int64(amount)))
		current.sources = append(current.sources, obligation.SourceExpenseVersionID)
	}
	slices.SortFunc(pairs, comparePairs)
	merged := make([]MergedObligation, 0, len(pairs))
	for _, pair := range pairs {
		current := slots[pair]
		sources := slices.Clone(current.sources)
		slices.Sort(sources)
		merged = append(merged, MergedObligation{
			SenderID:                pair.SenderID,
			RecipientID:             pair.RecipientID,
			AmountVND:               current.total,
			SourceExpenseVersionIDs: slices.Compact(sources),
		})
	}
	return merged, nil
}

// GroupBalances is group_balances: each pair's obligations summed, the pair's
// confirmed receipt subtracted once, a pair already cleared dropped, and the
// rest netted per person, ordered by person id. Display only: the result must
// never be turned back into obligations. A receipt is read only for a pair
// somebody owes, in the order the pairs were first seen; an absent pair is 0.
func GroupBalances(obligations []MergedObligation, receipts map[Pair]*big.Int) ([]Balance, error) {
	owed := map[Pair]*big.Int{}
	var pairs []Pair
	for _, obligation := range obligations {
		if obligation.SenderID == obligation.RecipientID {
			return nil, refuse(CodeSelfObligation)
		}
		if err := requireSum(obligation.AmountVND, true); err != nil {
			return nil, err
		}
		pair := Pair{SenderID: obligation.SenderID, RecipientID: obligation.RecipientID}
		total := owed[pair]
		if total == nil {
			total = new(big.Int)
			owed[pair] = total
			pairs = append(pairs, pair)
		}
		total.Add(total, obligation.AmountVND)
	}
	positions := map[string]*big.Int{}
	position := func(person string) *big.Int {
		current := positions[person]
		if current == nil {
			current = new(big.Int)
			positions[person] = current
		}
		return current
	}
	for _, pair := range pairs {
		receipt, found := receipts[pair]
		if !found {
			receipt = new(big.Int)
		}
		if err := requireSum(receipt, false); err != nil {
			return nil, err
		}
		remaining := new(big.Int).Sub(owed[pair], receipt)
		if remaining.Sign() <= 0 {
			continue
		}
		sender := position(pair.SenderID)
		sender.Sub(sender, remaining)
		recipient := position(pair.RecipientID)
		recipient.Add(recipient, remaining)
	}
	people := make([]string, 0, len(positions))
	for person, amount := range positions {
		if amount.Sign() != 0 {
			people = append(people, person)
		}
	}
	slices.Sort(people)
	balances := make([]Balance, len(people))
	for i, person := range people {
		balances[i] = Balance{PersonID: person, NetVND: positions[person]}
	}
	return balances, nil
}

// entry is one person's balance inside the settlement algorithm.
type entry struct {
	person string
	amount *big.Int
}

// SettlementPlan is settlement_plan: AMOUNT_NOT_INTEGER for a nil balance;
// BALANCES_DO_NOT_NET_TO_ZERO unless the balances sum to zero; no transfers
// when nobody is out; above exactLimit people the greedy plan, flagged not
// proven minimal; otherwise greedy inside each group of a maximum zero-sum
// partition, which is minimal. The order of balances does not matter, so a
// map carries them.
func SettlementPlan(balances map[string]*big.Int, exactLimit int) (Plan, error) {
	entries := make([]entry, 0, len(balances))
	for person, amount := range balances {
		entries = append(entries, entry{person: person, amount: amount})
	}
	return settle(entries, exactLimit)
}

func settle(entries []entry, exactLimit int) (Plan, error) {
	for _, e := range entries {
		if e.amount == nil {
			return Plan{}, refuse(CodeAmountNotInteger)
		}
	}
	total := new(big.Int)
	for _, e := range entries {
		total.Add(total, e.amount)
	}
	if total.Sign() != 0 {
		return Plan{}, refuse(CodeBalancesDoNotNetToZero)
	}
	var nonZero []entry
	for _, e := range entries {
		if e.amount.Sign() != 0 {
			nonZero = append(nonZero, e)
		}
	}
	count := len(nonZero)
	switch {
	case count == 0:
		return Plan{Transfers: []Transfer{}, ProvenMinimal: true}, nil
	case count > exactLimit:
		transfers := greedyTransfers(nonZero)
		return Plan{Transfers: transfers, TransferCount: len(transfers), PersonCount: count}, nil
	case count > maxExactPeople:
		return Plan{}, &RangeError{Reason: "an exact settlement over more than 20 people"}
	}
	transfers := []Transfer{}
	for _, group := range maximumZeroSumPartition(nonZero) {
		transfers = append(transfers, greedyTransfers(group)...)
	}
	return Plan{Transfers: transfers, TransferCount: len(transfers), ProvenMinimal: true, PersonCount: count}, nil
}

// greedyTransfers is _greedy_settlement_transfers: the largest debtor pays the
// largest creditor, ties broken by person id, until one side is cleared.
func greedyTransfers(entries []entry) []Transfer {
	type side struct {
		person string
		left   *big.Int
	}
	var debtors, creditors []side
	for _, e := range entries {
		switch e.amount.Sign() {
		case -1:
			debtors = append(debtors, side{person: e.person, left: new(big.Int).Neg(e.amount)})
		case 1:
			creditors = append(creditors, side{person: e.person, left: new(big.Int).Set(e.amount)})
		}
	}
	largestFirst := func(a, b side) int {
		if c := b.left.Cmp(a.left); c != 0 {
			return c
		}
		return strings.Compare(a.person, b.person)
	}
	slices.SortFunc(debtors, largestFirst)
	slices.SortFunc(creditors, largestFirst)
	var transfers []Transfer
	d, c := 0, 0
	for d < len(debtors) && c < len(creditors) {
		smaller := debtors[d].left
		if creditors[c].left.Cmp(smaller) < 0 {
			smaller = creditors[c].left
		}
		amount := new(big.Int).Set(smaller)
		transfers = append(transfers, Transfer{
			Kind:        TransferKind,
			SenderID:    debtors[d].person,
			RecipientID: creditors[c].person,
			AmountVND:   amount,
		})
		debtors[d].left.Sub(debtors[d].left, amount)
		creditors[c].left.Sub(creditors[c].left, amount)
		if debtors[d].left.Sign() == 0 {
			d++
		}
		if creditors[c].left.Sign() == 0 {
			c++
		}
	}
	return transfers
}

// maximumZeroSumPartition is _maximum_zero_sum_partition: the people split
// into the largest number of disjoint zero-sum groups, by the same bitmask
// dynamic program, visiting submasks in the same order so that among several
// maximum partitions the same one is chosen. The balances must net to zero.
func maximumZeroSumPartition(entries []entry) [][]entry {
	people := slices.Clone(entries)
	slices.SortFunc(people, func(a, b entry) int { return strings.Compare(a.person, b.person) })
	stateCount := 1 << len(people)

	sums := make([]big.Int, stateCount)
	for mask := 1; mask < stateCount; mask++ {
		lowest := mask & -mask
		index := 0
		for 1<<index != lowest {
			index++
		}
		sums[mask].Add(&sums[mask^lowest], people[index].amount)
	}

	const unreachable = -1
	counts := make([]int, stateCount)
	chosen := make([]int, stateCount)
	for mask := 1; mask < stateCount; mask++ {
		counts[mask] = unreachable
	}
	for mask := 1; mask < stateCount; mask++ {
		if sums[mask].Sign() != 0 {
			continue
		}
		lowest := mask & -mask
		optional := mask ^ lowest
		for submask := optional; ; submask = (submask - 1) & optional {
			group := submask | lowest
			remainder := mask ^ group
			if sums[group].Sign() == 0 && counts[remainder] != unreachable && counts[remainder]+1 > counts[mask] {
				counts[mask] = counts[remainder] + 1
				chosen[mask] = group
			}
			if submask == 0 {
				break
			}
		}
	}

	var groups [][]entry
	for remaining := stateCount - 1; remaining != 0; {
		group := chosen[remaining]
		if group == 0 {
			break // unreachable when the balances net to zero
		}
		var members []entry
		for index, person := range people {
			if group&(1<<index) != 0 {
				members = append(members, person)
			}
		}
		groups = append(groups, members)
		remaining ^= group
	}
	return groups
}

// ConfirmedExpense is what the repository's load_batch_inputs(context, None)
// hands the balance read for one expense: its latest confirmed version, who
// paid, and every allocation row in the order the rows were read.
type ConfirmedExpense struct {
	VersionID   string
	PaidByID    string
	Allocations []Allocation
}

// Sheet is ContextBalancesResponse.
type Sheet struct {
	Balances      []Balance
	Transfers     []Transfer
	ProvenMinimal bool
	TransferCount int
}

// ContextBalances is get_context_balances between its repository reads and
// its response: every expense's allocations turned into obligations, merged,
// netted against the confirmed receipts, and planned with DefaultExactLimit.
// receipts holds load_confirmed_receipts: each pair's SUM of confirmed receipt
// amounts, exact. Balances come back ordered by the bytes of the person's
// uuid. Ids must be the canonical uuid strings the repository reads.
//
// A *LedgerError is the 409 the service answers (ConflictStatus, err.Code,
// ConflictDetail). A *RangeError is Go-only; see the package comment.
func ContextBalances(expenses []ConfirmedExpense, receipts map[Pair]*big.Int) (Sheet, error) {
	var obligations []Obligation
	for _, expense := range expenses {
		payer := expense.PaidByID
		found, err := ObligationsFromAllocations(expense.Allocations, &payer, expense.VersionID)
		if err != nil {
			return Sheet{}, err
		}
		obligations = append(obligations, found...)
	}
	merged, err := MergeObligations(obligations)
	if err != nil {
		return Sheet{}, err
	}
	balances, err := GroupBalances(merged, receipts)
	if err != nil {
		return Sheet{}, err
	}
	entries := make([]entry, len(balances))
	for i, balance := range balances {
		entries[i] = entry{person: balance.PersonID, amount: balance.NetVND}
	}
	plan, err := settle(entries, DefaultExactLimit)
	if err != nil {
		return Sheet{}, err
	}

	canonical := func(id string) (string, error) {
		key, ok := pyuuid.Parse(id)
		if !ok {
			return "", &RangeError{Reason: "a person id that is not a uuid"}
		}
		return key, nil
	}
	for i := range balances {
		if balances[i].PersonID, err = canonical(balances[i].PersonID); err != nil {
			return Sheet{}, err
		}
	}
	// The canonical spelling orders exactly as uuid.UUID(...).bytes does:
	// fixed-width lower-case hex with hyphens at fixed places.
	slices.SortStableFunc(balances, func(a, b Balance) int { return strings.Compare(a.PersonID, b.PersonID) })
	for i := range plan.Transfers {
		if plan.Transfers[i].SenderID, err = canonical(plan.Transfers[i].SenderID); err != nil {
			return Sheet{}, err
		}
		if plan.Transfers[i].RecipientID, err = canonical(plan.Transfers[i].RecipientID); err != nil {
			return Sheet{}, err
		}
	}
	return Sheet{
		Balances:      balances,
		Transfers:     plan.Transfers,
		ProvenMinimal: plan.ProvenMinimal,
		TransferCount: plan.TransferCount,
	}, nil
}
