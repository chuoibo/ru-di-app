package routes

import (
	"context"
	"fmt"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// listFriendRequests is GET /people/{person_id}/friend-requests
// (routes/friends.py list_friend_requests, ApiService.list_friend_requests):
// pending requests on one side of the edge. The route reads direction as
// "outgoing" only when it says exactly that, so an unknown value gives the
// narrower incoming list, and it does so before the permission check.
func listFriendRequests() Route {
	return Route{ID: "GET /people/{person_id}/friend-requests", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		raw, err := stringParam(call, "direction")
		if err != nil {
			return endpoint.Reply{}, err
		}
		direction := "incoming"
		if raw == "outgoing" {
			direction = "outgoing"
		}
		if err := requireOwnFriends(call, personID); err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		edges, err := repo.Repository{Q: tx}.ListFriendRequests(ctx, personID, direction)
		if err != nil {
			return endpoint.Reply{}, err
		}
		requests := pyjson.List{}
		for _, edge := range edges {
			requests = append(requests, wireFriendEdge(edge))
		}
		body := pyjson.NewOrderedMap()
		body.Set("requests", requests)
		return endpoint.Reply{Body: body}, nil
	}}
}

// listFriends is GET /people/{person_id}/friends (ApiService.list_friends).
func listFriends() Route {
	return Route{ID: "GET /people/{person_id}/friends", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireOwnFriends(call, personID); err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		edges, err := repo.Repository{Q: tx}.ListFriends(ctx, personID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		friends := pyjson.List{}
		for _, edge := range edges {
			// FriendSummary.friends_since is `decided_at or created_at`.
			since := edge.CreatedAt
			if edge.DecidedAt != nil {
				since = *edge.DecidedAt
			}
			item := pyjson.NewOrderedMap()
			item.Set("person_id", pyjson.String(edge.OtherPersonID))
			item.Set("display_name", pyjson.String(edge.OtherDisplayName))
			item.Set("friends_since", pyjson.String(pyjson.DateTime(since.UTC())))
			friends = append(friends, item)
		}
		body := pyjson.NewOrderedMap()
		body.Set("friends", friends)
		return endpoint.Reply{Body: body}, nil
	}}
}

// requireOwnFriends is `_require_permission("view_own_friends", actor,
// {"is_self": actor.id == person_id})`.
func requireOwnFriends(call *endpoint.Call, personID string) error {
	refused, err := service.RequirePermission("view_own_friends", *call.Actor,
		service.Resource{Proven: map[string]bool{"is_self": call.Actor.ID == personID}})
	if err != nil {
		return err
	}
	if refused != nil {
		return &endpoint.Refusal{Problem: *refused}
	}
	return nil
}

// wireFriendEdge is _wire_friend_edge: FriendRequestResponse. The database
// runs in UTC, so psycopg hands Python aware UTC values and pydantic writes "Z".
func wireFriendEdge(edge repo.FriendEdge) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(edge.ID))
	out.Set("requester_id", pyjson.String(edge.RequesterID))
	out.Set("addressee_id", pyjson.String(edge.AddresseeID))
	out.Set("other_person_id", pyjson.String(edge.OtherPersonID))
	out.Set("other_display_name", pyjson.String(edge.OtherDisplayName))
	out.Set("state", pyjson.String(edge.State))
	out.Set("created_at", pyjson.String(pyjson.DateTime(edge.CreatedAt.UTC())))
	if edge.DecidedAt == nil {
		out.Set("decided_at", pyjson.Null{})
	} else {
		out.Set("decided_at", pyjson.String(pyjson.DateTime(edge.DecidedAt.UTC())))
	}
	return out
}

// stringParam reads a path or query string pyval validated, its default
// already applied.
func stringParam(call *endpoint.Call, name string) (string, error) {
	text, ok := call.Values[name].(pyjson.String)
	if !ok {
		return "", fmt.Errorf("routes: parameter %q is %T, not a string", name, call.Values[name])
	}
	return string(text), nil
}
