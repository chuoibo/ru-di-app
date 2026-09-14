package blocking

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.domain.blocking in the parity API image. None of these
// functions raises, so every case compares a returned value.

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

func edgeArg(values map[string]any) (*Edge, error) {
	raw, ok := values["edge"]
	if !ok {
		return nil, errors.New("missing edge")
	}
	if raw == nil {
		return nil, nil
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("edge is %T", raw)
	}
	var edge Edge
	for key, value := range fields {
		switch key {
		case "state":
			state, err := text(value)
			if err != nil {
				return nil, err
			}
			edge.State = state
		case "decided_by_id":
			if value != nil {
				decider, err := text(value)
				if err != nil {
					return nil, err
				}
				edge.DecidedByID = &decider
			}
		default:
			return nil, fmt.Errorf("edge has key %q", key)
		}
	}
	if _, ok := fields["state"]; !ok {
		return nil, errors.New("edge without state: the typed Edge cannot say so")
	}
	return &edge, nil
}

func replay(c goldenCase) (any, error) {
	edge, err := edgeArg(c.Args)
	if err != nil {
		return nil, err
	}
	switch c.Fn {
	case "is_blocked":
		return IsBlocked(edge), nil
	case "blocker_of":
		if blocker := BlockerOf(edge); blocker != nil {
			return *blocker, nil
		}
		return nil, nil
	case "hidden_between":
		return HiddenBetween(edge), nil
	case "dm_allowed":
		deleted, ok := c.Args["other_deleted"].(bool)
		if !ok {
			return nil, errors.New("other_deleted is not a bool")
		}
		return DMAllowed(edge, deleted), nil
	}
	return nil, fmt.Errorf("unknown function %q", c.Fn)
}

func TestBlockingMatchesPython(t *testing.T) {
	perFn := map[string]int{}
	mismatches, total := 0, 0
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			body, ok := c.Result["ok"]
			if !ok {
				t.Fatalf("%s %s: Python raised %v", c.Fn, c.Name, c.Result["raised"])
			}
			want := body
			if s, isString := body.(string); isString {
				decoded, err := text(s)
				if err != nil {
					t.Fatal(err)
				}
				want = decoded
			}
			got, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			total++
			perFn[c.Fn]++
			if !reflect.DeepEqual(want, got) {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s(%v): Python %#v, Go %#v", c.Name, c.Fn, c.Args, want, got)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	for _, fn := range []string{"is_blocked", "blocker_of", "hidden_between", "dm_allowed"} {
		if perFn[fn] < 500 {
			t.Errorf("%s: %d cases, want at least 500", fn, perFn[fn])
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
	"DIRECT_MESSAGE_UNAVAILABLE": "DirectMessageUnavailable",
	"blocker_of":                 "BlockerOf",
	"dm_allowed":                 "DMAllowed",
	"hidden_between":             "HiddenBetween",
	"is_blocked":                 "IsBlocked",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		DirectMessageUnavailable string   `json:"direct_message_unavailable"`
		Names                    []string `json:"names"`
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
	if DirectMessageUnavailable != constants.DirectMessageUnavailable {
		t.Errorf("DIRECT_MESSAGE_UNAVAILABLE %q, Go %q", constants.DirectMessageUnavailable, DirectMessageUnavailable)
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
