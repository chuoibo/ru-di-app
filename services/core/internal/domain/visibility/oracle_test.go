package visibility

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.domain.visibility in the parity API image. Every case is
// replayed here; an error must match Python's exception type, message and code.

type goldenFile struct {
	Mode      string          `json:"mode"`
	Constants json.RawMessage `json:"constants"`
	Fuzz      *struct {
		Shard  int `json:"shard"`
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Fn     string         `json:"fn"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
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
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		var file goldenFile
		if err := decoder.Decode(&file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		files = append(files, file)
	}
	return files
}

// text reads a str of the oracle encoding: plain, or "$sp:" groups of six
// code points joined by "|".
func text(raw any) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("want a str, got %T", raw)
	}
	if !strings.HasPrefix(s, "$") {
		return s, nil
	}
	body, found := strings.CutPrefix(s, "$sp:")
	if !found {
		return "", fmt.Errorf("not a str: %q", s)
	}
	var out strings.Builder
	count := 0
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRuneInString(body[i:])
		i += size
		if count == 6 {
			if r != '|' {
				return "", fmt.Errorf("bad group separator in %q", s)
			}
			count = 0
			continue
		}
		out.WriteRune(r)
		count++
	}
	return out.String(), nil
}

// value expands the oracle encoding into the package's value model: nil,
// bool, int64, float64, string, []any, map[string]any.
func value(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, bool:
		return v, nil
	case json.Number:
		return strconv.ParseInt(string(v), 10, 64)
	case string:
		switch {
		case strings.HasPrefix(v, "$i:"):
			return strconv.ParseInt(strings.TrimPrefix(v, "$i:"), 0, 64)
		case strings.HasPrefix(v, "$f:"):
			return strconv.ParseFloat(strings.TrimPrefix(v, "$f:"), 64)
		}
		return text(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			decoded, err := value(item)
			if err != nil {
				return nil, err
			}
			out[i] = decoded
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			decoded, err := value(item)
			if err != nil {
				return nil, err
			}
			out[key] = decoded
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected JSON %T", raw)
}

// instant reads "$dt:<wall>|<offset>", the offset being ±HH:MM or ±HH:MM:SS.
func instant(raw any) (time.Time, error) {
	s, _ := raw.(string)
	body, found := strings.CutPrefix(s, "$dt:")
	wall, offset, cut := strings.Cut(body, "|")
	if !found || !cut || len(offset) < 6 {
		return time.Time{}, fmt.Errorf("not a datetime: %v", raw)
	}
	clock, err := time.Parse("2006-01-02T15:04:05.000000", wall)
	if err != nil {
		return time.Time{}, err
	}
	parts := strings.Split(offset[1:], ":")
	seconds := 0
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || i > 2 {
			return time.Time{}, fmt.Errorf("bad offset %q", offset)
		}
		seconds += n * []int{3600, 60, 1}[i]
	}
	if offset[0] == '-' {
		seconds = -seconds
	}
	return time.Date(clock.Year(), clock.Month(), clock.Day(), clock.Hour(), clock.Minute(), clock.Second(),
		clock.Nanosecond(), time.FixedZone("", seconds)), nil
}

type outcome struct {
	value   any
	errType string
	message string
	code    any
}

func (o outcome) String() string {
	if o.errType != "" {
		return fmt.Sprintf("raise %s(%q) code=%v", o.errType, o.message, o.code)
	}
	return fmt.Sprintf("return %#v", o.value)
}

func pythonOutcome(c goldenCase) (outcome, error) {
	if raised, ok := c.Result["raised"].(map[string]any); ok {
		kind, _ := raised["type"].(string)
		message, err := text(raised["message"])
		if err != nil {
			return outcome{}, err
		}
		var code any
		if raised["code"] != nil {
			if code, err = text(raised["code"]); err != nil {
				return outcome{}, err
			}
		}
		return outcome{errType: kind, message: message, code: code}, nil
	}
	body, ok := c.Result["ok"]
	if !ok {
		return outcome{}, errors.New("result has neither ok nor raised")
	}
	decoded, err := value(body)
	return outcome{value: decoded}, err
}

func goOutcome(result any, err error) outcome {
	if err == nil {
		return outcome{value: result}
	}
	var refused *VisibilityError
	if errors.As(err, &refused) {
		return outcome{errType: "VisibilityError", message: refused.Error(), code: refused.Code}
	}
	var wrongType *TypeError
	if errors.As(err, &wrongType) {
		return outcome{errType: "TypeError", message: wrongType.Error()}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

type args struct {
	values map[string]any
	err    error
}

func (a *args) fail(err error) {
	if a.err == nil && err != nil {
		a.err = err
	}
}

func (a *args) raw(key string) any {
	raw, ok := a.values[key]
	if !ok {
		a.fail(fmt.Errorf("missing argument %q", key))
	}
	return raw
}

func (a *args) text(key string) string {
	s, err := text(a.raw(key))
	a.fail(err)
	return s
}

func (a *args) flag(key string) bool {
	b, ok := a.raw(key).(bool)
	if !ok {
		a.fail(fmt.Errorf("%s is not a bool", key))
	}
	return b
}

func (a *args) texts(key string) []string {
	items, ok := a.raw(key).([]any)
	if !ok {
		a.fail(fmt.Errorf("%s is not a list", key))
	}
	out := []string{}
	for _, item := range items {
		s, err := text(item)
		a.fail(err)
		out = append(out, s)
	}
	return out
}

func (a *args) dict(key string) map[string]any {
	decoded, err := value(a.raw(key))
	a.fail(err)
	m, ok := decoded.(map[string]any)
	if !ok {
		a.fail(fmt.Errorf("%s is not a dict", key))
	}
	return m
}

func (a *args) instant(key string) *time.Time {
	raw := a.raw(key)
	if raw == nil {
		return nil
	}
	moment, err := instant(raw)
	a.fail(err)
	return &moment
}

func (a *args) field(key string) Field {
	fields, ok := a.raw(key).(map[string]any)
	if !ok {
		a.fail(fmt.Errorf("%s is not a dict", key))
	}
	var field Field
	for name, raw := range fields {
		var err error
		switch name {
		case "id":
			field.ID, err = text(raw)
		case "owner_id":
			if raw != nil {
				var owner string
				owner, err = text(raw)
				field.OwnerID = &owner
			}
		case "component":
			field.Component, err = text(raw)
		case "visibility":
			field.Visibility, err = text(raw)
		default:
			err = fmt.Errorf("field has key %q", name)
		}
		a.fail(err)
	}
	for _, name := range []string{"id", "component", "visibility"} {
		if _, ok := fields[name]; !ok {
			a.fail(fmt.Errorf("field without %s: the typed Field cannot say so", name))
		}
	}
	return field
}

func renderDerivative(d Derivative) map[string]any {
	return map[string]any{
		"derived_from_id":             d.DerivedFromID,
		"component":                   d.Component,
		"visibility":                  d.Visibility,
		"redaction":                   d.Redaction,
		"declassified_by":             d.DeclassifiedBy,
		"source_visibility_unchanged": d.SourceVisibilityUnchanged,
	}
}

func replay(c goldenCase) (outcome, error) {
	in := &args{values: c.Args}
	var got outcome
	switch c.Fn {
	case "rank":
		rank, err := Rank(in.text("level"))
		got = goOutcome(int64(rank), err)
	case "permitted_output_visibility":
		level, err := PermittedOutputVisibility(in.texts("input_levels"))
		got = goOutcome(level, err)
	case "check_no_context_laundering":
		err := CheckNoContextLaundering(in.text("component"), in.text("requested_level"), in.texts("input_levels"),
			in.flag("redacted"), in.flag("owner_consented"))
		got = goOutcome(nil, err)
	case "declassify":
		derivative, err := Declassify(in.field("field"), in.text("to_level"), in.text("actor_id"), in.dict("redaction"))
		got = goOutcome(renderDerivative(derivative), err)
	case "can_view_history":
		created := in.instant("object_created_at")
		if created == nil {
			in.fail(errors.New("object_created_at is None"))
			break
		}
		got = goOutcome(CanViewHistory(in.text("object_visibility"), in.instant("viewer_joined_at"),
			in.instant("viewer_left_at"), *created, in.texts("audience_snapshot"), in.text("viewer_id")), nil)
	case "settlement_view":
		view, err := SettlementView(in.dict("obligation"))
		got = goOutcome(view, err)
	default:
		return outcome{}, fmt.Errorf("unknown function %q", c.Fn)
	}
	return got, in.err
}

func same(want, got outcome) bool {
	if want.errType != "" || got.errType != "" {
		return want.errType == got.errType && want.message == got.message && want.code == got.code
	}
	return reflect.DeepEqual(want.value, got.value)
}

func TestVisibilityMatchesPython(t *testing.T) {
	perFn := map[string]int{}
	mismatches, total := 0, 0
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			want, err := pythonOutcome(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			got, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			total++
			perFn[c.Fn]++
			if !same(want, got) {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s %s(%v):\n  Python %s\n  Go     %s", file.Mode, c.Name, c.Fn, c.Args, want, got)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	for _, fn := range []string{"rank", "permitted_output_visibility", "check_no_context_laundering",
		"declassify", "can_view_history", "settlement_view"} {
		if perFn[fn] < 700 {
			t.Errorf("%s: %d cases, want at least 700", fn, perFn[fn])
		}
	}
	t.Logf("%d Python cases agree, 0 mismatches; per function %v", total, perFn)
}

func TestFuzzShardsAreComplete(t *testing.T) {
	seen := map[int]bool{}
	declared, shards, rendered := -1, -1, 0
	for _, file := range loadGoldens(t) {
		if file.Fuzz == nil {
			continue
		}
		if declared >= 0 && (declared != file.Fuzz.Total || shards != file.Fuzz.Shards) {
			t.Fatal("fuzz shards disagree on their totals")
		}
		declared, shards = file.Fuzz.Total, file.Fuzz.Shards
		if seen[file.Fuzz.Shard] {
			t.Fatalf("shard %d appears twice", file.Fuzz.Shard)
		}
		seen[file.Fuzz.Shard] = true
		rendered += len(file.Cases)
	}
	if shards < 1 || len(seen) != shards || rendered != declared {
		t.Fatalf("%d of %d shards holding %d of %d fuzz cases", len(seen), shards, rendered, declared)
	}
}

var goNames = map[string]string{
	"DEFAULT_VISIBILITY":          "DefaultVisibility",
	"LEVELS":                      "Levels",
	"NEVER_GROUP_VISIBLE":         "NeverGroupVisible",
	"REDACTABLE":                  "Redactable",
	"SETTLEMENT_VIEW_FIELDS":      "SettlementViewFields",
	"VisibilityError":             "VisibilityError",
	"can_view_history":            "CanViewHistory",
	"check_no_context_laundering": "CheckNoContextLaundering",
	"declassify":                  "Declassify",
	"permitted_output_visibility": "PermittedOutputVisibility",
	"rank":                        "Rank",
	"settlement_view":             "SettlementView",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		Levels               []string    `json:"levels"`
		DefaultVisibility    [][2]string `json:"default_visibility"`
		NeverGroupVisible    []string    `json:"never_group_visible"`
		Redactable           []string    `json:"redactable"`
		SettlementViewFields []string    `json:"settlement_view_fields"`
		Names                []string    `json:"names"`
	}
	found := 0
	for _, file := range loadGoldens(t) {
		if file.Constants == nil {
			continue
		}
		found++
		if err := json.Unmarshal(file.Constants, &constants); err != nil {
			t.Fatal(err)
		}
	}
	if found != 1 {
		t.Fatalf("%d files carry constants, want 1", found)
	}
	var defaults [][2]string
	for _, entry := range DefaultVisibility() {
		defaults = append(defaults, [2]string{entry.Component, entry.Level})
	}
	if !slices.Equal(Levels(), constants.Levels) || !slices.Equal(defaults, constants.DefaultVisibility) ||
		!slices.Equal(NeverGroupVisible(), constants.NeverGroupVisible) ||
		!slices.Equal(Redactable(), constants.Redactable) ||
		!slices.Equal(SettlementViewFields(), constants.SettlementViewFields) {
		t.Errorf("constants differ: Python %+v", constants)
	}
	if len(constants.Names) != len(goNames) {
		t.Errorf("Python has %d public names %v, goNames lists %d", len(constants.Names), constants.Names, len(goNames))
	}
	for _, name := range constants.Names {
		if goNames[name] == "" {
			t.Errorf("Python exports %s and the port lists no counterpart", name)
		}
	}
}
