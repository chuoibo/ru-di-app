// Package expense is the Go port of app/domain/expense.py: the scalar
// roll-ups POST /expenses/{expense_id}/confirm stores beside a confirmed
// expense version.
//
// Every roll-up but the total is a derived sum, so a *big.Int; the total is
// the expense's own money.VND. A surcharge kind is matched with CPython's
// str.casefold(), reused from the taste port that checks it against CPython on
// every code point, so "VAT" is vat and "ſhipping" (U+017F) is shipping,
// which strings.ToLower would miss. Callers run the allocator first; this
// package validates nothing, as Python does not.
//
// testdata/python_*.json is rendered from the real module by
// scripts/render_domain_w4_goldens.py and replayed by oracle_test.go.
package expense

import (
	"math/big"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/taste"
)

// The two surcharge kinds with a column of their own; every other kind is a
// fee.
const (
	KindVAT      = "vat"
	KindShipping = "shipping"
)

// Rollups is component_rollups' dict.
type Rollups struct {
	SubtotalAmountVND *big.Int
	FeeAmountVND      *big.Int
	VATAmountVND      *big.Int
	ShippingAmountVND *big.Int
	DiscountAmountVND *big.Int
	TotalAmountVND    money.VND
}

func add(sum *big.Int, amount money.VND) { sum.Add(sum, big.NewInt(int64(amount))) }

// ComponentRollups is component_rollups. A pure even split, with no lines at
// all, has its total as its subtotal.
func ComponentRollups(e allocator.Expense) Rollups {
	rollups := Rollups{
		SubtotalAmountVND: new(big.Int),
		FeeAmountVND:      new(big.Int),
		VATAmountVND:      new(big.Int),
		ShippingAmountVND: new(big.Int),
		DiscountAmountVND: new(big.Int),
		TotalAmountVND:    e.TotalVND,
	}
	for _, item := range e.Items {
		add(rollups.SubtotalAmountVND, item.AmountVND)
	}
	if len(e.Items) == 0 && len(e.Surcharges) == 0 && len(e.Discounts) == 0 {
		rollups.SubtotalAmountVND.SetInt64(int64(e.TotalVND))
	}
	for _, surcharge := range e.Surcharges {
		switch taste.Casefold(surcharge.Kind) {
		case KindVAT:
			add(rollups.VATAmountVND, surcharge.AmountVND)
		case KindShipping:
			add(rollups.ShippingAmountVND, surcharge.AmountVND)
		default:
			add(rollups.FeeAmountVND, surcharge.AmountVND)
		}
	}
	for _, discount := range e.Discounts {
		add(rollups.DiscountAmountVND, discount.AmountVND)
	}
	return rollups
}
