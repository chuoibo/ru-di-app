// Package money is the Go port of app/domain/money.py: the one integer-shape
// check shared by every đồng amount (ADR-0029 §2.5, money law 1).
//
// An amount is VND, a whole number of đồng in int64, and nothing else: no
// float, no fraction, no decimal. Python needs `_not_an_integer` because a bool
// or a float can arrive where an int was meant; in Go the type already refuses
// both, so NOT_INTEGER can never be returned here and the check keeps only the
// sign rules. The code stays exported so callers can compare against it.
//
// testdata/python_*.json is rendered from the real module by
// scripts/render_domain_w3_goldens.py and replayed by oracle_test.go.
package money

// VND is an amount of đồng.
type VND int64

// Violation codes, identical to the Python constants.
const (
	NotInteger   = "not_integer"
	Negative     = "negative"
	NonPositive  = "non_positive"
	BelowMinimum = "below_minimum"
)

// Violation is vnd_violation: the first rule value breaks, or "" when none.
// allowNegative lets a signed balance through; positive refuses zero.
func Violation(value VND, allowNegative, positive bool) string {
	if value < 0 && !allowNegative {
		return Negative
	}
	if positive && value == 0 {
		return NonPositive
	}
	return ""
}
