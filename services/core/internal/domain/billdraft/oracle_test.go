package billdraft

import (
	"errors"
	"testing"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_billdraft*.json is rendered by
// scripts/render_domain_w4_goldens.py from the real app.domain.bill in the
// parity API image; every case is replayed here.

func vnd(value any) (money.VND, error) {
	n, err := oracletest.Int64(value)
	return money.VND(n), err
}

func billOf(value any) (Bill, error) {
	m, err := oracletest.Row(value, "participants", "printed_total_vnd", "items", "surcharges", "discounts", "advancer_id")
	if err != nil {
		return Bill{}, err
	}
	var b Bill
	if b.Participants, err = oracletest.Strings(m["participants"]); err != nil {
		return Bill{}, err
	}
	if m["printed_total_vnd"] != nil {
		printed, err := vnd(m["printed_total_vnd"])
		if err != nil {
			return Bill{}, err
		}
		b.PrintedTotalVND = &printed
	}
	if b.AdvancerID, err = oracletest.OptionalString(m["advancer_id"]); err != nil {
		return Bill{}, err
	}
	items, err := oracletest.List(m["items"])
	if err != nil {
		return Bill{}, err
	}
	for _, raw := range items {
		r, err := oracletest.Row(raw, "item_key", "amount_vnd", "shares")
		if err != nil {
			return Bill{}, err
		}
		var item Item
		if item.ItemKey, err = oracletest.Str(r["item_key"]); err != nil {
			return Bill{}, err
		}
		if item.AmountVND, err = vnd(r["amount_vnd"]); err != nil {
			return Bill{}, err
		}
		shares, err := oracletest.List(r["shares"])
		if err != nil {
			return Bill{}, err
		}
		for _, rawShare := range shares {
			s, err := oracletest.Row(rawShare, "participant_id", "source")
			if err != nil {
				return Bill{}, err
			}
			var share Share
			if share.ParticipantID, err = oracletest.Str(s["participant_id"]); err != nil {
				return Bill{}, err
			}
			if share.Source, err = oracletest.Str(s["source"]); err != nil {
				return Bill{}, err
			}
			item.Shares = append(item.Shares, share)
		}
		b.Items = append(b.Items, item)
	}
	surcharges, err := oracletest.List(m["surcharges"])
	if err != nil {
		return Bill{}, err
	}
	for _, raw := range surcharges {
		r, err := oracletest.Row(raw, "surcharge_id", "kind", "amount_vnd", "mode")
		if err != nil {
			return Bill{}, err
		}
		var s allocator.Surcharge
		if s.SurchargeID, err = oracletest.Str(r["surcharge_id"]); err != nil {
			return Bill{}, err
		}
		if s.Kind, err = oracletest.Str(r["kind"]); err != nil {
			return Bill{}, err
		}
		if s.AmountVND, err = vnd(r["amount_vnd"]); err != nil {
			return Bill{}, err
		}
		if s.Mode, err = oracletest.Str(r["mode"]); err != nil {
			return Bill{}, err
		}
		b.Surcharges = append(b.Surcharges, s)
	}
	discounts, err := oracletest.List(m["discounts"])
	if err != nil {
		return Bill{}, err
	}
	for _, raw := range discounts {
		r, err := oracletest.Row(raw, "discount_id", "amount_vnd", "scope", "item_id")
		if err != nil {
			return Bill{}, err
		}
		var d allocator.Discount
		if d.DiscountID, err = oracletest.Str(r["discount_id"]); err != nil {
			return Bill{}, err
		}
		if d.AmountVND, err = vnd(r["amount_vnd"]); err != nil {
			return Bill{}, err
		}
		if d.Scope, err = oracletest.Str(r["scope"]); err != nil {
			return Bill{}, err
		}
		if d.ItemID, err = oracletest.OptionalString(r["item_id"]); err != nil {
			return Bill{}, err
		}
		b.Discounts = append(b.Discounts, d)
	}
	return b, nil
}

func renderProjection(p Projection) map[string]any {
	e := p.Expense
	items := make([]any, len(e.Items))
	for i, item := range e.Items {
		items[i] = map[string]any{"item_id": item.ItemID, "amount_vnd": int64(item.AmountVND), "shared_by": oracletest.AnyStrings(item.SharedBy)}
	}
	surcharges := make([]any, len(e.Surcharges))
	for i, s := range e.Surcharges {
		surcharges[i] = map[string]any{"surcharge_id": s.SurchargeID, "kind": s.Kind, "amount_vnd": int64(s.AmountVND), "mode": s.Mode}
	}
	discounts := make([]any, len(e.Discounts))
	for i, d := range e.Discounts {
		var target any
		if d.ItemID != nil {
			target = *d.ItemID
		}
		discounts[i] = map[string]any{"discount_id": d.DiscountID, "amount_vnd": int64(d.AmountVND), "scope": d.Scope, "item_id": target}
	}
	var advancer any
	if e.AdvancerID != nil {
		advancer = *e.AdvancerID
	}
	return map[string]any{
		"expense": map[string]any{
			"participants": oracletest.AnyStrings(e.Participants),
			"total_vnd":    oracletest.Exact(p.TotalVND),
			"items":        items,
			"surcharges":   surcharges,
			"discounts":    discounts,
			"advancer_id":  advancer,
		},
		"assignment_state":    p.AssignmentState,
		"suggested_item_keys": oracletest.AnyStrings(p.SuggestedItemKeys),
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "allocator_input_from_bill" {
		return nil, oracletest.Decode(errors.New("unknown function " + c.Fn))
	}
	b, err := billOf(args["bill"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	projection, err := AllocatorInputFromBill(b)
	if err != nil {
		return nil, err
	}
	if projection.Expense.TotalVND != allocator.Saturate(projection.TotalVND) {
		return nil, errors.New("the allocator total is not the saturated exact total")
	}
	return renderProjection(projection), nil
}

func refusal(err error) (string, string, bool) {
	var refused *BillError
	if errors.As(err, &refused) {
		return "BillError", refused.Code, true
	}
	return "", "", false
}

func TestAllocatorInputFromBillMatchesPython(t *testing.T) {
	checkBill(t, oracletest.Load(t, "testdata/python_*.json"), "billdraft", 200)
}

// checkBill replays the cases of module in files and asserts their spread: at
// least least cases, every refusal code, and some totals past int64.
func checkBill(t *testing.T, files []oracletest.File, module string, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	tally := report.ByFn["allocator_input_from_bill"]
	if tally == nil || tally.Cases < least || tally.BigResults == 0 {
		t.Fatalf("%+v, want at least %d cases: the corpus lost its spread", tally, least)
	}
	for _, code := range []string{CodeBillHasNoItems, CodeItemHasNoAssignee, CodeInvalidShareSource} {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "billdraft")
	if constants["share_suggested"] != ShareSuggested || constants["share_confirmed"] != ShareConfirmed {
		t.Errorf("share sources: Python %v %v", constants["share_suggested"], constants["share_confirmed"])
	}
	ported := map[string]bool{"SHARE_CONFIRMED": true, "SHARE_SUGGESTED": true, "BillError": true, "allocator_input_from_bill": true}
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
