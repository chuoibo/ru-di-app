package repo

// The W8 slice of SqlAlchemyApiRepository behind the notebook routes
// (routes/pair_notebooks.py): the notebook row and its lock, cycles and their
// participants, consent proposals and grants, the couple slot and the two
// shared constraints. get_pair_notebook is pair_notebook.go's, from W1;
// get_context, is_member and list_members are the pilot wave's.
//
// Three SQLAlchemy behaviours every method here reproduces:
//
//   - One flush per method. Each Python method adds or assigns and then calls
//     session.flush() itself, so its writes leave in the method that makes
//     them, never later. None of the pair models declares a relationship(),
//     and no method here has two pending rows of different mappers in one
//     flush, so the only ordering inside a flush is among rows of one table.
//   - Changed columns only. An UPDATE lists the columns whose new value is not
//     equal to the loaded one (Python ==: instants by instant), in table
//     column order, and there is no UPDATE when nothing changed. A row whose
//     key is given in full goes out as a true executemany, one statement per
//     row, rows in insertion order for an INSERT and in primary key order for
//     an UPDATE.
//   - Identity. `session.get` loads by primary key with every column labelled
//     table_column. The session holds a clean object only weakly: every method
//     here drops the objects it loaded before it returns, so the next method
//     loads again and issues its SELECT. None of these methods depends on an
//     object an earlier call left in the identity map.
//
// No method maps a PostgreSQL refusal to a Conflict and none opens a
// savepoint. The unique indexes (uq_pair_notebooks_context,
// uq_pair_cycles_open_per_notebook, uq_pair_consents_proposal) and every CHECK
// and foreign key surface as the *pgconn.PgError they are, and the request
// that hit one ends in a 500. The two Conflicts raised here are raised from
// nothing: `couple_slot_taken` (set_couple_member) and, in pair_papers.go,
// `paper_already_agreed` and `paper_outing_exists`.
//
// The two deferred constraint triggers (active_couple_needs_two_consents,
// pair_paper_outing_link_valid) fire at COMMIT, after every statement below.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// column is one column of a SET list or a WHERE-by-key clause: its name, the
// bind cast the psycopg dialect renders on its parameter ("" for none), and
// the value bound.
type column struct {
	name, cast string
	value      any
}

// updateRow is the flush of assignments to one loaded row: nothing when no
// column changed, otherwise one UPDATE of the changed columns keyed by the
// row's primary key, which must match exactly one row (see ErrStaleUpdate).
func (r Repository) updateRow(ctx context.Context, table string, sets, keys []column) error {
	if len(sets) == 0 {
		return nil
	}
	var b strings.Builder
	args := make([]any, 0, len(sets)+len(keys))
	b.WriteString("UPDATE " + table + " SET ")
	for i, c := range sets {
		if i > 0 {
			b.WriteString(", ")
		}
		args = append(args, c.value)
		b.WriteString(c.name + "=$" + strconv.Itoa(len(args)) + c.cast)
	}
	b.WriteString(" WHERE ")
	b.WriteString(keyClause(table, keys, &args))
	return r.execUpdate(ctx, b.String(), args...)
}

// deleteRow is the flush of `session.delete(row)` for one loaded row: DELETE
// by primary key. SQLAlchemy only warns when such a DELETE matches no row;
// that needs the row to vanish between the read and the flush, which one
// transaction cannot stage, so the refusal here is never reached.
func (r Repository) deleteRow(ctx context.Context, table string, keys []column) error {
	args := make([]any, 0, len(keys))
	sql := "DELETE FROM " + table + " WHERE " + keyClause(table, keys, &args)
	tag, err := r.Q.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleUpdate
	}
	return nil
}

// keyClause renders `table.k1 = $n::T AND table.k2 = ...`, appending the key
// values to args.
func keyClause(table string, keys []column, args *[]any) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		*args = append(*args, k.value)
		parts[i] = table + "." + k.name + " = $" + strconv.Itoa(len(*args)) + k.cast
	}
	return strings.Join(parts, " AND ")
}

// optionalText binds a Python `str | None`.
func optionalText(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// sameInstant is Python == between an optional stored datetime and an
// optional new one.
func sameInstant(stored, next *time.Time) bool {
	if stored == nil || next == nil {
		return stored == nil && next == nil
	}
	return stored.Equal(*next)
}

// ---------------------------------------------------------------------------
// The notebook and its cycles
// ---------------------------------------------------------------------------

// CreatePairNotebook is create_pair_notebook: one INSERT of every column
// (context_kind is the Python default 'pair', created_at the caller's clock),
// and the record of a notebook with no cycle, built without a statement. A
// second notebook for the context fails on uq_pair_notebooks_context, a group
// or missing context on fk_pair_notebooks_context.
func (r Repository) CreatePairNotebook(ctx context.Context, contextID string, now time.Time) (PairNotebook, error) {
	id, err := newUUID()
	if err != nil {
		return PairNotebook{}, err
	}
	if _, err := r.Q.Exec(ctx, renderInsert("pair_notebooks", []insertColumn{{"id", "::UUID"}, {"context_id", "::UUID"},
		{"context_kind", "::VARCHAR"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		id, contextID, "pair", pythonInstant(now)); err != nil {
		return PairNotebook{}, err
	}
	return PairNotebook{ID: id, ContextID: contextID, Participants: []string{}, Consents: []PairConsent{},
		Proposals: []PairProposal{}, Constraints: []PairConstraint{}}, nil
}

// LockPairNotebook is lock_pair_notebook: GetPairNotebook with FOR UPDATE on
// the notebook statement only. The cycle, participant, proposal, constraint
// and consent reads after it take no lock.
func (r Repository) LockPairNotebook(ctx context.Context, contextID string) (*PairNotebook, error) {
	return r.readPairNotebook(ctx, contextID, " FOR UPDATE")
}

// OpenPairCycle is open_pair_cycle: the cycle INSERT (state 'pending',
// opened_at and closed_at as explicit NULLs, created_at the caller's clock),
// flushed alone; then one INSERT per participant, in the order given, as the
// executemany of the second flush (none for no participants). A second live
// cycle fails on uq_pair_cycles_open_per_notebook before any participant is
// written. Returns the new cycle's id.
func (r Repository) OpenPairCycle(ctx context.Context, notebookID string, participants []string, termsVersion int64, now time.Time) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx, renderInsert("pair_notebook_cycles", []insertColumn{{"id", "::UUID"},
		{"notebook_id", "::UUID"}, {"state", "::VARCHAR"}, {"terms_version", "::INTEGER"},
		{"opened_at", "::TIMESTAMP WITH TIME ZONE"}, {"closed_at", "::TIMESTAMP WITH TIME ZONE"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		id, notebookID, "pending", sqlInteger(termsVersion), nil, nil, created); err != nil {
		return "", err
	}
	rows := make([][]any, len(participants))
	for i, person := range participants {
		rows[i] = []any{id, person, created}
	}
	if err := r.insertEach(ctx, "pair_cycle_participants", []insertColumn{{"cycle_id", "::UUID"},
		{"person_id", "::UUID"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"}}, rows); err != nil {
		return "", err
	}
	return id, nil
}

// pairCycleRow is the PairNotebookCycle columns activate and close compare.
type pairCycleRow struct {
	state              string
	openedAt, closedAt *time.Time
}

// getPairCycle is `session.get(PairNotebookCycle, id)`: nil when there is none.
func (r Repository) getPairCycle(ctx context.Context, cycleID string) (*pairCycleRow, error) {
	var row pairCycleRow
	err := r.Q.QueryRow(ctx,
		`SELECT pair_notebook_cycles.id AS pair_notebook_cycles_id,
		        pair_notebook_cycles.notebook_id AS pair_notebook_cycles_notebook_id,
		        pair_notebook_cycles.state AS pair_notebook_cycles_state,
		        pair_notebook_cycles.terms_version AS pair_notebook_cycles_terms_version,
		        pair_notebook_cycles.opened_at AS pair_notebook_cycles_opened_at,
		        pair_notebook_cycles.closed_at AS pair_notebook_cycles_closed_at,
		        pair_notebook_cycles.created_at AS pair_notebook_cycles_created_at
		   FROM pair_notebook_cycles
		  WHERE pair_notebook_cycles.id = $1::UUID`, cycleID).
		Scan(new(string), new(string), &row.state, new(int64), &row.openedAt, &row.closedAt, new(time.Time))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

var cycleKey = func(cycleID string) []column { return []column{{"id", "::UUID", cycleID}} }

// ActivatePairCycle is activate_pair_cycle: the cycle by id; nothing more for
// a missing or closed cycle; otherwise state 'active' and opened_at kept when
// set, now when not, each written only when it changes.
func (r Repository) ActivatePairCycle(ctx context.Context, cycleID string, now time.Time) error {
	cycle, err := r.getPairCycle(ctx, cycleID)
	if err != nil || cycle == nil || cycle.state == "closed" {
		return err
	}
	var sets []column
	if cycle.state != "active" {
		sets = append(sets, column{"state", "::VARCHAR", "active"})
	}
	if cycle.openedAt == nil {
		sets = append(sets, column{"opened_at", "::TIMESTAMP WITH TIME ZONE", pythonInstant(now)})
	}
	return r.updateRow(ctx, "pair_notebook_cycles", sets, cycleKey(cycleID))
}

// ClosePairCycle is close_pair_cycle: the cycle by id; nothing more for a
// missing or closed cycle; otherwise state 'closed' and closed_at now, in one
// UPDATE (a live cycle's closed_at is NULL by cycle_closed_matches_timestamp,
// so both always change).
func (r Repository) ClosePairCycle(ctx context.Context, cycleID string, now time.Time) error {
	cycle, err := r.getPairCycle(ctx, cycleID)
	if err != nil || cycle == nil || cycle.state == "closed" {
		return err
	}
	closed := pythonInstant(now)
	sets := []column{{"state", "::VARCHAR", "closed"}}
	if !sameInstant(cycle.closedAt, &closed) {
		sets = append(sets, column{"closed_at", "::TIMESTAMP WITH TIME ZONE", closed})
	}
	return r.updateRow(ctx, "pair_notebook_cycles", sets, cycleKey(cycleID))
}

// ---------------------------------------------------------------------------
// Proposals and grants
// ---------------------------------------------------------------------------

// ConsentProposalInput is create_consent_proposal's keyword arguments.
type ConsentProposalInput struct {
	CycleID      string
	Purpose      string
	ProposedByID string
	TermsVersion int64
	ExpiresAt    time.Time
	Now          time.Time
}

// CreateConsentProposal is create_consent_proposal: one INSERT (completed_at
// an explicit NULL), and the record of what was written, built without a
// statement. consent_purpose_known and consent_expires_after_created refuse
// in PostgreSQL.
func (r Repository) CreateConsentProposal(ctx context.Context, in ConsentProposalInput) (PairProposal, error) {
	id, err := newUUID()
	if err != nil {
		return PairProposal{}, err
	}
	created, expires := pythonInstant(in.Now), pythonInstant(in.ExpiresAt)
	if _, err := r.Q.Exec(ctx, renderInsert("pair_consent_proposals", []insertColumn{{"id", "::UUID"},
		{"cycle_id", "::UUID"}, {"purpose", "::VARCHAR"}, {"proposed_by_id", "::UUID"}, {"terms_version", "::INTEGER"},
		{"completed_at", "::TIMESTAMP WITH TIME ZONE"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"},
		{"expires_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		id, in.CycleID, in.Purpose, in.ProposedByID, sqlInteger(in.TermsVersion), nil, created, expires); err != nil {
		return PairProposal{}, err
	}
	return PairProposal{ID: id, CycleID: in.CycleID, Purpose: in.Purpose, ProposedByID: in.ProposedByID,
		TermsVersion: int(in.TermsVersion), CreatedAt: created.UTC(), ExpiresAt: expires.UTC()}, nil
}

// GetConsentProposal is get_consent_proposal: `session.get` by id, nil when
// there is none.
func (r Repository) GetConsentProposal(ctx context.Context, proposalID string) (*PairProposal, error) {
	var p PairProposal
	err := r.Q.QueryRow(ctx,
		`SELECT pair_consent_proposals.id AS pair_consent_proposals_id,
		        pair_consent_proposals.cycle_id AS pair_consent_proposals_cycle_id,
		        pair_consent_proposals.purpose AS pair_consent_proposals_purpose,
		        pair_consent_proposals.proposed_by_id AS pair_consent_proposals_proposed_by_id,
		        pair_consent_proposals.terms_version AS pair_consent_proposals_terms_version,
		        pair_consent_proposals.completed_at AS pair_consent_proposals_completed_at,
		        pair_consent_proposals.created_at AS pair_consent_proposals_created_at,
		        pair_consent_proposals.expires_at AS pair_consent_proposals_expires_at
		   FROM pair_consent_proposals
		  WHERE pair_consent_proposals.id = $1::UUID`, proposalID).
		Scan(&p.ID, &p.CycleID, &p.Purpose, &p.ProposedByID, &p.TermsVersion, &p.CompletedAt, &p.CreatedAt, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CompletedAt, p.CreatedAt, p.ExpiresAt = utcOptional(p.CompletedAt), p.CreatedAt.UTC(), p.ExpiresAt.UTC()
	return &p, nil
}

// GrantConsent is grant_consent, idempotent by (proposal, person).
//
// Statements, in Python's order:
//  1. the person's consent row for the proposal (`scalars(...).first()`: no
//     LIMIT, uq_pair_consents_proposal keeps it to one);
//  2. an existing row whose granted_at is set is left alone, even when it was
//     revoked: a revoked grant is NOT granted again (Python returns before
//     touching it); an existing row never granted gets granted_at now and
//     revoked_at NULL, only the columns that change;
//  3. no row: an INSERT (revoked_at an explicit NULL, created_at now).
func (r Repository) GrantConsent(ctx context.Context, proposalID, personID string, now time.Time) error {
	var id string
	var grantedAt, revokedAt *time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT pair_consents.id, pair_consents.proposal_id, pair_consents.person_id, pair_consents.granted_at,
		        pair_consents.revoked_at, pair_consents.created_at
		   FROM pair_consents
		  WHERE pair_consents.proposal_id = $1::UUID AND pair_consents.person_id = $2::UUID`, proposalID, personID).
		Scan(&id, new(string), new(string), &grantedAt, &revokedAt, new(time.Time))
	granted := pythonInstant(now)
	switch {
	case err == nil:
		if grantedAt != nil {
			return nil
		}
		sets := []column{{"granted_at", "::TIMESTAMP WITH TIME ZONE", granted}}
		if revokedAt != nil {
			sets = append(sets, column{"revoked_at", "::TIMESTAMP WITH TIME ZONE", nil})
		}
		return r.updateRow(ctx, "pair_consents", sets, []column{{"id", "::UUID", id}})
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	newID, err := newUUID()
	if err != nil {
		return err
	}
	_, err = r.Q.Exec(ctx, renderInsert("pair_consents", []insertColumn{{"id", "::UUID"}, {"proposal_id", "::UUID"},
		{"person_id", "::UUID"}, {"granted_at", "::TIMESTAMP WITH TIME ZONE"}, {"revoked_at", "::TIMESTAMP WITH TIME ZONE"},
		{"created_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
		newID, proposalID, personID, granted, nil, granted)
	return err
}

// CompleteConsentProposal is complete_consent_proposal: the proposal by id;
// nothing more when it is missing or already completed; otherwise one UPDATE
// of completed_at.
func (r Repository) CompleteConsentProposal(ctx context.Context, proposalID string, now time.Time) error {
	proposal, err := r.GetConsentProposal(ctx, proposalID)
	if err != nil || proposal == nil || proposal.CompletedAt != nil {
		return err
	}
	return r.updateRow(ctx, "pair_consent_proposals",
		[]column{{"completed_at", "::TIMESTAMP WITH TIME ZONE", pythonInstant(now)}},
		[]column{{"id", "::UUID", proposalID}})
}

// RevokeConsents is revoke_consents: every live grant (granted, not revoked)
// this person gave for this purpose in this cycle, whatever proposal it
// answered and whether or not that proposal lapsed, locked FOR UPDATE OF
// pair_consents (the proposals joined are not locked); then revoked_at now on
// each, as one executemany in primary key order. Returns how many.
func (r Repository) RevokeConsents(ctx context.Context, cycleID, purpose, personID string, now time.Time) (int, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT pair_consents.id, pair_consents.proposal_id, pair_consents.person_id, pair_consents.granted_at,
		        pair_consents.revoked_at, pair_consents.created_at
		   FROM pair_consents JOIN pair_consent_proposals ON pair_consent_proposals.id = pair_consents.proposal_id
		  WHERE pair_consent_proposals.cycle_id = $1::UUID AND pair_consent_proposals.purpose = $2::VARCHAR
		    AND pair_consents.person_id = $3::UUID AND pair_consents.granted_at IS NOT NULL
		    AND pair_consents.revoked_at IS NULL FOR UPDATE OF pair_consents`, cycleID, purpose, personID)
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var id string
		err := row.Scan(&id, new(string), new(string), new(*time.Time), new(*time.Time), new(time.Time))
		return id, err
	})
	if err != nil {
		return 0, err
	}
	sorted := append([]string{}, ids...)
	sortStrings(sorted)
	revoked := pythonInstant(now)
	for _, id := range sorted {
		if err := r.updateRow(ctx, "pair_consents", []column{{"revoked_at", "::TIMESTAMP WITH TIME ZONE", revoked}},
			[]column{{"id", "::UUID", id}}); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// ---------------------------------------------------------------------------
// The couple slot
// ---------------------------------------------------------------------------

// coupleCycle is `session.get(ActiveCoupleMember, person_id)`: the cycle the
// person's couple row names, nil when there is none.
func (r Repository) coupleCycle(ctx context.Context, personID string) (*string, error) {
	var cycle string
	err := r.Q.QueryRow(ctx,
		`SELECT active_couple_members.person_id AS active_couple_members_person_id,
		        active_couple_members.cycle_id AS active_couple_members_cycle_id,
		        active_couple_members.since AS active_couple_members_since
		   FROM active_couple_members
		  WHERE active_couple_members.person_id = $1::UUID`, personID).
		Scan(new(string), &cycle, new(time.Time))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cycle, nil
}

// SetCoupleMember is set_couple_member: the person's couple row; the same
// cycle is a no-op, another cycle is Conflict couple_slot_taken raised from
// nothing, no row is an INSERT (since = now).
func (r Repository) SetCoupleMember(ctx context.Context, personID, cycleID string, now time.Time) error {
	existing, err := r.coupleCycle(ctx, personID)
	if err != nil {
		return err
	}
	if existing != nil {
		if *existing == cycleID {
			return nil
		}
		return &Conflict{Code: "couple_slot_taken"}
	}
	_, err = r.Q.Exec(ctx, renderInsert("active_couple_members", []insertColumn{{"person_id", "::UUID"},
		{"cycle_id", "::UUID"}, {"since", "::TIMESTAMP WITH TIME ZONE"}}, 1), personID, cycleID, pythonInstant(now))
	return err
}

// ClearCoupleMember is clear_couple_member: the person's couple row, whatever
// cycle it names, and its DELETE when there is one.
func (r Repository) ClearCoupleMember(ctx context.Context, personID string) error {
	existing, err := r.coupleCycle(ctx, personID)
	if err != nil || existing == nil {
		return err
	}
	return r.deleteRow(ctx, "active_couple_members", []column{{"person_id", "::UUID", personID}})
}

// ---------------------------------------------------------------------------
// Shared constraints
// ---------------------------------------------------------------------------

// PairConstraintInput is set_pair_constraint's keyword arguments.
type PairConstraintInput struct {
	CycleID, OwnerID, Kind, Content string
	Now                             time.Time
}

func constraintKey(cycleID, ownerID, kind string) []column {
	return []column{{"cycle_id", "::UUID", cycleID}, {"owner_id", "::UUID", ownerID}, {"kind", "::VARCHAR", kind}}
}

// getPairConstraint is `session.get(PairSharedConstraint, (cycle, owner, kind))`.
func (r Repository) getPairConstraint(ctx context.Context, cycleID, ownerID, kind string) (*PairConstraint, error) {
	var c PairConstraint
	err := r.Q.QueryRow(ctx,
		`SELECT pair_shared_constraints.cycle_id AS pair_shared_constraints_cycle_id,
		        pair_shared_constraints.owner_id AS pair_shared_constraints_owner_id,
		        pair_shared_constraints.kind AS pair_shared_constraints_kind,
		        pair_shared_constraints.content AS pair_shared_constraints_content,
		        pair_shared_constraints.version AS pair_shared_constraints_version,
		        pair_shared_constraints.updated_at AS pair_shared_constraints_updated_at
		   FROM pair_shared_constraints
		  WHERE pair_shared_constraints.cycle_id = $1::UUID AND pair_shared_constraints.owner_id = $2::UUID
		    AND pair_shared_constraints.kind = $3::VARCHAR`, cycleID, ownerID, kind).
		Scan(new(string), &c.OwnerID, &c.Kind, &c.Content, &c.Version, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.UpdatedAt = c.UpdatedAt.UTC()
	return &c, nil
}

// SetPairConstraint is set_pair_constraint.
//
// Statements, in Python's order: the row by its key; then either an INSERT of
// every column at version 1, or an UPDATE of content (when it changes),
// version + 1 (always) and updated_at (when it changes). The record carries
// what was assigned, not a read back. A version at the INTEGER maximum fails
// in PostgreSQL (22003) and constraint_content_length refuses more than 200
// characters.
func (r Repository) SetPairConstraint(ctx context.Context, in PairConstraintInput) (PairConstraint, error) {
	existing, err := r.getPairConstraint(ctx, in.CycleID, in.OwnerID, in.Kind)
	if err != nil {
		return PairConstraint{}, err
	}
	updated := pythonInstant(in.Now)
	if existing == nil {
		if _, err := r.Q.Exec(ctx, renderInsert("pair_shared_constraints", []insertColumn{{"cycle_id", "::UUID"},
			{"owner_id", "::UUID"}, {"kind", "::VARCHAR"}, {"content", "::VARCHAR"}, {"version", "::INTEGER"},
			{"updated_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
			in.CycleID, in.OwnerID, in.Kind, in.Content, sqlInteger(1), updated); err != nil {
			return PairConstraint{}, err
		}
		return PairConstraint{OwnerID: in.OwnerID, Kind: in.Kind, Content: in.Content, Version: 1,
			UpdatedAt: updated.UTC()}, nil
	}
	var sets []column
	if existing.Content != in.Content {
		sets = append(sets, column{"content", "::VARCHAR", in.Content})
	}
	version := int64(existing.Version) + 1
	sets = append(sets, column{"version", "::INTEGER", sqlInteger(version)})
	if !existing.UpdatedAt.Equal(updated) {
		sets = append(sets, column{"updated_at", "::TIMESTAMP WITH TIME ZONE", updated})
	}
	if err := r.updateRow(ctx, "pair_shared_constraints", sets, constraintKey(in.CycleID, in.OwnerID, in.Kind)); err != nil {
		return PairConstraint{}, err
	}
	return PairConstraint{OwnerID: existing.OwnerID, Kind: existing.Kind, Content: in.Content, Version: int(version),
		UpdatedAt: updated.UTC()}, nil
}

// DeletePairConstraint is delete_pair_constraint: the row by its key, false
// without a second statement when there is none, otherwise its DELETE and
// true.
func (r Repository) DeletePairConstraint(ctx context.Context, cycleID, ownerID, kind string) (bool, error) {
	existing, err := r.getPairConstraint(ctx, cycleID, ownerID, kind)
	if err != nil || existing == nil {
		return false, err
	}
	if err := r.deleteRow(ctx, "pair_shared_constraints", constraintKey(cycleID, ownerID, kind)); err != nil {
		return false, err
	}
	return true, nil
}

// PairRhythm is PairRhythmRecord (ADR-0034 §2.4). NguoiLoID nil is «cả hai».
type PairRhythm struct {
	CycleID   string
	Tuan      time.Time
	NguoiLoID *string
	ChonBoiID string
	UpdatedAt time.Time
}

// PairRhythmInput is set_pair_rhythm's arguments.
type PairRhythmInput struct {
	CycleID   string
	Tuan      time.Time
	NguoiLoID *string
	ChonBoiID string
	Now       time.Time
}

func rhythmKey(cycleID string, tuan time.Time) []column {
	return []column{{"cycle_id", "::UUID", cycleID}, {"tuan", "::DATE", calendarDay(tuan)}}
}

// GetPairRhythm is get_pair_rhythm: an explicit SELECT by the key, every time.
func (r Repository) GetPairRhythm(ctx context.Context, cycleID string, tuan time.Time) (*PairRhythm, error) {
	var p PairRhythm
	err := r.Q.QueryRow(ctx,
		`SELECT pair_cycle_rhythms.cycle_id, pair_cycle_rhythms.tuan, pair_cycle_rhythms.nguoi_lo_id,
		        pair_cycle_rhythms.chon_boi_id, pair_cycle_rhythms.updated_at
		   FROM pair_cycle_rhythms
		  WHERE pair_cycle_rhythms.cycle_id = $1::UUID AND pair_cycle_rhythms.tuan = $2::DATE`,
		cycleID, calendarDay(tuan)).
		Scan(&p.CycleID, &p.Tuan, &p.NguoiLoID, &p.ChonBoiID, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Tuan = calendarDay(p.Tuan)
	p.UpdatedAt = p.UpdatedAt.UTC()
	return &p, nil
}

// SetPairRhythm is set_pair_rhythm: the row by its key, then an INSERT, or an
// UPDATE of only the columns whose value changed (the session's flush).
func (r Repository) SetPairRhythm(ctx context.Context, in PairRhythmInput) (PairRhythm, error) {
	existing, err := r.GetPairRhythm(ctx, in.CycleID, in.Tuan)
	if err != nil {
		return PairRhythm{}, err
	}
	updated := pythonInstant(in.Now)
	out := PairRhythm{CycleID: in.CycleID, Tuan: calendarDay(in.Tuan), NguoiLoID: in.NguoiLoID, ChonBoiID: in.ChonBoiID, UpdatedAt: updated.UTC()}
	if existing == nil {
		_, err := r.Q.Exec(ctx, renderInsert("pair_cycle_rhythms", []insertColumn{{"cycle_id", "::UUID"}, {"tuan", "::DATE"},
			{"nguoi_lo_id", "::UUID"}, {"chon_boi_id", "::UUID"}, {"updated_at", "::TIMESTAMP WITH TIME ZONE"}}, 1),
			in.CycleID, calendarDay(in.Tuan), in.NguoiLoID, in.ChonBoiID, updated)
		return out, err
	}
	var sets []column
	if !sameText(existing.NguoiLoID, in.NguoiLoID) {
		sets = append(sets, column{"nguoi_lo_id", "::UUID", in.NguoiLoID})
	}
	if existing.ChonBoiID != in.ChonBoiID {
		sets = append(sets, column{"chon_boi_id", "::UUID", in.ChonBoiID})
	}
	if !existing.UpdatedAt.Equal(updated) {
		sets = append(sets, column{"updated_at", "::TIMESTAMP WITH TIME ZONE", updated})
	}
	return out, r.updateRow(ctx, "pair_cycle_rhythms", sets, rhythmKey(in.CycleID, in.Tuan))
}
