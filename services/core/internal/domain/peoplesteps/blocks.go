package peoplesteps

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"mobile/services/core/internal/domain/accountlifecycle"
	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/friendship"
)

// BlockState is BlockResponse: the edge after the button.
type BlockState struct {
	PersonID string
	State    string
}

// BlockedPerson is BlockedPersonSummary.
type BlockedPerson struct {
	PersonID    string
	DisplayName string
	BlockedAt   time.Time
}

// LogLine is one record the service logs.
type LogLine struct {
	Level   string
	Message string
}

// Log lines delete_own_account writes.
const (
	LogUnlinkFailed = "account.deleted: could not unlink a stored photo"
	logRemovedOf    = "account.deleted: %d photo file(s) removed of %d"
)

// AccountErased is what delete_own_account did with the photographs: how many
// storage keys the erasure reported, how many unlinks answered true, and how
// many failed.
type AccountErased struct {
	Total   int
	Removed int
	Failed  int
}

// Logs is what delete_own_account logged, in order: a WARNING for each failed
// unlink, then one INFO line with the counts.
func (e AccountErased) Logs() []LogLine {
	out := make([]LogLine, 0, e.Failed+1)
	for range e.Failed {
		out = append(out, LogLine{"WARNING", LogUnlinkFailed})
	}
	return append(out, LogLine{"INFO", fmt.Sprintf(logRemovedOf, e.Removed, e.Total)})
}

func lower(code string) string { return strings.ToLower(code) }

// ListBlockedPeople is list_blocked_people (GET /people/me/blocked): who the
// caller is blocking, blocked at the decision time or, without one, the
// edge's creation.
func ListBlockedPeople(s Store, actor Actor) ([]BlockedPerson, error) {
	if err := requirePermission("view_own_blocks", actor, nil, fact{"is_self", true}); err != nil {
		return nil, err
	}
	rows, err := s.ListBlocked(actor.ID)
	if err != nil {
		return nil, err
	}
	blocked := make([]BlockedPerson, len(rows))
	for i, row := range rows {
		at := row.CreatedAt
		if row.DecidedAt != nil {
			at = *row.DecidedAt
		}
		blocked[i] = BlockedPerson{PersonID: row.OtherPersonID, DisplayName: row.OtherDisplayName, BlockedAt: at}
	}
	return blocked, nil
}

// DeleteOwnAccount is delete_own_account (DELETE /people/me): permission, then
// the literal confirmation, then the erasure, then one unlink per storage key.
// A failed unlink is counted and never fails the request.
func DeleteOwnAccount(s Store, photos PhotoStorage, actor Actor, confirm bool, now time.Time) (AccountErased, error) {
	if err := requirePermission("delete_own_account", actor, nil, fact{"is_self", true}); err != nil {
		return AccountErased{}, err
	}
	if err := accountlifecycle.CheckConfirmation(confirm); err != nil {
		return AccountErased{}, refusal(422, "confirm_required", "Cần xác nhận rõ ràng để xoá tài khoản.")
	}
	report, err := s.ErasePerson(actor.ID, now)
	if err != nil {
		if _, ok := conflictCode(err); ok {
			return AccountErased{}, refusal(404, "person_not_found", "Chưa có hồ sơ cho tài khoản này.")
		}
		return AccountErased{}, err
	}
	erased := AccountErased{Total: len(report.StorageKeys)}
	for _, key := range report.StorageKeys {
		removed, err := photos.Delete(key)
		switch {
		case err != nil:
			erased.Failed++
		case removed:
			erased.Removed++
		}
	}
	return erased, nil
}

// BlockPerson is block_person (POST /people/{person_id}/block): idempotent, so
// an edge that already is a block answers blocked without a write, and a race
// on the insert (EDGE_EXISTS) is the same wall.
func BlockPerson(s Store, actor Actor, personID string, now time.Time) (BlockState, error) {
	if err := requirePermission("block_person", actor, nil, fact{"is_not_self", actor.ID != personID}); err != nil {
		return BlockState{}, err
	}
	person, err := s.GetPerson(personID)
	if err != nil {
		return BlockState{}, err
	}
	if person == nil {
		return BlockState{}, refusal(404, "person_not_found", "Chưa có ai mang danh tính này.")
	}
	edge, err := s.GetFriendEdge(actor.ID, personID)
	if err != nil {
		return BlockState{}, err
	}
	var existing *friendship.Edge
	if edge != nil {
		existing = &friendship.Edge{
			RequesterID: edge.RequesterID,
			AddresseeID: edge.AddresseeID,
			State:       edge.State,
			DecidedByID: edge.DecidedByID,
		}
	}
	blocked := BlockState{PersonID: personID, State: friendship.StateBlocked}
	if _, err := friendship.OpenBlock(actor.ID, personID, existing); err != nil {
		var refused *friendship.FriendshipError
		if !errors.As(err, &refused) {
			return BlockState{}, err
		}
		if refused.Code == friendship.CodeAlreadyBlocked {
			return blocked, nil
		}
		return BlockState{}, FriendRefusal(refused.Code)
	}
	if err := s.OpenBlockEdge(actor.ID, personID, now); err != nil {
		if code, ok := conflictCode(err); !ok || code != "EDGE_EXISTS" {
			return BlockState{}, err
		}
	}
	return blocked, nil
}

// UnblockPerson is unblock_person (DELETE /people/{person_id}/block): the edge
// is read first, only whoever decided the block may lift it, and a refused lift
// is 409 with the repository's code in lower case.
func UnblockPerson(s Store, actor Actor, personID string, now time.Time) (BlockState, error) {
	edge, err := friendEdge(s, actor.ID, personID)
	if err != nil {
		return BlockState{}, err
	}
	blocker := blocking.BlockerOf(edge)
	isBlocker := blocker != nil && *blocker == actor.ID
	if err := requirePermission("unblock_person", actor, nil, fact{"is_blocker", isBlocker}); err != nil {
		return BlockState{}, err
	}
	if err := s.LiftBlockEdge(actor.ID, personID, now); err != nil {
		if code, ok := conflictCode(err); ok {
			return BlockState{}, refusal(409, lower(code), "Không gỡ chặn được người này.")
		}
		return BlockState{}, err
	}
	return BlockState{PersonID: personID, State: friendship.StateDeclined}, nil
}
