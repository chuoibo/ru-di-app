package repo

// The W3 slice of SqlAlchemyApiRepository for groups and their rosters:
// create_context, update_context, add_member, get_membership,
// accept_membership, leave_context and membership_role. get_context and
// list_members are members.go's, from the pilot wave.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ContextChanges is the `changes` dict ApiService.update_context hands
// update_context. A nil pointer is a key that is absent.
type ContextChanges struct {
	DisplayName *string
	Theme       *string
}

// ErrUnknownMembershipRole is the ValueError `MembershipRole(role)` raises in
// add_member before any statement runs.
var ErrUnknownMembershipRole = errors.New("repo: not a membership role")

// ErrStaleUpdate is SQLAlchemy's StaleDataError: a flush whose UPDATE by
// primary key matched no row. It needs the row to vanish between the read and
// the write, which a single transaction cannot stage, so no oracle case
// reaches it.
var ErrStaleUpdate = errors.New("repo: UPDATE by primary key matched no row")

// membershipSavepoint is the name SQLAlchemy gives the first savepoint of a
// connection. add_member's begin_nested is the first savepoint both routes
// that reach it (POST /contexts, POST /contexts/{id}/members) open.
const membershipSavepoint = "sa_savepoint_1"

var membershipRoles = map[string]bool{"member": true, "admin": true}

// membershipColumns is `select(Membership)`: every mapped column in
// declaration order, unlabelled.
const membershipColumns = `memberships.id, memberships.context_id, memberships.person_id, memberships.state,
       memberships.role, memberships.origin, memberships.invited_by_id, memberships.joined_at,
       memberships.left_at, memberships.created_at`

func scanMembership(row pgx.Row) (*Membership, error) {
	var m Membership
	err := row.Scan(&m.ID, &m.ContextID, &m.PersonID, &m.State, &m.Role, &m.Origin,
		&m.InvitedByID, &m.JoinedAt, &m.LeftAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.JoinedAt, m.LeftAt, m.CreatedAt = utcOptional(m.JoinedAt), utcOptional(m.LeftAt), m.CreatedAt.UTC()
	return &m, nil
}

// membershipRecord is `_membership_record(membership)` for a single row: one
// `_display_names` statement for its person.
func (r Repository) membershipRecord(ctx context.Context, m *Membership) (*Membership, error) {
	names, err := r.displayNames(ctx, []string{m.PersonID})
	if err != nil {
		return nil, err
	}
	m.DisplayName = names[m.PersonID]
	return m, nil
}

// execUpdate runs a flush's UPDATE by primary key and refuses a statement that
// matched no row, as the unit of work does.
func (r Repository) execUpdate(ctx context.Context, sql string, args ...any) error {
	tag, err := r.Q.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleUpdate
	}
	return nil
}

// CreateContext is create_context: one INSERT of a group with a client-side
// uuid4. The unit of work lists every mapped column that has no server default
// (pair_key as NULL, kind as the Python default "group") and reads theme and
// created_at back with RETURNING, so created_at is the transaction's now().
// A creator with no people row fails on fk_contexts_created_by and that
// *pgconn.PgError is returned as is.
func (r Repository) CreateContext(ctx context.Context, displayName, createdByID string) (ContextRecord, error) {
	id, err := newUUID()
	if err != nil {
		return ContextRecord{}, err
	}
	c := ContextRecord{ID: id, DisplayName: displayName, CreatedByID: createdByID, Kind: "group"}
	if err := r.Q.QueryRow(ctx,
		`INSERT INTO contexts (id, display_name, created_by_id, kind, pair_key)
		 VALUES ($1::UUID, $2::VARCHAR, $3::UUID, $4::VARCHAR, $5::VARCHAR)
		 RETURNING contexts.theme, contexts.created_at`,
		id, displayName, createdByID, "group", nil).Scan(&c.Theme, &c.CreatedAt); err != nil {
		return ContextRecord{}, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return c, nil
}

// UpdateContext is update_context: `session.get(Context, id)` (members.go's
// GetContext statement, no lock), nil when there is none, then one UPDATE of
// the columns whose value changes, in mapper order (display_name, theme). A
// value equal to the stored one is not a change, and no changed column means
// no UPDATE. The CHECK on theme and its varchar(16) refuse in PostgreSQL.
//
// SQLAlchemy note: when the same session already loaded the context
// (ApiService.update_context calls _require_group_kind, which reads it, before
// a rename), session.get answers from the identity map and Python issues no
// SELECT here. Go always reads.
func (r Repository) UpdateContext(ctx context.Context, contextID string, changes ContextChanges) (*ContextRecord, error) {
	c, err := r.GetContext(ctx, contextID)
	if err != nil || c == nil {
		return nil, err
	}
	var sets []string
	var args []any
	set := func(column string, value string) {
		args = append(args, value)
		sets = append(sets, column+"=$"+strconv.Itoa(len(args))+"::VARCHAR")
	}
	if changes.DisplayName != nil && *changes.DisplayName != c.DisplayName {
		c.DisplayName = *changes.DisplayName
		set("display_name", c.DisplayName)
	}
	if changes.Theme != nil && *changes.Theme != c.Theme {
		c.Theme = *changes.Theme
		set("theme", c.Theme)
	}
	if len(sets) == 0 {
		return c, nil
	}
	args = append(args, contextID)
	if err := r.execUpdate(ctx,
		`UPDATE contexts SET `+strings.Join(sets, ", ")+` WHERE contexts.id = $`+strconv.Itoa(len(args))+`::UUID`,
		args...); err != nil {
		return nil, err
	}
	return c, nil
}

// AddMember is add_member: always a new invited, named membership (rejoining
// is a new period, never a revived row).
//
// Statements, in Python's order:
//  1. `MembershipRole(role)`: ErrUnknownMembershipRole before anything;
//  2. SAVEPOINT, then the INSERT of every mapped column except created_at
//     (joined_at and left_at as NULL), created_at read back with RETURNING;
//  3. on success RELEASE SAVEPOINT and the person's `_display_names`;
//  4. on failure ROLLBACK TO SAVEPOINT first. A unique violation of
//     uq_memberships_open_per_person is Conflict MEMBERSHIP_ALREADY_OPEN;
//     every other error (a missing person or context, which are foreign keys)
//     is returned as is, with the transaction still usable.
func (r Repository) AddMember(ctx context.Context, contextID, personID, invitedByID, role string) (Membership, error) {
	if !membershipRoles[role] {
		return Membership{}, ErrUnknownMembershipRole
	}
	id, err := newUUID()
	if err != nil {
		return Membership{}, err
	}
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+membershipSavepoint); err != nil {
		return Membership{}, err
	}
	invitedBy := invitedByID
	m := Membership{ID: id, ContextID: contextID, PersonID: personID, State: "invited", Role: role,
		Origin: "named", InvitedByID: &invitedBy}
	insertErr := r.Q.QueryRow(ctx,
		`INSERT INTO memberships (id, context_id, person_id, state, role, origin, invited_by_id, joined_at, left_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5, $6, $7::UUID, $8::TIMESTAMP WITH TIME ZONE,
		         $9::TIMESTAMP WITH TIME ZONE)
		 RETURNING memberships.created_at`,
		id, contextID, personID, "invited", role, "named", invitedByID, nil, nil).Scan(&m.CreatedAt)
	if insertErr != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+membershipSavepoint); err != nil {
			return Membership{}, err
		}
		if pg := integrityViolation(insertErr); pg != nil && pg.ConstraintName == "uq_memberships_open_per_person" {
			return Membership{}, &Conflict{Code: "MEMBERSHIP_ALREADY_OPEN", Err: pg}
		}
		return Membership{}, insertErr
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+membershipSavepoint); err != nil {
		return Membership{}, err
	}
	m.CreatedAt = m.CreatedAt.UTC()
	record, err := r.membershipRecord(ctx, &m)
	if err != nil {
		return Membership{}, err
	}
	return *record, nil
}

// GetMembership is get_membership: `session.scalar` by id (no lock, no
// LIMIT), nil when there is none, then the person's `_display_names`.
func (r Repository) GetMembership(ctx context.Context, membershipID string) (*Membership, error) {
	m, err := scanMembership(r.Q.QueryRow(ctx,
		`SELECT `+membershipColumns+`
		   FROM memberships
		  WHERE memberships.id = $1::UUID`, membershipID))
	if err != nil || m == nil {
		return nil, err
	}
	return r.membershipRecord(ctx, m)
}

// AcceptMembership is accept_membership.
//
// Statements and refusals, in Python's order:
//  1. the row by id, SELECT ... FOR UPDATE (populate_existing, so always a
//     statement); nil when there is none;
//  2. a state other than invited is Conflict MEMBERSHIP_NOT_INVITED, raised
//     from nothing, with the row still locked;
//  3. one UPDATE of the columns whose value changes, in mapper order (state,
//     joined_at): a stored joined_at equal to now is left out;
//  4. the person's `_display_names`.
func (r Repository) AcceptMembership(ctx context.Context, membershipID string, now time.Time) (*Membership, error) {
	m, err := scanMembership(r.Q.QueryRow(ctx,
		`SELECT `+membershipColumns+`
		   FROM memberships
		  WHERE memberships.id = $1::UUID FOR UPDATE`, membershipID))
	if err != nil || m == nil {
		return nil, err
	}
	if m.State != "invited" {
		return nil, &Conflict{Code: "MEMBERSHIP_NOT_INVITED"}
	}
	joined := pythonInstant(now)
	if err := r.transitionMembership(ctx, m, "active", "joined_at", m.JoinedAt, joined); err != nil {
		return nil, err
	}
	m.State, m.JoinedAt = "active", &joined
	return r.membershipRecord(ctx, m)
}

// LeaveContext is leave_context: the person's ACTIVE, open membership of the
// context, SELECT ... FOR UPDATE (`session.scalar`: no LIMIT, the first row;
// uq_memberships_open_per_person keeps it to one), nil when there is none,
// then one UPDATE of state and left_at (the columns whose value changes, in
// mapper order), then the person's `_display_names`.
func (r Repository) LeaveContext(ctx context.Context, contextID, personID string, now time.Time) (*Membership, error) {
	m, err := scanMembership(r.Q.QueryRow(ctx,
		`SELECT `+membershipColumns+`
		   FROM memberships
		  WHERE memberships.context_id = $1::UUID AND memberships.person_id = $2::UUID
		    AND memberships.state = $3 AND memberships.left_at IS NULL FOR UPDATE`,
		contextID, personID, "active"))
	if err != nil || m == nil {
		return nil, err
	}
	left := pythonInstant(now)
	if err := r.transitionMembership(ctx, m, "left", "left_at", m.LeftAt, left); err != nil {
		return nil, err
	}
	m.State, m.LeftAt = "left", &left
	return r.membershipRecord(ctx, m)
}

// transitionMembership is the flush of `membership.state = state;
// membership.<column> = instant`: the changed columns only, state first.
func (r Repository) transitionMembership(ctx context.Context, m *Membership, state, column string, stored *time.Time, instant time.Time) error {
	var sets []string
	var args []any
	if m.State != state {
		args = append(args, state)
		sets = append(sets, "state=$"+strconv.Itoa(len(args)))
	}
	if stored == nil || !stored.Equal(instant) {
		args = append(args, instant)
		sets = append(sets, column+"=$"+strconv.Itoa(len(args))+"::TIMESTAMP WITH TIME ZONE")
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, m.ID)
	return r.execUpdate(ctx,
		`UPDATE memberships SET `+strings.Join(sets, ", ")+` WHERE memberships.id = $`+strconv.Itoa(len(args))+`::UUID`,
		args...)
}

// MembershipRole is membership_role: the role of the person's ACTIVE, open
// membership (LIMIT 1), nil when there is none.
func (r Repository) MembershipRole(ctx context.Context, contextID, personID string) (*string, error) {
	var role string
	err := r.Q.QueryRow(ctx,
		`SELECT memberships.role
		   FROM memberships
		  WHERE memberships.context_id = $1::UUID
		    AND memberships.person_id = $2::UUID
		    AND memberships.state = $3
		    AND memberships.left_at IS NULL
		  LIMIT $4::INTEGER`,
		contextID, personID, "active", 1).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}
