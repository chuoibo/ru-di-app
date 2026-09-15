// Package capability is the Go port of app/domain/capability.py: the guest
// capability scope of product spec invariant 6, which POST
// /batches/{batch_id}/publish checks once per sender before it mints a link.
//
// A link covers exactly one sender's obligations inside one batch version.
// Scope refuses a set that crosses either boundary or repeats an obligation,
// with the code Python raises, found in the order Python finds it.
//
// testdata/python_*.json is rendered from the real module by
// scripts/render_domain_w4_goldens.py and replayed by oracle_test.go.
package capability

import (
	"slices"
	"strings"
)

// Codes carried by *CapabilityScopeError.
const (
	CodeNoObligations       = "NO_OBLIGATIONS"
	CodeCrossesBatchVersion = "CROSSES_BATCH_VERSION"
	CodeCrossesSender       = "CROSSES_SENDER"
	CodeDuplicateObligation = "DUPLICATE_OBLIGATION"
)

// CapabilityScopeError mirrors CapabilityScopeError: Error returns the code.
type CapabilityScopeError struct {
	Code string
}

func (e *CapabilityScopeError) Error() string { return e.Code }

// Envelope is the envelope dict: which version and which sender a link names.
type Envelope struct {
	BatchVersionID string
	SenderID       string
}

// Obligation is one obligation dict of the queried set.
type Obligation struct {
	ObligationID   string
	BatchVersionID string
	SenderID       string
}

// Scope is capability_scope's dict. ObligationIDs are sorted by str, which
// for UTF-8 is the byte order strings.Compare uses.
type Scope struct {
	BatchVersionID string
	SenderID       string
	ObligationIDs  []string
}

// ScopeOf is capability_scope: NO_OBLIGATIONS for an empty set; then, per
// obligation in order, CROSSES_BATCH_VERSION before CROSSES_SENDER; then
// DUPLICATE_OBLIGATION.
func ScopeOf(envelope Envelope, obligations []Obligation) (Scope, error) {
	if len(obligations) == 0 {
		return Scope{}, &CapabilityScopeError{Code: CodeNoObligations}
	}
	ids := make([]string, 0, len(obligations))
	seen := make(map[string]bool, len(obligations))
	repeated := false
	for _, obligation := range obligations {
		if obligation.BatchVersionID != envelope.BatchVersionID {
			return Scope{}, &CapabilityScopeError{Code: CodeCrossesBatchVersion}
		}
		if obligation.SenderID != envelope.SenderID {
			return Scope{}, &CapabilityScopeError{Code: CodeCrossesSender}
		}
		if seen[obligation.ObligationID] {
			repeated = true
		}
		seen[obligation.ObligationID] = true
		ids = append(ids, obligation.ObligationID)
	}
	if repeated {
		return Scope{}, &CapabilityScopeError{Code: CodeDuplicateObligation}
	}
	slices.SortStableFunc(ids, strings.Compare)
	return Scope{BatchVersionID: envelope.BatchVersionID, SenderID: envelope.SenderID, ObligationIDs: ids}, nil
}
