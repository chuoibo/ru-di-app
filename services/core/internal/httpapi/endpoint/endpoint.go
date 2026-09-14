// Package endpoint runs one Go-served route the way FastAPI's
// get_request_handler runs an APIRoute in services/api, inside that app's unit
// of work (app/api/unit_of_work.py and deps.get_repository):
//
//  1. The body is read and pyval validates the request in FastAPI's order,
//     calling the route's dependencies as solve_dependencies reaches them.
//     get_repository stands for the request's transaction, begun on first
//     query. get_actor authenticates in the configured mode; in prod the
//     session lookup runs inside that same transaction, as Python's does.
//  2. A body pyval cannot decode is FastAPI's 400, and validation errors are
//     its 422. An ApiProblem raised by a dependency or by the route rolls the
//     transaction back and answers its JSON.
//  3. A reply is serialised first, then the transaction commits, then the
//     response is written: serialize_response runs inside the handler that
//     _wrap_route_handler commits after, and nothing is sent before the commit
//     succeeded.
//  4. Anything else is an unhandled exception. servererror answers it and the
//     deferred rollback discards the transaction, as get_repository's except
//     branch does.
//
// Route code receives a context that ignores client disconnects: Python's
// handlers run to completion when the caller goes away, and a write that
// half-happened because a phone lost signal is worse than one that finished.
package endpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/mw/servererror"
	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
)

// The Python dependency functions a Go route may depend on.
const (
	CallGetRepository = "app.api.deps.get_repository"
	CallGetActor      = "app.api.deps.get_actor"
)

// SupportedDependencies lists every dependency call this package stands in
// for. A route whose dependency tree calls anything else cannot move to Go.
var SupportedDependencies = map[string]bool{CallGetRepository: true, CallGetActor: true}

// Mode is the auth mode the Python app resolved from MOBILE_AUTH_MODE.
type Mode string

// The two auth modes (ADR-0014).
const (
	ModeDev  Mode = "dev"
	ModeProd Mode = "prod"
)

// Refusal is an ApiProblem: an expected failure with a wire-stable code.
type Refusal struct{ Problem problem.Problem }

func (r *Refusal) Error() string { return r.Problem.Code }

// Refuse returns the ApiProblem a route raises.
func Refuse(status int, code, detail string) error {
	return &Refusal{Problem: problem.Problem{Status: status, Code: code, Detail: detail}}
}

// Call is what a route's Go implementation receives.
type Call struct {
	Request *http.Request
	Scope   dispatch.Scope
	// Values are the endpoint's own validated parameters by Python name.
	Values map[string]pyval.Value
	// Actor is set when the route depends on get_actor.
	Actor *auth.Actor
	// Unit is the request's transaction; route code queries through Unit.Tx.
	Unit *db.Unit
	// Body is the request body as read, for a route that parses it by hand.
	Body []byte
	// Limits are the process's in-memory limiters (Python's app.state).
	Limits *limit.Set
	// PersonIDKey is the raw MOBILE_PERSON_ID_KEY.
	PersonIDKey string
}

// Reply is a route's answer when it does not refuse.
type Reply struct {
	// Status replaces the route's declared status_code when not zero.
	Status int
	// Body is the response model, already in field order.
	Body pyjson.Value
	// Empty answers with the status alone: no body and no content headers, as
	// a Starlette `Response(status_code=...)` does. Body is ignored.
	Empty bool
}

// Serve is a route's Go implementation: the body of the Python endpoint.
type Serve func(ctx context.Context, call *Call) (Reply, error)

// Env is what every route shares.
type Env struct {
	Mode Mode
	// NewUnit returns a fresh, unbegun unit for one request.
	NewUnit func() *db.Unit
	// Now is the clock a prod session's expiry is checked against.
	Now func() time.Time
	// Limits are shared by every request the process serves.
	Limits *limit.Set
	// PersonIDKey is MOBILE_PERSON_ID_KEY as the process started with it.
	// Python reads os.environ on every call; nothing in the app changes it, so
	// one read at startup gives the same answers.
	PersonIDKey string
}

// New builds the handler for one route. status is the route decorator's
// status_code.
func New(route *pyval.Route, status int, serve Serve, env Env) (http.Handler, error) {
	switch {
	case route == nil || serve == nil:
		return nil, errors.New("endpoint: a route needs its pyval binding and its Go implementation")
	case env.Mode != ModeDev && env.Mode != ModeProd:
		return nil, fmt.Errorf("endpoint: auth mode %q", env.Mode)
	case env.NewUnit == nil || env.Now == nil:
		return nil, errors.New("endpoint: Env needs NewUnit and Now")
	case status < 100 || status > 599:
		return nil, fmt.Errorf("endpoint: status %d", status)
	}
	return &handler{route: route, status: status, serve: serve, env: env}, nil
}

type handler struct {
	route  *pyval.Route
	status int
	serve  Serve
	env    Env
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scope, ok := dispatch.FromRequest(r)
	if !ok {
		servererror.Raise(fmt.Errorf("endpoint %s: request did not come through dispatch", h.route.ID))
	}
	ctx := context.WithoutCancel(r.Context())
	body, err := io.ReadAll(r.Body)
	if err != nil {
		servererror.Raise(err)
	}
	unit := h.env.NewUnit()
	// A committed unit ignores this; every other path discards its writes.
	defer func() { _ = unit.Rollback(ctx) }()

	call := &Call{Request: r, Scope: scope, Unit: unit, Body: body, Limits: h.env.Limits, PersonIDKey: h.env.PersonIDKey}
	hook := func(dependency pyval.Dependency) error {
		switch dependency.Call {
		case CallGetRepository:
			return nil
		case CallGetActor:
			actor, refused, err := h.actor(ctx, r, unit)
			if err != nil {
				return err
			}
			if refused != nil {
				return &Refusal{Problem: problem.Problem{Status: refused.Status, Code: refused.Code, Detail: refused.Detail}}
			}
			call.Actor = actor
			return nil
		default:
			return fmt.Errorf("endpoint %s: no Go dependency for %s", h.route.ID, dependency.Call)
		}
	}
	_, rawQuery := router.SplitTarget(r.RequestURI)
	result, err := h.route.Validate(&pyval.Request{
		PathParams: scope.Params,
		RawQuery:   rawQuery,
		Headers:    headerPairs(r.Header),
		Body:       body,
	}, hook)
	var refusal *Refusal
	var bodyError *pyval.BodyError
	switch {
	case errors.As(err, &bodyError):
		_ = unit.Rollback(ctx)
		_ = pyval.WriteBodyError(w)
		return
	case errors.As(err, &refusal):
		_ = unit.Rollback(ctx)
		_ = problem.WriteJSON(w, refusal.Problem)
		return
	case err != nil:
		servererror.Raise(err)
	case len(result.Errors) > 0:
		_ = unit.Rollback(ctx)
		_ = problem.WriteValidation(w, result.Errors)
		return
	}

	call.Values = result.Values
	reply, err := h.serve(ctx, call)
	if errors.As(err, &refusal) {
		_ = unit.Rollback(ctx)
		_ = problem.WriteJSON(w, refusal.Problem)
		return
	}
	if err != nil {
		servererror.Raise(err)
	}
	if reply.Empty {
		if err := unit.Commit(ctx); err != nil {
			servererror.Raise(err)
		}
		status := reply.Status
		if status == 0 {
			status = h.status
		}
		w.WriteHeader(status)
		return
	}
	encoded, err := pyjson.Compact(reply.Body)
	if err != nil {
		servererror.Raise(err)
	}
	if err := unit.Commit(ctx); err != nil {
		servererror.Raise(err)
	}
	status := reply.Status
	if status == 0 {
		status = h.status
	}
	header := w.Header()
	header.Set("Content-Length", strconv.Itoa(len(encoded)))
	header.Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func (h *handler) actor(ctx context.Context, r *http.Request, unit *db.Unit) (*auth.Actor, *auth.Problem, error) {
	if h.env.Mode == ModeDev {
		actor, refused := auth.DevActor(r.Header)
		return actor, refused, nil
	}
	return auth.ProdActor(ctx, r.Header, lazySessions{unit: unit}, h.env.Now())
}

// lazySessions opens the request's transaction only when a lookup runs, so a
// request refused for a missing bearer never begins one, as in Python.
type lazySessions struct{ unit *db.Unit }

func (s lazySessions) SessionByDigest(ctx context.Context, digest []byte) (*auth.SessionRecord, error) {
	tx, err := s.unit.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return repo.Sessions{Q: tx}.SessionByDigest(ctx, digest)
}

func (s lazySessions) Grants(ctx context.Context, personID string) (auth.Grants, error) {
	tx, err := s.unit.Tx(ctx)
	if err != nil {
		return auth.Grants{}, err
	}
	return repo.Sessions{Q: tx}.Grants(ctx, personID)
}

// headerPairs flattens the request headers for pyval. net/http keeps each
// name's values in wire order but not the order between names, which pyval's
// first-value and all-values lookups never read.
func headerPairs(header http.Header) [][2]string {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)
	var pairs [][2]string
	for _, name := range names {
		for _, value := range header[name] {
			pairs = append(pairs, [2]string{name, value})
		}
	}
	return pairs
}
