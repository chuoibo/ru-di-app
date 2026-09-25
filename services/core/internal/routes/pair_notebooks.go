package routes

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairsteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/service"
)

// The eight routes of routes/pair_notebooks.py. Each is the route body plus
// its ApiService method, which pairsteps holds with every repository call
// behind service.PairStore; the route reads the validated parameters, runs
// the method, and renders the response model in its field order.

// pairDigest is the sha256 the close revision is taken with.
var pairDigest pairsteps.Digest = sha256.Sum256

// pairStore is the request's repository.
func pairStore(ctx context.Context, call *endpoint.Call) service.PairStore {
	return service.PairStore{Ctx: ctx, Unit: call.Unit}
}

func pairActor(call *endpoint.Call) pairsteps.Actor {
	return pairsteps.Actor{ID: call.Actor.ID, Roles: call.Actor.Roles}
}

// pairNow is ApiService._now(): datetime.now(UTC) at microseconds, read once
// per request.
func pairNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// pairError ends a request a pair method failed: an ApiProblem answers as
// one, and everything else (a PermissionError_, an untranslated
// RepositoryConflict, a failed assert, a database error) is the 500 Python's
// unhandled exception is.
func pairError(err error) error {
	var refused *pairsteps.Refusal
	if errors.As(err, &refused) {
		return endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
	}
	return err
}

// pathString reads a `str` path parameter as the router decoded it.
func pathString(call *endpoint.Call, name string) (string, error) {
	text, ok := call.Values[name].(pyjson.String)
	if !ok {
		return "", fmt.Errorf("routes: path parameter %q is %T, not a str", name, call.Values[name])
	}
	return string(text), nil
}

// saturated is a Python int clamped to int64: the versions it is compared
// with are INTEGER columns, so every comparison answers as Python's does.
func saturated(n pyjson.Int) int64 {
	if v, ok := n.Int64(); ok {
		return v
	}
	if n.Big().Sign() > 0 {
		return math.MaxInt64
	}
	return math.MinInt64
}

// pathVersion reads an `int` path parameter.
func pathVersion(call *endpoint.Call, name string) (int64, error) {
	n, ok := call.Values[name].(pyjson.Int)
	if !ok {
		return 0, fmt.Errorf("routes: path parameter %q is %T, not an int", name, call.Values[name])
	}
	return saturated(n), nil
}

func dateTimeValue(t time.Time) pyjson.Value { return pyjson.String(pyjson.DateTime(t.UTC())) }

func optionalDateTimeValue(t *time.Time) pyjson.Value {
	if t == nil {
		return pyjson.Null{}
	}
	return dateTimeValue(*t)
}

// readPairNotebook is GET /contexts/{context_id}/notebook (read_pair_notebook,
// ApiService.pair_notebook).
func readPairNotebook() Route {
	return Route{ID: "GET /contexts/{context_id}/notebook", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := pairsteps.ReadNotebook(pairStore(ctx, call), pairActor(call), contextID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wireNotebook(view)}, nil
	}}
}

// proposePairConsent is POST /contexts/{context_id}/notebook/proposals
// (propose_pair_consent): the offer and the asker's own grant.
func proposePairConsent() Route {
	return Route{ID: "POST /contexts/{context_id}/notebook/proposals", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		purpose, err := stringField(request, "purpose")
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := pairsteps.ProposeConsent(pairStore(ctx, call), pairActor(call), contextID, purpose, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wireProposal(view)}, nil
	}}
}

// grantPairConsent is POST
// /contexts/{context_id}/notebook/proposals/{proposal_id}/grant
// (grant_pair_consent): the second yes.
func grantPairConsent() Route {
	return Route{ID: "POST /contexts/{context_id}/notebook/proposals/{proposal_id}/grant", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		proposalID, err := pathUUID(call, "proposal_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := pairsteps.GrantConsent(pairStore(ctx, call), pairActor(call), contextID, proposalID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wireProposal(view)}, nil
	}}
}

// revokePairConsent is DELETE /contexts/{context_id}/notebook/consents/{purpose}
// (revoke_pair_consent): `Response(status_code=204)`.
func revokePairConsent() Route {
	return Route{ID: "DELETE /contexts/{context_id}/notebook/consents/{purpose}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		purpose, err := pathString(call, "purpose")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := pairsteps.RevokeConsent(pairStore(ctx, call), pairActor(call), contextID, purpose, pairNow()); err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// putPairConstraint is PUT /contexts/{context_id}/notebook/constraints/{kind}
// (put_pair_constraint).
func putPairConstraint() Route {
	return Route{ID: "PUT /contexts/{context_id}/notebook/constraints/{kind}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := pathString(call, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		content, err := stringField(request, "content")
		if err != nil {
			return endpoint.Reply{}, err
		}
		constraint, err := pairsteps.PutConstraint(pairStore(ctx, call), pairActor(call), contextID, kind, content, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wireConstraint(constraint)}, nil
	}}
}

// deletePairConstraint is DELETE
// /contexts/{context_id}/notebook/constraints/{kind} (delete_pair_constraint).
func deletePairConstraint() Route {
	return Route{ID: "DELETE /contexts/{context_id}/notebook/constraints/{kind}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := pathString(call, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := pairsteps.DeleteConstraint(pairStore(ctx, call), pairActor(call), contextID, kind, pairNow()); err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// previewClosePairNotebook is POST /contexts/{context_id}/notebook/close/preview
// (preview_close_pair_notebook): three counts and the revision, no write.
func previewClosePairNotebook() Route {
	return Route{ID: "POST /contexts/{context_id}/notebook/close/preview", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		preview, err := pairsteps.PreviewClose(pairStore(ctx, call), pairActor(call), contextID, pairNow(), pairDigest)
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePreview(preview)}, nil
	}}
}

// closePairNotebook is POST /contexts/{context_id}/notebook/close
// (close_pair_notebook).
func closePairNotebook() Route {
	return Route{ID: "POST /contexts/{context_id}/notebook/close", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		revision, err := stringField(request, "revision")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := pairsteps.CloseNotebook(pairStore(ctx, call), pairActor(call), contextID, revision, pairNow(), pairDigest); err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// wireNotebook is PairNotebookResponse.
func wireNotebook(view pairsteps.NotebookView) *pyjson.OrderedMap {
	participants := pyjson.List{}
	for _, person := range view.Participants {
		participants = append(participants, pyjson.String(person))
	}
	mine := pyjson.List{}
	for _, state := range view.MyConsents {
		item := pyjson.NewOrderedMap()
		item.Set("purpose", pyjson.String(state.Purpose))
		item.Set("granted", pyjson.Bool(state.Granted))
		mine = append(mine, item)
	}
	theirs := pyjson.NewOrderedMap()
	for _, state := range view.TheirConsentsGranted {
		theirs.Set(state.Purpose, pyjson.Bool(state.Granted))
	}
	pending := pyjson.List{}
	for _, proposal := range view.PendingProposals {
		pending = append(pending, wireProposal(proposal))
	}
	constraints := pyjson.List{}
	for _, constraint := range view.Constraints {
		constraints = append(constraints, wireConstraint(constraint))
	}
	out := pyjson.NewOrderedMap()
	out.Set("context_id", pyjson.String(view.ContextID))
	out.Set("cycle_state", textOrNull(view.CycleState))
	out.Set("participants", participants)
	out.Set("my_consents", mine)
	out.Set("their_consents_granted", theirs)
	out.Set("pending_proposals", pending)
	out.Set("constraints", constraints)
	out.Set("nep_gui_ho", pyjson.Bool(view.NepGuiHo))
	out.Set("open_paper_id", textOrNull(view.OpenPaperID))
	granted := pyjson.List{}
	for _, purpose := range view.GrantedPurposes {
		granted = append(granted, pyjson.String(purpose))
	}
	out.Set("granted_purposes", granted)
	out.Set("taste", wirePairTaste(view.Taste))
	return out
}

// wirePairTaste is PairTasteResponse | None (ADR-0034).
func wirePairTaste(taste *pairnotebook.Taste) pyjson.Value {
	if taste == nil {
		return pyjson.Null{}
	}
	list := func(values []string) pyjson.List {
		out := pyjson.List{}
		for _, value := range values {
			out = append(out, pyjson.String(value))
		}
		return out
	}
	out := pyjson.NewOrderedMap()
	out.Set("mine_shared", pyjson.Bool(taste.MineShared))
	out.Set("theirs_shared", pyjson.Bool(taste.TheirsShared))
	out.Set("theirs", list(taste.Theirs))
	out.Set("common", list(taste.Common))
	return out
}

// wireProposal is PairProposalResponse.
func wireProposal(view pairsteps.ProposalView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(view.ID))
	out.Set("purpose", pyjson.String(view.Purpose))
	out.Set("expires_at", dateTimeValue(view.ExpiresAt))
	out.Set("proposed_by_id", pyjson.String(view.ProposedByID))
	out.Set("my_granted", pyjson.Bool(view.MyGranted))
	return out
}

// wireConstraint is PairConstraintResponse.
func wireConstraint(constraint pairsteps.Constraint) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("owner_id", pyjson.String(constraint.OwnerID))
	out.Set("kind", pyjson.String(constraint.Kind))
	out.Set("content", pyjson.String(constraint.Content))
	out.Set("version", pyjson.NewInt(int64(constraint.Version)))
	return out
}

// wirePreview is ClosePreviewResponse.
func wirePreview(preview pairnotebook.ClosePreview) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("revision", pyjson.String(preview.Revision))
	out.Set("so_nhap_bo", pyjson.NewInt(int64(preview.SoNhapBo)))
	out.Set("so_to_huy", pyjson.NewInt(int64(preview.SoToHuy)))
	out.Set("so_to_khoa", pyjson.NewInt(int64(preview.SoToKhoa)))
	out.Set("so_de_nghi_huy", pyjson.NewInt(int64(preview.SoDeNghiHuy)))
	return out
}
