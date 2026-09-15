package repo

// POST /expenses and POST /expenses/{id}/confirm: create_expense, get_expense
// and save_expense_confirmation. Every amount stored here is integer dong in
// a BIGINT column; the rollups arrive already summed by the caller.

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// ExpenseIdentity is ExpenseIdentity.
type ExpenseIdentity struct {
	ID        string
	ContextID string
}

// ExpenseItemInput is one ExpenseItemInput of the proposal.
type ExpenseItemInput struct {
	ItemID    string
	Label     *string
	AmountVND int64
	SharedBy  []string
}

// ExpenseSurchargeInput is one ExpenseSurchargeInput of the proposal.
type ExpenseSurchargeInput struct {
	SurchargeID string
	Kind        string
	AmountVND   int64
	Mode        string
}

// ExpenseDiscountInput is one ExpenseDiscountInput of the proposal.
type ExpenseDiscountInput struct {
	DiscountID string
	AmountVND  int64
	Scope      string
	ItemID     *string
}

// ExpenseProposal is the part of ExpenseInput save_expense_confirmation reads.
type ExpenseProposal struct {
	Description       *string
	RecordedByID      string
	PaidByID          string
	VerificationScope string
	OccurredAt        time.Time
	Items             []ExpenseItemInput
	Surcharges        []ExpenseSurchargeInput
	Discounts         []ExpenseDiscountInput
}

// ExpenseRollups is component_rollups' dict, spread into ExpenseVersion.
type ExpenseRollups struct {
	SubtotalVND, FeeVND, VATVND, ShippingVND, DiscountVND, TotalVND int64
}

// ParticipantAmount is one entry of the expected allocations dict.
type ParticipantAmount struct {
	ParticipantID string
	AmountVND     int64
}

// ExpenseConfirmation is save_expense_confirmation's keyword arguments.
// AllocatorWarnings is `allocator_expense.get("warnings", [])`.
type ExpenseConfirmation struct {
	ExpenseID            string
	Proposal             ExpenseProposal
	AllocatorWarnings    []string
	Rollups              ExpenseRollups
	Allocations          []ParticipantAmount
	ConfirmedByID        string
	PayerAcknowledgement string
	Now                  time.Time
}

// ConfirmationRecord is ConfirmationRecord.
type ConfirmationRecord struct {
	ExpenseVersionID string
	VersionNumber    int64
}

// CreateExpense is create_expense: one INSERT with a client-side uuid4,
// created_at left to the server and read back with RETURNING. A context with
// no row is fk_expenses_context_id, raised as Conflict
// EXPENSE_CONTEXT_NOT_FOUND from that violation; any other error is returned
// as is. There is no savepoint: the transaction is aborted either way.
func (r Repository) CreateExpense(ctx context.Context, contextID string) (ExpenseIdentity, error) {
	id, err := newUUID()
	if err != nil {
		return ExpenseIdentity{}, err
	}
	var created time.Time
	err = r.Q.QueryRow(ctx,
		`INSERT INTO expenses (id, context_id) VALUES ($1::UUID, $2::UUID) RETURNING expenses.created_at`,
		id, contextID).Scan(&created)
	if err != nil {
		if pg := integrityViolation(err); pg != nil && pg.ConstraintName == "fk_expenses_context_id" {
			return ExpenseIdentity{}, &Conflict{Code: "EXPENSE_CONTEXT_NOT_FOUND", Err: pg}
		}
		return ExpenseIdentity{}, err
	}
	return ExpenseIdentity{ID: id, ContextID: contextID}, nil
}

// GetExpense is get_expense: the expense row by id, SELECT ... FOR UPDATE,
// nil when there is none.
func (r Repository) GetExpense(ctx context.Context, expenseID string) (*ExpenseIdentity, error) {
	var e ExpenseIdentity
	var created time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT expenses.id, expenses.context_id, expenses.created_at
		   FROM expenses
		  WHERE expenses.id = $1::UUID FOR UPDATE`, expenseID).Scan(&e.ID, &e.ContextID, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

var (
	expenseVersionInsert = []insertColumn{{"id", "::UUID"}, {"expense_id", "::UUID"}, {"version_number", "::INTEGER"},
		{"previous_version_number", "::INTEGER"}, {"description", "::VARCHAR"}, {"recorded_by_id", "::UUID"},
		{"paid_by_id", "::UUID"}, {"payer_acknowledgement", ""}, {"verification_scope", ""},
		{"subtotal_amount_vnd", "::BIGINT"}, {"fee_amount_vnd", "::BIGINT"}, {"vat_amount_vnd", "::BIGINT"},
		{"shipping_amount_vnd", "::BIGINT"}, {"discount_amount_vnd", "::BIGINT"}, {"total_amount_vnd", "::BIGINT"},
		{"occurred_at", "::TIMESTAMP WITH TIME ZONE"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}}
	expenseItemInsert = []insertColumn{{"id", "::UUID"}, {"expense_version_id", "::UUID"}, {"item_key", "::VARCHAR"},
		{"label", "::VARCHAR"}, {"amount_vnd", "::BIGINT"}}
	confirmedAllocationInsert = []insertColumn{{"id", "::UUID"}, {"expense_version_id", "::UUID"},
		{"participant_id", "::UUID"}, {"amount_vnd", "::BIGINT"}, {"confirmed_by_id", "::UUID"},
		{"confirmed_at", "::TIMESTAMP WITH TIME ZONE"}}
	expenseDiscountInsert = []insertColumn{{"id", "::UUID"}, {"expense_version_id", "::UUID"},
		{"discount_key", "::VARCHAR"}, {"amount_vnd", "::BIGINT"}, {"scope", ""}, {"target_item_id", "::UUID"}}
	expenseItemShareInsert = []insertColumn{{"id", "::UUID"}, {"expense_item_id", "::UUID"},
		{"participant_id", "::UUID"}}
	expenseSurchargeInsert = []insertColumn{{"id", "::UUID"}, {"expense_version_id", "::UUID"},
		{"surcharge_key", "::VARCHAR"}, {"kind", "::VARCHAR"}, {"amount_vnd", "::BIGINT"}, {"mode", ""}}
)

func uuids(n int) ([]string, error) {
	out := make([]string, n)
	for i := range out {
		id, err := newUUID()
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

// SaveExpenseConfirmation is save_expense_confirmation: a new immutable
// version of the expense with its lines and the confirmed allocations.
//
// Statements and refusals, in Python's order:
//  1. the expense by id, SELECT ... FOR UPDATE; none is Conflict
//     EXPENSE_NOT_FOUND raised from nothing;
//  2. `SELECT max(version_number)` of the expense, unlocked; the new version
//     is that plus one (1 when there is none), chained to it;
//  3. `PayerAcknowledgement(...)` then `VerificationScope(...)`: ValueError
//     before any write;
//  4. flush: the version INSERT (every column, the caller's clock);
//  5. flush: the items, one insertmanyvalues INSERT; a repeated item_id is
//     uq_expense_items_version_key and returned as is;
//  6. flush, in mapper order: the audit event, the allocations in participant
//     UUID order, the discounts (an item-scoped discount points at the last
//     item with its key, NULL when none has it), the item shares item by item,
//     the surcharges. A discount scope or surcharge mode outside its enum
//     stops that table's INSERT with ErrNotAnEnumValue, the tables before it
//     already written. Every PostgreSQL refusal is returned as is.
func (r Repository) SaveExpenseConfirmation(ctx context.Context, in ExpenseConfirmation) (ConfirmationRecord, error) {
	expense, err := r.GetExpense(ctx, in.ExpenseID)
	if err != nil {
		return ConfirmationRecord{}, err
	}
	if expense == nil {
		return ConfirmationRecord{}, &Conflict{Code: "EXPENSE_NOT_FOUND"}
	}
	var latest *int64
	if err := r.Q.QueryRow(ctx,
		`SELECT max(expense_versions.version_number) AS max_1
		   FROM expense_versions
		  WHERE expense_versions.expense_id = $1::UUID`, in.ExpenseID).Scan(&latest); err != nil {
		return ConfirmationRecord{}, err
	}
	versionNumber := int64(1)
	var previous any
	if latest != nil {
		versionNumber = *latest + 1
		previous = sqlInteger(*latest)
	}
	if !payerAcknowledgements[in.PayerAcknowledgement] {
		return ConfirmationRecord{}, ErrUnknownPayerAcknowledgement
	}
	p := in.Proposal
	if !verificationScopes[p.VerificationScope] {
		return ConfirmationRecord{}, ErrUnknownVerificationScope
	}

	versionID, err := newUUID()
	if err != nil {
		return ConfirmationRecord{}, err
	}
	now := pythonInstant(in.Now)
	rollups := in.Rollups
	if _, err := r.Q.Exec(ctx, renderInsert("expense_versions", expenseVersionInsert, 1),
		versionID, in.ExpenseID, sqlInteger(versionNumber), previous, p.Description, p.RecordedByID, p.PaidByID,
		in.PayerAcknowledgement, p.VerificationScope, rollups.SubtotalVND, rollups.FeeVND, rollups.VATVND,
		rollups.ShippingVND, rollups.DiscountVND, rollups.TotalVND, p.OccurredAt, now); err != nil {
		return ConfirmationRecord{}, err
	}

	itemIDs, err := uuids(len(p.Items))
	if err != nil {
		return ConfirmationRecord{}, err
	}
	itemByKey := map[string]string{}
	var items [][]any
	for i, item := range p.Items {
		items = append(items, []any{itemIDs[i], versionID, item.ItemID, item.Label, item.AmountVND})
		itemByKey[item.ItemID] = itemIDs[i]
	}
	if err := r.insertManyValues(ctx, "expense_items", expenseItemInsert, items); err != nil {
		return ConfirmationRecord{}, err
	}

	warnings := append([]string{}, in.AllocatorWarnings...)
	if err := r.insertAudit(ctx, auditEvent{actorID: &in.ConfirmedByID, eventType: "expense_confirmed",
		aggregateType: "expense", aggregateID: in.ExpenseID, occurredAt: now,
		eventData: map[string]any{"expense_version_id": versionID, "version_number": versionNumber,
			"allocator_warnings": warnings}}); err != nil {
		return ConfirmationRecord{}, err
	}

	allocations := append([]ParticipantAmount{}, in.Allocations...)
	sort.SliceStable(allocations, func(i, j int) bool { return allocations[i].ParticipantID < allocations[j].ParticipantID })
	allocationIDs, err := uuids(len(allocations))
	if err != nil {
		return ConfirmationRecord{}, err
	}
	var allocationRows [][]any
	for i, a := range allocations {
		allocationRows = append(allocationRows, []any{allocationIDs[i], versionID, a.ParticipantID, a.AmountVND,
			in.ConfirmedByID, now})
	}
	if err := r.insertManyValues(ctx, "confirmed_allocations", confirmedAllocationInsert, allocationRows); err != nil {
		return ConfirmationRecord{}, err
	}

	discountIDs, err := uuids(len(p.Discounts))
	if err != nil {
		return ConfirmationRecord{}, err
	}
	var discounts [][]any
	for i, d := range p.Discounts {
		if !discountScopes[d.Scope] {
			return ConfirmationRecord{}, ErrNotAnEnumValue
		}
		var target any
		if d.ItemID != nil {
			if id, ok := itemByKey[*d.ItemID]; ok {
				target = id
			}
		}
		discounts = append(discounts, []any{discountIDs[i], versionID, d.DiscountID, d.AmountVND, d.Scope, target})
	}
	if err := r.insertManyValues(ctx, "expense_discounts", expenseDiscountInsert, discounts); err != nil {
		return ConfirmationRecord{}, err
	}

	var shares [][]any
	for _, item := range p.Items {
		for _, participant := range item.SharedBy {
			id, err := newUUID()
			if err != nil {
				return ConfirmationRecord{}, err
			}
			shares = append(shares, []any{id, itemByKey[item.ItemID], participant})
		}
	}
	if err := r.insertManyValues(ctx, "expense_item_shares", expenseItemShareInsert, shares); err != nil {
		return ConfirmationRecord{}, err
	}

	surchargeIDs, err := uuids(len(p.Surcharges))
	if err != nil {
		return ConfirmationRecord{}, err
	}
	var surcharges [][]any
	for i, s := range p.Surcharges {
		if !surchargeModes[s.Mode] {
			return ConfirmationRecord{}, ErrNotAnEnumValue
		}
		surcharges = append(surcharges, []any{surchargeIDs[i], versionID, s.SurchargeID, s.Kind, s.AmountVND, s.Mode})
	}
	if err := r.insertManyValues(ctx, "expense_surcharges", expenseSurchargeInsert, surcharges); err != nil {
		return ConfirmationRecord{}, err
	}
	return ConfirmationRecord{ExpenseVersionID: versionID, VersionNumber: versionNumber}, nil
}
