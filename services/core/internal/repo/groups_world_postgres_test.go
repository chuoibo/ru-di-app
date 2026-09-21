//go:build postgres

package repo

// Fixtures for the W3 methods (groups, rosters, balances and the memory
// wall), written as literal SQL through seed_postgres_test.go's world so the
// same statements seed the Go side and the Python side of the oracle. The
// roster has somebody in every membership state and origin (a named invite, a
// link request, a person who left, a person who left and came back, an erased
// account, a person with no name), the ledger has an expense with an old and a
// latest version, one with no allocations, one whose only collectable row is
// the payer's, one another group owns and one between people who left, and the
// wall has hearts, comments that tie on created_at with ids against insertion
// order, and another group's photograph.

const (
	kindBatch        = 0x71
	kindBatchVersion = 0x72
	kindObligation   = 0x73
	kindReceipt      = 0x74
	kindReceiptKey   = 0x75
)

// groupsNow is the clock the write cases hand in.
const groupsNow = "2030-07-10T12:00:00.654321Z"

type groupsWorld struct {
	world
	chu, ban, moi, xin, roi, lai, xoa, trong, la, hai, som, missingPerson string
	g1, g2, g3, pair, missingContext                                      string

	mChu, mBan, mMoi, mXin, mRoi, mLaiOld, mLaiNow, mXoa, mTrong string
	mHai, mChuG2, mSom, mPairChu, mPairBan, missingMembership    string

	photo1, photo2, checkin3, photoQuiet, photoG2, missingMemory string
	commentTrong, commentBan, commentXoa                         string

	e1v1, e1v2, e2v1, e3v1, e4v1, e5v1, e6v1, missingVersion string
}

func (w *groupsWorld) groupPerson(n int, name string, extra ...any) string {
	return w.person(0x30+n, name, extra...)
}

func (w *groupsWorld) groupContext(n int, name, creator string, extra ...any) string {
	id := fid(kindContext, 0x30+n)
	w.insert("contexts", append([]any{"id", id, "display_name", name, "created_by_id", creator,
		"created_at", "2030-06-01T00:00:00Z"}, extra...)...)
	return id
}

// member inserts one membership with its state's timestamps; extra pairs
// (role, origin, invited_by_id, created_at, joined_at) override the defaults.
func (w *groupsWorld) member(n int, contextID, personID, state string, extra ...any) string {
	id := fid(kindMembership, 0x30+n)
	pairs := []any{"id", id, "context_id", contextID, "person_id", personID, "state", state,
		"created_at", "2030-06-02T00:00:00Z"}
	switch state {
	case "active":
		pairs = append(pairs, "joined_at", "2030-06-03T00:00:00Z")
	case "left":
		pairs = append(pairs, "joined_at", "2030-06-03T00:00:00Z", "left_at", "2030-06-04T00:00:00Z")
	}
	w.insert("memberships", append(pairs, extra...)...)
	return id
}

type share struct {
	participant string
	amount      int64
}

// ledgerVersion is one expense version: its total, acknowledgement and the
// allocations in the order they are inserted. Allocation ids are handed out
// in reverse, so insertion order, id order and participant order disagree.
type ledgerVersion struct {
	acknowledgement string
	shares          []share
}

func (w *groupsWorld) ledger(n int, contextID, payer string, versions ...ledgerVersion) []string {
	expense := fid(kindExpense, 0x30+n)
	w.insert("expenses", "id", expense, "context_id", contextID, "created_at", stdCreated)
	var ids []string
	for v, version := range versions {
		id := fid(kindVersion, (0x30+n)*16+v)
		var total int64
		for _, s := range version.shares {
			total += s.amount
		}
		pairs := []any{"id", id, "expense_id", expense, "version_number", v + 1, "recorded_by_id", payer,
			"paid_by_id", payer, "verification_scope", "totals_only", "subtotal_amount_vnd", total,
			"total_amount_vnd", total, "occurred_at", "2030-06-05T00:00:00Z", "created_at", stdCreated,
			"payer_acknowledgement", version.acknowledgement}
		if v > 0 {
			pairs = append(pairs, "previous_version_number", v)
		}
		w.insert("expense_versions", pairs...)
		for s, sh := range version.shares {
			w.insert("confirmed_allocations", "id", fid(kindAllocation, ((0x30+n)*16+v)*16+(15-s)),
				"expense_version_id", id, "participant_id", sh.participant, "amount_vnd", sh.amount,
				"confirmed_by_id", payer, "confirmed_at", stdCreated)
		}
		ids = append(ids, id)
	}
	return ids
}

func allocationID(n, version, share int) string {
	return fid(kindAllocation, ((0x30+n)*16+version)*16+(15-share))
}

// orderedVersion is one version of an extra expense a single case seeds: its
// id exactly as given, and the participants it allocates to (none for a
// version with no allocations).
type orderedVersion struct {
	id     string
	shares []share
}

// orderedLedger is the SQL of one extra expense for one case's own setup,
// kept out of the shared world so no other case sees it. n is below 16;
// allocation ids are fid(kindAllocation, 0xf000+...), clear of ledger's.
func orderedLedger(n int, contextID, payer string, versions ...orderedVersion) []string {
	expense := fid(kindExpense, 0x70+n)
	sql := []string{insertSQL("expenses", "id", expense, "context_id", contextID, "created_at", stdCreated)}
	for v, version := range versions {
		var total int64
		for _, s := range version.shares {
			total += s.amount
		}
		pairs := []any{"id", version.id, "expense_id", expense, "version_number", v + 1, "recorded_by_id", payer,
			"paid_by_id", payer, "verification_scope", "totals_only", "subtotal_amount_vnd", total,
			"total_amount_vnd", total, "occurred_at", "2030-06-05T00:00:00Z", "created_at", stdCreated}
		if v > 0 {
			pairs = append(pairs, "previous_version_number", v)
		}
		sql = append(sql, insertSQL("expense_versions", pairs...))
		for s, sh := range version.shares {
			sql = append(sql, insertSQL("confirmed_allocations", "id", orderedAllocationID(n, v, s),
				"expense_version_id", version.id, "participant_id", sh.participant, "amount_vnd", sh.amount,
				"confirmed_by_id", payer, "confirmed_at", stdCreated))
		}
	}
	return sql
}

func orderedAllocationID(n, version, share int) string {
	return fid(kindAllocation, 0xf000+(n*16+version)*16+share)
}

// largestBigint is the most one BIGINT amount_vnd can hold, math.MaxInt64 as
// Go defines it.
const largestBigint int64 = 1<<63 - 1

// receiptPair is one obligation of a receipt batch and the amounts its
// recipient confirmed against it, in insertion order.
type receiptPair struct {
	sender, recipient string
	amounts           []int64
}

// receiptBatch is the SQL of one frozen batch for one case's own setup: a
// version, one obligation per pair (owing the pair's first amount) and one
// receipt per amount, each confirmed by the pair's recipient. n is below 16;
// ids are clear of newGroupsWorld's.
func receiptBatch(n int, contextID, owner string, pairs ...receiptPair) []string {
	batch, version := fid(kindBatch, 0x100+n), fid(kindBatchVersion, 0x100+n)
	sql := []string{
		insertSQL("collection_batches", "id", batch, "context_id", contextID, "owner_id", owner, "status", "frozen",
			"created_at", stdCreated, "frozen_at", stdCreated),
		insertSQL("collection_batch_versions", "id", version, "batch_id", batch, "version_number", 1,
			"created_by_id", owner, "created_at", stdCreated),
	}
	for p, pair := range pairs {
		obligation := fid(kindObligation, (0x100+n)*16+p)
		sql = append(sql, insertSQL("collection_obligations", "id", obligation, "batch_version_id", version,
			"sender_id", pair.sender, "recipient_id", pair.recipient, "amount_vnd", pair.amounts[0],
			"due_at", "2030-08-01T00:00:00Z", "created_at", stdCreated))
		for r, amount := range pair.amounts {
			key := ((0x100+n)*16+p)*16 + r
			sql = append(sql, insertSQL("receipt_confirmations", "id", fid(kindReceipt, key), "obligation_id", obligation,
				"confirmed_by_id", pair.recipient, "amount_vnd", amount, "idempotency_key", fid(kindReceiptKey, key),
				"confirmed_at", "2030-07-09T00:00:00Z"))
		}
	}
	return sql
}

func newGroupsWorld() *groupsWorld {
	w := &groupsWorld{}
	w.chu = w.groupPerson(1, "Chủ nhóm (dữ liệu mẫu)")
	w.ban = w.groupPerson(2, "Bạn (dữ liệu mẫu)")
	w.moi = w.groupPerson(3, "Được mời (dữ liệu mẫu)")
	w.xin = w.groupPerson(4, "Xin vào (dữ liệu mẫu)")
	w.roi = w.groupPerson(5, "Đã rời (dữ liệu mẫu)")
	w.lai = w.groupPerson(6, "Quay lại (dữ liệu mẫu)")
	w.xoa = w.groupPerson(7, "Đã xoá (dữ liệu mẫu)", "deleted_at", "2030-06-20T00:00:00Z")
	w.trong = w.groupPerson(8, "")
	w.la = w.groupPerson(9, "Người lạ 🌿 (dữ liệu mẫu)")
	w.hai = w.groupPerson(10, "Nhóm hai (dữ liệu mẫu)")
	w.som = w.groupPerson(11, "Vào sớm (dữ liệu mẫu)")
	w.missingPerson = fid(kindPerson, 0xfd)

	w.g1 = w.groupContext(1, "Nhóm một (dữ liệu mẫu)", w.chu)
	w.g2 = w.groupContext(2, "Nhóm hai ' \" (dữ liệu mẫu)", w.hai, "theme", "bien-dem")
	w.g3 = w.groupContext(3, "Nhóm trống (dữ liệu mẫu)", w.la)
	w.pair = w.groupContext(4, "", w.chu, "kind", "pair", "pair_key", "cap-mau-chu-ban")
	w.missingContext = fid(kindContext, 0xfd)

	// g1's roster. mBan and mChu tie on created_at with ids against insertion
	// order; lai left once and came back.
	w.mBan = w.member(2, w.g1, w.ban, "active", "invited_by_id", w.chu)
	w.mChu = w.member(1, w.g1, w.chu, "active", "role", "admin")
	w.mMoi = w.member(3, w.g1, w.moi, "invited", "invited_by_id", w.ban)
	w.mXin = w.member(4, w.g1, w.xin, "invited", "origin", "link")
	w.mRoi = w.member(5, w.g1, w.roi, "left", "invited_by_id", w.chu)
	w.mLaiOld = w.member(6, w.g1, w.lai, "left", "created_at", "2030-06-01T00:00:00Z")
	w.mLaiNow = w.member(7, w.g1, w.lai, "active", "created_at", "2030-06-06T00:00:00Z", "joined_at", "2030-06-06T00:00:00Z")
	w.mXoa = w.member(8, w.g1, w.xoa, "active")
	w.mTrong = w.member(9, w.g1, w.trong, "active")
	// g2: hai runs it, chu is a member, som was invited with joined_at already
	// at the instant the accept case hands in.
	w.mHai = w.member(10, w.g2, w.hai, "active", "role", "admin")
	w.mChuG2 = w.member(11, w.g2, w.chu, "active", "invited_by_id", w.hai)
	w.mSom = w.member(12, w.g2, w.som, "invited", "joined_at", groupsNow)
	w.mPairChu = w.member(13, w.pair, w.chu, "active")
	w.mPairBan = w.member(14, w.pair, w.ban, "active")
	w.missingMembership = fid(kindMembership, 0xfd)

	w.catalogue()

	// The wall of g1, newest photo first: photo2 (by the erased account),
	// photoQuiet, photo1; checkin3 is newer than all of them.
	w.photo1 = w.photo(0x31, w.g1, w.chu, "2030-07-01T00:00:00Z")
	w.photo2 = w.photo(0x32, w.g1, w.xoa, "2030-07-03T00:00:00Z", "caption", "Ảnh 📸 (dữ liệu mẫu)")
	w.checkin3 = w.checkin(0x33, w.g1, w.ban, "2030-07-04T00:00:00Z", "p-b", "Quán p-b (dữ liệu mẫu)", 10.7702, 106.7)
	w.photoQuiet = w.photo(0x34, w.g1, w.ban, "2030-07-02T00:00:00Z", "place_id", "p-c", "place_name", "Quán p-c (dữ liệu mẫu)")
	w.photoG2 = w.photo(0x35, w.g2, w.hai, "2030-07-05T00:00:00Z")
	w.missingMemory = fid(kindMemory, 0xfd)
	heart := func(n int, memory, person string) {
		w.insert("memory_reactions", "id", fid(kindReaction, 0x30+n), "memory_id", memory, "person_id", person,
			"created_at", "2030-07-06T00:00:00Z")
	}
	heart(3, w.photo1, w.chu)
	heart(1, w.photo1, w.ban)
	heart(2, w.photo1, w.lai)
	heart(4, w.photoG2, w.chu)
	comment := func(n int, author, createdAt string) string {
		id := fid(kindComment, 0x30+n)
		w.insert("memory_comments", "id", id, "memory_id", w.photo1, "author_id", author,
			"body", "Lời nhắn (dữ liệu mẫu)", "created_at", createdAt)
		return id
	}
	w.commentBan = comment(2, w.ban, "2030-07-07T00:00:00.5Z")
	w.commentTrong = comment(1, w.trong, "2030-07-07T00:00:00.5Z")
	w.commentXoa = comment(3, w.xoa, "2030-07-08T00:00:00Z")

	// The ledger. Expenses are seeded out of id order.
	ids := w.ledger(2, w.g1, w.ban, ledgerVersion{"acknowledged", []share{{w.chu, 30_000}, {w.ban, 30_000}}})
	w.e2v1 = ids[0]
	ids = w.ledger(1, w.g1, w.chu,
		ledgerVersion{"pending", []share{{w.ban, 50_000}, {w.chu, 50_000}}},
		ledgerVersion{"disputed", []share{{w.trong, 0}, {w.chu, 40_000}, {w.ban, 60_000}}})
	w.e1v1, w.e1v2 = ids[0], ids[1]
	w.e3v1 = w.ledger(3, w.g1, w.chu, ledgerVersion{"pending", nil})[0]
	w.e4v1 = w.ledger(4, w.g1, w.chu, ledgerVersion{"pending", []share{{w.chu, 70_000}, {w.ban, 0}}})[0]
	w.e5v1 = w.ledger(5, w.g2, w.hai, ledgerVersion{"pending", []share{{w.chu, 9_000_000_000_000}}})[0]
	w.e6v1 = w.ledger(6, w.g1, w.lai, ledgerVersion{"pending", []share{{w.roi, 20_000}, {w.lai, 20_000}}})[0]
	w.missingVersion = fid(kindVersion, 0xfffd)

	// Collections. Batch 1 of g1 has two versions; ban owes chu in both, so the
	// confirmed receipts sum across versions. One receipt was confirmed by the
	// sender, which is not a confirmation. Batch 2 belongs to g2.
	batch := func(n int, contextID, owner string) string {
		id := fid(kindBatch, n)
		w.insert("collection_batches", "id", id, "context_id", contextID, "owner_id", owner, "status", "frozen",
			"created_at", stdCreated, "frozen_at", stdCreated)
		return id
	}
	version := func(n int, batchID string, number int) string {
		id := fid(kindBatchVersion, n)
		pairs := []any{"id", id, "batch_id", batchID, "version_number", number, "created_by_id", w.chu, "created_at", stdCreated}
		if number > 1 {
			pairs = append(pairs, "previous_version_number", number-1)
		}
		w.insert("collection_batch_versions", pairs...)
		return id
	}
	obligation := func(n int, versionID, sender, recipient string, amount int64) string {
		id := fid(kindObligation, n)
		w.insert("collection_obligations", "id", id, "batch_version_id", versionID, "sender_id", sender,
			"recipient_id", recipient, "amount_vnd", amount, "due_at", "2030-08-01T00:00:00Z", "created_at", stdCreated)
		return id
	}
	receipt := func(n int, obligationID, confirmedBy string, amount int64) {
		w.insert("receipt_confirmations", "id", fid(kindReceipt, n), "obligation_id", obligationID,
			"confirmed_by_id", confirmedBy, "amount_vnd", amount, "idempotency_key", fid(kindReceiptKey, n),
			"confirmed_at", "2030-07-09T00:00:00Z")
	}
	b1 := batch(1, w.g1, w.chu)
	b1v1 := version(1, b1, 1)
	b1v2 := version(2, b1, 2)
	ob1 := obligation(1, b1v1, w.ban, w.chu, 60_000)
	w.insert("collection_obligation_sources", "obligation_id", ob1, "confirmed_allocation_id", allocationID(1, 1, 2),
		"amount_vnd", int64(60_000))
	ob2 := obligation(2, b1v1, w.trong, w.ban, 15_000)
	ob3 := obligation(3, b1v2, w.ban, w.chu, 7_000)
	ob4 := obligation(4, b1v2, w.roi, w.lai, 20_000)
	receipt(1, ob1, w.chu, 25_000)
	receipt(2, ob1, w.chu, 10_000)
	receipt(3, ob1, w.ban, 5_000)
	receipt(4, ob2, w.ban, 15_000)
	receipt(5, ob3, w.chu, 7_000)
	receipt(6, ob4, w.lai, 20_000)
	b2 := batch(2, w.g2, w.hai)
	ob5 := obligation(5, version(3, b2, 1), w.chu, w.hai, 99_000)
	receipt(7, ob5, w.hai, 99_000)
	return w
}
