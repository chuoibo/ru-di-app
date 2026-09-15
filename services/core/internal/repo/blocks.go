package repo

// Blocking (ADR-0023 §2.3): open_block_edge, lift_block_edge and list_blocked
// of SqlAlchemyApiRepository. A block is the `blocked` state of the one live
// friend_requests row a pair may have, so these write friendRows.

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// blockSavepoint is the name SQLAlchemy gives a session's first begin_nested.
const blockSavepoint = "sa_savepoint_1"

// friendEdgeRow is `_friend_edge_row(a, b, for_update=True)`: the pair's row
// whose state is not declined, either way round, `LIMIT 1 FOR UPDATE`, no
// ORDER BY (uq_friend_edge_live allows one such row).
//
// SQLAlchemy note: without populate_existing, a row the same session already
// loaded keeps its loaded attributes; the flush below compares against those.
// Go compares against the row as read under the lock.
func (r Repository) friendEdgeRow(ctx context.Context, a, b string) (*friendRow, error) {
	return scanFriendRow(r.Q.QueryRow(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE (friend_requests.requester_id = $1::UUID AND friend_requests.addressee_id = $2::UUID
		         OR friend_requests.requester_id = $3::UUID AND friend_requests.addressee_id = $4::UUID)
		    AND friend_requests.state != $5
		  LIMIT $6::INTEGER FOR UPDATE`,
		a, b, b, a, "declined", 1))
}

// updateFriendRow is the flush of a friend_requests row whose state,
// decided_by_id and decided_at were assigned: an UPDATE of the columns whose
// value changed, in mapper order, none at all when nothing did.
func (r Repository) updateFriendRow(ctx context.Context, f *friendRow, state string, decidedBy string, decidedAt time.Time) error {
	var sets []string
	var args []any
	set := func(column, cast string, value any) {
		args = append(args, value)
		sets = append(sets, column+"=$"+strconv.Itoa(len(args))+cast)
	}
	if f.state != state {
		f.state = state
		set("state", "", state)
	}
	if f.decidedByID == nil || *f.decidedByID != decidedBy {
		decider := decidedBy
		f.decidedByID = &decider
		set("decided_by_id", "::UUID", decidedBy)
	}
	if f.decidedAt == nil || !f.decidedAt.Equal(decidedAt) {
		at := decidedAt
		f.decidedAt = &at
		set("decided_at", "::TIMESTAMP WITH TIME ZONE", decidedAt)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, f.id)
	return r.execUpdate(ctx,
		`UPDATE friend_requests SET `+strings.Join(sets, ", ")+` WHERE friend_requests.id = $`+strconv.Itoa(len(args))+`::UUID`,
		args...)
}

// OpenBlockEdge is open_block_edge: block, whether or not an edge exists.
//
// Statements, in Python's order:
//  1. friendEdgeRow, locked;
//  2. with a live row: its UPDATE to blocked by the blocker at now (only the
//     columns that change, possibly none), then one `_display_names` of BOTH
//     parties, and the record oriented for the blocker;
//  3. without one: SAVEPOINT, one INSERT of a blocked row (client-side uuid4,
//     created_at and decided_at both now, no RETURNING), RELEASE SAVEPOINT,
//     then the addressee's `_display_names`. Any IntegrityError of the INSERT
//     (the live-edge index, a missing person, the blocker blocking themself)
//     is Conflict EDGE_EXISTS after ROLLBACK TO SAVEPOINT; any other error is
//     returned after the same rollback.
func (r Repository) OpenBlockEdge(ctx context.Context, blockerID, addresseeID string, now time.Time) (FriendEdge, error) {
	at := pythonInstant(now)
	existing, err := r.friendEdgeRow(ctx, blockerID, addresseeID)
	if err != nil {
		return FriendEdge{}, err
	}
	if existing != nil {
		if err := r.updateFriendRow(ctx, existing, "blocked", blockerID, at); err != nil {
			return FriendEdge{}, err
		}
		names, err := r.displayNames(ctx, []string{existing.addresseeID, existing.requesterID})
		if err != nil {
			return FriendEdge{}, err
		}
		return existing.record(blockerID, names[existing.other(blockerID)]), nil
	}
	id, err := newUUID()
	if err != nil {
		return FriendEdge{}, err
	}
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+blockSavepoint); err != nil {
		return FriendEdge{}, err
	}
	decider := blockerID
	edge := friendRow{id: id, requesterID: blockerID, addresseeID: addresseeID, state: "blocked", decidedByID: &decider,
		createdAt: at, decidedAt: &at}
	if _, insertErr := r.Q.Exec(ctx,
		`INSERT INTO friend_requests (id, requester_id, addressee_id, state, decided_by_id, created_at, decided_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5::UUID, $6::TIMESTAMP WITH TIME ZONE, $7::TIMESTAMP WITH TIME ZONE)`,
		id, blockerID, addresseeID, "blocked", blockerID, at, at); insertErr != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+blockSavepoint); err != nil {
			return FriendEdge{}, err
		}
		if pg := integrityViolation(insertErr); pg != nil {
			return FriendEdge{}, &Conflict{Code: "EDGE_EXISTS", Err: pg}
		}
		return FriendEdge{}, insertErr
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+blockSavepoint); err != nil {
		return FriendEdge{}, err
	}
	names, err := r.displayNames(ctx, []string{addresseeID})
	if err != nil {
		return FriendEdge{}, err
	}
	return edge.record(blockerID, names[addresseeID]), nil
}

// LiftBlockEdge is lift_block_edge: the locked live row; Conflict NOT_BLOCKED
// when there is none or it is not blocked, ONLY_BLOCKER_MAY_UNBLOCK when
// somebody else decided it (both raised from nothing, before any write); then
// the UPDATE to declined by the blocker at now (decided_by_id never changes
// here) and the other party's `_display_names`.
func (r Repository) LiftBlockEdge(ctx context.Context, blockerID, addresseeID string, now time.Time) (FriendEdge, error) {
	edge, err := r.friendEdgeRow(ctx, blockerID, addresseeID)
	if err != nil {
		return FriendEdge{}, err
	}
	if edge == nil || edge.state != "blocked" {
		return FriendEdge{}, &Conflict{Code: "NOT_BLOCKED"}
	}
	if edge.decidedByID == nil || *edge.decidedByID != blockerID {
		return FriendEdge{}, &Conflict{Code: "ONLY_BLOCKER_MAY_UNBLOCK"}
	}
	if err := r.updateFriendRow(ctx, edge, "declined", blockerID, pythonInstant(now)); err != nil {
		return FriendEdge{}, err
	}
	other := edge.other(blockerID)
	names, err := r.displayNames(ctx, []string{other})
	if err != nil {
		return FriendEdge{}, err
	}
	return edge.record(blockerID, names[other]), nil
}

// ListBlocked is list_blocked: the blocked rows this person decided, newest
// decision first with the id breaking ties, then one `_display_names` of the
// other parties (none when there are no rows), each record oriented for the
// person.
func (r Repository) ListBlocked(ctx context.Context, personID string) ([]FriendEdge, error) {
	rows, err := r.friendRows(ctx,
		`SELECT `+friendRequestColumns+`
		   FROM friend_requests
		  WHERE friend_requests.state = $1 AND friend_requests.decided_by_id = $2::UUID
		  ORDER BY friend_requests.decided_at DESC, friend_requests.id`,
		"blocked", personID)
	if err != nil {
		return nil, err
	}
	others := make([]string, 0, len(rows))
	for _, row := range rows {
		others = append(others, row.other(personID))
	}
	names, err := r.displayNames(ctx, others)
	if err != nil {
		return nil, err
	}
	out := make([]FriendEdge, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.record(personID, names[row.other(personID)]))
	}
	return out, nil
}
