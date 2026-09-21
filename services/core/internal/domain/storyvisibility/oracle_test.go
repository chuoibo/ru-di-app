package storyvisibility

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
// from the real app.domain.story_visibility in the parity API image. Every
// case is replayed here; an error must match Python's exception type, message
// and code.

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

// isoformat renders a time the way the oracle writes a datetime.
func isoformat(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign, offset = "-", -offset
	}
	zone := fmt.Sprintf("%s%02d:%02d", sign, offset/3600, offset%3600/60)
	if offset%60 != 0 {
		zone += fmt.Sprintf(":%02d", offset%60)
	}
	return "$dt:" + t.Format("2006-01-02T15:04:05.000000") + "|" + zone
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
	switch v := body.(type) {
	case []any:
		indices := make([]any, len(v))
		for i, item := range v {
			n, err := strconv.Atoi(fmt.Sprint(item))
			if err != nil {
				return outcome{}, err
			}
			indices[i] = n
		}
		return outcome{value: indices}, nil
	case json.Number:
		return outcome{}, fmt.Errorf("unexpected number %v", v)
	}
	return outcome{value: body}, nil
}

func goOutcome(result any, err error) outcome {
	if err == nil {
		return outcome{value: result}
	}
	var refused *StoryError
	if errors.As(err, &refused) {
		return outcome{errType: "StoryError", message: refused.Error(), code: refused.Code}
	}
	var overflow *OverflowError
	if errors.As(err, &overflow) {
		return outcome{errType: "OverflowError", message: overflow.Error()}
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

func (a *args) instant(key string) time.Time {
	moment, err := instant(a.raw(key))
	a.fail(err)
	return moment
}

func (a *args) story(key string) Story {
	fields, ok := a.raw(key).(map[string]any)
	if !ok {
		a.fail(fmt.Errorf("%s is not a dict", key))
	}
	story := Story{}
	for name, raw := range fields {
		var err error
		switch name {
		case "author_id":
			story.AuthorID, err = text(raw)
		case "audience":
			story.Audience, err = text(raw)
		case "expires_at":
			story.ExpiresAt, err = instant(raw)
		default:
			err = fmt.Errorf("story has key %q", name)
		}
		a.fail(err)
	}
	if len(fields) != 3 {
		a.fail(fmt.Errorf("story dict %v is not the service's shape", fields))
	}
	return story
}

func (a *args) groups(key string) []AuthorFacts {
	items, ok := a.raw(key).([]any)
	if !ok {
		a.fail(fmt.Errorf("%s is not a list", key))
	}
	groups := []AuthorFacts{}
	for _, item := range items {
		fields, _ := item.(map[string]any)
		mine, okMine := fields["mine"].(bool)
		seen, okSeen := fields["all_seen"].(bool)
		latest, err := instant(fields["latest_at"])
		if !okMine || !okSeen || len(fields) != 3 {
			err = fmt.Errorf("group %v is not the service's shape", fields)
		}
		a.fail(err)
		groups = append(groups, AuthorFacts{Mine: mine, AllSeen: seen, LatestAt: latest})
	}
	return groups
}

func replay(c goldenCase) (outcome, error) {
	in := &args{values: c.Args}
	var got outcome
	switch c.Fn {
	case "expires_at_for":
		expires, err := ExpiresAtFor(in.instant("created_at"))
		got = goOutcome(isoformat(expires), err)
	case "is_live":
		got = goOutcome(IsLive(in.story("story"), in.instant("now")), nil)
	case "check_caption":
		var caption *string
		if raw := in.raw("caption"); raw != nil {
			s, err := text(raw)
			in.fail(err)
			caption = &s
		}
		got = goOutcome(nil, CheckCaption(caption))
	case "can_view":
		got = goOutcome(CanView(in.story("story"), in.text("reader_id"), in.flag("is_friend"),
			in.flag("is_blocked"), in.instant("now")), nil)
	case "order_authors":
		type indexed struct {
			index int
			facts AuthorFacts
		}
		var rows []indexed
		for i, facts := range in.groups("groups") {
			rows = append(rows, indexed{index: i, facts: facts})
		}
		ordered := OrderAuthors(rows, func(row indexed) AuthorFacts { return row.facts })
		indices := make([]any, len(ordered))
		for i, row := range ordered {
			indices[i] = row.index
		}
		got = goOutcome(indices, nil)
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

func TestStoryVisibilityMatchesPython(t *testing.T) {
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
	for _, fn := range []string{"expires_at_for", "is_live", "check_caption", "can_view", "order_authors"} {
		if perFn[fn] < 800 {
			t.Errorf("%s: %d cases, want at least 800", fn, perFn[fn])
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
	"DEFAULT_STORY_AUDIENCE": "DefaultStoryAudience",
	"MAX_CAPTION_LENGTH":     "MaxCaptionLength",
	"STORY_AUDIENCES":        "StoryAudiences",
	"STORY_TTL":              "StoryTTL",
	"StoryError":             "StoryError",
	"can_view":               "CanView",
	"check_caption":          "CheckCaption",
	"expires_at_for":         "ExpiresAtFor",
	"is_live":                "IsLive",
	"order_authors":          "OrderAuthors",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		TTLMicroseconds  string   `json:"ttl_microseconds"`
		Audiences        []string `json:"audiences"`
		DefaultAudience  string   `json:"default_audience"`
		MaxCaptionLength int      `json:"max_caption_length"`
		Names            []string `json:"names"`
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
	ttl, err := strconv.ParseInt(strings.TrimPrefix(constants.TTLMicroseconds, "$i:"), 0, 64)
	if err != nil || time.Duration(ttl)*time.Microsecond != StoryTTL {
		t.Errorf("STORY_TTL %s µs, Go %v (%v)", constants.TTLMicroseconds, StoryTTL, err)
	}
	if !slices.Equal(StoryAudiences(), constants.Audiences) || DefaultStoryAudience != constants.DefaultAudience ||
		MaxCaptionLength != constants.MaxCaptionLength {
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
