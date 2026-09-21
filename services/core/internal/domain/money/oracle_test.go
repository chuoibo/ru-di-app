package money

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_*.json is rendered by scripts/render_domain_w3_goldens.py
// from the real app.domain.money in the parity API image.

func TestViolationMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.CheckShards(t, files, "money")
	total, mismatches := 0, 0
	outcomes := map[string]int{}
	for _, file := range files {
		for _, c := range file.Cases {
			if c.Fn != "vnd_violation" {
				t.Fatalf("%s %s: unknown function %q", file.Mode, c.Name, c.Fn)
			}
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatal(err)
			}
			value, isInt := args["value"].(int64)
			allowNegative, isAllow := args["allow_negative"].(bool)
			positive, isPositive := args["positive"].(bool)
			if !isInt || !isAllow || !isPositive {
				t.Fatalf("%s %s: args %v are not an int64 and two bools", file.Mode, c.Name, args)
			}
			want, raised, err := c.Outcome()
			if err != nil || raised != nil {
				t.Fatalf("%s %s: Python raised %v (%v)", file.Mode, c.Name, raised, err)
			}
			got := Violation(VND(value), allowNegative, positive)
			var gotValue any
			if got != "" {
				gotValue = got
			}
			total++
			outcomes[got]++
			if want != gotValue {
				mismatches++
				t.Errorf("%s %s vnd_violation(%d, allow_negative=%v, positive=%v): Python %v, Go %v",
					file.Mode, c.Name, value, allowNegative, positive, want, gotValue)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	if total < 650 || outcomes[Negative] < 50 || outcomes[NonPositive] < 10 || outcomes[""] < 50 {
		t.Errorf("%d cases, outcomes %v: the corpus lost its spread", total, outcomes)
	}
	t.Logf("vnd_violation: %d Python cases agree, 0 mismatches; outcomes %v", total, outcomes)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "money")
	for key, want := range map[string]string{
		"not_integer":   NotInteger,
		"negative":      Negative,
		"non_positive":  NonPositive,
		"below_minimum": BelowMinimum,
	} {
		if constants[key] != want {
			t.Errorf("%s: Python %v, Go %q", key, constants[key], want)
		}
	}
	ported := map[string]string{
		"NOT_INTEGER":   "NotInteger",
		"NEGATIVE":      "Negative",
		"NON_POSITIVE":  "NonPositive",
		"BELOW_MINIMUM": "BelowMinimum",
		"vnd_violation": "Violation",
	}
	skipped := map[string]string{
		"count_violation": "validates head counts for outings and budgets; no W3 route reaches it",
	}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if ported[name] == "" && skipped[name] == "" {
			t.Errorf("Python exports %s and the port neither maps nor skips it", name)
		}
	}
	if len(names) != len(ported)+len(skipped) {
		t.Errorf("Python has %d public names %v; the port accounts for %d", len(names), names, len(ported)+len(skipped))
	}
}
