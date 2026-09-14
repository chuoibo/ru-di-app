package routes

import (
	"context"
	"errors"
	"strings"
	"time"

	"mobile/services/core/internal/domain/friendship"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/repo"
)

// sendFriendRequest is POST /friends/requests (routes/friends.py
// send_friend_request, ApiService.send_friend_request): permission, then the
// addressee, then the domain on the pair's live edge, then the insert.
func sendFriendRequest() Route {
	return Route{ID: "POST /friends/requests", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "body")
		if err != nil {
			return endpoint.Reply{}, err
		}
		addresseeID, err := uuidField(body, "addressee_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "send_friend_request", map[string]bool{"is_not_self": call.Actor.ID != addresseeID}); err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		person, err := store.GetPerson(ctx, addresseeID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if person == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "person_not_found", "Chưa có ai mang danh tính này.")
		}
		existing, err := store.GetFriendEdge(ctx, call.Actor.ID, addresseeID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		var existingEdge *friendship.Edge
		if existing != nil {
			existingEdge = &friendship.Edge{State: existing.State}
		}
		if _, err := friendship.OpenRequest(call.Actor.ID, addresseeID, existingEdge); err != nil {
			var refused *friendship.FriendshipError
			if errors.As(err, &refused) {
				return endpoint.Reply{}, friendRefusal(refused.Code)
			}
			return endpoint.Reply{}, err
		}
		record, err := store.OpenFriendRequest(ctx, call.Actor.ID, addresseeID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			// The race arm of the same refusal, with the same code.
			return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(friendship.BlockedIsSilent), "Chưa gửi được lời mời này.")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireFriendEdge(record)}, nil
	}}
}

// respondToFriendRequest is POST /friends/requests/{request_id}/respond
// (respond_to_friend_request): the request as this reader sees it, then
// is_invitee (either party may block, only the addressee may accept or
// decline), then the domain, then the locked write, whose refusals answer with
// the codes the domain would have given on the fresher read.
func respondToFriendRequest() Route {
	return Route{ID: "POST /friends/requests/{request_id}/respond", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		requestID, err := pathUUID(call, "request_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "body")
		if err != nil {
			return endpoint.Reply{}, err
		}
		decision, err := stringField(body, "decision")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		notFound := endpoint.Refuse(404, "friend_request_not_found", "Không có lời mời này.")
		edge, err := store.GetFriendRequest(ctx, requestID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if edge == nil {
			return endpoint.Reply{}, notFound
		}
		isInvitee := call.Actor.ID == edge.AddresseeID
		if decision == friendship.DecisionBlock {
			isInvitee = call.Actor.ID == edge.RequesterID || call.Actor.ID == edge.AddresseeID
		}
		if err := requireFacts(call, "respond_to_friend_request", map[string]bool{"is_invitee": isInvitee}); err != nil {
			return endpoint.Reply{}, err
		}
		decided, err := friendship.Decide(friendship.Edge{
			RequesterID: edge.RequesterID, AddresseeID: edge.AddresseeID, State: edge.State,
		}, call.Actor.ID, decision)
		if err != nil {
			var refused *friendship.FriendshipError
			if errors.As(err, &refused) {
				return endpoint.Reply{}, friendRefusal(refused.Code)
			}
			return endpoint.Reply{}, err
		}
		record, err := store.DecideFriendRequest(ctx, requestID, decided.State, call.Actor.ID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			if conflict.Code == "FRIEND_EDGE_EXISTS" {
				return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(friendship.BlockedIsSilent), "Chưa trả lời được lời mời này.")
			}
			return endpoint.Reply{}, friendRefusal(conflict.Code)
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, notFound
		}
		return endpoint.Reply{Body: wireFriendEdge(*record)}, nil
	}}
}

// friendRefusal is ApiService._friend_refusal: a domain code as an answer that
// does not narrate the graph.
func friendRefusal(code string) error {
	switch code {
	case friendship.BlockedIsSilent:
		return endpoint.Refuse(409, strings.ToLower(code), "Chưa gửi được lời mời này.")
	case friendship.CodeSelfEdge:
		return endpoint.Refuse(422, "self_edge", "Không tự kết bạn với chính mình được.")
	case friendship.CodeOnlyAddresseeMayAnswer, friendship.CodeNotAParty:
		return endpoint.Refuse(403, "permission_denied", strings.ToLower(code))
	}
	return endpoint.Refuse(409, strings.ToLower(code), "Lời mời không ở trạng thái đó.")
}
