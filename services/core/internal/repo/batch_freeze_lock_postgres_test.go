//go:build postgres

package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"mobile/services/core/internal/testdb"
)

// A batch freeze and a new version of one of its expenses cannot pass each
// other (security review of ADR-0056 §2.1): a confirmation holds the expense
// row (GetExpense) until it commits, and the freeze path now waits for that
// row before reading the latest versions. The balances read takes no lock.
func TestBatchFreezeWaitsForAConfirmationOfTheSameExpense(t *testing.T) {
	p := testdb.Pool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var person string
	if err := p.QueryRow(ctx, `INSERT INTO people(id,display_name) VALUES(gen_random_uuid(),'Khoá đợt thu (dữ liệu mẫu)') RETURNING id::text`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	group, err := (Repository{Q: p}).CreateContext(ctx, "Nhóm khoá (dữ liệu mẫu)", person)
	if err != nil {
		t.Fatal(err)
	}
	expense, err := (Repository{Q: p}).CreateExpense(ctx, group.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = p.Exec(bg, `DELETE FROM expenses WHERE id=$1`, expense.ID)
		_, _ = p.Exec(bg, `DELETE FROM contexts WHERE id=$1`, group.ID)
		_, _ = p.Exec(bg, `DELETE FROM people WHERE id=$1`, person)
	})

	confirming, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer confirming.Rollback(context.Background())
	if e, err := (Repository{Q: confirming}).GetExpense(ctx, expense.ID); err != nil || e == nil {
		t.Fatalf("lock the expense as a confirmation does: %v %v", e, err)
	}

	freezing, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer freezing.Rollback(context.Background())
	if _, err := freezing.Exec(ctx, `SET LOCAL lock_timeout = '300ms'`); err != nil {
		t.Fatal(err)
	}
	_, err = (Repository{Q: freezing}).LoadBatchInputs(ctx, group.ID, []string{})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("the freeze path did not wait for the confirmation: %v", err)
	}

	reading, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reading.Rollback(context.Background())
	if _, err := reading.Exec(ctx, `SET LOCAL lock_timeout = '300ms'`); err != nil {
		t.Fatal(err)
	}
	if _, err := (Repository{Q: reading}).LoadBatchInputs(ctx, group.ID, nil); err != nil {
		t.Fatalf("the balances read waited on a lock: %v", err)
	}
}
