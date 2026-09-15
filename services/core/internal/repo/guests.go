package repo

// The guest capability routes (GET /g/{token} and the objection and payment
// report routes beside it): get_guest_envelope, get_payment_report_target,
// save_payment_report and save_guest_objection.
//
// A guest has no account; the token digest is the subject. None of these
// methods maps a PostgreSQL refusal to a Conflict and none opens a savepoint:
// every IntegrityError propagates as the *pgconn.PgError it is, and the only
// Conflict raised here (IDEMPOTENCY_KEY_REUSED) is raised from nothing.
//
// Obligation state is derived, never read from a column: receiver_confirmed
// comes from the receipt_confirmations rows (ledger.ObligationStatus), a
// dispute, an evidence request and the objection quota from the guest_link's
// audit events.

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/domain/capability"
	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
)

// GuestObjectionLimit is app/api/limits.py's OBJECTION_LIMIT.
const GuestObjectionLimit = 3

// GuestReportLimit is app/api/limits.py's REPORT_LIMIT.
const GuestReportLimit = 3

// The guest link states (GuestLinkStatus).
const (
	GuestLinkActive  = "active"
	GuestLinkRevoked = "revoked"
	GuestLinkExpired = "expired"
	GuestLinkRotated = "rotated"
)

// guestNoOccasion is the occasion label of an obligation none of whose source
// expenses has a description.
const guestNoOccasion = "đợt thu này"

// guestManyRecorders names the recorder when the envelope's sources were
// recorded by nobody or by more than one person.
const guestManyRecorders = "Người tạo đợt"

// GuestObligationBlock is one dict of the envelope's "obligations" list, its
// keys in Python's order: obligation_id, occasion_label, amount_vnd,
// recipient_display_name, already_reported, evidence_requested, disputed,
// objections_used, objections_allowed, receiver_confirmed.
type GuestObligationBlock struct {
	ObligationID         string
	OccasionLabel        string
	AmountVND            int64
	RecipientDisplayName string
	AlreadyReported      bool
	EvidenceRequested    bool
	Disputed             bool
	ObjectionsUsed       int64
	ObjectionsAllowed    int64
	ReceiverConfirmed    bool
}

// GuestEnvelope is the raw envelope dict, its keys in Python's order:
// recorded_by_display_name, claimed_person_display_name, link_state,
// obligations, reports_used, reports_allowed, objections_used,
// objections_allowed.
type GuestEnvelope struct {
	RecordedByDisplayName    string
	ClaimedPersonDisplayName string
	LinkState                string
	Obligations              []GuestObligationBlock
	ReportsUsed              int64
	ReportsAllowed           int64
	ObjectionsUsed           int64
	ObjectionsAllowed        int64
}

// GuestEnvelopeRecord is GuestEnvelopeRecord.
type GuestEnvelopeRecord struct {
	LinkID   string
	Envelope GuestEnvelope
}

// PaymentReportTarget is PaymentReportTarget.
type PaymentReportTarget struct {
	LinkID           string
	ObligationID     string
	AmountVND        int64
	ActiveCapability bool
	ReportsUsed      int64
}

// PaymentReportRecord is PaymentReportRecord.
type PaymentReportRecord struct {
	ID                string
	ObligationID      string
	AmountVND         int64
	ReceiptAmountsVND []int64
}

// PaymentReportInput is save_payment_report's keyword arguments.
type PaymentReportInput struct {
	Target         PaymentReportTarget
	IdempotencyKey string
	Now            time.Time
}

// GuestObjectionInput is save_guest_objection's keyword arguments. The
// obligation id, when given, is a canonical lowercase UUID: Python writes
// str(uuid.UUID), and the caller parses the form value the way the route does.
type GuestObjectionInput struct {
	TokenDigest  []byte
	Kind         string
	ObligationID *string
	Reason       *string
	Now          time.Time
}

const guestLinkColumns = `guest_links.id, guest_links.envelope_id, guest_links.token_digest, guest_links.status,
       guest_links.expires_at, guest_links.created_at, guest_links.capability_exposed_at,
       guest_links.first_opened_at, guest_links.revoked_at, guest_links.rotated_from_id`

// guestLinkRow is the GuestLink columns a guest method reads.
type guestLinkRow struct {
	id, envelopeID, status string
	expiresAt              time.Time
	firstOpenedAt          *time.Time
	revokedAt              *time.Time
}

// guestLinkTargets are the Scan destinations of guestLinkColumns, in order.
func (l *guestLinkRow) targets() []any {
	var digest []byte
	var created time.Time
	var exposed *time.Time
	var rotatedFrom *string
	return []any{&l.id, &l.envelopeID, &digest, &l.status, &l.expiresAt, &created, &exposed, &l.firstOpenedAt,
		&l.revokedAt, &rotatedFrom}
}

// GetGuestEnvelope is get_guest_envelope: nil for a digest no link carries.
//
// Statements, in Python's order:
//  1. the link joined to its envelope, batch version and batch, FOR UPDATE:
//     one row lock on each of the four;
//  2. the autoflush of the link's pending changes, issued by the next
//     statement: first_opened_at at the caller's clock when it is NULL, and
//     status "expired" when it is active and now >= expires_at. The UPDATE
//     sets the changed columns in table order (status, first_opened_at) and
//     is not issued when neither changed;
//  3. the envelope's obligations (same version, same sender) ORDER BY
//     recipient_id, then capability_scope over them: none is
//     CapabilityScopeError NO_OBLIGATIONS, the UPDATE already sent;
//  4. the link's evidence_request events, then its wrong_amount events, then
//     its not_me and wrong_amount events. Each is read whole before the next
//     statement; event data that is not a JSON object is AttributeError
//     (ErrEventDataNotAnObject) at the first such row;
//  5. the recipients' names (one statement, one parameter per distinct id);
//  6. per obligation: its source descriptions and recorders, its receipt
//     amounts (unordered; the status sums them), and the count of this link's
//     reports on it;
//  7. the count of the link's reports, the count of its not_me and
//     wrong_amount events, and the names of the recorders and the sender.
//
// An event marks an obligation only when its obligation_id is a non-empty
// JSON string: Python compares str(value) with str(obligation.id), which no
// other truthy JSON value can equal, and skips a falsy one.
func (r Repository) GetGuestEnvelope(ctx context.Context, tokenDigest []byte, now time.Time) (*GuestEnvelopeRecord, error) {
	now = pythonInstant(now)
	var link guestLinkRow
	var envelopeID, versionID, senderID string
	var envelopeCreated time.Time
	var versionBatch, versionCreatedBy string
	var versionNumber int64
	var previousVersion *int64
	var versionCreated time.Time
	var batch batchRow
	var frozenAt, collectingAt, closedAt *time.Time
	dest := append(link.targets(), &envelopeID, &versionID, &senderID, &envelopeCreated,
		new(string), &versionBatch, &versionNumber, &previousVersion, &versionCreatedBy, &versionCreated,
		&batch.id, &batch.contextID, &batch.ownerID, &batch.status, &batch.createdAt, &frozenAt, &batch.publishedAt,
		&collectingAt, &closedAt)
	err := r.Q.QueryRow(ctx,
		`SELECT `+guestLinkColumns+`,
		        collection_envelopes.id AS id_1, collection_envelopes.batch_version_id, collection_envelopes.sender_id,
		        collection_envelopes.created_at AS created_at_1,
		        collection_batch_versions.id AS id_2, collection_batch_versions.batch_id,
		        collection_batch_versions.version_number, collection_batch_versions.previous_version_number,
		        collection_batch_versions.created_by_id, collection_batch_versions.created_at AS created_at_2,
		        collection_batches.id AS id_3, collection_batches.context_id, collection_batches.owner_id,
		        collection_batches.status AS status_1, collection_batches.created_at AS created_at_3,
		        collection_batches.frozen_at, collection_batches.published_at, collection_batches.collecting_at,
		        collection_batches.closed_at
		   FROM guest_links
		   JOIN collection_envelopes ON collection_envelopes.id = guest_links.envelope_id
		   JOIN collection_batch_versions ON collection_batch_versions.id = collection_envelopes.batch_version_id
		   JOIN collection_batches ON collection_batches.id = collection_batch_versions.batch_id
		  WHERE guest_links.token_digest = $1 FOR UPDATE`, tokenDigest).Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	state := link.status
	var sets []string
	var args []any
	if state == GuestLinkActive && !now.Before(link.expiresAt) {
		state = GuestLinkExpired
		args = append(args, GuestLinkExpired)
		sets = append(sets, "status=$"+itoa(len(args)))
	}
	if link.firstOpenedAt == nil {
		args = append(args, now)
		sets = append(sets, "first_opened_at=$"+itoa(len(args))+"::TIMESTAMP WITH TIME ZONE")
	}
	if len(sets) > 0 {
		if err := r.execUpdate(ctx, `UPDATE guest_links SET `+strings.Join(sets, ", ")+
			` WHERE guest_links.id = $`+itoa(len(args)+1)+`::UUID`, append(args, link.id)...); err != nil {
			return nil, err
		}
	}

	obligations, err := r.obligationRows(ctx,
		`SELECT `+obligationColumns+`
		   FROM collection_obligations
		  WHERE collection_obligations.batch_version_id = $1::UUID AND collection_obligations.sender_id = $2::UUID
		  ORDER BY collection_obligations.recipient_id`, versionID, senderID)
	if err != nil {
		return nil, err
	}
	scoped := make([]capability.Obligation, len(obligations))
	for i, o := range obligations {
		scoped[i] = capability.Obligation{ObligationID: o.id, BatchVersionID: o.versionID, SenderID: o.senderID}
	}
	if _, err := capability.ScopeOf(capability.Envelope{BatchVersionID: versionID, SenderID: senderID}, scoped); err != nil {
		return nil, err
	}

	evidenceAsked := map[string]bool{}
	if err := r.guestLinkEvents(ctx, link.id, []string{"guest_objection.evidence_request"}, func(target string) {
		evidenceAsked[target] = true
	}); err != nil {
		return nil, err
	}
	disputed := map[string]bool{}
	if err := r.guestLinkEvents(ctx, link.id, []string{"guest_objection.wrong_amount"}, func(target string) {
		disputed[target] = true
	}); err != nil {
		return nil, err
	}
	objectionCounts := map[string]int64{}
	if err := r.guestLinkEvents(ctx, link.id, guestQuotaEvents, func(target string) {
		objectionCounts[target]++
	}); err != nil {
		return nil, err
	}

	recipients := make([]string, len(obligations))
	for i, o := range obligations {
		recipients[i] = o.recipientID
	}
	recipientNames, err := r.displayNames(ctx, recipients)
	if err != nil {
		return nil, err
	}

	blocks := []GuestObligationBlock{}
	recordedBy := map[string]bool{}
	var recorders []string
	for _, o := range obligations {
		labels, err := r.guestObligationSources(ctx, o.id, func(recorder string) {
			if !recordedBy[recorder] {
				recordedBy[recorder] = true
				recorders = append(recorders, recorder)
			}
		})
		if err != nil {
			return nil, err
		}
		occasion := guestNoOccasion
		if len(labels) > 0 {
			occasion = strings.Join(labels, ", ")
		}
		receipts, err := r.unorderedReceiptAmounts(ctx, o.id)
		if err != nil {
			return nil, err
		}
		status, err := ledger.ObligationStatus(money.VND(o.amountVND), receipts)
		if err != nil {
			return nil, err
		}
		var reported int64
		if err := r.Q.QueryRow(ctx,
			`SELECT count(payment_reports.id) AS count_1
			   FROM payment_reports
			  WHERE payment_reports.guest_link_id = $1::UUID AND payment_reports.obligation_id = $2::UUID`,
			link.id, o.id).Scan(&reported); err != nil {
			return nil, err
		}
		blocks = append(blocks, GuestObligationBlock{
			ObligationID:         o.id,
			OccasionLabel:        occasion,
			AmountVND:            o.amountVND,
			RecipientDisplayName: recipientNames[o.recipientID],
			AlreadyReported:      reported > 0,
			EvidenceRequested:    evidenceAsked[o.id],
			Disputed:             disputed[o.id],
			ObjectionsUsed:       objectionCounts[o.id],
			ObjectionsAllowed:    GuestObjectionLimit,
			ReceiverConfirmed:    status == "confirmed" || status == "over_confirmed",
		})
	}

	out := &GuestEnvelopeRecord{LinkID: link.id, Envelope: GuestEnvelope{LinkState: state, Obligations: blocks,
		ReportsAllowed: GuestReportLimit, ObjectionsAllowed: GuestObjectionLimit}}
	if err := r.Q.QueryRow(ctx,
		`SELECT count(payment_reports.id) AS count_1
		   FROM payment_reports
		  WHERE payment_reports.guest_link_id = $1::UUID`, link.id).Scan(&out.Envelope.ReportsUsed); err != nil {
		return nil, err
	}
	if err := r.Q.QueryRow(ctx,
		`SELECT count(audit_events.id) AS count_1
		   FROM audit_events
		  WHERE audit_events.aggregate_type = $1::VARCHAR AND audit_events.aggregate_id = $2::UUID
		    AND audit_events.event_type IN ($3::VARCHAR, $4::VARCHAR)`,
		"guest_link", link.id, guestQuotaEvents[0], guestQuotaEvents[1]).Scan(&out.Envelope.ObjectionsUsed); err != nil {
		return nil, err
	}
	names, err := r.displayNames(ctx, append(append([]string{}, recorders...), senderID))
	if err != nil {
		return nil, err
	}
	out.Envelope.RecordedByDisplayName = guestManyRecorders
	if len(recorders) == 1 {
		out.Envelope.RecordedByDisplayName = names[recorders[0]]
	}
	out.Envelope.ClaimedPersonDisplayName = names[senderID]
	return out, nil
}

// guestQuotaEvents are the objection kinds that spend the quota, as the IN
// list renders them.
var guestQuotaEvents = []string{"guest_objection.not_me", "guest_objection.wrong_amount"}

// guestLinkEvents reads the event_data of the link's guest_link audit events
// of the given types (one type as "=", several as an IN list), all of them,
// then hands each target an obligation can match to mark.
func (r Repository) guestLinkEvents(ctx context.Context, linkID string, eventTypes []string, mark func(string)) error {
	sql := `SELECT audit_events.event_data
	          FROM audit_events
	         WHERE audit_events.aggregate_type = $1::VARCHAR AND audit_events.aggregate_id = $2::UUID
	           AND audit_events.event_type `
	args := []any{"guest_link", linkID}
	if len(eventTypes) == 1 {
		sql += `= $3::VARCHAR`
	} else {
		sql += `IN (` + varcharPlaceholders(3, len(eventTypes)) + `)`
	}
	for _, t := range eventTypes {
		args = append(args, t)
	}
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	var events [][]byte
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			rows.Close()
			return err
		}
		events = append(events, data)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, data := range events {
		target, ok, err := guestEventTarget(data)
		if err != nil {
			return err
		}
		if ok {
			mark(target)
		}
	}
	return nil
}

// guestEventTarget is `row.get("obligation_id")` of one event: an error when
// the data is not a JSON object, the target when it is a non-empty string.
func guestEventTarget(data []byte) (string, bool, error) {
	var event map[string]json.RawMessage
	if len(data) == 0 || data[0] != '{' || json.Unmarshal(data, &event) != nil {
		return "", false, ErrEventDataNotAnObject
	}
	raw, present := event["obligation_id"]
	if !present {
		return "", false, nil
	}
	var target string
	if json.Unmarshal(raw, &target) != nil || target == "" {
		return "", false, nil
	}
	return target, true, nil
}

func varcharPlaceholders(first, count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = "$" + itoa(first+i) + "::VARCHAR"
	}
	return strings.Join(parts, ", ")
}

// guestObligationSources reads the description and recorder of every expense
// version behind the obligation's sources (unordered, one row per source),
// hands each recorder to record, and returns the distinct non-empty
// descriptions sorted by code point, as Python's sorted() of a set of str.
func (r Repository) guestObligationSources(ctx context.Context, obligationID string, record func(string)) ([]string, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT expense_versions.description, expense_versions.recorded_by_id
		   FROM expense_versions
		   JOIN confirmed_allocations ON confirmed_allocations.expense_version_id = expense_versions.id
		   JOIN collection_obligation_sources
		     ON collection_obligation_sources.confirmed_allocation_id = confirmed_allocations.id
		  WHERE collection_obligation_sources.obligation_id = $1::UUID`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	labels := []string{}
	for rows.Next() {
		var description *string
		var recorder string
		if err := rows.Scan(&description, &recorder); err != nil {
			return nil, err
		}
		record(recorder)
		if description != nil && *description != "" && !seen[*description] {
			seen[*description] = true
			labels = append(labels, *description)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Byte order of UTF-8 is code point order.
	sort.Strings(labels)
	return labels, nil
}

// unorderedReceiptAmounts is the obligation's receipt amounts with no ORDER
// BY, as get_guest_envelope reads them: only their sum is used.
func (r Repository) unorderedReceiptAmounts(ctx context.Context, obligationID string) ([]money.VND, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT receipt_confirmations.amount_vnd
		   FROM receipt_confirmations
		  WHERE receipt_confirmations.obligation_id = $1::UUID`, obligationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []money.VND{}
	for rows.Next() {
		var amount int64
		if err := rows.Scan(&amount); err != nil {
			return nil, err
		}
		out = append(out, money.VND(amount))
	}
	return out, rows.Err()
}

// GetPaymentReportTarget is get_payment_report_target: nil for a digest no
// link carries, and nil for an obligation outside the link's envelope.
//
// Statements, in Python's order: the link joined to its envelope, FOR UPDATE
// (a row lock on both); the obligation by id, version and sender, unlocked;
// only when there is one, the count of the link's reports. Nothing is
// written: unlike get_guest_envelope this read neither records the first open
// nor flips an expired link. The capability is active when the stored status
// is active and now < expires_at.
func (r Repository) GetPaymentReportTarget(ctx context.Context, tokenDigest []byte, obligationID string, now time.Time) (*PaymentReportTarget, error) {
	now = pythonInstant(now)
	var link guestLinkRow
	var envelopeID, versionID, senderID string
	var envelopeCreated time.Time
	dest := append(link.targets(), &envelopeID, &versionID, &senderID, &envelopeCreated)
	err := r.Q.QueryRow(ctx,
		`SELECT `+guestLinkColumns+`,
		        collection_envelopes.id AS id_1, collection_envelopes.batch_version_id, collection_envelopes.sender_id,
		        collection_envelopes.created_at AS created_at_1
		   FROM guest_links
		   JOIN collection_envelopes ON collection_envelopes.id = guest_links.envelope_id
		  WHERE guest_links.token_digest = $1 FOR UPDATE`, tokenDigest).Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	obligations, err := r.obligationRows(ctx,
		`SELECT `+obligationColumns+`
		   FROM collection_obligations
		  WHERE collection_obligations.id = $1::UUID AND collection_obligations.batch_version_id = $2::UUID
		    AND collection_obligations.sender_id = $3::UUID`, obligationID, versionID, senderID)
	if err != nil || len(obligations) == 0 {
		return nil, err
	}
	o := obligations[0]
	out := &PaymentReportTarget{LinkID: link.id, ObligationID: o.id, AmountVND: o.amountVND,
		ActiveCapability: link.status == GuestLinkActive && now.Before(link.expiresAt)}
	if err := r.Q.QueryRow(ctx,
		`SELECT count(payment_reports.id) AS count_1
		   FROM payment_reports
		  WHERE payment_reports.guest_link_id = $1::UUID`, link.id).Scan(&out.ReportsUsed); err != nil {
		return nil, err
	}
	return out, nil
}

// SavePaymentReport is save_payment_report.
//
// Statements and refusals, in Python's order:
//  1. the report with this idempotency key, unlocked. When there is one: a
//     different obligation, a different or NULL guest link, or a different
//     amount is Conflict IDEMPOTENCY_KEY_REUSED raised from nothing;
//     otherwise the stored report is answered with the obligation's receipt
//     amounts, nothing written;
//  2. flush: the report INSERT (no reporter, the target's amount, the caller's
//     clock); a check or foreign key refusal, or a unique violation of a key
//     written by a concurrent request, is returned as is;
//  3. the payment_reported audit event, flushed by the next statement;
//  4. the obligation's receipt amounts ORDER BY confirmed_at, id.
func (r Repository) SavePaymentReport(ctx context.Context, in PaymentReportInput) (PaymentReportRecord, error) {
	var existingID, existingObligation, key string
	var existingLink, reporter *string
	var amount int64
	var reportedAt time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT payment_reports.id, payment_reports.obligation_id, payment_reports.guest_link_id,
		        payment_reports.reported_by_id, payment_reports.amount_vnd, payment_reports.idempotency_key,
		        payment_reports.reported_at
		   FROM payment_reports
		  WHERE payment_reports.idempotency_key = $1::UUID`, in.IdempotencyKey).
		Scan(&existingID, &existingObligation, &existingLink, &reporter, &amount, &key, &reportedAt)
	switch {
	case err == nil:
		if existingObligation != in.Target.ObligationID || existingLink == nil || *existingLink != in.Target.LinkID ||
			amount != in.Target.AmountVND {
			return PaymentReportRecord{}, &Conflict{Code: "IDEMPOTENCY_KEY_REUSED"}
		}
		amounts, err := r.receiptAmounts(ctx, existingObligation)
		if err != nil {
			return PaymentReportRecord{}, err
		}
		return PaymentReportRecord{ID: existingID, ObligationID: existingObligation, AmountVND: amount,
			ReceiptAmountsVND: amounts}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return PaymentReportRecord{}, err
	}

	id, err := newUUID()
	if err != nil {
		return PaymentReportRecord{}, err
	}
	now := pythonInstant(in.Now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO payment_reports (id, obligation_id, guest_link_id, reported_by_id, amount_vnd, idempotency_key,
		                             reported_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::UUID, $5::BIGINT, $6::UUID, $7::TIMESTAMP WITH TIME ZONE)`,
		id, in.Target.ObligationID, in.Target.LinkID, nil, in.Target.AmountVND, in.IdempotencyKey, now); err != nil {
		return PaymentReportRecord{}, err
	}
	requestID := in.IdempotencyKey
	if err := r.insertAudit(ctx, auditEvent{eventType: "payment_reported", aggregateType: "collection_obligation",
		aggregateID: in.Target.ObligationID, requestID: &requestID, occurredAt: now,
		eventData: map[string]any{"payment_report_id": id}}); err != nil {
		return PaymentReportRecord{}, err
	}
	amounts, err := r.receiptAmounts(ctx, in.Target.ObligationID)
	if err != nil {
		return PaymentReportRecord{}, err
	}
	return PaymentReportRecord{ID: id, ObligationID: in.Target.ObligationID, AmountVND: in.Target.AmountVND,
		ReceiptAmountsVND: amounts}, nil
}

// SaveGuestObjection is save_guest_objection followed by the flush the
// route's commit performs: the Python method only adds and assigns, and
// nothing else in the request reads the session after it.
//
// Statements, in Python's order: the link by digest, unlocked (nothing more
// for a digest no link carries); then the flush, mapper by mapper: the audit
// event guest_objection.<kind> (no actor; kind, obligation_id and reason in
// its data), then for kind "not_me" only the link UPDATE setting status
// revoked and revoked_at at the caller's clock, each only when it changes
// and no UPDATE when neither does. Nothing checks the link's state, its
// expiry or whether the obligation belongs to it: the service does.
func (r Repository) SaveGuestObjection(ctx context.Context, in GuestObjectionInput) error {
	var link guestLinkRow
	err := r.Q.QueryRow(ctx,
		`SELECT `+guestLinkColumns+`
		   FROM guest_links
		  WHERE guest_links.token_digest = $1`, in.TokenDigest).Scan(link.targets()...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	now := pythonInstant(in.Now)
	data := map[string]any{"kind": in.Kind, "obligation_id": nil, "reason": nil}
	if in.ObligationID != nil {
		data["obligation_id"] = *in.ObligationID
	}
	if in.Reason != nil {
		data["reason"] = *in.Reason
	}
	if err := r.insertAudit(ctx, auditEvent{eventType: "guest_objection." + in.Kind, aggregateType: "guest_link",
		aggregateID: link.id, occurredAt: now, eventData: data}); err != nil {
		return err
	}
	if in.Kind != "not_me" {
		return nil
	}
	var sets []string
	var args []any
	if link.status != GuestLinkRevoked {
		args = append(args, GuestLinkRevoked)
		sets = append(sets, "status=$"+itoa(len(args)))
	}
	if link.revokedAt == nil || !link.revokedAt.Equal(now) {
		args = append(args, now)
		sets = append(sets, "revoked_at=$"+itoa(len(args))+"::TIMESTAMP WITH TIME ZONE")
	}
	if len(sets) == 0 {
		return nil
	}
	return r.execUpdate(ctx, `UPDATE guest_links SET `+strings.Join(sets, ", ")+
		` WHERE guest_links.id = $`+itoa(len(args)+1)+`::UUID`, append(args, link.id)...)
}
