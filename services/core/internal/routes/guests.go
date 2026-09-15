package routes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/pyuuid"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	guestweb "mobile/services/core/internal/web/guest"
)

// The seven routes of app/api/routes/guests.py. A guest has no actor: the
// token in the path is the capability, and ApiService reads it through
// token_digest. Each handler is the Python one step for step: the pure steps
// and the framing live in package web/guest, the statements in repo/guests.go.
//
// Every read of the envelope is get_guest_envelope at its own _now(), and the
// store is reached only when Python first queries, so a request refused before
// that (a form value uuid.UUID rejects, an unknown reason) sends nothing.
// A page commits only when it answers 200: every refusal rolls back, the first
// open and an expiry flip included.

// guestLink is one request's token and the transaction it reads through.
type guestLink struct {
	ctx   context.Context
	call  *endpoint.Call
	token string
}

func newGuestLink(ctx context.Context, call *endpoint.Call) (guestLink, error) {
	token, err := stringParam(call, "token")
	return guestLink{ctx: ctx, call: call, token: token}, err
}

func (g guestLink) digest() []byte { return auth.TokenDigest(g.token) }

// envelope is get_guest_envelope(token_digest(token), _now()): the dict the
// web steps read, and whether a link carries the digest.
func (g guestLink) envelope() (map[string]any, bool, error) {
	store, err := groupStore(g.ctx, g.call)
	if err != nil {
		return nil, false, err
	}
	record, err := store.GetGuestEnvelope(g.ctx, g.digest(), time.Now().UTC())
	if err != nil || record == nil {
		return nil, false, err
	}
	e := record.Envelope
	obligations := make([]guestweb.EnvelopeObligation, len(e.Obligations))
	for i, o := range e.Obligations {
		obligations[i] = guestweb.EnvelopeObligation{
			ObligationID:         o.ObligationID,
			OccasionLabel:        o.OccasionLabel,
			AmountVND:            money.VND(o.AmountVND),
			RecipientDisplayName: o.RecipientDisplayName,
			AlreadyReported:      o.AlreadyReported,
			EvidenceRequested:    o.EvidenceRequested,
			Disputed:             o.Disputed,
			ObjectionsUsed:       o.ObjectionsUsed,
			ObjectionsAllowed:    o.ObjectionsAllowed,
			ReceiverConfirmed:    o.ReceiverConfirmed,
		}
	}
	return guestweb.Envelope{
		RecordedByDisplayName:    e.RecordedByDisplayName,
		ClaimedPersonDisplayName: e.ClaimedPersonDisplayName,
		LinkState:                e.LinkState,
		Obligations:              obligations,
		ReportsUsed:              e.ReportsUsed,
		ReportsAllowed:           e.ReportsAllowed,
		ObjectionsUsed:           e.ObjectionsUsed,
		ObjectionsAllowed:        e.ObjectionsAllowed,
	}.Dict(), true, nil
}

// recordObjection is ApiService.record_objection: the gate, which loads the
// envelope only once kind and reason pass, then save_guest_objection at a
// _now() of its own.
func (g guestLink) recordObjection(kind string, obligationID, reason *string) error {
	if err := guestStep(guestweb.RecordObjection(g.token, kind, obligationID, reason, g.envelope)); err != nil {
		return err
	}
	store, err := groupStore(g.ctx, g.call)
	if err != nil {
		return err
	}
	return store.SaveGuestObjection(g.ctx, repo.GuestObjectionInput{
		TokenDigest: g.digest(), Kind: kind, ObligationID: obligationID, Reason: reason, Now: time.Now().UTC(),
	})
}

// guestStep turns a web step's answer into the ApiProblem it raises.
func guestStep(refusal *guestweb.Refusal, err error) error {
	if err != nil {
		return err
	}
	if refusal != nil {
		return endpoint.Refuse(refusal.Status, refusal.Code, refusal.Detail)
	}
	return nil
}

func rawReply(response guestweb.Response, err error) (endpoint.Reply, error) {
	if err != nil {
		return endpoint.Reply{}, err
	}
	return endpoint.Reply{Raw: &response}, nil
}

// formUUID is uuid.UUID(value) in the handler, before the service runs:
// laxer than pydantic (braces, "urn:uuid:", "0x", underscores, Unicode
// digits), and a ValueError it raises is not caught, so a value it refuses
// ends the request as a plain 500 before any repository call.
func formUUID(value string) (string, error) {
	canonical, ok := pyuuid.Parse(value)
	if !ok {
		return "", errors.New("routes: uuid.UUID raised ValueError on a form obligation_id")
	}
	return canonical, nil
}

// guestPage is GET /g/{token} (guest_page, ApiService.guest_view).
func guestPage() Route {
	return Route{ID: "GET /g/{token}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		envelope, found, err := link.envelope()
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, refusal, err := guestweb.GuestView(link.token, envelope, found)
		if err := guestStep(refusal, err); err != nil {
			return endpoint.Reply{}, err
		}
		return rawReply(guestweb.GuestPage(view, link.token))
	}}
}

// guestReportPayment is POST /g/{token}/da-chuyen (report_payment,
// ApiService.report_payment): the target locked FOR UPDATE (404 before any
// permission), the capability and the report budget, then the report, whose
// body idempotency_key is unique across the table. The obligation status is
// computed before the Accept header is read, so a browser is redirected only
// once the answer it would have been given was built.
func guestReportPayment() Route {
	return Route{ID: "POST /g/{token}/da-chuyen", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		obligationID, err := uuidField(body, "obligation_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		key, err := optionalUUIDField(body, "idempotency_key")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		now := time.Now().UTC()
		target, err := store.GetPaymentReportTarget(ctx, link.digest(), obligationID, now)
		if err != nil {
			return endpoint.Reply{}, err
		}
		var gate *guestweb.PaymentReportTarget
		if target != nil {
			gate = &guestweb.PaymentReportTarget{ActiveCapability: target.ActiveCapability, ReportsUsed: target.ReportsUsed}
		}
		if err := guestStep(guestweb.PaymentReportGate(link.token, gate)); err != nil {
			return endpoint.Reply{}, err
		}
		if key == nil {
			minted, err := repo.NewUUID()
			if err != nil {
				return endpoint.Reply{}, err
			}
			key = &minted
		}
		record, err := store.SavePaymentReport(ctx, repo.PaymentReportInput{Target: *target, IdempotencyKey: *key, Now: now})
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			refused := guestweb.PaymentReportConflict(conflict.Code)
			return endpoint.Reply{}, endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		receipts := make([]money.VND, len(record.ReceiptAmountsVND))
		for i, amount := range record.ReceiptAmountsVND {
			receipts[i] = money.VND(amount)
		}
		status, err := guestweb.PaymentReportStatus(money.VND(target.AmountVND), receipts)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if guestweb.AcceptsHTML(call.Request.Header.Values("Accept")) {
			return rawReply(guestweb.SeeOther(guestweb.GuestPageURL(link.token)), nil)
		}
		out := pyjson.NewOrderedMap()
		out.Set("payment_report_id", pyjson.String(record.ID))
		out.Set("obligation_id", pyjson.String(record.ObligationID))
		out.Set("amount_vnd", pyjson.NewInt(record.AmountVND))
		out.Set("obligation_status", pyjson.String(status))
		return endpoint.Reply{Body: out}, nil
	}}
}

// guestNotMePage is GET /g/{token}/khong-phai-toi (not_me_page,
// ApiService.not_me_view).
func guestNotMePage() Route {
	return Route{ID: "GET /g/{token}/khong-phai-toi", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		envelope, found, err := link.envelope()
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, refusal, err := guestweb.NotMeView(link.token, envelope, found)
		if err := guestStep(refusal, err); err != nil {
			return endpoint.Reply{}, err
		}
		return rawReply(guestweb.NotMePage(view, link.token))
	}}
}

// guestNotMeSubmit is POST /g/{token}/khong-phai-toi (not_me_submit): the
// names are read first, under the row locks that load takes, because the
// objection revokes the link; then the objection loads the envelope again.
// The body is never read.
func guestNotMeSubmit() Route {
	return Route{ID: "POST /g/{token}/khong-phai-toi", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		envelope, found, err := link.envelope()
		if err != nil {
			return endpoint.Reply{}, err
		}
		seen, refusal, err := guestweb.NotMeView(link.token, envelope, found)
		if err := guestStep(refusal, err); err != nil {
			return endpoint.Reply{}, err
		}
		if err := link.recordObjection("not_me", nil, nil); err != nil {
			return endpoint.Reply{}, err
		}
		view, err := guestweb.NotMeSubmittedView(seen)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return rawReply(guestweb.NotMePage(view, link.token))
	}}
}

// guestWrongAmountPage is GET /g/{token}/doi-so-tien (wrong_amount_page).
// obligation_id is a plain str compared byte for byte with the canonical ids,
// never parsed. Without it, the guest view picks the first block (and 409s on
// none) before the page loads the envelope again.
func guestWrongAmountPage() Route {
	return Route{ID: "GET /g/{token}/doi-so-tien", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		var obligationID string
		switch value := call.Values["obligation_id"].(type) {
		case pyjson.String:
			obligationID = string(value)
		case pyjson.Null, nil:
			envelope, found, err := link.envelope()
			if err != nil {
				return endpoint.Reply{}, err
			}
			view, refusal, err := guestweb.GuestView(link.token, envelope, found)
			if err := guestStep(refusal, err); err != nil {
				return endpoint.Reply{}, err
			}
			first, refusal, err := guestweb.DefaultObligationID(view)
			if err := guestStep(refusal, err); err != nil {
				return endpoint.Reply{}, err
			}
			obligationID = first
		default:
			return endpoint.Reply{}, fmt.Errorf("routes: query obligation_id is %T, not a string or None", value)
		}
		envelope, found, err := link.envelope()
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, refusal, err := guestweb.WrongAmountView(link.token, envelope, found, obligationID)
		if err := guestStep(refusal, err); err != nil {
			return endpoint.Reply{}, err
		}
		return rawReply(guestweb.WrongAmountPage(view, link.token))
	}}
}

// guestWrongAmountSubmit is POST /g/{token}/doi-so-tien (wrong_amount_submit):
// uuid.UUID on the form value, then record_objection, then 303 to the page.
func guestWrongAmountSubmit() Route {
	return Route{ID: "POST /g/{token}/doi-so-tien", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		raw, err := bodyString(call, "obligation_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		reason, err := bodyString(call, "reason")
		if err != nil {
			return endpoint.Reply{}, err
		}
		obligationID, err := formUUID(raw)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := link.recordObjection("wrong_amount", &obligationID, &reason); err != nil {
			return endpoint.Reply{}, err
		}
		return rawReply(guestweb.SeeOther(guestweb.GuestPageURL(link.token)), nil)
	}}
}

// guestRequestEvidence is POST /g/{token}/xin-cach-tinh (request_evidence):
// uuid.UUID on the form value, record_objection with no reason, then 303 to
// the wrong-amount page carrying the form value as it was posted.
func guestRequestEvidence() Route {
	return Route{ID: "POST /g/{token}/xin-cach-tinh", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		link, err := newGuestLink(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		raw, err := bodyString(call, "obligation_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		obligationID, err := formUUID(raw)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := link.recordObjection("evidence_request", &obligationID, nil); err != nil {
			return endpoint.Reply{}, err
		}
		return rawReply(guestweb.SeeOther(guestweb.EvidenceRequestedURL(link.token, raw)), nil)
	}}
}
