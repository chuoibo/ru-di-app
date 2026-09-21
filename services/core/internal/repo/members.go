package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ContextRecord is ContextRecord.
type ContextRecord struct {
	ID          string
	DisplayName string
	CreatedByID string
	CreatedAt   time.Time
	Theme       string
	Kind        string
	PairKey     *string
}

// GetContext is get_context: `session.get(Context, id)`, every mapped column
// in declaration order, labelled table_column as Session.get labels them.
//
// SQLAlchemy note: session.get answers from the identity map without a
// statement when the same session already loaded the context. A request that
// reads the context twice issues this SELECT twice in Go.
func (r Repository) GetContext(ctx context.Context, contextID string) (*ContextRecord, error) {
	var c ContextRecord
	err := r.Q.QueryRow(ctx,
		`SELECT contexts.id AS contexts_id, contexts.display_name AS contexts_display_name,
		        contexts.created_by_id AS contexts_created_by_id, contexts.theme AS contexts_theme,
		        contexts.kind AS contexts_kind, contexts.pair_key AS contexts_pair_key,
		        contexts.created_at AS contexts_created_at
		   FROM contexts
		  WHERE contexts.id = $1::UUID`, contextID).
		Scan(&c.ID, &c.DisplayName, &c.CreatedByID, &c.Theme, &c.Kind, &c.PairKey, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return &c, nil
}

// Membership is MembershipRecord.
type Membership struct {
	ID          string
	ContextID   string
	PersonID    string
	DisplayName string
	State       string
	Role        string
	Origin      string
	InvitedByID *string
	JoinedAt    *time.Time
	LeftAt      *time.Time
	CreatedAt   time.Time
}

func utcOptional(instant *time.Time) *time.Time {
	if instant == nil {
		return nil
	}
	utc := instant.UTC()
	return &utc
}

// ListMembers is list_members: every membership that has not ended (left_at
// IS NULL, whatever its state), oldest first with the id breaking ties, then
// one `_display_names` statement for the distinct people.
//
// Reproduced from `_display_names`:
//   - no people statement when there is no membership;
//   - the IN list has one parameter per DISTINCT person (Python builds a set);
//   - a name is `found.get(id) or str(id)`, so an EMPTY display_name falls
//     back to the id exactly like a missing row does.
func (r Repository) ListMembers(ctx context.Context, contextID string) ([]Membership, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT memberships.id, memberships.context_id, memberships.person_id, memberships.state,
		        memberships.role, memberships.origin, memberships.invited_by_id, memberships.joined_at,
		        memberships.left_at, memberships.created_at
		   FROM memberships
		  WHERE memberships.context_id = $1::UUID AND memberships.left_at IS NULL
		  ORDER BY memberships.created_at, memberships.id`, contextID)
	if err != nil {
		return nil, err
	}
	out := []Membership{}
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.ID, &m.ContextID, &m.PersonID, &m.State, &m.Role, &m.Origin,
			&m.InvitedByID, &m.JoinedAt, &m.LeftAt, &m.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		m.JoinedAt, m.LeftAt, m.CreatedAt = utcOptional(m.JoinedAt), utcOptional(m.LeftAt), m.CreatedAt.UTC()
		out = append(out, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	var people []string
	seen := map[string]bool{}
	for _, m := range out {
		if !seen[m.PersonID] {
			seen[m.PersonID] = true
			people = append(people, m.PersonID)
		}
	}
	nameRows, err := r.Q.Query(ctx,
		`SELECT people.id, people.display_name
		   FROM people
		  WHERE people.id IN (`+uuidPlaceholders(1, len(people))+`)`, uuidArgs(people)...)
	if err != nil {
		return nil, err
	}
	found := map[string]string{}
	for nameRows.Next() {
		var id, name string
		if err := nameRows.Scan(&id, &name); err != nil {
			nameRows.Close()
			return nil, err
		}
		found[id] = name
	}
	nameRows.Close()
	if err := nameRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].DisplayName = found[out[i].PersonID]
		if out[i].DisplayName == "" {
			out[i].DisplayName = out[i].PersonID
		}
	}
	return out, nil
}

// PersonBand is one entry of budget_bands_by_person's dict, in insertion
// order.
type PersonBand struct {
	PersonID string
	Band     string
}

// BudgetBandsByPerson is budget_bands_by_person. People who never answered
// (NULL) are absent. The statement has no ORDER BY, so entries keep the order
// PostgreSQL returned the rows in, which is what the Python dict keeps; the IN
// list has one parameter per id as passed, duplicates included; an empty id
// list answers without a statement.
func (r Repository) BudgetBandsByPerson(ctx context.Context, personIDs []string) ([]PersonBand, error) {
	out := []PersonBand{}
	if len(personIDs) == 0 {
		return out, nil
	}
	rows, err := r.Q.Query(ctx,
		`SELECT people.id, people.budget_band
		   FROM people
		  WHERE people.id IN (`+uuidPlaceholders(1, len(personIDs))+`)`, uuidArgs(personIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var band *string
		if err := rows.Scan(&id, &band); err != nil {
			return nil, err
		}
		if band != nil {
			out = append(out, PersonBand{PersonID: id, Band: *band})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
