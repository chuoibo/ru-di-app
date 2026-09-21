// Package collection is the Go port of app/domain/collection.py: the state
// machine of a collection batch (product spec section 8).
//
// POST /batches reaches Transition("accruing", "freeze") and POST
// /batches/{batch_id}/publish reaches UnmetPublishGates and Transition(status,
// "publish"). TerminalStateFor is reached through the "close" event, and
// Progress, IsStale and CountsTowardCollectionRate by no W4 route; they are
// ported with the module because they are the same few lines of the same
// rules, and the goldens replay all of them.
//
// Python reads a context dict by truthiness. Context carries the facts the
// dict can hold: a non-empty obligation list, two flags that are truthy,
// capability_exposed_at being not None, and consent being truthy.
//
// testdata/python_*.json is rendered from the real module by
// scripts/render_domain_w4_goldens.py and replayed by oracle_test.go.
package collection

import (
	"slices"
	"strings"
	"time"
)

// The states of STATES, in its order.
const (
	StateAccruing             = "accruing"
	StateFrozen               = "frozen"
	StatePublished            = "published"
	StateCollecting           = "collecting"
	StateCompleted            = "completed"
	StateClosedWithExceptions = "closed_with_exceptions"
	StateCancelled            = "cancelled"
)

// Codes carried by *CollectionError.
const (
	CodeUnknownState                    = "UNKNOWN_STATE"
	CodeIllegalTransition               = "ILLEGAL_TRANSITION"
	CodeNoObligations                   = "NO_OBLIGATIONS"
	CodeCancelAfterExposureNeedsConsent = "CANCEL_AFTER_EXPOSURE_NEEDS_CONSENT"
	CodeObligationsStillOpen            = "OBLIGATIONS_STILL_OPEN"
)

// The requirement and gate names, lower case as the lists hold them.
const (
	RequirementNoObligations    = "no_obligations"
	GateAdvancerAcknowledgement = "advancer_acknowledgement_required"
	GateDeliveryMethod          = "delivery_method_required"
)

var states = []string{
	StateAccruing, StateFrozen, StatePublished, StateCollecting,
	StateCompleted, StateClosedWithExceptions, StateCancelled,
}

// allowed is _ALLOWED; a "close" target is decided by TerminalStateFor.
var allowed = map[string]map[string]string{
	StateAccruing:             {"freeze": StateFrozen, "cancel": StateCancelled},
	StateFrozen:               {"publish": StatePublished, "reopen": StateAccruing, "cancel": StateCancelled},
	StatePublished:            {"expose_capability": StateCollecting, "cancel": StateCancelled},
	StateCollecting:           {"close": "", "cancel": StateCancelled},
	StateCompleted:            {},
	StateClosedWithExceptions: {},
	StateCancelled:            {},
}

// States is STATES, in its order. The slice is a copy.
func States() []string { return slices.Clone(states) }

// CollectionError mirrors CollectionError: Error returns the code.
type CollectionError struct {
	Code string
}

func (e *CollectionError) Error() string { return e.Code }

// Obligation is one obligation dict: who owes, and its derived status.
type Obligation struct {
	SenderID string
	Status   string
}

// Context is the context dict of Transition.
type Context struct {
	Obligations                 []Obligation
	AdvancerAcknowledged        bool
	DeliveryMethodChosen        bool
	CapabilityExposed           bool
	AllAffectedPartiesConsented bool
}

// UnmetFreezeRequirements is unmet_freeze_requirements.
func UnmetFreezeRequirements(ctx Context) []string {
	unmet := []string{}
	if len(ctx.Obligations) == 0 {
		unmet = append(unmet, RequirementNoObligations)
	}
	return unmet
}

// UnmetPublishGates is unmet_publish_gates, in its order.
func UnmetPublishGates(ctx Context) []string {
	unmet := []string{}
	if !ctx.AdvancerAcknowledged {
		unmet = append(unmet, GateAdvancerAcknowledgement)
	}
	if !ctx.DeliveryMethodChosen {
		unmet = append(unmet, GateDeliveryMethod)
	}
	return unmet
}

// Transition is transition: the next state, or *CollectionError. An unmet
// requirement or gate is refused with its name upper-cased, as Python does.
func Transition(state, event string, ctx Context) (string, error) {
	events, known := allowed[state]
	if !known {
		return "", &CollectionError{Code: CodeUnknownState}
	}
	next, ok := events[event]
	if !ok {
		return "", &CollectionError{Code: CodeIllegalTransition}
	}
	if event == "freeze" {
		if unmet := UnmetFreezeRequirements(ctx); len(unmet) > 0 {
			return "", &CollectionError{Code: strings.ToUpper(unmet[0])}
		}
	}
	if event == "publish" {
		if unmet := UnmetPublishGates(ctx); len(unmet) > 0 {
			return "", &CollectionError{Code: strings.ToUpper(unmet[0])}
		}
	}
	if event == "cancel" && ctx.CapabilityExposed && !ctx.AllAffectedPartiesConsented {
		return "", &CollectionError{Code: CodeCancelAfterExposureNeedsConsent}
	}
	if event == "close" {
		return TerminalStateFor(ctx.Obligations)
	}
	return next, nil
}

func isException(status string) bool {
	return status == "waived" || status == "disputed" || status == "cancelled"
}

func isTransferred(status string) bool {
	return status == "confirmed" || status == "over_confirmed"
}

// TerminalStateFor is terminal_state_for: completed only when every
// obligation ended cleanly, closed_with_exceptions when some were waived,
// disputed or cancelled; the first other status is OBLIGATIONS_STILL_OPEN.
func TerminalStateFor(obligations []Obligation) (string, error) {
	if len(obligations) == 0 {
		return "", &CollectionError{Code: CodeNoObligations}
	}
	hasException := false
	for _, obligation := range obligations {
		switch {
		case isException(obligation.Status):
			hasException = true
		case !isTransferred(obligation.Status):
			return "", &CollectionError{Code: CodeObligationsStillOpen}
		}
	}
	if hasException {
		return StateClosedWithExceptions, nil
	}
	return StateCompleted, nil
}

// Progress is progress's dict.
type Progress struct {
	TransfersDone  int
	TransfersTotal int
	PeopleDone     int
	PeopleTotal    int
}

// ProgressOf is progress: transfers first, people second. A waived or
// cancelled obligation leaves a person nothing to do but is not a transfer.
func ProgressOf(obligations []Obligation) Progress {
	progress := Progress{TransfersTotal: len(obligations)}
	finished := map[string]bool{}
	var people []string
	for _, obligation := range obligations {
		if isTransferred(obligation.Status) {
			progress.TransfersDone++
		}
		done, seen := finished[obligation.SenderID]
		if !seen {
			people = append(people, obligation.SenderID)
			done = true
		}
		finished[obligation.SenderID] = done && (isTransferred(obligation.Status) ||
			obligation.Status == "waived" || obligation.Status == "cancelled")
	}
	progress.PeopleTotal = len(people)
	for _, person := range people {
		if finished[person] {
			progress.PeopleDone++
		}
	}
	return progress
}

// staleDue and staleQuiet are the two timedeltas of is_stale.
const (
	staleDue   = 14 * 24 * time.Hour
	staleQuiet = 7 * 24 * time.Hour
)

// IsStale is is_stale: past due by more than 14 days and quiet for more than
// 7. time.Time.Sub saturates at the bounds of time.Duration (about 292
// years); a saturated difference is still on the same side of both
// thresholds, so the answer is Python's for every datetime Python can hold.
func IsStale(now, dueAt, lastMeaningfulActivityAt time.Time) bool {
	return now.Sub(dueAt) > staleDue && now.Sub(lastMeaningfulActivityAt) > staleQuiet
}

// CountsTowardCollectionRate is counts_toward_collection_rate: a batch whose
// capability was exposed always counts; otherwise every state but cancelled.
// state "" stands for a batch dict without one.
func CountsTowardCollectionRate(state string, capabilityExposed bool) bool {
	if capabilityExposed {
		return true
	}
	return state != StateCancelled
}
