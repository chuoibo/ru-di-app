package postaudience

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.domain.post_audience in the parity API image. Every case
// is replayed here; an error must match Python's exception type, message and
// code.

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
		var file goldenFile
		if err := json.Unmarshal(raw, &file); err != nil {
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
	switch body.(type) {
	case nil, bool:
		return outcome{value: body}, nil
	}
	return outcome{}, fmt.Errorf("unexpected result %#v", body)
}

func goOutcome(result any, err error) outcome {
	if err == nil {
		return outcome{value: result}
	}
	var refused *AudienceError
	if errors.As(err, &refused) {
		return outcome{errType: "AudienceError", message: refused.Error(), code: refused.Code}
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

func (a *args) optText(raw any) *string {
	if raw == nil {
		return nil
	}
	s, err := text(raw)
	a.fail(err)
	return &s
}

func (a *args) flag(key string) bool {
	b, ok := a.raw(key).(bool)
	if !ok {
		a.fail(fmt.Errorf("%s is not a bool", key))
	}
	return b
}

func (a *args) post(key string) Post {
	fields, ok := a.raw(key).(map[string]any)
	if !ok || len(fields) != 3 {
		a.fail(fmt.Errorf("%s %v is not the service's post dict", key, fields))
	}
	author, err := text(fields["author_id"])
	a.fail(err)
	audience, err := text(fields["audience"])
	a.fail(err)
	context, present := fields["context_id"]
	if !present {
		a.fail(errors.New("post without context_id"))
	}
	return Post{AuthorID: author, Audience: audience, ContextID: a.optText(context)}
}

func replay(c goldenCase) (outcome, error) {
	in := &args{values: c.Args}
	var got outcome
	switch c.Fn {
	case "needs_context":
		got = goOutcome(NeedsContext(in.text("audience")), nil)
	case "check_writable":
		got = goOutcome(nil, CheckWritable(in.text("audience"), in.optText(in.raw("context_id"))))
	case "is_comment_policy":
		got = goOutcome(IsCommentPolicy(in.text("value")), nil)
	case "can_comment":
		got = goOutcome(CanComment(in.post("post"), in.text("policy"), in.text("reader_id"), in.flag("is_friend"),
			in.flag("is_group_member")), nil)
	case "visible_to":
		got = goOutcome(VisibleTo(in.post("post"), in.text("reader_id"), in.flag("is_friend"),
			in.flag("is_group_member"), in.flag("is_blocked")), nil)
	case "can_delete_comment":
		comment, ok := in.raw("comment").(map[string]any)
		if !ok || len(comment) != 1 {
			in.fail(fmt.Errorf("comment %v is not the service's dict", comment))
		}
		author, err := text(comment["author_id"])
		in.fail(err)
		got = goOutcome(CanDeleteComment(Comment{AuthorID: author}, in.post("post"), in.text("actor_id")), nil)
	case "can_read":
		got = goOutcome(CanRead(in.post("post"), in.text("reader_id"), in.flag("is_friend"),
			in.flag("is_group_member")), nil)
	default:
		return outcome{}, fmt.Errorf("unknown function %q", c.Fn)
	}
	return got, in.err
}

func TestPostAudienceMatchesPython(t *testing.T) {
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
			if want != got {
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
	for _, fn := range []string{"needs_context", "check_writable", "is_comment_policy", "can_comment",
		"visible_to", "can_delete_comment", "can_read"} {
		if perFn[fn] < 600 {
			t.Errorf("%s: %d cases, want at least 600", fn, perFn[fn])
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
	"AUDIENCES":              "Audiences",
	"AudienceError":          "AudienceError",
	"COMMENT_POLICIES":       "CommentPolicies",
	"DEFAULT_AUDIENCE":       "DefaultAudience",
	"DEFAULT_COMMENT_POLICY": "DefaultCommentPolicy",
	"can_comment":            "CanComment",
	"can_delete_comment":     "CanDeleteComment",
	"can_read":               "CanRead",
	"check_writable":         "CheckWritable",
	"is_comment_policy":      "IsCommentPolicy",
	"needs_context":          "NeedsContext",
	"visible_to":             "VisibleTo",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		Audiences            []string `json:"audiences"`
		DefaultAudience      string   `json:"default_audience"`
		CommentPolicies      []string `json:"comment_policies"`
		DefaultCommentPolicy string   `json:"default_comment_policy"`
		Names                []string `json:"names"`
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
	if !slices.Equal(Audiences(), constants.Audiences) || DefaultAudience != constants.DefaultAudience ||
		!slices.Equal(CommentPolicies(), constants.CommentPolicies) ||
		DefaultCommentPolicy != constants.DefaultCommentPolicy {
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
