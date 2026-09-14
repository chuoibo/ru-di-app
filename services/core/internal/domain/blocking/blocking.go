// Package blocking ports app.domain.blocking (ADR-0023 §2.3): what a block
// means wherever it means something. A block is the `blocked` state of the one
// friend edge a pair shares, so every function reads that edge.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image.
package blocking

// DirectMessageUnavailable is DIRECT_MESSAGE_UNAVAILABLE: the one code both
// «blocked» and «the other person deleted their account» answer with.
const DirectMessageUnavailable = "direct_message_unavailable"

// Edge is the dict the service builds for this module (`_friend_edge_dict`):
// the edge's state and who decided it. A nil *Edge is None; DecidedByID nil is
// None or an absent key, which `edge.get` reads alike.
type Edge struct {
	State       string
	DecidedByID *string
}

// IsBlocked is is_blocked.
func IsBlocked(edge *Edge) bool {
	return edge != nil && edge.State == "blocked"
}

// BlockerOf is blocker_of: decided_by_id of a blocked edge, else nil.
func BlockerOf(edge *Edge) *string {
	if !IsBlocked(edge) {
		return nil
	}
	return edge.DecidedByID
}

// HiddenBetween is hidden_between: symmetric, the same answer as IsBlocked.
func HiddenBetween(edge *Edge) bool {
	return IsBlocked(edge)
}

// DMAllowed is dm_allowed.
func DMAllowed(edge *Edge, otherDeleted bool) bool {
	return !IsBlocked(edge) && !otherDeleted
}
