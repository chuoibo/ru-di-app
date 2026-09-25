// Package accountlifecycle ports app.domain.account_lifecycle (ADR-0023
// §2.1): how an account ends. «Xoá tài khoản» is not a DELETE of the people
// row, because a person's id is a foreign key in the money ledger; instead
// every table takes one of six ways, and Erasure is the closed map of which.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w10_goldens.py from the real module in the parity API
// image.
//
// # Values
//
// Python's person is a dict; here it is a []Column in the dict's key order,
// names unique. Values pass through untouched, so they may be anything the
// caller's row holds. Python refuses a naive datetime with NAIVE_DATETIME; a
// time.Time always carries a location, so that refusal has no Go spelling and
// AnonymisedPerson returns no error. `confirm` is `object` in Python and `any`
// here: only the bool true confirms, as only `True` is `True`.
package accountlifecycle

import (
	"slices"
	"time"
)

// AnonymousDisplayName is ANONYMOUS_DISPLAY_NAME: what the product shows where
// an ended account's name used to be.
const AnonymousDisplayName = "Người dùng đã rời"

// ConfirmationRequired is CONFIRMATION_REQUIRED. The refusal code is its
// upper-case spelling, CodeConfirmRequired.
const ConfirmationRequired = "confirm_required"

// Codes AccountLifecycleError carries. CodeNaiveDatetime cannot arise from Go
// values; see the package comment.
const (
	CodeUnknownErasureAction = "UNKNOWN_ERASURE_ACTION"
	CodeConfirmRequired      = "CONFIRM_REQUIRED"
	CodeNaiveDatetime        = "NAIVE_DATETIME"
)

// The actions of ERASURE, in its key order.
const (
	ActionDelete    = "delete"
	ActionRevoke    = "revoke"
	ActionLeave     = "leave"
	ActionAnonymise = "anonymise"
	ActionKeep      = "keep"
	ActionUntouched = "untouched"
)

// Branch is one entry of ERASURE: an action and the tables it happens to.
type Branch struct {
	Action string
	Tables []string
}

var erasure = [...]Branch{
	{ActionDelete, []string{
		"posts",
		"post_comments",
		"post_reactions",
		"stories",
		"story_views",
		"uploaded_images",
		"person_interests",
		"saved_places",
		"friend_requests",
		"account_identities",
		"context_read_marks",
		"pair_shared_constraints",
		"pair_paper_views",
		"active_couple_members",
	}},
	{ActionRevoke, []string{"account_sessions"}},
	{ActionLeave, []string{"memberships"}},
	{ActionAnonymise, []string{"people"}},
	{ActionKeep, []string{
		"pair_notebooks",
		"pair_notebook_cycles",
		"pair_cycle_participants",
		"pair_consent_proposals",
		"pair_consents",
		"pair_papers",
		"pair_paper_versions",
		"pair_paper_responses",
		"pair_paper_outings",
		"pair_paper_keeps",
		"pair_cycle_rhythms",
		"messages",
		"message_reactions",
		"memories",
		"memory_comments",
		"memory_reactions",
		"outing_stop_checkins",
		"votes",
		"vote_options",
		"vote_ballots",
		"outings",
		"outing_stops",
		"outing_invites",
		"contexts",
		"expenses",
		"expense_versions",
		"expense_items",
		"expense_item_shares",
		"expense_discounts",
		"expense_surcharges",
		"confirmed_allocations",
		"collection_batches",
		"collection_batch_versions",
		"collection_obligations",
		"collection_obligation_sources",
		"collection_envelopes",
		"payment_reports",
		"receipt_confirmations",
		"guest_links",
		"reports",
		"bills",
		"bill_items",
		"bill_item_shares",
		"bill_discounts",
		"bill_surcharges",
		"audit_events",
	}},
	{ActionUntouched, []string{
		"places",
		"place_photos",
		"destinations",
		"idempotency_keys",
		"otp_challenges",
	}},
}

var moneyTables = [...]string{
	"expenses",
	"expense_versions",
	"expense_items",
	"expense_item_shares",
	"expense_surcharges",
	"expense_discounts",
	"confirmed_allocations",
	"collection_batches",
	"collection_batch_versions",
	"collection_obligations",
	"collection_obligation_sources",
	"collection_envelopes",
	"payment_reports",
	"receipt_confirmations",
	"guest_links",
	"bills",
	"bill_items",
	"bill_item_shares",
	"bill_surcharges",
	"bill_discounts",
}

var othersKeepTables = [...]string{
	"pair_papers",
	"pair_paper_versions",
	"pair_paper_responses",
	"pair_paper_keeps",
	"messages",
	"message_reactions",
	"memories",
	"memory_comments",
	"memory_reactions",
	"reports",
	"contexts",
	"outings",
	"audit_events",
}

// Erasure returns ERASURE in its key order. The copy is the caller's.
func Erasure() []Branch {
	out := make([]Branch, len(erasure))
	for i, branch := range erasure {
		out[i] = Branch{Action: branch.Action, Tables: slices.Clone(branch.Tables)}
	}
	return out
}

// MoneyTables returns MONEY_TABLES: what an erasure must not touch by one
// byte.
func MoneyTables() []string { return slices.Clone(moneyTables[:]) }

// OthersKeepTables returns OTHERS_KEEP_TABLES: tables carrying somebody
// else's words, or one's own words in somebody else's conversation.
func OthersKeepTables() []string { return slices.Clone(othersKeepTables[:]) }

// AccountLifecycleError is Python's AccountLifecycleError: `str(exc)` is the
// code.
type AccountLifecycleError struct {
	Code string
}

func (e *AccountLifecycleError) Error() string { return e.Code }

// TablesFor is tables_for: the tables one action touches, or
// UNKNOWN_ERASURE_ACTION for an action the map has no opinion about. The
// comparison is exact, as a dict lookup is.
func TablesFor(action string) ([]string, error) {
	for _, branch := range erasure {
		if branch.Action == action {
			return slices.Clone(branch.Tables), nil
		}
	}
	return nil, &AccountLifecycleError{Code: CodeUnknownErasureAction}
}

// CheckConfirmation is check_confirmation: nil for the bool true, and
// CONFIRM_REQUIRED for anything else, 1 and "true" included.
func CheckConfirmation(confirm any) error {
	if yes, ok := confirm.(bool); ok && yes {
		return nil
	}
	return &AccountLifecycleError{Code: CodeConfirmRequired}
}

// Column is one key of the person dict and its value.
type Column struct {
	Name  string
	Value any
}

// anonymous is the dict literal anonymised_person spreads over the person, in
// its order; deleted_at is filled with now.
var anonymous = [...]Column{
	{"display_name", AnonymousDisplayName},
	{"bio", nil},
	{"city", nil},
	{"budget_band", nil},
	{"discoverable_by_phone", false},
	{"wall_comment_policy", "nobody"},
	{"deleted_at", nil},
}

// AnonymisedPerson is anonymised_person: `{**person, <anonymous>}`. A column
// the person already has keeps its position and takes the anonymous value; one
// it lacks is appended in the literal's order. deleted_at is now, a time.Time.
// Every other column is kept as it was. The person slice is not modified.
func AnonymisedPerson(person []Column, now time.Time) []Column {
	out := slices.Clone(person)
	for _, column := range anonymous {
		value := column.Value
		if column.Name == "deleted_at" {
			value = now
		}
		found := false
		for i := range out {
			if out[i].Name == column.Name {
				out[i].Value = value
				found = true
				break
			}
		}
		if !found {
			out = append(out, Column{Name: column.Name, Value: value})
		}
	}
	return out
}
