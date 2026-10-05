package chatv2http

import (
	"context"
	"net/http"
	"regexp"
)

// ADR-0057 §8.2: once a room's v2 lane is open, the legacy plaintext writers
// refuse it. Reading the legacy history stays open (read-only, labelled by
// the app); writing, deleting, reacting and marking read go through v2 alone.

// LegacyStore answers whether a room is on the v2 lane, for one member.
type LegacyStore interface {
	OnV2Lane(ctx context.Context, actor, conversation string) (member, onLane bool, err error)
}

var legacyWrite = regexp.MustCompile(`^/contexts/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/(messages|messages/[^/]+|messages/[^/]+/reactions|messages/[^/]+/reactions/[^/]+|messages/[^/]+/expense-draft|read-mark)$`)

func legacyWriteMethod(m string) bool {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodDelete || m == http.MethodPatch
}

// GuardLegacy answers a legacy chat write on a v2 room: 409
// conversation_is_e2ee for an authenticated member, never a plaintext write.
// It answers nothing (false) for anything else -- another route, a caller it
// cannot authenticate, a non-member -- and the route answers as it always has.
// When it cannot tell, it refuses with 503: fail closed, not open.
func (h *Handler) GuardLegacy(w http.ResponseWriter, r *http.Request) bool {
	if !legacyWriteMethod(r.Method) {
		return false
	}
	match := legacyWrite.FindStringSubmatch(r.URL.Path)
	if match == nil {
		return false
	}
	store, ok := h.options.Store.(LegacyStore)
	if !ok || h.options.Authenticate == nil {
		problem(w, http.StatusServiceUnavailable, "chat_v2_unavailable")
		return true
	}
	actor, err := h.options.Authenticate(r.Context(), r.Header)
	if err != nil {
		return false
	}
	member, onLane, err := store.OnV2Lane(r.Context(), actor, match[1])
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
