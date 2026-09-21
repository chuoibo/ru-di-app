package reports

// This file is identical in the interests, preferences and reports packages
// apart from the package clause: domain packages may not share a test helper
// package, and the oracle encoding is one contract with
// scripts/render_domain_w1_goldens.py.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

type oracleFile struct {
	Mode      string          `json:"mode"`
	Constants json.RawMessage `json:"constants"`
	Fuzz      *struct {
		Shard  int `json:"shard"`
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Cases []oracleCase `json:"cases"`
}

type oracleCase struct {
	Fn     string         `json:"fn"`
	Name   string         `json:"name"`
	Args   []any          `json:"args"`
	Kwargs map[string]any `json:"kwargs"`
	Result map[string]any `json:"result"`
}

func decodeJSON(raw []byte, into any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	return decoder.Decode(into)
}

func loadOracle(t *testing.T) []oracleFile {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no oracle files: %v", err)
	}
	sort.Strings(paths)
	files := make([]oracleFile, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file oracleFile
		if err := decodeJSON(raw, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		files = append(files, file)
	}
	return files
}

// loadConstants decodes the one constants block the edge-case file carries.
func loadConstants(t *testing.T, into any) {
	t.Helper()
	seen := 0
	for _, file := range loadOracle(t) {
		if file.Constants == nil {
			continue
		}
		seen++
		if err := decodeJSON(file.Constants, into); err != nil {
			t.Fatal(err)
		}
	}
	if seen != 1 {
		t.Fatalf("%d oracle files carry constants, want 1", seen)
	}
}

// otherObject is a Python object of a type the value model has no Go type for
// (bytes, set, ...). It is never a str, a list or a dict.
type otherObject struct{ kind string }

// nilSlice stands for a Go result slice that is nil where Python returns a
// list: it equals nothing, because it would encode as null instead of [].
type nilSlice struct{}

// decodeValue expands the oracle encoding into the value model of the ...Value
// functions: nil, bool, string, int64, float64, []any, map[string]any.
func decodeValue(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, bool:
		return v, nil
	case json.Number:
		return strconv.ParseInt(string(v), 10, 64)
	case string:
		if !strings.HasPrefix(v, "$") {
			return v, nil
		}
		tag, body, _ := strings.Cut(v, ":")
		switch tag {
		case "$i":
			return strconv.ParseInt(body, 0, 64)
		case "$f":
			return strconv.ParseFloat(body, 64)
		case "$o":
			return otherObject{kind: body}, nil
		}
		return nil, fmt.Errorf("unknown scalar tag %q", v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			decoded, err := decodeValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = decoded
		}
		return out, nil
	case map[string]any:
		if len(v) == 1 {
			for key, body := range v {
				switch key {
				case "$tuple":
					list, err := decodeValue(body)
					if _, ok := list.([]any); err == nil && !ok {
						err = fmt.Errorf("$tuple holds %T", list)
					}
					return list, err
				case "$runs":
					return decodeRuns(body)
				case "$str":
					return decodeStr(body)
				}
			}
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			if strings.HasPrefix(key, "$") {
				return nil, fmt.Errorf("unknown object tag %q", key)
			}
			decoded, err := decodeValue(item)
			if err != nil {
				return nil, err
			}
			out[key] = decoded
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected JSON %T", raw)
}

func runPair(run any) (any, int, error) {
	pair, ok := run.([]any)
	if !ok || len(pair) != 2 {
		return nil, 0, fmt.Errorf("bad run %v", run)
	}
	count, err := strconv.Atoi(fmt.Sprint(pair[1]))
	if err != nil || count < 1 {
		return nil, 0, fmt.Errorf("bad run count %v", pair[1])
	}
	return pair[0], count, nil
}

func decodeRuns(body any) (any, error) {
	runs, ok := body.([]any)
	if !ok {
		return nil, fmt.Errorf("$runs holds %T", body)
	}
	out := []any{}
	for _, run := range runs {
		raw, count, err := runPair(run)
		if err != nil {
			return nil, err
		}
		for range count {
			item, err := decodeValue(raw)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func decodeStr(body any) (any, error) {
	runs, ok := body.([]any)
	if !ok {
		return nil, fmt.Errorf("$str holds %T", body)
	}
	var b strings.Builder
	for _, run := range runs {
		raw, count, err := runPair(run)
		if err != nil {
			return nil, err
		}
		unit, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("$str unit %T", raw)
		}
		b.WriteString(strings.Repeat(unit, count))
	}
	return b.String(), nil
}

// sameValue compares two values of the model strictly: an int64 never equals
// a float64, and floats compare by bits.
func sameValue(want, got any) bool {
	switch w := want.(type) {
	case nil:
		return got == nil
	case bool:
		g, ok := got.(bool)
		return ok && g == w
	case string:
		g, ok := got.(string)
		return ok && g == w
	case int64:
		g, ok := got.(int64)
		return ok && g == w
	case float64:
		g, ok := got.(float64)
		return ok && math.Float64bits(g) == math.Float64bits(w)
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !sameValue(w[i], g[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for key, item := range w {
			other, present := g[key]
			if !present || !sameValue(item, other) {
				return false
			}
		}
		return true
	}
	return false
}

func show(v any) string {
	text := fmt.Sprintf("%#v", v)
	if len(text) > 400 {
		text = text[:400] + "..."
	}
	return text
}

// outcome is what one call produced, in the oracle's terms.
type outcome struct {
	value   any
	errType string
	message string
	code    any
}

func pythonOutcome(c oracleCase) (outcome, error) {
	if raised, ok := c.Result["raised"].(map[string]any); ok {
		kind, _ := raised["type"].(string)
		message, _ := raised["message"].(string)
		return outcome{errType: kind, message: message, code: raised["code"]}, nil
	}
	body, ok := c.Result["ok"]
	if !ok {
		return outcome{}, errors.New("result has neither ok nor raised")
	}
	value, err := decodeValue(body)
	return outcome{value: value}, err
}

func (o outcome) same(other outcome) bool {
	if o.errType != "" || other.errType != "" {
		return o.errType == other.errType && o.message == other.message && sameValue(o.code, other.code)
	}
	return sameValue(o.value, other.value)
}

func (o outcome) String() string {
	if o.errType != "" {
		return fmt.Sprintf("raise %s(%q) code=%v", o.errType, o.message, o.code)
	}
	return "return " + show(o.value)
}

// call is one Go entry point's answer to a case.
type call struct {
	path  string
	typed bool
	got   outcome
}

// runOracle replays every case of every oracle file. replay returns the Go
// calls it made; each is compared with Python's outcome.
func runOracle(t *testing.T, minTyped int, replay func(oracleCase) (string, []call, error)) {
	t.Helper()
	var problems []string
	total, typed := 0, 0
	for _, file := range loadOracle(t) {
		for _, c := range file.Cases {
			want, err := pythonOutcome(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			input, calls, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			for _, made := range calls {
				if made.typed {
					typed++
				}
				if !made.got.same(want) {
					problems = append(problems, fmt.Sprintf("%s %s via %s(%s):\n  Python %s\n  Go     %s", c.Fn, c.Name, made.path, input, want, made.got))
				}
			}
			total++
		}
	}
	for i, problem := range problems {
		if i == 20 {
			t.Errorf("... and %d more", len(problems)-20)
			break
		}
		t.Error(problem)
	}
	if len(problems) > 0 {
		t.Fatalf("%d disagreements over %d Python cases", len(problems), total)
	}
	if typed < minTyped {
		t.Fatalf("only %d cases reached a typed entry point, want at least %d", typed, minTyped)
	}
	t.Logf("%d Python cases agree; %d also through a typed entry point", total, typed)
}

// checkFuzzVolume proves the fuzz shards are complete and hold at least 2000
// cases for each function.
func checkFuzzVolume(t *testing.T, fns ...string) {
	t.Helper()
	perFn := map[string]int{}
	shards := map[int]bool{}
	declared, declaredShards, rendered := -1, -1, 0
	for _, file := range loadOracle(t) {
		if file.Fuzz == nil {
			continue
		}
		if declared >= 0 && (declared != file.Fuzz.Total || declaredShards != file.Fuzz.Shards) {
			t.Fatalf("fuzz shards disagree on their totals")
		}
		declared, declaredShards = file.Fuzz.Total, file.Fuzz.Shards
		if shards[file.Fuzz.Shard] {
			t.Fatalf("shard %d appears twice", file.Fuzz.Shard)
		}
		shards[file.Fuzz.Shard] = true
		for _, c := range file.Cases {
			if strings.HasPrefix(c.Name, "fuzz/") {
				perFn[c.Fn]++
				rendered++
			}
		}
	}
	if len(shards) != declaredShards || rendered != declared {
		t.Fatalf("%d of %d shards present holding %d of %d fuzz cases", len(shards), declaredShards, rendered, declared)
	}
	for _, fn := range fns {
		if perFn[fn] < 2000 {
			t.Errorf("%s: %d fuzz cases, want at least 2000", fn, perFn[fn])
		}
	}
}

// checkIsSpace compares a port of str.isspace() with the code point ranges
// CPython reported, over every code point.
func checkIsSpace(t *testing.T, ranges [][2]rune, isSpace func(rune) bool) {
	t.Helper()
	if len(ranges) == 0 {
		t.Fatal("no isspace ranges recorded")
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		want := false
		for _, span := range ranges {
			if r >= span[0] && r <= span[1] {
				want = true
				break
			}
		}
		if isSpace(r) != want {
			t.Errorf("U+%04X: str.isspace() is %v, Go %v", r, want, isSpace(r))
		}
	}
}
