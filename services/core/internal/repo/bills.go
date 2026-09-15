package repo

// The scanned-bill drafts behind /bills: create_bill, get_bill,
// confirm_bill_assignments and claim_bill_items, with `_bill_record`. A bill
// is a draft, never the ledger; nothing here reaches confirmed_allocations.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Bill is BillRecord.
type Bill struct {
	ID              string
	ContextID       string
	PrintedTotalVND *int64
	ItemsTotalVND   int64
	Confidence      int64
	NeedsReview     bool
	CreatedByID     string
	CreatedAt       time.Time
	Items           []BillItem
	Surcharges      []BillSurcharge
	Discounts       []BillDiscount
}

// BillItem is BillItemRecord.
type BillItem struct {
	ItemKey      string
	Name         string
	Quantity     int64
	UnitPriceVND *int64
	LineTotalVND int64
	Position     int64
	Shares       []BillShare
}

// BillShare is BillShareRecord.
type BillShare struct {
	ParticipantID string
	Source        string
	DecidedByID   *string
	DecidedAt     *time.Time
}

// BillSurcharge is BillSurchargeRecord, and one surcharge dict create_bill takes.
type BillSurcharge struct {
	SurchargeKey string
	Kind         string
	AmountVND    int64
	Mode         string
}

// BillDiscount is BillDiscountRecord, and one discount dict create_bill takes.
type BillDiscount struct {
	DiscountKey   string
	AmountVND     int64
	Scope         string
	TargetItemKey *string
}

// BillItemInput is one item dict create_bill takes.
type BillItemInput struct {
	ItemKey                 string
	Name                    string
	Quantity                int64
	UnitPriceVND            *int64
	LineTotalVND            int64
	Position                int64
	SuggestedParticipantIDs []string
}

// BillInput is create_bill's keyword arguments.
type BillInput struct {
	ContextID       string
	CreatedByID     string
	PrintedTotalVND *int64
	ItemsTotalVND   int64
	Confidence      int64
	NeedsReview     bool
	Items           []BillItemInput
	Surcharges      []BillSurcharge
	Discounts       []BillDiscount
	Now             time.Time
}

// BillAssignment is one assignment dict confirm_bill_assignments takes.
type BillAssignment struct {
	ItemKey        string
	ParticipantIDs []string
}

// billWriteConflicts is _BILL_WRITE_CONFLICTS; any other integrity violation
// of create_bill is BILL_WRITE_CONFLICT.
var billWriteConflicts = map[string]string{
	"uq_bill_items_bill_item_key":           "DUPLICATE_BILL_ITEM_KEY",
	"uq_bill_surcharges_bill_surcharge_key": "DUPLICATE_BILL_SURCHARGE_KEY",
	"uq_bill_discounts_bill_discount_key":   "DUPLICATE_BILL_DISCOUNT_KEY",
}

// billSavepoint is create_bill's begin_nested, the first savepoint of the
// connection on POST /bills.
const billSavepoint = "sa_savepoint_1"

const billColumns = `bills.id, bills.context_id, bills.created_by_id, bills.printed_total_vnd, bills.items_total_vnd,
       bills.confidence, bills.needs_review, bills.created_at`

const billItemColumns = `bill_items.id, bill_items.bill_id, bill_items.item_key, bill_items.name, bill_items.quantity,
       bill_items.unit_price_vnd, bill_items.line_total_vnd, bill_items.position`

const billShareColumns = `bill_item_shares.id, bill_item_shares.bill_item_id, bill_item_shares.participant_id,
       bill_item_shares.source, bill_item_shares.decided_by_id, bill_item_shares.decided_at`

var (
	billInsert = []insertColumn{{"id", "::UUID"}, {"context_id", "::UUID"}, {"created_by_id", "::UUID"},
		{"printed_total_vnd", "::BIGINT"}, {"items_total_vnd", "::BIGINT"}, {"confidence", "::INTEGER"},
		{"needs_review", ""}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}}
	billItemInsert = []insertColumn{{"id", "::UUID"}, {"bill_id", "::UUID"}, {"item_key", "::VARCHAR"},
		{"name", "::VARCHAR"}, {"quantity", "::INTEGER"}, {"unit_price_vnd", "::BIGINT"},
		{"line_total_vnd", "::BIGINT"}, {"position", "::INTEGER"}}
	billDiscountInsert = []insertColumn{{"id", "::UUID"}, {"bill_id", "::UUID"}, {"discount_key", "::VARCHAR"},
		{"amount_vnd", "::BIGINT"}, {"scope", ""}, {"target_item_key", "::VARCHAR"}}
	billShareInsert = []insertColumn{{"id", "::UUID"}, {"bill_item_id", "::UUID"}, {"participant_id", "::UUID"},
		{"source", ""}, {"decided_by_id", "::UUID"}, {"decided_at", "::TIMESTAMP WITH TIME ZONE"}}
	billSurchargeInsert = []insertColumn{{"id", "::UUID"}, {"bill_id", "::UUID"}, {"surcharge_key", "::VARCHAR"},
		{"kind", "::VARCHAR"}, {"amount_vnd", "::BIGINT"}, {"mode", ""}}
)

func scanBill(row pgx.Row) (*Bill, error) {
	var b Bill
	err := row.Scan(&b.ID, &b.ContextID, &b.CreatedByID, &b.PrintedTotalVND, &b.ItemsTotalVND, &b.Confidence,
		&b.NeedsReview, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.CreatedAt = b.CreatedAt.UTC()
	return &b, nil
}

type billItemRow struct {
	id, itemKey string
	item        BillItem
}

func (r Repository) billItemRows(ctx context.Context, sql string, args ...any) ([]billItemRow, error) {
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []billItemRow
	for rows.Next() {
		var row billItemRow
		var billID string
		if err := rows.Scan(&row.id, &billID, &row.item.ItemKey, &row.item.Name, &row.item.Quantity,
			&row.item.UnitPriceVND, &row.item.LineTotalVND, &row.item.Position); err != nil {
			return nil, err
		}
		row.itemKey = row.item.ItemKey
		out = append(out, row)
	}
	return out, rows.Err()
}

type billShareRow struct {
	id, itemID string
	share      BillShare
}

func (r Repository) billShareRows(ctx context.Context, sql string, args ...any) ([]billShareRow, error) {
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []billShareRow
	for rows.Next() {
		var row billShareRow
		if err := rows.Scan(&row.id, &row.itemID, &row.share.ParticipantID, &row.share.Source,
			&row.share.DecidedByID, &row.share.DecidedAt); err != nil {
			return nil, err
		}
		row.share.DecidedAt = utcOptional(row.share.DecidedAt)
		out = append(out, row)
	}
	return out, rows.Err()
}

// billRecord is `_bill_record(bill)`: the items ORDER BY position, item_key;
// only when there are items, their shares ORDER BY bill_item_id,
// participant_id; the surcharges ORDER BY surcharge_key; the discounts ORDER
// BY discount_key. The header fields are the ones handed in.
func (r Repository) billRecord(ctx context.Context, head Bill) (Bill, error) {
	items, err := r.billItemRows(ctx,
		`SELECT `+billItemColumns+`
		   FROM bill_items
		  WHERE bill_items.bill_id = $1::UUID
		  ORDER BY bill_items.position, bill_items.item_key`, head.ID)
	if err != nil {
		return Bill{}, err
	}
	out := head
	out.Items, out.Surcharges, out.Discounts = []BillItem{}, []BillSurcharge{}, []BillDiscount{}
	index := map[string]int{}
	ids := make([]string, 0, len(items))
	for i, row := range items {
		row.item.Shares = []BillShare{}
		out.Items = append(out.Items, row.item)
		index[row.id] = i
		ids = append(ids, row.id)
	}
	if len(items) > 0 {
		shares, err := r.billShareRows(ctx,
			`SELECT `+billShareColumns+`
			   FROM bill_item_shares
			  WHERE bill_item_shares.bill_item_id IN (`+uuidPlaceholders(1, len(ids))+`)
			  ORDER BY bill_item_shares.bill_item_id, bill_item_shares.participant_id`, uuidArgs(ids)...)
		if err != nil {
			return Bill{}, err
		}
		for _, row := range shares {
			i := index[row.itemID]
			out.Items[i].Shares = append(out.Items[i].Shares, row.share)
		}
	}

	rows, err := r.Q.Query(ctx,
		`SELECT bill_surcharges.id, bill_surcharges.bill_id, bill_surcharges.surcharge_key, bill_surcharges.kind,
		        bill_surcharges.amount_vnd, bill_surcharges.mode
		   FROM bill_surcharges
		  WHERE bill_surcharges.bill_id = $1::UUID
		  ORDER BY bill_surcharges.surcharge_key`, head.ID)
	if err != nil {
		return Bill{}, err
	}
	for rows.Next() {
		var s BillSurcharge
		var id, billID string
		if err := rows.Scan(&id, &billID, &s.SurchargeKey, &s.Kind, &s.AmountVND, &s.Mode); err != nil {
			rows.Close()
			return Bill{}, err
		}
		out.Surcharges = append(out.Surcharges, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Bill{}, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT bill_discounts.id, bill_discounts.bill_id, bill_discounts.discount_key, bill_discounts.amount_vnd,
		        bill_discounts.scope, bill_discounts.target_item_key
		   FROM bill_discounts
		  WHERE bill_discounts.bill_id = $1::UUID
		  ORDER BY bill_discounts.discount_key`, head.ID)
	if err != nil {
		return Bill{}, err
	}
	for rows.Next() {
		var d BillDiscount
		var id, billID string
		if err := rows.Scan(&id, &billID, &d.DiscountKey, &d.AmountVND, &d.Scope, &d.TargetItemKey); err != nil {
			rows.Close()
			return Bill{}, err
		}
		out.Discounts = append(out.Discounts, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Bill{}, err
	}
	return out, nil
}

// CreateBill is create_bill: the whole draft inside one savepoint.
//
// Statements, in Python's order:
//  1. SAVEPOINT;
//  2. flush: the bill INSERT (the caller's clock, no RETURNING);
//  3. flush: the items, one insertmanyvalues INSERT, none without items;
//  4. flush, in mapper order: the discounts, the suggested shares item by item
//     (source ai_suggested, no decision), the surcharges. A discount scope or
//     surcharge mode outside its enum stops that table with ErrNotAnEnumValue;
//  5. RELEASE SAVEPOINT, then `_bill_record` with the header as written.
//
// On any failure the savepoint is rolled back first. An integrity violation
// is then Conflict with the code of its constraint in billWriteConflicts, or
// BILL_WRITE_CONFLICT for every other one (fail closed: a foreign key, a
// check, a repeated share), raised from that violation; anything else (a
// DataError such as a key past varchar(64), ErrNotAnEnumValue) is returned
// as is.
func (r Repository) CreateBill(ctx context.Context, in BillInput) (Bill, error) {
	billID, err := newUUID()
	if err != nil {
		return Bill{}, err
	}
	itemIDs, err := uuids(len(in.Items))
	if err != nil {
		return Bill{}, err
	}
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+billSavepoint); err != nil {
		return Bill{}, err
	}
	fail := func(cause error) (Bill, error) {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+billSavepoint); err != nil {
			return Bill{}, err
		}
		if pg := integrityViolation(cause); pg != nil {
			code, named := billWriteConflicts[pg.ConstraintName]
			if !named {
				code = "BILL_WRITE_CONFLICT"
			}
			return Bill{}, &Conflict{Code: code, Err: pg}
		}
		return Bill{}, cause
	}

	created := pythonInstant(in.Now)
	if _, err := r.Q.Exec(ctx, renderInsert("bills", billInsert, 1), billID, in.ContextID, in.CreatedByID,
		in.PrintedTotalVND, in.ItemsTotalVND, sqlInteger(in.Confidence), in.NeedsReview, created); err != nil {
		return fail(err)
	}
	var items [][]any
	for i, item := range in.Items {
		items = append(items, []any{itemIDs[i], billID, item.ItemKey, item.Name, sqlInteger(item.Quantity),
			item.UnitPriceVND, item.LineTotalVND, sqlInteger(item.Position)})
	}
	if err := r.insertManyValues(ctx, "bill_items", billItemInsert, items); err != nil {
		return fail(err)
	}

	var discounts [][]any
	for _, d := range in.Discounts {
		if !discountScopes[d.Scope] {
			return fail(ErrNotAnEnumValue)
		}
		id, err := newUUID()
		if err != nil {
			return fail(err)
		}
		discounts = append(discounts, []any{id, billID, d.DiscountKey, d.AmountVND, d.Scope, d.TargetItemKey})
	}
	if err := r.insertManyValues(ctx, "bill_discounts", billDiscountInsert, discounts); err != nil {
		return fail(err)
	}
	var shares [][]any
	for i, item := range in.Items {
		for _, participant := range item.SuggestedParticipantIDs {
			id, err := newUUID()
			if err != nil {
				return fail(err)
			}
			shares = append(shares, []any{id, itemIDs[i], participant, "ai_suggested", nil, nil})
		}
	}
	if err := r.insertManyValues(ctx, "bill_item_shares", billShareInsert, shares); err != nil {
		return fail(err)
	}
	var surcharges [][]any
	for _, s := range in.Surcharges {
		if !surchargeModes[s.Mode] {
			return fail(ErrNotAnEnumValue)
		}
		id, err := newUUID()
		if err != nil {
			return fail(err)
		}
		surcharges = append(surcharges, []any{id, billID, s.SurchargeKey, s.Kind, s.AmountVND, s.Mode})
	}
	if err := r.insertManyValues(ctx, "bill_surcharges", billSurchargeInsert, surcharges); err != nil {
		return fail(err)
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+billSavepoint); err != nil {
		return Bill{}, err
	}
	return r.billRecord(ctx, Bill{ID: billID, ContextID: in.ContextID, PrintedTotalVND: in.PrintedTotalVND,
		ItemsTotalVND: in.ItemsTotalVND, Confidence: in.Confidence, NeedsReview: in.NeedsReview,
		CreatedByID: in.CreatedByID, CreatedAt: created})
}

// GetBill is get_bill: `session.get(Bill, id)` (labelled, no lock), nil when
// there is none, then `_bill_record`.
func (r Repository) GetBill(ctx context.Context, billID string) (*Bill, error) {
	head, err := scanBill(r.Q.QueryRow(ctx,
		`SELECT bills.id AS bills_id, bills.context_id AS bills_context_id, bills.created_by_id AS bills_created_by_id,
		        bills.printed_total_vnd AS bills_printed_total_vnd, bills.items_total_vnd AS bills_items_total_vnd,
		        bills.confidence AS bills_confidence, bills.needs_review AS bills_needs_review,
		        bills.created_at AS bills_created_at
		   FROM bills
		  WHERE bills.id = $1::UUID`, billID))
	if err != nil || head == nil {
		return nil, err
	}
	record, err := r.billRecord(ctx, *head)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// lockedBill is the first two statements both assignment writes issue: the
// bill FOR UPDATE (Conflict BILL_NOT_FOUND when there is none) and every item
// of it FOR UPDATE, in no order.
func (r Repository) lockedBill(ctx context.Context, billID string) (*Bill, []billItemRow, error) {
	head, err := scanBill(r.Q.QueryRow(ctx,
		`SELECT `+billColumns+`
		   FROM bills
		  WHERE bills.id = $1::UUID FOR UPDATE`, billID))
	if err != nil {
		return nil, nil, err
	}
	if head == nil {
		return nil, nil, &Conflict{Code: "BILL_NOT_FOUND"}
	}
	items, err := r.billItemRows(ctx,
		`SELECT `+billItemColumns+`
		   FROM bill_items
		  WHERE bill_items.bill_id = $1::UUID FOR UPDATE`, billID)
	if err != nil {
		return nil, nil, err
	}
	return head, items, nil
}

// ConfirmBillAssignments is confirm_bill_assignments: the table's whole
// answer for the item keys it names.
//
// After lockedBill: assignments are keyed by item_key (a repeated key keeps
// its first position and its last participants); a key the bill does not
// have is Conflict UNKNOWN_BILL_ITEM raised from nothing. With at least one
// key, every share of those items is read FOR UPDATE (no order) and deleted
// (executemany in id order). Then one insertmanyvalues INSERT of the new
// confirmed shares, key by key, participant by participant, decided by the
// caller at the caller's clock (a repeated participant is
// uq_bill_item_shares_item_participant, returned as is), and `_bill_record`.
func (r Repository) ConfirmBillAssignments(ctx context.Context, billID string, assignments []BillAssignment, decidedByID string, now time.Time) (Bill, error) {
	head, items, err := r.lockedBill(ctx, billID)
	if err != nil {
		return Bill{}, err
	}
	itemID := map[string]string{}
	for _, row := range items {
		itemID[row.itemKey] = row.id
	}
	var keys []string
	participants := map[string][]string{}
	for _, a := range assignments {
		if _, seen := participants[a.ItemKey]; !seen {
			keys = append(keys, a.ItemKey)
		}
		participants[a.ItemKey] = append([]string{}, a.ParticipantIDs...)
	}
	for _, key := range keys {
		if _, ok := itemID[key]; !ok {
			return Bill{}, &Conflict{Code: "UNKNOWN_BILL_ITEM"}
		}
	}
	if len(keys) > 0 {
		targets := make([]string, len(keys))
		for i, key := range keys {
			targets[i] = itemID[key]
		}
		existing, err := r.billShareRows(ctx,
			`SELECT `+billShareColumns+`
			   FROM bill_item_shares
			  WHERE bill_item_shares.bill_item_id IN (`+uuidPlaceholders(1, len(targets))+`) FOR UPDATE`,
			uuidArgs(targets)...)
		if err != nil {
			return Bill{}, err
		}
		ids := make([]string, len(existing))
		for i, row := range existing {
			ids[i] = row.id
		}
		if err := r.deleteByID(ctx, "bill_item_shares", ids); err != nil {
			return Bill{}, err
		}
	}
	decided := pythonInstant(now)
	var rows [][]any
	for _, key := range keys {
		for _, participant := range participants[key] {
			id, err := newUUID()
			if err != nil {
				return Bill{}, err
			}
			rows = append(rows, []any{id, itemID[key], participant, "confirmed", decidedByID, decided})
		}
	}
	if err := r.insertManyValues(ctx, "bill_item_shares", billShareInsert, rows); err != nil {
		return Bill{}, err
	}
	return r.billRecord(ctx, *head)
}

// ClaimBillItems is claim_bill_items: one person's own claims across the
// whole bill, the other diners' shares untouched.
//
// After lockedBill: the requested keys de-duplicated in first order; a key
// the bill does not have is Conflict UNKNOWN_BILL_ITEM. When the bill has
// items, this participant's shares on all of them are read FOR UPDATE and
// deleted (executemany in id order). Then one insertmanyvalues INSERT of a
// confirmed share per requested key, decided by the participant, and
// `_bill_record`.
func (r Repository) ClaimBillItems(ctx context.Context, billID, participantID string, itemKeys []string, now time.Time) (Bill, error) {
	head, items, err := r.lockedBill(ctx, billID)
	if err != nil {
		return Bill{}, err
	}
	itemID := map[string]string{}
	for _, row := range items {
		itemID[row.itemKey] = row.id
	}
	var requested []string
	seen := map[string]bool{}
	for _, key := range itemKeys {
		if !seen[key] {
			seen[key] = true
			requested = append(requested, key)
		}
	}
	for _, key := range requested {
		if _, ok := itemID[key]; !ok {
			return Bill{}, &Conflict{Code: "UNKNOWN_BILL_ITEM"}
		}
	}
	if len(items) > 0 {
		all := make([]string, len(items))
		for i, row := range items {
			all[i] = row.id
		}
		mine, err := r.billShareRows(ctx,
			`SELECT `+billShareColumns+`
			   FROM bill_item_shares
			  WHERE bill_item_shares.bill_item_id IN (`+uuidPlaceholders(1, len(all))+`)
			    AND bill_item_shares.participant_id = $`+itoa(len(all)+1)+`::UUID FOR UPDATE`,
			append(uuidArgs(all), participantID)...)
		if err != nil {
			return Bill{}, err
		}
		ids := make([]string, len(mine))
		for i, row := range mine {
			ids[i] = row.id
		}
		if err := r.deleteByID(ctx, "bill_item_shares", ids); err != nil {
			return Bill{}, err
		}
	}
	decided := pythonInstant(now)
	var rows [][]any
	for _, key := range requested {
		id, err := newUUID()
		if err != nil {
			return Bill{}, err
		}
		rows = append(rows, []any{id, itemID[key], participantID, "confirmed", participantID, decided})
	}
	if err := r.insertManyValues(ctx, "bill_item_shares", billShareInsert, rows); err != nil {
		return Bill{}, err
	}
	return r.billRecord(ctx, *head)
}
