package scoring

// This file is identical in the taste and scoring packages apart from the
// package clause: domain packages may not share a test helper package, and
// the encoding is one contract with scripts/render_places_taste_goldens.py.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type goldenFile struct {
	path      string
	Module    string          `json:"module"`
	Mode      string          `json:"mode"`
	Python    string          `json:"python"`
	Constants json.RawMessage `json:"constants"`
	Fuzz      *struct {
		Shard  int `json:"shard"`
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	RawName any                       `json:"name"`
	In      map[string]any            `json:"in"`
	Calls   map[string]map[string]any `json:"calls"`
}

func decodeJSON(raw []byte, into any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	return decoder.Decode(into)
}

func loadGoldens(t *testing.T) []goldenFile {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	sort.Strings(paths)
	files := make([]goldenFile, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file := goldenFile{path: path}
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
	for _, file := range loadGoldens(t) {
		if file.Constants == nil {
			continue
		}
		seen++
		if err := decodeJSON(file.Constants, into); err != nil {
			t.Fatal(err)
		}
	}
	if seen != 1 {
		t.Fatalf("%d golden files carry constants, want 1", seen)
	}
}

// The decoded value model: nil, bool, string, *big.Int (every int),
// float64, *big.Rat (a Fraction), []any (a list), pyTuple, pySet,
// map[string]any (a dict) and pyProfile (a TasteProfile with its properties).
type (
	pyTuple   []any
	pySet     []string
	pyProfile map[string]any
)

func ungroup(text string) string { return strings.ReplaceAll(text, "_", "") }

func parseInt(text string) (*big.Int, error) {
	out, ok := new(big.Int).SetString(ungroup(text), 10)
	if !ok {
		return nil, fmt.Errorf("bad int %q", text)
	}
	return out, nil
}

func decodeScalar(text string) (any, error) {
	if !strings.HasPrefix(text, "$") {
		return text, nil
	}
	tag, body, _ := strings.Cut(text, ":")
	switch tag {
	case "$i":
		return parseInt(body)
	case "$f":
		value, err := strconv.ParseFloat(ungroup(body), 64)
		if err != nil {
			return nil, fmt.Errorf("bad float %q: %w", text, err)
		}
		return value, nil
	case "$q":
		top, bottom, ok := strings.Cut(body, "/")
		if !ok {
			return nil, fmt.Errorf("bad fraction %q", text)
		}
		numerator, err := parseInt(top)
		if err != nil {
			return nil, err
		}
		denominator, err := parseInt(bottom)
		if err != nil {
			return nil, err
		}
		if denominator.Sign() <= 0 || new(big.Int).GCD(nil, nil, new(big.Int).Abs(numerator), denominator).Cmp(big.NewInt(1)) != 0 && numerator.Sign() != 0 {
			return nil, fmt.Errorf("fraction %q is not in lowest terms", text)
		}
		return new(big.Rat).SetFrac(numerator, denominator), nil
	}
	return nil, fmt.Errorf("unknown scalar tag %q", text)
}

func decodeValue(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, bool:
		return v, nil
	case json.Number:
		return parseInt(string(v))
	case string:
		return decodeScalar(v)
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
				if strings.HasPrefix(key, "$") {
					return decodeTagged(key, body)
				}
			}
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			if strings.HasPrefix(key, "$") {
				return nil, fmt.Errorf("tag %q beside other keys", key)
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

func decodeTagged(key string, body any) (any, error) {
	switch key {
	case "$cat":
		pieces, ok := body.([]any)
		if !ok {
			return nil, fmt.Errorf("$cat holds %T", body)
		}
		var b strings.Builder
		for _, piece := range pieces {
			text, ok := piece.(string)
			if !ok {
				return nil, fmt.Errorf("$cat piece %T", piece)
			}
			b.WriteString(text)
		}
		return b.String(), nil
	case "$tuple", "$set":
		list, err := decodeValue(body)
		if err != nil {
			return nil, err
		}
		items, ok := list.([]any)
		if !ok {
			return nil, fmt.Errorf("%s holds %T", key, list)
		}
		if key == "$tuple" {
			return pyTuple(items), nil
		}
		set := make(pySet, len(items))
		for i, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("$set element %T", item)
			}
			set[i] = text
		}
		return set, nil
	case "$profile":
		fields, err := decodeValue(body)
		if err != nil {
			return nil, err
		}
		dict, ok := fields.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("$profile holds %T", fields)
		}
		return pyProfile(dict), nil
	}
	return nil, fmt.Errorf("unknown object tag %q", key)
}

func decodeString(raw any) (string, error) {
	value, err := decodeValue(raw)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("want a str, got %T", value)
	}
	return text, nil
}

// sameValue compares two values of the model strictly: types must agree, ints
// and fractions compare by value, floats by bits.
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
	case *big.Int:
		g, ok := got.(*big.Int)
		return ok && g != nil && g.Cmp(w) == 0
	case float64:
		g, ok := got.(float64)
		return ok && math.Float64bits(g) == math.Float64bits(w)
	case *big.Rat:
		g, ok := got.(*big.Rat)
		return ok && g != nil && g.Cmp(w) == 0
	case []any:
		g, ok := got.([]any)
		return ok && sameList(w, g)
	case pyTuple:
		g, ok := got.(pyTuple)
		return ok && sameList(w, g)
	case pySet:
		g, ok := got.(pySet)
		return ok && strings.Join(w, "\x00") == strings.Join(g, "\x00") && len(w) == len(g)
	case map[string]any:
		g, ok := got.(map[string]any)
		return ok && sameDict(w, g)
	case pyProfile:
		g, ok := got.(pyProfile)
		return ok && sameDict(w, g)
	}
	return false
}

func sameList(want, got []any) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if !sameValue(want[i], got[i]) {
			return false
		}
	}
	return true
}

func sameDict(want, got map[string]any) bool {
	if len(want) != len(got) {
		return false
	}
	for key, item := range want {
		other, present := got[key]
		if !present || !sameValue(item, other) {
			return false
		}
	}
	return true
}

func show(v any) string {
	var text string
	switch value := v.(type) {
	case *big.Int:
		text = value.String()
	case *big.Rat:
		text = value.String()
	default:
		text = fmt.Sprintf("%#v", v)
	}
	if len(text) > 600 {
		text = text[:600] + "..."
	}
	return text
}

// outcome is what one call produced: a value, or an exception.
type outcome struct {
	value   any
	raised  bool
	errType string
	message string
}

func (o outcome) same(other outcome) bool {
	if o.raised || other.raised {
		return o.raised == other.raised && o.errType == other.errType && o.message == other.message
	}
	return sameValue(o.value, other.value)
}

func (o outcome) String() string {
	if o.raised {
		return fmt.Sprintf("raise %s(%q)", o.errType, o.message)
	}
	return "return " + show(o.value)
}

func pythonOutcome(result map[string]any) (outcome, error) {
	if raised, ok := result["raised"].(map[string]any); ok {
		kind, _ := raised["type"].(string)
		message, err := decodeString(raised["message"])
		return outcome{raised: true, errType: kind, message: message}, err
	}
	body, ok := result["ok"]
	if !ok {
		return outcome{}, errors.New("result has neither ok nor raised")
	}
	value, err := decodeValue(body)
	return outcome{value: value}, err
}

// replayFunc answers one named call of a case through the Go port.
type replayFunc func(fn string, in map[string]any) (outcome, error)

// runGoldens replays every call of every case in every golden file, and
// returns how many calls each function answered.
func runGoldens(t *testing.T, replay replayFunc) map[string]int {
	t.Helper()
	perFn := map[string]int{}
	var problems []string
	cases, calls := 0, 0
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			name, err := decodeString(c.RawName)
			if err != nil {
				t.Fatalf("%s: case name: %v", file.path, err)
			}
			in := make(map[string]any, len(c.In))
			for key, raw := range c.In {
				if in[key], err = decodeValue(raw); err != nil {
					t.Fatalf("%s %s input %s: %v", file.path, name, key, err)
				}
			}
			fns := make([]string, 0, len(c.Calls))
			for fn := range c.Calls {
				fns = append(fns, fn)
			}
			sort.Strings(fns)
			for _, fn := range fns {
				want, err := pythonOutcome(c.Calls[fn])
				if err != nil {
					t.Fatalf("%s %s %s: %v", file.path, name, fn, err)
				}
				got, err := replay(fn, in)
				if err != nil {
					t.Fatalf("%s %s %s: replay: %v", file.path, name, fn, err)
				}
				if !want.same(got) {
					problems = append(problems, fmt.Sprintf("%s %s %s:\n  Python %s\n  Go     %s", file.mode(), name, fn, want, got))
				}
				perFn[fn]++
				calls++
			}
			cases++
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
		t.Fatalf("%d mismatches over %d calls in %d Python cases", len(problems), calls, cases)
	}
	t.Logf("0 mismatches: %d calls in %d Python cases agree; per function %v", calls, cases, perFn)
	return perFn
}

func (f goldenFile) mode() string { return f.Mode }

// checkFuzzVolume proves the fuzz shards are complete and hold at least 2000
// calls of each function.
func checkFuzzVolume(t *testing.T, fns ...string) {
	t.Helper()
	perFn := map[string]int{}
	shards := map[int]bool{}
	declared, declaredShards, rendered := -1, -1, 0
	for _, file := range loadGoldens(t) {
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
			name, err := decodeString(c.RawName)
			if err != nil || !strings.HasPrefix(name, "fuzz/") {
				t.Fatalf("%s: case %v is not a fuzz case", file.path, c.RawName)
			}
			for fn := range c.Calls {
				perFn[fn]++
			}
			rendered++
		}
	}
	if len(shards) != declaredShards || rendered != declared {
		t.Fatalf("%d of %d shards present holding %d of %d fuzz cases", len(shards), declaredShards, rendered, declared)
	}
	for _, fn := range fns {
		if perFn[fn] < 2000 {
			t.Errorf("%s: %d fuzz calls, want at least 2000", fn, perFn[fn])
		}
	}
	t.Logf("fuzz calls per function: %v", perFn)
}

// Typed input readers. Each refuses a value the Go types cannot hold, so a
// golden that strays outside the typed contract fails loudly instead of being
// skipped.

func intInput(value any) (int64, error) {
	number, ok := value.(*big.Int)
	if !ok || !number.IsInt64() {
		return 0, fmt.Errorf("want an int64, got %s", show(value))
	}
	return number.Int64(), nil
}

func optionalIntInput(value any) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	number, err := intInput(value)
	return &number, err
}

func optionalStringInput(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("want str | None, got %s", show(value))
	}
	return &text, nil
}

func stringsInput(value any) ([]string, error) {
	var items []any
	switch list := value.(type) {
	case []any:
		items = list
	case pyTuple:
		items = list
	default:
		return nil, fmt.Errorf("want a list of str, got %s", show(value))
	}
	out := make([]string, len(items))
	for i, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("want a list of str, got element %s", show(item))
		}
		out[i] = text
	}
	return out, nil
}

// jsonbWords reads a JSONB list the way the repository must hand it over:
// absent or None is nil, and an element that is not a str becomes "".
func jsonbWords(value any, present bool) ([]string, error) {
	if !present || value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("want a JSONB list, got %s", show(value))
	}
	out := make([]string, len(items))
	for i, item := range items {
		if text, ok := item.(string); ok {
			out[i] = text
		}
	}
	return out, nil
}

// categoryInput reads `category`: absent or None becomes "", which no
// evidence names, exactly as `None in categories` is false.
func categoryInput(value any, present bool) (string, error) {
	if !present || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("want a str category, got %s", show(value))
	}
	return text, nil
}

func dictInput(value any) (map[string]any, error) {
	dict, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("want a dict, got %s", show(value))
	}
	return dict, nil
}

func stringList(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func optionalBig(value *int64) any {
	if value == nil {
		return nil
	}
	return big.NewInt(*value)
}
