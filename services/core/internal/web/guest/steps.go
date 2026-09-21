package guest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/permissions"
)

// Refusal is an ApiProblem the guest service methods and routes raise.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

// ObjectionKinds is OBJECTION_KINDS; QuotaConsumingObjections is
// QUOTA_CONSUMING_OBJECTIONS (app/api/limits.py).
var (
	ObjectionKinds           = map[string]bool{"not_me": true, "wrong_amount": true, "evidence_request": true}
	QuotaConsumingObjections = map[string]bool{"not_me": true, "wrong_amount": true}
)

// ReportLimit is the literal report_payment compares reports_used with.
const ReportLimit = 3

// GuestActorID is _guest_actor(token).id.
func GuestActorID(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "capability:" + hex.EncodeToString(digest[:])[:16]
}

func guestPermission(action, token string, proven ...string) (*Refusal, error) {
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    GuestActorID(token),
		Roles:      []string{"guest"},
		Proven:     proven,
		Provenance: "api_service",
	})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return &Refusal{Status: 403, Code: "permission_denied", Detail: reason}, nil
	}
	return nil, nil
}

// openEnvelope is the part of guest_view and _objection_envelope after
// get_guest_envelope: 404 for no record, then the capability check.
func openEnvelope(token string, found bool) (*Refusal, error) {
	if !found {
		return &Refusal{Status: 404, Code: "guest_link_not_found", Detail: "Guest link does not exist"}, nil
	}
	return guestPermission("view_guest_envelope", token, "is_own_capability")
}

// GuestView is ApiService.guest_view after get_guest_envelope: envelope is
// record.envelope and found says whether there was a record.
func GuestView(token string, envelope map[string]any, found bool) (map[string]any, *Refusal, error) {
	if refusal, err := openEnvelope(token, found); refusal != nil || err != nil {
		return nil, refusal, err
	}
	view, err := BuildGuestView(envelope)
	var refused *ViewError
	if errors.As(err, &refused) {
		return nil, &Refusal{Status: 409, Code: refused.Code, Detail: "Guest envelope is not renderable"}, nil
	}
	return view, nil, err
}

func objectionPage(token string, found bool, build func() (map[string]any, error)) (map[string]any, *Refusal, error) {
	if refusal, err := openEnvelope(token, found); refusal != nil || err != nil {
		return nil, refusal, err
	}
	view, err := build()
	var refused *ObjectionError
	if errors.As(err, &refused) {
		return nil, &Refusal{Status: 409, Code: refused.Code, Detail: "Objection page is not renderable"}, nil
	}
	return view, nil, err
}

// NotMeView is ApiService.not_me_view after get_guest_envelope.
func NotMeView(token string, envelope map[string]any, found bool) (map[string]any, *Refusal, error) {
	return objectionPage(token, found, func() (map[string]any, error) { return BuildNotMeView(envelope) })
}

// WrongAmountView is ApiService.wrong_amount_view after get_guest_envelope.
// A *ViewError from format_vnd is not caught there and comes back as an
// error (a 500), as in Python.
func WrongAmountView(token string, envelope map[string]any, found bool, obligationID string) (map[string]any, *Refusal, error) {
	return objectionPage(token, found, func() (map[string]any, error) { return BuildWrongAmountView(envelope, obligationID) })
}

// RecordObjection is ApiService.record_objection up to save_guest_objection:
// a nil refusal and nil error mean the route writes the objection. load is
// get_guest_envelope, called only once kind and reason pass, as Python reads
// it. obligationID is str(uuid.UUID) or nil; reason is nil for None.
func RecordObjection(token, kind string, obligationID, reason *string, load func() (map[string]any, bool, error)) (*Refusal, error) {
	if !ObjectionKinds[kind] {
		return &Refusal{Status: 422, Code: "unknown_objection", Detail: "Unknown objection kind"}, nil
	}
	if reason != nil {
		known := false
		for _, r := range ObjectionReasons {
			known = known || r[0] == *reason
		}
		if !known {
			return &Refusal{Status: 422, Code: "unknown_reason", Detail: "Unknown objection reason"}, nil
		}
	}
	envelope, found, err := load()
	if err != nil {
		return nil, err
	}
	if refusal, err := openEnvelope(token, found); refusal != nil || err != nil {
		return refusal, err
	}
	state, err := item(envelope, "link_state")
	if err != nil {
		return nil, err
	}
	if !equalStr(state, "active") {
		return &Refusal{Status: 409, Code: "link_not_active", Detail: "This link is no longer open"}, nil
	}
	// str(None) is "None": the quota lookup below compares with it.
	target := "None"
	if obligationID != nil {
		target = *obligationID
		block, err := firstObligation(envelope, target)
		if err != nil {
			return nil, err
		}
		if block == nil {
			return &Refusal{Status: 404, Code: "unknown_obligation", Detail: "No such obligation on this link"}, nil
		}
	}
	if QuotaConsumingObjections[kind] {
		block, err := firstObligation(envelope, target)
		if err != nil {
			return nil, err
		}
		source := envelope
		if block != nil && len(block) > 0 {
			source = block
		}
		used, err := item(source, "objections_used")
		if err != nil {
			return nil, err
		}
		allowed, err := item(source, "objections_allowed")
		if err != nil {
			return nil, err
		}
		under, err := less(used, allowed)
		if err != nil {
			return nil, err
		}
		if !under {
			return &Refusal{Status: 429, Code: "objection_rate_limited", Detail: "Too many objections on this obligation"}, nil
		}
	}
	return nil, nil
}

// firstObligation is next(item for item in envelope["obligations"] if
// item["obligation_id"] == id): it stops at the first match, so entries after
// it are never read.
func firstObligation(envelope map[string]any, id string) (map[string]any, error) {
	obligations, err := item(envelope, "obligations")
	if err != nil {
		return nil, err
	}
	rows, err := iterate(obligations)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		value, err := item(row, "obligation_id")
		if err != nil {
			return nil, err
		}
		if equalStr(value, id) {
			return row.(map[string]any), nil
		}
	}
	return nil, nil
}

// PaymentReportTarget is what report_payment reads from PaymentReportTarget.
type PaymentReportTarget struct {
	ActiveCapability bool
	ReportsUsed      int64
}

// PaymentReportGate is ApiService.report_payment between
// get_payment_report_target and save_payment_report; target is nil for None.
func PaymentReportGate(token string, target *PaymentReportTarget) (*Refusal, error) {
	if target == nil {
		return &Refusal{Status: 404, Code: "guest_obligation_not_found", Detail: "Obligation is outside this link"}, nil
	}
	proven := []string{"is_own_capability"}
	if target.ActiveCapability {
		proven = append(proven, "active_capability")
	}
	if target.ReportsUsed < ReportLimit {
		proven = append(proven, "report_budget_available")
	}
	return guestPermission("report_payment", token, proven...)
}

// PaymentReportConflict is the ApiProblem for a RepositoryConflict that
// save_payment_report raises with code.
func PaymentReportConflict(code string) *Refusal {
	return &Refusal{Status: 409, Code: strings.ToLower(code), Detail: "Payment report conflicted"}
}

// PaymentReportStatus is the obligation_status report_payment answers with.
// report_payment catches nothing here, so a ledger refusal is an error.
func PaymentReportStatus(amountVND money.VND, receiptsVND []money.VND) (string, error) {
	return ledger.ObligationStatus(amountVND, receiptsVND)
}

// NotMeSubmittedView is the view not_me_submit renders after recording the
// objection, from the view it read first.
func NotMeSubmittedView(seen map[string]any) (map[string]any, error) {
	claimed, err := item(seen, "claimed_person_display_name")
	if err != nil {
		return nil, err
	}
	recorded, err := item(seen, "recorded_by_display_name")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"claimed_person_display_name": claimed,
		"recorded_by_display_name":    recorded,
		"already_reported":            true,
		"can_object":                  false,
	}, nil
}

// DefaultObligationID is wrong_amount_page without obligation_id: the first
// block of the guest view, or 409 no_open_obligation when there is none.
func DefaultObligationID(view map[string]any) (string, *Refusal, error) {
	blocks, err := item(view, "blocks")
	if err != nil {
		return "", nil, err
	}
	nonEmpty, err := truthy(blocks)
	if err != nil {
		return "", nil, err
	}
	if !nonEmpty {
		return "", &Refusal{Status: 409, Code: "no_open_obligation", Detail: "Nothing to dispute on this link"}, nil
	}
	list, ok := blocks.([]any)
	if !ok {
		return "", nil, &UnsupportedError{Reason: "blocks that are not a list"}
	}
	id, err := item(list[0], "obligation_id")
	if err != nil {
		return "", nil, err
	}
	s, ok := id.(string)
	if !ok {
		return "", nil, &UnsupportedError{Reason: "an obligation_id that is not a str"}
	}
	return s, nil, nil
}
