package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// PairConsent is PairConsentRecord: one person's answer to one proposal,
// carrying the PROPOSAL's deadline and terms version.
type PairConsent struct {
	ProposalID        string
	PersonID          string
	Purpose           string
	GrantedAt         *time.Time
	RevokedAt         *time.Time
	ProposalExpiresAt time.Time
	TermsVersion      int
}

// PairProposal is PairProposalRecord.
type PairProposal struct {
	ID           string
	CycleID      string
	Purpose      string
	ProposedByID string
	TermsVersion int
	CompletedAt  *time.Time
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// PairConstraint is PairConstraintRecord.
type PairConstraint struct {
	OwnerID   string
	Kind      string
	Content   string
	Version   int
	UpdatedAt time.Time
}

// PairNotebook is PairNotebookRecord. Without a live cycle the cycle fields
// are nil, TermsVersion is 0 and every list is empty (never nil).
type PairNotebook struct {
	ID           string
	ContextID    string
	CycleID      *string
	CycleState   *string
	TermsVersion int
	Participants []string
	Consents     []PairConsent
	Proposals    []PairProposal
	Constraints  []PairConstraint
}

// GetPairNotebook is get_pair_notebook.
//
// Statements, in Python's order:
//  1. the notebook by context_id (`scalars(...).first()`: no LIMIT, the
//     unique constraint keeps it to one row);
//  2. `_live_cycle`: the newest cycle whose state is not 'closed';
//
// and only when there is a live cycle, `_pair_notebook_record`'s reads:
//  3. participants by (created_at, person_id);
//  4. proposals by (created_at, id);
//  5. shared constraints by (owner_id, kind);
//  6. consents joined to their proposals by (consent created_at, consent id)
//     -- last, because Python evaluates `consents=self._pair_consent_rows(...)`
//     inside the record constructor, after the three locals above.
func (r Repository) GetPairNotebook(ctx context.Context, contextID string) (*PairNotebook, error) {
	var notebook PairNotebook
	var contextKind string
	var notebookCreated time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT pair_notebooks.id, pair_notebooks.context_id, pair_notebooks.context_kind, pair_notebooks.created_at
		   FROM pair_notebooks
		  WHERE pair_notebooks.context_id = $1::UUID`, contextID).
		Scan(&notebook.ID, &notebook.ContextID, &contextKind, &notebookCreated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	notebook.Participants = []string{}
	notebook.Consents = []PairConsent{}
	notebook.Proposals = []PairProposal{}
	notebook.Constraints = []PairConstraint{}

	var cycleID, cycleState string
	var termsVersion int
	var openedAt, closedAt *time.Time
	var cycleCreated time.Time
	err = r.Q.QueryRow(ctx,
		`SELECT pair_notebook_cycles.id, pair_notebook_cycles.notebook_id, pair_notebook_cycles.state,
		        pair_notebook_cycles.terms_version, pair_notebook_cycles.opened_at,
		        pair_notebook_cycles.closed_at, pair_notebook_cycles.created_at
		   FROM pair_notebook_cycles
		  WHERE pair_notebook_cycles.notebook_id = $1::UUID AND pair_notebook_cycles.state != $2::VARCHAR
		  ORDER BY pair_notebook_cycles.created_at DESC
		  LIMIT $3::INTEGER`, notebook.ID, "closed", 1).
		Scan(&cycleID, new(string), &cycleState, &termsVersion, &openedAt, &closedAt, &cycleCreated)
	if errors.Is(err, pgx.ErrNoRows) {
		return &notebook, nil
	}
	if err != nil {
		return nil, err
	}
	notebook.CycleID, notebook.CycleState, notebook.TermsVersion = &cycleID, &cycleState, termsVersion

	rows, err := r.Q.Query(ctx,
		`SELECT pair_cycle_participants.person_id
		   FROM pair_cycle_participants
		  WHERE pair_cycle_participants.cycle_id = $1::UUID
		  ORDER BY pair_cycle_participants.created_at, pair_cycle_participants.person_id`, cycleID)
	if err != nil {
		return nil, err
	}
	people, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	notebook.Participants = append(notebook.Participants, people...)

	rows, err = r.Q.Query(ctx,
		`SELECT pair_consent_proposals.id, pair_consent_proposals.cycle_id, pair_consent_proposals.purpose,
		        pair_consent_proposals.proposed_by_id, pair_consent_proposals.terms_version,
		        pair_consent_proposals.completed_at, pair_consent_proposals.created_at,
		        pair_consent_proposals.expires_at
		   FROM pair_consent_proposals
		  WHERE pair_consent_proposals.cycle_id = $1::UUID
		  ORDER BY pair_consent_proposals.created_at, pair_consent_proposals.id`, cycleID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p PairProposal
		if err := rows.Scan(&p.ID, &p.CycleID, &p.Purpose, &p.ProposedByID, &p.TermsVersion,
			&p.CompletedAt, &p.CreatedAt, &p.ExpiresAt); err != nil {
			rows.Close()
			return nil, err
		}
		p.CompletedAt, p.CreatedAt, p.ExpiresAt = utcOptional(p.CompletedAt), p.CreatedAt.UTC(), p.ExpiresAt.UTC()
		notebook.Proposals = append(notebook.Proposals, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT pair_shared_constraints.cycle_id, pair_shared_constraints.owner_id, pair_shared_constraints.kind,
		        pair_shared_constraints.content, pair_shared_constraints.version, pair_shared_constraints.updated_at
		   FROM pair_shared_constraints
		  WHERE pair_shared_constraints.cycle_id = $1::UUID
		  ORDER BY pair_shared_constraints.owner_id, pair_shared_constraints.kind`, cycleID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c PairConstraint
		if err := rows.Scan(new(string), &c.OwnerID, &c.Kind, &c.Content, &c.Version, &c.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		c.UpdatedAt = c.UpdatedAt.UTC()
		notebook.Constraints = append(notebook.Constraints, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT pair_consents.id, pair_consents.proposal_id, pair_consents.person_id, pair_consents.granted_at,
		        pair_consents.revoked_at, pair_consents.created_at, pair_consent_proposals.id AS id_1,
		        pair_consent_proposals.cycle_id, pair_consent_proposals.purpose,
		        pair_consent_proposals.proposed_by_id, pair_consent_proposals.terms_version,
		        pair_consent_proposals.completed_at, pair_consent_proposals.created_at AS created_at_1,
		        pair_consent_proposals.expires_at
		   FROM pair_consents JOIN pair_consent_proposals ON pair_consent_proposals.id = pair_consents.proposal_id
		  WHERE pair_consent_proposals.cycle_id = $1::UUID
		  ORDER BY pair_consents.created_at, pair_consents.id`, cycleID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c PairConsent
		var ignored string
		var ignoredInstant time.Time
		var ignoredOptional *time.Time
		if err := rows.Scan(&ignored, &c.ProposalID, &c.PersonID, &c.GrantedAt, &c.RevokedAt, &ignoredInstant,
			&ignored, &ignored, &c.Purpose, &ignored, &c.TermsVersion, &ignoredOptional, &ignoredInstant,
			&c.ProposalExpiresAt); err != nil {
			rows.Close()
			return nil, err
		}
		c.GrantedAt, c.RevokedAt, c.ProposalExpiresAt = utcOptional(c.GrantedAt), utcOptional(c.RevokedAt), c.ProposalExpiresAt.UTC()
		notebook.Consents = append(notebook.Consents, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &notebook, nil
}
