package vote

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.domain.vote in the parity API image. Every case is
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

// plain expands an oracle value into nil, bool, int, string, []any and
// map[string]any.
func plain(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, bool:
		return v, nil
	case json.Number:
		return strconv.Atoi(string(v))
	case string:
		return text(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			value, err := plain(item)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			value, err := plain(item)
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected JSON %T", raw)
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
	value, err := plain(body)
	return outcome{value: value}, err
}

func renderResult(result Result) map[string]any {
	counts := make([]any, len(result.Counts))
	for i, count := range result.Counts {
		counts[i] = []any{count.OptionID, count.Count}
	}
	leading := make([]any, len(result.LeadingOptionIDs))
	for i, id := range result.LeadingOptionIDs {
		leading[i] = id
	}
	var decided any
	if result.DecidedOptionID != nil {
		decided = *result.DecidedOptionID
	}
	return map[string]any{
		"total_ballots":      result.TotalBallots,
		"counts":             counts,
		"leading_option_ids": leading,
		"is_tie":             result.IsTie,
		"decided_option_id":  decided,
	}
}

func goOutcome(result Result, err error) outcome {
	if err == nil {
		return outcome{value: renderResult(result)}
	}
	var refused *VoteError
	if errors.As(err, &refused) {
		return outcome{errType: "VoteError", message: refused.Error(), code: refused.Code}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

func replay(c goldenCase) (outcome, [][2]any, error) {
	if c.Fn != "tally" {
		return outcome{}, nil, fmt.Errorf("unknown function %q", c.Fn)
	}
	decoded, err := plain(map[string]any(c.Args))
	if err != nil {
		return outcome{}, nil, err
	}
	fields := decoded.(map[string]any)
	var options []Option
	for _, item := range fields["options"].([]any) {
		row := item.(map[string]any)
		id, isText := row["id"].(string)
		position, isInt := row["position"].(int)
		if !isText || !isInt || len(row) != 2 {
			return outcome{}, nil, fmt.Errorf("option %v is not the service's dict", row)
		}
		options = append(options, Option{ID: id, Position: position})
	}
	var ballots []Ballot
	for _, item := range fields["ballots"].([]any) {
		row := item.(map[string]any)
		voter, isVoter := row["voter_id"].(string)
		option, isOption := row["option_id"].(string)
		if !isVoter || !isOption || len(row) != 2 {
			return outcome{}, nil, fmt.Errorf("ballot %v is not the service's dict", row)
		}
		ballots = append(ballots, Ballot{VoterID: voter, OptionID: option})
	}
	result, err := Tally(options, ballots)
	var lookups [][2]any
	if err == nil {
		for _, option := range options {
			count, found := result.CountOf(option.ID)
			if !found {
				return outcome{}, nil, fmt.Errorf("CountOf(%q) found nothing", option.ID)
			}
			lookups = append(lookups, [2]any{option.ID, count})
		}
	}
	return goOutcome(result, err), lookups, nil
}

func same(want, got outcome) bool {
	if want.errType != "" || got.errType != "" {
		return want.errType == got.errType && want.message == got.message && want.code == got.code
	}
	return reflect.DeepEqual(want.value, got.value)
}

func TestTallyMatchesPython(t *testing.T) {
	mismatches, total, decided, ties := 0, 0, 0, 0
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			want, err := pythonOutcome(c)
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			got, lookups, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			total++
			if !same(want, got) {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s tally(%v):\n  Python %s\n  Go     %s", file.Mode, c.Name, c.Args, want, got)
				}
				continue
			}
			if result, ok := want.value.(map[string]any); ok {
				if result["decided_option_id"] != nil {
					decided++
				}
				if result["is_tie"] == true {
					ties++
				}
				// The service reads `result["counts"][option.id]` for every option:
				// CountOf must find each one with the dict's number.
				counts := map[any]any{}
				for _, pair := range result["counts"].([]any) {
					entry := pair.([]any)
					counts[entry[0]] = entry[1]
				}
				for _, lookup := range lookups {
					if counts[lookup[0]] != lookup[1] {
						t.Errorf("%s: CountOf(%v) is %v, Python's counts say %v", c.Name, lookup[0], lookup[1], counts[lookup[0]])
					}
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	if total < 2400 || decided < 100 || ties < 100 {
		t.Errorf("%d cases with %d decided and %d ties: the corpus lost its spread", total, decided, ties)
	}
	t.Logf("%d Python cases agree, 0 mismatches; %d decided, %d ties", total, decided, ties)
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
	"VoteError": "VoteError",
	"tally":     "Tally",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		Names []string `json:"names"`
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
	if len(constants.Names) != len(goNames) {
		t.Errorf("Python has %d public names %v, goNames lists %d", len(constants.Names), constants.Names, len(goNames))
	}
	for _, name := range constants.Names {
		if goNames[name] == "" {
			t.Errorf("Python exports %s and the port lists no counterpart", name)
		}
	}
}
