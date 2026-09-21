package allocator

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_allocator*.json and python_apportion*.json are rendered by
// scripts/render_domain_w4_goldens.py from the real app.domain.allocator in
// the parity API image: every edge case and a small seeded sample of the
// fuzz. Every case is replayed here; an amount past int64 in the arguments is
// saturated as a route would (Saturate) and still compared with Python's
// answer. oracle_live_test.go (-tags oracle) replays the full differential
// fuzz, drawn at test time.

func integer(value any) (*big.Int, error) {
	n, err := oracletest.Integer(value)
	if err == nil && n == nil {
		err = errors.New("None is not an int")
	}
	return n, err
}

// amount reads a request amount of any size, saturated.
func amount(value any) (money.VND, error) {
	n, err := integer(value)
	if err != nil {
		return 0, err
	}
	return Saturate(n), nil
}

// expenseOf decodes an allocator input dict.
func expenseOf(value any) (Expense, error) {
	m, err := oracletest.Row(value, "participants", "total_vnd", "items", "surcharges", "discounts", "advancer_id")
	if err != nil {
		return Expense{}, err
	}
	var e Expense
	if e.Participants, err = oracletest.Strings(m["participants"]); err != nil {
		return Expense{}, err
	}
	if e.TotalVND, err = amount(m["total_vnd"]); err != nil {
		return Expense{}, err
	}
	if e.AdvancerID, err = oracletest.OptionalString(m["advancer_id"]); err != nil {
		return Expense{}, err
	}
	items, err := oracletest.List(m["items"])
	if err != nil {
		return Expense{}, err
	}
	for _, raw := range items {
		r, err := oracletest.Row(raw, "item_id", "amount_vnd", "shared_by")
		if err != nil {
			return Expense{}, err
		}
		var it Item
		if it.ItemID, err = oracletest.Str(r["item_id"]); err != nil {
			return Expense{}, err
		}
		if it.AmountVND, err = amount(r["amount_vnd"]); err != nil {
			return Expense{}, err
		}
		if it.SharedBy, err = oracletest.Strings(r["shared_by"]); err != nil {
			return Expense{}, err
		}
		e.Items = append(e.Items, it)
	}
	surcharges, err := oracletest.List(m["surcharges"])
	if err != nil {
		return Expense{}, err
	}
	for _, raw := range surcharges {
		r, err := oracletest.Row(raw, "surcharge_id", "kind", "amount_vnd", "mode")
		if err != nil {
			return Expense{}, err
		}
		var s Surcharge
		if s.SurchargeID, err = oracletest.Str(r["surcharge_id"]); err != nil {
			return Expense{}, err
		}
		if s.Kind, err = oracletest.Str(r["kind"]); err != nil {
			return Expense{}, err
		}
		if s.AmountVND, err = amount(r["amount_vnd"]); err != nil {
			return Expense{}, err
		}
		if s.Mode, err = oracletest.Str(r["mode"]); err != nil {
			return Expense{}, err
		}
		e.Surcharges = append(e.Surcharges, s)
	}
	discounts, err := oracletest.List(m["discounts"])
	if err != nil {
		return Expense{}, err
	}
	for _, raw := range discounts {
		r, err := oracletest.Row(raw, "discount_id", "amount_vnd", "scope", "item_id")
		if err != nil {
			return Expense{}, err
		}
		var d Discount
		if d.DiscountID, err = oracletest.Str(r["discount_id"]); err != nil {
			return Expense{}, err
		}
		if d.AmountVND, err = amount(r["amount_vnd"]); err != nil {
			return Expense{}, err
		}
		if d.Scope, err = oracletest.Str(r["scope"]); err != nil {
			return Expense{}, err
		}
		if d.ItemID, err = oracletest.OptionalString(r["item_id"]); err != nil {
			return Expense{}, err
		}
		e.Discounts = append(e.Discounts, d)
	}
	return e, nil
}

func renderResult(r Result) map[string]any {
	allocations := make([]any, len(r.Allocations))
	for i, share := range r.Allocations {
		allocations[i] = []any{share.ParticipantID, int64(share.AmountVND)}
	}
	shares := make([]any, len(r.ExactShares))
	for i, share := range r.ExactShares {
		shares[i] = []any{share.ParticipantID, share.Fraction()}
	}
	return map[string]any{
		"allocations":      allocations,
		"exact_shares":     shares,
		"rounding_gainers": oracletest.AnyStrings(r.RoundingGainers),
		"warnings":         oracletest.AnyStrings(r.Warnings),
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "allocate":
		e, err := expenseOf(args["expense"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		result, err := Allocate(e)
		if err != nil {
			return nil, err
		}
		return renderResult(result), nil
	case "apportion":
		total, err := integer(args["total_vnd"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		entries, err := oracletest.List(args["exact"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		people := make([]string, len(entries))
		values := make([]*big.Rat, len(entries))
		for i, entry := range entries {
			pair, err := oracletest.Strings(entry)
			if err != nil || len(pair) != 2 {
				return nil, oracletest.Decode(fmt.Errorf("exact share %v", entry))
			}
			value, ok := new(big.Rat).SetString(pair[1])
			if !ok {
				return nil, oracletest.Decode(fmt.Errorf("fraction %q", pair[1]))
			}
			people[i], values[i] = pair[0], value
		}
		advancer, err := oracletest.OptionalString(args["advancer_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		allocations, gainers := apportion(total, people, values, advancer)
		rendered := make([]any, len(allocations))
		for i, n := range allocations {
			rendered[i] = []any{people[i], oracletest.Exact(n)}
		}
		return map[string]any{"allocations": rendered, "gainers": oracletest.AnyStrings(gainers)}, nil
	}
	return nil, oracletest.Decode(fmt.Errorf("unknown function %q", c.Fn))
}

func refusal(err error) (string, string, bool) {
	var refused *AllocationError
	if errors.As(err, &refused) {
		return "AllocationError", refused.Code, true
	}
	return "", "", false
}

// checkAllocate replays the allocate cases of module in files and asserts
// their spread: at least least cases, every refusal code an int64 input can
// reach, every warning, and some arguments past int64. It then checks money
// law 2 over every Python answer that allocated.
func checkAllocate(t *testing.T, files []oracletest.File, module string, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	tally := report.ByFn["allocate"]
	if tally == nil || tally.Cases < least || tally.Refusals == 0 || tally.BigArgs == 0 {
		t.Fatalf("allocate: %+v, want at least %d cases: the corpus lost its spread", tally, least)
	}
	for _, code := range errorPrecedence {
		if code == CodeAmountNotInteger {
			continue // money.VND is an integer by type; no Go input can carry it.
		}
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	warned := map[string]int{}
	checked := 0
	for _, file := range files {
		if file.Mode != module && !strings.HasPrefix(file.Mode, module+"-fuzz-") {
			continue
		}
		for _, c := range file.Cases {
			want, raised, err := c.Outcome()
			if err != nil {
				t.Fatal(err)
			}
			if raised != nil {
				continue
			}
			for _, w := range want.(map[string]any)["warnings"].([]any) {
				warned[fmt.Sprint(w)]++
			}
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatal(err)
			}
			e, err := expenseOf(args["expense"])
			if err != nil {
				t.Fatal(err)
			}
			result, err := Allocate(e)
			if err != nil {
				t.Fatalf("%s: %v", c.Name, err)
			}
			sum := new(big.Int)
			for _, share := range result.Allocations {
				if share.AmountVND < 0 {
					t.Fatalf("%s: negative allocation %v", c.Name, share)
				}
				sum.Add(sum, big.NewInt(int64(share.AmountVND)))
			}
			if sum.Cmp(big.NewInt(int64(e.TotalVND))) != 0 {
				t.Fatalf("%s: allocations sum to %s, total %d", c.Name, sum, e.TotalVND)
			}
			checked++
		}
	}
	for _, warning := range warnings {
		if warned[warning] == 0 {
			t.Errorf("no Python case warned %s", warning)
		}
	}
	t.Logf("refusal codes %v; warnings %v; %d allocations sum to their total", report.Codes, warned, checked)
}

// checkApportion replays the direct _apportion cases of module in files.
func checkApportion(t *testing.T, files []oracletest.File, module string, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	if tally := report.ByFn["apportion"]; tally == nil || tally.Cases < least || tally.BigArgs == 0 {
		t.Fatalf("apportion: %+v, want at least %d cases: the corpus lost its spread", tally, least)
	}
}

func TestAllocateMatchesPython(t *testing.T) {
	checkAllocate(t, oracletest.Load(t, "testdata/python_*.json"), "allocator", 800)
}

func TestApportionMatchesPython(t *testing.T) {
	checkApportion(t, oracletest.Load(t, "testdata/python_*.json"), "apportion", 200)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "allocator")
	if constants["max_amount_vnd"] != int64(MaxAmountVND) || constants["max_id_bytes"] != int64(MaxIDBytes) {
		t.Errorf("bounds: Python %v %v, Go %d %d", constants["max_amount_vnd"], constants["max_id_bytes"], MaxAmountVND, MaxIDBytes)
	}
	for key, goValue := range map[string][]string{
		"error_precedence": ErrorPrecedence(),
		"warnings":         Warnings(),
		"surcharge_modes":  SurchargeModes(),
		"discount_scopes":  DiscountScopes(),
	} {
		python, err := oracletest.Strings(constants[key])
		if err != nil || !slices.Equal(python, goValue) {
			t.Errorf("%s: Python %v, Go %v (%v)", key, constants[key], goValue, err)
		}
	}
	ported := map[string]string{
		"allocate":         "Allocate",
		"AllocationError":  "AllocationError",
		"MAX_AMOUNT_VND":   "MaxAmountVND",
		"MAX_ID_BYTES":     "MaxIDBytes",
		"ERROR_PRECEDENCE": "ErrorPrecedence",
		"WARNINGS":         "Warnings",
		"SURCHARGE_MODES":  "SurchargeModes",
		"DISCOUNT_SCOPES":  "DiscountScopes",
		"NOT_INTEGER":      "money.NotInteger",
	}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if ported[name] == "" {
			t.Errorf("Python exports %s and the port does not map it", name)
		}
	}
}

// The 41 hand-computed vectors are read where they live (ADR-0029 §2.5),
// never copied.
func TestGoldenCorpusInPlace(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "api", "tests", "domain", "golden", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden corpus found: %v", err)
	}
	sort.Strings(paths)
	successes, refusals, disagree := 0, 0, 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		var vectors []map[string]any
		if err := decoder.Decode(&vectors); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, vector := range vectors {
			input, err := oracletest.Plain(vector["input"])
			if err != nil {
				t.Fatal(err)
			}
			e, err := expenseOf(input)
			if err != nil {
				t.Fatalf("%v: %v", vector["id"], err)
			}
			result, goErr := Allocate(e)
			if code, isError := vector["expect_error"].(string); isError {
				refusals++
				var refused *AllocationError
				if !errors.As(goErr, &refused) || refused.Code != code {
					disagree++
					t.Errorf("%v: corpus %s, Go %v", vector["id"], code, goErr)
				}
				continue
			}
			successes++
			if goErr != nil {
				disagree++
				t.Errorf("%v: Go refused with %v", vector["id"], goErr)
				continue
			}
			expect, err := oracletest.Plain(vector["expect"])
			if err != nil {
				t.Fatal(err)
			}
			want := expect.(map[string]any)
			got := map[string]any{}
			sum := int64(0)
			for _, share := range result.Allocations {
				got[share.ParticipantID] = int64(share.AmountVND)
				sum += int64(share.AmountVND)
			}
			fractions := map[string]any{}
			for _, share := range result.ExactShares {
				fractions[share.ParticipantID] = share.Fraction()
			}
			if !reflect.DeepEqual(want["allocations"], got) || !reflect.DeepEqual(want["exact_shares"], fractions) ||
				!reflect.DeepEqual(want["rounding_gainers"], oracletest.AnyStrings(result.RoundingGainers)) ||
				!reflect.DeepEqual(want["warnings"], oracletest.AnyStrings(result.Warnings)) {
				disagree++
				t.Errorf("%v:\n  corpus %v\n  Go     %v %v %v %v", vector["id"], want, got, fractions, result.RoundingGainers, result.Warnings)
			}
			if sum != int64(e.TotalVND) {
				t.Errorf("%v: allocations sum to %d, total %d", vector["id"], sum, e.TotalVND)
			}
		}
	}
	if successes != 23 || refusals != 18 {
		t.Fatalf("%d success and %d error vectors, want the corpus's 23 and 18", successes, refusals)
	}
	if disagree > 0 {
		t.Fatalf("%d of %d hand-computed allocator vectors disagree", disagree, successes+refusals)
	}
	t.Logf("%d hand-computed allocator vectors agree (%d allocations, %d refusals)", successes+refusals, successes, refusals)
}
