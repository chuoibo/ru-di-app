package routes

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/outingsteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/idem"
	"mobile/services/core/internal/routing"
	guestweb "mobile/services/core/internal/web/guest"
)

// The eleven routes of app/api/routes/outings.py. Each handler reads what
// pyval validated, hands it to the matching outingsteps method over a Store
// that is this request's transaction, and renders what comes back in the
// response model's field order. Preview is the one that may open a socket:
// outingsteps stops at the draft, and itinerary.Build calls the configured
// Valhalla (or answers unavailable when there is none).

func outingActor(call *endpoint.Call) outingsteps.Actor {
	return outingsteps.Actor{ID: call.Actor.ID, Roles: call.Actor.Roles}
}

func outingRefusal(err error) error {
	var refused *outingsteps.Refusal
	if errors.As(err, &refused) {
		return endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
	}
	return err
}

type inviteSecrets struct{}

func (inviteSecrets) NewInviteToken() (string, []byte, error) {
	raw, err := mintGuestToken()
	if err != nil {
		return "", nil, err
	}
	return raw, auth.TokenDigest(raw), nil
}

func spendItineraryLimiter(call *endpoint.Call) error {
	if call.Limits == nil || call.Limits.ItineraryLimiter == nil {
		return errNoLimits
	}
	decision := call.Limits.ItineraryLimiter.Check(call.Actor.ID)
	if !decision.Allowed {
		return &endpoint.Refusal{Problem: decision.Problem}
	}
	return nil
}

func previewItinerary(draft itinerary.Draft) (itinerary.Preview, error) {
	router := routing.Configured()
	env := itinerary.Env{
		SuggestionsEnabled: os.Getenv("MOBILE_JOURNEY_SUGGESTIONS_ENABLED") != "0",
		CapacityAvailable:  true,
	}
	if router != nil {
		if !routing.TrySlot() {
			env.CapacityAvailable = false
		} else {
			defer routing.ReleaseSlot()
		}
	}
	return itinerary.Build(draft, router, env)
}

func itineraryReplay(stored idem.StoredResponse) endpoint.Reply {
	headers := [][2]string{
		{idem.ReplayHeaderName, "true"},
		{"Cache-Control", "no-store"},
	}
	if !(stored.Status < 200 || stored.Status == 204 || stored.Status == 304) {
		headers = append(headers, [2]string{"content-length", strconv.Itoa(len(stored.Body))})
	}
	if stored.MediaType != nil && *stored.MediaType != "" {
		media := *stored.MediaType
		if strings.HasPrefix(media, "text/") && !strings.Contains(strings.ToLower(media), "charset=") {
			media += "; charset=utf-8"
		}
		headers = append(headers, [2]string{"content-type", media})
	}
	return endpoint.Reply{Raw: &guestweb.Response{Status: stored.Status, Headers: headers, Body: stored.Body}}
}

func previewOutingItinerary() Route {
	return Route{ID: "POST /outings/{outing_id}/itinerary/preview", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := itineraryPreviewRequest(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := spendItineraryLimiter(call); err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		draft, err := outingsteps.PreviewOutingItinerary(store, outingID, request, outingActor(call))
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		preview, err := previewItinerary(draft)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireItineraryPreview(preview), Headers: cacheControlNoStore}, nil
	}}
}

func replaceOutingItinerary() Route {
	return Route{ID: "PUT /outings/{outing_id}/itinerary", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if stored, ok := idem.AuthorizedReplay(call.Request.Context()); ok {
			if _, err := outingsteps.AuthorizeOutingItinerary(store, outingID, outingActor(call)); err != nil {
				return endpoint.Reply{}, outingRefusal(err)
			}
			return itineraryReplay(stored), nil
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := itineraryRequest(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.ReplaceOutingItinerary(store, outingID, request, outingActor(call))
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireOutingView(view), Headers: cacheControlNoStore}, nil
	}}
}

func createOuting() Route {
	return Route{ID: "POST /contexts/{context_id}/outings", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := outingCreateRequest(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.CreateOuting(store, contextID, request, outingActor(call), time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireOutingView(view)}, nil
	}}
}

func listContextOutings() Route {
	return Route{ID: "GET /contexts/{context_id}/outings", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.ListContextOutings(store, contextID, outingActor(call))
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireOutingList(view)}, nil
	}}
}

func replaceOutingTimeline() Route {
	return Route{ID: "PUT /outings/{outing_id}/timeline", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := timelineRequest(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.ReplaceOutingTimeline(store, outingID, request, outingActor(call))
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireOutingView(view)}, nil
	}}
}

func checkInToStop() Route {
	return Route{ID: "POST /outing-stops/{stop_id}/checkins", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		stopID, err := pathUUID(call, "stop_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.CheckInToStop(store, stopID, outingActor(call), time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireCheckinView(view)}, nil
	}}
}

func listOutingCheckins() Route {
	return Route{ID: "GET /outings/{outing_id}/checkins", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.ListOutingCheckins(store, outingID, outingActor(call))
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireCheckinList(view)}, nil
	}}
}

func createOutingInvite() Route {
	return Route{ID: "POST /outings/{outing_id}/invites", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := inviteCreateRequest(body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.CreateOutingInvite(store, inviteSecrets{}, outingID, request, outingActor(call), time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireInviteView(view)}, nil
	}}
}

func revokeOutingInvite() Route {
	return Route{ID: "POST /outings/{outing_id}/invites/{invite_id}/revoke", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		inviteID, err := pathUUID(call, "invite_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.RevokeOutingInvite(store, outingID, inviteID, outingActor(call), time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireInviteView(view)}, nil
	}}
}

func rotateOutingInvite() Route {
	return Route{ID: "POST /outings/{outing_id}/invites/{invite_id}/rotate", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		outingID, err := pathUUID(call, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		inviteID, err := pathUUID(call, "invite_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.RotateOutingInviteSecret(store, inviteSecrets{}, outingID, inviteID, outingActor(call), time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireInviteView(view)}, nil
	}}
}

func acceptOutingInvite() Route {
	return Route{ID: "POST /outing-invites/{token}/accept", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		token, err := stringParam(call, "token")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newOutingStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := outingsteps.AcceptOutingInvite(store, auth.TokenDigest(token), outingActor(call), time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, outingRefusal(err)
		}
		return endpoint.Reply{Body: wireInviteAccept(view)}, nil
	}}
}
