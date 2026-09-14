package photoref

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.domain.photo_ref in the parity API image.

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
		return "", fmt.Errorf("unexpected tag in %q", s)
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

func arg(c goldenCase, name string) string {
	value, err := text(c.Args[name])
	if err != nil {
		panic(fmt.Sprintf("%s %s: %s: %v", c.Fn, c.Name, name, err))
	}
	return value
}

// outcome renders one call the way the golden records it.
type outcome struct {
	ok     map[string]string
	str    *string
	raised *[2]string // code, message
}

func pythonOutcome(c goldenCase) outcome {
	if raised, ok := c.Result["raised"].(map[string]any); ok {
		code, _ := text(raised["code"])
		message, _ := text(raised["message"])
		if raised["type"] != "PhotoUrlError" {
			code = "type:" + fmt.Sprint(raised["type"])
		}
		return outcome{raised: &[2]string{code, message}}
	}
	switch body := c.Result["ok"].(type) {
	case map[string]any:
		out := map[string]string{}
		for key, value := range body {
			decoded, err := text(value)
			if err != nil {
				panic(err)
			}
			out[key] = decoded
		}
		return outcome{ok: out}
	default:
		decoded, err := text(body)
		if err != nil {
			panic(err)
		}
		return outcome{str: &decoded}
	}
}

func goOutcome(c goldenCase) (outcome, error) {
	switch c.Fn {
	case "parse_photo_url":
		ref, err := Parse(arg(c, "image_url"))
		if err != nil {
			refusal, ok := err.(*PhotoURLError)
			if !ok {
				return outcome{}, err
			}
			return outcome{raised: &[2]string{refusal.Code, refusal.Error()}}, nil
		}
		return outcome{ok: map[string]string{
			"owner_kind": ref.OwnerKind, "owner_id": ref.OwnerID, "photo_id": ref.PhotoID, "url": ref.URL(),
		}}, nil
	case "person_photo_url":
		url := PersonPhotoURL(arg(c, "person_id"), arg(c, "photo_id"))
		return outcome{str: &url}, nil
	case "context_photo_url":
		url := ContextPhotoURL(arg(c, "context_id"), arg(c, "photo_id"))
		return outcome{str: &url}, nil
	}
	return outcome{}, fmt.Errorf("unknown function %q", c.Fn)
}

func (o outcome) String() string {
	switch {
	case o.raised != nil:
		return fmt.Sprintf("raised %q %q", o.raised[0], o.raised[1])
	case o.str != nil:
		return fmt.Sprintf("%q", *o.str)
	}
	keys := make([]string, 0, len(o.ok))
	for key := range o.ok {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = key + "=" + o.ok[key]
	}
	return "ok " + strings.Join(parts, " ")
}

func TestPhotoRefMatchesPython(t *testing.T) {
	perFn := map[string]int{}
	returned, refused := 0, 0
	mismatches, total := 0, 0
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			want := pythonOutcome(c)
			got, err := goOutcome(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			total++
			perFn[c.Fn]++
			if want.raised != nil {
				refused++
			} else {
				returned++
			}
			if want.String() != got.String() {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s(%v):\n  Python %s\n  Go     %s", c.Name, c.Fn, c.Args, want, got)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	if perFn["parse_photo_url"] < 3000 || returned < 500 || refused < 500 {
		t.Fatalf("thin corpus: %v, %d returned, %d refused", perFn, returned, refused)
	}
	t.Logf("%d Python cases agree, 0 mismatches (%d returned, %d refused); per function %v", total, returned, refused, perFn)
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
	"OWNER_CONTEXT":     "OwnerContext",
	"OWNER_PERSON":      "OwnerPerson",
	"PhotoRef":          "Ref",
	"PhotoUrlError":     "PhotoURLError",
	"context_photo_url": "ContextPhotoURL",
	"parse_photo_url":   "Parse",
	"person_photo_url":  "PersonPhotoURL",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		OwnerContext string   `json:"owner_context"`
		OwnerPerson  string   `json:"owner_person"`
		Names        []string `json:"names"`
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
	if OwnerContext != constants.OwnerContext || OwnerPerson != constants.OwnerPerson {
		t.Errorf("owner words: Python %q %q, Go %q %q", constants.OwnerContext, constants.OwnerPerson, OwnerContext, OwnerPerson)
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
