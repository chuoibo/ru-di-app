package routes

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/domain/direct"
	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// createContext is POST /contexts (routes/contexts.py create_context,
// ApiService.create_context): permission, the registered creator, the insert,
// then the creator admitted through the same invited -> active transition as
// every later member.
func createContext() Route {
	return Route{ID: "POST /contexts", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		name, err := stringField(body, "display_name")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "create_context", map[string]bool{}); err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireRegisteredPerson(ctx, store, call.Actor.ID); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.CreateContext(ctx, name, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		membership, err := store.AddMember(ctx, record.ID, call.Actor.ID, call.Actor.ID, "admin")
		if err != nil {
			// RepositoryConflict is not caught here in Python: a 500.
			return endpoint.Reply{}, err
		}
		accepted, err := store.AcceptMembership(ctx, membership.ID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		if accepted == nil {
			return endpoint.Reply{}, endpoint.Refuse(409, "creator_membership_missing", "Creator membership disappeared during context creation")
		}
		out, err := contextResponse(ctx, store, record, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: out}, nil
	}}
}

// updateContext is PATCH /contexts/{context_id} (update_context): membership
// from the roster before the row is read, then the domain's changes (a rename
// reads the kind first), then the update.
func updateContext() Route {
	return Route{ID: "PATCH /contexts/{context_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		displayName, err := optionalStringField(body, "display_name")
		if err != nil {
			return endpoint.Reply{}, err
		}
		theme, err := optionalStringField(body, "theme")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "edit_context", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		changes, refused, err := contexts.UpdateChanges(displayName, theme, func() (bool, error) {
			record, err := store.GetContext(ctx, contextID)
			if err != nil {
				return false, err
			}
			return record != nil && direct.IsPair(record.Kind), nil
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
		}
		record, err := store.UpdateContext(ctx, contextID, repo.ContextChanges{DisplayName: changes.DisplayName, Theme: changes.Theme})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, contextNotFound()
		}
		out, err := contextResponse(ctx, store, *record, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: out}, nil
	}}
}

// inviteContextMember is POST /contexts/{context_id}/members
// (invite_context_member): membership and this group's admin role, both read
// before the permission check, then the kind, the registered invitee and the
// insert.
func inviteContextMember() Route {
	return Route{ID: "POST /contexts/{context_id}/members", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		personID, err := uuidField(body, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		roles, err := groupAdminRole(ctx, store, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFactsWithRoles(call, "invite_context_member", map[string]bool{"is_group_member": member}, roles); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupKind(ctx, store, contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireRegisteredPerson(ctx, store, personID); err != nil {
			return endpoint.Reply{}, err
		}
		membership, err := store.AddMember(ctx, contextID, personID, call.Actor.ID, "member")
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(conflict.Code), "Membership invitation conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireMembership(membership)}, nil
	}}
}

// acceptContextMembership is POST /memberships/{membership_id}/accept
// (accept_context_membership): the invitation first (404 before any
// permission), then the predicate its origin warrants, the kind, and the
// locked transition.
func acceptContextMembership() Route {
	return Route{ID: "POST /memberships/{membership_id}/accept", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		membershipID, err := pathUUID(call, "membership_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		membership, err := store.GetMembership(ctx, membershipID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if membership == nil {
			return endpoint.Reply{}, membershipNotFound("Membership does not exist")
		}
		action, facts, err := contexts.AcceptPermission(membership.Origin, call.Actor.ID, membership.PersonID, func() (bool, error) {
			return store.IsMember(ctx, membership.ContextID, call.Actor.ID)
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, action, facts); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupKind(ctx, store, membership.ContextID); err != nil {
			return endpoint.Reply{}, err
		}
		accepted, err := store.AcceptMembership(ctx, membershipID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(conflict.Code), "Membership acceptance conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		if accepted == nil {
			return endpoint.Reply{}, membershipNotFound("Membership does not exist")
		}
		return endpoint.Reply{Body: wireMembership(*accepted)}, nil
	}}
}

// leaveContext is DELETE /contexts/{context_id}/members/{person_id}
// (leave_context): only oneself, only a group, only an active membership; 204
// with no body.
func leaveContext() Route {
	return Route{ID: "DELETE /contexts/{context_id}/members/{person_id}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "leave_context", map[string]bool{"is_group_member": member, "is_self": call.Actor.ID == personID}); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupKind(ctx, store, contextID); err != nil {
			return endpoint.Reply{}, err
		}
		left, err := store.LeaveContext(ctx, contextID, personID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		if left == nil {
			return endpoint.Reply{}, membershipNotFound("Active membership does not exist")
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// listContextMembers is GET /contexts/{context_id}/members
// (list_context_members): members only; former members are refused.
func listContextMembers() Route {
	return Route{ID: "GET /contexts/{context_id}/members", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_context_members", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		members, err := store.ListMembers(ctx, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := make(pyjson.List, len(members))
		for i, member := range members {
			list[i] = wireMembership(member)
		}
		out := pyjson.NewOrderedMap()
		out.Set("context_id", pyjson.String(contextID))
		out.Set("members", list)
		return endpoint.Reply{Body: out}, nil
	}}
}

// getContextBalances is GET /contexts/{context_id}/balances
// (get_context_balances): members only, then the latest confirmed version of
// every expense (row locks), the confirmed receipts, and the ledger. Derived
// sums stay exact past int64, as Python's int does.
func getContextBalances() Route {
	return Route{ID: "GET /contexts/{context_id}/balances", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_context_members", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		inputs, err := store.LoadBatchInputs(ctx, contextID, nil)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows, err := store.LoadConfirmedReceipts(ctx, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		receipts := make(map[ledger.Pair]*big.Int, len(rows))
		for _, row := range rows {
			receipts[ledger.Pair{SenderID: row.SenderID, RecipientID: row.RecipientID}] = row.AmountVND
		}
		expenses := make([]ledger.ConfirmedExpense, len(inputs.Expenses))
		for i, expense := range inputs.Expenses {
			allocations := make([]ledger.Allocation, len(expense.Allocations))
			for j, allocation := range expense.Allocations {
				allocations[j] = ledger.Allocation{ParticipantID: allocation.ParticipantID, AmountVND: money.VND(allocation.AmountVND)}
			}
			expenses[i] = ledger.ConfirmedExpense{VersionID: expense.VersionID, PaidByID: expense.PaidByID, Allocations: allocations}
		}
		sheet, err := ledger.ContextBalances(expenses, receipts)
		var refused *ledger.LedgerError
		if errors.As(err, &refused) {
			return endpoint.Reply{}, endpoint.Refuse(409, refused.Code, "Confirmed ledger events cannot be balanced")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		balances := make(pyjson.List, len(sheet.Balances))
		for i, balance := range sheet.Balances {
			entry := pyjson.NewOrderedMap()
			entry.Set("person_id", pyjson.String(balance.PersonID))
			entry.Set("net_vnd", pyjson.NewBigInt(balance.NetVND))
			balances[i] = entry
		}
		transfers := make(pyjson.List, len(sheet.Transfers))
		for i, transfer := range sheet.Transfers {
			entry := pyjson.NewOrderedMap()
			entry.Set("sender_id", pyjson.String(transfer.SenderID))
			entry.Set("recipient_id", pyjson.String(transfer.RecipientID))
			entry.Set("amount_vnd", pyjson.NewBigInt(transfer.AmountVND))
			transfers[i] = entry
		}
		out := pyjson.NewOrderedMap()
		out.Set("balances", balances)
		out.Set("transfers", transfers)
		out.Set("proven_minimal", pyjson.Bool(sheet.ProvenMinimal))
		out.Set("transfer_count", pyjson.NewInt(int64(sheet.TransferCount)))
		return endpoint.Reply{Body: out}, nil
	}}
}

// getContext is GET /contexts/{context_id} (get_context): membership is
// decided before the row is read, so a stranger gets the same 403 for an id
// that exists and one that does not.
func getContext() Route {
	return Route{ID: "GET /contexts/{context_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_context_members", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.GetContext(ctx, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, contextNotFound()
		}
		out, err := contextResponse(ctx, store, *record, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: out}, nil
	}}
}

// groupStore is the request's repository.
func groupStore(ctx context.Context, call *endpoint.Call) (repo.Repository, error) {
	tx, err := call.Unit.Tx(ctx)
	if err != nil {
		return repo.Repository{}, err
	}
	return repo.Repository{Q: tx}, nil
}

// requireGroupMember is `_require_permission(action, actor, {"is_group_member":
// repository.is_member(context_id, actor.id)})`.
func requireGroupMember(ctx context.Context, call *endpoint.Call, store repo.Repository, action, contextID string) error {
	member, err := store.IsMember(ctx, contextID, call.Actor.ID)
	if err != nil {
		return err
	}
	return requireFacts(call, action, map[string]bool{"is_group_member": member})
}

// requireFactsWithRoles is `_require_permission(action, actor, facts,
// extra_roles=roles)`.
func requireFactsWithRoles(call *endpoint.Call, action string, facts map[string]bool, roles []string) error {
	refused, err := service.RequirePermission(action, *call.Actor, service.Resource{Proven: facts}, roles...)
	if err != nil {
		return err
	}
	if refused != nil {
		return &endpoint.Refusal{Problem: *refused}
	}
	return nil
}

// groupAdminRole is _group_admin_role: {"group_admin"} when this person is an
// admin of THIS group, read from the roster, never from the session.
func groupAdminRole(ctx context.Context, store repo.Repository, contextID, personID string) ([]string, error) {
	role, err := store.MembershipRole(ctx, contextID, personID)
	if err != nil {
		return nil, err
	}
	if role != nil && *role == "admin" {
		return []string{"group_admin"}, nil
	}
	return nil, nil
}

// requireRegisteredPerson is _require_registered_person. An ended account
// still has its row, so it passes, as in Python.
func requireRegisteredPerson(ctx context.Context, store repo.Repository, personID string) error {
	person, err := store.GetPerson(ctx, personID)
	if err != nil {
		return err
	}
	if person == nil {
		return endpoint.Refuse(409, "person_not_registered", "Register this person with PUT /people/{person_id} first")
	}
	return nil
}

// requireGroupKind is _require_group_kind: a roster door on a pair is a 409;
// an unknown id passes, and the next step answers for it.
func requireGroupKind(ctx context.Context, store repo.Repository, contextID string) error {
	record, err := store.GetContext(ctx, contextID)
	if err != nil {
		return err
	}
	if record != nil && direct.IsPair(record.Kind) {
		return endpoint.Refuse(409, "not_a_group", "Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.")
	}
	return nil
}

func contextNotFound() error {
	return endpoint.Refuse(404, "context_not_found", "Context does not exist")
}

func membershipNotFound(detail string) error {
	return endpoint.Refuse(404, "membership_not_found", detail)
}

// contextResponse is _context_response: a pair is named after the other
// member, read from the roster now; a group by its stored name.
func contextResponse(ctx context.Context, store repo.Repository, record repo.ContextRecord, actorID string) (*pyjson.OrderedMap, error) {
	view, err := contexts.View(record.Kind, record.DisplayName, actorID, func() ([]contexts.Member, error) {
		members, err := store.ListMembers(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		roster := make([]contexts.Member, len(members))
		for i, member := range members {
			roster[i] = contexts.Member{PersonID: member.PersonID, DisplayName: member.DisplayName}
		}
		return roster, nil
	})
	if err != nil {
		return nil, err
	}
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("display_name", pyjson.String(view.DisplayName))
	out.Set("created_by_id", pyjson.String(record.CreatedByID))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	out.Set("theme", pyjson.String(record.Theme))
	out.Set("kind", pyjson.String(record.Kind))
	if view.Counterpart == nil {
		out.Set("counterpart", pyjson.Null{})
	} else {
		counterpart := pyjson.NewOrderedMap()
		counterpart.Set("id", pyjson.String(view.Counterpart.ID))
		counterpart.Set("display_name", pyjson.String(view.Counterpart.DisplayName))
		out.Set("counterpart", counterpart)
	}
	return out, nil
}

// wireMembership is _wire_membership: MembershipResponse, origin left out.
func wireMembership(m repo.Membership) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(m.ID))
	out.Set("context_id", pyjson.String(m.ContextID))
	out.Set("person_id", pyjson.String(m.PersonID))
	out.Set("display_name", pyjson.String(m.DisplayName))
	out.Set("state", pyjson.String(m.State))
	out.Set("role", pyjson.String(m.Role))
	out.Set("invited_by_id", textOrNull(m.InvitedByID))
	out.Set("joined_at", instantOrNull(m.JoinedAt))
	out.Set("left_at", instantOrNull(m.LeftAt))
	out.Set("created_at", pyjson.String(pyjson.DateTime(m.CreatedAt.UTC())))
	return out
}

func instantOrNull(instant *time.Time) pyjson.Value {
	if instant == nil {
		return pyjson.Null{}
	}
	return pyjson.String(pyjson.DateTime(instant.UTC()))
}
