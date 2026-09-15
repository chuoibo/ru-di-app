// Package direct ports app/domain/direct.py (ADR-0021 §2.5): whether a context
// is a pair, who the other person of a pair is, what the reader sees as its
// name, and who may open one.
//
// POST /contexts, PATCH /contexts/{id} and GET /contexts/{id} answer through
// `_context_response`, which names a pair after the other member; the roster
// doors refuse a pair through `_require_group_kind` (W3,
// scripts/render_domain_w3_goldens.py). POST /people/{person_id}/dm opens a
// pair with PairKey and CanOpen (W10, scripts/render_domain_w10_goldens.py).
package direct

import (
	"slices"

	"mobile/services/core/internal/domain/friendship"
)

// Kinds of context, KIND_GROUP and KIND_PAIR.
const (
	KindGroup = "group"
	KindPair  = "pair"
)

var kinds = [...]string{KindGroup, KindPair}

// Kinds returns KINDS: every value contexts.kind may hold.
func Kinds() []string { return slices.Clone(kinds[:]) }

var rosterOnlyDoors = [...]string{
	"invite_context_member",
	"accept_context_membership",
	"leave_context",
	"set_context_member_role",
	"create_outing_invite",
	"rename_context",
}

// RosterOnlyDoors returns ROSTER_ONLY_DOORS: the service doors that answer
// not_a_group on a pair.
func RosterOnlyDoors() []string { return slices.Clone(rosterOnlyDoors[:]) }

// AnonymousCounterpart is ANONYMOUS_COUNTERPART: what a pair is called when
// nobody's name is available. A word, never an id.
const AnonymousCounterpart = "Thành viên"

// IsPair is is_pair for a str kind. The service always passes the stored
// kind, which is never None.
func IsPair(kind string) bool { return kind == KindPair }

// CounterpartOf is counterpart_of: the one member id that is not me, found
// only when exactly one entry of memberIDs differs from me. Entries equal to
// me are skipped however many there are; two entries naming the same other
// person count as two.
func CounterpartOf(memberIDs []string, me string) (string, bool) {
	other, found := "", 0
	for _, member := range memberIDs {
		if member == me {
			continue
		}
		other = member
		found++
	}
	if found != 1 {
		return "", false
	}
	return other, true
}

// DisplayNameFor is display_name_for. counterpartName nil is None; an empty
// name is falsy in Python and falls back to AnonymousCounterpart as well.
func DisplayNameFor(kind, storedName string, counterpartName *string) string {
	if !IsPair(kind) {
		return storedName
	}
	if counterpartName != nil && *counterpartName != "" {
		return *counterpartName
	}
	return AnonymousCounterpart
}

// IsKind is is_kind: a str that is one of Kinds. Any other type answers false,
// as isinstance does.
func IsKind(value any) bool {
	kind, ok := value.(string)
	return ok && slices.Contains(kinds[:], kind)
}

// PairKey is pair_key: both ids ordered by friendship.PairKey and joined by
// ":", the value uq_contexts_pair_key guards. The refusals are friendship's:
// *friendship.FriendshipError with PERSON_REQUIRED for an empty id and
// SELF_EDGE for one person twice.
func PairKey(a, b string) (string, error) {
	pair, err := friendship.PairKey(a, b)
	if err != nil {
		return "", err
	}
	return pair[0] + ":" + pair[1], nil
}

// CanOpen is can_open: a friend, who exists, whose account has not ended. The
// Python defaults are otherExists true and otherDeleted false.
func CanOpen(isFriend, otherExists, otherDeleted bool) bool {
	return isFriend && otherExists && !otherDeleted
}
