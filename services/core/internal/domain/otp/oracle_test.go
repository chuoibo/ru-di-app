package otp

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"testing"
	"time"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_otp*.json is rendered by scripts/render_domain_w9_goldens.py
// from the real app.domain.otp inside the parity API image. Every case is
// replayed here: the returned dict, or the OverflowError, must match.
//
// The comparison is written out rather than taken from oracletest.Agree
// because these functions raise OverflowError, whose `.code` is None while
// `str(exc)` is a sentence -- the shape Agree's refusal contract cannot carry.

// functions is every ported function, in the order of the script's FUNCTIONS.
var functions = []string{"generate_code", "plan_request", "plan_verify"}

// reasons and outcomes are every answer the committed corpus must show.
var (
	reasons  = []string{"ok", "resend_too_soon", "too_many_requests"}
	outcomes = []string{"not_found", "consumed", "expired", "burned_already", "ok", "burned", "wrong_code"}
	messages = []string{
		"date value out of range",
		"int too large to convert to float",
		"Python int too large to convert to C int",
		"days=1000000" + "000; must have magnitude <= " + "999999" + "999",
	}
)

type outcome struct {
	value   any
	errType string
	message string
}

func pythonOutcome(c oracletest.Case) (outcome, error) {
	value, raised, err := c.Outcome()
	if err != nil {
		return outcome{}, err
	}
	if raised != nil {
		return outcome{errType: raised.Type, message: raised.Message}, nil
	}
	return outcome{value: value}, nil
}

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var overflow *OverflowError
	if errors.As(err, &overflow) {
		return outcome{errType: "OverflowError", message: overflow.Error()}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

// --- decoding ---------------------------------------------------------------

func instants(value any) ([]time.Time, error) {
	items, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, len(items))
	for i, item := range items {
		if out[i], err = oracletest.Instant(item); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// limitsOf reads the `limits` argument. A key the port does not carry (the
// corpus sends one on purpose) is ignored here exactly as the dict merge
// ignores it.
func limitsOf(value any) (*Limits, error) {
	if value == nil {
		return nil, nil
	}
	row, err := oracletest.Row(value)
	if err != nil {
		return nil, err
	}
	lim := &Limits{}
	fields := map[string]**big.Int{
		"code_ttl_seconds":          &lim.CodeTTLSeconds,
		"max_attempts":              &lim.MaxAttempts,
		"resend_cooldown_seconds":   &lim.ResendCooldownSeconds,
		"max_challenges_per_window": &lim.MaxChallengesPerWindow,
		"window_seconds":            &lim.WindowSeconds,
	}
	for key, raw := range row {
		field, known := fields[key]
		if !known {
			continue
		}
		number, err := oracletest.Integer(raw)
		if err != nil {
			return nil, err
		}
		*field = number
	}
	return lim, nil
}

func challengeOf(value any) (*Challenge, error) {
	if value == nil {
		return nil, nil
	}
	row, err := oracletest.Row(value, "expires_at", "attempts", "consumed_at")
	if err != nil {
		return nil, err
	}
	expires, err := oracletest.Instant(row["expires_at"])
	if err != nil {
		return nil, err
	}
	attempts, err := oracletest.Integer(row["attempts"])
	if err != nil {
		return nil, err
	}
	var consumed *time.Time
	if row["consumed_at"] != nil {
		moment, err := oracletest.Instant(row["consumed_at"])
		if err != nil {
			return nil, err
		}
		consumed = &moment
	}
	return &Challenge{ExpiresAt: expires, Attempts: attempts, ConsumedAt: consumed}, nil
}

// --- replay -----------------------------------------------------------------

func replay(c oracletest.Case, args map[string]any) (value any, err error) {
	defer func() {
		if p := recover(); p != nil {
			value, err = nil, fmt.Errorf("panic: %v", p)
		}
	}()
	switch c.Fn {
	case "generate_code":
		draw, err := oracletest.Integer(args["draw"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		var bounds []any
		code, err := GenerateCode(func(bound *big.Int) (*big.Int, error) {
			bounds = append(bounds, oracletest.Exact(bound))
			return draw, nil
		})
		if err != nil {
			return nil, err
		}
		if bounds == nil {
			bounds = []any{}
		}
		return map[string]any{"code": code, "bounds": bounds}, nil

	case "plan_request":
		recent, err := instants(args["recent"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		limits, err := limitsOf(args["limits"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		plan, err := PlanRequest(recent, now, limits)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"allowed":             plan.Allowed,
			"reason":              plan.Reason,
			"retry_after_seconds": oracletest.Exact(plan.RetryAfterSeconds),
		}, nil

	case "plan_verify":
		challenge, err := challengeOf(args["challenge"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		matches, err := oracletest.Bool(args["code_matches"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		limits, err := limitsOf(args["limits"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		plan := PlanVerify(challenge, now, matches, limits)
		row := map[string]any{
			"outcome":  plan.Outcome,
			"attempts": oracletest.Exact(plan.Attempts),
		}
		if plan.AttemptsLeft != nil {
			row["attempts_left"] = oracletest.Exact(plan.AttemptsLeft)
		}
		return row, nil
	}
	return nil, oracletest.Decode(fmt.Errorf("no replay for %s", c.Fn))
}

type tally struct{ cases, mismatches, raises int }

// checkOtp replays every case of the files whose mode is the otp module or one
// of its fuzz shards, and logs a count per function so a run says what it saw.
func checkOtp(t *testing.T, files []oracletest.File, committed bool) {
	t.Helper()
	counts := map[string]*tally{}
	seenReasons := map[string]bool{}
	seenOutcomes := map[string]bool{}
	seenMessages := map[string]bool{}
	mismatches, total := 0, 0
	for _, file := range files {
		if file.Mode != "otp" && !hasPrefix(file.Mode, "otp-fuzz-") {
			continue
		}
		for _, c := range file.Cases {
			want, err := pythonOutcome(c)
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			value, goErr := replay(c, args)
			if errors.Is(goErr, oracletest.ErrDecode) {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, goErr)
			}
			got := goOutcome(value, goErr)
			count := counts[c.Fn]
			if count == nil {
				count = &tally{}
				counts[c.Fn] = count
			}
			count.cases++
			total++
			if want.errType != "" {
				count.raises++
				seenMessages[want.message] = true
			} else if row, ok := want.value.(map[string]any); ok {
				if reason, ok := row["reason"].(string); ok {
					seenReasons[reason] = true
				}
				if outcome, ok := row["outcome"].(string); ok {
					seenOutcomes[outcome] = true
				}
			}
			if !reflect.DeepEqual(want, got) {
				mismatches++
				count.mismatches++
				if mismatches <= 10 {
					t.Errorf("%s %s %s(%v):\n  Python %+v\n  Go     %+v", file.Mode, c.Name, c.Fn, c.Args, want, got)
				}
			}
		}
	}
	oracletest.CheckShards(t, files, "otp")
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		count := counts[name]
		t.Logf("%s: %d Python cases, %d mismatches (%d raised)", name, count.cases, count.mismatches, count.raises)
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	if !committed {
		return
	}
	for _, name := range functions {
		if counts[name] == nil {
			t.Errorf("no case calls %s", name)
		}
	}
	for _, reason := range reasons {
		if !seenReasons[reason] {
			t.Errorf("no committed case answers reason %q", reason)
		}
	}
	for _, name := range outcomes {
		if !seenOutcomes[name] {
			t.Errorf("no committed case answers outcome %q", name)
		}
	}
	for _, message := range messages {
		if !seenMessages[message] {
			t.Errorf("no committed case raises %q", message)
		}
	}
}

func hasPrefix(text, prefix string) bool {
	return len(text) >= len(prefix) && text[:len(prefix)] == prefix
}

func TestOtpMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_otp*.json")
	constants := oracletest.Constants(t, files, "otp")
	length, err := oracletest.Int64(constants["code_length"])
	if err != nil || length != CodeLength {
		t.Errorf("CODE_LENGTH: Python %v, Go %d (%v)", constants["code_length"], CodeLength, err)
	}
	rows, err := oracletest.List(constants["default_limits"])
	if err != nil {
		t.Fatal(err)
	}
	defaults := DefaultLimits()
	mine := map[string]*big.Int{
		"code_ttl_seconds":          defaults.CodeTTLSeconds,
		"max_attempts":              defaults.MaxAttempts,
		"resend_cooldown_seconds":   defaults.ResendCooldownSeconds,
		"max_challenges_per_window": defaults.MaxChallengesPerWindow,
		"window_seconds":            defaults.WindowSeconds,
	}
	if len(rows) != len(mine) {
		t.Errorf("DEFAULT_LIMITS has %d keys, Go has %d", len(rows), len(mine))
	}
	for _, raw := range rows {
		pair, err := oracletest.List(raw)
		if err != nil || len(pair) != 2 {
			t.Fatalf("limit %v", raw)
		}
		key, err := oracletest.Str(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		want, err := oracletest.Integer(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if mine[key] == nil || mine[key].Cmp(want) != 0 {
			t.Errorf("DEFAULT_LIMITS[%q]: Python %v, Go %v", key, want, mine[key])
		}
	}
	checkOtp(t, files, true)
}
