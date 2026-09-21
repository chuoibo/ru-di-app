// Package dispatch makes the front door's first decision: does core serve
// this request, or does Python?
//
// Core serves a request only when Starlette's router would FULL-match it to a
// route the manifest gives Go (after MOBILE_FORCE_PYTHON). Everything else goes
// to Python unchanged: 404, 405, the trailing-slash 307, CORS preflights,
// request lines llhttp refuses, targets the router does not emulate, and every
// route Python still owns. While Python runs, those answers are Python's to
// give, and forwarding them keeps them byte-identical.
//
// A Go-served request runs inside the layers that wrap every route in
// app/api/main.py, from the outside in: ServerErrorMiddleware with the guest
// aware Exception handler (servererror), CORS (install_cors, added last and so
// outermost of the app's middleware), GuestPrivacyHeadersMiddleware (guest),
// IdempotencyMiddleware (Options.Idempotency, package idem), then the route
// handler.
package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/mw/guest"
	"mobile/services/core/internal/httpapi/mw/servererror"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/ownership"
)

// Scope is the router's decision, handed to the route handler.
type Scope struct {
	RouteID string
	// Path is scope["path"]: the raw path percent-decoded once.
	Path string
	// Params are the decoded path parameters, before any type validation.
	Params map[string]string
}

type scopeKey struct{}

// FromRequest returns the Scope dispatch attached to a Go-served request.
func FromRequest(r *http.Request) (Scope, bool) {
	scope, ok := r.Context().Value(scopeKey{}).(Scope)
	return scope, ok
}

// Options configures New.
type Options struct {
	// Router holds every manifest route in registration order, Python's
	// included: a Python route declared earlier must still win.
	Router *router.Router
	// Served are the routes Go serves after MOBILE_FORCE_PYTHON.
	Served []ownership.Route
	// Handlers maps a manifest route id to its Go handler.
	Handlers map[string]http.Handler
	// Python forwards a request to the Python API.
	Python http.Handler
	CORS   *cors.Policy
	Logger *slog.Logger
	// Idempotency wraps every Go route handler, inside the guest layer as in
	// Python. Required whenever Go serves a route: Python's layer sees every
	// request, and a Go route without it would take a key and never replay.
	Idempotency func(http.Handler) http.Handler
}

// New builds the front door. It refuses a served route without a handler.
func New(o Options) (http.Handler, error) {
	if o.Router == nil || o.Python == nil || o.CORS == nil || o.Logger == nil {
		return nil, fmt.Errorf("dispatch: Router, Python, CORS and Logger are required")
	}
	if len(o.Served) > 0 && o.Idempotency == nil {
		return nil, fmt.Errorf("dispatch: Go serves %d route(s) but no idempotency layer was given", len(o.Served))
	}
	chains := make(map[string]http.Handler, len(o.Served))
	for _, route := range o.Served {
		handler := o.Handlers[route.ID]
		if handler == nil {
			return nil, fmt.Errorf("dispatch: manifest gives Go %q but there is no handler for it", route.ID)
		}
		chains[route.ID] = servererror.Middleware(o.Logger, scopePath,
			o.CORS.Middleware(guest.Middleware(scopePath, o.Idempotency(handler))))
	}
	return &front{router: o.Router, python: o.Python, chains: chains}, nil
}

type front struct {
	router *router.Router
	python http.Handler
	chains map[string]http.Handler
}

func (f *front) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawPath, _ := router.SplitTarget(r.RequestURI)
	path := router.ScopePath(rawPath)
	if isInternalPath(path) {
		// The public core never proxies /internal to Python: the brain is
		// reached only on the backend network, gated by X-Internal-Token.
		writePublicNotFound(w)
		return
	}
	if len(f.chains) == 0 || isPreflight(r) {
		f.python.ServeHTTP(w, r)
		return
	}
	// The host and scheme only shape a redirect's Location, and a redirect is
	// never served here.
	decision := f.router.DecideTarget(r.Method, r.RequestURI, r.Host, "http")
	chain, served := f.chains[decision.RouteID]
	if decision.Kind != router.KindFull || !served {
		f.python.ServeHTTP(w, r)
		return
	}
	scope := Scope{RouteID: decision.RouteID, Path: path, Params: decision.Params}
	chain.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scopeKey{}, scope)))
}

// isInternalPath is the public door's refusal of the brain seam: the exact
// prefix and every path under it, including a trailing-slash-less /internal.
func isInternalPath(path string) bool {
	return path == "/internal" || strings.HasPrefix(path, "/internal/")
}

// writePublicNotFound is FastAPI's JSON 404 for a path the public app has
// never registered: {"detail":"Not Found"}.
func writePublicNotFound(w http.ResponseWriter) {
	body := []byte(`{"detail":"Not Found"}`)
	header := w.Header()
	header.Set("Content-Length", strconv.Itoa(len(body)))
	header.Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(body)
}

// isPreflight is Starlette CORSMiddleware's test. Python's CORS layer answers
// a preflight before any route is reached, so it is never a route's request.
func isPreflight(r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	_, origin := r.Header["Origin"]
	_, requested := r.Header["Access-Control-Request-Method"]
	return origin && requested
}

func scopePath(r *http.Request) string {
	if scope, ok := FromRequest(r); ok {
		return scope.Path
	}
	rawPath, _ := router.SplitTarget(r.RequestURI)
	return router.ScopePath(rawPath)
}
