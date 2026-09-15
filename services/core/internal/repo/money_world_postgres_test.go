//go:build postgres

package repo

// Fixtures for the W4 methods (expenses, bills, collection rounds, receipts
// and the finance screen), written as literal SQL through
// seed_postgres_test.go's world so the same statements seed the Go side and
// the Python side of the oracle.
//
// The ledger has an expense with a pending and an acknowledged version, one
// with no description, one with an empty description, one with no version at
// all, and one in another group. The
// bill has items tied on position and inserted out of key order, suggested
// and confirmed shares, and surcharges and discounts inserted out of key
// order. The collection round has two versions; its board has receipts
// tied on confirmed_at with ids against
// insertion order, a receipt that cites a payment report, reports whose
// earliest is not the first inserted, and objection events tied on
// occurred_at, missing or empty or non-text obligation ids, other event
// types, events on another version's link and on no link. The group's rounds
// tie on created_at in pairs, one has no version, and another group owns one.
//
// Every tie here is broken by a later ORDER BY key. A tie on a single-key
// ORDER BY (the board's sender_id, an occasion's occurred_at) comes back in
// whatever order the plan produces, and the shared schema's heap grows between
// the Python pass and the Go pass, so no round has a sender owing twice and no
// obligation's sources share an occurred_at; the statement text still pins
// the ORDER BY itself.

import "fmt"

const (
	kindBill          = 0x76
	kindBillItem      = 0x77
	kindBillShare     = 0x78
	kindEnvelope      = 0x79
	kindGuestLink     = 0x7a
	kindPaymentReport = 0x7b
	kindAuditEvent    = 0x7c
)

// moneyNow is the clock the write cases hand in; a later step of the same
// case hands in a later one, so no two generated rows tie on it.
const moneyNow = "2030-07-10T12:00:00.654321Z"

type moneyVersion struct {
	acknowledgement string
	description     any
	occurredAt      string
	shares          []share
}

type moneyWorld struct {
	world
	an, binh, chi, dung, erased, stranger, missingPerson string
	g, g2, empty, missingContext                         string

	e1, e2, e3, e4, e5, e6, missingExpense string
	e1v1, e1v2, e2v1, e3v1, e5v1, e6v1     string
	alloc                                  map[string]string

	b1, b2, b3, missingBill, itemLau, itemBia, itemCom string

	bt1, bt2, bt3, bt4, bt5, missingBatch                         string
	bt1v1, bt1v2, bt2v1, bt4v1, bt5v1                             string
	obOld, obBinhAn, obChiAn, obDungBinh, obAnBinh, obBt2, obBt4  string
	obBt5, missingObligation                                      string
	linkOld, linkBinh, linkChi                                    string
	reportBinhLate, reportBinhEarly, reportChi, reportOld         string
	receiptChiAn, keyChiAn, receiptWithReport, keyReceiptReported string
}

// allocKey names one allocation of the world: its version and participant.
func allocKey(version, participant string) string { return version + "/" + participant }

// expenseSQL is the SQL of one expense and its versions. Versions are
// fid(kindVersion, n*16+v) and allocations fid(kindAllocation,
// (n*16+v)*16+(15-s)), handed out in reverse so insertion, id and participant
// orders disagree. Each version's total is the sum of its shares.
func expenseSQL(n int, contextID, payer string, alloc map[string]string, versions ...moneyVersion) ([]string, []string) {
	expense := fid(kindExpense, n)
	sql := []string{insertSQL("expenses", "id", expense, "context_id", contextID, "created_at", stdCreated)}
	var ids []string
	for v, version := range versions {
		id := fid(kindVersion, n*16+v)
		var total int64
		for _, s := range version.shares {
			total += s.amount
		}
		pairs := []any{"id", id, "expense_id", expense, "version_number", v + 1, "recorded_by_id", payer,
			"paid_by_id", payer, "verification_scope", "totals_only", "subtotal_amount_vnd", total,
			"total_amount_vnd", total, "occurred_at", version.occurredAt, "created_at", stdCreated,
			"payer_acknowledgement", version.acknowledgement, "description", version.description}
		if v > 0 {
			pairs = append(pairs, "previous_version_number", v)
		}
		sql = append(sql, insertSQL("expense_versions", pairs...))
		for s, sh := range version.shares {
			allocation := fid(kindAllocation, (n*16+v)*16+(15-s))
			sql = append(sql, insertSQL("confirmed_allocations", "id", allocation, "expense_version_id", id,
				"participant_id", sh.participant, "amount_vnd", sh.amount, "confirmed_by_id", payer,
				"confirmed_at", stdCreated))
			if alloc != nil {
				alloc[allocKey(id, sh.participant)] = allocation
			}
		}
		ids = append(ids, id)
	}
	return sql, ids
}

// extraObligation is one obligation of a case's own round and the receipts
// its recipient confirmed, in insertion order.
type extraObligation struct {
	sender, recipient string
	amount            int64
	receipts          []int64
}

// extraBatchSQL is one case's own frozen round, version 1, with its
// obligations and receipts. n is below 16; ids are clear of the world's.
func extraBatchSQL(n int, contextID, owner, createdAt string, obligations ...extraObligation) []string {
	batch, version := fid(kindBatch, 0x60+n), fid(kindBatchVersion, 0x60+n)
	sql := []string{
		insertSQL("collection_batches", "id", batch, "context_id", contextID, "owner_id", owner, "status", "frozen",
			"created_at", createdAt, "frozen_at", createdAt),
		insertSQL("collection_batch_versions", "id", version, "batch_id", batch, "version_number", 1,
			"created_by_id", owner, "created_at", createdAt),
	}
	for i, o := range obligations {
		obligation := fid(kindObligation, (0x60+n)*16+i)
		sql = append(sql, insertSQL("collection_obligations", "id", obligation, "batch_version_id", version,
			"sender_id", o.sender, "recipient_id", o.recipient, "amount_vnd", o.amount,
			"due_at", "2030-08-01T00:00:00Z", "created_at", createdAt))
		for r, amount := range o.receipts {
			key := ((0x60+n)*16+i)*16 + r
			sql = append(sql, insertSQL("receipt_confirmations", "id", fid(kindReceipt, key), "obligation_id", obligation,
				"confirmed_by_id", o.recipient, "amount_vnd", amount, "idempotency_key", fid(kindReceiptKey, key),
				"confirmed_at", fmt.Sprintf("2030-06-2%dT00:00:00Z", r%10)))
		}
	}
	return sql
}

func extraObligationID(n, i int) string { return fid(kindObligation, (0x60+n)*16+i) }

func newMoneyWorld() *moneyWorld {
	w := &moneyWorld{alloc: map[string]string{}}
	w.an = w.person(0x51, "An (dữ liệu mẫu)")
	w.binh = w.person(0x52, "Bình (dữ liệu mẫu)")
	w.chi = w.person(0x53, "Chi 🌿 (dữ liệu mẫu)")
	w.dung = w.person(0x54, "")
	w.erased = w.person(0x55, "Đã xoá (dữ liệu mẫu)", "deleted_at", "2030-06-20T00:00:00Z")
	w.stranger = w.person(0x56, "Người lạ (dữ liệu mẫu)")
	w.missingPerson = fid(kindPerson, 0x5f)

	group := func(n int, name, creator string) string {
		id := fid(kindContext, 0x50+n)
		w.insert("contexts", "id", id, "display_name", name, "created_by_id", creator, "created_at", stdCreated)
		return id
	}
	w.g = group(1, "Nhóm tiền (dữ liệu mẫu)", w.an)
	w.g2 = group(2, "Nhóm hai ' \" (dữ liệu mẫu)", w.chi)
	w.empty = group(3, "Nhóm trống (dữ liệu mẫu)", w.stranger)
	w.missingContext = fid(kindContext, 0x5f)

	member := func(n int, contextID, personID, state string, extra ...any) {
		pairs := []any{"id", fid(kindMembership, 0x50+n), "context_id", contextID, "person_id", personID,
			"state", state, "created_at", "2030-01-02T00:00:00Z"}
		switch state {
		case "active":
			pairs = append(pairs, "joined_at", "2030-01-03T00:00:00Z")
		case "left":
			pairs = append(pairs, "joined_at", "2030-01-03T00:00:00Z", "left_at", "2030-01-04T00:00:00Z")
		}
		w.insert("memberships", append(pairs, extra...)...)
	}
	member(1, w.g, w.an, "active", "role", "admin")
	member(2, w.g, w.binh, "active")
	member(3, w.g, w.chi, "active")
	member(4, w.g, w.dung, "active")
	member(5, w.g, w.erased, "left")
	member(6, w.g2, w.chi, "active", "role", "admin")
	member(7, w.g2, w.binh, "active")
	member(8, w.g2, w.an, "invited")

	expense := func(n int, contextID, payer string, versions ...moneyVersion) []string {
		sql, ids := expenseSQL(n, contextID, payer, w.alloc, versions...)
		w.sql = append(w.sql, sql...)
		return ids
	}
	lunch := "2030-05-01T12:00:00Z"
	ids := expense(0x51, w.g, w.an,
		moneyVersion{"pending", "Lẩu (dữ liệu mẫu)", lunch, []share{{w.an, 50_000}, {w.binh, 30_000}, {w.chi, 20_000}}},
		moneyVersion{"acknowledged", "Lẩu, sửa lại (dữ liệu mẫu)", lunch,
			[]share{{w.chi, 20_000}, {w.an, 40_000}, {w.binh, 40_000}, {w.dung, 0}}})
	w.e1, w.e1v1, w.e1v2 = fid(kindExpense, 0x51), ids[0], ids[1]
	w.e2v1 = expense(0x52, w.g, w.binh,
		moneyVersion{"acknowledged", nil, "2030-05-02T08:00:00Z", []share{{w.an, 15_000}, {w.binh, 15_000}}})[0]
	w.e3v1 = expense(0x53, w.g2, w.chi,
		moneyVersion{"pending", "Nhóm hai (dữ liệu mẫu)", "2030-05-02T09:00:00Z", []share{{w.binh, 9_000}, {w.chi, 1_000}}})[0]
	expense(0x54, w.g, w.an)
	w.e5v1 = expense(0x55, w.g, w.an,
		moneyVersion{"acknowledged", "", "2030-05-03T10:00:00Z", []share{{w.binh, 5_000}, {w.an, 5_000}}})[0]
	w.e6v1 = expense(0x56, w.g, w.an,
		moneyVersion{"acknowledged", "Cà phê (dữ liệu mẫu)", "2030-05-01T13:00:00Z", []share{{w.binh, 12_000}, {w.an, 12_000}}})[0]
	w.e2, w.e3, w.e4, w.e5, w.e6 = fid(kindExpense, 0x52), fid(kindExpense, 0x53), fid(kindExpense, 0x54),
		fid(kindExpense, 0x55), fid(kindExpense, 0x56)
	w.missingExpense = fid(kindExpense, 0x5f)

	w.insert("outings", "id", fid(kindOuting, 0x51), "context_id", w.g, "created_by_id", w.an,
		"title", "Đà Lạt (dữ liệu mẫu)", "starts_on", "2030-05-01", "ends_on", "2030-05-03", "headcount", 4,
		"budget_per_person_vnd", int64(500_000), "created_at", stdCreated)

	// Bills.
	bill := func(n int, contextID, createdBy string, printed any, items int64, confidence int, review bool, createdAt string) string {
		id := fid(kindBill, 0x50+n)
		w.insert("bills", "id", id, "context_id", contextID, "created_by_id", createdBy, "printed_total_vnd", printed,
			"items_total_vnd", items, "confidence", confidence, "needs_review", review, "created_at", createdAt)
		return id
	}
	item := func(n int, billID, key, name string, position, quantity int, unit any, total int64) string {
		id := fid(kindBillItem, 0x50+n)
		w.insert("bill_items", "id", id, "bill_id", billID, "item_key", key, "name", name, "quantity", quantity,
			"unit_price_vnd", unit, "line_total_vnd", total, "position", position)
		return id
	}
	shareRow := func(n int, itemID, participant, source string, decidedBy, decidedAt any) {
		w.insert("bill_item_shares", "id", fid(kindBillShare, 0x50+n), "bill_item_id", itemID,
			"participant_id", participant, "source", source, "decided_by_id", decidedBy, "decided_at", decidedAt)
	}
	w.b1 = bill(1, w.g, w.an, int64(125_000), 110_000, 87, true, "2030-04-01T09:00:00.25Z")
	w.itemLau = item(3, w.b1, "k-lau", "Lẩu (dữ liệu mẫu)", 1, 1, nil, 80_000)
	w.itemBia = item(1, w.b1, "k-bia", "Bia 🍺 (dữ liệu mẫu)", 0, 4, int64(5_000), 20_000)
	w.itemCom = item(2, w.b1, "k-com", "Cơm ' \" (dữ liệu mẫu)", 1, 2, int64(5_000), 10_000)
	shareRow(4, w.itemLau, w.chi, "confirmed", w.an, "2030-04-02T00:00:00Z")
	shareRow(3, w.itemLau, w.an, "confirmed", w.an, "2030-04-02T00:00:00Z")
	shareRow(6, w.itemBia, w.binh, "ai_suggested", nil, nil)
	shareRow(5, w.itemBia, w.an, "ai_suggested", nil, nil)
	shareRow(7, w.itemBia, w.dung, "confirmed", w.dung, "2030-04-02T00:00:01.5Z")
	w.insert("bill_surcharges", "id", fid(kindBill, 0x1002), "bill_id", w.b1, "surcharge_key", "s-vat", "kind", "vat",
		"amount_vnd", int64(8_000), "mode", "proportional")
	w.insert("bill_surcharges", "id", fid(kindBill, 0x1001), "bill_id", w.b1, "surcharge_key", "s-phi",
		"kind", "phi-dich-vu", "amount_vnd", int64(2_000), "mode", "even")
	w.insert("bill_discounts", "id", fid(kindBill, 0x2002), "bill_id", w.b1, "discount_key", "d-toan",
		"amount_vnd", int64(1_000), "scope", "global_proportional", "target_item_key", nil)
	w.insert("bill_discounts", "id", fid(kindBill, 0x2001), "bill_id", w.b1, "discount_key", "d-lau",
		"amount_vnd", int64(5_000), "scope", "item", "target_item_key", "k-lau")
	w.b2 = bill(2, w.g, w.binh, nil, 0, 0, false, "2030-04-03T00:00:00Z")
	w.b3 = bill(3, w.g2, w.chi, int64(9_000), 9_000, 100, false, "2030-04-04T00:00:00Z")
	shareRow(9, item(4, w.b3, "k1", "Phở (dữ liệu mẫu)", 0, 1, nil, 9_000), w.binh, "ai_suggested", nil, nil)
	w.missingBill = fid(kindBill, 0x5f)

	// Collection rounds.
	batch := func(n int, contextID, owner, status, createdAt string, extra ...any) string {
		id := fid(kindBatch, 0x50+n)
		w.insert("collection_batches", append([]any{"id", id, "context_id", contextID, "owner_id", owner,
			"status", status, "created_at", createdAt}, extra...)...)
		return id
	}
	version := func(n int, batchID string, number int, createdBy string) string {
		id := fid(kindBatchVersion, 0x50+n)
		pairs := []any{"id", id, "batch_id", batchID, "version_number", number, "created_by_id", createdBy,
			"created_at", stdCreated}
		if number > 1 {
			pairs = append(pairs, "previous_version_number", number-1)
		}
		w.insert("collection_batch_versions", pairs...)
		return id
	}
	obligation := func(n int, versionID, sender, recipient string, amount int64) string {
		id := fid(kindObligation, 0x50+n)
		w.insert("collection_obligations", "id", id, "batch_version_id", versionID, "sender_id", sender,
			"recipient_id", recipient, "amount_vnd", amount, "due_at", "2030-08-01T00:00:00Z", "created_at", stdCreated)
		return id
	}
	source := func(obligationID, allocation string, amount int64) {
		w.insert("collection_obligation_sources", "obligation_id", obligationID, "confirmed_allocation_id", allocation,
			"amount_vnd", amount, "created_at", stdCreated)
	}
	receipt := func(n int, obligationID, by string, amount int64, at string, extra ...any) (string, string) {
		id, key := fid(kindReceipt, 0x50+n), fid(kindReceiptKey, 0x50+n)
		w.insert("receipt_confirmations", append([]any{"id", id, "obligation_id", obligationID, "confirmed_by_id", by,
			"amount_vnd", amount, "idempotency_key", key, "confirmed_at", at}, extra...)...)
		return id, key
	}
	envelope := func(n int, versionID, sender string) string {
		id := fid(kindEnvelope, 0x50+n)
		w.insert("collection_envelopes", "id", id, "batch_version_id", versionID, "sender_id", sender, "created_at", stdCreated)
		return id
	}
	link := func(n int, envelopeID, status string) string {
		id := fid(kindGuestLink, 0x50+n)
		w.insert("guest_links", "id", id, "envelope_id", envelopeID,
			"token_digest", raw(fmt.Sprintf("decode(repeat('%02x', 32), 'hex')", n)), "status", status,
			"expires_at", "2031-01-01T00:00:00Z", "created_at", stdCreated)
		return id
	}
	report := func(n int, obligationID, linkID string, amount int64, at string) string {
		id := fid(kindPaymentReport, 0x50+n)
		w.insert("payment_reports", "id", id, "obligation_id", obligationID, "guest_link_id", linkID,
			"amount_vnd", amount, "idempotency_key", fid(kindReceiptKey, 0x900+n), "reported_at", at)
		return id
	}
	objection := func(n int, linkID, eventType, data, at string, extra ...any) {
		w.insert("audit_events", append([]any{"id", fid(kindAuditEvent, 0x50+n), "event_type", eventType,
			"aggregate_type", "guest_link", "aggregate_id", linkID, "event_data", data, "occurred_at", at}, extra...)...)
	}

	w.bt1 = batch(1, w.g, w.an, "frozen", stdCreated, "frozen_at", stdCreated)
	w.bt5 = batch(5, w.g, w.an, "frozen", stdCreated, "frozen_at", stdCreated)
	w.bt2 = batch(2, w.g, w.binh, "published", "2030-01-02T00:00:00Z", "frozen_at", "2030-01-02T00:00:00Z",
		"published_at", "2030-01-03T00:00:00Z")
	w.bt3 = batch(3, w.g, w.an, "accruing", "2030-01-02T00:00:00Z")
	w.bt4 = batch(4, w.g2, w.chi, "frozen", stdCreated, "frozen_at", stdCreated)
	w.missingBatch = fid(kindBatch, 0x5f)

	w.bt1v1 = version(1, w.bt1, 1, w.an)
	w.bt1v2 = version(2, w.bt1, 2, w.an)
	w.obOld = obligation(1, w.bt1v1, w.binh, w.an, 30_000)
	source(w.obOld, w.alloc[allocKey(w.e1v1, w.binh)], 30_000)
	w.obDungBinh = obligation(5, w.bt1v2, w.dung, w.binh, 7_000)
	w.obBinhAn = obligation(3, w.bt1v2, w.binh, w.an, 57_000)
	source(w.obBinhAn, w.alloc[allocKey(w.e1v2, w.binh)], 40_000)
	source(w.obBinhAn, w.alloc[allocKey(w.e6v1, w.binh)], 12_000)
	source(w.obBinhAn, w.alloc[allocKey(w.e5v1, w.binh)], 5_000)
	w.obChiAn = obligation(4, w.bt1v2, w.chi, w.an, 20_000)
	source(w.obChiAn, w.alloc[allocKey(w.e1v2, w.chi)], 20_000)
	w.obAnBinh = obligation(2, w.bt1v2, w.an, w.binh, 15_000)
	source(w.obAnBinh, w.alloc[allocKey(w.e2v1, w.an)], 15_000)

	envOld := envelope(1, w.bt1v1, w.binh)
	w.linkOld = link(1, envOld, "active")
	w.linkBinh = link(2, envelope(2, w.bt1v2, w.binh), "active")
	w.linkChi = link(3, envelope(3, w.bt1v2, w.chi), "revoked")
	w.reportBinhLate = report(1, w.obBinhAn, w.linkBinh, 57_000, "2030-06-05T00:00:00Z")
	w.reportBinhEarly = report(2, w.obBinhAn, w.linkBinh, 57_000, "2030-05-30T00:00:00Z")
	w.reportChi = report(3, w.obChiAn, w.linkChi, 20_000, "2030-06-06T00:00:00Z")
	w.reportOld = report(4, w.obOld, w.linkOld, 30_000, "2030-05-21T00:00:00Z")

	receipt(2, w.obBinhAn, w.an, 20_000, "2030-06-01T00:00:00Z")
	receipt(1, w.obBinhAn, w.an, 10_000, "2030-06-01T00:00:00Z")
	w.receiptChiAn, w.keyChiAn = receipt(3, w.obChiAn, w.an, 20_000, "2030-06-02T00:00:00Z")
	receipt(4, w.obAnBinh, w.binh, 10_000, "2030-06-03T00:00:00Z")
	receipt(5, w.obAnBinh, w.binh, 10_000, "2030-06-04T00:00:00Z")
	receipt(6, w.obOld, w.an, 5_000, "2030-05-20T00:00:00Z")
	w.receiptWithReport, w.keyReceiptReported = receipt(8, w.obBinhAn, w.an, 1_000, "2030-06-07T00:00:00Z",
		"payment_report_id", w.reportBinhLate)

	wrong := "guest_objection.wrong_amount"
	objection(2, w.linkChi, wrong, `{"obligation_id": "`+w.obChiAn+`", "reason": "so_tien_sai"}`, "2030-06-10T00:00:00Z")
	objection(1, w.linkChi, wrong, `{"obligation_id": "`+w.obChiAn+`", "reason": "chua_nhan"}`, "2030-06-10T00:00:00Z")
	objection(3, w.linkBinh, wrong, `{"obligation_id": "`+w.obBinhAn+`"}`, "2030-06-11T00:00:00Z")
	objection(4, w.linkBinh, wrong, `{"obligation_id": "", "reason": "rong"}`, "2030-06-09T00:00:00Z")
	objection(5, w.linkBinh, wrong, `{"kind": "wrong_amount", "reason": "khong_co_ma"}`, "2030-06-09T00:00:00Z")
	objection(6, w.linkBinh, wrong, `{"obligation_id": 7, "reason": "so"}`, "2030-06-09T00:00:00Z")
	objection(12, w.linkBinh, wrong, `{"obligation_id": null, "reason": "null"}`, "2030-06-09T00:00:00Z")
	objection(7, w.linkBinh, "guest_objection.not_me", `{"obligation_id": "`+w.obAnBinh+`", "reason": "khong_phai_toi"}`,
		"2030-06-08T00:00:00Z")
	objection(11, w.linkBinh, "guest_objection.evidence_request", `{"obligation_id": "`+w.obAnBinh+`"}`,
		"2030-06-08T00:00:00Z")
	objection(8, w.linkOld, wrong, `{"obligation_id": "`+w.obAnBinh+`", "reason": "phien_ban_cu"}`, "2030-06-08T00:00:00Z")
	objection(9, fid(kindGuestLink, 0x5e), wrong, `{"obligation_id": "`+w.obAnBinh+`", "reason": "khong_lien_ket"}`,
		"2030-06-08T00:00:00Z")
	objection(10, w.linkBinh, wrong, `{"obligation_id": "`+w.obAnBinh+`", "reason": "loai_khac"}`,
		"2030-06-08T00:00:00Z", "aggregate_type", "collection_obligation")

	w.bt2v1 = version(3, w.bt2, 1, w.binh)
	w.obBt2 = obligation(6, w.bt2v1, w.chi, w.an, 20_000)
	source(w.obBt2, w.alloc[allocKey(w.e1v1, w.chi)], 20_000)
	w.bt4v1 = version(4, w.bt4, 1, w.chi)
	w.obBt4 = obligation(7, w.bt4v1, w.binh, w.chi, 9_000)
	source(w.obBt4, w.alloc[allocKey(w.e3v1, w.binh)], 9_000)
	receipt(9, w.obBt4, w.chi, 9_000, "2030-06-12T00:00:00Z")
	w.bt5v1 = version(5, w.bt5, 1, w.an)
	w.obBt5 = obligation(8, w.bt5v1, w.dung, w.an, 1_000)
	w.missingObligation = fid(kindObligation, 0x5f)
	return w
}
