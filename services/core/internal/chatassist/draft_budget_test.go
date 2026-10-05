package chatassist

import (
	"testing"

	"mobile/services/core/internal/domain/allocator"
)

// A draft budget is bounded like any amount the ledger takes (audit
// 2026-10-05, DB-12): past it a draft could be saved but never promoted, and
// past 2^53 the app could not even show it exactly.
func TestDraftBudgetIsBoundedLikeTheLedger(t *testing.T) {
	for _, c := range []struct {
		budget int64
		ok     bool
	}{{0, true}, {int64(allocator.MaxAmountVND), true}, {int64(allocator.MaxAmountVND) + 1, false}, {1 << 53, false}, {-1, false}} {
		b := c.budget
		err := validateDraftHead("Đi chơi", nil, nil, nil, &b)
		if (err == nil) != c.ok {
			t.Errorf("budget %d: %v", c.budget, err)
		}
	}
}
