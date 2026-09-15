package routes

// The scanned-bill drafts of routes/bills.py: POST /bills, GET
// /bills/{bill_id}, PUT /bills/{bill_id}/assignments, POST
// /bills/{bill_id}/my-items and POST /bills/{bill_id}/split. A draft never
// reaches the ledger; the split is a preview.

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"time"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/billdraft"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
)

// billPermission is the action every bill door checks.
const billPermission = "confirm_expense_proposal"

// createBill is POST /bills (ApiService.create_bill): role and ACTIVE
// membership of the body's group, every suggested person an ACTIVE member, the
// declared items total equal to the lines, then the draft in one savepoint; a
// constraint is a 409 with the repository's code in capitals.
func createBill() Route {
	return Route{ID: "POST /bills", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		draft, err := readBillDraft(body, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, billPermission, draft.in.ContextID); err != nil {
			return endpoint.Reply{}, err
		}
		refused, err := moneysteps.CreateBillChecks(draft.itemsTotal, draft.lines, func() ([]moneysteps.Member, error) {
			return billRoster(ctx, store, draft.in.ContextID)
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, billRefusal(refused)
		}
		draft.in.Now = time.Now().UTC()
		record, err := writeBill(ctx, store, draft)
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			return endpoint.Reply{}, endpoint.Refuse(409, conflict.Code, "Bill creation conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireBill(record)}, nil
	}}
}

// getBill is GET /bills/{bill_id} (get_bill): 404 before the permission check.
func getBill() Route {
	return Route{ID: "GET /bills/{bill_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		billID, err := pathUUID(call, "bill_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := billForActor(ctx, call, store, billID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireBill(*record)}, nil
	}}
}

// confirmBillAssignments is PUT /bills/{bill_id}/assignments
// (confirm_bill_assignments): the bill (404, then 403), every named person an
// ACTIVE member (422), then the locked rewrite. An unknown item is 409
// UNKNOWN_BILL_ITEM; a person named twice on one item is the unique violation
// Python does not catch, a 500.
func confirmBillAssignments() Route {
	return Route{ID: "PUT /bills/{bill_id}/assignments", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		billID, err := pathUUID(call, "bill_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows, err := modelListField(body, "assignments")
		if err != nil {
			return endpoint.Reply{}, err
		}
		assignments := make([]repo.BillAssignment, len(rows))
		var named []string
		for i, row := range rows {
			if assignments[i].ItemKey, err = stringField(row, "item_key"); err != nil {
				return endpoint.Reply{}, err
			}
			if assignments[i].ParticipantIDs, err = billUUIDs(row, "participant_ids"); err != nil {
				return endpoint.Reply{}, err
			}
			named = append(named, assignments[i].ParticipantIDs...)
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := billForActor(ctx, call, store, billID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		members, err := billRoster(ctx, store, record.ContextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused := moneysteps.RequireParticipantsAreMembers(members, named); refused != nil {
			return endpoint.Reply{}, billRefusal(refused)
		}
		updated, err := store.ConfirmBillAssignments(ctx, billID, assignments, call.Actor.ID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			if conflict.Code == "BILL_NOT_FOUND" {
				return endpoint.Reply{}, billNotFound()
			}
			return endpoint.Reply{}, endpoint.Refuse(409, conflict.Code, "Bill assignment conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireBill(updated)}, nil
	}}
}

// claimBillItems is POST /bills/{bill_id}/my-items (claim_bill_items): the
// caller's whole set of claims; no roster check, because the body names
// nobody. An unknown item is 422 unknown_bill_item here.
func claimBillItems() Route {
	return Route{ID: "POST /bills/{bill_id}/my-items", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		billID, err := pathUUID(call, "bill_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		keys, err := stringListField(body, "item_keys")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if _, err := billForActor(ctx, call, store, billID); err != nil {
			return endpoint.Reply{}, err
		}
		updated, err := store.ClaimBillItems(ctx, billID, call.Actor.ID, keys, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			switch conflict.Code {
			case "BILL_NOT_FOUND":
				return endpoint.Reply{}, billNotFound()
			case "UNKNOWN_BILL_ITEM":
				return endpoint.Reply{}, endpoint.Refuse(422, "unknown_bill_item", "Bill does not contain one of these items")
			}
			return endpoint.Reply{}, endpoint.Refuse(409, conflict.Code, "Bill assignment conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireBill(updated)}, nil
	}}
}

// splitBill is POST /bills/{bill_id}/split (split_bill): the bill (404, then
// 403), the roster read once, then the projection, the ledger gate and the
// allocator, all in moneysteps.SplitBill. Nothing is written.
func splitBill() Route {
	return Route{ID: "POST /bills/{bill_id}/split", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		billID, err := pathUUID(call, "bill_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		forLedger, err := billBool(body, "for_ledger")
		if err != nil {
			return endpoint.Reply{}, err
		}
		paidByID, err := optionalUUIDField(body, "paid_by_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := billForActor(ctx, call, store, billID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		members, err := billRoster(ctx, store, record.ContextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		split, refused, err := moneysteps.SplitBill(splitRecord(*record), forLedger, paidByID, members)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, billRefusal(refused)
		}
		return endpoint.Reply{Body: wireBillSplit(split)}, nil
	}}
}

// billForActor is _bill_for_actor: the bill first, so a missing one is 404
// for anybody, then the permission on the bill's own group.
func billForActor(ctx context.Context, call *endpoint.Call, store repo.Repository, billID string) (*repo.Bill, error) {
	record, err := store.GetBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, billNotFound()
	}
	if err := requireGroupMember(ctx, call, store, billPermission, record.ContextID); err != nil {
		return nil, err
	}
	return record, nil
}

func billNotFound() error {
	return endpoint.Refuse(404, "bill_not_found", "Bill does not exist")
}

func billRefusal(refused *moneysteps.Refusal) error {
	return endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
}

// billRoster is list_members as the money steps read it.
func billRoster(ctx context.Context, store repo.Repository, contextID string) ([]moneysteps.Member, error) {
	rows, err := store.ListMembers(ctx, contextID)
	if err != nil {
		return nil, err
	}
	members := make([]moneysteps.Member, len(rows))
	for i, row := range rows {
		members[i] = moneysteps.Member{PersonID: row.PersonID, State: row.State}
	}
	return members, nil
}

// billFlush is one statement group of repository.create_bill, in the order it
// reaches the database: the bill, the items, then in mapper order the
// discounts, the suggested shares (no amount) and the surcharges.
type billFlush int

const (
	flushNothing billFlush = iota
	flushBill
	flushItems
	flushDiscounts
	flushSurcharges
)

// errPastColumn stands for the psycopg DataError (22003) PostgreSQL raises on
// a value its column cannot hold. create_bill catches only IntegrityError, so
// Python answers it with a 500.
var errPastColumn = errors.New("routes: a bill value is past the range of its column")

// billDraft is a validated BillCreateRequest. in holds every value that fits
// int64; past is the first flush carrying one that does not, whose slot in
// in is zero.
type billDraft struct {
	in         repo.BillInput
	itemsTotal *big.Int
	lines      []moneysteps.BillLine
	past       billFlush
}

// column is n for a column of the given flush, noting the flush when n does
// not fit int64 (so no BIGINT or INTEGER column holds it).
func (d *billDraft) column(flush billFlush, n pyjson.Int) int64 {
	v, ok := n.Int64()
	if !ok && (d.past == flushNothing || flush < d.past) {
		d.past = flush
	}
	return v
}

func readBillDraft(body *pyval.Model, actorID string) (billDraft, error) {
	d := billDraft{in: repo.BillInput{CreatedByID: actorID}}
	var err error
	if d.in.ContextID, err = uuidField(body, "context_id"); err != nil {
		return d, err
	}
	printed, err := billOptionalInt(body, "printed_total_vnd")
	if err != nil {
		return d, err
	}
	if printed != nil {
		v := d.column(flushBill, *printed)
		d.in.PrintedTotalVND = &v
	}
	itemsTotal, err := billInt(body, "items_total_vnd")
	if err != nil {
		return d, err
	}
	d.itemsTotal = itemsTotal.Big()
	d.in.ItemsTotalVND = d.column(flushBill, itemsTotal)
	confidence, err := billInt(body, "confidence")
	if err != nil {
		return d, err
	}
	d.in.Confidence = d.column(flushBill, confidence)
	if d.in.NeedsReview, err = billBool(body, "needs_review"); err != nil {
		return d, err
	}

	items, err := modelListField(body, "items")
	if err != nil {
		return d, err
	}
	d.in.Items = make([]repo.BillItemInput, len(items))
	d.lines = make([]moneysteps.BillLine, len(items))
	for position, row := range items {
		item := repo.BillItemInput{Position: int64(position)}
		if item.ItemKey, err = stringField(row, "item_key"); err != nil {
			return d, err
		}
		if item.Name, err = stringField(row, "name"); err != nil {
			return d, err
		}
		quantity, err := billInt(row, "quantity")
		if err != nil {
			return d, err
		}
		item.Quantity = d.column(flushItems, quantity)
		unitPrice, err := billOptionalInt(row, "unit_price_vnd")
		if err != nil {
			return d, err
		}
		if unitPrice != nil {
			v := d.column(flushItems, *unitPrice)
			item.UnitPriceVND = &v
		}
		lineTotal, err := billInt(row, "line_total_vnd")
		if err != nil {
			return d, err
		}
		item.LineTotalVND = d.column(flushItems, lineTotal)
		if item.SuggestedParticipantIDs, err = billUUIDs(row, "suggested_participant_ids"); err != nil {
			return d, err
		}
		d.in.Items[position] = item
		d.lines[position] = moneysteps.BillLine{LineTotalVND: lineTotal.Big(), SuggestedParticipantIDs: item.SuggestedParticipantIDs}
	}

	surcharges, err := modelListField(body, "surcharges")
	if err != nil {
		return d, err
	}
	d.in.Surcharges = make([]repo.BillSurcharge, len(surcharges))
	for i, row := range surcharges {
		s := &d.in.Surcharges[i]
		if s.SurchargeKey, err = stringField(row, "surcharge_key"); err != nil {
			return d, err
		}
		if s.Kind, err = stringField(row, "kind"); err != nil {
			return d, err
		}
		amount, err := billInt(row, "amount_vnd")
		if err != nil {
			return d, err
		}
		s.AmountVND = d.column(flushSurcharges, amount)
		if s.Mode, err = stringField(row, "mode"); err != nil {
			return d, err
		}
	}

	discounts, err := modelListField(body, "discounts")
	if err != nil {
		return d, err
	}
	d.in.Discounts = make([]repo.BillDiscount, len(discounts))
	for i, row := range discounts {
		disc := &d.in.Discounts[i]
		if disc.DiscountKey, err = stringField(row, "discount_key"); err != nil {
			return d, err
		}
		amount, err := billInt(row, "amount_vnd")
		if err != nil {
			return d, err
		}
		disc.AmountVND = d.column(flushDiscounts, amount)
		if disc.Scope, err = stringField(row, "scope"); err != nil {
			return d, err
		}
		if disc.TargetItemKey, err = optionalStringField(row, "item_key"); err != nil {
			return d, err
		}
	}
	return d, nil
}

// writeBill is repository.create_bill. Python binds every value and the
// database refuses one past its column in the statement that carries it,
// before inserting any row of that statement; statements before it may still
// fail on a constraint first, which is the 409. Go cannot bind such a value,
// so it runs create_bill without that statement and what follows it, answers
// a conflict as a conflict, and otherwise ends with errPastColumn. The
// endpoint rolls the transaction back either way, as Python does.
func writeBill(ctx context.Context, store repo.Repository, d billDraft) (repo.Bill, error) {
	in := d.in
	switch d.past {
	case flushNothing:
		return store.CreateBill(ctx, in)
	case flushBill:
		return repo.Bill{}, errPastColumn
	case flushItems:
		in.Items, in.Discounts, in.Surcharges = nil, nil, nil
	case flushDiscounts:
		in.Discounts, in.Surcharges = nil, nil
		in.Items = slices.Clone(in.Items)
		for i := range in.Items {
			in.Items[i].SuggestedParticipantIDs = nil
		}
	case flushSurcharges:
		in.Surcharges = nil
	default:
		return repo.Bill{}, fmt.Errorf("routes: bill flush %d", d.past)
	}
	if _, err := store.CreateBill(ctx, in); err != nil {
		return repo.Bill{}, err
	}
	return repo.Bill{}, errPastColumn
}

// splitRecord is the bill dict split_bill hands allocator_input_from_bill.
func splitRecord(record repo.Bill) moneysteps.BillRecord {
	out := moneysteps.BillRecord{
		Items:      make([]billdraft.Item, len(record.Items)),
		Surcharges: make([]allocator.Surcharge, len(record.Surcharges)),
		Discounts:  make([]allocator.Discount, len(record.Discounts)),
	}
	if record.PrintedTotalVND != nil {
		printed := money.VND(*record.PrintedTotalVND)
		out.PrintedTotalVND = &printed
	}
	for i, item := range record.Items {
		shares := make([]billdraft.Share, len(item.Shares))
		for j, share := range item.Shares {
			shares[j] = billdraft.Share{ParticipantID: share.ParticipantID, Source: share.Source}
		}
		out.Items[i] = billdraft.Item{ItemKey: item.ItemKey, AmountVND: money.VND(item.LineTotalVND), Shares: shares}
	}
	for i, s := range record.Surcharges {
		out.Surcharges[i] = allocator.Surcharge{SurchargeID: s.SurchargeKey, Kind: s.Kind, AmountVND: money.VND(s.AmountVND), Mode: s.Mode}
	}
	for i, disc := range record.Discounts {
		out.Discounts[i] = allocator.Discount{DiscountID: disc.DiscountKey, AmountVND: money.VND(disc.AmountVND), Scope: disc.Scope, ItemID: disc.TargetItemKey}
	}
	return out
}

// wireBill is _wire_bill: BillResponse in declaration order, its assignment
// state and suggested keys derived from the shares on every read.
func wireBill(record repo.Bill) *pyjson.OrderedMap {
	sources := make([]moneysteps.BillItemSources, len(record.Items))
	for i, item := range record.Items {
		sources[i] = moneysteps.BillItemSources{ItemKey: item.ItemKey, Sources: make([]string, len(item.Shares))}
		for j, share := range item.Shares {
			sources[i].Sources[j] = share.Source
		}
	}
	state, suggested := moneysteps.BillAssignment(sources)

	items := make(pyjson.List, len(record.Items))
	for i, item := range record.Items {
		shares := make(pyjson.List, len(item.Shares))
		for j, share := range item.Shares {
			wire := pyjson.NewOrderedMap()
			wire.Set("participant_id", pyjson.String(share.ParticipantID))
			wire.Set("source", pyjson.String(share.Source))
			wire.Set("decided_by_id", textOrNull(share.DecidedByID))
			wire.Set("decided_at", instantOrNull(share.DecidedAt))
			shares[j] = wire
		}
		wire := pyjson.NewOrderedMap()
		wire.Set("item_key", pyjson.String(item.ItemKey))
		wire.Set("name", pyjson.String(item.Name))
		wire.Set("quantity", pyjson.NewInt(item.Quantity))
		wire.Set("unit_price_vnd", billIntOrNull(item.UnitPriceVND))
		wire.Set("line_total_vnd", pyjson.NewInt(item.LineTotalVND))
		wire.Set("position", pyjson.NewInt(item.Position))
		wire.Set("shares", shares)
		items[i] = wire
	}
	surcharges := make(pyjson.List, len(record.Surcharges))
	for i, s := range record.Surcharges {
		wire := pyjson.NewOrderedMap()
		wire.Set("surcharge_key", pyjson.String(s.SurchargeKey))
		wire.Set("kind", pyjson.String(s.Kind))
		wire.Set("amount_vnd", pyjson.NewInt(s.AmountVND))
		wire.Set("mode", pyjson.String(s.Mode))
		surcharges[i] = wire
	}
	discounts := make(pyjson.List, len(record.Discounts))
	for i, disc := range record.Discounts {
		wire := pyjson.NewOrderedMap()
		wire.Set("discount_key", pyjson.String(disc.DiscountKey))
		wire.Set("amount_vnd", pyjson.NewInt(disc.AmountVND))
		wire.Set("scope", pyjson.String(disc.Scope))
		wire.Set("item_key", textOrNull(disc.TargetItemKey))
		discounts[i] = wire
	}

	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("context_id", pyjson.String(record.ContextID))
	out.Set("printed_total_vnd", billIntOrNull(record.PrintedTotalVND))
	out.Set("items_total_vnd", pyjson.NewInt(record.ItemsTotalVND))
	out.Set("needs_review", pyjson.Bool(record.NeedsReview))
	out.Set("created_by_id", pyjson.String(record.CreatedByID))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	out.Set("assignment_state", pyjson.String(state))
	out.Set("suggested_item_keys", billStrings(suggested))
	out.Set("items", items)
	out.Set("surcharges", surcharges)
	out.Set("discounts", discounts)
	return out
}

// wireBillSplit is BillSplitResponse, with _wire_allocation: allocations and
// exact shares keyed by participant in the allocator's order.
func wireBillSplit(split moneysteps.Split) *pyjson.OrderedMap {
	allocations := pyjson.NewOrderedMap()
	for _, share := range split.Allocation.Allocations {
		allocations.Set(share.ParticipantID, pyjson.NewInt(int64(share.AmountVND)))
	}
	exact := pyjson.NewOrderedMap()
	for _, share := range split.Allocation.ExactShares {
		exact.Set(share.ParticipantID, pyjson.String(share.Fraction()))
	}
	allocation := pyjson.NewOrderedMap()
	allocation.Set("allocations", allocations)
	allocation.Set("exact_shares", exact)
	allocation.Set("rounding_gainers", billStrings(split.Allocation.RoundingGainers))
	allocation.Set("warnings", billStrings(split.Allocation.Warnings))

	out := pyjson.NewOrderedMap()
	out.Set("allocation", allocation)
	out.Set("assignment_state", pyjson.String(split.AssignmentState))
	out.Set("suggested_item_keys", billStrings(split.SuggestedItemKeys))
	out.Set("total_amount_vnd", pyjson.NewInt(int64(split.TotalAmountVND)))
	out.Set("participant_ids", billStrings(split.ParticipantIDs))
	out.Set("excluded_member_ids", billStrings(split.ExcludedMemberIDs))
	return out
}

func billStrings(values []string) pyjson.List {
	out := make(pyjson.List, len(values))
	for i, value := range values {
		out[i] = pyjson.String(value)
	}
	return out
}

func billIntOrNull(value *int64) pyjson.Value {
	if value == nil {
		return pyjson.Null{}
	}
	return pyjson.NewInt(*value)
}

func billInt(model *pyval.Model, name string) (pyjson.Int, error) {
	value, err := field(model, name)
	if err != nil {
		return pyjson.Int{}, err
	}
	n, ok := value.(pyjson.Int)
	if !ok {
		return pyjson.Int{}, fmt.Errorf("routes: %s.%s is %T, not an int", model.Class, name, value)
	}
	return n, nil
}

func billOptionalInt(model *pyval.Model, name string) (*pyjson.Int, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.Int:
		return &v, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not an int or None", model.Class, name, value)
}

func billBool(model *pyval.Model, name string) (bool, error) {
	value, err := field(model, name)
	if err != nil {
		return false, err
	}
	b, ok := value.(pyjson.Bool)
	if !ok {
		return false, fmt.Errorf("routes: %s.%s is %T, not a bool", model.Class, name, value)
	}
	return bool(b), nil
}

func billUUIDs(model *pyval.Model, name string) ([]string, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	list, ok := value.(pyval.List)
	if !ok {
		return nil, fmt.Errorf("routes: %s.%s is %T, not a list", model.Class, name, value)
	}
	out := make([]string, len(list))
	for i, item := range list {
		id, ok := item.(pyval.UUID)
		if !ok {
			return nil, fmt.Errorf("routes: %s.%s[%d] is %T, not a UUID", model.Class, name, i, item)
		}
		out[i] = id.String()
	}
	return out, nil
}
