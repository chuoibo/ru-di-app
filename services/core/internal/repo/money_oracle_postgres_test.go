//go:build postgres

package repo

// Differential test of the W4 repository methods (the money routes) against
// the real SqlAlchemyApiRepository, driven through
// scripts/render_money_repo_oracle.py. Same design as
// groups_oracle_postgres_test.go, whose case format, value tags, statement
// normalisation, conflict tagging, row lock probe and comparison it reuses,
// with three differences:
//
//   - Exact arguments. The spec is decoded with UseNumber, so an amount at the
//     BIGINT maximum reaches the Go method as the same integer Python reads.
//   - Generated rows. These writes create many rows per table (items, shares,
//     obligations, envelopes), so every dump orders its rows by what the case
//     wrote (fixture rows first, then parent clocks, positions, keys and
//     parties), never by a generated id; generated ids are then bound by order
//     of appearance exactly as before. The row lock probe keeps fixture ids and
//     writes every generated id as <generated>, sorted: a lock on a generated
//     row comes only from a foreign key check of the statement that wrote it.
//   - Identity map. A case loads a given bill, batch or person through
//     session.get at most once per step, which is what each method does.
//
// Without CORE_PYTHON_IMAGE the test skips; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

// probeMoneyRowLocks lists the row locks held on every table a W4 method
// locks FOR UPDATE or references through a foreign key it writes.
var probeMoneyRowLocks = func() string {
	var parts []string
	for _, table := range []string{"people", "contexts", "memberships", "expenses", "expense_versions", "expense_items",
		"confirmed_allocations", "bills", "bill_items", "bill_item_shares", "collection_batches",
		"collection_batch_versions", "collection_obligations", "collection_envelopes", "guest_links", "payment_reports",
		"receipt_confirmations"} {
		parts = append(parts, `SELECT '`+table+`' AS rel, t.id::text AS id, array_to_string(r.modes, ',') AS modes
			FROM public.pgrowlocks('`+table+`') r LEFT JOIN `+table+` t ON t.ctid = r.locked_row`)
	}
	return `SELECT l.rel || ' ' || coalesce(l.id, '(superseded row)') || ' ' || l.modes FROM (` +
		strings.Join(parts, " UNION ALL ") + `) l ORDER BY 1`
}()

// fixtureFirst sorts a row a case seeded before any row a method generated:
// every fixture id carries the fid suffix, and a uuid4 does not.
func fixtureFirst(column string) string {
	return `(` + column + `::text NOT LIKE '%-aaaa-4aaa-8aaa-aaaaaaaa%')`
}

func orderedDump(from, order string) string {
	return `SELECT row_to_json(t)::text FROM ` + from + ` ORDER BY ` + order
}

var (
	expensesDump   = nowDump("expenses", fixtureFirst("t.id")+", t.context_id, t.id")
	versionsDump   = orderedDump("expense_versions t", fixtureFirst("t.id")+", t.created_at, t.version_number, t.id")
	itemsDump      = orderedDump("expense_items t JOIN expense_versions v ON v.id = t.expense_version_id", "v.created_at, v.version_number, t.item_key, t.id")
	itemSharesDump = orderedDump("expense_item_shares t JOIN expense_items i ON i.id = t.expense_item_id "+
		"JOIN expense_versions v ON v.id = i.expense_version_id", "v.created_at, i.item_key, t.participant_id, t.id")
	surchargesDump  = orderedDump("expense_surcharges t JOIN expense_versions v ON v.id = t.expense_version_id", "v.created_at, t.surcharge_key, t.id")
	discountsDump   = orderedDump("expense_discounts t JOIN expense_versions v ON v.id = t.expense_version_id", "v.created_at, t.discount_key, t.id")
	allocationsDump = orderedDump("confirmed_allocations t JOIN expense_versions v ON v.id = t.expense_version_id",
		fixtureFirst("t.id")+", v.created_at, v.version_number, t.participant_id, t.id")
	auditDump          = orderedDump("audit_events t", fixtureFirst("t.id")+", t.occurred_at, t.event_type, t.id")
	billsDump          = orderedDump("bills t", fixtureFirst("t.id")+", t.created_at, t.id")
	billItemsDump      = orderedDump("bill_items t JOIN bills b ON b.id = t.bill_id", fixtureFirst("t.id")+", b.created_at, t.position, t.item_key, t.id")
	billSharesDump     = orderedDump("bill_item_shares t JOIN bill_items i ON i.id = t.bill_item_id JOIN bills b ON b.id = i.bill_id", fixtureFirst("t.id")+", b.created_at, i.position, i.item_key, t.participant_id, t.id")
	billSurchargesDump = orderedDump("bill_surcharges t JOIN bills b ON b.id = t.bill_id", fixtureFirst("t.id")+", b.created_at, t.surcharge_key, t.id")
	billDiscountsDump  = orderedDump("bill_discounts t JOIN bills b ON b.id = t.bill_id", fixtureFirst("t.id")+", b.created_at, t.discount_key, t.id")
	batchesDump        = orderedDump("collection_batches t", fixtureFirst("t.id")+", t.created_at, t.id")
	batchVersionsDump  = orderedDump("collection_batch_versions t JOIN collection_batches b ON b.id = t.batch_id", fixtureFirst("t.id")+", b.created_at, t.version_number, t.id")
	obligationsDump    = orderedDump("collection_obligations t JOIN collection_batch_versions bv ON bv.id = t.batch_version_id "+
		"JOIN collection_batches b ON b.id = bv.batch_id", fixtureFirst("t.id")+", b.created_at, bv.version_number, t.sender_id, t.recipient_id, t.id")
	sourcesDump = orderedDump("collection_obligation_sources t JOIN collection_obligations o ON o.id = t.obligation_id "+
		"JOIN collection_batch_versions bv ON bv.id = o.batch_version_id JOIN collection_batches b ON b.id = bv.batch_id",
		fixtureFirst("o.id")+", b.created_at, o.sender_id, o.recipient_id, t.confirmed_allocation_id, t.obligation_id")
	envelopesDump = orderedDump("collection_envelopes t JOIN collection_batch_versions bv ON bv.id = t.batch_version_id "+
		"JOIN collection_batches b ON b.id = bv.batch_id", fixtureFirst("t.id")+", b.created_at, bv.version_number, t.sender_id, t.id")
	linksDump    = orderedDump("guest_links t JOIN collection_envelopes e ON e.id = t.envelope_id", fixtureFirst("t.id")+", e.batch_version_id, e.sender_id, t.token_digest, t.id")
	receiptsDump = orderedDump("receipt_confirmations t", fixtureFirst("t.id")+", t.confirmed_at, t.idempotency_key, t.id")
)

// ---------------------------------------------------------------------------
// Arguments and tags
// ---------------------------------------------------------------------------

func argNumber(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			panic(err)
		}
		return n
	case float64:
		return int64(x)
	}
	panic(fmt.Sprintf("not a number: %T", v))
}

func argOptionalNumber(v any) *int64 {
	if v == nil {
		return nil
	}
	n := argNumber(v)
	return &n
}

func argText(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func argMaps(v any) []map[string]any {
	out := []map[string]any{}
	for _, item := range v.([]any) {
		out = append(out, item.(map[string]any))
	}
	return out
}

func argProposal(v any) ExpenseProposal {
	m := v.(map[string]any)
	p := ExpenseProposal{Description: argText(m["description"]), RecordedByID: m["recorded_by_id"].(string),
		PaidByID: m["paid_by_id"].(string), VerificationScope: m["verification_scope"].(string),
		OccurredAt: argInstant(m["occurred_at"].(string))}
	for _, item := range argMaps(m["items"]) {
		p.Items = append(p.Items, ExpenseItemInput{ItemID: item["item_id"].(string), Label: argText(item["label"]),
			AmountVND: argNumber(item["amount_vnd"]), SharedBy: argStrings(item["shared_by"])})
	}
	for _, s := range argMaps(m["surcharges"]) {
		p.Surcharges = append(p.Surcharges, ExpenseSurchargeInput{SurchargeID: s["surcharge_id"].(string),
			Kind: s["kind"].(string), AmountVND: argNumber(s["amount_vnd"]), Mode: s["mode"].(string)})
	}
	for _, d := range argMaps(m["discounts"]) {
		p.Discounts = append(p.Discounts, ExpenseDiscountInput{DiscountID: d["discount_id"].(string),
			AmountVND: argNumber(d["amount_vnd"]), Scope: d["scope"].(string), ItemID: argText(d["item_id"])})
	}
	return p
}

func argConfirmation(a map[string]any, expenseID string) ExpenseConfirmation {
	c := ExpenseConfirmation{ExpenseID: expenseID, Proposal: argProposal(a["proposal"]),
		AllocatorWarnings: argStrings(a["warnings"]), ConfirmedByID: argString(a, "confirmed_by_id"),
		PayerAcknowledgement: argString(a, "payer_acknowledgement"), Now: argInstant(argString(a, "now"))}
	for _, item := range a["rollups"].([]any) {
		pair := item.([]any)
		n := argNumber(pair[1])
		switch pair[0].(string) {
		case "subtotal_amount_vnd":
			c.Rollups.SubtotalVND = n
		case "fee_amount_vnd":
			c.Rollups.FeeVND = n
		case "vat_amount_vnd":
			c.Rollups.VATVND = n
		case "shipping_amount_vnd":
			c.Rollups.ShippingVND = n
		case "discount_amount_vnd":
			c.Rollups.DiscountVND = n
		case "total_amount_vnd":
			c.Rollups.TotalVND = n
		default:
			panic(pair[0])
		}
	}
	for _, item := range a["allocations"].([]any) {
		pair := item.([]any)
		c.Allocations = append(c.Allocations, ParticipantAmount{ParticipantID: pair[0].(string), AmountVND: argNumber(pair[1])})
	}
	return c
}

func argBill(a map[string]any) BillInput {
	in := BillInput{ContextID: argString(a, "context_id"), CreatedByID: argString(a, "created_by_id"),
		PrintedTotalVND: argOptionalNumber(a["printed_total_vnd"]), ItemsTotalVND: argNumber(a["items_total_vnd"]),
		Confidence: argNumber(a["confidence"]), NeedsReview: a["needs_review"].(bool), Now: argInstant(argString(a, "now"))}
	for _, item := range argMaps(a["items"]) {
		in.Items = append(in.Items, BillItemInput{ItemKey: item["item_key"].(string), Name: item["name"].(string),
			Quantity: argNumber(item["quantity"]), UnitPriceVND: argOptionalNumber(item["unit_price_vnd"]),
			LineTotalVND: argNumber(item["line_total_vnd"]), Position: argNumber(item["position"]),
			SuggestedParticipantIDs: argStrings(item["suggested_participant_ids"])})
	}
	for _, s := range argMaps(a["surcharges"]) {
		in.Surcharges = append(in.Surcharges, BillSurcharge{SurchargeKey: s["surcharge_key"].(string),
			Kind: s["kind"].(string), AmountVND: argNumber(s["amount_vnd"]), Mode: s["mode"].(string)})
	}
	for _, d := range argMaps(a["discounts"]) {
		in.Discounts = append(in.Discounts, BillDiscount{DiscountKey: d["discount_key"].(string),
			AmountVND: argNumber(d["amount_vnd"]), Scope: d["scope"].(string), TargetItemKey: argText(d["target_item_key"])})
	}
	return in
}

func argLinks(a map[string]any) []GuestLinkDraft {
	out := []GuestLinkDraft{}
	for _, l := range argMaps(a["links"]) {
		digest, err := hex.DecodeString(l["token_digest"].(string))
		if err != nil {
			panic(err)
		}
		out = append(out, GuestLinkDraft{SenderID: l["sender_id"].(string), TokenDigest: digest,
			ExpiresAt: argInstant(l["expires_at"].(string))})
	}
	return out
}

func argReceipt(a map[string]any, target ReceiptTarget) ReceiptConfirmationInput {
	return ReceiptConfirmationInput{Target: target, ConfirmedByID: argString(a, "confirmed_by_id"),
		AmountVND: argNumber(a["amount_vnd"]), PaymentReportID: argText(a["payment_report_id"]),
		IdempotencyKey: argString(a, "idempotency_key"), Now: argInstant(argString(a, "now"))}
}

func tBig(n *big.Int) any { return tv("int", n.String()) }

func tUUIDList(ids []string) any {
	items := []any{}
	for _, id := range ids {
		items = append(items, tUUID(id))
	}
	return tSeq(items)
}

func tExpenseIdentity(e ExpenseIdentity) any {
	return tRecord("ExpenseIdentity", "id", tUUID(e.ID), "context_id", tUUID(e.ContextID))
}

func tConfirmation(c ConfirmationRecord) any {
	return tRecord("ConfirmationRecord", "expense_version_id", tUUID(c.ExpenseVersionID), "version_number", tInt(c.VersionNumber))
}

func tBill(b Bill) any {
	items := []any{}
	for _, item := range b.Items {
		shares := []any{}
		for _, s := range item.Shares {
			shares = append(shares, tRecord("BillShareRecord", "participant_id", tUUID(s.ParticipantID),
				"source", tStr(s.Source), "decided_by_id", optional(s.DecidedByID, tUUID),
				"decided_at", optional(s.DecidedAt, tInstant)))
		}
		items = append(items, tRecord("BillItemRecord", "item_key", tStr(item.ItemKey), "name", tStr(item.Name),
			"quantity", tInt(item.Quantity), "unit_price_vnd", optional(item.UnitPriceVND, tInt),
			"line_total_vnd", tInt(item.LineTotalVND), "position", tInt(item.Position), "shares", tSeq(shares)))
	}
	surcharges := []any{}
	for _, s := range b.Surcharges {
		surcharges = append(surcharges, tRecord("BillSurchargeRecord", "surcharge_key", tStr(s.SurchargeKey),
			"kind", tStr(s.Kind), "amount_vnd", tInt(s.AmountVND), "mode", tStr(s.Mode)))
	}
	discounts := []any{}
	for _, d := range b.Discounts {
		discounts = append(discounts, tRecord("BillDiscountRecord", "discount_key", tStr(d.DiscountKey),
			"amount_vnd", tInt(d.AmountVND), "scope", tStr(d.Scope), "target_item_key", optional(d.TargetItemKey, tStr)))
	}
	return tRecord("BillRecord", "id", tUUID(b.ID), "context_id", tUUID(b.ContextID),
		"printed_total_vnd", optional(b.PrintedTotalVND, tInt), "items_total_vnd", tInt(b.ItemsTotalVND),
		"confidence", tInt(b.Confidence), "needs_review", tBool(b.NeedsReview), "created_by_id", tUUID(b.CreatedByID),
		"created_at", tInstant(b.CreatedAt), "items", tSeq(items), "surcharges", tSeq(surcharges),
		"discounts", tSeq(discounts))
}

func tFrozenBatch(b FrozenBatch) any {
	obligations := []any{}
	for _, o := range b.Obligations {
		obligations = append(obligations, tRecord("FrozenObligation", "id", tUUID(o.ID), "sender_id", tUUID(o.SenderID),
			"recipient_id", tUUID(o.RecipientID), "amount_vnd", tInt(o.AmountVND), "due_at", tInstant(o.DueAt),
			"source_expense_version_ids", tUUIDList(o.SourceExpenseVersionIDs)))
	}
	return tRecord("FrozenBatch", "id", tUUID(b.ID), "version_id", tUUID(b.VersionID), "obligations", tSeq(obligations))
}

func tBatchForPublish(b BatchForPublish) any {
	obligations := []any{}
	for _, o := range b.Obligations {
		obligations = append(obligations, tRecord("PublishObligation", "id", tUUID(o.ID),
			"batch_version_id", tUUID(o.BatchVersionID), "sender_id", tUUID(o.SenderID),
			"recipient_id", tUUID(o.RecipientID), "amount_vnd", tInt(o.AmountVND)))
	}
	return tRecord("BatchForPublish", "id", tUUID(b.ID), "version_id", tUUID(b.VersionID), "owner_id", tUUID(b.OwnerID),
		"status", tStr(b.Status), "context_id", tUUID(b.ContextID), "advancer_acknowledged", tBool(b.AdvancerAcknowledged),
		"obligations", tSeq(obligations))
}

func tStoredLinks(links []StoredGuestLink) any {
	items := []any{}
	for _, l := range links {
		items = append(items, tRecord("StoredGuestLink", "id", tUUID(l.ID), "envelope_id", tUUID(l.EnvelopeID),
			"sender_id", tUUID(l.SenderID)))
	}
	return tSeq(items)
}

func tBoard(b BatchBoard) any {
	rows := []any{}
	for _, r := range b.Obligations {
		rows = append(rows, tRecord("BatchObligationRow", "obligation_id", tUUID(r.ObligationID),
			"sender_id", tUUID(r.SenderID), "recipient_id", tUUID(r.RecipientID), "amount_vnd", tInt(r.AmountVND),
			"status", tStr(r.Status), "disputed", tBool(r.Disputed), "disputed_reason", optional(r.DisputedReason, tStr),
			"payment_reported_at", optional(r.PaymentReportedAt, tInstant)))
	}
	return tRecord("BatchBoard", "context_id", tUUID(b.ContextID), "obligations", tSeq(rows))
}

func tContextBatches(rows []ContextBatchRow) any {
	items := []any{}
	for _, r := range rows {
		items = append(items, tRecord("ContextBatchRow", "batch_id", tUUID(r.BatchID), "status", tStr(r.Status),
			"created_at", tInstant(r.CreatedAt), "published_at", optional(r.PublishedAt, tInstant),
			"obligation_count", tInt(r.ObligationCount), "confirmed_count", tInt(r.ConfirmedCount),
			"disputed_count", tInt(r.DisputedCount), "total_vnd", tBig(r.TotalVND)))
	}
	return tSeq(items)
}

func tReceiptTarget(r ReceiptTarget) any {
	return tRecord("ReceiptTarget", "obligation_id", tUUID(r.ObligationID), "recipient_id", tUUID(r.RecipientID),
		"amount_vnd", tInt(r.AmountVND))
}

func tReceiptRecord(r ReceiptRecord) any {
	amounts := []any{}
	for _, a := range r.ReceiptAmountsVND {
		amounts = append(amounts, tInt(a))
	}
	return tRecord("ReceiptRecord", "id", tUUID(r.ID), "obligation_id", tUUID(r.ObligationID),
		"amount_vnd", tInt(r.AmountVND), "receipt_amounts_vnd", tSeq(amounts))
}

func tFinance(f PersonFinanceSummary) any {
	movements := []any{}
	for _, m := range f.Movements {
		movements = append(movements, tRecord("FinanceMovement", "obligation_id", tUUID(m.ObligationID),
			"direction", tStr(m.Direction), "amount_vnd", tInt(m.AmountVND), "counterparty_id", tUUID(m.CounterpartyID),
			"counterparty_name", optional(m.CounterpartyName, tStr), "context_id", tUUID(m.ContextID),
			"context_name", optional(m.ContextName, tStr), "occasion", optional(m.Occasion, tStr),
			"occurred_at", tInstant(m.OccurredAt)))
	}
	return tRecord("PersonFinanceSummary", "person_id", tUUID(f.PersonID), "display_name", optional(f.DisplayName, tStr),
		"spend_vnd", tBig(f.SpendVND), "settled_vnd", tBig(f.SettledVND), "outstanding_vnd", tBig(f.OutstandingVND),
		"receivable_vnd", tBig(f.ReceivableVND), "expense_count", tInt(f.ExpenseCount), "group_count", tInt(f.GroupCount),
		"movements", tSeq(movements))
}

func moneyGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	switch method {
	case "create_expense":
		e, err := repo.CreateExpense(bg, s("context_id"))
		return tExpenseIdentity(e), err
	case "get_expense":
		e, err := repo.GetExpense(bg, s("expense_id"))
		return nilOr(e, tExpenseIdentity), err
	case "save_expense_confirmation":
		c, err := repo.SaveExpenseConfirmation(bg, argConfirmation(a, s("expense_id")))
		return tConfirmation(c), err
	case "flow.create_expense_confirm":
		identity, err := repo.CreateExpense(bg, s("context_id"))
		if err != nil {
			return nil, err
		}
		loaded, err := repo.GetExpense(bg, identity.ID)
		if err != nil {
			return nil, err
		}
		c, err := repo.SaveExpenseConfirmation(bg, argConfirmation(a, identity.ID))
		return tSeq([]any{tExpenseIdentity(identity), nilOr(loaded, tExpenseIdentity), tConfirmation(c)}), err
	case "create_bill":
		b, err := repo.CreateBill(bg, argBill(a))
		return tBill(b), err
	case "get_bill":
		b, err := repo.GetBill(bg, s("bill_id"))
		return nilOr(b, tBill), err
	case "confirm_bill_assignments":
		var assignments []BillAssignment
		for _, item := range argMaps(a["assignments"]) {
			assignments = append(assignments, BillAssignment{ItemKey: item["item_key"].(string),
				ParticipantIDs: argStrings(item["participant_ids"])})
		}
		b, err := repo.ConfirmBillAssignments(bg, s("bill_id"), assignments, s("decided_by_id"), argInstant(s("now")))
		return tBill(b), err
	case "claim_bill_items":
		b, err := repo.ClaimBillItems(bg, s("bill_id"), s("participant_id"), argStrings(a["item_keys"]), argInstant(s("now")))
		return tBill(b), err
	case "save_frozen_batch":
		in := FrozenBatchInput{ContextID: s("context_id"), OwnerID: s("owner_id"), DueAt: argInstant(s("due_at")),
			Now: argInstant(s("now"))}
		for _, d := range argMaps(a["obligations"]) {
			draft := ObligationDraft{SenderID: d["sender_id"].(string), RecipientID: d["recipient_id"].(string),
				AmountVND: argNumber(d["amount_vnd"]), SourceExpenseVersionIDs: argStrings(d["source_expense_version_ids"])}
			for _, source := range argMaps(d["sources"]) {
				draft.Sources = append(draft.Sources, AllocationRow{ID: source["id"].(string),
					ParticipantID: source["participant_id"].(string), AmountVND: argNumber(source["amount_vnd"])})
			}
			in.Obligations = append(in.Obligations, draft)
		}
		b, err := repo.SaveFrozenBatch(bg, in)
		return tFrozenBatch(b), err
	case "load_batch_for_publish":
		b, err := repo.LoadBatchForPublish(bg, s("batch_id"))
		return nilOr(b, tBatchForPublish), err
	case "save_published_batch":
		links, err := repo.SavePublishedBatch(bg, BatchForPublish{ID: s("batch_id"), VersionID: s("version_id")},
			s("status"), argLinks(a), s("actor_id"), argInstant(s("now")))
		return tStoredLinks(links), err
	case "flow.publish":
		b, err := repo.LoadBatchForPublish(bg, s("batch_id"))
		if err != nil {
			return nil, err
		}
		if b == nil {
			return tSeq([]any{nil, nil}), nil
		}
		links, err := repo.SavePublishedBatch(bg, *b, s("status"), argLinks(a), s("actor_id"), argInstant(s("now")))
		return tSeq([]any{tBatchForPublish(*b), tStoredLinks(links)}), err
	case "list_batch_obligations":
		b, err := repo.ListBatchObligations(bg, s("batch_id"))
		return nilOr(b, tBoard), err
	case "list_context_batches":
		rows, err := repo.ListContextBatches(bg, s("context_id"))
		return tContextBatches(rows), err
	case "get_receipt_target":
		r, err := repo.GetReceiptTarget(bg, s("obligation_id"))
		return nilOr(r, tReceiptTarget), err
	case "save_receipt_confirmation":
		r, err := repo.SaveReceiptConfirmation(bg, argReceipt(a, ReceiptTarget{ObligationID: s("obligation_id"),
			RecipientID: s("recipient_id"), AmountVND: argNumber(a["target_amount_vnd"])}))
		return tReceiptRecord(r), err
	case "flow.confirm_receipt":
		target, err := repo.GetReceiptTarget(bg, s("obligation_id"))
		if err != nil {
			return nil, err
		}
		if target == nil {
			return tSeq([]any{nil, nil}), nil
		}
		r, err := repo.SaveReceiptConfirmation(bg, argReceipt(a, *target))
		return tSeq([]any{tReceiptTarget(*target), tReceiptRecord(r)}), err
	case "person_finance_summary":
		f, err := repo.PersonFinanceSummary(bg, s("person_id"), argNumber(a["movement_limit"]))
		return tFinance(f), err
	}
	return groupsGoCall(repo, method, a)
}

// moneyGoError is groupsGoError plus the Python exception classes the W4
// methods raise themselves.
func moneyGoError(err error) map[string]any {
	out := groupsGoError(err)
	var ledger *LedgerRefusal
	switch {
	case errors.Is(err, ErrUnknownPayerAcknowledgement), errors.Is(err, ErrUnknownVerificationScope),
		errors.Is(err, ErrUnknownBatchStatus):
		out["type"] = "ValueError"
	case errors.Is(err, ErrNotAnEnumValue):
		out["type"] = "StatementError"
	case errors.Is(err, ErrEventDataNotAnObject):
		out["type"] = "AttributeError"
	case errors.As(err, &ledger):
		out["type"] = "LedgerError"
	}
	return out
}

func runMoneyGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	for _, sql := range c.Setup {
		if _, err := tx.Exec(bg, sql); err != nil {
			t.Fatalf("setup: %v\n%s", err, sql)
		}
	}
	steps := []any{}
	for _, step := range c.Steps {
		for _, sql := range step.Before {
			if _, err := tx.Exec(bg, sql); err != nil {
				t.Fatalf("before: %v\n%s", err, sql)
			}
		}
		rec := &recorder{Querier: tx}
		value, err := moneyGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = moneyGoError(err)
			steps = append(steps, generic(t, out))
			break
		}
		out["result"] = value
		probes := []any{}
		for _, sql := range step.Probes {
			rows, err := tx.Query(bg, sql)
			if err != nil {
				t.Fatalf("probe: %v\n%s", err, sql)
			}
			texts, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				t.Fatal(err)
			}
			probes = append(probes, append([]string{}, texts...))
		}
		out["probes"] = probes
		steps = append(steps, generic(t, out))
	}
	return steps
}

// normalizeMoneySteps prepares one side's steps for compareCase: the instant
// that side's probeNow read becomes <transaction-now>, and probeMoneyRowLocks
// rows name every id the case did not write as <generated>, sorted.
func normalizeMoneySteps(steps []any, c oracleCase) []any {
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
	}
	out := make([]any, len(steps))
	for i, step := range steps {
		out[i] = step
		probes, _ := step.(map[string]any)["probes"].([]any)
		for j, probe := range c.Steps[i].Probes {
			if j >= len(probes) {
				continue
			}
			rows, ok := probes[j].([]any)
			if !ok {
				continue
			}
			switch probe {
			case probeMoneyRowLocks:
				masked := make([]string, len(rows))
				for k, row := range rows {
					masked[k] = uuidText.ReplaceAllStringFunc(row.(string), func(id string) string {
						if known[id] {
							return id
						}
						return "<generated>"
					})
				}
				sort.Strings(masked)
				sorted := make([]any, len(masked))
				for k, row := range masked {
					sorted[k] = row
				}
				probes[j] = sorted
			case probeNow:
				if len(rows) == 1 {
					if now, ok := rows[0].(string); ok && now != "" {
						out[i] = replaceText(out[i], now, transactionNow)
					}
				}
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func moneyOracleCases() ([]socialCase, oracleSpec) {
	w := newMoneyWorld()
	var cases []socialCase
	add := func(name, wantEnd string, setup []string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps}, wantEnd})
	}
	probes := func(dumps ...string) []string {
		return append(append([]string{probeLocks, probeWrites, probeMoneyRowLocks}, dumps...), probeNow)
	}
	read := func(method string, a map[string]any) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...), Probes: probes()}
	}
	write := func(method string, a map[string]any, dumps ...string) oracleCall {
		return oracleCall{Call: method, Args: a, Before: append([]string{}, writesBaseline...), Probes: probes(dumps...)}
	}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	list := func(values ...any) []any { return append([]any{}, values...) }
	later := func(step int) string { return fmt.Sprintf("2030-07-1%dT12:00:00.654321Z", step) }
	base := w.sql
	vietnam := join([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base)
	const max = largestBigint
	isMember := func(context, person string) oracleCall {
		return read("is_member", args("context_id", context, "person_id", person))
	}
	listMembers := func(context string) oracleCall { return read("list_members", args("context_id", context)) }

	// --- expenses --------------------------------------------------------------
	createExpense := func(context string) oracleCall {
		return write("create_expense", args("context_id", context), expensesDump)
	}
	add("create_expense: an expense of a group", "", base, createExpense(w.g))
	add("create_expense: under a Vietnam session TimeZone", "", vietnam, createExpense(w.g2))
	add("create_expense: two groups in one transaction", "", base, createExpense(w.empty), createExpense(w.g))
	add("create_expense: a group with no row", "EXPENSE_CONTEXT_NOT_FOUND", base, createExpense(w.missingContext))
	add("create_expense: a group with no row under a Vietnam session TimeZone", "EXPENSE_CONTEXT_NOT_FOUND", vietnam,
		createExpense(w.missingContext))
	add("POST /expenses naming a group nobody created", "EXPENSE_CONTEXT_NOT_FOUND", base, createExpense(fid(kindContext, 0x5e)))
	add("POST /expenses", "", base, createExpense(w.g2))

	expense := func(id string) oracleCall { return read("get_expense", args("expense_id", id)) }
	add("get_expense: with versions, without, another group's, missing", "", base,
		expense(w.e1), expense(w.e4), expense(w.e3), expense(w.missingExpense))
	add("get_expense: under a Vietnam session TimeZone", "", vietnam, expense(w.e2))

	expenseDumps := []string{versionsDump, itemsDump, itemSharesDump, surchargesDump, discountsDump, allocationsDump, auditDump}
	proposal := func(context string, items, surcharges, discounts []any) map[string]any {
		return map[string]any{"context_id": context, "description": "Bữa tối ' \" 🙂 (dữ liệu mẫu)",
			"recorded_by_id": w.binh, "paid_by_id": w.an, "verification_scope": "items_reviewed",
			"occurred_at": "2030-07-09T19:30:00.25+07:00", "participants": list(w.an, w.binh, w.chi),
			"total_amount_vnd": 0, "items": items, "surcharges": surcharges, "discounts": discounts}
	}
	expenseItem := func(id string, label any, amount int64, sharedBy ...any) map[string]any {
		return map[string]any{"item_id": id, "label": label, "amount_vnd": amount, "shared_by": list(sharedBy...)}
	}
	surcharge := func(id, kind string, amount int64, mode string) map[string]any {
		return map[string]any{"surcharge_id": id, "kind": kind, "amount_vnd": amount, "mode": mode}
	}
	discount := func(id string, amount int64, scope string, item any) map[string]any {
		return map[string]any{"discount_id": id, "amount_vnd": amount, "scope": scope, "item_id": item}
	}
	rollups := func(subtotal, fee, vat, shipping, discount, total int64) []any {
		return list(list("subtotal_amount_vnd", subtotal), list("fee_amount_vnd", fee), list("vat_amount_vnd", vat),
			list("shipping_amount_vnd", shipping), list("discount_amount_vnd", discount), list("total_amount_vnd", total))
	}
	allocations := func(pairs ...any) []any {
		out := []any{}
		for i := 0; i < len(pairs); i += 2 {
			out = append(out, list(pairs[i], pairs[i+1]))
		}
		return out
	}
	confirmArgs := func(p map[string]any, r []any, allocs []any, ack, now string) map[string]any {
		return args("proposal", p, "warnings", list("LAM_TRON_DONG", "Cảnh báo (dữ liệu mẫu)"), "rollups", r,
			"allocations", allocs, "confirmed_by_id", w.an, "payer_acknowledgement", ack, "now", now)
	}
	confirm := func(expenseID string, p map[string]any, r []any, allocs []any, ack, now string) oracleCall {
		a := confirmArgs(p, r, allocs, ack, now)
		a["expense_id"] = expenseID
		return write("save_expense_confirmation", a, expenseDumps...)
	}
	noLines := proposal(w.g, list(), list(), list())
	fullLines := proposal(w.g,
		list(expenseItem("i-lau", "Lẩu (dữ liệu mẫu)", 200_000, w.chi, w.an, w.binh), expenseItem("i-bia", nil, 60_000, w.binh)),
		list(surcharge("s-vat", "VAT", 20_000, "proportional"), surcharge("s-ship", "Shipping", 10_000, "even"),
			surcharge("s-phi", "phi", 5_000, "proportional")),
		list(discount("d-lau", 15_000, "item", "i-lau"), discount("d-all", 5_000, "global_proportional", nil)))
	fullRollups := rollups(260_000, 5_000, 20_000, 10_000, 20_000, 275_000)
	fullAllocations := allocations(w.chi, 75_000, w.binh, 120_000, w.an, 80_000)
	add("save_expense_confirmation: a third version with every kind of line", "", base,
		confirm(w.e1, fullLines, fullRollups, fullAllocations, "pending", moneyNow))
	add("save_expense_confirmation: the first version of an expense with none, no lines", "", base,
		confirm(w.e4, noLines, rollups(50_000, 0, 0, 0, 0, 50_000), allocations(w.dung, 25_000, w.an, 25_000), "acknowledged", moneyNow))
	add("save_expense_confirmation: two versions in a row, under a Vietnam session TimeZone", "", vietnam,
		confirm(w.e2, proposal(w.g, list(expenseItem("i-1", "Một (dữ liệu mẫu)", 30_000, w.an)), list(), list()),
			rollups(30_000, 0, 0, 0, 0, 30_000), allocations(w.an, 30_000), "disputed", moneyNow),
		confirm(w.e2, fullLines, fullRollups, fullAllocations, "pending", later(1)))
	add("save_expense_confirmation: an item-scoped discount naming no item", "IntegrityError", base,
		confirm(w.e1, proposal(w.g, list(expenseItem("i-lau", nil, 100_000, w.an)), list(),
			list(discount("d-x", 1_000, "item", "i-khong-co"))), rollups(100_000, 0, 0, 0, 1_000, 99_000),
			allocations(w.an, 99_000), "pending", moneyNow))
	add("save_expense_confirmation: a repeated item id", "IntegrityError", base,
		confirm(w.e1, proposal(w.g, list(expenseItem("i-1", nil, 10_000, w.an), expenseItem("i-1", nil, 20_000, w.binh)),
			list(), list()), rollups(30_000, 0, 0, 0, 0, 30_000), allocations(w.an, 30_000), "pending", moneyNow))
	add("save_expense_confirmation: one diner sharing an item twice", "IntegrityError", base,
		confirm(w.e1, proposal(w.g, list(expenseItem("i-1", nil, 10_000, w.an, w.an)), list(surcharge("s", "vat", 1, "even")),
			list()), rollups(10_000, 0, 1, 0, 0, 10_001), allocations(w.an, 10_001), "pending", moneyNow))
	add("save_expense_confirmation: a missing expense", "EXPENSE_NOT_FOUND", base,
		confirm(w.missingExpense, noLines, rollups(1, 0, 0, 0, 0, 1), allocations(w.an, 1), "pending", moneyNow))
	add("save_expense_confirmation: an acknowledgement that does not exist", "ValueError", base,
		confirm(w.e1, noLines, rollups(1, 0, 0, 0, 0, 1), allocations(w.an, 1), "maybe", moneyNow))
	add("save_expense_confirmation: rollups that do not add up", "IntegrityError", base,
		confirm(w.e4, noLines, rollups(10, 0, 0, 0, 0, 11), allocations(w.an, 11), "pending", moneyNow))
	add("save_expense_confirmation: a negative allocation", "IntegrityError", base,
		confirm(w.e4, noLines, rollups(10, 0, 0, 0, 0, 10), allocations(w.an, -1, w.binh, 11), "pending", moneyNow))
	add("save_expense_confirmation: allocations at the bigint maximum", "", base,
		confirm(w.e4, noLines, rollups(max, 0, 0, 0, 0, max), allocations(w.an, max, w.binh, 0), "pending", moneyNow))
	var many []any
	var manyDiners []any
	for i := 0; i < 1001; i++ {
		many = append(many, fid(kindPerson, 0x10000+1000-i), 1)
		manyDiners = append(manyDiners, fid(kindPerson, 0x20000+i))
	}
	add("save_expense_confirmation: 1001 allocations", "", base,
		confirm(w.e4, noLines, rollups(1001, 0, 0, 0, 0, 1001), allocations(many...), "pending", moneyNow))
	add("POST /expenses/{id}/confirm", "", base, expense(w.e1), isMember(w.g, w.an), listMembers(w.g),
		confirm(w.e1, fullLines, fullRollups, fullAllocations, "acknowledged", moneyNow))
	flowArgs := func(context string, p map[string]any) map[string]any {
		a := confirmArgs(p, rollups(9_000, 0, 0, 0, 0, 9_000), allocations(w.binh, 4_000, w.chi, 5_000), "pending", moneyNow)
		a["context_id"] = context
		a["warnings"] = list()
		return a
	}
	add("POST /expenses then its confirmation, in one transaction", "", base,
		write("flow.create_expense_confirm", flowArgs(w.g2, proposal(w.g2, list(expenseItem("i-pho", "Phở", 9_000, w.binh, w.chi)),
			list(), list())), append([]string{expensesDump}, expenseDumps...)...))
	add("POST /expenses then its confirmation, for a group with no row", "EXPENSE_CONTEXT_NOT_FOUND", base,
		write("flow.create_expense_confirm", flowArgs(w.missingContext, proposal(w.missingContext, list(), list(), list())),
			append([]string{expensesDump}, expenseDumps...)...))

	// --- bills -----------------------------------------------------------------
	billDumps := []string{billsDump, billItemsDump, billSharesDump, billSurchargesDump, billDiscountsDump}
	billItem := func(key, name string, quantity int64, unit any, total int64, position int, suggested ...any) map[string]any {
		return map[string]any{"item_key": key, "name": name, "quantity": quantity, "unit_price_vnd": unit,
			"line_total_vnd": total, "position": position, "suggested_participant_ids": list(suggested...)}
	}
	billSurcharge := func(key, kind string, amount int64, mode string) map[string]any {
		return map[string]any{"surcharge_key": key, "kind": kind, "amount_vnd": amount, "mode": mode}
	}
	billDiscount := func(key string, amount int64, scope string, target any) map[string]any {
		return map[string]any{"discount_key": key, "amount_vnd": amount, "scope": scope, "target_item_key": target}
	}
	billArgs := func(context string, printed any, items, surcharges, discounts []any, now string) map[string]any {
		return args("context_id", context, "created_by_id", w.binh, "printed_total_vnd", printed,
			"items_total_vnd", int64(130_000), "confidence", 72, "needs_review", false, "items", items,
			"surcharges", surcharges, "discounts", discounts, "now", now)
	}
	createBill := func(context string, items, surcharges, discounts []any) oracleCall {
		return write("create_bill", billArgs(context, int64(150_000), items, surcharges, discounts, moneyNow), billDumps...)
	}
	fullBillItems := list(billItem("b-lau", "Lẩu 🍲 (dữ liệu mẫu)", 1, nil, 100_000, 0, w.chi, w.an),
		billItem("b-bia", "Bia (dữ liệu mẫu)", 6, int64(5_000), 30_000, 1, w.binh), billItem("b-rau", "Rau", 1, int64(0), 1, 1))
	fullBillSurcharges := list(billSurcharge("s-vat", "vat", 10_000, "proportional"), billSurcharge("s-ship", "shipping", 5_000, "even"))
	fullBillDiscounts := list(billDiscount("d-toan", 2_000, "global_proportional", nil), billDiscount("d-lau", 3_000, "item", "b-lau"))
	one := func(suggested ...any) []any {
		return list(billItem("k", "Món (dữ liệu mẫu)", 1, nil, 1, 0, suggested...))
	}
	add("create_bill: items, suggestions, surcharges and discounts", "", base,
		createBill(w.g, fullBillItems, fullBillSurcharges, fullBillDiscounts))
	add("create_bill: no lines and no printed total, under a Vietnam session TimeZone", "", vietnam,
		write("create_bill", billArgs(w.g2, nil, list(), list(), list(), "2030-07-10T19:00:00+07:00"), billDumps...))
	dupItems := list(billItem("k", "Một", 1, nil, 1, 0), billItem("k", "Hai", 1, nil, 2, 1))
	add("create_bill: a repeated item key", "DUPLICATE_BILL_ITEM_KEY", base, createBill(w.g, dupItems, list(), list()))
	add("create_bill: a repeated item key under a Vietnam session TimeZone", "DUPLICATE_BILL_ITEM_KEY", vietnam,
		createBill(w.g2, dupItems, list(), list()))
	add("POST /bills with a repeated item key", "DUPLICATE_BILL_ITEM_KEY", base, isMember(w.g, w.binh), listMembers(w.g),
		createBill(w.g, list(billItem("bia", "Bia Sài Gòn", 1, nil, 1, 0, w.an), billItem("bia", "Bia Sài Gòn", 2, nil, 2, 1)),
			list(), list()))
	add("create_bill: a repeated surcharge key", "DUPLICATE_BILL_SURCHARGE_KEY", base,
		createBill(w.g, one(w.an), list(billSurcharge("s", "vat", 1, "even"), billSurcharge("s", "phi", 2, "even")), list()))
	add("create_bill: a repeated discount key", "DUPLICATE_BILL_DISCOUNT_KEY", base,
		createBill(w.g, one(), list(), list(billDiscount("d", 1, "global_proportional", nil), billDiscount("d", 2, "item", "k"))))
	add("create_bill: a group with no row", "BILL_WRITE_CONFLICT", base,
		createBill(w.missingContext, fullBillItems, fullBillSurcharges, fullBillDiscounts))
	add("create_bill: an item-scoped discount with no target", "BILL_WRITE_CONFLICT", base,
		createBill(w.g, one(w.an), list(), list(billDiscount("d", 1, "item", nil))))
	add("create_bill: one diner suggested twice for an item", "BILL_WRITE_CONFLICT", base,
		createBill(w.g, one(w.an, w.an), list(), list(billDiscount("d", 1, "global_proportional", nil))))
	add("create_bill: a line total of zero", "BILL_WRITE_CONFLICT", base,
		createBill(w.g, list(billItem("k", "Món", 1, nil, 0, 0)), list(), list()))
	add("create_bill: an item key past varchar(64)", "DataError", base,
		createBill(w.g, list(billItem(strings.Repeat("k", 65), "Món", 1, nil, 1, 0)), list(), list()))
	add("create_bill: a quantity past INTEGER", "DataError", base,
		createBill(w.g, list(billItem("k", "Món", 3_000_000_000, nil, 1, 0)), list(), list()))
	add("create_bill: a surcharge mode that does not exist", "StatementError", base,
		createBill(w.g, one(w.an), list(billSurcharge("s", "vat", 1, "double")), list(billDiscount("d", 1, "global_proportional", nil))))
	add("create_bill: a discount scope that does not exist", "StatementError", base,
		createBill(w.g, one(w.an), list(billSurcharge("s", "vat", 1, "even")), list(billDiscount("d", 1, "per_person", nil))))
	var manyItems []any
	for i := 0; i < 1001; i++ {
		manyItems = append(manyItems, billItem(fmt.Sprintf("k%04d", 1000-i), "Món (dữ liệu mẫu)", 1, nil, 1, i%3))
	}
	add("create_bill: 1001 items", "", base, createBill(w.g, manyItems, list(), list()))

	getBill := func(id string) oracleCall { return read("get_bill", args("bill_id", id)) }
	add("get_bill: ties on position, shares, surcharges and discounts in key order, empty, missing", "", base,
		getBill(w.b1), getBill(w.b2), getBill(w.b3), getBill(w.missingBill))
	add("get_bill: under a Vietnam session TimeZone", "", vietnam, getBill(w.b1))

	assignment := func(key string, participants ...any) map[string]any {
		return map[string]any{"item_key": key, "participant_ids": list(participants...)}
	}
	assign := func(billID string, assignments []any, now string) oracleCall {
		return write("confirm_bill_assignments", args("bill_id", billID, "assignments", assignments,
			"decided_by_id", w.chi, "now", now), billSharesDump)
	}
	add("confirm_bill_assignments: one item's table replaced, another cleared", "", base,
		assign(w.b1, list(assignment("k-lau", w.binh, w.an), assignment("k-bia")), moneyNow))
	add("confirm_bill_assignments: a repeated key keeps its first place and its last diners", "", base,
		assign(w.b1, list(assignment("k-com", w.dung), assignment("k-lau", w.chi), assignment("k-com", w.binh, w.an)), moneyNow))
	add("confirm_bill_assignments: no assignments still locks the bill and its items", "", base, assign(w.b1, list(), moneyNow))
	add("confirm_bill_assignments: twice, under a Vietnam session TimeZone", "", vietnam,
		assign(w.b1, list(assignment("k-bia", w.an)), moneyNow), assign(w.b1, list(assignment("k-bia", w.dung, w.an)), later(1)))
	add("confirm_bill_assignments: an item the bill does not have", "UNKNOWN_BILL_ITEM", base,
		assign(w.b1, list(assignment("k-lau", w.an), assignment("k-khong")), moneyNow))
	add("confirm_bill_assignments: a missing bill", "BILL_NOT_FOUND", base, assign(w.missingBill, list(assignment("k-lau", w.an)), moneyNow))
	add("confirm_bill_assignments: one diner twice on an item", "IntegrityError", base,
		assign(w.b1, list(assignment("k-lau", w.an, w.an)), moneyNow))
	add("confirm_bill_assignments: 1001 diners on an item", "", base, assign(w.b1, list(assignment("k-com", manyDiners...)), moneyNow))
	add("confirm_bill_assignments: another group's bill", "", base, assign(w.b3, list(assignment("k1", w.chi)), moneyNow))
	add("PUT /bills/{id}/assignments", "", base, getBill(w.b1), isMember(w.g, w.chi), listMembers(w.g),
		assign(w.b1, list(assignment("k-com", w.chi, w.dung)), moneyNow))

	claim := func(billID, participant string, keys []any, now string) oracleCall {
		return write("claim_bill_items", args("bill_id", billID, "participant_id", participant, "item_keys", keys, "now", now),
			billSharesDump)
	}
	add("claim_bill_items: claim two dishes, the other diners untouched", "", base, claim(w.b1, w.an, list("k-com", "k-lau"), moneyNow))
	add("claim_bill_items: a repeated key is one claim", "", base, claim(w.b1, w.binh, list("k-bia", "k-com", "k-bia"), moneyNow))
	add("claim_bill_items: claiming nothing releases every dish", "", base, claim(w.b1, w.dung, list(), moneyNow))
	add("claim_bill_items: twice, under a Vietnam session TimeZone", "", vietnam,
		claim(w.b1, w.chi, list("k-bia"), moneyNow), claim(w.b1, w.chi, list("k-com", "k-lau"), later(1)))
	add("claim_bill_items: a dish the bill does not have", "UNKNOWN_BILL_ITEM", base, claim(w.b1, w.an, list("k-lau", "k-khong"), moneyNow))
	add("claim_bill_items: a missing bill", "BILL_NOT_FOUND", base, claim(w.missingBill, w.an, list(), moneyNow))
	add("claim_bill_items: a bill with no items", "", base, claim(w.b2, w.an, list(), moneyNow))
	add("POST /bills/{id}/my-items", "", base, getBill(w.b1), isMember(w.g, w.dung), claim(w.b1, w.dung, list("k-lau"), moneyNow))
	add("POST /bills/{id}/split", "", base, getBill(w.b1), isMember(w.g, w.an), listMembers(w.g))
	add("GET /bills/{id}", "", base, getBill(w.b3), isMember(w.g2, w.binh))

	// --- budget ------------------------------------------------------------------
	add("GET /contexts/{id}/budget", "", base, isMember(w.g, w.an),
		read("group_recap", args("context_id", w.g, "today", "2030-05-02")), listMembers(w.g))
	add("GET /contexts/{id}/budget of a group with no trips", "", base, isMember(w.g2, w.chi),
		read("group_recap", args("context_id", w.g2, "today", "2030-05-02")), listMembers(w.g2))

	// --- collection rounds -------------------------------------------------------
	batchDumps := []string{batchesDump, batchVersionsDump, obligationsDump, sourcesDump, auditDump}
	draft := func(sender, recipient string, amount int64, versions []any, sources ...any) map[string]any {
		return map[string]any{"sender_id": sender, "recipient_id": recipient, "amount_vnd": amount,
			"source_expense_version_ids": versions, "sources": list(sources...)}
	}
	src := func(allocation, participant string, amount int64) map[string]any {
		return map[string]any{"id": allocation, "participant_id": participant, "amount_vnd": amount}
	}
	freeze := func(context, owner string, drafts []any, now string) oracleCall {
		return write("save_frozen_batch", args("context_id", context, "owner_id", owner, "due_at", "2030-08-15T17:00:00+07:00",
			"now", now, "obligations", drafts), batchDumps...)
	}
	allocOf := func(version, participant string) string { return w.alloc[allocKey(version, participant)] }
	add("save_frozen_batch: three obligations, sources on the first and the last", "", base, freeze(w.g, w.an, list(
		draft(w.binh, w.an, 45_000, list(w.e1v2, w.e5v1), src(allocOf(w.e1v2, w.binh), w.binh, 40_000), src(allocOf(w.e5v1, w.binh), w.binh, 5_000)),
		draft(w.dung, w.an, 3_000, list()),
		draft(w.chi, w.an, 40_000, list(w.e1v2, w.e1v1), src(allocOf(w.e1v2, w.chi), w.chi, 20_000), src(allocOf(w.e1v1, w.chi), w.chi, 20_000))),
		moneyNow))
	add("save_frozen_batch: no obligations", "", base, freeze(w.g, w.binh, list(), moneyNow))
	add("save_frozen_batch: an obligation at the bigint maximum, under a Vietnam session TimeZone", "", vietnam,
		freeze(w.g2, w.chi, list(draft(w.binh, w.chi, max, list(w.e3v1), src(allocOf(w.e3v1, w.binh), w.binh, max))), moneyNow))
	add("save_frozen_batch: a repeated pair", "IntegrityError", base,
		freeze(w.g, w.an, list(draft(w.binh, w.an, 1, list()), draft(w.binh, w.an, 2, list())), moneyNow))
	add("save_frozen_batch: a party owing itself", "IntegrityError", base, freeze(w.g, w.an, list(draft(w.an, w.an, 1, list())), moneyNow))
	add("save_frozen_batch: a source with no allocation row, written at the next obligation", "IntegrityError", base,
		freeze(w.g, w.an, list(draft(w.binh, w.an, 5, list(), src(fid(kindAllocation, 0xfffff), w.binh, 5)), draft(w.chi, w.an, 5, list())), moneyNow))
	add("save_frozen_batch: a group with no row", "IntegrityError", base, freeze(w.missingContext, w.an, list(), moneyNow))
	add("save_frozen_batch: a source amount of zero on the last obligation", "IntegrityError", base,
		freeze(w.g, w.an, list(draft(w.binh, w.an, 5, list()), draft(w.chi, w.an, 5, list(), src(allocOf(w.e1v2, w.chi), w.chi, 0))), moneyNow))
	breakfastSQL, _ := expenseSQL(0x75, w.g, w.chi, nil, moneyVersion{"acknowledged", "Bánh mì (dữ liệu mẫu)", "2030-05-04T07:00:00Z",
		[]share{{w.binh, 3_000}, {w.dung, 2_000}, {w.chi, 1_000}}})
	breakfast := fid(kindVersion, 0x75*16)
	add("POST /batches", "", join(base, breakfastSQL), isMember(w.g, w.chi),
		read("load_batch_inputs", args("context_id", w.g, "expense_version_ids", nil)),
		read("load_batch_inputs", args("context_id", w.g, "expense_version_ids", list(breakfast))),
		freeze(w.g, w.chi, list(
			draft(w.binh, w.chi, 3_000, list(breakfast), src(fid(kindAllocation, 0x75*16*16+15), w.binh, 3_000)),
			draft(w.dung, w.chi, 2_000, list(breakfast), src(fid(kindAllocation, 0x75*16*16+14), w.dung, 2_000))), moneyNow))

	forPublish := func(id string) oracleCall { return read("load_batch_for_publish", args("batch_id", id)) }
	add("load_batch_for_publish: acknowledged, pending, without sources, another group's, missing", "", base,
		forPublish(w.bt1), forPublish(w.bt2), forPublish(w.bt5), forPublish(w.bt4), forPublish(w.missingBatch))
	add("load_batch_for_publish: under a Vietnam session TimeZone", "", vietnam, forPublish(w.bt1))
	add("load_batch_for_publish: a batch with no version", "BATCH_HAS_NO_VERSION", base, forPublish(w.bt3))

	publishDumps := []string{batchesDump, envelopesDump, linksDump, auditDump}
	linkDraft := func(sender string, digest int, expires string) map[string]any {
		return map[string]any{"sender_id": sender, "token_digest": strings.Repeat(fmt.Sprintf("%02x", digest), 32), "expires_at": expires}
	}
	publish := func(batchID, versionID, status string, links []any, now string) oracleCall {
		return write("save_published_batch", args("batch_id", batchID, "version_id", versionID, "status", status,
			"actor_id", w.an, "now", now, "links", links), publishDumps...)
	}
	add("save_published_batch: two links, senders out of id order", "", base, publish(w.bt5, w.bt5v1, "published",
		list(linkDraft(w.dung, 0xa1, "2031-02-01T00:00:00+00:00"), linkDraft(w.an, 0xa2, "2031-02-01T00:00:00+07:00")), moneyNow))
	add("save_published_batch: no links, the status unchanged", "", base, publish(w.bt1, w.bt1v2, "frozen", list(), moneyNow))
	add("save_published_batch: no links and nothing to change", "", base,
		publish(w.bt2, w.bt2v1, "published", list(), "2030-01-03T07:00:00+07:00"))
	add("save_published_batch: a status the repository does not judge", "", vietnam,
		publish(w.bt3, fid(kindBatchVersion, 0x5e), "cancelled", list(), moneyNow))
	add("save_published_batch: a missing batch", "BATCH_NOT_FOUND", base, publish(w.missingBatch, w.bt1v2, "published", list(), moneyNow))
	add("save_published_batch: a status that does not exist", "ValueError", base, publish(w.bt1, w.bt1v2, "sent", list(), moneyNow))
	add("save_published_batch: an expiry not after creation", "IntegrityError", base,
		publish(w.bt5, w.bt5v1, "published", list(linkDraft(w.dung, 1, moneyNow)), moneyNow))
	add("save_published_batch: a sender who already has an envelope", "IntegrityError", base,
		publish(w.bt1, w.bt1v2, "published", list(linkDraft(w.an, 2, "2031-01-01T00:00:00Z"), linkDraft(w.chi, 3, "2031-01-01T00:00:00Z")), moneyNow))
	flowPublish := func(batchID string, links []any) oracleCall {
		return write("flow.publish", args("batch_id", batchID, "status", "published", "actor_id", w.binh, "now", moneyNow,
			"links", links), publishDumps...)
	}
	add("POST /batches/{id}/publish", "", base, flowPublish(w.bt5, list(linkDraft(w.dung, 0xb1, "2031-03-01T00:00:00Z"))))
	add("POST /batches/{id}/publish of a missing batch", "", base, flowPublish(w.missingBatch, list()))
	add("POST /batches/{id}/publish of a batch with no version", "BATCH_HAS_NO_VERSION", base, flowPublish(w.bt3, list()))

	board := func(id string) oracleCall { return read("list_batch_obligations", args("batch_id", id)) }
	add("list_batch_obligations: statuses, disputes and their first reason, claims", "", base,
		board(w.bt1), board(w.bt2), board(w.bt4), board(w.bt5), board(w.bt3), board(w.missingBatch))
	add("list_batch_obligations: under a Vietnam session TimeZone", "", vietnam, board(w.bt1))
	add("list_batch_obligations: event data that is not an object", "AttributeError",
		join(base, []string{insertSQL("audit_events", "id", fid(kindAuditEvent, 0x70), "event_type", "guest_objection.wrong_amount",
			"aggregate_type", "guest_link", "aggregate_id", w.linkBinh, "event_data", `["khong", "phai"]`,
			"occurred_at", "2030-06-12T00:00:00Z")}), board(w.bt1))
	bigRound := extraBatchSQL(7, w.g, w.binh, "2030-02-02T00:00:00Z",
		extraObligation{w.an, w.binh, max, []int64{max, max}},
		extraObligation{w.chi, w.binh, max, []int64{max - 5, 5}},
		extraObligation{w.dung, w.binh, max, []int64{max, 1}},
		extraObligation{w.erased, w.binh, max, []int64{max - 5, 4}})
	bigBatch := fid(kindBatch, 0x67)
	add("list_batch_obligations: receipts past int64 on obligations at the bigint maximum", "", join(base, bigRound), board(bigBatch))
	add("GET /batches/{id}/obligations with receipts past int64", "", join(vietnam, bigRound), board(bigBatch), isMember(w.g, w.binh))
	add("GET /batches/{id}/obligations", "", base, board(w.bt1), isMember(w.g, w.chi))

	batches := func(id string) oracleCall { return read("list_context_batches", args("context_id", id)) }
	add("list_context_batches: newest first, ties on created_at by id, a round with no version", "", base,
		batches(w.g), batches(w.g2), batches(w.empty), batches(w.missingContext))
	add("list_context_batches: under a Vietnam session TimeZone", "", vietnam, batches(w.g))
	add("list_context_batches: a round whose total passes int64", "", join(base, bigRound), batches(w.g))
	add("GET /contexts/{id}/batches", "", base, isMember(w.g, w.dung), batches(w.g))

	// --- receipts ----------------------------------------------------------------
	target := func(id string) oracleCall { return read("get_receipt_target", args("obligation_id", id)) }
	add("get_receipt_target: found, in an old version, another group's, missing", "", base,
		target(w.obBinhAn), target(w.obOld), target(w.obBt4), target(w.missingObligation))
	add("get_receipt_target: under a Vietnam session TimeZone", "", vietnam, target(w.obChiAn))
	receiptDumps := []string{receiptsDump, auditDump}
	receive := func(obligation, recipient string, targetAmount int64, by string, amount int64, report any, key, now string) oracleCall {
		return write("save_receipt_confirmation", args("obligation_id", obligation, "recipient_id", recipient,
			"target_amount_vnd", targetAmount, "confirmed_by_id", by, "amount_vnd", amount, "payment_report_id", report,
			"idempotency_key", key, "now", now), receiptDumps...)
	}
	newKey := func(n int) string { return fid(kindReceiptKey, 0x700+n) }
	add("save_receipt_confirmation: a first receipt", "", base, receive(w.obDungBinh, w.binh, 7_000, w.binh, 3_000, nil, newKey(1), moneyNow))
	add("save_receipt_confirmation: against a payment report of the obligation", "", base,
		receive(w.obBinhAn, w.an, 57_000, w.an, 26_000, w.reportBinhEarly, newKey(2), moneyNow))
	add("save_receipt_confirmation: stored keys again answer their receipts", "", base,
		receive(w.obChiAn, w.an, 20_000, w.an, 20_000, nil, w.keyChiAn, moneyNow),
		receive(w.obBinhAn, w.an, 57_000, w.an, 1_000, w.reportBinhLate, w.keyReceiptReported, later(1)))
	add("save_receipt_confirmation: a new receipt, then its key again", "", vietnam,
		receive(w.obAnBinh, w.binh, 15_000, w.binh, 5, nil, newKey(3), moneyNow),
		receive(w.obAnBinh, w.binh, 15_000, w.binh, 5, nil, newKey(3), later(1)))
	add("save_receipt_confirmation: the same key with another amount", "IDEMPOTENCY_KEY_REUSED", base,
		receive(w.obChiAn, w.an, 20_000, w.an, 19_999, nil, w.keyChiAn, moneyNow))
	add("save_receipt_confirmation: the same key from another confirmer", "IDEMPOTENCY_KEY_REUSED", base,
		receive(w.obChiAn, w.an, 20_000, w.chi, 20_000, nil, w.keyChiAn, moneyNow))
	add("save_receipt_confirmation: the same key citing a report it did not", "IDEMPOTENCY_KEY_REUSED", base,
		receive(w.obChiAn, w.an, 20_000, w.an, 20_000, w.reportChi, w.keyChiAn, moneyNow))
	add("save_receipt_confirmation: the same key without the report it cited", "IDEMPOTENCY_KEY_REUSED", base,
		receive(w.obBinhAn, w.an, 57_000, w.an, 1_000, nil, w.keyReceiptReported, moneyNow))
	add("save_receipt_confirmation: the same key on another obligation", "IDEMPOTENCY_KEY_REUSED", base,
		receive(w.obBinhAn, w.an, 57_000, w.an, 20_000, nil, w.keyChiAn, moneyNow))
	add("save_receipt_confirmation: a payment report of another obligation", "PAYMENT_REPORT_NOT_FOR_OBLIGATION", base,
		receive(w.obBinhAn, w.an, 57_000, w.an, 5, w.reportChi, newKey(4), moneyNow))
	add("save_receipt_confirmation: a payment report that does not exist", "PAYMENT_REPORT_NOT_FOR_OBLIGATION", base,
		receive(w.obBinhAn, w.an, 57_000, w.an, 5, fid(kindPaymentReport, 0x5f), newKey(5), moneyNow))
	add("save_receipt_confirmation: an amount of zero", "IntegrityError", base, receive(w.obDungBinh, w.binh, 7_000, w.binh, 0, nil, newKey(6), moneyNow))
	add("save_receipt_confirmation: an obligation with no row", "IntegrityError", base,
		receive(w.missingObligation, w.an, 1, w.an, 1, nil, newKey(7), moneyNow))
	add("save_receipt_confirmation: two receipts at the bigint maximum", "", base,
		receive(w.obDungBinh, w.binh, 7_000, w.binh, max, nil, newKey(8), moneyNow),
		receive(w.obDungBinh, w.binh, 7_000, w.binh, max, nil, newKey(9), later(1)))
	confirmReceipt := func(obligation, by string, amount int64, report any, key, now string) oracleCall {
		return write("flow.confirm_receipt", args("obligation_id", obligation, "confirmed_by_id", by, "amount_vnd", amount,
			"payment_report_id", report, "idempotency_key", key, "now", now), receiptDumps...)
	}
	add("POST /obligations/{id}/confirm-receipt", "", base, confirmReceipt(w.obChiAn, w.an, 500, w.reportChi, newKey(10), moneyNow))
	add("POST /obligations/{id}/confirm-receipt of a missing obligation", "", base,
		confirmReceipt(w.missingObligation, w.an, 500, nil, newKey(11), moneyNow))
	add("POST /obligations/{id}/confirm-receipt replayed, under a Vietnam session TimeZone", "", vietnam,
		confirmReceipt(w.obDungBinh, w.binh, 7_000, nil, newKey(12), moneyNow),
		confirmReceipt(w.obDungBinh, w.binh, 7_000, nil, newKey(12), later(1)))

	// --- finance -----------------------------------------------------------------
	finance := func(person string, limit int) oracleCall {
		return read("person_finance_summary", args("person_id", person, "movement_limit", limit))
	}
	add("person_finance_summary: payer, member, unnamed, erased, stranger, missing", "", base,
		finance(w.an, 20), finance(w.binh, 20), finance(w.dung, 20), finance(w.erased, 20), finance(w.stranger, 20),
		finance(w.missingPerson, 20))
	add("person_finance_summary: limits of none and two", "", base, finance(w.an, 0), finance(w.an, 2))
	add("person_finance_summary: under a Vietnam session TimeZone", "", vietnam, finance(w.binh, 50))
	add("person_finance_summary: a negative limit", "DataError", base, finance(w.an, -1))
	ghostRound := extraBatchSQL(1, w.g, w.dung, "2030-02-03T00:00:00Z",
		extraObligation{w.missingPerson, w.dung, 9_000, []int64{1_000, 2_000}},
		extraObligation{w.dung, w.chi, 4_000, nil})
	add("person_finance_summary: a counterparty with no row, twice in a row", "", join(base, ghostRound), finance(w.dung, 20))
	coffee1, _ := expenseSQL(0x77, w.g, w.chi, nil, moneyVersion{"pending", "Cà phê (dữ liệu mẫu)", "2030-05-05T00:00:00Z", []share{{w.dung, 1_000}}})
	coffee2, _ := expenseSQL(0x78, w.g, w.chi, nil, moneyVersion{"pending", "Cà phê (dữ liệu mẫu)", "2030-05-06T00:00:00Z", []share{{w.dung, 2_000}}})
	coffee3, _ := expenseSQL(0x79, w.g, w.chi, nil, moneyVersion{"pending", "Trà (dữ liệu mẫu)", "2030-05-07T00:00:00Z", []share{{w.dung, 500}}})
	coffeeRound := extraBatchSQL(2, w.g, w.chi, "2030-02-04T00:00:00Z", extraObligation{w.dung, w.chi, 3_500, []int64{3_500}})
	coffeeSources := []string{}
	for _, n := range []int{0x79, 0x78, 0x77} {
		coffeeSources = append(coffeeSources, insertSQL("collection_obligation_sources", "obligation_id", extraObligationID(2, 0),
			"confirmed_allocation_id", fid(kindAllocation, n*16*16+15), "amount_vnd", int64(1), "created_at", stdCreated))
	}
	add("person_finance_summary: an occasion named twice counts once", "",
		join(base, coffee1, coffee2, coffee3, coffeeRound, coffeeSources), finance(w.dung, 20), finance(w.chi, 20))
	big1, _ := expenseSQL(0x71, w.g, w.an, nil, moneyVersion{"acknowledged", "Lớn một (dữ liệu mẫu)", "2030-05-07T00:00:00Z", []share{{w.binh, max}}})
	big2, _ := expenseSQL(0x72, w.g, w.an, nil, moneyVersion{"acknowledged", "Lớn hai (dữ liệu mẫu)", "2030-05-08T00:00:00Z", []share{{w.binh, max}}})
	bigPaid := extraBatchSQL(3, w.g, w.an, "2030-02-05T00:00:00Z", extraObligation{w.binh, w.an, max, []int64{max, max}})
	big3, _ := expenseSQL(0x73, w.g, w.binh, nil, moneyVersion{"acknowledged", nil, "2030-05-09T00:00:00Z", []share{{w.chi, max}}})
	big4, _ := expenseSQL(0x74, w.g, w.binh, nil, moneyVersion{"acknowledged", nil, "2030-05-10T00:00:00Z", []share{{w.dung, max}}})
	bigCollected := extraBatchSQL(4, w.g, w.binh, "2030-02-06T00:00:00Z", extraObligation{w.chi, w.binh, max, []int64{max, max, max}})
	add("person_finance_summary: spend, owed and paid past int64", "", join(base, big1, big2, bigPaid), finance(w.binh, 20))
	add("person_finance_summary: advanced and collected past int64", "", join(base, big3, big4, bigCollected), finance(w.binh, 20))
	add("GET /people/{id}/finance past int64, under a Vietnam session TimeZone", "",
		join(vietnam, big1, big2, big3, big4, bigPaid, bigCollected), finance(w.binh, 20), finance(w.an, 20))
	add("GET /people/{id}/finance", "", base, finance(w.an, 20))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// moneyMethods is every method this port covers; the corpus must reach each
// with at least one normal return.
var moneyMethods = []string{
	"create_expense", "get_expense", "save_expense_confirmation", "flow.create_expense_confirm", "create_bill", "get_bill",
	"confirm_bill_assignments", "claim_bill_items", "save_frozen_batch", "load_batch_for_publish",
	"save_published_batch", "flow.publish", "list_batch_obligations", "list_context_batches", "get_receipt_target",
	"save_receipt_confirmation", "flow.confirm_receipt", "person_finance_summary",
}

func TestMoneyRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of money_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_money_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "money_oracle_")

	cases, built := moneyOracleCases()
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	// UseNumber: an amount at the BIGINT maximum must reach Go as the integer
	// Python reads, not as the nearest float64.
	var spec oracleSpec
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_money_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_money_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	returned := map[string]int{}
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statements = append(statements, normalizeSQL(entry[0].(string)))
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			if s.Error == nil {
				returned[c.Steps[j].Call]++
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}

		// The fixture must reach what the case is named for.
		pyCase := python.Cases[i]
		end := ""
		if len(pyCase.Steps) > 0 {
			if e, ok := pyCase.Steps[len(pyCase.Steps)-1].Error.(map[string]any); ok {
				end, _ = e["type"].(string)
				if code, ok := e["code"].(string); ok {
					end = code
				}
			}
		}
		if len(pyCase.Steps) != len(c.Steps) && end == "" {
			t.Errorf("case %q: python ran %d of %d steps", c.Name, len(pyCase.Steps), len(c.Steps))
		}
		if end != cases[i].wantEnd {
			t.Errorf("case %q: python ended in %q, the case is written for %q", c.Name, end, cases[i].wantEnd)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python refused at step %d of %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}

		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, normalizeMoneySteps(pySteps, c), normalizeMoneySteps(runMoneyGoCase(t, pool, c), c))
		})
	}
	for _, method := range moneyMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	t.Logf("money repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which "+
		"%d table rows, %d generated ids bound, %d mismatches", tally.cases, tally.steps, tally.results, tally.errors,
		tally.statements, tally.probeRows, tally.tableRows, tally.generated, tally.mismatches)
}
