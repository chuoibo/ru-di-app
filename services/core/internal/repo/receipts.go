package repo

// POST /obligations/{id}/confirm-receipt and GET /people/{id}/finance:
// get_receipt_target, save_receipt_confirmation and person_finance_summary.
// A receipt confirmation is the only event that moves an obligation; the
// finance figures are recomputed from the ledger on every read, each SQL SUM
// carried exactly past int64 as Python's int() carries it.

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReceiptTarget is ReceiptTarget.
type ReceiptTarget struct {
	ObligationID string
	RecipientID  string
	AmountVND    int64
}

// ReceiptRecord is ReceiptRecord.
type ReceiptRecord struct {
	ID                string
	ObligationID      string
	AmountVND         int64
	ReceiptAmountsVND []int64
}

// ReceiptConfirmationInput is save_receipt_confirmation's keyword arguments.
// AmountVND is the request's amount exactly: ReceiptConfirmationRequest has no
// ceiling, so it can pass int64, and Python still compares it with a stored
// receipt and binds it to the INSERT, where PostgreSQL refuses it.
type ReceiptConfirmationInput struct {
	Target          ReceiptTarget
	ConfirmedByID   string
	AmountVND       *big.Int
	PaymentReportID *string
	IdempotencyKey  string
	Now             time.Time
}

// FinanceMovement is FinanceMovement.
type FinanceMovement struct {
	ObligationID     string
	Direction        string
	AmountVND        int64
	CounterpartyID   string
	CounterpartyName *string
	ContextID        string
	ContextName      *string
	Occasion         *string
	OccurredAt       time.Time
}

// PersonFinanceSummary is PersonFinanceSummary. The four money figures are
// exact integers: each is a SUM (or a difference of SUMs) that can pass int64.
type PersonFinanceSummary struct {
	PersonID       string
	DisplayName    *string
	SpendVND       *big.Int
	SettledVND     *big.Int
	OutstandingVND *big.Int
	ReceivableVND  *big.Int
	ExpenseCount   int64
	GroupCount     int64
	Movements      []FinanceMovement
}

// GetReceiptTarget is get_receipt_target: the obligation by id, SELECT ...
// FOR UPDATE, nil when there is none.
func (r Repository) GetReceiptTarget(ctx context.Context, obligationID string) (*ReceiptTarget, error) {
	rows, err := r.obligationRows(ctx,
		`SELECT `+obligationColumns+`
		   FROM collection_obligations
		  WHERE collection_obligations.id = $1::UUID FOR UPDATE`, obligationID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	o := rows[0]
	return &ReceiptTarget{ObligationID: o.id, RecipientID: o.recipientID, AmountVND: o.amountVND}, nil
}

// SaveReceiptConfirmation is save_receipt_confirmation.
//
// Statements and refusals, in Python's order:
//  1. the receipt with this idempotency key, unlocked. When there is one: a
//     different obligation, confirmer, amount (an amount past int64 is never
//     the stored one) or payment report is Conflict
//     IDEMPOTENCY_KEY_REUSED raised from nothing; otherwise the stored receipt
//     is answered with the obligation's receipt amounts, nothing written;
//  2. with a payment report id, `session.get(PaymentReport, id)` (labelled):
//     none, or one of another obligation, is Conflict
//     PAYMENT_REPORT_NOT_FOR_OBLIGATION;
//  3. flush: the receipt INSERT (the caller's clock); a check or foreign key
//     refusal is returned as is, and so is PostgreSQL's 22003 for an amount
//     past BIGINT, which psycopg sends as numeric and pgx here as text;
//  4. the audit event, flushed by the next statement's autoflush;
//  5. the obligation's receipt amounts ORDER BY confirmed_at, id.
func (r Repository) SaveReceiptConfirmation(ctx context.Context, in ReceiptConfirmationInput) (ReceiptRecord, error) {
	var existingID, existingObligation, confirmedBy, key string
	var reportID *string
	var amount int64
	var confirmedAt time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT receipt_confirmations.id, receipt_confirmations.obligation_id, receipt_confirmations.payment_report_id,
		        receipt_confirmations.confirmed_by_id, receipt_confirmations.amount_vnd,
		        receipt_confirmations.idempotency_key, receipt_confirmations.confirmed_at
		   FROM receipt_confirmations
		  WHERE receipt_confirmations.idempotency_key = $1::UUID`, in.IdempotencyKey).
		Scan(&existingID, &existingObligation, &reportID, &confirmedBy, &amount, &key, &confirmedAt)
	switch {
	case err == nil:
		if existingObligation != in.Target.ObligationID || confirmedBy != in.ConfirmedByID ||
			big.NewInt(amount).Cmp(in.AmountVND) != 0 || !sameText(reportID, in.PaymentReportID) {
			return ReceiptRecord{}, &Conflict{Code: "IDEMPOTENCY_KEY_REUSED"}
		}
		amounts, err := r.receiptAmounts(ctx, existingObligation)
		if err != nil {
			return ReceiptRecord{}, err
		}
		return ReceiptRecord{ID: existingID, ObligationID: existingObligation, AmountVND: amount,
			ReceiptAmountsVND: amounts}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return ReceiptRecord{}, err
	}

	if in.PaymentReportID != nil {
		var id, obligation string
		var link, reporter *string
		var reportAmount int64
		var reportKey string
		var reportedAt time.Time
		err := r.Q.QueryRow(ctx,
			`SELECT payment_reports.id AS payment_reports_id,
			        payment_reports.obligation_id AS payment_reports_obligation_id,
			        payment_reports.guest_link_id AS payment_reports_guest_link_id,
			        payment_reports.reported_by_id AS payment_reports_reported_by_id,
			        payment_reports.amount_vnd AS payment_reports_amount_vnd,
			        payment_reports.idempotency_key AS payment_reports_idempotency_key,
			        payment_reports.reported_at AS payment_reports_reported_at
			   FROM payment_reports
			  WHERE payment_reports.id = $1::UUID`, *in.PaymentReportID).
			Scan(&id, &obligation, &link, &reporter, &reportAmount, &reportKey, &reportedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return ReceiptRecord{}, err
		}
		if err != nil || obligation != in.Target.ObligationID {
			return ReceiptRecord{}, &Conflict{Code: "PAYMENT_REPORT_NOT_FOR_OBLIGATION"}
		}
	}

	id, err := newUUID()
	if err != nil {
		return ReceiptRecord{}, err
	}
	now := pythonInstant(in.Now)
	// A Go string travels in text format, so PostgreSQL itself refuses an
	// amount past BIGINT with 22003, as it refuses psycopg's numeric.
	var amountParam any = in.AmountVND.String()
	if in.AmountVND.IsInt64() {
		amountParam = in.AmountVND.Int64()
	}
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO receipt_confirmations (id, obligation_id, payment_report_id, confirmed_by_id, amount_vnd,
		                                    idempotency_key, confirmed_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::UUID, $5::BIGINT, $6::UUID, $7::TIMESTAMP WITH TIME ZONE)`,
		id, in.Target.ObligationID, in.PaymentReportID, in.ConfirmedByID, amountParam, in.IdempotencyKey,
		now); err != nil {
		return ReceiptRecord{}, err
	}
	requestID := in.IdempotencyKey
	if err := r.insertAudit(ctx, auditEvent{actorID: &in.ConfirmedByID, eventType: "receipt_confirmed",
		aggregateType: "collection_obligation", aggregateID: in.Target.ObligationID, requestID: &requestID,
		occurredAt: now, eventData: map[string]any{"receipt_confirmation_id": id}}); err != nil {
		return ReceiptRecord{}, err
	}
	amounts, err := r.receiptAmounts(ctx, in.Target.ObligationID)
	if err != nil {
		return ReceiptRecord{}, err
	}
	// The INSERT succeeded, so the amount fits BIGINT.
	return ReceiptRecord{ID: id, ObligationID: in.Target.ObligationID, AmountVND: in.AmountVND.Int64(),
		ReceiptAmountsVND: amounts}, nil
}

// currentAllocations is the `current_allocations` subquery: the person's
// confirmed allocations on the newest version of each expense.
const currentAllocations = `(SELECT confirmed_allocations.amount_vnd AS amount_vnd,
        expense_versions.expense_id AS expense_id, expense_versions.paid_by_id AS paid_by_id
   FROM confirmed_allocations
   JOIN expense_versions ON expense_versions.id = confirmed_allocations.expense_version_id
   JOIN (SELECT expense_versions.expense_id AS expense_id, max(expense_versions.version_number) AS version_number
           FROM expense_versions GROUP BY expense_versions.expense_id) AS anon_2
     ON anon_2.expense_id = expense_versions.expense_id AND anon_2.version_number = expense_versions.version_number
  WHERE confirmed_allocations.participant_id = $2::UUID) AS anon_1`

// PersonFinanceSummary is person_finance_summary.
//
// Statements, in Python's order: `session.get(Person, id)`; the person's
// spend (SUM over currentAllocations, coalesced to 0); the count of distinct
// expenses in it; the count of their ACTIVE memberships (left_at not read);
// what they owe (the spend on expenses somebody else paid); what they paid
// (receipts on obligations they send); what they advanced (other people's
// allocations on the newest versions of expenses they paid); what they
// collected (receipts on obligations they receive); then movements.
//
// outstanding = max(0, owed - paid), settled = spend - outstanding,
// receivable = max(0, advanced - collected), every term exact.
func (r Repository) PersonFinanceSummary(ctx context.Context, personID string, movementLimit int64) (PersonFinanceSummary, error) {
	person, err := r.GetPerson(ctx, personID)
	if err != nil {
		return PersonFinanceSummary{}, err
	}
	out := PersonFinanceSummary{PersonID: personID}
	if person != nil {
		name := person.DisplayName
		out.DisplayName = &name
	}
	sum := func(sql string, args ...any) (*big.Int, error) {
		return wholeNumber(r.Q.QueryRow(ctx, sql, args...))
	}
	if out.SpendVND, err = sum(`SELECT coalesce(sum(anon_1.amount_vnd), $1::INTEGER) AS coalesce_1 FROM `+
		currentAllocations, int32(0), personID); err != nil {
		return PersonFinanceSummary{}, err
	}
	if err := r.Q.QueryRow(ctx, `SELECT count(distinct(anon_1.expense_id)) AS count_1 FROM `+
		currentAllocationsFrom(1), personID).Scan(&out.ExpenseCount); err != nil {
		return PersonFinanceSummary{}, err
	}
	if err := r.Q.QueryRow(ctx,
		`SELECT count(memberships.id) AS count_1
		   FROM memberships
		  WHERE memberships.person_id = $1::UUID AND memberships.state = $2`, personID, "active").
		Scan(&out.GroupCount); err != nil {
		return PersonFinanceSummary{}, err
	}
	owed, err := sum(`SELECT coalesce(sum(anon_1.amount_vnd), $1::INTEGER) AS coalesce_1 FROM `+
		currentAllocations+` WHERE anon_1.paid_by_id != $3::UUID`, int32(0), personID, personID)
	if err != nil {
		return PersonFinanceSummary{}, err
	}
	paid, err := sum(
		`SELECT coalesce(sum(receipt_confirmations.amount_vnd), $1::INTEGER) AS coalesce_1
		   FROM receipt_confirmations
		   JOIN collection_obligations ON collection_obligations.id = receipt_confirmations.obligation_id
		  WHERE collection_obligations.sender_id = $2::UUID`, int32(0), personID)
	if err != nil {
		return PersonFinanceSummary{}, err
	}
	advanced, err := sum(
		`SELECT coalesce(sum(anon_1.amount_vnd), $1::INTEGER) AS coalesce_1
		   FROM (SELECT confirmed_allocations.amount_vnd AS amount_vnd
		           FROM confirmed_allocations
		           JOIN expense_versions ON expense_versions.id = confirmed_allocations.expense_version_id
		           JOIN (SELECT expense_versions.expense_id AS expense_id,
		                        max(expense_versions.version_number) AS version_number
		                   FROM expense_versions GROUP BY expense_versions.expense_id) AS anon_2
		             ON anon_2.expense_id = expense_versions.expense_id
		            AND anon_2.version_number = expense_versions.version_number
		          WHERE expense_versions.paid_by_id = $2::UUID
		            AND confirmed_allocations.participant_id != $3::UUID) AS anon_1`, int32(0), personID, personID)
	if err != nil {
		return PersonFinanceSummary{}, err
	}
	collected, err := sum(
		`SELECT coalesce(sum(receipt_confirmations.amount_vnd), $1::INTEGER) AS coalesce_1
		   FROM receipt_confirmations
		   JOIN collection_obligations ON collection_obligations.id = receipt_confirmations.obligation_id
		  WHERE collection_obligations.recipient_id = $2::UUID`, int32(0), personID)
	if err != nil {
		return PersonFinanceSummary{}, err
	}
	out.OutstandingVND = clampAtZero(new(big.Int).Sub(owed, paid))
	out.SettledVND = new(big.Int).Sub(out.SpendVND, out.OutstandingVND)
	out.ReceivableVND = clampAtZero(new(big.Int).Sub(advanced, collected))
	if out.Movements, err = r.financeMovements(ctx, personID, movementLimit); err != nil {
		return PersonFinanceSummary{}, err
	}
	return out, nil
}

// currentAllocationsFrom is currentAllocations with its person bound at $n.
func currentAllocationsFrom(n int) string {
	if n == 2 {
		return currentAllocations
	}
	return `(SELECT confirmed_allocations.amount_vnd AS amount_vnd,
        expense_versions.expense_id AS expense_id, expense_versions.paid_by_id AS paid_by_id
   FROM confirmed_allocations
   JOIN expense_versions ON expense_versions.id = confirmed_allocations.expense_version_id
   JOIN (SELECT expense_versions.expense_id AS expense_id, max(expense_versions.version_number) AS version_number
           FROM expense_versions GROUP BY expense_versions.expense_id) AS anon_2
     ON anon_2.expense_id = expense_versions.expense_id AND anon_2.version_number = expense_versions.version_number
  WHERE confirmed_allocations.participant_id = $` + itoa(n) + `::UUID) AS anon_1`
}

func clampAtZero(n *big.Int) *big.Int {
	if n.Sign() < 0 {
		return new(big.Int)
	}
	return n
}

// financeMovements is `_finance_movements`: the receipts on obligations the
// person sends or receives, newest first (confirmed_at DESC, id), LIMIT, the
// group name through an outer join; all rows read, then per row the
// counterparty's `session.get(Person, id)` and `_obligation_occasion`.
//
// SQLAlchemy note: the counterparty loaded for one row is still referenced
// when the next row asks for the same person, so the identity map answers
// without a statement; a person asked for again after somebody else, or one
// with no row, is read again.
func (r Repository) financeMovements(ctx context.Context, personID string, limit int64) ([]FinanceMovement, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT collection_obligations.id, collection_obligations.sender_id, collection_obligations.recipient_id,
		        receipt_confirmations.amount_vnd, receipt_confirmations.confirmed_at, collection_batches.context_id,
		        contexts.display_name
		   FROM receipt_confirmations
		   JOIN collection_obligations ON collection_obligations.id = receipt_confirmations.obligation_id
		   JOIN collection_batch_versions ON collection_batch_versions.id = collection_obligations.batch_version_id
		   JOIN collection_batches ON collection_batches.id = collection_batch_versions.batch_id
		   LEFT OUTER JOIN contexts ON contexts.id = collection_batches.context_id
		  WHERE collection_obligations.sender_id = $1::UUID OR collection_obligations.recipient_id = $2::UUID
		  ORDER BY receipt_confirmations.confirmed_at DESC, receipt_confirmations.id
		  LIMIT $3::INTEGER`, personID, personID, sqlInteger(limit))
	if err != nil {
		return nil, err
	}
	type movementRow struct {
		movement          FinanceMovement
		sender, recipient string
	}
	var found []movementRow
	for rows.Next() {
		var m movementRow
		if err := rows.Scan(&m.movement.ObligationID, &m.sender, &m.recipient, &m.movement.AmountVND,
			&m.movement.OccurredAt, &m.movement.ContextID, &m.movement.ContextName); err != nil {
			rows.Close()
			return nil, err
		}
		m.movement.OccurredAt = m.movement.OccurredAt.UTC()
		found = append(found, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := []FinanceMovement{}
	var held *Person
	for _, row := range found {
		m := row.movement
		m.Direction, m.CounterpartyID = "in", row.sender
		if row.sender == personID {
			m.Direction, m.CounterpartyID = "out", row.recipient
		}
		if held == nil || held.ID != m.CounterpartyID {
			if held, err = r.GetPerson(ctx, m.CounterpartyID); err != nil {
				return nil, err
			}
		}
		if held != nil {
			name := held.DisplayName
			m.CounterpartyName = &name
		}
		if m.Occasion, err = r.obligationOccasion(ctx, m.ObligationID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// obligationOccasion is `_obligation_occasion`: the descriptions of the
// expense versions behind the obligation's sources ORDER BY occurred_at (ties
// in scan order); the non-empty ones, first occurrence kept; nil for none,
// the one for one, "<first> +<others>" otherwise.
func (r Repository) obligationOccasion(ctx context.Context, obligationID string) (*string, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT expense_versions.description
		   FROM collection_obligation_sources
		   JOIN confirmed_allocations ON confirmed_allocations.id = collection_obligation_sources.confirmed_allocation_id
		   JOIN expense_versions ON expense_versions.id = confirmed_allocations.expense_version_id
		  WHERE collection_obligation_sources.obligation_id = $1::UUID
		  ORDER BY expense_versions.occurred_at`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var unique []string
	seen := map[string]bool{}
	for rows.Next() {
		var description *string
		if err := rows.Scan(&description); err != nil {
			return nil, err
		}
		if description != nil && *description != "" && !seen[*description] {
			seen[*description] = true
			unique = append(unique, *description)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch len(unique) {
	case 0:
		return nil, nil
	case 1:
		return &unique[0], nil
	}
	text := unique[0] + " +" + itoa(len(unique)-1)
	return &text, nil
}
