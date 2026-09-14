// Package problem writes error responses exactly as services/api does
// (app/api/main.py exception handlers, app/api/guest_privacy.py):
//
//   - an ApiProblem is JSONResponse({"code": ..., "detail": ...});
//   - a request validation failure is FastAPI's error list with "input"
//     removed, so a 422 names the field and rule without echoing what a person
//     typed;
//   - an unhandled failure is Starlette's plain "Internal Server Error", and
//     under /g it carries the guest privacy headers, because there the URL is
//     the credential.
//
// Bytes are checked against the real handlers through
// testdata/python_problems.json (scripts/render_problem_goldens.py).
package problem

import (
	"net/http"
	"strconv"
	"strings"

	"mobile/services/core/internal/pyjson"
)

// Problem is an ApiProblem.
type Problem struct {
	Status int
	Code   string
	Detail string
}

// GuestLinkNotFound is the one code that answers an HTML page under /g
// instead of JSON (api_problem_handler). The page belongs to the guest wave.
const GuestLinkNotFound = "guest_link_not_found"

// GuestPrivacyHeaders is GUEST_PRIVACY_HEADERS, in its declaration order.
var GuestPrivacyHeaders = [][2]string{
	{"Cache-Control", "no-store"},
	{"Referrer-Policy", "no-referrer"},
	{"X-Robots-Tag", "noindex, nofollow"},
}

// IsGuestPath is is_guest_path: "/g" itself and "/g/...", never "/goals".
func IsGuestPath(path string) bool {
	return path == "/g" || strings.HasPrefix(path, "/g/")
}

// WriteJSON answers an ApiProblem. Starlette's JSONResponse sets exactly
// content-length and content-type: application/json (no charset).
func WriteJSON(w http.ResponseWriter, p Problem) error {
	body := pyjson.NewOrderedMap()
	body.Set("code", pyjson.String(p.Code))
	body.Set("detail", pyjson.String(p.Detail))
	encoded, err := pyjson.Compact(body)
	if err != nil {
		return err
	}
	writeBody(w, p.Status, "application/json", encoded)
	return nil
}

// ValidationError is one pydantic error with "input" already removed.
// Loc holds strings and integers; Ctx is nil when pydantic gave none.
type ValidationError struct {
	Type string
	Loc  pyjson.List
	Msg  string
	Ctx  *pyjson.OrderedMap
}

// WriteValidation answers a RequestValidationError: {"detail": [...]}, each
// entry keyed type, loc, msg, then ctx when present — pydantic's order.
func WriteValidation(w http.ResponseWriter, errs []ValidationError) error {
	detail := make(pyjson.List, 0, len(errs))
	for _, e := range errs {
		entry := pyjson.NewOrderedMap()
		entry.Set("type", pyjson.String(e.Type))
		loc := e.Loc
		if loc == nil {
			loc = pyjson.List{}
		}
		entry.Set("loc", loc)
		entry.Set("msg", pyjson.String(e.Msg))
		if e.Ctx != nil {
			entry.Set("ctx", e.Ctx)
		}
		detail = append(detail, entry)
	}
	body := pyjson.NewOrderedMap()
	body.Set("detail", detail)
	encoded, err := pyjson.Compact(body)
	if err != nil {
		return err
	}
	writeBody(w, http.StatusUnprocessableEntity, "application/json", encoded)
	return nil
}

// serverErrorBody is Starlette's PlainTextResponse default.
const serverErrorBody = "Internal Server Error"

// WriteServerError answers an unhandled failure. path is the decoded request
// path (scope["path"]), the same string the guest middleware matches.
func WriteServerError(w http.ResponseWriter, path string) {
	if IsGuestPath(path) {
		for _, pair := range GuestPrivacyHeaders {
			w.Header().Set(pair[0], pair[1])
		}
	}
	writeBody(w, http.StatusInternalServerError, "text/plain; charset=utf-8", []byte(serverErrorBody))
}

func writeBody(w http.ResponseWriter, status int, contentType string, body []byte) {
	header := w.Header()
	header.Set("Content-Length", strconv.Itoa(len(body)))
	header.Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
