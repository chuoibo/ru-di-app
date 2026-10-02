//go:build postgres

package repo

// CountUnbatchedExpenses (QA UI-058) on a real schema: the expenses a new
// collection round would gather, the same selection FreezeBatch makes. A
// group with one expense still outside any round counts it; once its
// collectable row backs an obligation it is gone; a version with no
// allocations and another group's expense never count.

import (
	"context"
	"testing"

	"mobile/services/core/internal/testdb"
)

func TestCountUnbatchedExpenses(t *testing.T) {
	ctx := context.Background()
	tx := testdb.Tx(t)
	w := &world{}
	an := w.person(0x7a1, "An (dữ liệu mẫu)")
	binh := w.person(0x7a2, "Bình (dữ liệu mẫu)")
	g := w.context(0x7a1, an)
	other := w.context(0x7a2, an)
	alloc := map[string]string{}
	when := "2030-07-01T00:00:00Z"
	// Gathered: an edited expense whose latest version still owes Bình.
	e1, v1 := expenseSQL(0x7a1, g, an, alloc,
		moneyVersion{"acknowledged", "Lẩu (dữ liệu mẫu)", when, []share{{an, 40_000}, {binh, 40_000}}},
		moneyVersion{"acknowledged", "Lẩu, sửa lại (dữ liệu mẫu)", when, []share{{an, 50_000}, {binh, 30_000}}})
	// Already in a round: Bình's share backs an obligation.
	e2, v2 := expenseSQL(0x7a2, g, an, alloc,
		moneyVersion{"acknowledged", "Cà phê (dữ liệu mẫu)", when, []share{{an, 20_000}, {binh, 20_000}}})
	// Not gatherable: the latest version has no allocation at all.
	e3, _ := expenseSQL(0x7a3, g, an, alloc, moneyVersion{"pending", "Nháp (dữ liệu mẫu)", when, nil})
	// Another group's expense.
	e4, _ := expenseSQL(0x7a4, other, an, alloc,
		moneyVersion{"acknowledged", "Nhóm khác (dữ liệu mẫu)", when, []share{{an, 10_000}, {binh, 10_000}}})
	round := extraBatchSQL(0x7, g, an, when, extraObligation{sender: binh, recipient: an, amount: 20_000})
	source := insertSQL("collection_obligation_sources", "obligation_id", extraObligationID(0x7, 0),
		"confirmed_allocation_id", alloc[allocKey(v2[0], binh)], "amount_vnd", int64(20_000))
	for _, part := range [][]string{w.sql, e1, e2, e3, e4, round, {source}} {
		for _, statement := range part {
			if _, err := tx.Exec(ctx, statement); err != nil {
				t.Fatalf("%v\n%s", err, statement)
			}
		}
	}
	store := Repository{Q: tx}
	count := func() int {
		t.Helper()
		n, err := store.CountUnbatchedExpenses(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(); n != 1 {
		t.Fatalf("one expense is outside every round, counted %d", n)
	}
	// Once the last expense's collectable row backs an obligation, nothing is left.
	if _, err := tx.Exec(ctx, insertSQL("collection_obligation_sources", "obligation_id", extraObligationID(0x7, 0),
		"confirmed_allocation_id", alloc[allocKey(v1[1], binh)], "amount_vnd", int64(30_000))); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 0 {
		t.Fatalf("every expense is in a round, counted %d", n)
	}
	if n, err := store.CountUnbatchedExpenses(ctx, fid(kindContext, 0x7a9)); err != nil || n != 0 {
		t.Fatalf("an unknown group counts nothing: %d, %v", n, err)
	}
}
