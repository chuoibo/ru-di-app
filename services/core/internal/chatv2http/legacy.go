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

// LegacyStore answers whether a room is on the v2 lane.
type LegacyStore interface {
	OnV2Lane(ctx context.Context, conversation string) (bool, error)
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
	Store   LegacyStore
	Resolve Resolve
}

// Guard answers every legacy chat write on a v2 room 409
// conversation_is_e2ee, whoever asks: the guard does not authenticate, so no
// difference between how it and the route read a caller (a bearer here, a dev
// X-Actor-ID there) can open the plaintext path (security review 05/10). The
// cost, accepted: anyone holding a room's id learns its lane is v2. It
// answers nothing (false) for another route or a room id the route itself
// will refuse. When it cannot tell, it refuses with 503: fail closed.
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
	if g.Store == nil {
		problem(w, http.StatusServiceUnavailable, "chat_v2_unavailable")
		return true
	}
	onLane, err := g.Store.OnV2Lane(r.Context(), room)
	if err != nil {
		problem(w, http.StatusServiceUnavailable, "chat_v2_unavailable")
		return true
	}
	if onLane {
		problem(w, http.StatusConflict, "conversation_is_e2ee")
		return true
	}
	return false
}
