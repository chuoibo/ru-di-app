package routes

import (
	"context"
	"net/http"
	"strings"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/idem"
)

// authorizeChatReplay runs inside the endpoint pipeline, after session auth.
// A cached answer is not a capability to bypass current conversation access.
func authorizeChatReplay(next endpoint.Serve) endpoint.Serve {
	return func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		stored, replay := idem.AuthorizedReplay(call.Request.Context())
		if !replay || !idem.ChatReplayNeedsAuthorization(call.Request.Method, call.Request.URL.Path) {
			return next(ctx, call)
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_messages", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requirePairAlive(ctx, store, call, contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if _, hasMessage := call.Values["message_id"]; hasMessage {
			messageID, err := pathUUID(call, "message_id")
			if err != nil {
				return endpoint.Reply{}, err
			}
			message, err := messageInContext(ctx, store, contextID, messageID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			isDelete := call.Request.Method == http.MethodDelete && !strings.Contains(call.Request.URL.Path, "/reactions/")
			if isDelete {
				author := message.AuthorID != nil && *message.AuthorID == call.Actor.ID
				if err := requireFacts(call, "delete_own_message", map[string]bool{"is_group_member": true, "is_author": author}); err != nil {
					return endpoint.Reply{}, err
				}
			} else if message.Kind == "deleted" {
				return endpoint.Reply{}, endpoint.Refuse(409, "message_deleted", "Tin nhắn đã bị xoá.")
			}
		}
		return itineraryReplay(stored), nil
	}
}
