// Package allocator is the Go port of app/domain/contract.py and
// app/domain/allocator.py: one expense split into whole đồng per participant
// under the frozen ADR-0004 contract. POST /expenses, POST
// /expenses/{expense_id}/confirm and POST /bills/{bill_id}/split all reach it.
//
// The three money laws hold exactly as in Python (ADR-0029 §2.5):
//
//  1. Amounts are whole đồng. Every input amount is money.VND; the exact
//     shares are *big.Rat, Python's Fraction; derived sums are *big.Int or
//     *big.Rat; nothing touches a float. Rounding happens once, in apportion,
//     by the largest remainder: every exact share is floored, then one đồng
//     each goes to the participants with the largest remainders. There is no
//     round() on this path, so there is no half-even rule to reproduce.
//  2. The allocations sum to the total: apportion hands out exactly
//     total - Σ floor đồng, and Allocate refuses exact shares that miss it.
//  3. Nothing is stored or cached; Allocate is a function of its argument.
//
// ADR-0029 §2.4 makes Python the reference, including which refusal comes
// back when several apply: structural, then referential, then arithmetic,
// then reconciliation, each group visiting elements in the byte order of
// their id. testdata/python_*.json is rendered from the real modules by
// scripts/render_domain_w4_goldens.py and replayed by oracle_test.go, which
// also reads the 41 hand-computed vectors in
// services/api/tests/domain/golden in place.
//
// # Amounts outside int64
//
// Python's int has no bound, so a request can carry an amount past int64. The
// structural checks compare each amount only with 0 and MaxAmountVND, all of
// them before any arithmetic reads an amount, so an amount above MaxAmountVND
// answers exactly as MaxAmountVND+1 would and a negative one as -1 would.
// Saturate turns such a value into the money.VND this port takes without
// changing the answer; oracle_test.go replays Python's cases past int64 so.
//
// # An allocation is a stored amount
//
// An allocation is written to a BIGINT row and compared with a request's
// expected allocations, so it is money.VND. It is never narrowed from a sum:
// every exact share lies in [0, total] and total is at most MaxAmountVND, so
// floor plus the one đồng it may gain fits; Allocate still checks and answers
// *InvariantError rather than wrap.
//
// # Where Go and Python differ
//
// Python raises AssertionError if the exact shares miss the total; Go returns
// *InvariantError. Neither is reachable. Python's `name != "total"` in the
// zero-amount check compares the element's id, so a line whose id is "total"
// is never refused as ZERO_AMOUNT; Go reproduces that.
package allocator

import (
	"math"
	"math/big"
	"slices"
	"strings"

	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/domain/money"
)

// MaxAmountVND is MAX_AMOUNT_VND, 10**12 đồng, the largest amount any line or
// total may carry.
const MaxAmountVND money.VND = 1000 * 1000 * 1000 * 1000

// MaxIDBytes is MAX_ID_BYTES: the longest id, in UTF-8 bytes.
const MaxIDBytes = 64

// maxKindBytes is the literal 32 in _validate_structure's kind check.
const maxKindBytes = 32

// The refusal codes of ERROR_PRECEDENCE.
const (
	CodeNoParticipants         = "NO_PARTICIPANTS"
	CodeInvalidParticipantID   = "INVALID_PARTICIPANT_ID"
	CodeDuplicateParticipant   = "DUPLICATE_PARTICIPANT"
	CodeInvalidEntityID        = "INVALID_ENTITY_ID"
	CodeDuplicateEntityID      = "DUPLICATE_ENTITY_ID"
	CodeAmountNotInteger       = "AMOUNT_NOT_INTEGER"
	CodeNegativeAmount         = "NEGATIVE_AMOUNT"
	CodeZeroAmount             = "ZERO_AMOUNT"
	CodeAmountTooLarge         = "AMOUNT_TOO_LARGE"
	CodeInvalidKind            = "INVALID_KIND"
	CodeInvalidMode            = "INVALID_MODE"
	CodeInvalidScope           = "INVALID_SCOPE"
	CodeScopeTargetMismatch    = "SCOPE_TARGET_MISMATCH"
	CodeEmptySharedBy          = "EMPTY_SHARED_BY"
	CodeDuplicateSharedBy      = "DUPLICATE_SHARED_BY"
	CodeUnknownParticipant     = "UNKNOWN_PARTICIPANT"
	CodeUnknownItem            = "UNKNOWN_ITEM"
	CodeDiscountExceedsItem    = "DISCOUNT_EXCEEDS_ITEM"
	CodeDiscountExceedsBase    = "DISCOUNT_EXCEEDS_BASE"
	CodeReconciliationMismatch = "RECONCILIATION_MISMATCH"
)

// The warnings of WARNINGS.
const (
	WarningAdvancerNotParticipant     = "advancer_not_participant"
	WarningProportionalFallbackToEven = "proportional_fallback_to_even"
	WarningZeroShareParticipants      = "zero_share_participants"
)

// SURCHARGE_MODES and DISCOUNT_SCOPES.
const (
	ModeProportional        = "proportional"
	ModeEven                = "even"
	ScopeGlobalProportional = "global_proportional"
	ScopeItem               = "item"
)

// totalName is the name _validate_structure gives the expense total.
const totalName = "total"

var errorPrecedence = []string{
	CodeNoParticipants, CodeInvalidParticipantID, CodeDuplicateParticipant,
	CodeInvalidEntityID, CodeDuplicateEntityID, CodeAmountNotInteger,
	CodeNegativeAmount, CodeZeroAmount, CodeAmountTooLarge, CodeInvalidKind,
	CodeInvalidMode, CodeInvalidScope, CodeScopeTargetMismatch,
	CodeEmptySharedBy, CodeDuplicateSharedBy,
	CodeUnknownParticipant, CodeUnknownItem,
	CodeDiscountExceedsItem, CodeDiscountExceedsBase,
	CodeReconciliationMismatch,
}

var (
	warnings       = []string{WarningAdvancerNotParticipant, WarningProportionalFallbackToEven, WarningZeroShareParticipants}
	surchargeModes = []string{ModeProportional, ModeEven}
	discountScopes = []string{ScopeGlobalProportional, ScopeItem}
)

// ErrorPrecedence is ERROR_PRECEDENCE, in its order. The slice is a copy.
func ErrorPrecedence() []string { return slices.Clone(errorPrecedence) }

// Warnings is WARNINGS. The slice is a copy.
func Warnings() []string { return slices.Clone(warnings) }

// SurchargeModes is SURCHARGE_MODES. The slice is a copy.
func SurchargeModes() []string { return slices.Clone(surchargeModes) }

// DiscountScopes is DISCOUNT_SCOPES. The slice is a copy.
func DiscountScopes() []string { return slices.Clone(discountScopes) }

// AllocationError mirrors AllocationError: Error returns the code, as str(exc)
// does.
type AllocationError struct {
	Code string
}

func (e *AllocationError) Error() string { return e.Code }

// InvariantError is Python's AssertionError: exact shares that do not sum to
// the total, or an allocation outside int64. Neither is reachable.
type InvariantError struct {
	Reason string
}

func (e *InvariantError) Error() string { return "allocator: " + e.Reason }

func refuse(code string) error { return &AllocationError{Code: code} }

// Item is one entry of expense["items"].
type Item struct {
	ItemID    string
	AmountVND money.VND
	SharedBy  []string
}

// Surcharge is one entry of expense["surcharges"].
type Surcharge struct {
	SurchargeID string
	Kind        string
	AmountVND   money.VND
	Mode        string
}

// Discount is one entry of expense["discounts"]; ItemID nil is None.
type Discount struct {
	DiscountID string
	AmountVND  money.VND
	Scope      string
	ItemID     *string
}

// Expense is the allocator input dict. AdvancerID nil is None.
type Expense struct {
	Participants []string
	TotalVND     money.VND
	Items        []Item
	Surcharges   []Surcharge
	Discounts    []Discount
	AdvancerID   *string
}

// Share is one entry of result["allocations"].
type Share struct {
	ParticipantID string
	AmountVND     money.VND
}

// ExactShare is one entry of result["exact_shares"]. Value is in lowest terms.
type ExactShare struct {
	ParticipantID string
	Value         *big.Rat
}

// Fraction is the share as Python writes it, f"{numerator}/{denominator}".
func (s ExactShare) Fraction() string { return s.Value.String() }

// Result is allocate's dict. Allocations and ExactShares follow the order of
// the participants; Warnings are sorted and without repeats.
type Result struct {
	Allocations     []Share
	ExactShares     []ExactShare
	RoundingGainers []string
	Warnings        []string
}

// Saturate is the money.VND an integer takes on its way into Allocate: itself
// inside int64, else the int64 bound on its side. See the package comment for
// why the answer does not change.
func Saturate(n *big.Int) money.VND {
	switch {
	case n.IsInt64():
		return money.VND(n.Int64())
	case n.Sign() > 0:
		return money.VND(math.MaxInt64)
	}
	return money.VND(math.MinInt64)
}

// validID is _is_valid_id for a str: non-empty, equal to its str.strip(), at
// most MaxIDBytes bytes of UTF-8.
func validID(value string) bool {
	return value != "" && contexts.Strip(value) == value && len(value) <= MaxIDBytes
}

// byID is _by_id: a stable sort by the UTF-8 bytes of each element's id.
func byID[T any](elements []T, id func(T) string) []T {
	sorted := slices.Clone(elements)
	slices.SortStableFunc(sorted, func(a, b T) int { return strings.Compare(id(a), id(b)) })
	return sorted
}

func sortedStrings(values []string) []string {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted
}

func hasRepeats(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func itemID(i Item) string           { return i.ItemID }
func surchargeID(s Surcharge) string { return s.SurchargeID }
func discountID(d Discount) string   { return d.DiscountID }

// named is one entry of _validate_structure's amounts list.
type named struct {
	name   string
	amount money.VND
}

// validateStructure is _validate_structure.
func validateStructure(e Expense) error {
	if len(e.Participants) == 0 {
		return refuse(CodeNoParticipants)
	}
	for _, participant := range sortedStrings(e.Participants) {
		if !validID(participant) {
			return refuse(CodeInvalidParticipantID)
		}
	}
	if hasRepeats(e.Participants) {
		return refuse(CodeDuplicateParticipant)
	}

	items := byID(e.Items, itemID)
	surcharges := byID(e.Surcharges, surchargeID)
	discounts := byID(e.Discounts, discountID)
	var itemIDs, surchargeIDs, discountIDs []string
	for _, item := range items {
		itemIDs = append(itemIDs, item.ItemID)
	}
	for _, surcharge := range surcharges {
		surchargeIDs = append(surchargeIDs, surcharge.SurchargeID)
	}
	for _, discount := range discounts {
		discountIDs = append(discountIDs, discount.DiscountID)
	}
	for _, ids := range [][]string{itemIDs, surchargeIDs, discountIDs} {
		for _, id := range ids {
			if !validID(id) {
				return refuse(CodeInvalidEntityID)
			}
		}
	}
	for _, ids := range [][]string{itemIDs, surchargeIDs, discountIDs} {
		if hasRepeats(ids) {
			return refuse(CodeDuplicateEntityID)
		}
	}

	amounts := []named{{totalName, e.TotalVND}}
	for _, item := range items {
		amounts = append(amounts, named{item.ItemID, item.AmountVND})
	}
	for _, surcharge := range surcharges {
		amounts = append(amounts, named{surcharge.SurchargeID, surcharge.AmountVND})
	}
	for _, discount := range discounts {
		amounts = append(amounts, named{discount.DiscountID, discount.AmountVND})
	}
	// AMOUNT_NOT_INTEGER cannot happen: money.VND is an integer by type.
	for _, a := range amounts {
		if a.amount < 0 {
			return refuse(CodeNegativeAmount)
		}
	}
	for _, a := range amounts {
		if a.name != totalName && a.amount == 0 {
			return refuse(CodeZeroAmount)
		}
	}
	for _, a := range amounts {
		if a.amount > MaxAmountVND {
			return refuse(CodeAmountTooLarge)
		}
	}

	for _, surcharge := range surcharges {
		if surcharge.Kind == "" || len(surcharge.Kind) > maxKindBytes {
			return refuse(CodeInvalidKind)
		}
	}
	for _, surcharge := range surcharges {
		if !slices.Contains(surchargeModes, surcharge.Mode) {
			return refuse(CodeInvalidMode)
		}
	}
	for _, discount := range discounts {
		if !slices.Contains(discountScopes, discount.Scope) {
			return refuse(CodeInvalidScope)
		}
	}
	for _, discount := range discounts {
		if (discount.Scope == ScopeItem) != (discount.ItemID != nil) {
			return refuse(CodeScopeTargetMismatch)
		}
	}
	for _, item := range items {
		if len(item.SharedBy) == 0 {
			return refuse(CodeEmptySharedBy)
		}
	}
	for _, item := range items {
		if hasRepeats(item.SharedBy) {
			return refuse(CodeDuplicateSharedBy)
		}
	}
	return nil
}

// validateReferences is _validate_references.
func validateReferences(e Expense) error {
	participants := make(map[string]bool, len(e.Participants))
	for _, participant := range e.Participants {
		participants[participant] = true
	}
	for _, item := range byID(e.Items, itemID) {
		for _, participant := range sortedStrings(item.SharedBy) {
			if !participants[participant] {
				return refuse(CodeUnknownParticipant)
			}
		}
	}
	itemIDs := make(map[string]bool, len(e.Items))
	for _, item := range e.Items {
		itemIDs[item.ItemID] = true
	}
	for _, discount := range byID(e.Discounts, discountID) {
		if discount.Scope == ScopeItem && (discount.ItemID == nil || !itemIDs[*discount.ItemID]) {
			return refuse(CodeUnknownItem)
		}
	}
	return nil
}

func rat(amount money.VND) *big.Rat { return new(big.Rat).SetInt64(int64(amount)) }

// exactShares is _exact_shares: one exact share per participant, in their
// order, and the warnings it raised.
func exactShares(e Expense) ([]*big.Rat, []string, error) {
	count := len(e.Participants)
	var raised []string
	if len(e.Items) == 0 && len(e.Surcharges) == 0 && len(e.Discounts) == 0 {
		shares := make([]*big.Rat, count)
		for i := range shares {
			shares[i] = new(big.Rat).Quo(rat(e.TotalVND), new(big.Rat).SetInt64(int64(count)))
		}
		return shares, raised, nil
	}

	// _item_net: each item's amount less its item-scoped discounts.
	net := make(map[string]*big.Rat, len(e.Items))
	for _, item := range e.Items {
		net[item.ItemID] = rat(item.AmountVND)
	}
	for _, discount := range e.Discounts {
		if discount.Scope == ScopeItem {
			target := net[*discount.ItemID]
			target.Sub(target, rat(discount.AmountVND))
		}
	}
	for _, item := range byID(e.Items, itemID) {
		if net[item.ItemID].Sign() < 0 {
			return nil, nil, refuse(CodeDiscountExceedsItem)
		}
	}

	// Stage 1: item shares, net of item-scoped discounts, split evenly.
	index := make(map[string]int, count)
	base := make([]*big.Rat, count)
	for i, participant := range e.Participants {
		index[participant] = i
		base[i] = new(big.Rat)
	}
	for _, item := range e.Items {
		share := new(big.Rat).Quo(net[item.ItemID], new(big.Rat).SetInt64(int64(len(item.SharedBy))))
		for _, participant := range item.SharedBy {
			target := base[index[participant]]
			target.Add(target, share)
		}
	}

	// Stage 2: global discounts, proportional.
	totalBase := sumRats(base)
	globalDiscount := new(big.Rat)
	for _, discount := range e.Discounts {
		if discount.Scope == ScopeGlobalProportional {
			globalDiscount.Add(globalDiscount, rat(discount.AmountVND))
		}
	}
	if globalDiscount.Cmp(totalBase) > 0 {
		return nil, nil, refuse(CodeDiscountExceedsBase)
	}
	if totalBase.Sign() > 0 {
		factor := new(big.Rat).Sub(totalBase, globalDiscount)
		factor.Quo(factor, totalBase)
		for _, value := range base {
			value.Mul(value, factor)
		}
	}

	// Reconciliation, after the arithmetic group.
	listed := new(big.Int)
	for _, item := range e.Items {
		listed.Add(listed, big.NewInt(int64(item.AmountVND)))
	}
	for _, surcharge := range e.Surcharges {
		listed.Add(listed, big.NewInt(int64(surcharge.AmountVND)))
	}
	for _, discount := range e.Discounts {
		listed.Sub(listed, big.NewInt(int64(discount.AmountVND)))
	}
	if listed.Cmp(big.NewInt(int64(e.TotalVND))) != 0 {
		return nil, nil, refuse(CodeReconciliationMismatch)
	}

	// Stage 3: surcharges.
	basis := sumRats(base)
	extra := make([]*big.Rat, count)
	for i := range extra {
		extra[i] = new(big.Rat)
	}
	people := new(big.Rat).SetInt64(int64(count))
	for _, surcharge := range e.Surcharges {
		amount := rat(surcharge.AmountVND)
		if surcharge.Mode == ModeEven || basis.Sign() == 0 {
			if surcharge.Mode == ModeProportional && !slices.Contains(raised, WarningProportionalFallbackToEven) {
				raised = append(raised, WarningProportionalFallbackToEven)
			}
			each := new(big.Rat).Quo(amount, people)
			for _, value := range extra {
				value.Add(value, each)
			}
			continue
		}
		for i, value := range extra {
			part := new(big.Rat).Mul(amount, base[i])
			value.Add(value, part.Quo(part, basis))
		}
	}

	// Stage 4.
	shares := make([]*big.Rat, count)
	for i := range shares {
		shares[i] = new(big.Rat).Add(base[i], extra[i])
	}
	return shares, raised, nil
}

func sumRats(values []*big.Rat) *big.Rat {
	total := new(big.Rat)
	for _, value := range values {
		total.Add(total, value)
	}
	return total
}

// candidate is one participant being ranked for the leftover đồng.
type candidate struct {
	participant string
	remainder   *big.Rat
	advancer    bool
}

// apportion is _apportion, stage 5 and the only rounding point: each exact
// share floored, then the deficit total - Σ floor handed out one đồng each
// down the ranking (largest remainder, then the advancer, then the UTF-8
// bytes of the id). It returns each participant's allocation, in their order,
// and the gainers, in ranking order. participants must not repeat.
func apportion(total *big.Int, participants []string, exact []*big.Rat, advancerID *string) ([]*big.Int, []string) {
	floors := make([]*big.Int, len(exact))
	deficit := new(big.Int).Set(total)
	ranked := make([]candidate, len(exact))
	for i, value := range exact {
		// Euclidean division by a positive denominator is Python's floor //.
		floors[i] = new(big.Int).Div(value.Num(), value.Denom())
		deficit.Sub(deficit, floors[i])
		ranked[i] = candidate{
			participant: participants[i],
			remainder:   new(big.Rat).Sub(value, new(big.Rat).SetInt(floors[i])),
			advancer:    advancerID != nil && participants[i] == *advancerID,
		}
	}
	slices.SortStableFunc(ranked, func(a, b candidate) int {
		if c := b.remainder.Cmp(a.remainder); c != 0 {
			return c
		}
		if a.advancer != b.advancer {
			if a.advancer {
				return -1
			}
			return 1
		}
		return strings.Compare(a.participant, b.participant)
	})
	stop := sliceStop(deficit, len(ranked))
	gainers := make([]string, 0, stop)
	gained := make(map[string]bool, stop)
	for _, c := range ranked[:stop] {
		gainers = append(gainers, c.participant)
		gained[c.participant] = true
	}
	allocations := make([]*big.Int, len(exact))
	for i, floor := range floors {
		allocations[i] = new(big.Int).Set(floor)
		if gained[participants[i]] {
			allocations[i].Add(allocations[i], big.NewInt(1))
		}
	}
	return allocations, gainers
}

// sliceStop is where Python's sequence[:stop] ends for a sequence of length n.
func sliceStop(stop *big.Int, n int) int {
	length := big.NewInt(int64(n))
	if stop.Sign() >= 0 {
		if stop.Cmp(length) > 0 {
			return n
		}
		return int(stop.Int64())
	}
	from := new(big.Int).Add(length, stop)
	if from.Sign() < 0 {
		return 0
	}
	return int(from.Int64())
}

// Allocate is allocate: expense split into whole đồng per participant, or
// *AllocationError with the first code of ERROR_PRECEDENCE that applies.
func Allocate(e Expense) (Result, error) {
	if err := validateStructure(e); err != nil {
		return Result{}, err
	}
	if err := validateReferences(e); err != nil {
		return Result{}, err
	}
	exact, raised, err := exactShares(e)
	if err != nil {
		return Result{}, err
	}
	total := big.NewInt(int64(e.TotalVND))
	if sumRats(exact).Cmp(new(big.Rat).SetInt(total)) != 0 {
		return Result{}, &InvariantError{Reason: "exact shares do not sum to the expense total"}
	}

	amounts, gainers := apportion(total, e.Participants, exact, e.AdvancerID)

	if e.AdvancerID != nil && !slices.Contains(e.Participants, *e.AdvancerID) {
		raised = append(raised, WarningAdvancerNotParticipant)
	}
	if total.Sign() > 0 && slices.ContainsFunc(exact, func(value *big.Rat) bool { return value.Sign() == 0 }) {
		raised = append(raised, WarningZeroShareParticipants)
	}
	slices.Sort(raised)

	result := Result{
		Allocations:     make([]Share, len(e.Participants)),
		ExactShares:     make([]ExactShare, len(e.Participants)),
		RoundingGainers: gainers,
		Warnings:        slices.Compact(raised),
	}
	for i, participant := range e.Participants {
		if !amounts[i].IsInt64() {
			return Result{}, &InvariantError{Reason: "an allocation outside int64"}
		}
		result.Allocations[i] = Share{ParticipantID: participant, AmountVND: money.VND(amounts[i].Int64())}
		result.ExactShares[i] = ExactShare{ParticipantID: participant, Value: exact[i]}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result, nil
}
