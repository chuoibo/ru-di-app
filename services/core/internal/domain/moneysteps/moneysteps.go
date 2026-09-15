// Package moneysteps holds the pure steps app/api/service.py runs inline for
// the W4 money routes, which are not functions of app/domain in Python:
//
//	RequireParticipantsAreMembers  _require_participants_are_members
//	ProposeExpense                 propose_expense, before create_expense
//	ConfirmExpense                 confirm_expense, from its roster read to
//	                               save_expense_confirmation
//	CreateBillChecks               create_bill, after its permission check
//	BillAssignment                 _wire_bill's assignment state (GET, PUT,
//	                               POST .../my-items and POST /bills)
//	SplitBill                      split_bill, after _bill_for_actor
//	FreezeBatch                    create_batch, after its permission checks
//	PublishBatch                   publish_batch, after its permission check
//	ReceiptStatus                  confirm_receipt, after the receipt is saved
//	FinanceReadable                person_finance_summary's self-only rule
//	GroupBudget                    group_budget, after its reads
//
// The route keeps what is not pure: the permission check that opens a method
// (the permissions package decides it), the repository reads and writes, the
// clock, the token of a guest link. A repository read the service makes only
// on some branches is a callback here, so a route calling these functions
// issues the same reads, in the same order, as the Python method. A refusal is
// the ApiProblem the method raises, as *Refusal with its status, code and
// detail; an error is what Python lets escape as a 500.
//
// Ids are the canonical uuid strings a route reads (str(uuid.UUID)), whose
// byte order is the order of uuid.UUID.bytes the service sorts by.
//
// The money laws hold as in the domain packages: stored and request amounts
// are money.VND, derived sums (a merged obligation, the sum of a bill's
// lines) are *big.Int.
//
// testdata/python_*.json is rendered by scripts/render_domain_w4_goldens.py by
// running the real service methods over a recording stub repository;
// oracle_test.go replays every case, including which calls were made.
package moneysteps

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/billdraft"
	"mobile/services/core/internal/domain/budget"
	"mobile/services/core/internal/domain/capability"
	"mobile/services/core/internal/domain/collection"
	"mobile/services/core/internal/domain/expense"
	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/permissions"
)

// Refusal is an ApiProblem the service raises.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

// ResponseError is a pydantic ValidationError the response model raises on
// data the service built, which FastAPI answers as a 500.
type ResponseError struct {
	Reason string
}

func (e *ResponseError) Error() string { return "moneysteps: " + e.Reason }

// Member is one row of list_members: who, and the state of the membership.
type Member struct {
	PersonID string
	State    string
}

const stateActive = "active"

// provenance is the provenance _require_permission records.
const provenance = "api_service"

// requirePermission is _require_permission with the facts it proves, in the
// order the service writes them. A nil *Refusal is a grant.
func requirePermission(action, actorID string, roles []string, facts ...fact) (*Refusal, error) {
	var proven []string
	for _, f := range facts {
		if f.holds {
			proven = append(proven, f.name)
		}
	}
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    actorID,
		Roles:      roles,
		Proven:     proven,
		Provenance: provenance,
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return &Refusal{Status: 403, Code: "permission_denied", Detail: reason}, nil
	}
	return nil, nil
}

type fact struct {
	name  string
	holds bool
}

func activeIDs(roster []Member) map[string]bool {
	active := map[string]bool{}
	for _, member := range roster {
		if member.State == stateActive {
			active[member.PersonID] = true
		}
	}
	return active
}

func sortedUnique(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

// RequireParticipantsAreMembers is _require_participants_are_members over the
// roster the service read: 422 participant_not_in_context naming every id
// that is not an active member, once each, in uuid order.
func RequireParticipantsAreMembers(roster []Member, participants []string) *Refusal {
	active := activeIDs(roster)
	var strangers []string
	for _, participant := range participants {
		if !active[participant] {
			strangers = append(strangers, participant)
		}
	}
	if len(strangers) == 0 {
		return nil
	}
	return &Refusal{
		Status: 422,
		Code:   "participant_not_in_context",
		Detail: "Not members of this group: " + strings.Join(sortedUnique(strangers), ", "),
	}
}

// allocate runs the allocator and answers its refusal as the 422 the service
// raises with detail.
func allocate(e allocator.Expense, detail string) (allocator.Result, *Refusal, error) {
	result, err := allocator.Allocate(e)
	var refused *allocator.AllocationError
	if errors.As(err, &refused) {
		return allocator.Result{}, &Refusal{Status: 422, Code: refused.Code, Detail: detail}, nil
	}
	return result, nil, err
}

const expenseNotAllocated = "Expense cannot be allocated"

// ProposeExpense is propose_expense up to create_expense: the allocation, or
// 422 with the allocator's code.
func ProposeExpense(e allocator.Expense) (allocator.Result, *Refusal, error) {
	return allocate(e, expenseNotAllocated)
}

// ExpenseConfirmation is what confirm_expense reads from its request and the
// actor once the expense exists, its context matches and the actor may
// confirm. Expense is _allocator_input(request.proposal), whose AdvancerID is
// the proposal's paid_by_id.
type ExpenseConfirmation struct {
	ActorID               string
	ActorRoles            []string
	Expense               allocator.Expense
	PaidByID              string
	RecordedByID          string
	ExpectedAllocations   []allocator.Share
	AcknowledgeAsAdvancer bool
}

// ConfirmationPlan is what confirm_expense hands save_expense_confirmation.
type ConfirmationPlan struct {
	PayerAcknowledgement string
	Allocation           allocator.Result
	Rollups              expense.Rollups
}

// ConfirmExpense is confirm_expense from list_members to
// save_expense_confirmation: every id that names a person must be an active
// member (422); acknowledging needs the advancer role and to be the named
// payer (403); the allocation must succeed (422) and equal the reviewed
// allocations as a dict (409 proposal_changed).
func ConfirmExpense(in ExpenseConfirmation, listMembers func() ([]Member, error)) (ConfirmationPlan, *Refusal, error) {
	roster, err := listMembers()
	if err != nil {
		return ConfirmationPlan{}, nil, err
	}
	named := append(slices.Clone(in.Expense.Participants), in.PaidByID, in.RecordedByID)
	if refused := RequireParticipantsAreMembers(roster, named); refused != nil {
		return ConfirmationPlan{}, refused, nil
	}
	acknowledgement := "pending"
	if in.AcknowledgeAsAdvancer {
		refused, err := requirePermission("acknowledge_advancer_role", in.ActorID, in.ActorRoles,
			fact{"is_named_advancer", in.ActorID == in.PaidByID})
		if err != nil || refused != nil {
			return ConfirmationPlan{}, refused, err
		}
		acknowledgement = "acknowledged"
	}
	result, refused, err := allocate(in.Expense, expenseNotAllocated)
	if err != nil || refused != nil {
		return ConfirmationPlan{}, refused, err
	}
	if !sameAllocations(result.Allocations, in.ExpectedAllocations) {
		return ConfirmationPlan{}, &Refusal{
			Status: 409,
			Code:   "proposal_changed",
			Detail: "Confirmed allocations differ from the reviewed proposal",
		}, nil
	}
	return ConfirmationPlan{
		PayerAcknowledgement: acknowledgement,
		Allocation:           result,
		Rollups:              expense.ComponentRollups(in.Expense),
	}, nil, nil
}

// sameAllocations is dict equality of two {participant: amount} maps. Neither
// side repeats a participant.
func sameAllocations(got, expected []allocator.Share) bool {
	if len(got) != len(expected) {
		return false
	}
	amounts := make(map[string]money.VND, len(got))
	for _, share := range got {
		amounts[share.ParticipantID] = share.AmountVND
	}
	for _, share := range expected {
		amount, found := amounts[share.ParticipantID]
		if !found || amount != share.AmountVND {
			return false
		}
	}
	return true
}

// BillLine is one item of a bill creation request, as create_bill reads it.
type BillLine struct {
	LineTotalVND            money.VND
	SuggestedParticipantIDs []string
}

// CreateBillChecks is create_bill between its permission check and the
// repository write: every suggested id must be an active member (422), then
// the declared items total must equal the sum of the lines (422
// bill_items_total_mismatch, both figures in the detail).
func CreateBillChecks(itemsTotalVND money.VND, lines []BillLine, listMembers func() ([]Member, error)) (*Refusal, error) {
	roster, err := listMembers()
	if err != nil {
		return nil, err
	}
	var suggested []string
	sum := new(big.Int)
	for _, line := range lines {
		suggested = append(suggested, line.SuggestedParticipantIDs...)
		sum.Add(sum, big.NewInt(int64(line.LineTotalVND)))
	}
	if refused := RequireParticipantsAreMembers(roster, suggested); refused != nil {
		return refused, nil
	}
	if sum.Cmp(big.NewInt(int64(itemsTotalVND))) != 0 {
		return &Refusal{
			Status: 422,
			Code:   "bill_items_total_mismatch",
			Detail: fmt.Sprintf("Declared items total %d does not match the sum of the lines %s", itemsTotalVND, sum),
		}, nil
	}
	return nil, nil
}

// BillItemSources is one stored bill item as _wire_bill reads it: its key and
// the source of each share.
type BillItemSources struct {
	ItemKey string
	Sources []string
}

// BillAssignment is _wire_bill's assignment_state and suggested_item_keys:
// confirmed only when the bill has items and every item has shares that are
// all confirmed; the keys of items with a suggested share, in byte order. A
// source outside the two is refused by BillResponse itself; the store's check
// constraint admits only the two.
func BillAssignment(items []BillItemSources) (string, []string) {
	suggested := []string{}
	allConfirmed := len(items) > 0
	for _, item := range items {
		if slices.Contains(item.Sources, billdraft.ShareSuggested) {
			suggested = append(suggested, item.ItemKey)
		}
		if len(item.Sources) == 0 || slices.ContainsFunc(item.Sources, func(s string) bool { return s != billdraft.ShareConfirmed }) {
			allConfirmed = false
		}
	}
	slices.SortStableFunc(suggested, strings.Compare)
	if allConfirmed {
		return billdraft.ShareConfirmed, suggested
	}
	return billdraft.ShareSuggested, suggested
}

// BillRecord is the stored bill split_bill projects: each item's
// line_total_vnd is its amount.
type BillRecord struct {
	PrintedTotalVND *money.VND
	Items           []billdraft.Item
	Surcharges      []allocator.Surcharge
	Discounts       []allocator.Discount
}

// Split is BillSplitResponse.
type Split struct {
	Allocation        allocator.Result
	AssignmentState   string
	SuggestedItemKeys []string
	TotalAmountVND    money.VND
	ParticipantIDs    []string
	ExcludedMemberIDs []string
}

// SplitBill is split_bill after _bill_for_actor, over the roster it reads
// once: the active members are the participants, the rest are excluded (a
// list, so a repeated row repeats); the bill is projected (422 with the
// BillError code), must be confirmed when it is for the ledger (422
// bill_assignments_not_confirmed) and is allocated (422 with the allocator's
// code). A person both active and not is refused by the response model, a
// *ResponseError.
func SplitBill(record BillRecord, forLedger bool, paidByID *string, roster []Member) (Split, *Refusal, error) {
	var participants, excluded []string
	for _, member := range roster {
		if member.State == stateActive {
			participants = append(participants, member.PersonID)
		} else {
			excluded = append(excluded, member.PersonID)
		}
	}
	participants = sortedUnique(participants)
	slices.Sort(excluded)
	if excluded == nil {
		excluded = []string{}
	}

	projection, err := billdraft.AllocatorInputFromBill(billdraft.Bill{
		Participants:    participants,
		PrintedTotalVND: record.PrintedTotalVND,
		Items:           record.Items,
		Surcharges:      record.Surcharges,
		Discounts:       record.Discounts,
		AdvancerID:      paidByID,
	})
	var refused *billdraft.BillError
	if errors.As(err, &refused) {
		return Split{}, &Refusal{Status: 422, Code: refused.Code, Detail: "Bill cannot be projected"}, nil
	}
	if err != nil {
		return Split{}, nil, err
	}
	if forLedger && projection.AssignmentState != billdraft.ShareConfirmed {
		return Split{}, &Refusal{
			Status: 422,
			Code:   "bill_assignments_not_confirmed",
			Detail: "Bill assignments must be confirmed before ledger use",
		}, nil
	}
	result, problem, err := allocate(projection.Expense, "Bill cannot be allocated")
	if err != nil || problem != nil {
		return Split{}, problem, err
	}
	for _, id := range excluded {
		if slices.Contains(participants, id) {
			return Split{}, nil, &ResponseError{Reason: "nobody can be both split between and left out"}
		}
	}
	if participants == nil {
		participants = []string{}
	}
	return Split{
		Allocation:        result,
		AssignmentState:   projection.AssignmentState,
		SuggestedItemKeys: projection.SuggestedItemKeys,
		// The allocator accepted the total, so it lies in [0, MaxAmountVND]
		// and its saturated value is exact.
		TotalAmountVND:    projection.Expense.TotalVND,
		ParticipantIDs:    participants,
		ExcludedMemberIDs: excluded,
	}, nil, nil
}

// AllocationRow is one confirmed allocation row.
type AllocationRow struct {
	ID            string
	ParticipantID string
	AmountVND     money.VND
}

// BatchExpense is one ConfirmedExpense of load_batch_inputs.
type BatchExpense struct {
	VersionID   string
	PaidByID    string
	Allocations []AllocationRow
}

// BatchInputs is load_batch_inputs' answer.
type BatchInputs struct {
	Expenses              []BatchExpense
	UnavailableVersionIDs []string
}

// ObligationDraft is ObligationDraft: one merged obligation, its sources.
type ObligationDraft struct {
	SenderID                string
	RecipientID             string
	AmountVND               *big.Int
	SourceExpenseVersionIDs []string
	Sources                 []AllocationRow
}

// FreezeBatch is create_batch after its two permission checks. due_at must be
// after now (422). Without named versions the service reads every confirmed
// version first (load(true, nil)) and selects them all; it then reads the
// selection (load(false, selected)). A named version that is unavailable is
// 409, as is a selection with no expenses; each expense's allocations become
// obligations, merged per pair (409 with the LedgerError code); nothing owed
// is 409 no_obligations; the batch freezes (409 with the CollectionError
// code). Each draft carries the rows behind its pair: every row, in order, of
// a participant other than the payer with an amount above zero.
func FreezeBatch(dueAt, now time.Time, namedVersionIDs []string, named bool, load func(all bool, versionIDs []string) (BatchInputs, error)) ([]ObligationDraft, *Refusal, error) {
	if !dueAt.After(now) {
		return nil, &Refusal{Status: 422, Code: "due_at_not_future", Detail: "due_at must be in the future"}, nil
	}
	selected := namedVersionIDs
	if !named {
		confirmed, err := load(true, nil)
		if err != nil {
			return nil, nil, err
		}
		selected = []string{}
		for _, e := range confirmed.Expenses {
			selected = append(selected, e.VersionID)
		}
	}
	inputs, err := load(false, selected)
	if err != nil {
		return nil, nil, err
	}
	if named && len(inputs.UnavailableVersionIDs) > 0 {
		return nil, &Refusal{Status: 409, Code: "expense_versions_unavailable", Detail: "A selected version is missing, superseded, or already batched"}, nil
	}
	if len(inputs.Expenses) == 0 {
		return nil, &Refusal{Status: 409, Code: "no_unbatched_allocations", Detail: "No allocations are available"}, nil
	}

	var raw []ledger.Obligation
	sources := map[ledger.Pair][]AllocationRow{}
	var merged []ledger.MergedObligation
	err = func() error {
		for _, e := range inputs.Expenses {
			allocations := make([]ledger.Allocation, len(e.Allocations))
			for i, row := range e.Allocations {
				allocations[i] = ledger.Allocation{ParticipantID: row.ParticipantID, AmountVND: row.AmountVND}
			}
			payer := e.PaidByID
			found, err := ledger.ObligationsFromAllocations(allocations, &payer, e.VersionID)
			if err != nil {
				return err
			}
			raw = append(raw, found...)
			for _, row := range e.Allocations {
				if row.ParticipantID != e.PaidByID && row.AmountVND > 0 {
					pair := ledger.Pair{SenderID: row.ParticipantID, RecipientID: e.PaidByID}
					sources[pair] = append(sources[pair], row)
				}
			}
		}
		var err error
		merged, err = ledger.MergeObligations(raw)
		return err
	}()
	var ledgerRefused *ledger.LedgerError
	if errors.As(err, &ledgerRefused) {
		return nil, &Refusal{Status: 409, Code: ledgerRefused.Code, Detail: "Allocations cannot form obligations"}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if len(merged) == 0 {
		return nil, &Refusal{Status: 409, Code: "no_obligations", Detail: "The selected allocations owe no money"}, nil
	}

	obligations := make([]collection.Obligation, len(merged))
	for i, m := range merged {
		obligations[i] = collection.Obligation{SenderID: m.SenderID}
	}
	state, err := collection.Transition(collection.StateAccruing, "freeze", collection.Context{Obligations: obligations})
	var collectionRefused *collection.CollectionError
	if errors.As(err, &collectionRefused) {
		return nil, &Refusal{Status: 409, Code: collectionRefused.Code, Detail: "Batch cannot be frozen"}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if state != collection.StateFrozen {
		return nil, &Refusal{Status: 500, Code: "unexpected_batch_state", Detail: "Domain returned an invalid state"}, nil
	}

	drafts := make([]ObligationDraft, len(merged))
	for i, m := range merged {
		rows := sources[ledger.Pair{SenderID: m.SenderID, RecipientID: m.RecipientID}]
		if rows == nil {
			rows = []AllocationRow{}
		}
		drafts[i] = ObligationDraft{
			SenderID:                m.SenderID,
			RecipientID:             m.RecipientID,
			AmountVND:               m.AmountVND,
			SourceExpenseVersionIDs: m.SourceExpenseVersionIDs,
			Sources:                 rows,
		}
	}
	return drafts, nil, nil
}

// PublishObligation is one obligation of BatchForPublish.
type PublishObligation struct {
	ID             string
	BatchVersionID string
	SenderID       string
	RecipientID    string
	AmountVND      money.VND
}

// Batch is BatchForPublish as publish_batch reads it after its permission
// check.
type Batch struct {
	VersionID            string
	Status               string
	AdvancerAcknowledged bool
	Obligations          []PublishObligation
}

// SenderLink is one guest link to mint: a sender and their obligations, in
// the order the batch lists them.
type SenderLink struct {
	SenderID    string
	Obligations []PublishObligation
}

// PublishPlan is the state to store and the links to mint, in uuid order of
// sender.
type PublishPlan struct {
	State string
	Links []SenderLink
}

// PublishBatch is publish_batch between its permission check and its tokens:
// the link must expire after now (422); the gates must be met (409 with the
// gate's name, lower case); the batch must be publishable (409 with the
// CollectionError code); each sender's obligations must form one capability
// scope (409 with the CapabilityScopeError code). deliveryMethod "" is a
// method not chosen.
func PublishBatch(batch Batch, deliveryMethod string, expiresAt, now time.Time) (PublishPlan, *Refusal, error) {
	if !expiresAt.After(now) {
		return PublishPlan{}, &Refusal{Status: 422, Code: "guest_link_expiry_not_future", Detail: "guest_link_expires_at must be in the future"}, nil
	}
	gates := collection.Context{AdvancerAcknowledged: batch.AdvancerAcknowledged, DeliveryMethodChosen: deliveryMethod != ""}
	if unmet := collection.UnmetPublishGates(gates); len(unmet) > 0 {
		return PublishPlan{}, &Refusal{Status: 409, Code: unmet[0], Detail: "A publish gate is not satisfied"}, nil
	}
	state, err := collection.Transition(batch.Status, "publish", gates)
	var collectionRefused *collection.CollectionError
	if errors.As(err, &collectionRefused) {
		return PublishPlan{}, &Refusal{Status: 409, Code: collectionRefused.Code, Detail: "Batch cannot be published"}, nil
	}
	if err != nil {
		return PublishPlan{}, nil, err
	}

	bySender := map[string][]PublishObligation{}
	var senders []string
	for _, obligation := range batch.Obligations {
		if _, seen := bySender[obligation.SenderID]; !seen {
			senders = append(senders, obligation.SenderID)
		}
		bySender[obligation.SenderID] = append(bySender[obligation.SenderID], obligation)
	}
	slices.Sort(senders)
	plan := PublishPlan{State: state, Links: []SenderLink{}}
	for _, sender := range senders {
		obligations := bySender[sender]
		scoped := make([]capability.Obligation, len(obligations))
		for i, o := range obligations {
			scoped[i] = capability.Obligation{ObligationID: o.ID, BatchVersionID: o.BatchVersionID, SenderID: o.SenderID}
		}
		_, err := capability.ScopeOf(capability.Envelope{BatchVersionID: batch.VersionID, SenderID: sender}, scoped)
		var scopeRefused *capability.CapabilityScopeError
		if errors.As(err, &scopeRefused) {
			return PublishPlan{}, &Refusal{Status: 409, Code: scopeRefused.Code, Detail: "A guest capability cannot be built"}, nil
		}
		if err != nil {
			return PublishPlan{}, nil, err
		}
		plan.Links = append(plan.Links, SenderLink{SenderID: sender, Obligations: obligations})
	}
	return plan, nil, nil
}

// ReceiptStatus is confirm_receipt after save_receipt_confirmation: the
// obligation's status over every confirmed receipt, or 409 with the
// LedgerError code.
func ReceiptStatus(declaredVND money.VND, receiptsVND []money.VND) (string, *Refusal, error) {
	status, err := ledger.ObligationStatus(declaredVND, receiptsVND)
	var refused *ledger.LedgerError
	if errors.As(err, &refused) {
		return "", &Refusal{Status: 409, Code: refused.Code, Detail: "Receipt events are invalid"}, nil
	}
	return status, nil, err
}

// FinanceReadable is person_finance_summary's rule: only the person it
// describes may read it (403 not_your_finances).
func FinanceReadable(actorID, personID string) *Refusal {
	if actorID != personID {
		return &Refusal{Status: 403, Code: "not_your_finances", Detail: "A finance summary is readable only by the person it describes"}
	}
	return nil
}

// GroupBudget is group_budget after its reads: the outings of group_recap
// and the number of active members, compared with the candidate. A
// *budget.BudgetError escapes, as it does in Python.
func GroupBudget(outings []budget.Outing, roster []Member, candidate *money.VND) (budget.Budget, error) {
	active := int64(0)
	for _, member := range roster {
		if member.State == stateActive {
			active++
		}
	}
	return budget.BuildGroupBudget(outings, active, candidate)
}
