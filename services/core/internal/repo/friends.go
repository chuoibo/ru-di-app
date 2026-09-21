package repo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// FriendEdge is FriendEdgeRecord: one friend_requests row oriented for a
// reader, with the other party's id and name.
type FriendEdge struct {
	ID               string
	RequesterID      string
	AddresseeID      string
	OtherPersonID    string
	OtherDisplayName string
	State            string
	DecidedByID      *string
	CreatedAt        time.Time
	DecidedAt        *time.Time
}

// friendRequestColumns is `select(FriendRequest)`: every mapped column in
// declaration order, unlabelled.
const friendRequestColumns = `friend_requests.id, friend_requests.requester_id, friend_requests.addressee_id,
       friend_requests.state, friend_requests.decided_by_id, friend_requests.created_at, friend_requests.decided_at`

type friendRow struct {
	id, requesterID, addresseeID, state string
	decidedByID                         *string
	createdAt                           time.Time
	decidedAt                           *time.Time
}

func scanFriendRow(row pgx.Row) (*friendRow, error) {
	var f friendRow
	err := row.Scan(&f.id, &f.requesterID, &f.addresseeID, &f.state, &f.decidedByID, &f.createdAt, &f.decidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.createdAt, f.decidedAt = f.createdAt.UTC(), utcOptional(f.decidedAt)
	return &f, nil
}

func (r Repository) friendRows(ctx context.Context, sql string, args ...any) ([]friendRow, error) {
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []friendRow{}
	for rows.Next() {
		f, err := scanFriendRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// other is `row.addressee_id if row.requester_id == reader_id else
// row.requester_id`: a reader who is neither party gets the requester.
func (f friendRow) other(readerID string) string {
	if f.requesterID == readerID {
		return f.addresseeID
	}
	return f.requesterID
}

func (f friendRow) record(readerID, name string) FriendEdge {
	return FriendEdge{ID: f.id, RequesterID: f.requesterID, AddresseeID: f.addresseeID,
		OtherPersonID: f.other(readerID), OtherDisplayName: name, State: f.state,
		DecidedByID: f.decidedByID, CreatedAt: f.createdAt, DecidedAt: f.decidedAt}
}

// friendEdge is `_friend_edge(row, reader_id)` without a name: one
// `_display_names` statement for the other party.
func (r Repository) friendEdge(ctx context.Context, f friendRow, readerID string) (FriendEdge, error) {
	other := f.other(readerID)
	names, err := r.displayNames(ctx, []string{other})
	if err != nil {
		return FriendEdge{}, err
	}
	return f.record(readerID, names[other]), nil
}

// GetFriendEdge is get_friend_edge: `_edge_between`, the pair's newest row
// whose state is not declined, whichever way it was asked
// (`session.scalar`: no LIMIT, the first row), oriented for person_a.
func (r Repository) GetFriendEdge(ctx context.Context, personA, personB string) (*FriendEdge, error) {
	f, err := scanFriendRow(r.Q.QueryRow(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE (friend_requests.requester_id = $1::UUID AND friend_requests.addressee_id = $2::UUID
		         OR friend_requests.requester_id = $3::UUID AND friend_requests.addressee_id = $4::UUID)
		    AND friend_requests.state != $5
		  ORDER BY friend_requests.created_at DESC`,
		personA, personB, personB, personA, "declined"))
	if err != nil || f == nil {
		return nil, err
	}
	edge, err := r.friendEdge(ctx, *f, personA)
	if err != nil {
		return nil, err
	}
	return &edge, nil
}

// GetFriendRequest is get_friend_request: the row by id, and nil (after the
// read, before any name) for a reader who is neither party.
func (r Repository) GetFriendRequest(ctx context.Context, requestID, readerID string) (*FriendEdge, error) {
	f, err := scanFriendRow(r.Q.QueryRow(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE friend_requests.id = $1::UUID`, requestID))
	if err != nil || f == nil {
		return nil, err
	}
	if readerID != f.requesterID && readerID != f.addresseeID {
		return nil, nil
	}
	edge, err := r.friendEdge(ctx, *f, readerID)
	if err != nil {
		return nil, err
	}
	return &edge, nil
}

// OpenFriendRequest is open_friend_request: one INSERT of a pending row with a
// client-side uuid4 and the caller's clock, every mapped column listed (the
// unset nullable ones as NULL), then the addressee's name.
//
// Every IntegrityError of the flush becomes Conflict FRIEND_EDGE_EXISTS, not
// only uq_friend_edge_live: a missing person (a foreign key) and a self-edge
// (no_self_friendship) answer the same code, as in Python. The transaction is
// aborted afterwards, as the Python session's is.
func (r Repository) OpenFriendRequest(ctx context.Context, requesterID, addresseeID string, now time.Time) (FriendEdge, error) {
	id, err := newUUID()
	if err != nil {
		return FriendEdge{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO friend_requests (id, requester_id, addressee_id, state, decided_by_id, created_at, decided_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5::UUID, $6::TIMESTAMP WITH TIME ZONE, $7::TIMESTAMP WITH TIME ZONE)`,
		id, requesterID, addresseeID, "pending", nil, created, nil); err != nil {
		if pg := integrityViolation(err); pg != nil {
			return FriendEdge{}, &Conflict{Code: "FRIEND_EDGE_EXISTS", Err: pg}
		}
		return FriendEdge{}, err
	}
	return r.friendEdge(ctx, friendRow{id: id, requesterID: requesterID, addresseeID: addresseeID,
		state: "pending", createdAt: created}, requesterID)
}

// answerProducing is _ANSWER_PRODUCING: the decision that moves a row into a
// state. pending is a state no decision produces.
var answerProducing = map[string]string{"accepted": "accept", "declined": "decline", "blocked": "block"}

// decideFriendship is app.domain.friendship.decide's refusals, in its order,
// for an edge whose stored state the column's CHECK keeps valid.
func decideFriendship(f friendRow, actorID, decision string) error {
	if actorID != f.requesterID && actorID != f.addresseeID {
		return &FriendshipRefusal{Code: "NOT_A_PARTY"}
	}
	if decision == "block" {
		if f.state == "blocked" {
			return &FriendshipRefusal{Code: "ALREADY_BLOCKED"}
		}
		return nil
	}
	if f.state != "pending" {
		return &FriendshipRefusal{Code: "NOT_PENDING"}
	}
	if actorID != f.addresseeID {
		return &FriendshipRefusal{Code: "ONLY_ADDRESSEE_MAY_ANSWER"}
	}
	return nil
}

// DecideFriendRequest is decide_friend_request.
//
// Statements and refusals, in Python's order:
//  1. the row by id, SELECT ... FOR UPDATE (populate_existing); nil when
//     there is none, before the state is even looked at;
//  2. `FriendRequestState(state)`: ErrUnknownFriendRequestState (ValueError);
//     a state no decision produces (pending) is Conflict NOT_A_DECISION;
//  3. the domain's decide on the locked row: its refusal code as a Conflict;
//  4. one UPDATE of the columns whose value changes, in mapper order (state,
//     decided_by_id, decided_at): a block by the person who already decided
//     leaves decided_by_id out, and a decision at the stored decided_at
//     instant leaves decided_at out, because the flush compares with `==`;
//     an IntegrityError there is Conflict FRIEND_EDGE_EXISTS;
//  5. the other party's name, the record oriented for decided_by_id.
func (r Repository) DecideFriendRequest(ctx context.Context, requestID, state, decidedByID string, now time.Time) (*FriendEdge, error) {
	f, err := scanFriendRow(r.Q.QueryRow(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE friend_requests.id = $1::UUID FOR UPDATE`, requestID))
	if err != nil || f == nil {
		return nil, err
	}
	if state != "pending" && answerProducing[state] == "" {
		return nil, ErrUnknownFriendRequestState
	}
	answer := answerProducing[state]
	if answer == "" {
		return nil, &Conflict{Code: "NOT_A_DECISION"}
	}
	if err := decideFriendship(*f, decidedByID, answer); err != nil {
		return nil, &Conflict{Code: err.(*FriendshipRefusal).Code, Err: err}
	}

	decidedAt := pythonInstant(now)
	var sets []string
	var args []any
	set := func(column, cast string, value any) {
		args = append(args, value)
		sets = append(sets, column+"=$"+strconv.Itoa(len(args))+cast)
	}
	if f.state != state {
		set("state", "", state)
	}
	if f.decidedByID == nil || *f.decidedByID != decidedByID {
		set("decided_by_id", "::UUID", decidedByID)
	}
	if f.decidedAt == nil || !f.decidedAt.Equal(decidedAt) {
		set("decided_at", "::TIMESTAMP WITH TIME ZONE", decidedAt)
	}
	f.state, f.decidedByID, f.decidedAt = state, &decidedByID, &decidedAt
	if len(sets) > 0 {
		args = append(args, requestID)
		if _, err := r.Q.Exec(ctx,
			`UPDATE friend_requests SET `+strings.Join(sets, ", ")+
				` WHERE friend_requests.id = $`+strconv.Itoa(len(args))+`::UUID`, args...); err != nil {
			if pg := integrityViolation(err); pg != nil {
				return nil, &Conflict{Code: "FRIEND_EDGE_EXISTS", Err: pg}
			}
			return nil, err
		}
	}
	edge, err := r.friendEdge(ctx, *f, decidedByID)
	if err != nil {
		return nil, err
	}
	return &edge, nil
}

// orientedFriendRows is the tail list_friend_requests and list_friends share:
// one `_display_names` statement for the distinct other parties (none when
// there is no row), then each row oriented for the person.
func (r Repository) orientedFriendRows(ctx context.Context, rows []friendRow, personID string) ([]FriendEdge, error) {
	others := make([]string, len(rows))
	for i, f := range rows {
		others[i] = f.other(personID)
	}
	names, err := r.displayNames(ctx, others)
	if err != nil {
		return nil, err
	}
	out := []FriendEdge{}
	for i, f := range rows {
		out = append(out, f.record(personID, names[others[i]]))
	}
	return out, nil
}

// ListFriendRequests is list_friend_requests: pending rows addressed to the
// person when direction is exactly "incoming", sent by the person for any
// other value, newest first with the id breaking ties.
func (r Repository) ListFriendRequests(ctx context.Context, personID, direction string) ([]FriendEdge, error) {
	side := "friend_requests.requester_id"
	if direction == "incoming" {
		side = "friend_requests.addressee_id"
	}
	rows, err := r.friendRows(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE `+side+` = $1::UUID AND friend_requests.state = $2
		  ORDER BY friend_requests.created_at DESC, friend_requests.id`, personID, "pending")
	if err != nil {
		return nil, err
	}
	return r.orientedFriendRows(ctx, rows, personID)
}

// ListFriends is list_friends: accepted rows on either side, by decided_at
// DESC (PostgreSQL's NULLS FIRST, which the CHECK makes moot) then id.
func (r Repository) ListFriends(ctx context.Context, personID string) ([]FriendEdge, error) {
	rows, err := r.friendRows(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE (friend_requests.requester_id = $1::UUID OR friend_requests.addressee_id = $2::UUID)
		    AND friend_requests.state = $3
		  ORDER BY friend_requests.decided_at DESC, friend_requests.id`, personID, personID, "accepted")
	if err != nil {
		return nil, err
	}
	return r.orientedFriendRows(ctx, rows, personID)
}

// AccountIdentity is AccountIdentityRecord.
type AccountIdentity struct {
	ID          string
	PersonID    string
	Provider    string
	Subject     string
	CreatedAt   time.Time
	LastLoginAt time.Time
}

// GetAccountIdentity is get_account_identity: `session.scalar` by provider and
// subject (no LIMIT; the unique constraint keeps it to one row).
func (r Repository) GetAccountIdentity(ctx context.Context, provider, subject string) (*AccountIdentity, error) {
	var a AccountIdentity
	err := r.Q.QueryRow(ctx,
		`SELECT account_identities.id, account_identities.person_id, account_identities.provider,
		        account_identities.subject, account_identities.created_at, account_identities.last_login_at
		   FROM account_identities
		  WHERE account_identities.provider = $1::VARCHAR AND account_identities.subject = $2::VARCHAR`,
		provider, subject).Scan(&a.ID, &a.PersonID, &a.Provider, &a.Subject, &a.CreatedAt, &a.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt, a.LastLoginAt = a.CreatedAt.UTC(), a.LastLoginAt.UTC()
	return &a, nil
}
