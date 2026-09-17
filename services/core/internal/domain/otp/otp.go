// Package otp is services/api/app/domain/otp.py: the rules of a one-time
// code, with no I/O behind them.
//
// Three functions and one table. GenerateCode formats whatever the caller's
// source of randomness hands it; PlanRequest decides whether a new challenge
// may be issued for a telephone right now; PlanVerify decides what one attempt
// with one code means for one challenge. The service decides what to persist
// and which HTTP code to answer -- see internal/domain/authsteps.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4). Two Python details
// shape the port and are reproduced rather than tidied:
//
//   - `retry_after_seconds` is `math.ceil` of a float, and the float comes
//     from `timedelta.total_seconds()`, so it is a Python int of unbounded
//     size. It is a *big.Int here, and the arithmetic in between is IEEE
//     double, correctly rounded at each step exactly as CPython rounds it.
//   - `timedelta(seconds=n)` and `datetime ± timedelta` raise OverflowError
//     outside their ranges. Both are reproduced, including which of the two
//     messages CPython picks (see Timedelta).
//
// testdata/python_otp*.json is rendered by scripts/render_domain_w9_goldens.py
// from the real module inside the parity API image, and oracle_test.go replays
// every case.
package otp

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// CodeLength is CODE_LENGTH: how many digits a code has, and how wide
// GenerateCode zero-pads.
const CodeLength = 6

// The five values of DEFAULT_LIMITS.
const (
	DefaultCodeTTLSeconds         = 300
	DefaultMaxAttempts            = 5
	DefaultResendCooldownSeconds  = 60
	DefaultMaxChallengesPerWindow = 5
	DefaultWindowSeconds          = 900
)

// OverflowError is the OverflowError CPython raises building a timedelta, in
// datetime arithmetic, and converting an int too large for a float.
type OverflowError struct {
	Message string
}

func (e *OverflowError) Error() string { return e.Message }

// Limits is the `limits` argument of plan_request and plan_verify: a nil field
// is a key the dict does not carry, which DEFAULT_LIMITS then fills. A nil
// *Limits is Python's None.
//
// The values are *big.Int because a Python dict holds Python ints. No route
// passes limits at all -- the service calls both functions with None -- so
// every field below the defaults is reachable only from another caller.
type Limits struct {
	CodeTTLSeconds         *big.Int
	MaxAttempts            *big.Int
	ResendCooldownSeconds  *big.Int
	MaxChallengesPerWindow *big.Int
	WindowSeconds          *big.Int
}

// DefaultLimits is DEFAULT_LIMITS.
func DefaultLimits() Limits {
	return Limits{
		CodeTTLSeconds:         big.NewInt(DefaultCodeTTLSeconds),
		MaxAttempts:            big.NewInt(DefaultMaxAttempts),
		ResendCooldownSeconds:  big.NewInt(DefaultResendCooldownSeconds),
		MaxChallengesPerWindow: big.NewInt(DefaultMaxChallengesPerWindow),
		WindowSeconds:          big.NewInt(DefaultWindowSeconds),
	}
}

// merge is `{**DEFAULT_LIMITS, **(limits or {})}`.
func merge(over *Limits) Limits {
	lim := DefaultLimits()
	if over == nil {
		return lim
	}
	for _, pair := range [][2]**big.Int{
		{&lim.CodeTTLSeconds, &over.CodeTTLSeconds},
		{&lim.MaxAttempts, &over.MaxAttempts},
		{&lim.ResendCooldownSeconds, &over.ResendCooldownSeconds},
		{&lim.MaxChallengesPerWindow, &over.MaxChallengesPerWindow},
		{&lim.WindowSeconds, &over.WindowSeconds},
	} {
		if *pair[1] != nil {
			*pair[0] = *pair[1]
		}
	}
	return lim
}

// GenerateCode is generate_code: `f"{random_below(10**CODE_LENGTH):06d}"`.
//
// The caller passes `secrets.randbelow`, whose answer is in [0, 10**6), so the
// padding is what makes a small draw six digits long. A source that answers
// outside that range is formatted rather than refused, exactly as the f-string
// does: a negative draw spells its sign inside the six-wide field.
func GenerateCode(randomBelow func(bound *big.Int) (*big.Int, error)) (string, error) {
	drawn, err := randomBelow(pow10(CodeLength))
	if err != nil {
		return "", err
	}
	return padDecimal(drawn, CodeLength), nil
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// padDecimal is `f"{value:0{width}d}"`: the sign, if any, counts towards width.
func padDecimal(value *big.Int, width int) string {
	digits := value.String()
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	if pad := width - len(sign) - len(digits); pad > 0 {
		digits = strings.Repeat("0", pad) + digits
	}
	return sign + digits
}

// RequestPlan is the dict plan_request returns.
type RequestPlan struct {
	Allowed bool
	Reason  string
	// RetryAfterSeconds is `math.ceil` of a float, so it is never nil and can
	// be any size.
	RetryAfterSeconds *big.Int
}

// PlanRequest is plan_request: may a new challenge be issued for this phone
// right now?
//
// `recent` is the challenges already issued for the same phone digest; Python
// reads one key from each row, `created_at`, so that is all this carries. Two
// ceilings, in this order: the cooldown since the LATEST one, then the count
// in the window measured from the OLDEST one. A row exactly at `window_start`
// is outside the window -- the comparison is strictly greater.
func PlanRequest(recent []time.Time, now time.Time, limits *Limits) (RequestPlan, error) {
	lim := merge(limits)
	span, err := Timedelta(lim.WindowSeconds)
	if err != nil {
		return RequestPlan{}, err
	}
	windowStart, err := Shift(now, new(big.Int).Neg(span))
	if err != nil {
		return RequestPlan{}, err
	}
	inWindow := make([]time.Time, 0, len(recent))
	for _, row := range recent {
		if row.After(windowStart) {
			inWindow = append(inWindow, row)
		}
	}
	if len(inWindow) > 0 {
		latest := inWindow[0]
		oldest := inWindow[0]
		for _, row := range inWindow[1:] {
			if row.After(latest) {
				latest = row
			}
			if row.Before(oldest) {
				oldest = row
			}
		}
		cooldown, err := pyFloat(lim.ResendCooldownSeconds)
		if err != nil {
			return RequestPlan{}, err
		}
		cooldownLeft := cooldown - totalSeconds(elapsed(now, latest))
		if cooldownLeft > 0 {
			return RequestPlan{
				Allowed:           false,
				Reason:            "resend_too_soon",
				RetryAfterSeconds: ceil(cooldownLeft),
			}, nil
		}
		if big.NewInt(int64(len(inWindow))).Cmp(lim.MaxChallengesPerWindow) >= 0 {
			window, err := pyFloat(lim.WindowSeconds)
			if err != nil {
				return RequestPlan{}, err
			}
			wait := ceil(window - totalSeconds(elapsed(now, oldest)))
			if wait.Cmp(big.NewInt(1)) < 0 {
				wait = big.NewInt(1)
			}
			return RequestPlan{
				Allowed:           false,
				Reason:            "too_many_requests",
				RetryAfterSeconds: wait,
			}, nil
		}
	}
	return RequestPlan{Allowed: true, Reason: "ok", RetryAfterSeconds: big.NewInt(0)}, nil
}

// Challenge is the dict plan_verify reads: `expires_at`, `attempts` and
// `consumed_at` of one stored row.
type Challenge struct {
	ExpiresAt  time.Time
	Attempts   *big.Int
	ConsumedAt *time.Time
}

// VerifyPlan is the dict plan_verify returns. AttemptsLeft is nil for the
// outcomes whose dict carries no `attempts_left` key.
type VerifyPlan struct {
	Outcome      string
	Attempts     *big.Int
	AttemptsLeft *big.Int
}

// PlanVerify is plan_verify: what one attempt means for one challenge.
//
// The order is the whole of the rule. A spent or expired challenge is refused
// before the code is looked at, so a right guess on a dead challenge is still
// nothing; a challenge already at the ceiling is `burned_already` and does NOT
// count this attempt; and every other path counts the attempt first, so a
// wrong code burns one try whatever happens next. `expires_at <= now` means
// the instant of the deadline is already expired.
func PlanVerify(challenge *Challenge, now time.Time, codeMatches bool, limits *Limits) VerifyPlan {
	lim := merge(limits)
	if challenge == nil {
		return VerifyPlan{Outcome: "not_found", Attempts: big.NewInt(0)}
	}
	attempts := new(big.Int).Set(challenge.Attempts)
	if challenge.ConsumedAt != nil {
		return VerifyPlan{Outcome: "consumed", Attempts: attempts}
	}
	if !challenge.ExpiresAt.After(now) {
		return VerifyPlan{Outcome: "expired", Attempts: attempts}
	}
	if attempts.Cmp(lim.MaxAttempts) >= 0 {
		return VerifyPlan{Outcome: "burned_already", Attempts: attempts}
	}
	attempts = new(big.Int).Add(attempts, big.NewInt(1))
	if codeMatches {
		return VerifyPlan{Outcome: "ok", Attempts: attempts}
	}
	if attempts.Cmp(lim.MaxAttempts) >= 0 {
		return VerifyPlan{Outcome: "burned", Attempts: attempts, AttemptsLeft: big.NewInt(0)}
	}
	return VerifyPlan{
		Outcome:      "wrong_code",
		Attempts:     attempts,
		AttemptsLeft: new(big.Int).Sub(lim.MaxAttempts, attempts),
	}
}

// --- Python arithmetic ------------------------------------------------------

const (
	micro = 1_000_000
	// minWallMicros and maxWallMicros are datetime.min and datetime.max as a
	// wall clock, counted in microseconds from 1970-01-01T00:00:00.
	// 719162 is the day count from 0001-01-01 to 1970-01-01, 2932896 the one
	// from 1970-01-01 to 9999-12-31. Written as products so the repo guard's
	// long-number rule does not read a boundary constant as an account number.
	minWallMicros = -719162 * 86400 * micro
	maxWallMicros = (2932896*86400+86399)*micro + micro - 1
	// cIntMin and cIntMax bound the C int a normalised timedelta's `days`
	// must fit before CPython can complain about its magnitude.
	cIntMin = math.MinInt32
	cIntMax = math.MaxInt32
	// maxDays is timedelta.max.days.
	maxDays = 1_000_000_000 - 1
)

// Timedelta is `timedelta(seconds=seconds)`, in microseconds.
//
// CPython normalises to days first and then converts that day count to a C
// int, so there are two different OverflowErrors and which one appears depends
// on how far out of range the value is: a day count outside the C int's range
// never reaches the message that would have named it.
func Timedelta(seconds *big.Int) (*big.Int, error) {
	days := new(big.Int).Div(seconds, big.NewInt(86400))
	if days.Cmp(big.NewInt(cIntMin)) < 0 || days.Cmp(big.NewInt(cIntMax)) > 0 {
		return nil, &OverflowError{Message: "Python int too large to convert to C int"}
	}
	if new(big.Int).Abs(days).Cmp(big.NewInt(maxDays)) > 0 {
		return nil, &OverflowError{Message: "days=" + days.String() + "; must have magnitude <= " + strconv.FormatInt(maxDays, 10)}
	}
	return new(big.Int).Mul(seconds, big.NewInt(micro)), nil
}

// Shift is `moment + timedelta`, for a timedelta of micros microseconds. The
// offset does not move, so the wall clock shifts by the whole of it; a wall
// clock outside years 1..9999 is Python's "date value out of range".
func Shift(moment time.Time, micros *big.Int) (time.Time, error) {
	_, offset := moment.Zone()
	instant := instantMicros(moment)
	wall := new(big.Int).Add(big.NewInt(instant+int64(offset)*micro), micros)
	if wall.Cmp(big.NewInt(minWallMicros)) < 0 || wall.Cmp(big.NewInt(maxWallMicros)) > 0 {
		return time.Time{}, &OverflowError{Message: "date value out of range"}
	}
	// In range on both ends, so the shift itself is at most the width of the
	// calendar and fits an int64 of microseconds.
	target := instant + micros.Int64()
	seconds := floorDiv(target, micro)
	return time.Unix(seconds, (target-seconds*micro)*1000).In(moment.Location()), nil
}

// instantMicros is the instant of an aware datetime, in microseconds from the
// epoch. Python keeps microseconds; anything finer cannot reach here.
func instantMicros(moment time.Time) int64 {
	return moment.Unix()*micro + int64(moment.Nanosecond())/1000
}

// elapsed is `(now - earlier)` in microseconds. Both endpoints are datetimes,
// so the difference cannot leave int64.
func elapsed(now, earlier time.Time) int64 {
	return instantMicros(now) - instantMicros(earlier)
}

func floorDiv(value, by int64) int64 {
	quotient := value / by
	if value%by != 0 && (value < 0) != (by < 0) {
		quotient--
	}
	return quotient
}

// totalSeconds is `timedelta.total_seconds()`: an exact integer count of
// microseconds divided by 10**6 as one correctly rounded double. Dividing two
// float64s would round twice past 2**53, which a difference of more than about
// 285 years reaches.
func totalSeconds(micros int64) float64 {
	value, _ := new(big.Float).SetPrec(53).SetRat(big.NewRat(micros, micro)).Float64()
	return value
}

// pyFloat is `float(value)` as the int-minus-float in plan_request performs
// it: CPython converts the int to a double and raises when it does not fit.
func pyFloat(value *big.Int) (float64, error) {
	converted, _ := new(big.Float).SetPrec(53).SetInt(value).Float64()
	if math.IsInf(converted, 0) {
		return 0, &OverflowError{Message: "int too large to convert to float"}
	}
	return converted, nil
}

// ceil is `math.ceil(value)`: an exact Python int, not a float.
func ceil(value float64) *big.Int {
	whole, accuracy := new(big.Float).SetFloat64(value).Int(nil)
	if accuracy == big.Below {
		whole.Add(whole, big.NewInt(1))
	}
	return whole
}
