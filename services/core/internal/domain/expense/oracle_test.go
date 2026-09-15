package expense

import (
	"errors"
	"testing"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_expense*.json is rendered by scripts/render_domain_w4_goldens.py
// from the real app.domain.expense in the parity API image; every case is
// replayed here. component_rollups reads only amounts, kinds and the total,
// so the cases carry only those keys.

func amounts(value any) ([]money.VND, []string, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, nil, err
	}
	var out []money.VND
	var kinds []string
	for _, raw := range rows {
		r, err := oracletest.Row(raw, "amount_vnd")
		if err != nil {
			return nil, nil, err
		}
		n, err := oracletest.Int64(r["amount_vnd"])
		if err != nil {
			return nil, nil, err
		}
		out = append(out, money.VND(n))
		if kind, found := r["kind"]; found {
			s, err := oracletest.Str(kind)
			if err != nil {
				return nil, nil, err
			}
			kinds = append(kinds, s)
		}
	}
	return out, kinds, nil
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "component_rollups" {
		return nil, oracletest.Decode(errors.New("unknown function " + c.Fn))
	}
	m, err := oracletest.Row(args["expense"], "items", "surcharges", "discounts", "total_vnd")
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	var e allocator.Expense
	total, err := oracletest.Int64(m["total_vnd"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	e.TotalVND = money.VND(total)
	items, _, err := amounts(m["items"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	for _, amount := range items {
		e.Items = append(e.Items, allocator.Item{AmountVND: amount})
	}
	surcharges, kinds, err := amounts(m["surcharges"])
	if err != nil || len(kinds) != len(surcharges) {
		return nil, oracletest.Decode(errors.New("surcharges need a kind and an amount"))
	}
	for i, amount := range surcharges {
		e.Surcharges = append(e.Surcharges, allocator.Surcharge{Kind: kinds[i], AmountVND: amount})
	}
	discounts, _, err := amounts(m["discounts"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	for _, amount := range discounts {
		e.Discounts = append(e.Discounts, allocator.Discount{AmountVND: amount})
	}
	r := ComponentRollups(e)
	return map[string]any{
		"subtotal_amount_vnd": oracletest.Exact(r.SubtotalAmountVND),
		"fee_amount_vnd":      oracletest.Exact(r.FeeAmountVND),
		"vat_amount_vnd":      oracletest.Exact(r.VATAmountVND),
		"shipping_amount_vnd": oracletest.Exact(r.ShippingAmountVND),
		"discount_amount_vnd": oracletest.Exact(r.DiscountAmountVND),
		"total_amount_vnd":    int64(r.TotalAmountVND),
	}, nil
}

func noRefusal(error) (string, string, bool) { return "", "", false }

func TestComponentRollupsMatchPython(t *testing.T) {
	checkRollups(t, oracletest.Load(t, "testdata/python_*.json"), "expense", 150)
}

// checkRollups replays the cases of module in files: at least least of them,
// with some sums past int64.
func checkRollups(t *testing.T, files []oracletest.File, module string, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, noRefusal)
	tally := report.ByFn["component_rollups"]
	if tally == nil || tally.Cases < least || tally.BigResults == 0 {
		t.Fatalf("%+v, want at least %d cases: the corpus lost its spread", tally, least)
	}
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "expense")
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "component_rollups" {
		t.Errorf("Python exports %v; the port maps component_rollups only", names)
	}
}
