package routes

import (
	"context"
	"fmt"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/authsteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyval"
)

// The four routes of app/api/routes/sessions.py. POST has no actor: it is
// where an identity is obtained. DELETE /sessions/current has no get_actor
// either; the bearer is the only credential, and a dead one still answers 204.

func createSession() Route {
	return Route{ID: "POST /sessions", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		model, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		token, err := stringField(model, "invite_token")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := newAuthStore(ctx, call)
		view, err := authsteps.BootstrapSessionFromInvite(store, processSecrets{}, token, authNow())
		if err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Body: wireSession(view)}, nil
	}}
}

func listSessions() Route {
	return Route{ID: "GET /sessions", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		header, err := optionalAuthorization(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		var current *string
		if header != nil {
			token, err := bearerFromValue(*header)
			if err != nil {
				return endpoint.Reply{}, err
			}
			current = &token
		}
		store := newAuthStore(ctx, call)
		view, err := authsteps.ListAccountSessions(store, processSecrets{}, authActor(call), current, authNow())
		if err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Body: wireSessionList(view)}, nil
	}}
}

func revokeCurrentSession() Route {
	return Route{ID: "DELETE /sessions/current", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		token, problem := auth.BearerToken(call.Request.Header)
		if problem != nil {
			return endpoint.Reply{}, endpoint.Refuse(problem.Status, problem.Code, problem.Detail)
		}
		store := newAuthStore(ctx, call)
		if err := authsteps.RevokeSessionToken(store, processSecrets{}, token, authNow()); err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

func revokeSession() Route {
	return Route{ID: "DELETE /sessions/{session_id}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		id, ok := call.Values["session_id"].(pyval.UUID)
		if !ok {
			return endpoint.Reply{}, fmt.Errorf("routes: session_id is %T, not a UUID", call.Values["session_id"])
		}
		store := newAuthStore(ctx, call)
		if err := authsteps.RevokeAccountSession(store, id.String(), authActor(call), authNow()); err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}
