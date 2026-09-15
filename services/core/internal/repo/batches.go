package repo

// Collection rounds: save_frozen_batch, load_batch_for_publish,
// save_published_batch, list_batch_obligations and list_context_batches.
// Obligation state is never read from a column: it is derived from the
// receipt_confirmations rows on every read (obligationStatus), and a dispute
// from the guest objection events.

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// ObligationDraft is ObligationDraft. Sources are the allocation rows the
// obligation is built from, each written as an obligation source.
type ObligationDraft struct {
	SenderID                string
	RecipientID             string
	AmountVND               int64
	SourceExpenseVersionIDs []string
	Sources                 []AllocationRow
}

// FrozenObligation is FrozenObligation.
type FrozenObligation struct {
	ID                      string
	SenderID                string
	RecipientID             string
	AmountVND               int64
	DueAt                   time.Time
	SourceExpenseVersionIDs []string
}

// FrozenBatch is FrozenBatch.
type FrozenBatch struct {
	ID          string
	VersionID   string
	Obligations []FrozenObligation
}

// FrozenBatchInput is save_frozen_batch's keyword arguments.
type FrozenBatchInput struct {
	ContextID   string
	OwnerID     string
	DueAt       time.Time
	Obligations []ObligationDraft
	Now         time.Time
}

// PublishObligation is PublishObligation.
type PublishObligation struct {
	ID             string
	BatchVersionID string
	SenderID       string
	RecipientID    string
	AmountVND      int64
}

// BatchForPublish is BatchForPublish.
type BatchForPublish struct {
	ID                   string
	VersionID            string
	OwnerID              string
	Status               string
	ContextID            string
	AdvancerAcknowledged bool
	Obligations          []PublishObligation
}

// GuestLinkDraft is GuestLinkDraft.
type GuestLinkDraft struct {
	SenderID    string
	TokenDigest []byte
	ExpiresAt   time.Time
}

// StoredGuestLink is StoredGuestLink.
type StoredGuestLink struct {
	ID         string
	EnvelopeID string
	SenderID   string
}

// BatchObligationRow is BatchObligationRow.
type BatchObligationRow struct {
	ObligationID      string
	SenderID          string
	RecipientID       string
	AmountVND         int64
	Status            string
	Disputed          bool
	DisputedReason    *string
	PaymentReportedAt *time.Time
}

// BatchBoard is BatchBoard.
type BatchBoard struct {
	ContextID   string
	Obligations []BatchObligationRow
}

// ContextBatchRow is ContextBatchRow. TotalVND is Python's sum of the
// obligation amounts, exact past int64.
type ContextBatchRow struct {
	BatchID         string
	Status          string
	CreatedAt       time.Time
	PublishedAt     *time.Time
	ObligationCount int64
	ConfirmedCount  int64
	DisputedCount   int64
	TotalVND        *big.Int
}

var (
	collectionBatchInsert = []insertColumn{{"id", "::UUID"}, {"context_id", "::UUID"}, {"owner_id", "::UUID"},
		{"status", ""}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}, {"frozen_at", "::TIMESTAMP WITH TIME ZONE"},
		{"published_at", "::TIMESTAMP WITH TIME ZONE"}, {"collecting_at", "::TIMESTAMP WITH TIME ZONE"},
		{"closed_at", "::TIMESTAMP WITH TIME ZONE"}}
	batchVersionInsert = []insertColumn{{"id", "::UUID"}, {"batch_id", "::UUID"}, {"version_number", "::INTEGER"},
		{"previous_version_number", "::INTEGER"}, {"created_by_id", "::UUID"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}
	obligationInsert = []insertColumn{{"id", "::UUID"}, {"batch_version_id", "::UUID"}, {"sender_id", "::UUID"},
		{"recipient_id", "::UUID"}, {"amount_vnd", "::BIGINT"}, {"due_at", "::TIMESTAMP WITH TIME ZONE"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}
	obligationSourceInsert = []insertColumn{{"obligation_id", "::UUID"}, {"confirmed_allocation_id", "::UUID"},
		{"amount_vnd", "::BIGINT"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}}
	envelopeInsert = []insertColumn{{"id", "::UUID"}, {"batch_version_id", "::UUID"}, {"sender_id", "::UUID"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}
	guestLinkInsert = []insertColumn{{"id", "::UUID"}, {"envelope_id", "::UUID"}, {"token_digest", ""},
		{"status", ""}, {"expires_at", "::TIMESTAMP WITH TIME ZONE"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"},
		{"capability_exposed_at", "::TIMESTAMP WITH TIME ZONE"}, {"first_opened_at", "::TIMESTAMP WITH TIME ZONE"},
		{"revoked_at", "::TIMESTAMP WITH TIME ZONE"}, {"rotated_from_id", "::UUID"}}
)

func itoa(n int) string { return strconv.Itoa(n) }

func sortStrings(values []string) { sort.Strings(values) }

// SaveFrozenBatch is save_frozen_batch: a frozen batch, its first version,
// one obligation per draft with its sources, and the audit event.
//
// Flushes, in Python's order:
//  1. the batch INSERT (status frozen, created_at and frozen_at at the
//     caller's clock, the other instants NULL);
//  2. the version INSERT (number 1, no previous);
//  3. per draft: its obligation INSERT, then (same flush, next mapper) the
//     sources of the draft before it, one INSERT each;
//  4. the audit event, then the last draft's sources.
//
// Every PostgreSQL refusal (a repeated pair, a party paying itself, an
// amount below one, a source allocation with no row) ends the method where
// it happens and is returned as is.
func (r Repository) SaveFrozenBatch(ctx context.Context, in FrozenBatchInput) (FrozenBatch, error) {
	batchID, err := newUUID()
	if err != nil {
		return FrozenBatch{}, err
	}
	versionID, err := newUUID()
	if err != nil {
		return FrozenBatch{}, err
	}
	now, due := pythonInstant(in.Now), pythonInstant(in.DueAt)
	if _, err := r.Q.Exec(ctx, renderInsert("collection_batches", collectionBatchInsert, 1),
		batchID, in.ContextID, in.OwnerID, "frozen", now, now, nil, nil, nil); err != nil {
		return FrozenBatch{}, err
	}
	if _, err := r.Q.Exec(ctx, renderInsert("collection_batch_versions", batchVersionInsert, 1),
		versionID, batchID, int32(1), nil, in.OwnerID, now); err != nil {
		return FrozenBatch{}, err
	}
	out := FrozenBatch{ID: batchID, VersionID: versionID, Obligations: []FrozenObligation{}}
	var pending [][]any
	for _, draft := range in.Obligations {
		id, err := newUUID()
		if err != nil {
			return FrozenBatch{}, err
		}
		if _, err := r.Q.Exec(ctx, renderInsert("collection_obligations", obligationInsert, 1),
			id, versionID, draft.SenderID, draft.RecipientID, draft.AmountVND, due, now); err != nil {
			return FrozenBatch{}, err
		}
		if err := r.insertEach(ctx, "collection_obligation_sources", obligationSourceInsert, pending); err != nil {
			return FrozenBatch{}, err
		}
		pending = nil
		for _, source := range draft.Sources {
			pending = append(pending, []any{id, source.ID, source.AmountVND, now})
		}
		out.Obligations = append(out.Obligations, FrozenObligation{ID: id, SenderID: draft.SenderID,
			RecipientID: draft.RecipientID, AmountVND: draft.AmountVND, DueAt: due,
			SourceExpenseVersionIDs: append([]string{}, draft.SourceExpenseVersionIDs...)})
	}
	if err := r.insertAudit(ctx, auditEvent{actorID: &in.OwnerID, eventType: "collection_batch_frozen",
		aggregateType: "collection_batch", aggregateID: batchID, occurredAt: now,
		eventData: map[string]any{"batch_version_id": versionID, "obligation_count": len(out.Obligations)}}); err != nil {
		return FrozenBatch{}, err
	}
	if err := r.insertEach(ctx, "collection_obligation_sources", obligationSourceInsert, pending); err != nil {
		return FrozenBatch{}, err
	}
	return out, nil
}

const collectionBatchColumns = `collection_batches.id, collection_batches.context_id, collection_batches.owner_id,
       collection_batches.status, collection_batches.created_at, collection_batches.frozen_at,
       collection_batches.published_at, collection_batches.collecting_at, collection_batches.closed_at`

const collectionBatchColumnsLabelled = `collection_batches.id AS collection_batches_id,
       collection_batches.context_id AS collection_batches_context_id,
       collection_batches.owner_id AS collection_batches_owner_id,
       collection_batches.status AS collection_batches_status,
       collection_batches.created_at AS collection_batches_created_at,
       collection_batches.frozen_at AS collection_batches_frozen_at,
       collection_batches.published_at AS collection_batches_published_at,
       collection_batches.collecting_at AS collection_batches_collecting_at,
       collection_batches.closed_at AS collection_batches_closed_at`

const obligationColumns = `collection_obligations.id, collection_obligations.batch_version_id,
       collection_obligations.sender_id, collection_obligations.recipient_id, collection_obligations.amount_vnd,
       collection_obligations.due_at, collection_obligations.created_at`

type batchRow struct {
	id, contextID, ownerID, status string
	createdAt                      time.Time
	publishedAt                    *time.Time
}

func scanBatch(row pgx.Row) (*batchRow, error) {
	var b batchRow
	var frozenAt, collectingAt, closedAt *time.Time
	err := row.Scan(&b.id, &b.contextID, &b.ownerID, &b.status, &b.createdAt, &frozenAt, &b.publishedAt,
		&collectingAt, &closedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.createdAt, b.publishedAt = b.createdAt.UTC(), utcOptional(b.publishedAt)
	return &b, nil
}

// latestBatchVersion is the batch's version with the highest number (LIMIT
// 1, no lock); "" when it has none.
func (r Repository) latestBatchVersion(ctx context.Context, batchID string) (string, error) {
	var id, batch, createdBy string
	var number int64
	var previous *int64
	var created time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT collection_batch_versions.id, collection_batch_versions.batch_id,
		        collection_batch_versions.version_number, collection_batch_versions.previous_version_number,
		        collection_batch_versions.created_by_id, collection_batch_versions.created_at
		   FROM collection_batch_versions
		  WHERE collection_batch_versions.batch_id = $1::UUID
		  ORDER BY collection_batch_versions.version_number DESC
		  LIMIT $2::INTEGER`, batchID, int32(1)).Scan(&id, &batch, &number, &previous, &createdBy, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

type obligationRow struct {
	id, versionID, senderID, recipientID string
	amountVND                            int64
}

func (r Repository) obligationRows(ctx context.Context, sql string, args ...any) ([]obligationRow, error) {
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []obligationRow
	for rows.Next() {
		var o obligationRow
		var due, created time.Time
		if err := rows.Scan(&o.id, &o.versionID, &o.senderID, &o.recipientID, &o.amountVND, &due, &created); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// LoadBatchForPublish is load_batch_for_publish.
//
// Statements, in Python's order: the batch FOR UPDATE (nil when there is
// none); its latest version (none is Conflict BATCH_HAS_NO_VERSION raised
// from nothing, the batch still locked); that version's obligations ORDER BY
// sender_id, recipient_id; every expense version behind those obligations'
// sources, joined through allocations, unordered and repeated per source.
// The advancer counts as acknowledged when at least one version came back and
// every one of them is acknowledged.
func (r Repository) LoadBatchForPublish(ctx context.Context, batchID string) (*BatchForPublish, error) {
	batch, err := scanBatch(r.Q.QueryRow(ctx,
		`SELECT `+collectionBatchColumns+`
		   FROM collection_batches
		  WHERE collection_batches.id = $1::UUID FOR UPDATE`, batchID))
	if err != nil || batch == nil {
		return nil, err
	}
	versionID, err := r.latestBatchVersion(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if versionID == "" {
		return nil, &Conflict{Code: "BATCH_HAS_NO_VERSION"}
	}
	obligations, err := r.obligationRows(ctx,
		`SELECT `+obligationColumns+`
		   FROM collection_obligations
		  WHERE collection_obligations.batch_version_id = $1::UUID
		  ORDER BY collection_obligations.sender_id, collection_obligations.recipient_id`, versionID)
	if err != nil {
		return nil, err
	}
	rows, err := r.Q.Query(ctx,
		`SELECT `+expenseVersionColumns+`
		   FROM expense_versions
		   JOIN confirmed_allocations ON confirmed_allocations.expense_version_id = expense_versions.id
		   JOIN collection_obligation_sources
		     ON collection_obligation_sources.confirmed_allocation_id = confirmed_allocations.id
		   JOIN collection_obligations ON collection_obligations.id = collection_obligation_sources.obligation_id
		  WHERE collection_obligations.batch_version_id = $1::UUID`, versionID)
	if err != nil {
		return nil, err
	}
	seen, acknowledged := false, true
	for rows.Next() {
		var id, expenseID, recordedBy, paidBy, acknowledgement, scope string
		var number int64
		var previous *int64
		var description *string
		var subtotal, fee, vat, shipping, discount, total int64
		var occurred, created time.Time
		if err := rows.Scan(&id, &expenseID, &number, &previous, &description, &recordedBy, &paidBy,
			&acknowledgement, &scope, &subtotal, &fee, &vat, &shipping, &discount, &total, &occurred, &created); err != nil {
			rows.Close()
			return nil, err
		}
		seen = true
		if acknowledgement != "acknowledged" {
			acknowledged = false
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := &BatchForPublish{ID: batch.id, VersionID: versionID, OwnerID: batch.ownerID, Status: batch.status,
		ContextID: batch.contextID, AdvancerAcknowledged: seen && acknowledged, Obligations: []PublishObligation{}}
	for _, o := range obligations {
		out.Obligations = append(out.Obligations, PublishObligation{ID: o.id, BatchVersionID: versionID,
			SenderID: o.senderID, RecipientID: o.recipientID, AmountVND: o.amountVND})
	}
	return out, nil
}

// SavePublishedBatch is save_published_batch. Of the batch it reads only ID
// and VersionID, as Python does.
//
// Statements and refusals, in Python's order:
//  1. `session.get(CollectionBatch, id)` (labelled, no lock): none is Conflict
//     BATCH_NOT_FOUND raised from nothing;
//  2. `CollectionBatchStatus(status)`: ErrUnknownBatchStatus;
//  3. per link: flush [the batch UPDATE, only for the first link, then the
//     envelope INSERT], flush [the guest link INSERT: active, the draft's
//     expiry, the caller's clock];
//  4. flush [the audit event, then the batch UPDATE when no link carried it].
//
// The UPDATE sets the columns whose value changes, in mapper order (status,
// published_at), and is not issued when neither changes.
func (r Repository) SavePublishedBatch(ctx context.Context, batch BatchForPublish, status string, links []GuestLinkDraft, actorID string, now time.Time) ([]StoredGuestLink, error) {
	model, err := scanBatch(r.Q.QueryRow(ctx,
		`SELECT `+collectionBatchColumnsLabelled+`
		   FROM collection_batches
		  WHERE collection_batches.id = $1::UUID`, batch.ID))
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, &Conflict{Code: "BATCH_NOT_FOUND"}
	}
	if !batchStatuses[status] {
		return nil, ErrUnknownBatchStatus
	}
	published := pythonInstant(now)
	var sets []string
	var args []any
	if model.status != status {
		args = append(args, status)
		sets = append(sets, "status=$"+itoa(len(args)))
	}
	if model.publishedAt == nil || !model.publishedAt.Equal(published) {
		args = append(args, published)
		sets = append(sets, "published_at=$"+itoa(len(args))+"::TIMESTAMP WITH TIME ZONE")
	}
	update := func() error {
		if len(sets) == 0 {
			return nil
		}
		sql := `UPDATE collection_batches SET ` + sets[0]
		for _, s := range sets[1:] {
			sql += ", " + s
		}
		err := r.execUpdate(ctx, sql+` WHERE collection_batches.id = $`+itoa(len(args)+1)+`::UUID`,
			append(args, batch.ID)...)
		sets = nil
		return err
	}

	stored := []StoredGuestLink{}
	for _, draft := range links {
		envelopeID, err := newUUID()
		if err != nil {
			return nil, err
		}
		if err := update(); err != nil {
			return nil, err
		}
		if _, err := r.Q.Exec(ctx, renderInsert("collection_envelopes", envelopeInsert, 1),
			envelopeID, batch.VersionID, draft.SenderID, published); err != nil {
			return nil, err
		}
		linkID, err := newUUID()
		if err != nil {
			return nil, err
		}
		if _, err := r.Q.Exec(ctx, renderInsert("guest_links", guestLinkInsert, 1),
			linkID, envelopeID, draft.TokenDigest, "active", draft.ExpiresAt, published, nil, nil, nil, nil); err != nil {
			return nil, err
		}
		stored = append(stored, StoredGuestLink{ID: linkID, EnvelopeID: envelopeID, SenderID: draft.SenderID})
	}
	if err := r.insertAudit(ctx, auditEvent{actorID: &actorID, eventType: "collection_batch_published",
		aggregateType: "collection_batch", aggregateID: batch.ID, occurredAt: published,
		eventData: map[string]any{"batch_version_id": batch.VersionID, "guest_link_count": len(stored)}}); err != nil {
		return nil, err
	}
	if err := update(); err != nil {
		return nil, err
	}
	return stored, nil
}

// receiptAmounts is `_receipt_amounts`: the obligation's receipt amounts
// ORDER BY confirmed_at, id.
func (r Repository) receiptAmounts(ctx context.Context, obligationID string) ([]int64, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT receipt_confirmations.amount_vnd
		   FROM receipt_confirmations
		  WHERE receipt_confirmations.obligation_id = $1::UUID
		  ORDER BY receipt_confirmations.confirmed_at, receipt_confirmations.id`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var amount int64
		if err := rows.Scan(&amount); err != nil {
			return nil, err
		}
		out = append(out, amount)
	}
	return out, rows.Err()
}

// ListBatchObligations is list_batch_obligations: nil for a batch or a
// version that does not exist.
func (r Repository) ListBatchObligations(ctx context.Context, batchID string) (*BatchBoard, error) {
	return r.batchBoard(ctx, batchID, nil)
}

// dispute is the first objection reason seen for an obligation, as JSON.
type dispute struct {
	reason json.RawMessage
}

// batchBoard is list_batch_obligations with the batch either read here
// (`session.get`, labelled, no lock) or, when loaded is given, already in the
// session's identity map, in which case Python issues no statement for it.
//
// Then: the latest version (nil board when none); its obligations ORDER BY
// sender_id alone (ties in scan order); the guest link ids of the version;
// only when there are any, the wrong_amount objection events on them ORDER BY
// occurred_at, id (the first reason per obligation wins; an event whose
// obligation_id is missing, empty or not text marks nothing); the earliest
// payment report per obligation; then per obligation its receipt amounts and
// the status derived from them.
func (r Repository) batchBoard(ctx context.Context, batchID string, loaded *batchRow) (*BatchBoard, error) {
	batch := loaded
	if batch == nil {
		var err error
		batch, err = scanBatch(r.Q.QueryRow(ctx,
			`SELECT `+collectionBatchColumnsLabelled+`
			   FROM collection_batches
			  WHERE collection_batches.id = $1::UUID`, batchID))
		if err != nil || batch == nil {
			return nil, err
		}
	}
	versionID, err := r.latestBatchVersion(ctx, batchID)
	if err != nil || versionID == "" {
		return nil, err
	}
	obligations, err := r.obligationRows(ctx,
		`SELECT `+obligationColumns+`
		   FROM collection_obligations
		  WHERE collection_obligations.batch_version_id = $1::UUID
		  ORDER BY collection_obligations.sender_id`, versionID)
	if err != nil {
		return nil, err
	}

	rows, err := r.Q.Query(ctx,
		`SELECT guest_links.id
		   FROM guest_links
		   JOIN collection_envelopes ON collection_envelopes.id = guest_links.envelope_id
		  WHERE collection_envelopes.batch_version_id = $1::UUID`, versionID)
	if err != nil {
		return nil, err
	}
	var links []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		links = append(links, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	disputes := map[string]dispute{}
	if len(links) > 0 {
		args := append([]any{"guest_link"}, uuidArgs(links)...)
		args = append(args, "guest_objection.wrong_amount")
		rows, err := r.Q.Query(ctx,
			`SELECT audit_events.event_data
			   FROM audit_events
			  WHERE audit_events.aggregate_type = $1::VARCHAR
			    AND audit_events.aggregate_id IN (`+uuidPlaceholders(2, len(links))+`)
			    AND audit_events.event_type = $`+itoa(len(links)+2)+`::VARCHAR
			  ORDER BY audit_events.occurred_at, audit_events.id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var data []byte
			if err := rows.Scan(&data); err != nil {
				rows.Close()
				return nil, err
			}
			var event map[string]json.RawMessage
			if len(data) == 0 || data[0] != '{' || json.Unmarshal(data, &event) != nil {
				rows.Close()
				return nil, ErrEventDataNotAnObject
			}
			var target string
			if raw, ok := event["obligation_id"]; !ok || json.Unmarshal(raw, &target) != nil || target == "" {
				continue
			}
			if _, first := disputes[target]; !first {
				disputes[target] = dispute{reason: event["reason"]}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	rows, err = r.Q.Query(ctx,
		`SELECT payment_reports.obligation_id, min(payment_reports.reported_at) AS min_1
		   FROM payment_reports
		   JOIN collection_obligations ON collection_obligations.id = payment_reports.obligation_id
		  WHERE collection_obligations.batch_version_id = $1::UUID
		  GROUP BY payment_reports.obligation_id`, versionID)
	if err != nil {
		return nil, err
	}
	claims := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			rows.Close()
			return nil, err
		}
		claims[id] = at.UTC()
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	board := &BatchBoard{ContextID: batch.contextID, Obligations: []BatchObligationRow{}}
	for _, o := range obligations {
		amounts, err := r.receiptAmounts(ctx, o.id)
		if err != nil {
			return nil, err
		}
		status, err := obligationStatus(o.amountVND, amounts)
		if err != nil {
			return nil, err
		}
		row := BatchObligationRow{ObligationID: o.id, SenderID: o.senderID, RecipientID: o.recipientID,
			AmountVND: o.amountVND, Status: status}
		if d, disputed := disputes[o.id]; disputed {
			row.Disputed = true
			if len(d.reason) > 0 && string(d.reason) != "null" {
				var reason string
				if err := json.Unmarshal(d.reason, &reason); err != nil {
					return nil, ErrDisputeReasonNotText
				}
				row.DisputedReason = &reason
			}
		}
		if at, claimed := claims[o.id]; claimed {
			row.PaymentReportedAt = &at
		}
		board.Obligations = append(board.Obligations, row)
	}
	return board, nil
}

// ListContextBatches is list_context_batches: the group's batches ORDER BY
// created_at DESC, id, each folded from its board. The batches stay loaded in
// the session, so each board's `session.get` of its batch issues no
// statement; a batch with no version folds an empty board.
func (r Repository) ListContextBatches(ctx context.Context, contextID string) ([]ContextBatchRow, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+collectionBatchColumns+`
		   FROM collection_batches
		  WHERE collection_batches.context_id = $1::UUID
		  ORDER BY collection_batches.created_at DESC, collection_batches.id`, contextID)
	if err != nil {
		return nil, err
	}
	var batches []*batchRow
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		batches = append(batches, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []ContextBatchRow{}
	for _, b := range batches {
		board, err := r.batchBoard(ctx, b.id, b)
		if err != nil {
			return nil, err
		}
		row := ContextBatchRow{BatchID: b.id, Status: b.status, CreatedAt: b.createdAt, PublishedAt: b.publishedAt,
			TotalVND: new(big.Int)}
		if board != nil {
			for _, o := range board.Obligations {
				row.ObligationCount++
				if o.Status == "confirmed" || o.Status == "over_confirmed" {
					row.ConfirmedCount++
				}
				if o.Disputed {
					row.DisputedCount++
				}
				row.TotalVND.Add(row.TotalVND, big.NewInt(o.AmountVND))
			}
		}
		out = append(out, row)
	}
	return out, nil
}
