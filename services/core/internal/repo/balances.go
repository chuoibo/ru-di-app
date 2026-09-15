package repo

// The ledger reads GET /contexts/{id}/balances makes: load_batch_inputs and
// load_confirmed_receipts. Stored amounts are integer dong (int64, the BIGINT
// columns); a SUM over them is numeric and is carried exactly as *big.Int.

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// AllocationRow is AllocationRow.
type AllocationRow struct {
	ID            string
	ParticipantID string
	AmountVND     int64
}

// ConfirmedExpense is ConfirmedExpense.
type ConfirmedExpense struct {
	VersionID            string
	ContextID            string
	PaidByID             string
	PayerAcknowledgement string
	Allocations          []AllocationRow
}

// BatchInputs is BatchInputs. Neither slice is ever nil.
type BatchInputs struct {
	Expenses              []ConfirmedExpense
	UnavailableVersionIDs []string
}

// ReceiptTotal is one entry of load_confirmed_receipts' dict, keyed by
// (sender_id, recipient_id), in insertion order. AmountVND is the exact sum:
// every receipt is at most the BIGINT maximum, but two of them on one pair
// already pass it, and Python's int() of the numeric SUM keeps every digit.
type ReceiptTotal struct {
	SenderID    string
	RecipientID string
	AmountVND   *big.Int
}

// ErrNotAnInteger marks a numeric that Python's int() would not have read as
// the same whole number: NULL, NaN, an infinity or a fraction. A SUM of
// BIGINT over at least one row is none of these.
var ErrNotAnInteger = errors.New("repo: numeric is not a whole number")

// expenseVersionColumns is `select(ExpenseVersion)`: every mapped column in
// declaration order, unlabelled.
const expenseVersionColumns = `expense_versions.id, expense_versions.expense_id, expense_versions.version_number,
       expense_versions.previous_version_number, expense_versions.description, expense_versions.recorded_by_id,
       expense_versions.paid_by_id, expense_versions.payer_acknowledgement, expense_versions.verification_scope,
       expense_versions.subtotal_amount_vnd, expense_versions.fee_amount_vnd, expense_versions.vat_amount_vnd,
       expense_versions.shipping_amount_vnd, expense_versions.discount_amount_vnd,
       expense_versions.total_amount_vnd, expense_versions.occurred_at, expense_versions.created_at`

// uuidIn is `column.in_(ids)` as the psycopg dialect renders it once the
// expanding parameter is compiled: one ::UUID parameter per element from
// $first, duplicates kept, and the empty-set form for no elements.
func uuidIn(column string, first int, ids []string) string {
	if len(ids) == 0 {
		return column + " IN (NULL) AND (1 != 1)"
	}
	return column + " IN (" + uuidPlaceholders(first, len(ids)) + ")"
}

type confirmedVersion struct {
	id, paidByID, payerAcknowledgement string
}

// LoadBatchInputs is load_batch_inputs. A nil expenseVersionIDs is Python's
// None (the balances read model); a non-nil slice, empty included, is the
// batch-creation tuple.
//
// Statements, in Python's order:
//  1. the latest version of every expense of the context, joined to the
//     grouped max(version_number) subquery, ORDER BY id, FOR UPDATE OF
//     expense_versions (row locks on the version rows only); with ids, an
//     extra `id IN (...)` (the empty-set form for an empty slice);
//  2. per version, in that order, its confirmed_allocations ORDER BY
//     participant_id FOR UPDATE; a version with none is unavailable;
//  3. with ids only, per version with allocations, the obligation sources
//     among its collectable rows (not the payer's, amount above zero), the
//     statement issued even when nothing is collectable; any source makes
//     the version unavailable.
//
// unavailable_version_ids is the requested ids not returned plus the versions
// refused above, distinct, in UUID byte order (the canonical lowercase text
// order). Ids travel in canonical form, so the set difference is exact.
func (r Repository) LoadBatchInputs(ctx context.Context, contextID string, expenseVersionIDs []string) (BatchInputs, error) {
	sql := `SELECT ` + expenseVersionColumns + `
	          FROM expense_versions
	          JOIN expenses ON expenses.id = expense_versions.expense_id
	          JOIN (SELECT expense_versions.expense_id AS expense_id,
	                       max(expense_versions.version_number) AS version_number
	                  FROM expense_versions GROUP BY expense_versions.expense_id) AS anon_1
	            ON anon_1.expense_id = expense_versions.expense_id
	           AND anon_1.version_number = expense_versions.version_number
	         WHERE expenses.context_id = $1::UUID`
	args := []any{contextID}
	if expenseVersionIDs != nil {
		sql += " AND " + uuidIn("expense_versions.id", 2, expenseVersionIDs)
		args = append(args, uuidArgs(expenseVersionIDs)...)
	}
	sql += ` ORDER BY expense_versions.id FOR UPDATE OF expense_versions`

	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return BatchInputs{}, err
	}
	var versions []confirmedVersion
	for rows.Next() {
		var v confirmedVersion
		var expenseID, recordedByID, verificationScope string
		var versionNumber int32
		var previousVersionNumber *int32
		var description *string
		var subtotal, fee, vat, shipping, discount, total int64
		var occurredAt, createdAt time.Time
		if err := rows.Scan(&v.id, &expenseID, &versionNumber, &previousVersionNumber, &description, &recordedByID,
			&v.paidByID, &v.payerAcknowledgement, &verificationScope, &subtotal, &fee, &vat, &shipping, &discount,
			&total, &occurredAt, &createdAt); err != nil {
			rows.Close()
			return BatchInputs{}, err
		}
		versions = append(versions, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return BatchInputs{}, err
	}

	unavailable := map[string]bool{}
	for _, id := range expenseVersionIDs {
		unavailable[id] = true
	}
	for _, v := range versions {
		delete(unavailable, v.id)
	}
	out := BatchInputs{Expenses: []ConfirmedExpense{}, UnavailableVersionIDs: []string{}}
	for _, v := range versions {
		allocations, err := r.lockedAllocations(ctx, v.id)
		if err != nil {
			return BatchInputs{}, err
		}
		if len(allocations) == 0 {
			unavailable[v.id] = true
			continue
		}
		if expenseVersionIDs != nil {
			var collectable []string
			for _, a := range allocations {
				if a.ParticipantID != v.paidByID && a.AmountVND > 0 {
					collectable = append(collectable, a.ID)
				}
			}
			sourced, err := r.anyObligationSource(ctx, collectable)
			if err != nil {
				return BatchInputs{}, err
			}
			if sourced {
				unavailable[v.id] = true
				continue
			}
		}
		out.Expenses = append(out.Expenses, ConfirmedExpense{VersionID: v.id, ContextID: contextID,
			PaidByID: v.paidByID, PayerAcknowledgement: v.payerAcknowledgement, Allocations: allocations})
	}
	for id := range unavailable {
		out.UnavailableVersionIDs = append(out.UnavailableVersionIDs, id)
	}
	sort.Strings(out.UnavailableVersionIDs)
	return out, nil
}

// lockedAllocations is one version's confirmed_allocations, every mapped
// column, ORDER BY participant_id, FOR UPDATE.
func (r Repository) lockedAllocations(ctx context.Context, versionID string) ([]AllocationRow, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT confirmed_allocations.id, confirmed_allocations.expense_version_id,
		        confirmed_allocations.participant_id, confirmed_allocations.amount_vnd,
		        confirmed_allocations.confirmed_by_id, confirmed_allocations.confirmed_at
		   FROM confirmed_allocations
		  WHERE confirmed_allocations.expense_version_id = $1::UUID
		  ORDER BY confirmed_allocations.participant_id FOR UPDATE`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AllocationRow
	for rows.Next() {
		var a AllocationRow
		var version, confirmedBy string
		var confirmedAt time.Time
		if err := rows.Scan(&a.ID, &version, &a.ParticipantID, &a.AmountVND, &confirmedBy, &confirmedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// anyObligationSource is whether `set(scalars(select(source id) where id IN
// collectable))` is non-empty.
func (r Repository) anyObligationSource(ctx context.Context, allocationIDs []string) (bool, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT collection_obligation_sources.confirmed_allocation_id
		   FROM collection_obligation_sources
		  WHERE `+uuidIn("collection_obligation_sources.confirmed_allocation_id", 1, allocationIDs),
		uuidArgs(allocationIDs)...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
	}
	return found, rows.Err()
}

// LoadConfirmedReceipts is load_confirmed_receipts: per (sender, recipient)
// of the context's obligations, the sum of the receipt confirmations the
// recipient themself made, ordered by sender then recipient. PostgreSQL's
// sum of a bigint is numeric, read here digit for digit into a *big.Int, as
// Python's int() reads the Decimal psycopg hands it: past int64 included.
func (r Repository) LoadConfirmedReceipts(ctx context.Context, contextID string) ([]ReceiptTotal, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT collection_obligations.sender_id, collection_obligations.recipient_id,
		        sum(receipt_confirmations.amount_vnd) AS sum_1
		   FROM collection_obligations
		   JOIN receipt_confirmations ON receipt_confirmations.obligation_id = collection_obligations.id
		   JOIN collection_batch_versions ON collection_batch_versions.id = collection_obligations.batch_version_id
		   JOIN collection_batches ON collection_batches.id = collection_batch_versions.batch_id
		  WHERE collection_batches.context_id = $1::UUID
		    AND receipt_confirmations.confirmed_by_id = collection_obligations.recipient_id
		  GROUP BY collection_obligations.sender_id, collection_obligations.recipient_id
		  ORDER BY collection_obligations.sender_id, collection_obligations.recipient_id`, contextID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReceiptTotal{}
	for rows.Next() {
		var t ReceiptTotal
		var total pgtype.Numeric
		if err := rows.Scan(&t.SenderID, &t.RecipientID, &total); err != nil {
			return nil, err
		}
		amount, err := numericInteger(total)
		if err != nil {
			return nil, err
		}
		t.AmountVND = amount
		out = append(out, t)
	}
	return out, rows.Err()
}

// numericInteger is the exact whole number a numeric holds: its digits times
// ten to its exponent, a negative exponent dividing without remainder.
func numericInteger(n pgtype.Numeric) (*big.Int, error) {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite || n.Int == nil {
		return nil, ErrNotAnInteger
	}
	out := new(big.Int).Set(n.Int)
	if n.Exp == 0 {
		return out, nil
	}
	exp := int64(n.Exp)
	if exp < 0 {
		exp = -exp
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(exp), nil)
	if n.Exp > 0 {
		return out.Mul(out, scale), nil
	}
	remainder := new(big.Int)
	out.QuoRem(out, scale, remainder)
	if remainder.Sign() != 0 {
		return nil, ErrNotAnInteger
	}
	return out, nil
}
