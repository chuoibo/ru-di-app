// Package direct is the part of app/domain/direct.py the W3 context routes
// reach (ADR-0021 §2.5): whether a context is a pair, who the other person of
// a pair is, and what the reader sees as its name.
//
// POST /contexts, PATCH /contexts/{id} and GET /contexts/{id} answer through
// `_context_response`, which names a pair after the other member; the roster
// doors refuse a pair through `_require_group_kind`. pair_key, can_open,
// is_kind, KINDS and ROSTER_ONLY_DOORS belong to the direct-message routes and
// are not ported here.
package direct

// Kinds of context, KIND_GROUP and KIND_PAIR.
const (
	KindGroup = "group"
	KindPair  = "pair"
)

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
