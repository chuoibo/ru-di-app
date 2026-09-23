package routes

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/cursors"
	"mobile/services/core/internal/domain/chatintent"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/domain/messageedit"
	"mobile/services/core/internal/domain/stickers"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
)

const (
	contextWindow           = 40
	chiaBillWindow          = 20
	chiaBillModelCalls      = 8
	chatExpenseNoText       = "Tin nhắn không mô tả một khoản chi."
	chatUnreadableDetail    = "Không đọc được khoản chi từ tin nhắn. Hãy kiểm tra lại nội dung."
	modelNamedPersonDetail  = "AI đã cố nêu người trả hoặc người tham gia; bản nháp bị từ chối để danh tính chỉ được đọc từ dữ liệu nhóm."
	chatReaderUnavailable   = "Không đọc được khoản chi từ tin nhắn lúc này, thử lại sau."
	chatReaderNotConfigured = "Máy chủ chưa cấu hình khoá đọc khoản chi từ tin nhắn. Đây là lỗi cấu hình phía máy chủ; sửa lại tin nhắn không giúp được."
)

func postContextMessage() Route {
	return Route{ID: "POST /contexts/{context_id}/messages", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := stringField(request, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := optionalStringField(request, "body")
		if err != nil {
			return endpoint.Reply{}, err
		}
		imageURL, err := optionalStringField(request, "image_url")
		if err != nil {
			return endpoint.Reply{}, err
		}
		card, err := optionalObjectField(request, "card")
		if err != nil {
			return endpoint.Reply{}, err
		}
		replyToID, err := optionalUUIDField(request, "reply_to_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "post_group_message", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requirePairAlive(ctx, store, call, contextID); err != nil {
			return endpoint.Reply{}, err
		}
		valid := (kind == "text" && body != nil && imageURL == nil && card == nil) ||
			(kind == "image" && imageURL != nil && card == nil) ||
			(kind == "ai_card" && card != nil && imageURL == nil && body == nil) ||
			(kind == "sticker" && body != nil && imageURL == nil && card == nil)
		if !valid {
			return endpoint.Reply{}, endpoint.Refuse(422, "message_payload_invalid", "Message payload does not match its kind")
		}
		if kind == "sticker" && !stickers.Is(*body) {
			return endpoint.Reply{}, endpoint.Refuse(422, "sticker_unknown", "Sticker này không có trong bộ của Rủ Đi.")
		}
		if replyToID != nil && kind == "ai_card" {
			return endpoint.Reply{}, endpoint.Refuse(422, "message_payload_invalid", "Message payload does not match its kind")
		}
		var replyPreview *pyjson.OrderedMap
		if replyToID != nil {
			target, err := store.GetMessage(ctx, *replyToID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			var facts *messageedit.Facts
			if target != nil {
				f := messageFacts(*target)
				facts = &f
			}
			if err := messageedit.CheckReplyTarget(facts, contextID); err != nil {
				var refused *messageedit.Error
				if errors.As(err, &refused) {
					switch refused.Code {
					case "REPLY_NOT_FOUND":
						return endpoint.Reply{}, endpoint.Refuse(404, "message_not_found", "Message does not exist")
					case "REPLY_TO_DELETED":
						return endpoint.Reply{}, endpoint.Refuse(409, "reply_target_deleted", "Tin bạn muốn trả lời đã bị xoá.")
					default:
						return endpoint.Reply{}, endpoint.Refuse(422, "reply_target_not_quotable", "Không trả lời được một thẻ; hãy trả lời một tin nhắn.")
					}
				}
				return endpoint.Reply{}, err
			}
			replyPreview = wireReplyPreview(*target)
		}
		if err := requirePhotoURLContext(contextID, imageURL); err != nil {
			return endpoint.Reply{}, err
		}
		var storedCard pyjson.Value
		if kind == "ai_card" {
			group, err := service.GroupTaste(ctx, store, contextID, time.Now().UTC())
			if err != nil {
				return endpoint.Reply{}, err
			}
			places, err := service.ModelPlaceRows(ctx, store, group)
			if err != nil {
				return endpoint.Reply{}, err
			}
			grounded, err := companion.GroundCard(treejson.To(card), treejson.MapsTo(places))
			if err != nil {
				return endpoint.Reply{}, endpoint.Refuse(422, "card_ungrounded", "Thẻ không hợp lệ: loại thẻ lạ hoặc nêu địa điểm không có trong danh mục.")
			}
			storedCard = treejson.From(grounded)
		} else if card != nil {
			storedCard = card
		}
		rawCard, err := cardBytes(storedCard)
		if err != nil {
			return endpoint.Reply{}, err
		}
		author := call.Actor.ID
		record, err := store.CreateMessage(ctx, repo.MessageInput{
			ContextID: contextID, AuthorID: &author, Kind: kind, Body: body,
			ImageURL: imageURL, Card: rawCard, Now: time.Now().UTC(), ReplyToID: replyToID,
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		posted := wireMessage(record, nil, replyPreview)
		acted, err := actOnMessageIntent(ctx, call, store, contextID, posted, record)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: acted}, nil
	}}
}

func listContextMessages() Route {
	return Route{ID: "GET /contexts/{context_id}/messages", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		limit, err := intParam(call, "limit")
		if err != nil {
			return endpoint.Reply{}, err
		}
		beforeRaw, err := optionalStringParam(call, "before")
		if err != nil {
			return endpoint.Reply{}, err
		}
		afterRaw, err := optionalStringParam(call, "after")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if beforeRaw != nil && afterRaw != nil {
			return endpoint.Reply{}, endpoint.Refuse(422, "cursor_direction_ambiguous", "Use either before or after, not both")
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_messages", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		before, err := decodeMessageCursor(beforeRaw)
		if err != nil {
			return endpoint.Reply{}, err
		}
		after, err := decodeMessageCursor(afterRaw)
		if err != nil {
			return endpoint.Reply{}, err
		}
		page, err := store.ListMessages(ctx, contextID, limit, before, after)
		if err != nil {
			return endpoint.Reply{}, err
		}
		ids := make([]string, len(page.Messages))
		quotedIDs := []string{}
		for i, record := range page.Messages {
			ids[i] = record.ID
			if record.ReplyToID != nil {
				quotedIDs = append(quotedIDs, *record.ReplyToID)
			}
		}
		reactions, err := store.ListReactions(ctx, ids)
		if err != nil {
			return endpoint.Reply{}, err
		}
		summaries := summariseReactions(reactions, call.Actor.ID)
		quoted := map[string]repo.Message{}
		if len(quotedIDs) > 0 {
			quoted, err = store.GetMessagesByIDs(ctx, quotedIDs)
			if err != nil {
				return endpoint.Reply{}, err
			}
		}
		list := pyjson.List{}
		for _, record := range page.Messages {
			var preview *pyjson.OrderedMap
			if record.ReplyToID != nil {
				if parent, ok := quoted[*record.ReplyToID]; ok {
					preview = wireReplyPreview(parent)
				}
			}
			wired := wireMessage(record, summaries[record.ID], preview)
			list = append(list, wired)
		}
		var next pyjson.Value = pyjson.Null{}
		if len(list) > 0 {
			last := list[len(list)-1].(*pyjson.OrderedMap)
			cursor, _ := last.Get("cursor")
			next = cursor
		} else if afterRaw != nil {
			next = pyjson.String(*afterRaw)
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("messages", list)
		body.Set("next_cursor", next)
		body.Set("has_more", pyjson.Bool(page.HasMore))
		return endpoint.Reply{Body: body}, nil
	}}
}

func deleteOwnMessage() Route {
	return Route{ID: "DELETE /contexts/{context_id}/messages/{message_id}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		messageID, err := pathUUID(call, "message_id")
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
		message, err := messageInContext(ctx, store, contextID, messageID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := messageedit.CheckDeletable(messageFacts(*message), call.Actor.ID); err != nil {
			var refused *messageedit.Error
			if errors.As(err, &refused) {
				switch refused.Code {
				case "ALREADY_DELETED":
					return endpoint.Reply{}, endpoint.Refuse(409, "message_already_deleted", "Tin này đã bị xoá rồi.")
				case "NOT_AUTHOR":
					if err := requireFacts(call, "delete_own_message", map[string]bool{"is_group_member": true, "is_author": false}); err != nil {
						return endpoint.Reply{}, err
					}
					return endpoint.Reply{}, err
				default:
					return endpoint.Reply{}, endpoint.Refuse(409, "message_kind_not_deletable", "Chỉ xoá được tin nhắn, ảnh hoặc sticker của chính bạn.")
				}
			}
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "delete_own_message", map[string]bool{"is_group_member": true, "is_author": true}); err != nil {
			return endpoint.Reply{}, err
		}
		_, err = store.SoftDeleteMessage(ctx, message.ID, time.Now().UTC())
		if err != nil {
			var conflict *repo.Conflict
			if errors.As(err, &conflict) && conflict.Code == "MESSAGE_ALREADY_DELETED" {
				return endpoint.Reply{}, endpoint.Refuse(409, "message_already_deleted", "Tin này đã bị xoá rồi.")
			}
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

func reactToMessage() Route {
	return Route{ID: "POST /contexts/{context_id}/messages/{message_id}/reactions", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		messageID, err := pathUUID(call, "message_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := stringField(request, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "react_to_message", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		message, err := messageInContext(ctx, store, contextID, messageID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if message.Kind == "deleted" {
			return endpoint.Reply{}, endpoint.Refuse(409, "message_deleted", "Tin nhắn đã bị xoá, không phản ứng được.")
		}
		_, err = store.AddReaction(ctx, messageID, call.Actor.ID, kind, time.Now().UTC())
		if err != nil {
			var conflict *repo.Conflict
			if errors.As(err, &conflict) {
				switch conflict.Code {
				case "MESSAGE_DELETED":
					return endpoint.Reply{}, endpoint.Refuse(409, "message_deleted", "Tin nhắn đã bị xoá, không phản ứng được.")
				case "MESSAGE_NOT_FOUND":
					return endpoint.Reply{}, endpoint.Refuse(404, "message_not_found", "Message does not exist")
				}
			}
			return endpoint.Reply{}, err
		}
		return reactionsReply(ctx, store, messageID, call.Actor.ID)
	}}
}

func unreactToMessage() Route {
	return Route{ID: "DELETE /contexts/{context_id}/messages/{message_id}/reactions/{kind}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		messageID, err := pathUUID(call, "message_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := stringParam(call, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "react_to_message", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if _, err := messageInContext(ctx, store, contextID, messageID); err != nil {
			return endpoint.Reply{}, err
		}
		if _, err := store.RemoveReaction(ctx, messageID, call.Actor.ID, kind); err != nil {
			return endpoint.Reply{}, err
		}
		return reactionsReply(ctx, store, messageID, call.Actor.ID)
	}}
}

func createChatExpenseDraft() Route {
	return Route{ID: "POST /contexts/{context_id}/messages/{message_id}/expense-draft", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.ChatExpenseLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		messageID, err := pathUUID(call, "message_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "invoke_group_companion", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		message, err := store.GetMessage(ctx, messageID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if message == nil || message.ContextID != contextID {
			return endpoint.Reply{}, endpoint.Refuse(404, "message_not_found", "Message does not exist")
		}
		if message.Kind == "deleted" {
			return endpoint.Reply{}, endpoint.Refuse(409, "message_deleted", "Tin này đã bị xoá.")
		}
		if message.AuthorID == nil {
			return endpoint.Reply{}, endpoint.Refuse(422, "message_has_no_author", "An AI message has no person who paid")
		}
		if message.Body == nil || strings.TrimSpace(*message.Body) == "" {
			return endpoint.Reply{}, endpoint.Refuse(422, "message_has_no_text", "Message has no text to read as an expense")
		}
		shared, err := activeSharedBy(ctx, store, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		reading, err := readChatExpense(*message.Body)
		if err != nil {
			return endpoint.Reply{}, mapChatExpenseErr(err)
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("message_id", pyjson.String(messageID))
		if !reading.isExpense {
			body.Set("detected", pyjson.Bool(false))
			body.Set("draft", pyjson.Null{})
			body.Set("reason", pyjson.String(chatExpenseNoText))
			return endpoint.Reply{Body: body}, nil
		}
		draft := pyjson.NewOrderedMap()
		draft.Set("title", pyjson.String(reading.title))
		draft.Set("amount_vnd", pyjson.NewInt(reading.amount))
		draft.Set("paid_by_id", pyjson.String(*message.AuthorID))
		people := pyjson.List{}
		for _, id := range shared {
			people = append(people, pyjson.String(id))
		}
		draft.Set("shared_by", people)
		draft.Set("needs_review", pyjson.Bool(reading.needsReview))
		body.Set("detected", pyjson.Bool(true))
		body.Set("draft", draft)
		body.Set("reason", pyjson.Null{})
		return endpoint.Reply{Body: body}, nil
	}}
}

func takeCompanionTurnRoute() Route {
	return Route{ID: "POST /contexts/{context_id}/ai-turn", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.CompanionTurnLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		requested := false
		request, err := optionalBodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if request != nil {
			flag, err := optionalBoolField(request, "requested")
			if err != nil {
				return endpoint.Reply{}, err
			}
			if flag != nil {
				requested = *flag
			}
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		turn, err := takeCompanionTurn(ctx, call, store, contextID, requested)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: turn}, nil
	}}
}

func setContextMemberRole() Route {
	return Route{ID: "PUT /contexts/{context_id}/members/{person_id}/role", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		role, err := stringField(request, "role")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		actorRole, err := store.MembershipRole(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		isAdmin := actorRole != nil && *actorRole == "admin"
		extra, err := groupAdminRole(ctx, store, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFactsWithRoles(call, "set_member_role", map[string]bool{"is_group_admin": isAdmin}, extra); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupKind(ctx, store, contextID); err != nil {
			return endpoint.Reply{}, err
		}
		membership, err := store.SetMembershipRole(ctx, contextID, personID, role)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if membership == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "membership_not_found", "Active membership does not exist")
		}
		return endpoint.Reply{Body: wireMembership(*membership)}, nil
	}}
}

func markContextRead() Route {
	return Route{ID: "PUT /contexts/{context_id}/read-mark", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		messageID, err := uuidField(request, "message_id")
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
		message, err := store.GetMessage(ctx, messageID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if message == nil || message.ContextID != contextID {
			return endpoint.Reply{}, endpoint.Refuse(404, "message_not_found", "Message not found")
		}
		mark, err := store.SetReadMark(ctx, contextID, call.Actor.ID, *message, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		unread, err := store.CountUnreadMessages(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("last_read_message_id", pyjson.String(mark.LastReadMessageID))
		body.Set("unread_count", pyjson.NewInt(unread))
		return endpoint.Reply{Body: body}, nil
	}}
}

func actOnMessageIntent(ctx context.Context, call *endpoint.Call, store repo.Repository, contextID string, posted *pyjson.OrderedMap, stored repo.Message) (*pyjson.OrderedMap, error) {
	out := clonePosted(posted)
	if stored.Kind != "text" || stored.Body == nil {
		return out, nil
	}
	intent := chatintent.Parse(*stored.Body)
	if intent == nil {
		return out, nil
	}
	if call.ExplicitChatInvocation && intent.Intent != chatintent.Vote {
		out.Set("intent", pyjson.String(intent.Intent))
		out.Set("intent_error", pyjson.String("explicit_invocation_required"))
		return out, nil
	}
	switch intent.Intent {
	case chatintent.Plan, chatintent.Mention:
		if err := spendActorWindow(call, call.Limits.MessageIntentLimiter); err != nil {
			if refused, ok := err.(*endpoint.Refusal); ok && refused.Problem.Status == 429 {
				out.Set("intent", pyjson.String(intent.Intent))
				out.Set("intent_error", pyjson.String("companion_rate_limited"))
				return out, nil
			}
			return nil, err
		}
		turn, err := takeCompanionTurn(ctx, call, store, contextID, true)
		if err != nil {
			return nil, err
		}
		out.Set("intent", pyjson.String(intent.Intent))
		out.Set("companion", turn)
		return out, nil
	case chatintent.Vote:
		spec := chatintent.ParseVote(intent.Args)
		if spec == nil {
			out.Set("intent", pyjson.String("vote"))
			out.Set("intent_error", pyjson.String("vote_malformed"))
			return out, nil
		}
		if err := requireGroupMember(ctx, call, store, "create_vote", contextID); err != nil {
			return nil, err
		}
		options := make([]repo.VoteOptionInput, len(spec.Options))
		for i, label := range spec.Options {
			options[i] = repo.VoteOptionInput{Label: label}
		}
		vote, err := store.CreateVote(ctx, repo.VoteInput{
			ContextID: contextID, CreatedByID: call.Actor.ID, Question: spec.Question,
			Options: options, Now: time.Now().UTC(),
		})
		if err != nil {
			return nil, err
		}
		wired, err := wireVote(vote, call.Actor.ID)
		if err != nil {
			return nil, err
		}
		payload := pyjson.NewOrderedMap()
		payload.Set("vote_id", pyjson.String(vote.ID))
		payload.Set("question", pyjson.String(vote.Question))
		opts := pyjson.List{}
		for _, option := range vote.Options {
			entry := pyjson.NewOrderedMap()
			entry.Set("id", pyjson.String(option.ID))
			entry.Set("label", pyjson.String(option.Label))
			opts = append(opts, entry)
		}
		payload.Set("options", opts)
		card := pyjson.NewOrderedMap()
		card.Set("kind", pyjson.String("poll"))
		card.Set("payload", payload)
		raw, err := cardBytes(card)
		if err != nil {
			return nil, err
		}
		author := call.Actor.ID
		if _, err := store.CreateMessage(ctx, repo.MessageInput{
			ContextID: contextID, AuthorID: &author, Kind: "ai_card", Now: time.Now().UTC(), Card: raw,
		}); err != nil {
			return nil, err
		}
		out.Set("intent", pyjson.String("vote"))
		out.Set("vote", wired)
		return out, nil
	default:
		client := brain.Configured()
		if client == nil {
			out.Set("intent", pyjson.String("chia_bill"))
			out.Set("intent_error", pyjson.String("chia_bill_not_available"))
			return out, nil
		}
		if err := spendActorWindow(call, call.Limits.MessageIntentLimiter); err != nil {
			if refused, ok := err.(*endpoint.Refusal); ok && refused.Problem.Status == 429 {
				out.Set("intent", pyjson.String("chia_bill"))
				out.Set("intent_error", pyjson.String("companion_rate_limited"))
				return out, nil
			}
			return nil, err
		}
		outcome, err := draftExpensesFromChat(ctx, store, contextID, stored.ID)
		if err != nil {
			return nil, err
		}
		out.Set("intent", pyjson.String("chia_bill"))
		if outcome.code != "" {
			out.Set("intent_error", pyjson.String(outcome.code))
			return out, nil
		}
		out.Set("expense_card", outcome.card)
		return out, nil
	}
}

type chiaOutcome struct {
	code string
	card *pyjson.OrderedMap
}

func draftExpensesFromChat(ctx context.Context, store repo.Repository, contextID, commandID string) (chiaOutcome, error) {
	page, err := store.ListMessages(ctx, contextID, chiaBillWindow, nil, nil)
	if err != nil {
		return chiaOutcome{}, err
	}
	shared, err := activeSharedBy(ctx, store, contextID)
	if err != nil {
		return chiaOutcome{}, err
	}
	drafts := pyjson.List{}
	calls := 0
	for _, message := range page.Messages {
		if message.ID == commandID || message.Kind != "text" || message.AuthorID == nil || message.Body == nil {
			continue
		}
		if strings.TrimSpace(*message.Body) == "" || chatintent.Parse(*message.Body) != nil {
			continue
		}
		if calls >= chiaBillModelCalls {
			break
		}
		calls++
		reading, err := readChatExpense(*message.Body)
		if err != nil {
			if refused := brainErr(err); refused != nil {
				switch refused.Code {
				case "chat_reader_not_configured", "chat_reader_unavailable", "brain_unavailable":
					return chiaOutcome{code: "chia_bill_not_available"}, nil
				case "chat_expense_model_named_a_person":
					return chiaOutcome{code: "chia_bill_refused"}, nil
				default:
					continue
				}
			}
			return chiaOutcome{code: "chia_bill_not_available"}, nil
		}
		if !reading.isExpense {
			continue
		}
		entry := pyjson.NewOrderedMap()
		entry.Set("title", pyjson.String(reading.title))
		entry.Set("amount_vnd", pyjson.NewInt(reading.amount))
		entry.Set("paid_by_id", pyjson.String(*message.AuthorID))
		people := pyjson.List{}
		for _, id := range shared {
			people = append(people, pyjson.String(id))
		}
		entry.Set("shared_by", people)
		entry.Set("source_message_id", pyjson.String(message.ID))
		entry.Set("needs_review", pyjson.Bool(true))
		drafts = append(drafts, entry)
	}
	if len(drafts) == 0 {
		return chiaOutcome{code: "chia_bill_no_expenses"}, nil
	}
	for i, j := 0, len(drafts)-1; i < j; i, j = i+1, j-1 {
		drafts[i], drafts[j] = drafts[j], drafts[i]
	}
	payload := pyjson.NewOrderedMap()
	payload.Set("drafts", drafts)
	card := pyjson.NewOrderedMap()
	card.Set("kind", pyjson.String("expense_draft"))
	card.Set("payload", payload)
	raw, err := cardBytes(card)
	if err != nil {
		return chiaOutcome{}, err
	}
	record, err := store.CreateMessage(ctx, repo.MessageInput{
		ContextID: contextID, Kind: "ai_card", Card: raw, Now: time.Now().UTC(),
	})
	if err != nil {
		return chiaOutcome{}, err
	}
	return chiaOutcome{card: wireMessage(record, nil, nil)}, nil
}

func takeCompanionTurn(ctx context.Context, call *endpoint.Call, store repo.Repository, contextID string, requested bool) (*pyjson.OrderedMap, error) {
	if err := requireGroupMember(ctx, call, store, "invoke_group_companion", contextID); err != nil {
		return nil, err
	}
	consent, err := service.PairChatConsent(ctx, store, contextID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if consent != nil && !*consent {
		return nil, endpoint.Refuse(403, "pair_chat_consent_required", "Cả hai cùng đồng ý cho Nếp đọc tin nhắn thì Nếp mới nói được.")
	}
	silent := func(reason string) *pyjson.OrderedMap {
		out := pyjson.NewOrderedMap()
		out.Set("context_id", pyjson.String(contextID))
		out.Set("spoke", pyjson.Bool(false))
		out.Set("reason", pyjson.String(reason))
		out.Set("message", pyjson.Null{})
		return out
	}
	page, err := store.ListMessages(ctx, contextID, contextWindow, nil, nil)
	if err != nil {
		return nil, err
	}
	messages := make([]repo.Message, len(page.Messages))
	copy(messages, page.Messages)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	kinds := make([]string, len(messages))
	created := make([]time.Time, len(messages))
	for i, message := range messages {
		if message.Kind == "ai_card" && message.AuthorID == nil {
			kinds[i] = "ai"
		} else {
			kinds[i] = "human"
		}
		created[i] = message.CreatedAt
	}
	decision := companion.PlanTurn(kinds, created, time.Now().UTC(), requested)
	if !decision.MaySpeak {
		return silent(decision.Reason), nil
	}
	conversation := pyjson.List{}
	for _, message := range messages {
		if message.Kind == "deleted" {
			continue
		}
		row := pyjson.NewOrderedMap()
		row.Set("id", pyjson.String(message.ID))
		row.Set("author_id", textOrNull(message.AuthorID))
		kind := "human"
		if message.Kind == "ai_card" && message.AuthorID == nil {
			kind = "ai"
		}
		row.Set("author_kind", pyjson.String(kind))
		row.Set("kind", pyjson.String(message.Kind))
		row.Set("body", textOrNull(message.Body))
		row.Set("image_url", textOrNull(message.ImageURL))
		row.Set("card", cardValue(message.Card))
		row.Set("created_at", pyjson.String(pyjson.DateTime(message.CreatedAt.UTC())))
		conversation = append(conversation, row)
	}
	members := pyjson.List{}
	roster, err := store.ListMembers(ctx, contextID)
	if err != nil {
		return nil, err
	}
	for _, membership := range roster {
		person, err := store.GetPerson(ctx, membership.PersonID)
		if err != nil {
			return nil, err
		}
		if person == nil {
			continue
		}
		entry := pyjson.NewOrderedMap()
		entry.Set("id", pyjson.String(person.ID))
		entry.Set("display_name", pyjson.String(person.DisplayName))
		members = append(members, entry)
	}
	group, err := service.GroupTaste(ctx, store, contextID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	places, err := service.ModelPlaceRows(ctx, store, group)
	if err != nil {
		return nil, err
	}
	placeList := pyjson.List{}
	for _, place := range places {
		placeList = append(placeList, place)
	}
	payload := pyjson.NewOrderedMap()
	payload.Set("conversation", conversation)
	payload.Set("members", members)
	payload.Set("places", placeList)
	if group.BudgetPerPersonVND == nil {
		payload.Set("budget_per_person_vnd", pyjson.Null{})
	} else {
		payload.Set("budget_per_person_vnd", pyjson.NewInt(*group.BudgetPerPersonVND))
	}
	raw, err := brain.Configured().PostJSON("companion-reply", payload)
	if err != nil {
		return silent("unavailable"), nil
	}
	grounded, err := companion.GroundCard(treejson.To(raw), treejson.MapsTo(places))
	if err != nil {
		return silent("ungrounded"), nil
	}
	stored, err := cardBytes(treejson.From(grounded))
	if err != nil {
		return nil, err
	}
	record, err := store.CreateMessage(ctx, repo.MessageInput{
		ContextID: contextID, Kind: "ai_card", Card: stored, Now: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	out := pyjson.NewOrderedMap()
	out.Set("context_id", pyjson.String(contextID))
	out.Set("spoke", pyjson.Bool(true))
	out.Set("reason", pyjson.String("ok"))
	out.Set("message", wireMessage(record, nil, nil))
	return out, nil
}

type chatReading struct {
	isExpense   bool
	title       string
	amount      int64
	needsReview bool
}

func readChatExpense(text string) (chatReading, error) {
	body := pyjson.NewOrderedMap()
	body.Set("text", pyjson.String(text))
	raw, err := brain.Configured().PostJSON("chat-expense", body)
	if err != nil {
		return chatReading{}, err
	}
	obj, err := brain.AsObject(raw)
	if err != nil {
		return chatReading{}, &brain.Error{Status: 502, Code: "chat_reader_unavailable"}
	}
	flag, _ := obj.Get("is_expense")
	on, _ := flag.(pyjson.Bool)
	if !bool(on) {
		return chatReading{}, nil
	}
	titleValue, _ := obj.Get("title")
	title, _ := titleValue.(pyjson.String)
	amountValue, _ := obj.Get("amount_vnd")
	amount, _ := amountValue.(pyjson.Int)
	n, _ := amount.Int64()
	reviewValue, _ := obj.Get("needs_review")
	review, _ := reviewValue.(pyjson.Bool)
	return chatReading{isExpense: true, title: string(title), amount: n, needsReview: bool(review)}, nil
}

func mapChatExpenseErr(err error) error {
	if refused := brainErr(err); refused != nil {
		switch refused.Code {
		case "chat_reader_not_configured":
			return endpoint.Refuse(503, "chat_reader_not_configured", chatReaderNotConfigured)
		case "chat_expense_model_named_a_person":
			return endpoint.Refuse(422, "chat_expense_model_named_a_person", modelNamedPersonDetail)
		case "chat_expense_unreadable":
			return endpoint.Refuse(422, "chat_expense_unreadable", chatUnreadableDetail)
		case "chat_reader_unavailable", "brain_unavailable":
			return endpoint.Refuse(502, "chat_reader_unavailable", chatReaderUnavailable)
		}
	}
	return err
}

func messageInContext(ctx context.Context, store repo.Repository, contextID, messageID string) (*repo.Message, error) {
	message, err := store.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if message == nil || message.ContextID != contextID {
		return nil, endpoint.Refuse(404, "message_not_found", "Message does not exist")
	}
	return message, nil
}

func messageFacts(record repo.Message) messageedit.Facts {
	return messageedit.Facts{ID: record.ID, ContextID: record.ContextID, AuthorID: record.AuthorID, Kind: record.Kind}
}

func requirePhotoURLContext(contextID string, imageURL *string) error {
	if imageURL == nil {
		return nil
	}
	parts := strings.Split(*imageURL, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "contexts" || parts[3] != "photos" {
		return endpoint.Refuse(422, "photo_url_invalid", "Photo URL is not a path into this product's photo storage")
	}
	if _, err := parseHyphenUUID(parts[2]); err != nil {
		return endpoint.Refuse(422, "photo_url_invalid", "Photo URL is not a path into this product's photo storage")
	}
	if _, err := parseHyphenUUID(parts[4]); err != nil {
		return endpoint.Refuse(422, "photo_url_invalid", "Photo URL is not a path into this product's photo storage")
	}
	if parts[2] != contextID {
		return endpoint.Refuse(422, "photo_context_mismatch", "Photo URL context does not match the requested context")
	}
	return nil
}

func parseHyphenUUID(s string) (string, error) {
	if len(s) != 36 {
		return "", errors.New("uuid")
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", errors.New("uuid")
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", errors.New("uuid")
		}
	}
	return strings.ToLower(s), nil
}

func decodeMessageCursor(raw *string) (*repo.MessageCursor, error) {
	if raw == nil {
		return nil, nil
	}
	position, err := cursors.DecodeCursor(*raw)
	if err != nil {
		return nil, endpoint.Refuse(422, "invalid_cursor", "Message cursor is invalid")
	}
	return &repo.MessageCursor{CreatedAt: position.CreatedAt.Time(), ID: position.MessageID}, nil
}

func wireMessage(record repo.Message, reactions pyjson.List, replyTo *pyjson.OrderedMap) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("context_id", pyjson.String(record.ContextID))
	out.Set("author_id", textOrNull(record.AuthorID))
	out.Set("kind", pyjson.String(record.Kind))
	out.Set("body", textOrNull(record.Body))
	out.Set("image_url", textOrNull(record.ImageURL))
	out.Set("card", cardValue(record.Card))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	out.Set("cursor", pyjson.String(cursors.EncodeCursor(record.CreatedAt, record.ID)))
	if record.Kind == "deleted" || reactions == nil {
		out.Set("reactions", pyjson.List{})
	} else {
		out.Set("reactions", reactions)
	}
	if replyTo == nil {
		out.Set("reply_to", pyjson.Null{})
	} else {
		out.Set("reply_to", replyTo)
	}
	if record.DeletedAt == nil {
		out.Set("deleted_at", pyjson.Null{})
	} else {
		out.Set("deleted_at", pyjson.String(pyjson.DateTime(record.DeletedAt.UTC())))
	}
	return out
}

func clonePosted(message *pyjson.OrderedMap) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	for key, value := range message.All() {
		out.Set(key, value)
	}
	out.Set("intent", pyjson.Null{})
	out.Set("companion", pyjson.Null{})
	out.Set("vote", pyjson.Null{})
	out.Set("expense_card", pyjson.Null{})
	out.Set("intent_error", pyjson.Null{})
	return out
}

func wireReplyPreview(target repo.Message) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(target.ID))
	out.Set("kind", pyjson.String(target.Kind))
	out.Set("author_id", textOrNull(target.AuthorID))
	out.Set("preview", pyjson.String(messagePreview(target)))
	return out
}

func messagePreview(record repo.Message) string {
	switch record.Kind {
	case "image":
		return "[Ảnh]"
	case "sticker":
		return "[Sticker]"
	case "deleted":
		return "Tin nhắn đã bị xoá"
	case "ai_card":
		card := cardValue(record.Card)
		obj, _ := card.(*pyjson.OrderedMap)
		kind := ""
		if obj != nil {
			if value, ok := obj.Get("kind"); ok {
				if text, ok := value.(pyjson.String); ok {
					kind = string(text)
				}
			}
		}
		labels := map[string]string{
			"itinerary": "[Rủ Đi AI: lịch trình]", "places": "[Rủ Đi AI: gợi ý địa điểm]",
			"poll": "[Bình chọn]", "expense_draft": "[Rủ Đi AI: bản nháp khoản chi]",
		}
		if label, ok := labels[kind]; ok {
			return label
		}
		return "[Rủ Đi AI]"
	}
	text := ""
	if record.Body != nil {
		text = strings.TrimSpace(strings.ReplaceAll(*record.Body, "\n", " "))
	}
	runes := []rune(text)
	if len(runes) <= 80 {
		return text
	}
	return string(runes[:79]) + "…"
}

func summariseReactions(rows []repo.Reaction, readerID string) map[string]pyjson.List {
	type key struct{ message, kind string }
	counts := map[key]int64{}
	mine := map[key]bool{}
	order := []key{}
	for _, row := range rows {
		k := key{row.MessageID, row.Kind}
		if _, seen := counts[k]; !seen {
			order = append(order, k)
		}
		counts[k]++
		if row.PersonID == readerID {
			mine[k] = true
		}
	}
	out := map[string]pyjson.List{}
	for _, k := range order {
		entry := pyjson.NewOrderedMap()
		entry.Set("kind", pyjson.String(k.kind))
		entry.Set("count", pyjson.NewInt(counts[k]))
		entry.Set("mine", pyjson.Bool(mine[k]))
		out[k.message] = append(out[k.message], entry)
	}
	return out
}

func reactionsReply(ctx context.Context, store repo.Repository, messageID, actorID string) (endpoint.Reply, error) {
	rows, err := store.ListReactions(ctx, []string{messageID})
	if err != nil {
		return endpoint.Reply{}, err
	}
	summaries := summariseReactions(rows, actorID)
	list := summaries[messageID]
	if list == nil {
		list = pyjson.List{}
	}
	body := pyjson.NewOrderedMap()
	body.Set("message_id", pyjson.String(messageID))
	body.Set("reactions", list)
	return endpoint.Reply{Body: body}, nil
}

func activeSharedBy(ctx context.Context, store repo.Repository, contextID string) ([]string, error) {
	members, err := store.ListMembers(ctx, contextID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, member := range members {
		if member.State == "active" {
			ids = append(ids, member.PersonID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return uuidBytesLess(ids[i], ids[j]) })
	return ids, nil
}
