package chatv2http

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
)

// LifecycleStore is the device and roster half of the transport (ADR-0057).
// Every method binds the bearer's session inside its own transaction.
type LifecycleStore interface {
	EnrollDevice(context.Context, string, []byte, chatv2.Enrollment) (chatv2.Card, error)
	RevokeDevice(context.Context, string, []byte, string) error
	ListDevices(context.Context, string, []byte) ([]chatv2.DeviceView, error)
	PublishKeyPackages(context.Context, string, []byte, string, [][]byte) (int, error)
	Roster(context.Context, string, []byte, string, string) (chatv2.RosterView, error)
	Bootstrap(context.Context, string, []byte, string, string) (chatv2.RosterView, error)
	ClaimKeyPackages(context.Context, string, []byte, string, string, []string) ([]chatv2.Claim, error)
	Commit(context.Context, string, []byte, chatv2.CommitRequest) (chatv2.CommitResult, error)
	Welcomes(context.Context, string, []byte, string) ([]chatv2.WelcomeView, error)
	AckWelcome(context.Context, string, []byte, string, string) error
	PutMedia(context.Context, string, []byte, string, string, string, []byte) ([]byte, error)
	GetMedia(context.Context, string, []byte, string, string, string) ([]byte, error)
}

const mediaTimeout = 60 * time.Second

// Matches reserves the lane's prefix on the core front door.
func Matches(path string) bool { return strings.HasPrefix(path, "/v2/chat/") || path == "/v2/chat" }

// RouteIDs names every route of the lane, for the ownership manifest.
func RouteIDs() []string {
	return append([]string{
		"POST /v2/chat/{conversation}/events",
		"GET /v2/chat/{conversation}/events",
		"PUT /v2/chat/{conversation}/marks",
		"GET /v2/chat/{conversation}/stream",
	}, lifecycleRouteIDs()...)
}

func lifecycleRouteIDs() []string {
	return []string{
		"POST /v2/chat/devices",
		"GET /v2/chat/devices",
		"DELETE /v2/chat/devices/{device}",
		"POST /v2/chat/devices/{device}/key-packages",
		"GET /v2/chat/devices/{device}/welcomes",
		"DELETE /v2/chat/devices/{device}/welcomes/{welcome}",
		"GET /v2/chat/{conversation}/roster",
		"POST /v2/chat/{conversation}/bootstrap",
		"POST /v2/chat/{conversation}/key-packages/claim",
		"POST /v2/chat/{conversation}/commits",
		"PUT /v2/chat/media/{conversation}/{media}",
		"GET /v2/chat/media/{conversation}/{media}",
	}
}

func (h *Handler) lifecycleRoutes() {
	h.mux.HandleFunc("POST /v2/chat/devices", h.enroll)
	h.mux.HandleFunc("GET /v2/chat/devices", h.devices)
	h.mux.HandleFunc("DELETE /v2/chat/devices/{device}", h.revoke)
	h.mux.HandleFunc("POST /v2/chat/devices/{device}/key-packages", h.publish)
	h.mux.HandleFunc("GET /v2/chat/devices/{device}/welcomes", h.welcomes)
	h.mux.HandleFunc("DELETE /v2/chat/devices/{device}/welcomes/{welcome}", h.ackWelcome)
	h.mux.HandleFunc("GET /v2/chat/{conversation}/roster", h.roster)
	h.mux.HandleFunc("POST /v2/chat/{conversation}/bootstrap", h.bootstrap)
	h.mux.HandleFunc("POST /v2/chat/{conversation}/key-packages/claim", h.claim)
	h.mux.HandleFunc("POST /v2/chat/{conversation}/commits", h.commit)
	h.mux.HandleFunc("PUT /v2/chat/media/{conversation}/{media}", h.putMedia)
	h.mux.HandleFunc("GET /v2/chat/media/{conversation}/{media}", h.getMedia)
}

// putMedia stores one sealed file: the raw ciphertext is the body.
func (h *Handler) putMedia(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/octet-stream" {
		problem(w, http.StatusUnsupportedMediaType, "octet_stream_required")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, chatv2.MaxMedia+64))
	if err != nil {
		problem(w, http.StatusRequestEntityTooLarge, "chat_v2_media_too_large")
		return
	}
	sum, err := store.PutMedia(r.Context(), actor, digest, r.URL.Query().Get("device_id"), r.PathValue("conversation"), r.PathValue("media"), data)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"media_id": r.PathValue("media"), "size": len(data), "sha256": sum})
}

// getMedia answers a sealed file's bytes to a member device.
func (h *Handler) getMedia(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	data, err := store.GetMedia(r.Context(), actor, digest, r.URL.Query().Get("device_id"), r.PathValue("conversation"), r.PathValue("media"))
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// lifecycle resolves the store and the bearer: the actor through the
// configured authenticator, the session digest for the transaction to hold.
func (h *Handler) lifecycle(w http.ResponseWriter, r *http.Request) (LifecycleStore, string, []byte, bool) {
	store, ok := h.options.Store.(LifecycleStore)
	if !ok {
		problem(w, http.StatusServiceUnavailable, "chat_v2_not_ready")
		return nil, "", nil, false
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return nil, "", nil, false
	}
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		fail(w, ErrAuthentication)
		return nil, "", nil, false
	}
	return store, actor, auth.TokenDigest(token), true
}

func (h *Handler) enroll(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	var e chatv2.Enrollment
	if !decode(w, r, &e) {
		return
	}
	card, err := store.EnrollDevice(r.Context(), actor, digest, e)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusCreated, card)
}

func (h *Handler) devices(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	list, err := store.ListDevices(r.Context(), actor, digest)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"devices": list, "max_devices": chatv2.MaxDevices})
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	if err := store.RevokeDevice(r.Context(), actor, digest, r.PathValue("device")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	var body struct {
		KeyPackages [][]byte `json:"key_packages"`
	}
	if !decode(w, r, &body) {
		return
	}
	n, err := store.PublishKeyPackages(r.Context(), actor, digest, r.PathValue("device"), body.KeyPackages)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]int{"available": n, "max": chatv2.MaxKeyPackages})
}

func (h *Handler) welcomes(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	list, err := store.Welcomes(r.Context(), actor, digest, r.PathValue("device"))
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"welcomes": list})
}

func (h *Handler) ackWelcome(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	if err := store.AckWelcome(r.Context(), actor, digest, r.PathValue("device"), r.PathValue("welcome")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) roster(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	v, err := store.Roster(r.Context(), actor, digest, r.URL.Query().Get("device_id"), r.PathValue("conversation"))
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, v)
}

func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	var body struct {
		DeviceID string `json:"device_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	v, err := store.Bootstrap(r.Context(), actor, digest, body.DeviceID, r.PathValue("conversation"))
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, v)
}

func (h *Handler) claim(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	var body struct {
		DeviceID string   `json:"device_id"`
		Targets  []string `json:"targets"`
	}
	if !decode(w, r, &body) {
		return
	}
	claims, err := store.ClaimKeyPackages(r.Context(), actor, digest, body.DeviceID, r.PathValue("conversation"), body.Targets)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"claims": claims})
}

func (h *Handler) commit(w http.ResponseWriter, r *http.Request) {
	store, actor, digest, ok := h.lifecycle(w, r)
	if !ok {
		return
	}
	var req chatv2.CommitRequest
	if !decodeLimit(w, r, &req, 3<<19) {
		return
	}
	if req.Envelope.ConversationID != r.PathValue("conversation") {
		fail(w, chatv2.ErrInvalid)
		return
	}
	result, err := store.Commit(r.Context(), actor, digest, req)
	if err != nil {
		fail(w, err)
		return
	}
	if !result.Replayed {
		h.Wake(req.Envelope.ConversationID)
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	write(w, status, result)
}
