package guest

import (
	"sort"

	"mobile/services/core/internal/domain/money"
)

// ViewError is GuestViewError: what build_guest_view and format_vnd refuse.
type ViewError struct{ Code string }

func (e *ViewError) Error() string { return e.Code }

// ObjectionError is app/web/objection_view.py ObjectionError.
type ObjectionError struct{ Code string }

func (e *ObjectionError) Error() string { return e.Code }

// AllowedTopLevel is ALLOWED_TOP_LEVEL, sorted.
var AllowedTopLevel = []string{"blocks", "can_object", "can_report_payment", "claimed_person_display_name", "link_state", "recorded_by_display_name"}

// AllowedBlock is ALLOWED_BLOCK, sorted.
var AllowedBlock = []string{"already_reported", "amount_display", "amount_vnd", "can_object", "disputed", "obligation_id", "occasion_label", "receiver_confirmed", "recipient_display_name"}

// ForbiddenInputKeys is FORBIDDEN_INPUT_KEYS, sorted; objection_view._guard
// holds the same six names.
var ForbiddenInputKeys = []string{"group_balance", "group_history", "invocation_thread", "member_list", "original_bill_url", "other_allocations"}

// AllowedNotMe is ALLOWED_NOT_ME, sorted.
var AllowedNotMe = []string{"already_reported", "can_object", "claimed_person_display_name", "recorded_by_display_name"}

// AllowedWrongAmount is ALLOWED_WRONG_AMOUNT, sorted.
var AllowedWrongAmount = []string{"amount_display", "can_object", "can_request_evidence", "claimed_person_display_name", "evidence_requested", "obligation_id", "occasion_label", "reasons", "recorded_by_display_name"}

// ObjectionReasons is OBJECTION_REASONS, in order: (value, label).
var ObjectionReasons = [][2]string{
	{"amount_too_high", "Số tiền cao hơn phần của tôi"},
	{"did_not_join", "Tôi không tham gia khoản này"},
	{"already_paid", "Tôi đã chuyển rồi"},
	{"split_wrong", "Chia sai người"},
	{"other", "Lý do khác"},
}

// NeutralPreview is NEUTRAL_PREVIEW.
func NeutralPreview() map[string]any {
	return map[string]any{
		"title":       "Chi tiết khoản cần gửi",
		"description": "Mở để xem phần của bạn và cách chuyển.",
	}
}

// linkStates are the states build_guest_view accepts.
var linkStates = map[string]bool{"active": true, "revoked": true, "expired": true, "rotated": true}

// maxStrDigits is sys.get_int_max_str_digits() in the parity image: an int
// with more decimal digits cannot be formatted.
const maxStrDigits = 4300

// FormatVND is format_vnd: 82000 -> "82.000". A bool or a non-int is
// AMOUNT_NOT_INTEGER, a negative int NEGATIVE_AMOUNT, and an int past
// Python's digit limit the ValueError int formatting raises.
func FormatVND(amount any) (string, error) {
	if _, isBool := amount.(bool); isBool {
		return "", &ViewError{Code: "AMOUNT_NOT_INTEGER"}
	}
	n, ok := pyInt(amount)
	if !ok {
		if err := checkSupported(amount); err != nil {
			return "", err
		}
		return "", &ViewError{Code: "AMOUNT_NOT_INTEGER"}
	}
	if n.Sign() < 0 {
		return "", &ViewError{Code: "NEGATIVE_AMOUNT"}
	}
	digits := n.String()
	if len(digits) > maxStrDigits {
		return "", &PyError{Class: "ValueError", Message: "Exceeds the limit (4300 digits) for integer string conversion"}
	}
	out := make([]byte, 0, len(digits)+len(digits)/3)
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, digits[i])
	}
	return string(out), nil
}

func hasForbidden(envelope map[string]any) bool {
	for _, key := range ForbiddenInputKeys {
		if _, found := envelope[key]; found {
			return true
		}
	}
	return false
}

func extraKeys(view map[string]any, allowed []string) bool {
	for key := range view {
		i := sort.SearchStrings(allowed, key)
		if i == len(allowed) || allowed[i] != key {
			return true
		}
	}
	return false
}

// BuildGuestView is build_guest_view. It evaluates the envelope in Python's
// order, so a malformed envelope raises what Python raises first.
func BuildGuestView(envelope map[string]any) (map[string]any, error) {
	if hasForbidden(envelope) {
		return nil, &ViewError{Code: "FORBIDDEN_FIELD_IN_INPUT"}
	}
	stateValue, err := get(envelope, "link_state", nil)
	if err != nil {
		return nil, err
	}
	switch stateValue.(type) {
	case []any, map[string]any:
		return nil, typeError("unhashable type: '%s'", typeName(stateValue))
	}
	state, isStr := stateValue.(string)
	if !isStr || !linkStates[state] {
		return nil, &ViewError{Code: "UNKNOWN_LINK_STATE"}
	}

	if state != "active" {
		recorded, err := item(envelope, "recorded_by_display_name")
		if err != nil {
			return nil, err
		}
		claimed, err := item(envelope, "claimed_person_display_name")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"recorded_by_display_name":    recorded,
			"claimed_person_display_name": claimed,
			"blocks":                      []any{},
			"link_state":                  state,
			"can_report_payment":          false,
			"can_object":                  false,
		}, nil
	}

	obligations, err := item(envelope, "obligations")
	if err != nil {
		return nil, err
	}
	rows, err := iterate(obligations)
	if err != nil {
		return nil, err
	}
	blocks := []any{}
	anyCanObject := false
	for _, obligation := range rows {
		block, err := guestBlock(obligation)
		if err != nil {
			return nil, err
		}
		if extraKeys(block, AllowedBlock) {
			return nil, &ViewError{Code: "BLOCK_FIELD_NOT_ALLOWED"}
		}
		if block["can_object"] == true {
			anyCanObject = true
		}
		blocks = append(blocks, block)
	}

	recorded, err := item(envelope, "recorded_by_display_name")
	if err != nil {
		return nil, err
	}
	claimed, err := item(envelope, "claimed_person_display_name")
	if err != nil {
		return nil, err
	}
	used, err := get(envelope, "reports_used", int64(0))
	if err != nil {
		return nil, err
	}
	allowed, err := get(envelope, "reports_allowed", int64(3))
	if err != nil {
		return nil, err
	}
	canReport, err := less(used, allowed)
	if err != nil {
		return nil, err
	}
	view := map[string]any{
		"recorded_by_display_name":    recorded,
		"claimed_person_display_name": claimed,
		"blocks":                      blocks,
		"link_state":                  state,
		"can_report_payment":          canReport,
		"can_object":                  len(blocks) > 0 && anyCanObject,
	}
	if extraKeys(view, AllowedTopLevel) {
		return nil, &ViewError{Code: "TOP_LEVEL_FIELD_NOT_ALLOWED"}
	}
	return view, nil
}

func guestBlock(obligation any) (map[string]any, error) {
	id, err := item(obligation, "obligation_id")
	if err != nil {
		return nil, err
	}
	m := obligation.(map[string]any)
	label, err := item(m, "occasion_label")
	if err != nil {
		return nil, err
	}
	amount, err := item(m, "amount_vnd")
	if err != nil {
		return nil, err
	}
	display, err := FormatVND(amount)
	if err != nil {
		return nil, err
	}
	recipient, err := item(m, "recipient_display_name")
	if err != nil {
		return nil, err
	}
	flags := map[string]bool{}
	for _, key := range []string{"already_reported", "receiver_confirmed", "disputed"} {
		raw, err := get(m, key, nil)
		if err != nil {
			return nil, err
		}
		if flags[key], err = truthy(raw); err != nil {
			return nil, err
		}
	}
	used, err := get(m, "objections_used", int64(0))
	if err != nil {
		return nil, err
	}
	allowed, err := get(m, "objections_allowed", int64(3))
	if err != nil {
		return nil, err
	}
	canObject, err := less(used, allowed)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"obligation_id":          id,
		"occasion_label":         label,
		"amount_vnd":             amount,
		"amount_display":         display,
		"recipient_display_name": recipient,
		"already_reported":       flags["already_reported"],
		"receiver_confirmed":     flags["receiver_confirmed"],
		"disputed":               flags["disputed"],
		"can_object":             canObject,
	}, nil
}

// guard is objection_view._guard.
func guard(envelope map[string]any) error {
	if hasForbidden(envelope) {
		return &ObjectionError{Code: "FORBIDDEN_FIELD_IN_INPUT"}
	}
	state, err := get(envelope, "link_state", nil)
	if err != nil {
		return err
	}
	if !equalStr(state, "active") {
		return &ObjectionError{Code: "LINK_NOT_ACTIVE"}
	}
	return nil
}

// BuildNotMeView is build_not_me_view.
func BuildNotMeView(envelope map[string]any) (map[string]any, error) {
	if err := guard(envelope); err != nil {
		return nil, err
	}
	claimed, err := item(envelope, "claimed_person_display_name")
	if err != nil {
		return nil, err
	}
	recorded, err := item(envelope, "recorded_by_display_name")
	if err != nil {
		return nil, err
	}
	reportedRaw, err := get(envelope, "not_me_reported", nil)
	if err != nil {
		return nil, err
	}
	reported, err := truthy(reportedRaw)
	if err != nil {
		return nil, err
	}
	used, err := item(envelope, "objections_used")
	if err != nil {
		return nil, err
	}
	allowed, err := item(envelope, "objections_allowed")
	if err != nil {
		return nil, err
	}
	canObject, err := less(used, allowed)
	if err != nil {
		return nil, err
	}
	view := map[string]any{
		"claimed_person_display_name": claimed,
		"recorded_by_display_name":    recorded,
		"already_reported":            reported,
		"can_object":                  canObject,
	}
	if extraKeys(view, AllowedNotMe) {
		return nil, &ObjectionError{Code: "FIELD_NOT_ALLOWED"}
	}
	return view, nil
}

// BuildWrongAmountView is build_wrong_amount_view. A negative stored amount
// raises *ViewError here, not *ObjectionError, as format_vnd does in Python.
func BuildWrongAmountView(envelope map[string]any, obligationID string) (map[string]any, error) {
	if err := guard(envelope); err != nil {
		return nil, err
	}
	obligations, err := item(envelope, "obligations")
	if err != nil {
		return nil, err
	}
	rows, err := iterate(obligations)
	if err != nil {
		return nil, err
	}
	var match map[string]any
	for _, row := range rows {
		id, err := item(row, "obligation_id")
		if err != nil {
			return nil, err
		}
		if match == nil && equalStr(id, obligationID) {
			match = row.(map[string]any)
		}
	}
	if match == nil {
		return nil, &ObjectionError{Code: "UNKNOWN_OBLIGATION"}
	}

	claimed, err := item(envelope, "claimed_person_display_name")
	if err != nil {
		return nil, err
	}
	recorded, err := item(envelope, "recorded_by_display_name")
	if err != nil {
		return nil, err
	}
	label, err := item(match, "occasion_label")
	if err != nil {
		return nil, err
	}
	display, err := get(match, "amount_display", nil)
	if err != nil {
		return nil, err
	}
	shown, err := truthy(display)
	if err != nil {
		return nil, err
	}
	if !shown {
		amount, err := item(match, "amount_vnd")
		if err != nil {
			return nil, err
		}
		if display, err = FormatVND(amount); err != nil {
			return nil, err
		}
	}
	// dict.get evaluates its fallback first, so the envelope's counts are read
	// even when the obligation carries its own.
	usedFallback, err := item(envelope, "objections_used")
	if err != nil {
		return nil, err
	}
	used, err := get(match, "objections_used", usedFallback)
	if err != nil {
		return nil, err
	}
	allowedFallback, err := item(envelope, "objections_allowed")
	if err != nil {
		return nil, err
	}
	allowed, err := get(match, "objections_allowed", allowedFallback)
	if err != nil {
		return nil, err
	}
	canObject, err := less(used, allowed)
	if err != nil {
		return nil, err
	}
	askedRaw, err := get(match, "evidence_requested", nil)
	if err != nil {
		return nil, err
	}
	asked, err := truthy(askedRaw)
	if err != nil {
		return nil, err
	}
	reasons := make([]any, len(ObjectionReasons))
	for i, reason := range ObjectionReasons {
		reasons[i] = []any{reason[0], reason[1]}
	}
	view := map[string]any{
		"claimed_person_display_name": claimed,
		"recorded_by_display_name":    recorded,
		"occasion_label":              label,
		"amount_display":              display,
		"obligation_id":               obligationID,
		"can_object":                  canObject,
		"can_request_evidence":        !asked,
		"evidence_requested":          asked,
		"reasons":                     reasons,
	}
	if extraKeys(view, AllowedWrongAmount) {
		return nil, &ObjectionError{Code: "FIELD_NOT_ALLOWED"}
	}
	return view, nil
}

// Envelope is the dict get_guest_envelope builds in
// SqlAlchemyApiRepository, typed; Dict turns it into what the builders read.
type Envelope struct {
	RecordedByDisplayName    string
	ClaimedPersonDisplayName string
	LinkState                string
	Obligations              []EnvelopeObligation
	ReportsUsed              int64
	ReportsAllowed           int64
	ObjectionsUsed           int64
	ObjectionsAllowed        int64
}

// EnvelopeObligation is one entry of the envelope's obligations.
type EnvelopeObligation struct {
	ObligationID         string
	OccasionLabel        string
	AmountVND            money.VND
	RecipientDisplayName string
	AlreadyReported      bool
	EvidenceRequested    bool
	Disputed             bool
	ObjectionsUsed       int64
	ObjectionsAllowed    int64
	ReceiverConfirmed    bool
}

// Dict is the envelope as the Python dict the repository returns.
func (e Envelope) Dict() map[string]any {
	obligations := make([]any, len(e.Obligations))
	for i, o := range e.Obligations {
		obligations[i] = map[string]any{
			"obligation_id":          o.ObligationID,
			"occasion_label":         o.OccasionLabel,
			"amount_vnd":             int64(o.AmountVND),
			"recipient_display_name": o.RecipientDisplayName,
			"already_reported":       o.AlreadyReported,
			"evidence_requested":     o.EvidenceRequested,
			"disputed":               o.Disputed,
			"objections_used":        o.ObjectionsUsed,
			"objections_allowed":     o.ObjectionsAllowed,
			"receiver_confirmed":     o.ReceiverConfirmed,
		}
	}
	return map[string]any{
		"recorded_by_display_name":    e.RecordedByDisplayName,
		"claimed_person_display_name": e.ClaimedPersonDisplayName,
		"link_state":                  e.LinkState,
		"obligations":                 obligations,
		"reports_used":                e.ReportsUsed,
		"reports_allowed":             e.ReportsAllowed,
		"objections_used":             e.ObjectionsUsed,
		"objections_allowed":          e.ObjectionsAllowed,
	}
}
