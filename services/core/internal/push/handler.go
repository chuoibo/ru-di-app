package push

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"mobile/services/core/internal/auth"
)

// RouteIDs names the routes for the ownership manifest.
func RouteIDs() []string {
	return []string{"PUT /push/devices/{installation_id}", "DELETE /push/devices/{installation_id}"}
}

// Matches reserves /push/devices/{installation_id}.
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) == 3 && p[0] == "push" && p[1] == "devices"
}

// Handler serves registration; bearer sessions only.
type Handler struct{ Store Store }

func answer(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !Matches(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	installation := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[2]
	token, problem := auth.BearerToken(r.Header)
	if problem != nil {
		answer(w, 401, map[string]string{"code": "authentication_required"})
		return
	}
	digest := auth.TokenDigest(token)
	var err error
	switch r.Method {
	case http.MethodPut:
		var body struct {
			Platform string `json:"platform"`
			Token    string `json:"expo_push_token"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
			answer(w, 422, map[string]string{"code": "push_invalid"})
			return
		}
		var id string
		id, err = h.Store.Register(r.Context(), digest, Registration{InstallationID: installation, Platform: body.Platform, Token: body.Token})
		if err == nil {
			answer(w, 200, map[string]string{"id": id})
			return
		}
	case http.MethodDelete:
		if err = h.Store.Unregister(r.Context(), digest, installation); err == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		answer(w, 405, map[string]string{"code": "method_not_allowed"})
		return
	}
	switch {
	case errors.Is(err, ErrInvalid):
		answer(w, 422, map[string]string{"code": "push_invalid"})
	case errors.Is(err, ErrForbidden):
		answer(w, 401, map[string]string{"code": "authentication_required"})
	case errors.Is(err, ErrConflict):
		answer(w, 409, map[string]string{"code": "push_token_taken"})
	default:
		answer(w, 503, map[string]string{"code": "push_unavailable"})
	}
}
