package friendship

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.domain.friendship in the parity API image. Every case is
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

// plain expands an oracle value into nil, bool, string, []any and
// map[string]any, the only shapes this module returns.
func plain(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, bool:
		return v, nil
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
	return nil, fmt.Errorf("unexpected %T in this module's oracle", raw)
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
	if err != nil {
		return outcome{}, err
	}
	if m, isDict := value.(map[string]any); isDict {
		value, err = normaliseEdgeDict(m)
	}
	return outcome{value: value}, err
}

// normaliseEdgeDict reads an absent decided_by_id or pair as nil, the way the
// typed Edge does, and refuses any key an Edge has no field for.
func normaliseEdgeDict(m map[string]any) (map[string]any, error) {
	for key := range m {
		switch key {
		case "requester_id", "addressee_id", "state", "decided_by_id", "pair":
		default:
			return nil, fmt.Errorf("edge dict has key %q", key)
		}
	}
	for _, key := range []string{"decided_by_id", "pair"} {
		if _, present := m[key]; !present {
			m[key] = nil
		}
	}
	return m, nil
}

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var refused *FriendshipError
	if errors.As(err, &refused) {
		return outcome{errType: "FriendshipError", message: refused.Error(), code: refused.Code}
	}
	var invalid *ValueError
	if errors.As(err, &invalid) {
		return outcome{errType: "ValueError", message: invalid.Error()}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

func renderEdge(edge Edge) map[string]any {
	var decider, pair any
	if edge.DecidedByID != nil {
		decider = *edge.DecidedByID
	}
	if edge.Pair != nil {
		items := make([]any, len(edge.Pair))
		for i, id := range edge.Pair {
			items[i] = id
		}
		pair = items
	}
	return map[string]any{
		"requester_id":  edge.RequesterID,
		"addressee_id":  edge.AddresseeID,
		"state":         edge.State,
		"decided_by_id": decider,
		"pair":          pair,
	}
}

// args reads one case's arguments, keeping the first decoding error.
type args struct {
	values map[string]any
	err    error
}

func (a *args) fail(err error) {
	if a.err == nil {
		a.err = err
	}
}

func (a *args) text(key string) string {
	raw, ok := a.values[key]
	if !ok {
		a.fail(fmt.Errorf("missing argument %q", key))
		return ""
	}
	value, err := text(raw)
	a.fail(err)
	return value
}

func (a *args) edge(key string) *Edge {
	raw, ok := a.values[key]
	if !ok {
		a.fail(fmt.Errorf("missing argument %q", key))
		return nil
	}
	if raw == nil {
		return nil
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		a.fail(fmt.Errorf("%s: want a dict, got %T", key, raw))
		return nil
	}
	var edge Edge
	for field, value := range fields {
		var err error
		switch field {
		case "requester_id":
			edge.RequesterID, err = text(value)
		case "addressee_id":
			edge.AddresseeID, err = text(value)
		case "state":
			edge.State, err = text(value)
		case "decided_by_id":
			if value != nil {
				var decider string
				decider, err = text(value)
				edge.DecidedByID = &decider
			}
		case "pair":
			items, isList := value.([]any)
			if !isList {
				err = fmt.Errorf("pair is %T", value)
				break
			}
			edge.Pair = []string{}
			for _, item := range items {
				id, itemErr := text(item)
				if itemErr != nil {
					err = itemErr
				}
				edge.Pair = append(edge.Pair, id)
			}
		default:
			err = fmt.Errorf("edge has key %q", field)
		}
		a.fail(err)
	}
	return &edge
}

func replay(c goldenCase) (outcome, error) {
	in := &args{values: c.Args}
	var got outcome
	switch c.Fn {
	case "pair_key":
		pair, err := PairKey(in.text("a"), in.text("b"))
		got = goOutcome([]any{pair[0], pair[1]}, err)
	case "is_live_edge":
		occupied, err := IsLiveEdge(in.text("state"))
		got = goOutcome(occupied, err)
	case "open_request":
		edge, err := OpenRequest(in.text("requester_id"), in.text("addressee_id"), in.edge("existing"))
		got = goOutcome(renderEdge(edge), err)
	case "decide":
		edge := in.edge("edge")
		if edge == nil {
			in.fail(errors.New("decide needs an edge"))
			break
		}
		decided, err := Decide(*edge, in.text("actor_id"), in.text("decision"))
		got = goOutcome(renderEdge(decided), err)
	case "open_block":
		edge, err := OpenBlock(in.text("blocker_id"), in.text("addressee_id"), in.edge("existing"))
		got = goOutcome(renderEdge(edge), err)
	case "unblock":
		edge, err := Unblock(in.edge("edge"), in.text("actor_id"))
		got = goOutcome(renderEdge(edge), err)
	case "are_friends":
		friends, err := AreFriends(in.edge("edge"))
		got = goOutcome(friends, err)
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

func TestFriendshipMatchesPython(t *testing.T) {
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
	for _, fn := range []string{"pair_key", "is_live_edge", "open_request", "decide", "open_block", "unblock", "are_friends"} {
		if perFn[fn] < 900 {
			t.Errorf("%s: %d cases, want at least 900", fn, perFn[fn])
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

// goNames maps every public name of the Python module to its Go spelling. A
// new Python export fails the test until it is ported or listed here.
var goNames = map[string]string{
	"BLOCKED_IS_SILENT": "BlockedIsSilent",
	"Decision":          "Decisions",
	"FriendState":       "States",
	"FriendshipError":   "FriendshipError",
	"are_friends":       "AreFriends",
	"decide":            "Decide",
	"is_live_edge":      "IsLiveEdge",
	"open_block":        "OpenBlock",
	"open_request":      "OpenRequest",
	"pair_key":          "PairKey",
	"unblock":           "Unblock",
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		States          []string  `json:"states"`
		Decisions       []string  `json:"decisions"`
		BlockedIsSilent string    `json:"blocked_is_silent"`
		Live            []string  `json:"live"`
		Printable       [][2]rune `json:"printable"`
		Names           []string  `json:"names"`
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
	if !slices.Equal(States(), constants.States) || !slices.Equal(Decisions(), constants.Decisions) ||
		!slices.Equal(LiveStates(), constants.Live) || BlockedIsSilent != constants.BlockedIsSilent {
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
	if len(constants.Printable) == 0 {
		t.Fatal("no printable ranges recorded")
	}
	span := 0
	for r := rune(0); r <= utf8.MaxRune; r++ {
		for span < len(constants.Printable) && constants.Printable[span][1] < r {
			span++
		}
		want := span < len(constants.Printable) && constants.Printable[span][0] <= r
		if isPrintable(r) != want {
			t.Errorf("U+%04X: str.isprintable() is %v, Go %v", r, want, isPrintable(r))
		}
	}
}
