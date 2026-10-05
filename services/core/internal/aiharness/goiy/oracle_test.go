package goiy

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_prompt*.json was rendered by scripts/render_ai_prompt_goldens.py
// from the real suggestion_gemini, reel_gemini and achievement_gemini before
// ADR-0052 deleted them: every prompt, line by line. On 2026-10-05 the two
// contextual rule lines about names were rewritten in place (lines now carry
// "Tên: lời nói" by owner decision); every other expected line is Python's.

func refusal(err error) (class, code string, ok bool) { return "", "", false }

func toPy(v any) (pyjson.Value, error) {
	switch x := v.(type) {
	case nil:
		return pyjson.Null{}, nil
	case bool:
		return pyjson.Bool(x), nil
	case int64:
		return pyjson.NewInt(x), nil
	case oracletest.BigInt:
		n, ok := new(big.Int).SetString(string(x), 10)
		if !ok {
			return nil, fmt.Errorf("bad int %q", x)
		}
		return pyjson.NewBigInt(n), nil
	case float64:
		return pyjson.Float(x), nil
	case string:
		return pyjson.String(x), nil
	case []any:
		out := pyjson.List{}
		for _, item := range x {
			p, err := toPy(item)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		// Reverse order on purpose: the golden JSON lost Python's key order,
		// and a builder that forgot sort_keys must not pass by luck.
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		out := pyjson.NewOrderedMap()
		for _, k := range keys {
			p, err := toPy(x[k])
			if err != nil {
				return nil, err
			}
			out.Set(k, p)
		}
		return out, nil
	}
	return nil, fmt.Errorf("no pyjson form for %T", v)
}

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	py := map[string]pyjson.Value{}
	for k, v := range args {
		p, err := toPy(v)
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		py[k] = p
	}
	obj := func(k string) *pyjson.OrderedMap { m, _ := py[k].(*pyjson.OrderedMap); return m }
	list := func(k string) pyjson.List { l, _ := py[k].(pyjson.List); return l }
	var prompt string
	var err error
	switch c.Fn {
	case "build_suggestion_prompt":
		prompt, err = PromptGoiY(obj("history"), list("places"))
	case "build_contextual_prompt":
		prompt, err = PromptTheoBoiCanh(obj("digest"), list("places"))
	case "build_reel_prompt":
		prompt, err = PromptReel(obj("trip"), list("memories"))
	case "achievement_prompt":
		prompt, err = PromptThanhTuu(obj("facts"), strs(args["candidate_ids"]), args["selected_route"].(string), strs(args["choice_history"]))
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	if err != nil {
		return nil, err
	}
	lines := []any{}
	for _, l := range strings.Split(prompt, "\n") {
		lines = append(lines, l)
	}
	return lines, nil
}

func TestPromptKhopPython(t *testing.T) {
	report := oracletest.Agree(t, oracletest.Load(t, "testdata/python_*.json"), "prompt", replay, refusal)
	for _, fn := range []string{"build_suggestion_prompt", "build_contextual_prompt", "build_reel_prompt", "achievement_prompt"} {
		if report.ByFn[fn] == nil {
			t.Fatalf("%s has no cases", fn)
		}
	}
}
