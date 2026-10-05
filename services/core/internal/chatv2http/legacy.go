package chatv2http

import (
	"context"
	"net/http"

	"mobile/services/core/internal/auth"
)

// ADR-0057 §8.2: once a room's v2 lane is open, the legacy plaintext writers
// refuse it. Reading the legacy history stays open (read-only, labelled by
// the app); writing, deleting, reacting and marking read go through v2 alone.
// The guard stands whenever the chat v2 schema exists, the lane's flag on or
// off: turning the lane off is a rollback of new v2 sends, never a return to
// plaintext in an encrypted room.

// LegacyStore answers whether a room is on the v2 lane, for one member.
type LegacyStore interface {
	OnV2Lane(ctx context.Context, actor, conversation string) (member, onLane bool, err error)
}

// Resolve is the front door's own routing decision for a request: the
// manifest route id it dispatches to and its path parameters. The guard asks
// it instead of parsing paths itself, so it can never read a path otherwise
// than the route will (security review 05/10: an upper-case or unhyphenated
// room id once slipped past a guard with its own pattern).
type Resolve func(r *http.Request) (routeID string, params map[string]string, ok bool)

// legacyWrites are the manifest ids of the legacy chat writers.
var legacyWrites = map[string]bool{
	"POST /contexts/{context_id}/messages":                                 true,
	"DELETE /contexts/{context_id}/messages/{message_id}":                  true,
	"POST /contexts/{context_id}/messages/{message_id}/reactions":          true,
	"DELETE /contexts/{context_id}/messages/{message_id}/reactions/{kind}": true,
	"POST /contexts/{context_id}/messages/{message_id}/expense-draft":      true,
	"PUT /contexts/{context_id}/read-mark":                                 true,
}

// LegacyGuard refuses legacy chat writes to v2 rooms.
type LegacyGuard struct {
	Store        LegacyStore
	Authenticate Authenticate
	Resolve      Resolve
}

// Guard answers a legacy chat write on a v2 room: 409 conversation_is_e2ee
// for an authenticated member, never a plaintext write. It answers nothing
// (false) for anything else -- another route, a room id the route itself will
// refuse, a caller it cannot authenticate, a non-member -- and the route
// answers as it always has. When it cannot tell, it refuses with 503: fail
// closed, not open.
func (g LegacyGuard) Guard(w http.ResponseWriter, r *http.Request) bool {
	if g.Resolve == nil {
		return false
	}
	route, params, ok := g.Resolve(r)
	if !ok || !legacyWrites[route] {
		return false
	}
	room, err := auth.ParsePythonUUID(params["context_id"])
	if err != nil {
		return false
	}
	if g.Store == nil || g.Authenticate == nil {
		problem(w, http.StatusServiceUnavailable, "chat_v2_unavailable")
		return true
	}
	actor, err := g.Authenticate(r.Context(), r.Header)
	if err != nil {
		return false
	}
	member, onLane, err := g.Store.OnV2Lane(r.Context(), actor, room)
	if err != nil {
		problem(w, http.StatusServiceUnavailable, "chat_v2_unavailable")
		return true
	}
	if member && onLane {
		problem(w, http.StatusConflict, "conversation_is_e2ee")
		return true
	}
	return false
}
