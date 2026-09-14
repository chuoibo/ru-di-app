// Package reports ports app.domain.reports (ADR-0023 §2.4): what one person
// can report, and for what. Two closed vocabularies and one note length.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w1_goldens.py from the real module in the pinned API
// image.
//
// # Values
//
// ValidateReport takes what the service passes after pydantic: two literal
// strings and `StrictStr | None`. ValidateReportValues takes Python objects
// the way encoding/json decodes them (nil is None, string is str, int64 or int
// is int, float64 is float, bool is bool, []any is a list or tuple,
// map[string]any is a dict, anything else is some other object), so the
// refusals pydantic makes unreachable keep their codes and precedence.
package reports

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// MaxNoteLength is MAX_NOTE_LENGTH, in code points of the stripped note.
const MaxNoteLength = 500

// Codes Python raises ReportError with. The service answers
// `422, code.lower(), "Báo cáo chưa hợp lệ."`.
const (
	CodeUnknownTargetType = "UNKNOWN_TARGET_TYPE"
	CodeUnknownReason     = "UNKNOWN_REASON"
	CodeNoteTooLong       = "NOTE_TOO_LONG"
	CodeNoteNotText       = "NOTE_NOT_TEXT"
)

// ReportError is Python's ReportError: `.code`, and `str(exc)` is the code.
type ReportError struct {
	Code string
}

func (e *ReportError) Error() string { return e.Code }

func refuse(code string) error { return &ReportError{Code: code} }

var targetTypes = [...]string{"person", "post", "message", "comment", "story"}

var reasons = [...]string{"spam", "harassment", "inappropriate", "impersonation", "other"}

// TargetTypes returns TARGET_TYPES in declaration order.
func TargetTypes() []string { return slices.Clone(targetTypes[:]) }

// Reasons returns REASONS in declaration order.
func Reasons() []string { return slices.Clone(reasons[:]) }

// Report is the dict validate_report returns. Note is nil when the caller
// sent none or when it was only whitespace.
type Report struct {
	TargetType string
	Reason     string
	Note       *string
}

// ValidateReport is validate_report for the service's argument types.
func ValidateReport(targetType, reason string, note *string) (Report, error) {
	if !slices.Contains(targetTypes[:], targetType) {
		return Report{}, refuse(CodeUnknownTargetType)
	}
	if !slices.Contains(reasons[:], reason) {
		return Report{}, refuse(CodeUnknownReason)
	}
	report := Report{TargetType: targetType, Reason: reason}
	if note == nil {
		return report, nil
	}
	cleaned, err := cleanNote(*note)
	if err != nil {
		return Report{}, err
	}
	report.Note = cleaned
	return report, nil
}

// ValidateReportValues is validate_report over Python objects. Python checks
// membership with `in` on a tuple of str, which only an equal str passes.
func ValidateReportValues(targetType, reason, note any) (Report, error) {
	target, ok := targetType.(string)
	if !ok || !slices.Contains(targetTypes[:], target) {
		return Report{}, refuse(CodeUnknownTargetType)
	}
	why, ok := reason.(string)
	if !ok || !slices.Contains(reasons[:], why) {
		return Report{}, refuse(CodeUnknownReason)
	}
	switch text := note.(type) {
	case nil:
		return ValidateReport(target, why, nil)
	case string:
		return ValidateReport(target, why, &text)
	default:
		return Report{}, refuse(CodeNoteNotText)
	}
}

// cleanNote is `note.strip() or None`, then the length check. The length is
// measured on the stripped note, in code points (Python len). pydantic's
// max_length on the raw body is a different, earlier check that this package
// does not own.
func cleanNote(note string) (*string, error) {
	cleaned := strings.TrimFunc(note, isPySpace)
	if cleaned == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(cleaned) > MaxNoteLength {
		return nil, refuse(CodeNoteTooLong)
	}
	return &cleaned, nil
}

// isPySpace is CPython's str.isspace() for one code point: bidirectional
// class WS, B or S, or category Zs. Unlike unicode.IsSpace it includes
// U+001C..U+001F. oracle_test.go checks it against every code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}
