// Package billdraft is the Go port of app/domain/bill.py: a scanned bill
// draft arranged into the frozen ADR-0004 allocator input, for POST
// /bills/{bill_id}/split. (The directory is not named after the Python module
// because scripts/repo_guard.py refuses data files named for bills.)
//
// This package arranges and never computes: a rename, a passthrough or a
// refusal. The one addition is the total of a bill whose printed total was
// not read, the listed lines added up; it is a derived sum, so a *big.Int that
// may leave int64 or fall below zero, exactly as Python's int does. The
// allocator takes it saturated (allocator.Saturate), which cannot change the
// allocator's answer; Projection.TotalVND keeps the exact value.
//
// Refusals are found in Python's order: BILL_HAS_NO_ITEMS, then per item in
// the byte order of its key ITEM_HAS_NO_ASSIGNEE, then per item in that order
// INVALID_SHARE_SOURCE.
//
// testdata/python_*.json is rendered from the real module by
// scripts/render_domain_w4_goldens.py and replayed by oracle_test.go.
package billdraft

import (
	"math/big"
	"slices"
	"strings"

	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/money"
)

// SHARE_SUGGESTED and SHARE_CONFIRMED.
const (
	ShareSuggested = "ai_suggested"
	ShareConfirmed = "confirmed"
)

// Codes carried by *BillError.
const (
	CodeBillHasNoItems     = "BILL_HAS_NO_ITEMS"
	CodeItemHasNoAssignee  = "ITEM_HAS_NO_ASSIGNEE"
	CodeInvalidShareSource = "INVALID_SHARE_SOURCE"
)

// BillError mirrors BillError: Error returns the code.
type BillError struct {
	Code string
}

func (e *BillError) Error() string { return e.Code }

// Share is one share dict of an item: who, and whether a person decided it.
type Share struct {
	ParticipantID string
	Source        string
}

// Item is one item dict of the draft.
type Item struct {
	ItemKey   string
	AmountVND money.VND
	Shares    []Share
}

// Bill is the bill dict. PrintedTotalVND and AdvancerID nil are None.
type Bill struct {
	Participants    []string
	PrintedTotalVND *money.VND
	Items           []Item
	Surcharges      []allocator.Surcharge
	Discounts       []allocator.Discount
	AdvancerID      *string
}

// Projection is allocator_input_from_bill's dict. Expense.TotalVND is
// allocator.Saturate(TotalVND); TotalVND is expense["total_vnd"] exactly.
type Projection struct {
	Expense           allocator.Expense
	TotalVND          *big.Int
	AssignmentState   string
	SuggestedItemKeys []string
}

func byKey(items []Item) []Item {
	sorted := slices.Clone(items)
	slices.SortStableFunc(sorted, func(a, b Item) int { return strings.Compare(a.ItemKey, b.ItemKey) })
	return sorted
}

// AllocatorInputFromBill is allocator_input_from_bill.
func AllocatorInputFromBill(bill Bill) (Projection, error) {
	if len(bill.Items) == 0 {
		return Projection{}, &BillError{Code: CodeBillHasNoItems}
	}
	sorted := byKey(bill.Items)
	for _, item := range sorted {
		if len(item.Shares) == 0 {
			return Projection{}, &BillError{Code: CodeItemHasNoAssignee}
		}
	}
	for _, item := range sorted {
		for _, share := range item.Shares {
			if share.Source != ShareSuggested && share.Source != ShareConfirmed {
				return Projection{}, &BillError{Code: CodeInvalidShareSource}
			}
		}
	}

	suggested := []string{}
	for _, item := range bill.Items {
		if slices.ContainsFunc(item.Shares, func(s Share) bool { return s.Source == ShareSuggested }) {
			suggested = append(suggested, item.ItemKey)
		}
	}
	slices.SortStableFunc(suggested, strings.Compare)

	total := new(big.Int)
	if bill.PrintedTotalVND == nil {
		for _, item := range bill.Items {
			total.Add(total, big.NewInt(int64(item.AmountVND)))
		}
		for _, surcharge := range bill.Surcharges {
			total.Add(total, big.NewInt(int64(surcharge.AmountVND)))
		}
		for _, discount := range bill.Discounts {
			total.Sub(total, big.NewInt(int64(discount.AmountVND)))
		}
	} else {
		total.SetInt64(int64(*bill.PrintedTotalVND))
	}

	items := make([]allocator.Item, len(bill.Items))
	for i, item := range bill.Items {
		sharedBy := make([]string, len(item.Shares))
		for j, share := range item.Shares {
			sharedBy[j] = share.ParticipantID
		}
		items[i] = allocator.Item{ItemID: item.ItemKey, AmountVND: item.AmountVND, SharedBy: sharedBy}
	}
	state := ShareConfirmed
	if len(suggested) > 0 {
		state = ShareSuggested
	}
	return Projection{
		Expense: allocator.Expense{
			Participants: slices.Clone(bill.Participants),
			TotalVND:     allocator.Saturate(total),
			Items:        items,
			Surcharges:   slices.Clone(bill.Surcharges),
			Discounts:    slices.Clone(bill.Discounts),
			AdvancerID:   bill.AdvancerID,
		},
		TotalVND:          total,
		AssignmentState:   state,
		SuggestedItemKeys: suggested,
	}, nil
}
