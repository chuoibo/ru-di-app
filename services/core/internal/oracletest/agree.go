package oracletest

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ErrDecode marks a replay that could not read its golden case; Agree stops
// on it rather than counting it as a disagreement.
var ErrDecode = errors.New("decode")

// Decode wraps err as a decoding failure.
func Decode(err error) error { return fmt.Errorf("%w: %v", ErrDecode, err) }

// Tally counts the cases of one Python function.
type Tally struct {
	Cases, Refusals, BigArgs, BigResults, Mismatches int
}

// Report is what Agree saw: per function, and every refusal code by count,
// whether Python raised it or a service step answered it as a problem.
type Report struct {
	ByFn  map[string]*Tally
	Codes map[string]int
}

// Refusal names the Python exception a Go error stands for: its class name
// and its code. ok is false for an error Python would not raise as a refusal.
type Refusal func(err error) (class, code string, ok bool)

// Agree replays every case of the files whose mode is module or one of its
// fuzz shards. replay answers one case in Go; a panic inside it is a
// mismatch, so a mutant that indexes out of range is counted, not fatal. A
// Python raise agrees with a Go error of the same class whose code is both
// str(exc) and exc.code. Mismatches are logged per function before the test
// fails, so a mutant's failing count is visible.
func Agree(t testing.TB, files []File, module string, replay func(Case, map[string]any) (any, error), refusal Refusal) Report {
	t.Helper()
	report := Report{ByFn: map[string]*Tally{}, Codes: map[string]int{}}
	mismatches, total := 0, 0
	for _, file := range files {
		if file.Mode != module && !strings.HasPrefix(file.Mode, module+"-fuzz-") {
			continue
		}
		for _, c := range file.Cases {
			want, raised, err := c.Outcome()
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			got, goErr := safely(replay, c, args)
			if errors.Is(goErr, ErrDecode) {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, goErr)
			}
			tally := report.ByFn[c.Fn]
			if tally == nil {
				tally = &Tally{}
				report.ByFn[c.Fn] = tally
			}
			tally.Cases++
			total++
			if HasBigInt(args) {
				tally.BigArgs++
			}
			ok := false
			if raised != nil {
				tally.Refusals++
				report.Codes[raised.Message]++
				if goErr != nil {
					class, code, isRefusal := refusal(goErr)
					ok = isRefusal && class == raised.Type && code == raised.Message && raised.Code == any(code)
				}
			} else {
				ok = goErr == nil && reflect.DeepEqual(want, got)
				if HasBigInt(want) {
					tally.BigResults++
				}
				if row, isRow := want.(map[string]any); isRow {
					if problem, isProblem := row["problem"].(map[string]any); isProblem {
						tally.Refusals++
						report.Codes[fmt.Sprint(problem["code"])]++
					}
				}
			}
			if !ok {
				mismatches++
				tally.Mismatches++
				if mismatches <= 10 {
					t.Errorf("%s %s %s(%v):\n  Python %#v raised %+v\n  Go     %#v err %v", file.Mode, c.Name, c.Fn, c.Args, want, raised, got, goErr)
				}
			}
		}
	}
	CheckShards(t, files, module)
	names := make([]string, 0, len(report.ByFn))
	for name := range report.ByFn {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tally := report.ByFn[name]
		t.Logf("%s: %d Python cases, %d mismatches (%d refusals; %d argument sets and %d answers past int64)",
			name, tally.Cases, tally.Mismatches, tally.Refusals, tally.BigArgs, tally.BigResults)
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	return report
}

func safely(replay func(Case, map[string]any) (any, error), c Case, args map[string]any) (got any, err error) {
	defer func() {
		if p := recover(); p != nil {
			got, err = nil, fmt.Errorf("panic: %v", p)
		}
	}()
	return replay(c, args)
}

// Str reads a decoded str.
func Str(value any) (string, error) {
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%v (%T) is not a str", value, value)
	}
	return s, nil
}

// List reads a decoded list.
func List(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%v (%T) is not a list", value, value)
	}
	return items, nil
}

// Row reads a decoded dict that has every one of keys.
func Row(value any, keys ...string) (map[string]any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%v (%T) is not a dict", value, value)
	}
	for _, key := range keys {
		if _, found := m[key]; !found {
			return nil, fmt.Errorf("%v has no %s", m, key)
		}
	}
	return m, nil
}

// Int64 reads an int that must fit int64, as a stored amount does.
func Int64(value any) (int64, error) {
	n, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("%v (%T) is not an int inside int64", value, value)
	}
	return n, nil
}

// Bool reads a decoded bool.
func Bool(value any) (bool, error) {
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%v (%T) is not a bool", value, value)
	}
	return b, nil
}

// Integer reads any Python int exactly; None is nil.
func Integer(value any) (*big.Int, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case int64:
		return big.NewInt(v), nil
	case BigInt:
		n, ok := new(big.Int).SetString(string(v), 10)
		if !ok {
			return nil, fmt.Errorf("bad int %q", v)
		}
		return n, nil
	}
	return nil, fmt.Errorf("%v (%T) is not an int or None", value, value)
}

// Exact renders a *big.Int as Plain decodes Python's int; nil is None.
func Exact(n *big.Int) any {
	switch {
	case n == nil:
		return nil
	case n.IsInt64():
		return n.Int64()
	}
	return BigInt(n.String())
}

// AnyStrings renders strings as a decoded list.
func AnyStrings(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
