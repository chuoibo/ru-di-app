// Package dieuchinh carries out ADR-0056: amending one expense of a collection
// batch that was already frozen or published, with every affected person's
// yes, as a new batch version. The arithmetic is domain/dieuchinh; this
// package reads and writes the rows, in one transaction per step.
package dieuchinh

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"slices"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/allocator"
	plan "mobile/services/core/internal/domain/dieuchinh"
	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/repo"
)

// Lifetime is how long an amendment waits for its answers.
const Lifetime = 7 * 24 * time.Hour

// Refusal is an expected refusal, answered as {"code","detail"}.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

func refuse(status int, code, detail string) error {
	return &Refusal{Status: status, Code: code, Detail: detail}
}

// Who names the person a request speaks for, read inside the transaction
// that acts, so a session revoked or an account erased meanwhile cannot act.
type Who func(ctx context.Context, tx pgx.Tx) (string, error)

// As is a Who already known: a test's, or the store's own.
func As(personID string) Who {
	return func(context.Context, pgx.Tx) (string, error) { return personID, nil }
}

// Store reads and writes amendments.
type Store struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

type allocationRow struct {
	ID            string
	ParticipantID string
	AmountVND     int64
}

type sourceVersion struct {
	ID           string
	ExpenseID    string
	PaidByID     string
	RecordedByID string
	Description  *string
	Allocations  []allocationRow
}

type obligationRow struct {
	ID, SenderID, RecipientID string
	AmountVND                 int64
	DueAt                     time.Time
}

type batchState struct {
	ID, ContextID, OwnerID, Status string
	VersionID                      string
	VersionNumber                  int
	Obligations                    []obligationRow
	Sources                        []sourceVersion
}

func (b *batchState) planSources() []plan.Source {
	out := make([]plan.Source, len(b.Sources))
	for i, s := range b.Sources {
		out[i] = toPlan(s)
	}
	return out
}

func toPlan(s sourceVersion) plan.Source {
	allocations := make([]ledger.Allocation, len(s.Allocations))
	for i, a := range s.Allocations {
		allocations[i] = ledger.Allocation{ParticipantID: a.ParticipantID, AmountVND: money.VND(a.AmountVND)}
	}
	return plan.Source{ExpenseVersionID: s.ID, PaidByID: s.PaidByID, Allocations: allocations}
}

func (b *batchState) edges() []plan.Edge {
	out := make([]plan.Edge, len(b.Obligations))
	for i, o := range b.Obligations {
		out[i] = plan.Edge{SenderID: o.SenderID, RecipientID: o.RecipientID, AmountVND: big.NewInt(o.AmountVND)}
	}
	return out
}

// loadBatch locks the batch row and reads its latest version: obligations and
// the expense versions behind them, each with every allocation.
func loadBatch(ctx context.Context, tx pgx.Tx, batchID string) (*batchState, error) {
	b := &batchState{ID: batchID}
	err := tx.QueryRow(ctx, `SELECT context_id::text, owner_id::text, status::text FROM collection_batches WHERE id=$1::uuid FOR UPDATE`, batchID).
		Scan(&b.ContextID, &b.OwnerID, &b.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refuse(404, "batch_not_found", "Batch does not exist")
	}
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, `SELECT id::text, version_number FROM collection_batch_versions WHERE batch_id=$1::uuid ORDER BY version_number DESC LIMIT 1`, batchID).
		Scan(&b.VersionID, &b.VersionNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refuse(409, "batch_has_no_version", "The batch was never frozen")
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, sender_id::text, recipient_id::text, amount_vnd, due_at FROM collection_obligations WHERE batch_version_id=$1::uuid ORDER BY id`, b.VersionID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var o obligationRow
		if err := rows.Scan(&o.ID, &o.SenderID, &o.RecipientID, &o.AmountVND, &o.DueAt); err != nil {
			rows.Close()
			return nil, err
		}
		b.Obligations = append(b.Obligations, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT v.id::text, v.expense_id::text, v.paid_by_id::text, v.recorded_by_id::text, v.description, a.id::text, a.participant_id::text, a.amount_vnd
		  FROM expense_versions v JOIN confirmed_allocations a ON a.expense_version_id = v.id
		 WHERE v.id IN (SELECT DISTINCT ca.expense_version_id
		                  FROM collection_obligation_sources s
		                  JOIN collection_obligations o ON o.id = s.obligation_id
		                  JOIN confirmed_allocations ca ON ca.id = s.confirmed_allocation_id
		                 WHERE o.batch_version_id = $1::uuid)
		 ORDER BY v.id, a.participant_id`, b.VersionID)
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for rows.Next() {
		var versionID, expenseID, paidBy, recordedBy string
		var description *string
		var a allocationRow
		if err := rows.Scan(&versionID, &expenseID, &paidBy, &recordedBy, &description, &a.ID, &a.ParticipantID, &a.AmountVND); err != nil {
			rows.Close()
			return nil, err
		}
		i, seen := index[versionID]
		if !seen {
			i = len(b.Sources)
			index[versionID] = i
			b.Sources = append(b.Sources, sourceVersion{ID: versionID, ExpenseID: expenseID, PaidByID: paidBy, RecordedByID: recordedBy, Description: description})
		}
		b.Sources[i].Allocations = append(b.Sources[i].Allocations, a)
	}
	rows.Close()
	return b, rows.Err()
}

// appUser is whether the person ever signed in: a session, live or not.
func appUser(ctx context.Context, tx pgx.Tx, personID string) (bool, error) {
	var yes bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_sessions WHERE person_id=$1::uuid)`, personID).Scan(&yes)
	return yes, err
}

func activeMember(ctx context.Context, tx pgx.Tx, contextID, personID string) (bool, error) {
	var yes bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships WHERE context_id=$1::uuid AND person_id=$2::uuid AND state='active' AND left_at IS NULL)`, contextID, personID).Scan(&yes)
	return yes, err
}

// ProposeInput is POST /batches/{batch_id}/amendments.
type ProposeInput struct {
	BatchID, ActorID, ExpenseID, Reason string
	// Allocations are the corrected allocations of the expense, integer đồng,
	// one per participant; their sum is the corrected total.
	Allocations map[string]int64
}

// ReviewLink is one guest's link to answer an amendment: shown once, to the
// proposer, who shares it as they shared the batch's links.
type ReviewLink struct {
	SenderID string `json:"sender_id"`
	Path     string `json:"path"`
}

// Proposed is what a proposal answers.
type Proposed struct {
	AmendmentID string       `json:"amendment_id"`
	Status      string       `json:"status"`
	Links       []ReviewLink `json:"review_links"`
}

// Propose records an amendment with its lines and the people who must
// accept, rotates the affected guests' links to review links, and applies at
// once when nobody but the proposer is affected.
func (s Store) Propose(ctx context.Context, in ProposeInput) (Proposed, error) {
	return s.ProposeAs(ctx, in, As(in.ActorID))
}

// ProposeAs is Propose for whoever who names; in.ActorID is ignored.
func (s Store) ProposeAs(ctx context.Context, in ProposeInput, who Who) (Proposed, error) {
	reason := in.Reason
	if l := len([]rune(reason)); l < 1 || l > 500 {
		return Proposed{}, refuse(422, "amendment_reason_invalid", "Say in 1 to 500 characters why the expense changes")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Proposed{}, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if in.ActorID, err = who(ctx, tx); err != nil {
		return Proposed{}, err
	}
	b, err := loadBatch(ctx, tx, in.BatchID)
	if err != nil {
		return Proposed{}, err
	}
	member, err := activeMember(ctx, tx, b.ContextID, in.ActorID)
	if err != nil {
		return Proposed{}, err
	}
	if !member {
		// A stranger learns nothing, not even that the batch exists.
		return Proposed{}, refuse(404, "batch_not_found", "Batch does not exist")
	}
	switch b.Status {
	case "frozen", "published", "collecting":
	default:
		return Proposed{}, refuse(409, "batch_not_amendable", "Only a frozen or published batch is amended; an open one takes corrections directly")
	}
	now := s.now()
	if err := expireDue(ctx, tx, b.ID, now); err != nil {
		return Proposed{}, err
	}
	var replaced *sourceVersion
	for i := range b.Sources {
		if b.Sources[i].ExpenseID == in.ExpenseID {
			replaced = &b.Sources[i]
		}
	}
	if replaced == nil {
		return Proposed{}, refuse(409, "expense_not_in_batch", "This expense is not in the batch")
	}
	if in.ActorID != b.OwnerID && in.ActorID != replaced.PaidByID && in.ActorID != replaced.RecordedByID {
		// ADR-0056 §2.2: the batch owner, or whoever recorded or paid the
		// expense, proposes; every affected person decides (spec §9.1).
		return Proposed{}, refuse(403, "amendment_not_allowed", "Only the batch owner or the expense's recorder or payer proposes an amendment")
	}
	participants := make([]string, 0, len(in.Allocations))
	var total int64
	for person, amount := range in.Allocations {
		if amount < 0 || amount > int64(allocator.MaxAmountVND) {
			return Proposed{}, refuse(422, "allocation_amount_invalid", "Each allocation is whole đồng between 0 and the amount ceiling")
		}
		if total > int64(allocator.MaxAmountVND)-amount {
			return Proposed{}, refuse(422, "allocation_total_invalid", "The corrected total passes the amount ceiling")
		}
		total += amount
		participants = append(participants, person)
	}
	if total <= 0 {
		return Proposed{}, refuse(422, "allocation_total_invalid", "The corrected expense must cost something")
	}
	sort.Strings(participants)
	for _, person := range participants {
		ok, err := activeMember(ctx, tx, b.ContextID, person)
		if err != nil {
			return Proposed{}, err
		}
		if !ok {
			return Proposed{}, refuse(422, "participant_not_member", "Every participant must be a current member of the group")
		}
	}
	replacement := plan.Source{ExpenseVersionID: "amended", PaidByID: replaced.PaidByID}
	for _, person := range participants {
		replacement.Allocations = append(replacement.Allocations, ledger.Allocation{ParticipantID: person, AmountVND: money.VND(in.Allocations[person])})
	}
	next, err := plan.Amend(b.planSources(), replaced.ID, replacement)
	if errors.Is(err, plan.ErrNoObligations) {
		return Proposed{}, refuse(409, "amendment_owes_nothing", "After this correction nobody owes anything: cancel the batch instead")
	}
	if err != nil {
		var ledgerRefused *ledger.LedgerError
		if errors.As(err, &ledgerRefused) {
			return Proposed{}, refuse(422, ledgerRefused.Code, "The corrected allocations cannot form obligations")
		}
		return Proposed{}, err
	}
	changes := plan.Diff(b.edges(), next)
	if len(changes) == 0 {
		return Proposed{}, refuse(409, "amendment_changes_nothing", "Nobody's amount changes")
	}
	affected := plan.Affected(changes)

	id, err := newID()
	if err != nil {
		return Proposed{}, err
	}
	confirmation, err := json.Marshal(map[string]any{"allocations": in.Allocations, "total_amount_vnd": total})
	if err != nil {
		return Proposed{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO collection_amendments(id,batch_id,base_batch_version_id,expense_id,replaced_expense_version_id,proposed_by_id,reason,confirmation,status,created_at,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'proposed',$9,$10)`,
		id, b.ID, b.VersionID, in.ExpenseID, replaced.ID, in.ActorID, reason, string(confirmation), now, now.Add(Lifetime)); err != nil {
		if isUnique(err) {
			return Proposed{}, refuse(409, "amendment_already_open", "This batch already has an amendment waiting for answers")
		}
		return Proposed{}, err
	}
	for _, c := range changes {
		if _, err := tx.Exec(ctx, `INSERT INTO collection_amendment_lines VALUES($1,$2,$3,$4,$5)`, id, c.SenderID, c.RecipientID, c.OldVND.Int64(), c.NewVND.Int64()); err != nil {
			return Proposed{}, err
		}
	}
	for _, p := range affected {
		if _, err := tx.Exec(ctx, `INSERT INTO collection_amendment_parties VALUES($1,$2)`, id, p); err != nil {
			return Proposed{}, err
		}
	}
	if slices.Contains(affected, in.ActorID) {
		if _, err := tx.Exec(ctx, `INSERT INTO collection_amendment_decisions VALUES($1,$2,true,'proposer',$3)`, id, in.ActorID, now); err != nil {
			return Proposed{}, err
		}
	}
	if err := audit(ctx, tx, in.ActorID, "collection_amendment.proposed", id, now, map[string]any{"batch_id": b.ID, "expense_id": in.ExpenseID, "reason": reason}); err != nil {
		return Proposed{}, err
	}
	links, err := rotateAffectedLinks(ctx, tx, id, b, next, affected, now)
	if err != nil {
		return Proposed{}, err
	}
	status, err := settle(ctx, tx, id, s.now)
	if err != nil {
		return Proposed{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Proposed{}, err
	}
	if status != "proposed" {
		links = []ReviewLink{}
	}
	return Proposed{AmendmentID: id, Status: status, Links: links}, nil
}

// rotateAffectedLinks gives each affected sender of a published batch the
// review link that answers in place of their live one: the live link is
// rotated, and a sender the amendment adds, who never had a link, gets one
// too. A sender whose link was revoked or ran out gets none; a member answers
// in the app either way.
func rotateAffectedLinks(ctx context.Context, tx pgx.Tx, amendmentID string, b *batchState, next []plan.Edge, affected []string, now time.Time) ([]ReviewLink, error) {
	links := []ReviewLink{}
	var latest *time.Time
	if err := tx.QueryRow(ctx, `SELECT max(gl.expires_at) FROM guest_links gl JOIN collection_envelopes e ON e.id = gl.envelope_id WHERE e.batch_version_id=$1::uuid`, b.VersionID).Scan(&latest); err != nil {
		return nil, err
	}
	if latest == nil {
		// Never published: nobody holds a link, and the members answer in the app.
		return links, nil
	}
	senders := map[string]bool{}
	for _, o := range b.Obligations {
		senders[o.SenderID] = true
	}
	for _, e := range next {
		senders[e.SenderID] = true
	}
	for _, person := range affected {
		if !senders[person] {
			continue
		}
		app, err := appUser(ctx, tx, person)
		if err != nil {
			return nil, err
		}
		var enveloped bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM collection_envelopes WHERE batch_version_id=$1::uuid AND sender_id=$2::uuid)`, b.VersionID, person).Scan(&enveloped); err != nil {
			return nil, err
		}
		var oldLink *string
		var oldExpiry *time.Time
		err = tx.QueryRow(ctx, `SELECT gl.id::text, gl.expires_at FROM guest_links gl JOIN collection_envelopes e ON e.id = gl.envelope_id
			 WHERE e.batch_version_id=$1::uuid AND e.sender_id=$2::uuid AND gl.status='active' AND gl.expires_at > $3
			 ORDER BY gl.expires_at DESC LIMIT 1 FOR UPDATE OF gl`, b.VersionID, person, now).Scan(&oldLink, &oldExpiry)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if app {
			// Someone with an account answers in the app, never through a
			// link: the proposer is who shares review links, and a link in
			// the proposer's hands must not be able to agree for a member.
			// Their guest link stops showing amounts about to change.
			if oldLink != nil {
				if _, err := tx.Exec(ctx, `UPDATE guest_links SET status='rotated', revoked_at=$2 WHERE id=$1::uuid`, *oldLink, now); err != nil {
					return nil, err
				}
			}
			continue
		}
		expires := now.Add(Lifetime)
		switch {
		case oldLink != nil:
			if _, err := tx.Exec(ctx, `UPDATE guest_links SET status='rotated', revoked_at=$2 WHERE id=$1::uuid`, *oldLink, now); err != nil {
				return nil, err
			}
			expires = *oldExpiry
		case enveloped:
			continue
		case latest.After(expires):
			expires = *latest
		}
		token, err := mintToken()
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO collection_amendment_links(token_digest,amendment_id,sender_id,old_link_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6)`,
			auth.TokenDigest(token), amendmentID, person, oldLink, now, expires); err != nil {
			return nil, err
		}
		links = append(links, ReviewLink{SenderID: person, Path: "/g/" + token + "/dieu-chinh"})
	}
	return links, nil
}

// Decide records one affected person's answer and settles the amendment.
func (s Store) Decide(ctx context.Context, amendmentID, personID string, accept bool, via string) (string, error) {
	return s.DecideAs(ctx, amendmentID, As(personID), accept, via)
}

// DecideAs is Decide for whoever who names.
func (s Store) DecideAs(ctx context.Context, amendmentID string, who Who, accept bool, via string) (string, error) {
	if via != "app" && via != "guest_link" {
		// "proposer" is written by Propose alone.
		return "", errors.New("dieuchinh: unknown decision channel " + via)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	personID, err := who(ctx, tx)
	if err != nil {
		return "", err
	}
	// An expiry found on the way is kept even when the answer is refused for
	// it: the late answerer and everyone after see it ended. Its own
	// transaction, after the caller is known.
	if err := s.expireFor(ctx, amendmentID); err != nil {
		return "", err
	}
	status, err := s.decideIn(ctx, tx, amendmentID, personID, accept, via)
	if err != nil {
		return "", err
	}
	return status, tx.Commit(ctx)
}

func (s Store) expireFor(ctx context.Context, amendmentID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	var batchID string
	var due bool
	err = tx.QueryRow(ctx, `SELECT batch_id::text, status='proposed' AND expires_at <= $2 FROM collection_amendments WHERE id=$1::uuid`, amendmentID, s.now()).Scan(&batchID, &due)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !due) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := loadBatch(ctx, tx, batchID); err != nil {
		return err
	}
	if err := expireDue(ctx, tx, batchID, s.now()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) decideIn(ctx context.Context, tx pgx.Tx, amendmentID, personID string, accept bool, via string) (string, error) {
	var batchID, status string
	var expires time.Time
	err := tx.QueryRow(ctx, `SELECT batch_id::text, status, expires_at FROM collection_amendments WHERE id=$1::uuid`, amendmentID).Scan(&batchID, &status, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", refuse(404, "amendment_not_found", "No such amendment")
	}
	if err != nil {
		return "", err
	}
	// Lock order: the batch, then the amendment, as Propose and Apply take it.
	if _, err := loadBatch(ctx, tx, batchID); err != nil {
		return "", err
	}
	now := s.now()
	if err := expireDue(ctx, tx, batchID, now); err != nil {
		return "", err
	}
	if err := tx.QueryRow(ctx, `SELECT status FROM collection_amendments WHERE id=$1::uuid FOR UPDATE`, amendmentID).Scan(&status); err != nil {
		return "", err
	}
	if status != "proposed" {
		return "", refuse(409, "amendment_closed", "This amendment was already "+status)
	}
	if via == "guest_link" {
		app, err := appUser(ctx, tx, personID)
		if err != nil {
			return "", err
		}
		if app {
			return "", refuse(403, "amendment_answer_in_app", "Answer this amendment in the app")
		}
	}
	var party bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM collection_amendment_parties WHERE amendment_id=$1::uuid AND person_id=$2::uuid)`, amendmentID, personID).Scan(&party); err != nil {
		return "", err
	}
	if !party {
		return "", refuse(403, "amendment_not_affected", "Only a person whose amount changes answers this amendment")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO collection_amendment_decisions VALUES($1,$2,$3,$4,$5)`, amendmentID, personID, accept, via, now); err != nil {
		if isUnique(err) {
			return "", refuse(409, "amendment_already_answered", "You already answered this amendment")
		}
		return "", err
	}
	if err := audit(ctx, tx, personID, "collection_amendment.decided", amendmentID, now, map[string]any{"accept": accept, "via": via}); err != nil {
		return "", err
	}
	return settle(ctx, tx, amendmentID, s.now)
}

// settle applies an amendment every affected person accepted, rejects one
// anybody refused, and leaves it proposed otherwise.
func settle(ctx context.Context, tx pgx.Tx, amendmentID string, clock func() time.Time) (string, error) {
	rows, err := tx.Query(ctx, `SELECT p.person_id::text, d.accept FROM collection_amendment_parties p
		LEFT JOIN collection_amendment_decisions d ON d.amendment_id = p.amendment_id AND d.person_id = p.person_id
		WHERE p.amendment_id=$1::uuid ORDER BY p.person_id`, amendmentID)
	if err != nil {
		return "", err
	}
	var affected []string
	var decisions []plan.Decision
	for rows.Next() {
		var person string
		var accept *bool
		if err := rows.Scan(&person, &accept); err != nil {
			rows.Close()
			return "", err
		}
		affected = append(affected, person)
		if accept != nil {
			decisions = append(decisions, plan.Decision{PersonID: person, Accept: *accept})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	now := clock().UTC()
	switch plan.Outcome(affected, decisions) {
	case "accepted":
		return "applied", apply(ctx, tx, amendmentID, now)
	case "rejected":
		return "rejected", resolveUnapplied(ctx, tx, amendmentID, "rejected", now)
	}
	return "proposed", nil
}

// expireDue ends every amendment of the batch past its expiry.
func expireDue(ctx context.Context, tx pgx.Tx, batchID string, now time.Time) error {
	var due []string
	rows, err := tx.Query(ctx, `SELECT id::text FROM collection_amendments WHERE batch_id=$1::uuid AND status='proposed' AND expires_at <= $2`, batchID, now)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		due = append(due, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range due {
		if err := resolveUnapplied(ctx, tx, id, "expired", now); err != nil {
			return err
		}
	}
	return nil
}

// reinstate points a review token back at the envelope its rotated link
// opened, with that link's expiry: the guest's one URL shows what it showed
// before. A token of an added sender had no envelope and answers nothing.
const reinstate = `INSERT INTO guest_links(id,envelope_id,token_digest,status,expires_at,created_at,rotated_from_id)
	SELECT gen_random_uuid(), old.envelope_id, l.token_digest, 'active', l.expires_at, l.created_at, old.id
	  FROM collection_amendment_links l JOIN guest_links old ON old.id = l.old_link_id`

// resolveUnapplied ends an amendment without applying it: each guest's review
// token answers again for the envelope it was rotated from.
func resolveUnapplied(ctx context.Context, tx pgx.Tx, amendmentID, status string, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE collection_amendments SET status=$2, resolved_at=$3 WHERE id=$1::uuid`, amendmentID, status, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, reinstate+` WHERE l.amendment_id=$1::uuid`, amendmentID); err != nil {
		return err
	}
	return audit(ctx, tx, nil, "collection_amendment."+status, amendmentID, now, map[string]any{})
}

// apply writes the accepted amendment: the expense's next version, the batch's
// next version with its obligations, sources and envelopes, the successions,
// and the links.
func apply(ctx context.Context, tx pgx.Tx, amendmentID string, now time.Time) error {
	var batchID, baseVersion, expenseID, replacedID, proposer string
	var confirmation []byte
	if err := tx.QueryRow(ctx, `SELECT batch_id::text, base_batch_version_id::text, expense_id::text, replaced_expense_version_id::text, proposed_by_id::text, confirmation
		  FROM collection_amendments WHERE id=$1::uuid FOR UPDATE`, amendmentID).
		Scan(&batchID, &baseVersion, &expenseID, &replacedID, &proposer, &confirmation); err != nil {
		return err
	}
	b, err := loadBatch(ctx, tx, batchID)
	if err != nil {
		return err
	}
	if b.VersionID != baseVersion {
		return refuse(409, "amendment_stale", "The batch changed since this amendment was proposed")
	}
	var stored struct {
		Allocations map[string]int64 `json:"allocations"`
		Total       int64            `json:"total_amount_vnd"`
	}
	if err := json.Unmarshal(confirmation, &stored); err != nil {
		return err
	}
	var old struct {
		description             *string
		recordedBy, paidBy, ack string
		occurredAt              time.Time
	}
	if err := tx.QueryRow(ctx, `SELECT description, recorded_by_id::text, paid_by_id::text, payer_acknowledgement::text, occurred_at FROM expense_versions WHERE id=$1::uuid`, replacedID).
		Scan(&old.description, &old.recordedBy, &old.paidBy, &old.ack, &old.occurredAt); err != nil {
		return err
	}
	var payerAccepted bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM collection_amendment_decisions WHERE amendment_id=$1::uuid AND person_id=$2::uuid AND accept)`, amendmentID, old.paidBy).Scan(&payerAccepted); err != nil {
		return err
	}
	ack := old.ack
	if payerAccepted {
		ack = "acknowledged"
	}
	allocations := make([]repo.ParticipantAmount, 0, len(stored.Allocations))
	for person, amount := range stored.Allocations {
		allocations = append(allocations, repo.ParticipantAmount{ParticipantID: person, AmountVND: amount})
	}
	sort.Slice(allocations, func(i, j int) bool { return allocations[i].ParticipantID < allocations[j].ParticipantID })
	record, err := (repo.Repository{Q: tx}).SaveExpenseConfirmation(ctx, repo.ExpenseConfirmation{
		ExpenseID: expenseID,
		Proposal: repo.ExpenseProposal{Description: old.description, RecordedByID: old.recordedBy, PaidByID: old.paidBy,
			VerificationScope: "totals_only", OccurredAt: old.occurredAt},
		AllocatorWarnings:    []string{"AMENDED_" + amendmentID},
		Rollups:              repo.ExpenseRollups{SubtotalVND: stored.Total, TotalVND: stored.Total},
		Allocations:          allocations,
		ConfirmedByID:        proposer,
		PayerAcknowledgement: ack,
		Now:                  now,
	})
	if err != nil {
		return err
	}
	newVersionID := record.ExpenseVersionID
	replacement := sourceVersion{ID: newVersionID, ExpenseID: expenseID, PaidByID: old.paidBy}
	rows, err := tx.Query(ctx, `SELECT id::text, participant_id::text, amount_vnd FROM confirmed_allocations WHERE expense_version_id=$1::uuid ORDER BY participant_id`, newVersionID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var a allocationRow
		if err := rows.Scan(&a.ID, &a.ParticipantID, &a.AmountVND); err != nil {
			rows.Close()
			return err
		}
		replacement.Allocations = append(replacement.Allocations, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	sources := make([]sourceVersion, 0, len(b.Sources))
	for _, s := range b.Sources {
		if s.ID == replacedID {
			sources = append(sources, replacement)
			continue
		}
		sources = append(sources, s)
	}
	planSources := make([]plan.Source, len(sources))
	for i, s := range sources {
		planSources[i] = toPlan(s)
	}
	edges, err := plan.Obligations(planSources)
	if err != nil {
		return err
	}

	versionID, err := newID()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO collection_batch_versions(id,batch_id,version_number,previous_version_number,created_by_id,created_at) VALUES($1,$2,$3,$4,$5,$6)`,
		versionID, batchID, b.VersionNumber+1, b.VersionNumber, proposer, now); err != nil {
		return err
	}
	due := now.Add(24 * time.Hour)
	if len(b.Obligations) > 0 {
		due = b.Obligations[0].DueAt
	}
	oldByPair := map[[2]string]obligationRow{}
	for _, o := range b.Obligations {
		oldByPair[[2]string{o.SenderID, o.RecipientID}] = o
	}
	senders := map[string]bool{}
	for _, e := range edges {
		obligationID, err := newID()
		if err != nil {
			return err
		}
		if !e.AmountVND.IsInt64() {
			return refuse(409, "amendment_amount_overflow", "An amended obligation passes the amount ceiling")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO collection_obligations(id,batch_version_id,sender_id,recipient_id,amount_vnd,due_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			obligationID, versionID, e.SenderID, e.RecipientID, e.AmountVND.Int64(), due, now); err != nil {
			return err
		}
		for _, s := range sources {
			if s.PaidByID != e.RecipientID {
				continue
			}
			for _, a := range s.Allocations {
				if a.ParticipantID == e.SenderID && a.AmountVND > 0 {
					if _, err := tx.Exec(ctx, `INSERT INTO collection_obligation_sources(obligation_id,confirmed_allocation_id,amount_vnd,created_at) VALUES($1,$2,$3,$4)`,
						obligationID, a.ID, a.AmountVND, now); err != nil {
						return err
					}
				}
			}
		}
		if prior, ok := oldByPair[[2]string{e.SenderID, e.RecipientID}]; ok {
			if _, err := tx.Exec(ctx, `INSERT INTO collection_obligation_successions VALUES($1,$2,$3)`, prior.ID, obligationID, amendmentID); err != nil {
				return err
			}
		}
		senders[e.SenderID] = true
	}
	ordered := make([]string, 0, len(senders))
	for s := range senders {
		ordered = append(ordered, s)
	}
	sort.Strings(ordered)
	affected := map[string]bool{}
	rows, err = tx.Query(ctx, `SELECT person_id::text FROM collection_amendment_parties WHERE amendment_id=$1::uuid`, amendmentID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		affected[p] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	enveloped := map[string]bool{}
	rows, err = tx.Query(ctx, `SELECT sender_id::text FROM collection_envelopes WHERE batch_version_id=$1::uuid
		UNION SELECT sender_id::text FROM collection_amendment_links WHERE amendment_id=$2::uuid`, b.VersionID, amendmentID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		enveloped[p] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	newBySender := map[string][]plan.Edge{}
	for _, e := range edges {
		newBySender[e.SenderID] = append(newBySender[e.SenderID], e)
	}
	oldCount := map[string]int{}
	for _, o := range b.Obligations {
		oldCount[o.SenderID]++
	}
	// A sender the amendment frees of every obligation still gets an
	// envelope, an empty one: their URL then says they owe nothing in this
	// batch, rather than showing the replaced obligations.
	for sender := range enveloped {
		if affected[sender] && !senders[sender] {
			ordered = append(ordered, sender)
		}
	}
	sort.Strings(ordered)
	for _, sender := range ordered {
		if !enveloped[sender] {
			continue
		}
		envelopeID, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO collection_envelopes(id,batch_version_id,sender_id,created_at) VALUES($1,$2,$3,$4)`, envelopeID, versionID, sender, now); err != nil {
			return err
		}
		if !affected[sender] {
			// Same obligations, same amounts: the sender's live link follows
			// to the new envelope, and what it shows does not change. Checked
			// pair by pair, not trusted to the diff.
			if oldCount[sender] != len(newBySender[sender]) {
				return errors.New("dieuchinh: an unaffected sender's obligations changed")
			}
			for _, e := range newBySender[sender] {
				if prior, ok := oldByPair[[2]string{e.SenderID, e.RecipientID}]; !ok || big.NewInt(prior.AmountVND).Cmp(e.AmountVND) != 0 {
					return errors.New("dieuchinh: an unaffected sender's obligations changed")
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE guest_links SET envelope_id=$1::uuid
				 WHERE status='active' AND envelope_id IN (SELECT id FROM collection_envelopes WHERE batch_version_id=$2::uuid AND sender_id=$3::uuid)`,
				envelopeID, b.VersionID, sender); err != nil {
				return err
			}
			continue
		}
		// The review token the guest already holds becomes their link to
		// the envelope they agreed to.
		if _, err := tx.Exec(ctx, `INSERT INTO guest_links(id,envelope_id,token_digest,status,expires_at,created_at,rotated_from_id)
			SELECT gen_random_uuid(), $1::uuid, l.token_digest, 'active', l.expires_at, l.created_at, l.old_link_id
			  FROM collection_amendment_links l WHERE l.amendment_id=$2::uuid AND l.sender_id=$3::uuid`, envelopeID, amendmentID, sender); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE collection_amendments SET status='applied', resolved_at=$2, applied_batch_version_id=$3 WHERE id=$1::uuid`, amendmentID, now, versionID); err != nil {
		return err
	}
	return audit(ctx, tx, nil, "collection_amendment.applied", amendmentID, now, map[string]any{"batch_version_id": versionID, "expense_version_id": newVersionID})
}

func audit(ctx context.Context, tx pgx.Tx, actor any, eventType, amendmentID string, now time.Time, data map[string]any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if s, ok := actor.(string); ok {
		actor = s
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(id,actor_id,event_type,aggregate_type,aggregate_id,request_id,event_data,occurred_at)
		VALUES(gen_random_uuid(),$1,$2,'collection_amendment',$3,NULL,$4,$5)`, actor, eventType, amendmentID, string(encoded), now)
	return err
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[v>>4], hex[v&0x0f])
	}
	return string(out), nil
}

// mintToken is the batch publish token: 32 random bytes, base64url, no padding.
func mintToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
