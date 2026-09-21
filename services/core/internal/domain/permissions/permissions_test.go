package permissions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

// goldens is testdata/python_permissions.json, rendered from the real Python
// module by scripts/render_permissions_goldens.py. Its docstring explains
// each section.
type goldens struct {
	Roles            []string `json:"roles"`
	Actions          []string `json:"actions"`
	KnownPredicates  []string `json:"known_predicates"`
	RoleVocab        []string `json:"role_vocab"`
	PredicateVocab   []string `json:"predicate_vocab"`
	ExhaustiveExtras []string `json:"exhaustive_extras"`
	ExhaustiveCases  int      `json:"exhaustive_cases"`
	Table            []struct {
		Action   string      `json:"action"`
		Roles    []string    `json:"roles"`
		Requires []string    `json:"requires"`
		Reasons  [][]*string `json:"reasons"`
	} `json:"table"`
	Malformed []recorded `json:"malformed"`
	Sample    []recorded `json:"sample"`
	Untyped   []struct {
		Action string  `json:"action"`
		Facts  string  `json:"facts"`
		Error  *string `json:"error"`
	} `json:"untyped"`
}

// recorded is one call as Python answered it. Roles and Proven are indices
// into RoleVocab and PredicateVocab.
type recorded struct {
	Action     string  `json:"action"`
	ActorID    string  `json:"actor_id"`
	Roles      []int   `json:"roles"`
	ResourceID *string `json:"resource_id"`
	Proven     []int   `json:"proven"`
	Provenance string  `json:"provenance"`
	FactsError *string `json:"facts_error"`
	Error      *string `json:"error"`
	Reason     *string `json:"reason"`
	Can        *bool   `json:"can"`
}

func load(t *testing.T) goldens {
	t.Helper()
	data, err := os.ReadFile("testdata/python_permissions.json")
	if err != nil {
		t.Fatalf("goldens missing, render them with scripts/render_permissions_goldens.py: %v", err)
	}
	var g goldens
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Roles) != 10 || len(g.Table) < 100 || len(g.Table) != len(g.Actions) ||
		len(g.Malformed) < 290 || len(g.Sample) < 2000 || len(g.Untyped) < 4 ||
		g.ExhaustiveCases < 900000 {
		t.Fatalf("golden file looks truncated: roles=%d table=%d actions=%d malformed=%d sample=%d untyped=%d exhaustive=%d",
			len(g.Roles), len(g.Table), len(g.Actions), len(g.Malformed), len(g.Sample), len(g.Untyped), g.ExhaustiveCases)
	}
	return g
}

// mismatches reports the first few differences and counts the rest, so a
// table-wide regression does not print a million lines.
type mismatches struct {
	t *testing.T
	n int
}

func (m *mismatches) errorf(format string, args ...any) {
	m.t.Helper()
	m.n++
	if m.n <= 25 {
		m.t.Errorf(format, args...)
	}
}

func (m *mismatches) finish(cases int) {
	m.t.Helper()
	if m.n > 25 {
		m.t.Errorf("... %d mismatches in total", m.n)
	}
	m.t.Logf("%d cases, %d mismatches", cases, m.n)
}

var sentinels = map[string]*Error{
	CodeAnonymousActor:         ErrAnonymousActor,
	CodeFactsWithoutProvenance: ErrFactsWithoutProvenance,
	CodeUnknownRole:            ErrUnknownRole,
	CodeUnknownAction:          ErrUnknownAction,
}

// codeOf is the PermissionError_ code a Go error stands for, and says so when
// the error is not one of this package's sentinels.
func codeOf(err error) string {
	var perm *Error
	if !errors.As(err, &perm) {
		return fmt.Sprintf("foreign error %q", err)
	}
	sentinel, ok := sentinels[perm.Code]
	if !ok || !errors.Is(err, sentinel) || err.Error() != perm.Code {
		return fmt.Sprintf("unsentinelled error %q", perm.Code)
	}
	return perm.Code
}

func describeReason(reason string, allowed bool, err error) string {
	switch {
	case err != nil:
		if reason != "" || allowed {
			return fmt.Sprintf("error %s with reason %q allowed=%v", codeOf(err), reason, allowed)
		}
		return "error " + codeOf(err)
	case allowed:
		if reason != "" {
			return "allowed with reason " + strconv.Quote(reason)
		}
		return "allowed"
	default:
		return "reason " + strconv.Quote(reason)
	}
}

func describeCan(allowed bool, err error) string {
	if err != nil {
		if allowed {
			return "error " + codeOf(err) + " with true"
		}
		return "error " + codeOf(err)
	}
	return strconv.FormatBool(allowed)
}

func wantReason(r recorded) string {
	switch {
	case r.FactsError != nil:
		return "error " + *r.FactsError
	case r.Error != nil:
		return "error " + *r.Error
	case r.Reason == nil:
		return "allowed"
	default:
		return "reason " + strconv.Quote(*r.Reason)
	}
}

func wantCan(r recorded) string {
	switch {
	case r.FactsError != nil:
		return "error " + *r.FactsError
	case r.Error != nil:
		return "error " + *r.Error
	case r.Can == nil:
		return "missing can"
	default:
		return strconv.FormatBool(*r.Can)
	}
}

func projected(reason *string) string {
	if reason == nil {
		return "allowed"
	}
	return "reason " + strconv.Quote(*reason)
}

func subset(items []string, mask int) []string {
	var out []string
	for bit, item := range items {
		if mask>>bit&1 == 1 {
			out = append(out, item)
		}
	}
	return out
}

func validFacts(roles, proven []string) AuthorizationFacts {
	resource := "r1"
	return AuthorizationFacts{
		ActorID:    "u1",
		Roles:      roles,
		ResourceID: &resource,
		Proven:     proven,
		Provenance: "api_service",
	}
}

func TestRolesAndActionsMatchPython(t *testing.T) {
	g := load(t)
	if got := Roles(); !reflect.DeepEqual(got, g.Roles) {
		t.Errorf("Roles() = %q\nPython ROLES = %q", got, g.Roles)
	}
	if got := Actions(); !reflect.DeepEqual(got, g.Actions) {
		t.Errorf("Actions() = %q\nPython ACTIONS = %q", got, g.Actions)
	}
	Roles()[0] = "mutated"
	Actions()[0] = "mutated"
	if Roles()[0] != g.Roles[0] || Actions()[0] != g.Actions[0] {
		t.Error("Roles() and Actions() must return copies")
	}
}

// TestTableMatchesPython compares the data itself, which names the entry
// that drifted before the behavioural replays below say how.
func TestTableMatchesPython(t *testing.T) {
	g := load(t)
	if len(table) != len(g.Table) {
		t.Errorf("Go table has %d actions, Python %d", len(table), len(g.Table))
	}
	for _, row := range g.Table {
		entry, ok := table[row.Action]
		if !ok {
			t.Errorf("%s: missing from the Go table", row.Action)
			continue
		}
		roles := append([]string(nil), entry.roles...)
		sort.Strings(roles)
		if !reflect.DeepEqual(roles, nonNil(row.Roles)) && !(len(roles) == 0 && len(row.Roles) == 0) {
			t.Errorf("%s: roles %q, Python %q", row.Action, roles, row.Roles)
		}
		if !reflect.DeepEqual(nonNil(entry.requires), nonNil(row.Requires)) {
			t.Errorf("%s: requires %q, Python %q", row.Action, entry.requires, row.Requires)
		}
	}
}

func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

// TestProjectionMatchesPython replays, per action, every subset of the rule's
// roles times every subset of its predicates: all the fields it reads.
func TestProjectionMatchesPython(t *testing.T) {
	g := load(t)
	m := &mismatches{t: t}
	cases := 0
	for _, row := range g.Table {
		for roleMask := range 1 << len(row.Roles) {
			for provenMask := range 1 << len(row.Requires) {
				cases++
				facts := validFacts(subset(row.Roles, roleMask), subset(row.Requires, provenMask))
				want := projected(row.Reasons[roleMask][provenMask])
				if got := describeReason(DenialReason(row.Action, facts)); got != want {
					m.errorf("DenialReason(%q, roles=%q proven=%q) = %s, Python %s",
						row.Action, facts.Roles, facts.Proven, got, want)
				}
				wantAllowed := strconv.FormatBool(row.Reasons[roleMask][provenMask] == nil)
				if got := describeCan(Can(row.Action, facts)); got != wantAllowed {
					m.errorf("Can(%q, roles=%q proven=%q) = %s, Python %s",
						row.Action, facts.Roles, facts.Proven, got, wantAllowed)
				}
			}
		}
	}
	m.finish(cases)
}

// TestEveryRoleSubsetMatchesPython walks the space the render script proved
// the projection sound on: every subset of ROLES, every subset of the rule's
// predicates, and four sets of foreign predicates on top. Python asserted
// its answers equal the projection there; Go must too, so the two agree on
// every one of these cases without a million lines of JSON.
func TestEveryRoleSubsetMatchesPython(t *testing.T) {
	g := load(t)
	wantExtras := []string{"none", "other_known", "outsiders", "other_known+outsiders"}
	if !reflect.DeepEqual(g.ExhaustiveExtras, wantExtras) {
		t.Fatalf("the render script enumerates extras %q; this test walks %q", g.ExhaustiveExtras, wantExtras)
	}
	known := map[string]bool{}
	for _, name := range g.KnownPredicates {
		known[name] = true
	}
	var outsiders []string
	for _, name := range g.PredicateVocab {
		if !known[name] {
			outsiders = append(outsiders, name)
		}
	}
	m := &mismatches{t: t}
	cases := 0
	for _, row := range g.Table {
		inRequires := map[string]bool{}
		for _, name := range row.Requires {
			inRequires[name] = true
		}
		var otherKnown []string
		for _, name := range g.KnownPredicates {
			if !inRequires[name] {
				otherKnown = append(otherKnown, name)
			}
		}
		extras := [][]string{nil, otherKnown, outsiders, append(append([]string(nil), otherKnown...), outsiders...)}
		for roleMask := range 1 << len(g.Roles) {
			roles := subset(g.Roles, roleMask)
			projectedRoles := 0
			for bit, name := range row.Roles {
				for _, have := range roles {
					if have == name {
						projectedRoles |= 1 << bit
					}
				}
			}
			for provenMask := range 1 << len(row.Requires) {
				reason := row.Reasons[projectedRoles][provenMask]
				want := projected(reason)
				wantAllowed := strconv.FormatBool(reason == nil)
				for kind, extra := range extras {
					cases++
					proven := append(subset(row.Requires, provenMask), extra...)
					facts := validFacts(roles, proven)
					if got := describeReason(DenialReason(row.Action, facts)); got != want {
						m.errorf("DenialReason(%q, roles=%q, requires mask %d, extras %s) = %s, Python %s",
							row.Action, roles, provenMask, wantExtras[kind], got, want)
					}
					if got := describeCan(Can(row.Action, facts)); got != wantAllowed {
						m.errorf("Can(%q, roles=%q, requires mask %d, extras %s) = %s, Python %s",
							row.Action, roles, provenMask, wantExtras[kind], got, wantAllowed)
					}
				}
			}
		}
	}
	if cases != g.ExhaustiveCases {
		t.Errorf("walked %d cases, Python proved %d", cases, g.ExhaustiveCases)
	}
	m.finish(cases)
}

func replay(t *testing.T, g goldens, section string, cases []recorded) {
	m := &mismatches{t: t}
	for i, c := range cases {
		roles := make([]string, len(c.Roles))
		for j, index := range c.Roles {
			roles[j] = g.RoleVocab[index]
		}
		proven := make([]string, len(c.Proven))
		for j, index := range c.Proven {
			proven[j] = g.PredicateVocab[index]
		}
		label := fmt.Sprintf("%s[%d] action=%q actor=%q provenance=%q roles=%q proven=%q",
			section, i, c.Action, c.ActorID, c.Provenance, roles, proven)

		built, err := NewAuthorizationFacts(c.ActorID, roles, c.ResourceID, proven, c.Provenance)
		switch {
		case c.FactsError != nil && err == nil:
			m.errorf("%s: NewAuthorizationFacts accepted, Python raised %s", label, *c.FactsError)
		case c.FactsError != nil && codeOf(err) != *c.FactsError:
			m.errorf("%s: NewAuthorizationFacts error %s, Python %s", label, codeOf(err), *c.FactsError)
		case c.FactsError != nil && !reflect.DeepEqual(built, AuthorizationFacts{}):
			m.errorf("%s: NewAuthorizationFacts returned facts with its error", label)
		case c.FactsError == nil && err != nil:
			m.errorf("%s: NewAuthorizationFacts error %s, Python accepted", label, codeOf(err))
		}

		// The struct literal must answer the same, and so must the same sets
		// in another order and with duplicates: they are frozensets in Python.
		literal := AuthorizationFacts{ActorID: c.ActorID, Roles: roles, ResourceID: c.ResourceID, Proven: proven, Provenance: c.Provenance}
		shuffled := literal
		shuffled.Roles = reversedTwice(roles)
		shuffled.Proven = reversedTwice(proven)
		variants := []struct {
			name  string
			facts AuthorizationFacts
		}{{"literal", literal}, {"reordered+duplicated", shuffled}}
		if c.FactsError == nil {
			variants = append(variants, struct {
				name  string
				facts AuthorizationFacts
			}{"constructed", built})
		}
		for _, v := range variants {
			if got, want := describeReason(DenialReason(c.Action, v.facts)), wantReason(c); got != want {
				m.errorf("%s (%s): DenialReason = %s, Python %s", label, v.name, got, want)
			}
			if got, want := describeCan(Can(c.Action, v.facts)), wantCan(c); got != want {
				m.errorf("%s (%s): Can = %s, Python %s", label, v.name, got, want)
			}
		}
	}
	m.finish(len(cases))
}

// reversedTwice is items reversed, followed by items again.
func reversedTwice(items []string) []string {
	out := make([]string, 0, 2*len(items))
	for i := len(items) - 1; i >= 0; i-- {
		out = append(out, items[i])
	}
	return append(out, items...)
}

func TestMalformedFactsAndUnknownActionsMatchPython(t *testing.T) {
	g := load(t)
	replay(t, g, "malformed", g.Malformed)
}

func TestRandomFullProductSampleMatchesPython(t *testing.T) {
	g := load(t)
	replay(t, g, "sample", g.Sample)
}

// TestZeroFactsAreAnonymous: the Go zero value is the Python call with every
// argument empty, and Python refuses that as an anonymous actor first.
func TestZeroFactsAreAnonymous(t *testing.T) {
	g := load(t)
	found := false
	for _, c := range g.Malformed {
		if c.ActorID == "" && c.Provenance == "" && len(c.Roles) == 0 && c.Action == "publsh_batch" {
			found = true
			if wantReason(c) != "error "+CodeAnonymousActor {
				t.Errorf("Python answered %s for empty facts and an unknown action", wantReason(c))
			}
		}
	}
	if !found {
		t.Fatal("goldens lost the empty-facts case")
	}
	if got := describeReason(DenialReason("publsh_batch", AuthorizationFacts{})); got != "error "+CodeAnonymousActor {
		t.Errorf("DenialReason on zero facts = %s", got)
	}
}

// TestUntypedFactsArePythonOnly pins the package comment to Python: a plain
// dict is UNTYPED_FACTS, but an unknown action is reported before it. Go
// cannot express the call at all.
func TestUntypedFactsArePythonOnly(t *testing.T) {
	g := load(t)
	for _, row := range g.Untyped {
		want := "UNTYPED_FACTS"
		if _, known := table[row.Action]; !known {
			want = CodeUnknownAction
		}
		if row.Error == nil || *row.Error != want {
			t.Errorf("denial_reason(%q, %s facts): Python raised %v, package comment says %s", row.Action, row.Facts, row.Error, want)
		}
	}
}
