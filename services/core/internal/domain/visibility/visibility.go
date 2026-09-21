// Package visibility ports app.domain.visibility (spec section 10): who can
// see what, and the rule that stops context from leaking. An output is never
// broader than its most sensitive input, unless it was redacted AND the owner
// consented.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image.
//
// # Values
//
// Levels, components and ids are Python str values. Declassify's redaction and
// SettlementView's obligation are Python objects the way encoding/json decodes
// them (nil is None, bool, int64 or int is int, float64 is float, string is
// str, []any is a list or tuple, map[string]any is a dict), because both
// functions pass them through and judge their contents. Datetimes are
// instants: every aware Python datetime with a fixed offset compares as its
// instant.
package visibility

import (
	"fmt"
	"slices"
	"time"
)

// The three levels, narrowest first (LEVELS).
const (
	PrivateToInvoker           = "private_to_invoker"
	GroupSummaryPrivateDetails = "group_summary_private_details"
	GroupVisible               = "group_visible"
)

// Codes VisibilityError carries.
const (
	CodeUnknownLevel             = "UNKNOWN_LEVEL"
	CodeNoInputs                 = "NO_INPUTS"
	CodeNeverGroupVisible        = "NEVER_GROUP_VISIBLE"
	CodeContextLaundering        = "CONTEXT_LAUNDERING"
	CodeNotFieldOwner            = "NOT_FIELD_OWNER"
	CodeRedactionRequired        = "REDACTION_REQUIRED"
	CodeUnknownRedaction         = "UNKNOWN_REDACTION"
	CodeIncompleteSettlementView = "INCOMPLETE_SETTLEMENT_VIEW"
)

var levels = [...]string{PrivateToInvoker, GroupSummaryPrivateDetails, GroupVisible}

// Default is one entry of DEFAULT_VISIBILITY.
type Default struct {
	Component string
	Level     string
}

var defaultVisibility = [...]Default{
	{"invocation_event", GroupSummaryPrivateDetails},
	{"user_typed_text", PrivateToInvoker},
	{"attachment", PrivateToInvoker},
	{"bot_clarification_question", GroupSummaryPrivateDetails},
	{"bot_clarification_answer", PrivateToInvoker},
	{"output_summary", GroupSummaryPrivateDetails},
	{"output_per_person_allocation", PrivateToInvoker},
	{"bank_account_number", PrivateToInvoker},
}

// neverGroupVisible is NEVER_GROUP_VISIBLE, sorted.
var neverGroupVisible = [...]string{"bank_account_number"}

// redactable is REDACTABLE, sorted.
var redactable = [...]string{
	"drop_attachment",
	"drop_other_allocations",
	"mask_bank_account",
	"mask_full_name",
	"mask_phone",
}

var settlementViewFields = [...]string{
	"own_amount_vnd",
	"recipient_and_transfer_instructions",
	"calculation_basis_summary",
	"versions_that_changed_own_obligation",
	"own_dispute_and_receipt_events",
}

// Levels returns LEVELS, narrowest first.
func Levels() []string { return slices.Clone(levels[:]) }

// DefaultVisibility returns DEFAULT_VISIBILITY in declaration order.
func DefaultVisibility() []Default { return slices.Clone(defaultVisibility[:]) }

// NeverGroupVisible returns NEVER_GROUP_VISIBLE, sorted.
func NeverGroupVisible() []string { return slices.Clone(neverGroupVisible[:]) }

// Redactable returns REDACTABLE, sorted.
func Redactable() []string { return slices.Clone(redactable[:]) }

// SettlementViewFields returns SETTLEMENT_VIEW_FIELDS in declaration order.
func SettlementViewFields() []string { return slices.Clone(settlementViewFields[:]) }

// VisibilityError is Python's VisibilityError: `str(exc)` is the code.
type VisibilityError struct {
	Code string
}

func (e *VisibilityError) Error() string { return e.Code }

func refuse(code string) error { return &VisibilityError{Code: code} }

// TypeError is the TypeError `set(applied)` raises in declassify.
type TypeError struct {
	Message string
}

func (e *TypeError) Error() string { return e.Message }

// Rank is rank.
func Rank(level string) (int, error) {
	index := slices.Index(levels[:], level)
	if index < 0 {
		return 0, refuse(CodeUnknownLevel)
	}
	return index, nil
}

// PermittedOutputVisibility is permitted_output_visibility:
// `min(input_levels, key=rank)`, which ranks every input and keeps the first
// of the narrowest.
func PermittedOutputVisibility(inputLevels []string) (string, error) {
	if len(inputLevels) == 0 {
		return "", refuse(CodeNoInputs)
	}
	best := inputLevels[0]
	bestRank, err := Rank(best)
	if err != nil {
		return "", err
	}
	for _, level := range inputLevels[1:] {
		current, err := Rank(level)
		if err != nil {
			return "", err
		}
		if current < bestRank {
			best, bestRank = level, current
		}
	}
	return best, nil
}

// CheckNoContextLaundering is check_no_context_laundering. The requested
// level is ranked even when redaction and consent would open the door, so an
// unknown one is refused either way.
func CheckNoContextLaundering(component, requestedLevel string, inputLevels []string, redacted, ownerConsented bool) error {
	if slices.Contains(neverGroupVisible[:], component) && requestedLevel == GroupVisible {
		return refuse(CodeNeverGroupVisible)
	}
	ceiling, err := PermittedOutputVisibility(inputLevels)
	if err != nil {
		return err
	}
	requested, err := Rank(requestedLevel)
	if err != nil {
		return err
	}
	allowed, err := Rank(ceiling)
	if err != nil {
		return err
	}
	if requested <= allowed {
		return nil
	}
	if redacted && ownerConsented {
		return nil
	}
	return refuse(CodeContextLaundering)
}

// Field is the field dict declassify reads. OwnerID nil is None or an absent
// key.
type Field struct {
	ID         string
	OwnerID    *string
	Component  string
	Visibility string
}

// Derivative is the dict declassify returns. Redaction is the caller's own
// redaction object, not a copy, as in Python.
type Derivative struct {
	DerivedFromID             string
	Component                 string
	Visibility                string
	Redaction                 map[string]any
	DeclassifiedBy            string
	SourceVisibilityUnchanged string
}

// Declassify is declassify.
func Declassify(field Field, toLevel, actorID string, redaction map[string]any) (Derivative, error) {
	if field.OwnerID == nil || *field.OwnerID != actorID {
		return Derivative{}, refuse(CodeNotFieldOwner)
	}
	if len(redaction) == 0 {
		return Derivative{}, refuse(CodeRedactionRequired)
	}
	applied := redaction["applied"]
	if !truthy(applied) {
		return Derivative{}, refuse(CodeUnknownRedaction)
	}
	items, err := setItems(applied)
	if err != nil {
		return Derivative{}, err
	}
	for _, item := range items {
		word, isText := item.(string)
		if !isText || !slices.Contains(redactable[:], word) {
			return Derivative{}, refuse(CodeUnknownRedaction)
		}
	}
	if err := CheckNoContextLaundering(field.Component, toLevel, []string{field.Visibility}, true, true); err != nil {
		return Derivative{}, err
	}
	return Derivative{
		DerivedFromID:             field.ID,
		Component:                 field.Component,
		Visibility:                toLevel,
		Redaction:                 redaction,
		DeclassifiedBy:            actorID,
		SourceVisibilityUnchanged: field.Visibility,
	}, nil
}

// truthy is Python's bool() over the value model.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return true
}

// setItems is what `set(value)` holds, over the value model: a str's
// characters, a list's items, a dict's keys. Building the set fails on the
// first unhashable item, and on a value that is not iterable at all.
func setItems(value any) ([]any, error) {
	switch v := value.(type) {
	case string:
		items := []any{}
		for _, r := range v {
			items = append(items, string(r))
		}
		return items, nil
	case []any:
		for _, item := range v {
			switch item.(type) {
			case []any:
				return nil, &TypeError{Message: "unhashable type: 'list'"}
			case map[string]any:
				return nil, &TypeError{Message: "unhashable type: 'dict'"}
			}
		}
		return v, nil
	case map[string]any:
		items := make([]any, 0, len(v))
		for key := range v {
			items = append(items, key)
		}
		return items, nil
	case bool:
		return nil, &TypeError{Message: "'bool' object is not iterable"}
	case int, int64:
		return nil, &TypeError{Message: "'int' object is not iterable"}
	case float64:
		return nil, &TypeError{Message: "'float' object is not iterable"}
	}
	return nil, &TypeError{Message: fmt.Sprintf("'%T' object is not iterable", value)}
}

// CanViewHistory is can_view_history: all three conditions at once, failing
// closed on an unknown visibility or a missing join date. A nil
// viewerLeftAt is a viewer who has not left.
func CanViewHistory(objectVisibility string, viewerJoinedAt, viewerLeftAt *time.Time, objectCreatedAt time.Time, audienceSnapshot []string, viewerID string) bool {
	if !slices.Contains(levels[:], objectVisibility) {
		return false
	}
	if viewerJoinedAt == nil {
		return false
	}
	if objectCreatedAt.Before(*viewerJoinedAt) {
		return false
	}
	if viewerLeftAt != nil && objectCreatedAt.After(*viewerLeftAt) {
		return false
	}
	if objectVisibility == PrivateToInvoker {
		return false
	}
	return slices.Contains(audienceSnapshot, viewerID)
}

// SettlementView is settlement_view: exactly SETTLEMENT_VIEW_FIELDS, or
// INCOMPLETE_SETTLEMENT_VIEW when any is missing. The values are the caller's.
func SettlementView(obligation map[string]any) (map[string]any, error) {
	for _, field := range settlementViewFields {
		if _, present := obligation[field]; !present {
			return nil, refuse(CodeIncompleteSettlementView)
		}
	}
	view := make(map[string]any, len(settlementViewFields))
	for _, field := range settlementViewFields {
		view[field] = obligation[field]
	}
	return view, nil
}
