package chatv2http

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
)

// ClockStore is optional so transports without this read capability retain
// their existing Store contract. The bearer is checked inside the read tx.
type ClockStore interface {
	Clock(context.Context, string, []byte, string, string) (time.Time, error)
}

func (h *Handler) clock(w http.ResponseWriter, r *http.Request) {
	store, ok := h.options.Store.(ClockStore)
	if !ok {
		problem(w, http.StatusServiceUnavailable, "chat_v2_not_ready")
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		fail(w, ErrAuthentication)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["device_id"]) != 1 {
		problem(w, http.StatusBadRequest, "invalid_query")
		return
	}
	device, conversation := query.Get("device_id"), r.PathValue("conversation")
	if !chatv2.ValidID(device) || !chatv2.ValidID(conversation) {
		fail(w, chatv2.ErrInvalid)
		return
	}
	// A clock sample has no request document, including for chunked bodies.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
	if err != nil || len(body) != 0 {
		problem(w, http.StatusBadRequest, "invalid_body")
		return
	}
	now, err := store.Clock(r.Context(), actor, auth.TokenDigest(token), device, conversation)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]string{"server_now": now.UTC().Format(time.RFC3339Nano)})
}
