package timquan

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_timquan*.json was rendered by
// scripts/render_place_ai_goldens.py from the real app.places.search,
// app.places.reasons and app.domain.place_search before ADR-0051 deleted
// them: every prompt line, gate and grounding answer.

func toPy(v any) pyjson.Value {
	switch x := v.(type) {
	case nil:
		return pyjson.Null{}
	case bool:
		return pyjson.Bool(x)
	case int64:
		return pyjson.NewInt(x)
	case oracletest.BigInt:
		n, _ := new(big.Int).SetString(string(x), 10)
		return pyjson.NewBigInt(n)
	case float64:
		return pyjson.Float(x)
	case string:
		return pyjson.String(x)
	case []any:
		out := pyjson.List{}
		for _, item := range x {
			out = append(out, toPy(item))
		}
		return out
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		// Reverse order: nothing may depend on the golden's lost key order.
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		out := pyjson.NewOrderedMap()
		for _, key := range keys {
			out.Set(key, toPy(x[key]))
		}
		return out
	}
	panic(fmt.Sprintf("no pyjson form for %T", v))
}

func fromPy(v pyjson.Value) any {
	switch x := v.(type) {
	case nil, pyjson.Null:
		return nil
	case pyjson.Bool:
		return bool(x)
	case pyjson.Int:
		if n, ok := x.Int64(); ok {
			return n
		}
		return oracletest.BigInt(x.String())
	case pyjson.Float:
		return float64(x)
	case pyjson.String:
		return string(x)
	case pyjson.List:
		out := []any{}
		for _, item := range x {
			out = append(out, fromPy(item))
		}
		return out
	case *pyjson.OrderedMap:
		out := map[string]any{}
		for key, item := range x.All() {
			out[key] = fromPy(item)
		}
		return out
	}
	panic(fmt.Sprintf("no plain form for %T", v))
}

// group is the brain's _taste(raw).
func group(v any) taste.Profile {
	raw, ok := v.(map[string]any)
	if !ok {
		return taste.Unknown()
	}
	p := taste.Profile{Basis: "chua-biet", Interests: []string{}}
	if b, ok := raw["basis"].(string); ok && b != "" {
		p.Basis = taste.Basis(b)
	}
	if list, ok := raw["interests"].([]any); ok {
		for _, item := range list {
			p.Interests = append(p.Interests, item.(string))
		}
	}
	if n, ok := raw["budget_per_person_vnd"].(int64); ok {
		p.BudgetPerPersonVND = &n
	}
	if n, ok := raw["size"].(int64); ok {
		p.Size = &n
	}
	if n, ok := raw["people"].(int64); ok {
		p.People = n
	}
	if n, ok := raw["people_answered"].(int64); ok {
		p.PeopleAnswered = n
	}
	return p
}

func placesOf(v any) []*pyjson.OrderedMap {
	out := []*pyjson.OrderedMap{}
	for _, item := range v.([]any) {
		out = append(out, toPy(item).(*pyjson.OrderedMap))
	}
	return out
}

func lines(s string, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, l := range strings.Split(s, "\n") {
		out = append(out, l)
	}
	return out, nil
}

func replay(fn string, args map[string]any) (any, error) {
	switch fn {
	case "profile_lines":
		got, err := ProfileLines(group(args["group"]))
		out := []any{}
		for _, l := range got {
			out = append(out, l)
		}
		return out, err
	case "build_search_prompt":
		return lines(PromptTimQuan(args["query"].(string), placesOf(args["places"]), group(args["group"])))
	case "build_reason_prompt":
		return lines(PromptLyDo(placesOf(args["places"]), group(args["group"])))
	case "echoes_the_query":
		var reason *string
		if s, ok := args["reason"].(string); ok {
			reason = &s
		}
		return EchoesTheQuery(reason, args["query"].(string)), nil
	case "ungrounded_numbers":
		stray, err := UngroundedNumbers(args["reason"].(string), toPy(args["place"]).(*pyjson.OrderedMap), group(args["group"]))
		out := []any{}
		for _, s := range stray {
			out = append(out, s)
		}
		return out, err
	case "parse_reasons":
		got, err := ParseReasons(args["text"].(string), placesOf(args["places"]), group(args["group"]))
		if err != nil {
			return nil, err
		}
		out := map[string]any{}
		for id, r := range got {
			out[id] = map[string]any{"verdict": r.Verdict, "reason": r.Reason}
		}
		return out, nil
	case "ground_search":
		understood, results, err := Ground(toPy(args["raw"]), placesOf(args["places"]))
		if err != nil {
			return nil, err
		}
		list := []any{}
		for _, r := range results {
			row := map[string]any{"place": fromPy(r.Place), "reason": nil, "verdict": nil}
			if r.Reason != nil {
				row["reason"] = *r.Reason
			}
			if r.Verdict != nil {
				row["verdict"] = *r.Verdict
			}
			list = append(list, row)
		}
		return map[string]any{"understood": fromPy(understood), "results": list}, nil
	}
	return nil, errors.New("unknown fn " + fn)
}

func TestTimQuanKhopPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	byFn := map[string]int{}
	bad := 0
	for _, f := range files {
		for _, c := range f.Cases {
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatal(err)
			}
			want, raised, err := c.Outcome()
			if err != nil {
				t.Fatal(err)
			}
			got, goErr := replay(c.Fn, args)
			ok := false
			switch {
			case raised == nil:
				ok = goErr == nil && reflect.DeepEqual(want, got)
			case raised.Type == "PlaceSearchError":
				var e *Error
				ok = errors.As(goErr, &e) && e.Code == raised.Message
			default:
				ok = errors.Is(goErr, ErrDuLieu)
			}
			byFn[c.Fn]++
			if !ok {
				bad++
				if bad <= 8 {
					t.Errorf("%s %s:\n Python %#v raised %+v\n Go     %#v err %v", c.Fn, c.Name, want, raised, got, goErr)
				}
			}
		}
	}
	for _, fn := range []string{"profile_lines", "build_search_prompt", "build_reason_prompt", "echoes_the_query", "ungrounded_numbers", "parse_reasons", "ground_search"} {
		if byFn[fn] == 0 {
			t.Errorf("%s has no cases", fn)
		}
	}
	t.Logf("cases per function %v, %d mismatches", byFn, bad)
}

func TestHangSoKhopPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_timquan.json"), "timquan")
	if c["MAX_QUERY_CHARS"] != int64(MaxQueryChars) || c["MIN_ECHO_CHARS"] != int64(MinEchoChars) || c["MAX_SALVAGE_MISSES"] != int64(maxSalvageMisses) {
		t.Fatalf("constants: %v", c)
	}
	rules, _ := lines(luatTim, nil)
	if !reflect.DeepEqual(c["SEARCH_RULES"], rules) {
		t.Fatal("the search rules are not Python's, line for line")
	}
}
