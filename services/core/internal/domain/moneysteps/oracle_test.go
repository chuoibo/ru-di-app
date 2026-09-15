package moneysteps

import (
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/billdraft"
	"mobile/services/core/internal/domain/budget"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_money_steps*.json is rendered by
// scripts/render_domain_w4_goldens.py by running the real app.api.service
// methods over a recording stub repository, with the clock pinned, in the
// parity API image. Every case is replayed here, including which repository
// calls the method made. The reads that open a method (and the permission
// they feed) belong to the route; they are the constant prefix of calls.

// harnessContext is CONTEXT in the render script.
const harnessContext = "c0c0c0c0-cdcd-4ece-8fcf-dadadadadada"

var harnessNow = time.Date(2026, 9, 15, 9, 30, 12, 0, time.UTC).Add(345678 * time.Microsecond)

func problemOf(r *Refusal) any {
	if r == nil {
		return nil
	}
	return map[string]any{"status": int64(r.Status), "code": r.Code, "detail": r.Detail}
}

func vnd(value any) (money.VND, error) {
	n, err := oracletest.Int64(value)
	return money.VND(n), err
}

// requestAmount reads an amount a request carries, of any size, saturated.
func requestAmount(value any) (money.VND, error) {
	n, err := oracletest.Integer(value)
	if err != nil || n == nil {
		return 0, fmt.Errorf("%v is not an int: %v", value, err)
	}
	return allocator.Saturate(n), nil
}

func membersOf(value any) ([]Member, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, raw := range rows {
		pair, err := oracletest.Strings(raw)
		if err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("member %v is not [id, state]", raw)
		}
		out[i] = Member{PersonID: pair[0], State: pair[1]}
	}
	return out, nil
}

func expenseOf(value any) (allocator.Expense, error) {
	m, err := oracletest.Row(value, "participants", "total_vnd", "items", "surcharges", "discounts", "advancer_id")
	if err != nil {
		return allocator.Expense{}, err
	}
	var e allocator.Expense
	if e.Participants, err = oracletest.Strings(m["participants"]); err != nil {
		return e, err
	}
	if e.TotalVND, err = requestAmount(m["total_vnd"]); err != nil {
		return e, err
	}
	if e.AdvancerID, err = oracletest.OptionalString(m["advancer_id"]); err != nil {
		return e, err
	}
	items, err := oracletest.List(m["items"])
	if err != nil {
		return e, err
	}
	for _, raw := range items {
		r, err := oracletest.Row(raw, "item_id", "amount_vnd", "shared_by")
		if err != nil {
			return e, err
		}
		var it allocator.Item
		if it.ItemID, err = oracletest.Str(r["item_id"]); err != nil {
			return e, err
		}
		if it.AmountVND, err = requestAmount(r["amount_vnd"]); err != nil {
			return e, err
		}
		if it.SharedBy, err = oracletest.Strings(r["shared_by"]); err != nil {
			return e, err
		}
		e.Items = append(e.Items, it)
	}
	surcharges, err := oracletest.List(m["surcharges"])
	if err != nil {
		return e, err
	}
	for _, raw := range surcharges {
		r, err := oracletest.Row(raw, "surcharge_id", "kind", "amount_vnd", "mode")
		if err != nil {
			return e, err
		}
		s, err := surchargeOf(r, "surcharge_id", requestAmount)
		if err != nil {
			return e, err
		}
		e.Surcharges = append(e.Surcharges, s)
	}
	discounts, err := oracletest.List(m["discounts"])
	if err != nil {
		return e, err
	}
	for _, raw := range discounts {
		r, err := oracletest.Row(raw, "discount_id", "amount_vnd", "scope", "item_id")
		if err != nil {
			return e, err
		}
		d, err := discountOf(r, "discount_id", "item_id", requestAmount)
		if err != nil {
			return e, err
		}
		e.Discounts = append(e.Discounts, d)
	}
	return e, nil
}

func surchargeOf(r map[string]any, idKey string, amount func(any) (money.VND, error)) (allocator.Surcharge, error) {
	var s allocator.Surcharge
	var err error
	if s.SurchargeID, err = oracletest.Str(r[idKey]); err != nil {
		return s, err
	}
	if s.Kind, err = oracletest.Str(r["kind"]); err != nil {
		return s, err
	}
	if s.AmountVND, err = amount(r["amount_vnd"]); err != nil {
		return s, err
	}
	s.Mode, err = oracletest.Str(r["mode"])
	return s, err
}

func discountOf(r map[string]any, idKey, targetKey string, amount func(any) (money.VND, error)) (allocator.Discount, error) {
	var d allocator.Discount
	var err error
	if d.DiscountID, err = oracletest.Str(r[idKey]); err != nil {
		return d, err
	}
	if d.AmountVND, err = amount(r["amount_vnd"]); err != nil {
		return d, err
	}
	if d.Scope, err = oracletest.Str(r["scope"]); err != nil {
		return d, err
	}
	d.ItemID, err = oracletest.OptionalString(r[targetKey])
	return d, err
}

func renderAllocation(r allocator.Result) map[string]any {
	allocations := make([]any, len(r.Allocations))
	for i, share := range r.Allocations {
		allocations[i] = []any{share.ParticipantID, int64(share.AmountVND)}
	}
	shares := make([]any, len(r.ExactShares))
	for i, share := range r.ExactShares {
		shares[i] = []any{share.ParticipantID, share.Fraction()}
	}
	return map[string]any{
		"allocations":      allocations,
		"exact_shares":     shares,
		"rounding_gainers": oracletest.AnyStrings(r.RoundingGainers),
		"warnings":         oracletest.AnyStrings(r.Warnings),
	}
}

func stamp(value any) (time.Time, error) {
	s, err := oracletest.Str(value)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, s)
}

func batchInputsOf(value any) (BatchInputs, error) {
	m, err := oracletest.Row(value, "expenses", "unavailable_version_ids")
	if err != nil {
		return BatchInputs{}, err
	}
	var inputs BatchInputs
	if inputs.UnavailableVersionIDs, err = oracletest.Strings(m["unavailable_version_ids"]); err != nil {
		return inputs, err
	}
	expenses, err := oracletest.List(m["expenses"])
	if err != nil {
		return inputs, err
	}
	for _, raw := range expenses {
		r, err := oracletest.Row(raw, "version_id", "paid_by_id", "allocations")
		if err != nil {
			return inputs, err
		}
		var e BatchExpense
		if e.VersionID, err = oracletest.Str(r["version_id"]); err != nil {
			return inputs, err
		}
		if e.PaidByID, err = oracletest.Str(r["paid_by_id"]); err != nil {
			return inputs, err
		}
		rows, err := oracletest.List(r["allocations"])
		if err != nil {
			return inputs, err
		}
		for _, rawRow := range rows {
			triple, err := oracletest.List(rawRow)
			if err != nil || len(triple) != 3 {
				return inputs, fmt.Errorf("row %v is not [id, participant, amount]", rawRow)
			}
			var row AllocationRow
			if row.ID, err = oracletest.Str(triple[0]); err != nil {
				return inputs, err
			}
			if row.ParticipantID, err = oracletest.Str(triple[1]); err != nil {
				return inputs, err
			}
			if row.AmountVND, err = vnd(triple[2]); err != nil {
				return inputs, err
			}
			e.Allocations = append(e.Allocations, row)
		}
		inputs.Expenses = append(inputs.Expenses, e)
	}
	return inputs, nil
}

func renderRows(rows []AllocationRow) []any {
	out := make([]any, len(rows))
	for i, row := range rows {
		out[i] = []any{row.ID, row.ParticipantID, int64(row.AmountVND)}
	}
	return out
}

func decodeFailed(err error) (any, error) { return nil, oracletest.Decode(err) }

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "propose_expense":
		e, err := expenseOf(args["expense"])
		if err != nil {
			return decodeFailed(err)
		}
		result, refused, err := ProposeExpense(e)
		if err != nil {
			return nil, err
		}
		if refused != nil {
			return map[string]any{"calls": []any{}, "problem": problemOf(refused), "allocation": nil}, nil
		}
		return map[string]any{"calls": []any{"create_expense"}, "problem": nil, "allocation": renderAllocation(result)}, nil

	case "confirm_expense":
		e, err := expenseOf(args["expense"])
		if err != nil || e.AdvancerID == nil {
			return decodeFailed(fmt.Errorf("expense %v: %v", args["expense"], err))
		}
		in := ExpenseConfirmation{Expense: e, PaidByID: *e.AdvancerID}
		if in.ActorID, err = oracletest.Str(args["actor_id"]); err != nil {
			return decodeFailed(err)
		}
		if in.ActorRoles, err = oracletest.Strings(args["actor_roles"]); err != nil {
			return decodeFailed(err)
		}
		if in.RecordedByID, err = oracletest.Str(args["recorded_by_id"]); err != nil {
			return decodeFailed(err)
		}
		if in.AcknowledgeAsAdvancer, err = oracletest.Bool(args["acknowledge_as_advancer"]); err != nil {
			return decodeFailed(err)
		}
		expected, err := oracletest.List(args["expected_allocations"])
		if err != nil {
			return decodeFailed(err)
		}
		for _, raw := range expected {
			pair, err := oracletest.List(raw)
			if err != nil || len(pair) != 2 {
				return decodeFailed(fmt.Errorf("expected allocation %v", raw))
			}
			var share allocator.Share
			if share.ParticipantID, err = oracletest.Str(pair[0]); err != nil {
				return decodeFailed(err)
			}
			if share.AmountVND, err = requestAmount(pair[1]); err != nil {
				return decodeFailed(err)
			}
			in.ExpectedAllocations = append(in.ExpectedAllocations, share)
		}
		roster, err := membersOf(args["roster"])
		if err != nil {
			return decodeFailed(err)
		}
		calls := []any{"get_expense", "is_member"}
		plan, refused, err := ConfirmExpense(in, func() ([]Member, error) {
			calls = append(calls, "list_members")
			return roster, nil
		})
		if err != nil {
			return nil, err
		}
		if refused != nil {
			return map[string]any{"calls": calls, "problem": problemOf(refused), "saved": nil}, nil
		}
		r := plan.Rollups
		return map[string]any{
			"calls":   append(calls, "save_expense_confirmation"),
			"problem": nil,
			"saved": map[string]any{
				"payer_acknowledgement": plan.PayerAcknowledgement,
				"allocation":            renderAllocation(plan.Allocation),
				"rollups": map[string]any{
					"subtotal_amount_vnd": oracletest.Exact(r.SubtotalAmountVND),
					"fee_amount_vnd":      oracletest.Exact(r.FeeAmountVND),
					"vat_amount_vnd":      oracletest.Exact(r.VATAmountVND),
					"shipping_amount_vnd": oracletest.Exact(r.ShippingAmountVND),
					"discount_amount_vnd": oracletest.Exact(r.DiscountAmountVND),
					"total_amount_vnd":    int64(r.TotalAmountVND),
				},
			},
		}, nil

	case "create_bill":
		total, err := vnd(args["items_total_vnd"])
		if err != nil {
			return decodeFailed(err)
		}
		rawLines, err := oracletest.List(args["lines"])
		if err != nil {
			return decodeFailed(err)
		}
		var lines []BillLine
		for _, raw := range rawLines {
			r, err := oracletest.Row(raw, "line_total_vnd", "suggested_participant_ids")
			if err != nil {
				return decodeFailed(err)
			}
			var line BillLine
			if line.LineTotalVND, err = vnd(r["line_total_vnd"]); err != nil {
				return decodeFailed(err)
			}
			if line.SuggestedParticipantIDs, err = oracletest.Strings(r["suggested_participant_ids"]); err != nil {
				return decodeFailed(err)
			}
			lines = append(lines, line)
		}
		roster, err := membersOf(args["roster"])
		if err != nil {
			return decodeFailed(err)
		}
		calls := []any{"is_member"}
		refused, err := CreateBillChecks(total, lines, func() ([]Member, error) {
			calls = append(calls, "list_members")
			return roster, nil
		})
		if err != nil {
			return nil, err
		}
		if refused == nil {
			calls = append(calls, "create_bill")
		}
		return map[string]any{"calls": calls, "problem": problemOf(refused)}, nil

	case "bill_assignment":
		rows, err := oracletest.List(args["items"])
		if err != nil {
			return decodeFailed(err)
		}
		items := make([]BillItemSources, len(rows))
		for i, raw := range rows {
			r, err := oracletest.Row(raw, "item_key", "sources")
			if err != nil {
				return decodeFailed(err)
			}
			if items[i].ItemKey, err = oracletest.Str(r["item_key"]); err != nil {
				return decodeFailed(err)
			}
			if items[i].Sources, err = oracletest.Strings(r["sources"]); err != nil {
				return decodeFailed(err)
			}
		}
		state, keys := BillAssignment(items)
		return map[string]any{"assignment_state": state, "suggested_item_keys": oracletest.AnyStrings(keys)}, nil

	case "split_bill":
		record, err := billRecordOf(args["bill"])
		if err != nil {
			return decodeFailed(err)
		}
		forLedger, err := oracletest.Bool(args["for_ledger"])
		if err != nil {
			return decodeFailed(err)
		}
		paidBy, err := oracletest.OptionalString(args["paid_by_id"])
		if err != nil {
			return decodeFailed(err)
		}
		roster, err := membersOf(args["roster"])
		if err != nil {
			return decodeFailed(err)
		}
		calls := []any{"get_bill", "is_member", "list_members"}
		split, refused, err := SplitBill(record, forLedger, paidBy, roster)
		var invalid *ResponseError
		if errors.As(err, &invalid) {
			return map[string]any{"calls": calls, "problem": nil, "split": nil, "response_error": true}, nil
		}
		if err != nil {
			return nil, err
		}
		if refused != nil {
			return map[string]any{"calls": calls, "problem": problemOf(refused), "split": nil, "response_error": false}, nil
		}
		return map[string]any{
			"calls":   calls,
			"problem": nil,
			"split": map[string]any{
				"allocation":          renderAllocation(split.Allocation),
				"assignment_state":    split.AssignmentState,
				"suggested_item_keys": oracletest.AnyStrings(split.SuggestedItemKeys),
				"total_amount_vnd":    int64(split.TotalAmountVND),
				"participant_ids":     oracletest.AnyStrings(split.ParticipantIDs),
				"excluded_member_ids": oracletest.AnyStrings(split.ExcludedMemberIDs),
			},
			"response_error": false,
		}, nil

	case "create_batch":
		due, err := stamp(args["due_at"])
		if err != nil {
			return decodeFailed(err)
		}
		named := args["expense_version_ids"] != nil
		var versions []string
		if named {
			if versions, err = oracletest.Strings(args["expense_version_ids"]); err != nil {
				return decodeFailed(err)
			}
		}
		selected, err := batchInputsOf(args["selected"])
		if err != nil {
			return decodeFailed(err)
		}
		calls := []any{"is_member"}
		drafts, refused, err := FreezeBatch(due, harnessNow, versions, named, func(all bool, ids []string) (BatchInputs, error) {
			if all {
				calls = append(calls, []any{"load_batch_inputs", nil})
				if args["all"] == nil {
					return BatchInputs{}, errors.New("no unselected inputs were given")
				}
				return batchInputsOf(args["all"])
			}
			calls = append(calls, []any{"load_batch_inputs", oracletest.AnyStrings(ids)})
			return selected, nil
		})
		if err != nil {
			return nil, err
		}
		if refused != nil {
			return map[string]any{"calls": calls, "problem": problemOf(refused), "drafts": nil}, nil
		}
		rendered := make([]any, len(drafts))
		for i, d := range drafts {
			rendered[i] = map[string]any{
				"sender_id":                  d.SenderID,
				"recipient_id":               d.RecipientID,
				"amount_vnd":                 oracletest.Exact(d.AmountVND),
				"source_expense_version_ids": oracletest.AnyStrings(d.SourceExpenseVersionIDs),
				"sources":                    renderRows(d.Sources),
			}
		}
		return map[string]any{"calls": append(calls, "save_frozen_batch"), "problem": nil, "drafts": rendered}, nil

	case "publish_batch":
		batch, err := batchOf(args["batch"])
		if err != nil {
			return decodeFailed(err)
		}
		method, err := oracletest.Str(args["delivery_method"])
		if err != nil {
			return decodeFailed(err)
		}
		expires, err := stamp(args["expires_at"])
		if err != nil {
			return decodeFailed(err)
		}
		plan, refused, err := PublishBatch(batch, method, expires, harnessNow)
		if err != nil {
			return nil, err
		}
		if refused != nil {
			return map[string]any{"calls": []any{"load_batch_for_publish"}, "problem": problemOf(refused), "status": nil, "links": nil}, nil
		}
		links := make([]any, len(plan.Links))
		for i, link := range plan.Links {
			obligations := make([]any, len(link.Obligations))
			for j, o := range link.Obligations {
				obligations[j] = []any{o.ID, int64(o.AmountVND)}
			}
			links[i] = map[string]any{"sender_id": link.SenderID, "path_is_a_token": true, "obligations": obligations}
		}
		return map[string]any{"calls": []any{"load_batch_for_publish", "save_published_batch"}, "problem": nil, "status": plan.State, "links": links}, nil

	case "confirm_receipt":
		declared, err := vnd(args["declared_amount_vnd"])
		if err != nil {
			return decodeFailed(err)
		}
		rows, err := oracletest.List(args["receipt_amounts_vnd"])
		if err != nil {
			return decodeFailed(err)
		}
		receipts := make([]money.VND, len(rows))
		for i, raw := range rows {
			if receipts[i], err = vnd(raw); err != nil {
				return decodeFailed(err)
			}
		}
		status, refused, err := ReceiptStatus(declared, receipts)
		if err != nil {
			return nil, err
		}
		calls := []any{"get_receipt_target", "save_receipt_confirmation"}
		if refused != nil {
			return map[string]any{"calls": calls, "problem": problemOf(refused), "status": nil}, nil
		}
		return map[string]any{"calls": calls, "problem": nil, "status": status}, nil

	case "person_finance":
		me, err := oracletest.Str(args["actor_id"])
		if err != nil {
			return decodeFailed(err)
		}
		person, err := oracletest.Str(args["person_id"])
		if err != nil {
			return decodeFailed(err)
		}
		if refused := FinanceReadable(me, person); refused != nil {
			return map[string]any{"calls": []any{}, "problem": problemOf(refused)}, nil
		}
		return map[string]any{"calls": []any{"person_finance_summary"}, "problem": nil}, nil

	case "group_budget":
		outings, err := outingsOf(args["outings"])
		if err != nil {
			return decodeFailed(err)
		}
		roster, err := membersOf(args["roster"])
		if err != nil {
			return decodeFailed(err)
		}
		var candidate *money.VND
		if args["candidate_per_person_vnd"] != nil {
			value, err := vnd(args["candidate_per_person_vnd"])
			if err != nil {
				return decodeFailed(err)
			}
			candidate = &value
		}
		result, err := GroupBudget(outings, roster, candidate)
		if err != nil {
			return nil, err
		}
		return map[string]any{"calls": []any{"is_member", "group_recap", "list_members"}, "response": renderBudget(result)}, nil
	}
	return decodeFailed(errors.New("unknown function " + c.Fn))
}

func billRecordOf(value any) (BillRecord, error) {
	m, err := oracletest.Row(value, "printed_total_vnd", "items", "surcharges", "discounts")
	if err != nil {
		return BillRecord{}, err
	}
	var record BillRecord
	if m["printed_total_vnd"] != nil {
		printed, err := vnd(m["printed_total_vnd"])
		if err != nil {
			return record, err
		}
		record.PrintedTotalVND = &printed
	}
	items, err := oracletest.List(m["items"])
	if err != nil {
		return record, err
	}
	for _, raw := range items {
		r, err := oracletest.Row(raw, "item_key", "line_total_vnd", "shares")
		if err != nil {
			return record, err
		}
		var item billdraft.Item
		if item.ItemKey, err = oracletest.Str(r["item_key"]); err != nil {
			return record, err
		}
		if item.AmountVND, err = vnd(r["line_total_vnd"]); err != nil {
			return record, err
		}
		shares, err := oracletest.List(r["shares"])
		if err != nil {
			return record, err
		}
		for _, rawShare := range shares {
			pair, err := oracletest.Strings(rawShare)
			if err != nil || len(pair) != 2 {
				return record, fmt.Errorf("share %v is not [participant, source]", rawShare)
			}
			item.Shares = append(item.Shares, billdraft.Share{ParticipantID: pair[0], Source: pair[1]})
		}
		record.Items = append(record.Items, item)
	}
	surcharges, err := oracletest.List(m["surcharges"])
	if err != nil {
		return record, err
	}
	for _, raw := range surcharges {
		r, err := oracletest.Row(raw, "surcharge_key", "kind", "amount_vnd", "mode")
		if err != nil {
			return record, err
		}
		s, err := surchargeOf(r, "surcharge_key", vnd)
		if err != nil {
			return record, err
		}
		record.Surcharges = append(record.Surcharges, s)
	}
	discounts, err := oracletest.List(m["discounts"])
	if err != nil {
		return record, err
	}
	for _, raw := range discounts {
		r, err := oracletest.Row(raw, "discount_key", "amount_vnd", "scope", "target_item_key")
		if err != nil {
			return record, err
		}
		d, err := discountOf(r, "discount_key", "target_item_key", vnd)
		if err != nil {
			return record, err
		}
		record.Discounts = append(record.Discounts, d)
	}
	return record, nil
}

func batchOf(value any) (Batch, error) {
	m, err := oracletest.Row(value, "version_id", "status", "advancer_acknowledged", "obligations")
	if err != nil {
		return Batch{}, err
	}
	var batch Batch
	if batch.VersionID, err = oracletest.Str(m["version_id"]); err != nil {
		return batch, err
	}
	if batch.Status, err = oracletest.Str(m["status"]); err != nil {
		return batch, err
	}
	if batch.AdvancerAcknowledged, err = oracletest.Bool(m["advancer_acknowledged"]); err != nil {
		return batch, err
	}
	rows, err := oracletest.List(m["obligations"])
	if err != nil {
		return batch, err
	}
	for _, raw := range rows {
		fields, err := oracletest.List(raw)
		if err != nil || len(fields) != 5 {
			return batch, fmt.Errorf("obligation %v is not [id, version, sender, recipient, amount]", raw)
		}
		var o PublishObligation
		for i, target := range []*string{&o.ID, &o.BatchVersionID, &o.SenderID, &o.RecipientID} {
			if *target, err = oracletest.Str(fields[i]); err != nil {
				return batch, err
			}
		}
		if o.AmountVND, err = vnd(fields[4]); err != nil {
			return batch, err
		}
		batch.Obligations = append(batch.Obligations, o)
	}
	return batch, nil
}

func outingsOf(value any) ([]budget.Outing, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]budget.Outing, len(rows))
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
		if o.BudgetPerPersonVND, err = vnd(r["budget_per_person_vnd"]); err != nil {
			return nil, err
		}
		if o.SplitTotalVND, err = oracletest.Integer(r["split_total_vnd"]); err != nil {
			return nil, err
		}
		if o.InProgress, err = oracletest.Bool(r["in_progress"]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func renderBudget(b budget.Budget) map[string]any {
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
			"candidate_per_person_vnd": int64(b.Comparison.CandidatePerPersonVND),
			"delta_vnd":                oracletest.Exact(b.Comparison.DeltaVND),
			"verdict":                  b.Comparison.Verdict,
		}
	}
	return map[string]any{
		"context_id":          harnessContext,
		"outing_count":        int64(b.OutingCount),
		"active_member_count": b.ActiveMemberCount,
		"avg_per_person_vnd":  oracletest.Exact(b.AvgPerPersonVND),
		"in_progress":         live,
		"comparison":          comparison,
	}
}

func refusal(err error) (string, string, bool) {
	var refused *budget.BudgetError
	if errors.As(err, &refused) {
		return "BudgetError", refused.Code, true
	}
	return "", "", false
}

// stepFunctions is every ported step; person_finance has edge cases only.
var stepFunctions = []string{
	"propose_expense", "confirm_expense", "create_bill", "bill_assignment", "split_bill",
	"create_batch", "publish_batch", "confirm_receipt", "group_budget", "person_finance",
}

// stepCodes is every refusal the steps answer; not_your_finances comes from
// the edge cases only.
var stepCodes = []string{
	"participant_not_in_context", "permission_denied", "proposal_changed", "bill_items_total_mismatch",
	"bill_assignments_not_confirmed", "ITEM_HAS_NO_ASSIGNEE", "INVALID_SHARE_SOURCE", "BILL_HAS_NO_ITEMS",
	"UNKNOWN_PARTICIPANT", "RECONCILIATION_MISMATCH", "due_at_not_future", "expense_versions_unavailable",
	"no_unbatched_allocations", "NEGATIVE_AMOUNT", "no_obligations", "guest_link_expiry_not_future",
	"advancer_acknowledgement_required", "delivery_method_required", "ILLEGAL_TRANSITION", "UNKNOWN_STATE",
	"CROSSES_BATCH_VERSION", "DUPLICATE_OBLIGATION", "NON_POSITIVE_OBLIGATION", "NON_POSITIVE_CONFIRMATION",
	"INVALID_BUDGET_INPUT", "not_your_finances",
}

func TestServiceStepsMatchPython(t *testing.T) {
	checkSteps(t, oracletest.Load(t, "testdata/python_*.json"), "money_steps", 300, stepFunctions, stepCodes)
}

// checkSteps replays the cases of module in files: at least least in all,
// some of each of fns, and each of codes.
func checkSteps(t *testing.T, files []oracletest.File, module string, least int, fns, codes []string) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	total := 0
	for _, tally := range report.ByFn {
		total += tally.Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	for _, fn := range fns {
		if report.ByFn[fn] == nil {
			t.Errorf("no Python case of %s", fn)
		}
	}
	for _, code := range codes {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s: the corpus lost its spread", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}

// A merged obligation past int64 stays exact in the draft handed to the
// repository.
func TestDraftAmountIsNeverNarrowed(t *testing.T) {
	const payer, a = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1", "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
	inputs := BatchInputs{Expenses: []BatchExpense{
		{VersionID: "v1", PaidByID: payer, Allocations: []AllocationRow{{ID: "r1", ParticipantID: a, AmountVND: money.VND(1<<63 - 1)}}},
		{VersionID: "v2", PaidByID: payer, Allocations: []AllocationRow{{ID: "r2", ParticipantID: a, AmountVND: money.VND(1<<63 - 1)}}},
	}}
	drafts, refused, err := FreezeBatch(harnessNow.Add(time.Hour), harnessNow, []string{"v1", "v2"}, true, func(bool, []string) (BatchInputs, error) { return inputs, nil })
	if err != nil || refused != nil || len(drafts) != 1 {
		t.Fatalf("drafts %v refused %v err %v", drafts, refused, err)
	}
	want := new(big.Int).Mul(big.NewInt(1<<63-1), big.NewInt(2))
	if drafts[0].AmountVND.Cmp(want) != 0 {
		t.Fatalf("draft amount %s, want %s", drafts[0].AmountVND, want)
	}
}
