package routes

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
)

// proposeExpense is POST /expenses (routes/expenses.py propose_expense,
// ApiService.propose_expense). The route declares no get_actor, so it serves
// anyone, anonymous callers included and in prod too, and reads no permission
// table. The allocator runs before the context is looked up: a bad split for
// a group that does not exist is the allocator's 422. The context is proven
// only by the expenses row's foreign key (404). Nothing reaches the ledger.
func proposeExpense() Route {
	return Route{ID: "POST /expenses", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		proposal, err := readExpenseInput(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		allocation, refused, err := moneysteps.ProposeExpense(proposal.expense)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, refuseMoney(refused)
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		identity, err := store.CreateExpense(ctx, proposal.contextID)
		var conflict *repo.Conflict
		if errors.As(err, &conflict) && conflict.Code == "EXPENSE_CONTEXT_NOT_FOUND" {
			return endpoint.Reply{}, contextNotFound()
		}
		if err != nil {
			// Any other conflict is re-raised in Python: a 500.
			return endpoint.Reply{}, err
		}
		echo, err := dumpModel(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		out := pyjson.NewOrderedMap()
		out.Set("expense_id", pyjson.String(identity.ID))
		out.Set("proposal", echo)
		out.Set("allocation", wireAllocation(allocation))
		return endpoint.Reply{Body: out}, nil
	}}
}

// confirmExpense is POST /expenses/{expense_id}/confirm (confirm_expense):
// the expense row locked FOR UPDATE (404) and its context compared with the
// proposal's (409), both before any permission; membership; then the pure
// steps (every named person an active member, the advancer acknowledgement,
// the allocation equal to the reviewed one) and a new immutable version. The
// response echoes the request's expected allocations in the request's key
// order.
func confirmExpense() Route {
	return Route{ID: "POST /expenses/{expense_id}/confirm", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		expenseID, err := pathUUID(call, "expense_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		proposalModel, err := modelField(body, "proposal")
		if err != nil {
			return endpoint.Reply{}, err
		}
		proposal, err := readExpenseInput(proposalModel)
		if err != nil {
			return endpoint.Reply{}, err
		}
		expected, err := readExpectedAllocations(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		acknowledge, err := boolField(body, "acknowledge_as_advancer")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		identity, err := store.GetExpense(ctx, expenseID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if identity == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "expense_not_found", "Expense does not exist")
		}
		if identity.ContextID != proposal.contextID {
			return endpoint.Reply{}, endpoint.Refuse(409, "expense_context_mismatch", "Proposal context does not match the expense identity")
		}
		if err := requireGroupMember(ctx, call, store, "confirm_expense_proposal", identity.ContextID); err != nil {
			return endpoint.Reply{}, err
		}
		plan, refused, err := moneysteps.ConfirmExpense(moneysteps.ExpenseConfirmation{
			ActorID:               call.Actor.ID,
			ActorRoles:            call.Actor.Roles,
			Expense:               proposal.expense,
			PaidByID:              proposal.paidByID,
			RecordedByID:          proposal.recordedByID,
			ExpectedAllocations:   expected.shares,
			AcknowledgeAsAdvancer: acknowledge,
		}, func() ([]moneysteps.Member, error) {
			return rosterOf(ctx, store, identity.ContextID)
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, refuseMoney(refused)
		}
		rollups, err := storedRollups(plan)
		if err != nil {
			return endpoint.Reply{}, err
		}
		stored, err := proposal.stored()
		if err != nil {
			return endpoint.Reply{}, err
		}
		allocations, err := expected.stored()
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.SaveExpenseConfirmation(ctx, repo.ExpenseConfirmation{
			ExpenseID:            expenseID,
			Proposal:             stored,
			AllocatorWarnings:    plan.Allocation.Warnings,
			Rollups:              rollups,
			Allocations:          allocations,
			ConfirmedByID:        call.Actor.ID,
			PayerAcknowledgement: plan.PayerAcknowledgement,
			Now:                  time.Now().UTC(),
		})
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(conflict.Code), "Expense confirmation conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		echoed := pyjson.NewOrderedMap()
		for _, entry := range expected.entries {
			echoed.Set(entry.participantID, entry.amount)
		}
		out := pyjson.NewOrderedMap()
		out.Set("expense_id", pyjson.String(expenseID))
		out.Set("expense_version_id", pyjson.String(record.ExpenseVersionID))
		out.Set("version_number", pyjson.NewInt(record.VersionNumber))
		out.Set("total_amount_vnd", proposal.total)
		out.Set("payer_acknowledgement", pyjson.String(plan.PayerAcknowledgement))
		out.Set("allocations", echoed)
		return endpoint.Reply{Body: out}, nil
	}}
}

// expenseInput is a validated ExpenseInput as the service reads it.
type expenseInput struct {
	contextID    string
	recordedByID string
	paidByID     string
	// expense is _allocator_input(proposal): every amount saturated into
	// money.VND, which changes no answer (allocator.Saturate).
	expense allocator.Expense
	// total is total_amount_vnd exactly, for the response.
	total pyjson.Int
	model *pyval.Model
}

func readExpenseInput(model *pyval.Model) (expenseInput, error) {
	in := expenseInput{model: model}
	var err error
	if in.contextID, err = uuidField(model, "context_id"); err != nil {
		return in, err
	}
	if in.recordedByID, err = uuidField(model, "recorded_by_id"); err != nil {
		return in, err
	}
	if in.paidByID, err = uuidField(model, "paid_by_id"); err != nil {
		return in, err
	}
	participants, err := uuidListField(model, "participants")
	if err != nil {
		return in, err
	}
	if in.total, err = intField(model, "total_amount_vnd"); err != nil {
		return in, err
	}
	paidBy := in.paidByID
	in.expense = allocator.Expense{
		Participants: participants,
		TotalVND:     allocator.Saturate(in.total.Big()),
		Items:        []allocator.Item{},
		Surcharges:   []allocator.Surcharge{},
		Discounts:    []allocator.Discount{},
		AdvancerID:   &paidBy,
	}
	items, err := modelListField(model, "items")
	if err != nil {
		return in, err
	}
	for _, item := range items {
		id, err := stringField(item, "item_id")
		if err != nil {
			return in, err
		}
		amount, err := intField(item, "amount_vnd")
		if err != nil {
			return in, err
		}
		sharedBy, err := uuidListField(item, "shared_by")
		if err != nil {
			return in, err
		}
		in.expense.Items = append(in.expense.Items, allocator.Item{ItemID: id, AmountVND: allocator.Saturate(amount.Big()), SharedBy: sharedBy})
	}
	surcharges, err := modelListField(model, "surcharges")
	if err != nil {
		return in, err
	}
	for _, surcharge := range surcharges {
		id, err := stringField(surcharge, "surcharge_id")
		if err != nil {
			return in, err
		}
		kind, err := stringField(surcharge, "kind")
		if err != nil {
			return in, err
		}
		amount, err := intField(surcharge, "amount_vnd")
		if err != nil {
			return in, err
		}
		mode, err := stringField(surcharge, "mode")
		if err != nil {
			return in, err
		}
		in.expense.Surcharges = append(in.expense.Surcharges, allocator.Surcharge{SurchargeID: id, Kind: kind, AmountVND: allocator.Saturate(amount.Big()), Mode: mode})
	}
	discounts, err := modelListField(model, "discounts")
	if err != nil {
		return in, err
	}
	for _, discount := range discounts {
		id, err := stringField(discount, "discount_id")
		if err != nil {
			return in, err
		}
		amount, err := intField(discount, "amount_vnd")
		if err != nil {
			return in, err
		}
		scope, err := stringField(discount, "scope")
		if err != nil {
			return in, err
		}
		itemID, err := optionalStringField(discount, "item_id")
		if err != nil {
			return in, err
		}
		in.expense.Discounts = append(in.expense.Discounts, allocator.Discount{DiscountID: id, AmountVND: allocator.Saturate(amount.Big()), Scope: scope, ItemID: itemID})
	}
	return in, nil
}

// stored is the proposal save_expense_confirmation writes. It is read only
// after the allocator accepted every amount, so each lies in [0, 10**12].
func (in expenseInput) stored() (repo.ExpenseProposal, error) {
	model := in.model
	description, err := optionalStringField(model, "description")
	if err != nil {
		return repo.ExpenseProposal{}, err
	}
	scope, err := stringField(model, "verification_scope")
	if err != nil {
		return repo.ExpenseProposal{}, err
	}
	occurredAt, err := dateTimeField(model, "occurred_at")
	if err != nil {
		return repo.ExpenseProposal{}, err
	}
	items, err := modelListField(model, "items")
	if err != nil {
		return repo.ExpenseProposal{}, err
	}
	out := repo.ExpenseProposal{
		Description:       description,
		RecordedByID:      in.recordedByID,
		PaidByID:          in.paidByID,
		VerificationScope: scope,
		OccurredAt:        occurredAt.Time(),
	}
	for i, item := range items {
		label, err := optionalStringField(item, "label")
		if err != nil {
			return repo.ExpenseProposal{}, err
		}
		line := in.expense.Items[i]
		out.Items = append(out.Items, repo.ExpenseItemInput{ItemID: line.ItemID, Label: label, AmountVND: int64(line.AmountVND), SharedBy: line.SharedBy})
	}
	for _, s := range in.expense.Surcharges {
		out.Surcharges = append(out.Surcharges, repo.ExpenseSurchargeInput{SurchargeID: s.SurchargeID, Kind: s.Kind, AmountVND: int64(s.AmountVND), Mode: s.Mode})
	}
	for _, d := range in.expense.Discounts {
		out.Discounts = append(out.Discounts, repo.ExpenseDiscountInput{DiscountID: d.DiscountID, AmountVND: int64(d.AmountVND), Scope: d.Scope, ItemID: d.ItemID})
	}
	return out, nil
}

// expectedAllocations is ExpenseConfirmationRequest.expected_allocations: the
// dict as validated (two spellings of one id are one key, first position,
// last value), exactly, and saturated for the comparison.
type expectedAllocations struct {
	entries []expectedEntry
	shares  []allocator.Share
}

type expectedEntry struct {
	participantID string
	amount        pyjson.Int
}

func readExpectedAllocations(body *pyval.Model) (expectedAllocations, error) {
	value, err := field(body, "expected_allocations")
	if err != nil {
		return expectedAllocations{}, err
	}
	dict, ok := value.(*pyval.Dict)
	if !ok {
		return expectedAllocations{}, fmt.Errorf("routes: %s.expected_allocations is %T, not a dict", body.Class, value)
	}
	out := expectedAllocations{shares: []allocator.Share{}}
	for i := 0; i < dict.Len(); i++ {
		key, val := dict.Entry(i)
		id, ok := key.(pyval.UUID)
		if !ok {
			return expectedAllocations{}, fmt.Errorf("routes: expected_allocations key %T is not a UUID", key)
		}
		amount, ok := val.(pyjson.Int)
		if !ok {
			return expectedAllocations{}, fmt.Errorf("routes: expected_allocations value %T is not an int", val)
		}
		out.entries = append(out.entries, expectedEntry{participantID: id.String(), amount: amount})
		out.shares = append(out.shares, allocator.Share{ParticipantID: id.String(), AmountVND: allocator.Saturate(amount.Big())})
	}
	return out, nil
}

// stored is the allocations to write, read once they equal the allocator's,
// so every amount fits int64.
func (e expectedAllocations) stored() ([]repo.ParticipantAmount, error) {
	out := make([]repo.ParticipantAmount, len(e.entries))
	for i, entry := range e.entries {
		amount, ok := entry.amount.Int64()
		if !ok {
			return nil, fmt.Errorf("routes: a confirmed allocation of %s passes int64", entry.amount)
		}
		out[i] = repo.ParticipantAmount{ParticipantID: entry.participantID, AmountVND: amount}
	}
	return out, nil
}

// storedRollups narrows component_rollups for BIGINT columns. The allocator
// bounds every line, so only an absurd number of lines could pass int64;
// Python's INSERT would fail there, and so does this request.
func storedRollups(plan moneysteps.ConfirmationPlan) (repo.ExpenseRollups, error) {
	r := plan.Rollups
	out := repo.ExpenseRollups{TotalVND: int64(r.TotalAmountVND)}
	for _, pair := range []struct {
		dst *int64
		sum *big.Int
	}{{&out.SubtotalVND, r.SubtotalAmountVND}, {&out.FeeVND, r.FeeAmountVND}, {&out.VATVND, r.VATAmountVND},
		{&out.ShippingVND, r.ShippingAmountVND}, {&out.DiscountVND, r.DiscountAmountVND}} {
		if !pair.sum.IsInt64() {
			return repo.ExpenseRollups{}, fmt.Errorf("routes: an expense rollup of %s passes BIGINT", pair.sum)
		}
		*pair.dst = pair.sum.Int64()
	}
	return out, nil
}

// rosterOf is list_members as the pure steps read it: who, and in what state.
func rosterOf(ctx context.Context, store repo.Repository, contextID string) ([]moneysteps.Member, error) {
	members, err := store.ListMembers(ctx, contextID)
	if err != nil {
		return nil, err
	}
	roster := make([]moneysteps.Member, len(members))
	for i, member := range members {
		roster[i] = moneysteps.Member{PersonID: member.PersonID, State: member.State}
	}
	return roster, nil
}

// wireAllocation is _wire_allocation: AllocationProposal, each dict in the
// order of the participants.
func wireAllocation(result allocator.Result) *pyjson.OrderedMap {
	allocations := pyjson.NewOrderedMap()
	for _, share := range result.Allocations {
		allocations.Set(share.ParticipantID, pyjson.NewInt(int64(share.AmountVND)))
	}
	exact := pyjson.NewOrderedMap()
	for _, share := range result.ExactShares {
		exact.Set(share.ParticipantID, pyjson.String(share.Fraction()))
	}
	gainers := make(pyjson.List, len(result.RoundingGainers))
	for i, id := range result.RoundingGainers {
		gainers[i] = pyjson.String(id)
	}
	warnings := make(pyjson.List, len(result.Warnings))
	for i, warning := range result.Warnings {
		warnings[i] = pyjson.String(warning)
	}
	out := pyjson.NewOrderedMap()
	out.Set("allocations", allocations)
	out.Set("exact_shares", exact)
	out.Set("rounding_gainers", gainers)
	out.Set("warnings", warnings)
	return out
}
