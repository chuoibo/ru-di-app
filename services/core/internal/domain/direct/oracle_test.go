package direct

import (
	"fmt"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_*.json is rendered by scripts/render_domain_w3_goldens.py
// from the real app.domain.direct in the parity API image.

func replay(c oracletest.Case) (any, error) {
	args, err := c.PlainArgs()
	if err != nil {
		return nil, err
	}
	str := func(key string) (string, error) {
		value, ok := args[key].(string)
		if !ok {
			return "", fmt.Errorf("%s = %v is not a str", key, args[key])
		}
		return value, nil
	}
	switch c.Fn {
	case "is_pair":
		kind, err := str("kind")
		if err != nil {
			return nil, err
		}
		return IsPair(kind), nil
	case "counterpart_of":
		members, err := oracletest.Strings(args["member_ids"])
		if err != nil {
			return nil, err
		}
		me, err := str("me")
		if err != nil {
			return nil, err
		}
		if other, found := CounterpartOf(members, me); found {
			return other, nil
		}
		return nil, nil
	case "display_name_for":
		kind, err := str("kind")
		if err != nil {
			return nil, err
		}
		stored, err := str("stored_name")
		if err != nil {
			return nil, err
		}
		counterpart, err := oracletest.OptionalString(args["counterpart_name"])
		if err != nil {
			return nil, err
		}
		return DisplayNameFor(kind, stored, counterpart), nil
	}
	return nil, fmt.Errorf("unknown function %q", c.Fn)
}

func TestDirectMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.CheckShards(t, files, "direct")
	counts := map[string]int{}
	total, mismatches, anonymous := 0, 0, 0
	for _, file := range files {
		for _, c := range file.Cases {
			want, raised, err := c.Outcome()
			if err != nil || raised != nil {
				t.Fatalf("%s %s: Python raised %v (%v)", file.Mode, c.Name, raised, err)
			}
			got, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			total++
			counts[c.Fn]++
			if got == AnonymousCounterpart {
				anonymous++
			}
			if want != got {
				mismatches++
				t.Errorf("%s %s %s(%v): Python %#v, Go %#v", file.Mode, c.Name, c.Fn, c.Args, want, got)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	if counts["is_pair"] < 10 || counts["counterpart_of"] < 300 || counts["display_name_for"] < 300 || anonymous < 15 {
		t.Errorf("counts %v, %d anonymous: the corpus lost its spread", counts, anonymous)
	}
	t.Logf("%d Python cases agree, 0 mismatches: %v", total, counts)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "direct")
	for key, want := range map[string]string{
		"kind_group":            KindGroup,
		"kind_pair":             KindPair,
		"anonymous_counterpart": AnonymousCounterpart,
	} {
		if constants[key] != want {
			t.Errorf("%s: Python %v, Go %q", key, constants[key], want)
		}
	}
	ported := map[string]string{
		"KIND_GROUP":            "KindGroup",
		"KIND_PAIR":             "KindPair",
		"ANONYMOUS_COUNTERPART": "AnonymousCounterpart",
		"is_pair":               "IsPair",
		"counterpart_of":        "CounterpartOf",
		"display_name_for":      "DisplayNameFor",
	}
	skipped := map[string]string{
		"KINDS":             "the CHECK on contexts.kind; no W3 route validates a kind",
		"ROSTER_ONLY_DOORS": "a list read by Python tests, not by the service",
		"pair_key":          "POST /direct-messages opens a pair; not W3",
		"can_open":          "POST /direct-messages opens a pair; not W3",
		"is_kind":           "not called by the service",
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
