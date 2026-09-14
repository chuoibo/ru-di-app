// Package ownership holds the route manifest: the single answer to "which
// process serves this route, and what evidence says it may". The JSON file is
// generated from services/api by scripts/render_route_manifest.py and embedded
// so a binary can never run against a manifest it was not built with.
package ownership

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

//go:embed routes.json
var embedded []byte

// Owners, Python states, classes and migration states the manifest may hold.
const (
	OwnerPython = "python"
	OwnerGo     = "go"

	PythonLive   = "live"
	PythonFrozen = "frozen"
)

var (
	classes = set("core", "ai", "mixed", "framework")
	kinds   = set("route", "mount")
	states  = set("PY", "CARDED", "PORTED", "PARITY-LOCAL", "AGY-PASS", "RERUN-PASS",
		"LIVE-GO", "FROZEN", "PY-DELETED", "DEFERRED")
	goServedStates     = set("LIVE-GO", "FROZEN", "PY-DELETED")
	pythonFrozenStates = set("FROZEN", "PY-DELETED")
)

// Route is one manifest row.
type Route struct {
	ID       string   `json:"id"`
	Order    int      `json:"order"`
	Kind     string   `json:"kind"`
	Method   string   `json:"method"`
	Path     string   `json:"path"`
	Group    string   `json:"group"`
	Class    string   `json:"class"`
	Owner    string   `json:"owner"`
	Python   string   `json:"python"`
	InMemory []string `json:"in_memory,omitempty"`
	State    string   `json:"state"`
	Evidence string   `json:"evidence,omitempty"`
}

// Manifest is the parsed, validated file.
type Manifest struct {
	Schema int     `json:"schema"`
	Routes []Route `json:"routes"`
}

// Load parses the embedded manifest.
func Load() (*Manifest, error) {
	return Parse(embedded)
}

// Parse decodes and validates a manifest, rejecting unknown fields.
func Parse(data []byte) (*Manifest, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var m Manifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("route manifest: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("route manifest: %w", err)
	}
	return &m, nil
}

func (m *Manifest) validate() error {
	if m.Schema != 1 {
		return fmt.Errorf("schema = %d, want 1", m.Schema)
	}
	if len(m.Routes) == 0 {
		return errors.New("no routes")
	}
	ids := map[string]bool{}
	limiterOwner := map[string]string{}
	limiterFirst := map[string]string{}
	for index, r := range m.Routes {
		where := fmt.Sprintf("routes[%d] %q", index, r.ID)
		if r.Order != index {
			return fmt.Errorf("%s: order = %d, want %d (rows follow registration order)", where, r.Order, index)
		}
		if ids[r.ID] {
			return fmt.Errorf("%s: duplicate id", where)
		}
		ids[r.ID] = true
		if !kinds[r.Kind] || !classes[r.Class] || !states[r.State] {
			return fmt.Errorf("%s: unknown kind/class/state %q/%q/%q", where, r.Kind, r.Class, r.State)
		}
		if want := r.Method + " " + r.Path; r.Kind == "route" && r.ID != want {
			return fmt.Errorf("%s: id must be %q", where, want)
		}
		if r.Owner != OwnerPython && r.Owner != OwnerGo {
			return fmt.Errorf("%s: owner = %q", where, r.Owner)
		}
		if r.Python != PythonLive && r.Python != PythonFrozen {
			return fmt.Errorf("%s: python = %q", where, r.Python)
		}
		if (r.Owner == OwnerGo) != goServedStates[r.State] {
			return fmt.Errorf("%s: owner %q does not match state %q", where, r.Owner, r.State)
		}
		if (r.Python == PythonFrozen) != pythonFrozenStates[r.State] {
			return fmt.Errorf("%s: python %q does not match state %q", where, r.Python, r.State)
		}
		if r.Owner == OwnerGo && r.Evidence == "" {
			return fmt.Errorf("%s: Go-owned route without evidence", where)
		}
		if r.Owner == OwnerGo && (r.Kind != "route" || r.Class == "framework") {
			return fmt.Errorf("%s: only API routes can move to Go", where)
		}
		// Routes that share an in-memory limiter or cache must be served by
		// the same process, or each process counts separately.
		for _, name := range r.InMemory {
			if owner, seen := limiterOwner[name]; seen && owner != r.Owner {
				return fmt.Errorf("%s: shares in-memory %q with %q but has a different owner",
					where, name, limiterFirst[name])
			}
			limiterOwner[name] = r.Owner
			limiterFirst[name] = r.ID
		}
	}
	return nil
}

// Force is a parsed MOBILE_FORCE_PYTHON value.
type Force struct {
	All    bool
	Routes map[string]bool
	Tokens []string
}

// ParseForce validates MOBILE_FORCE_PYTHON against the manifest. Tokens are
// comma-separated: "all", a group name, or "METHOD /path". Unknown tokens and
// frozen routes are refused so a typo can never silently keep Go serving.
func (m *Manifest) ParseForce(raw string) (Force, error) {
	force := Force{Routes: map[string]bool{}}
	byID := map[string]Route{}
	byGroup := map[string][]Route{}
	for _, r := range m.Routes {
		byID[r.ID] = r
		byGroup[r.Group] = append(byGroup[r.Group], r)
	}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		force.Tokens = append(force.Tokens, token)
		var matched []Route
		switch {
		case token == "all":
			force.All = true
			matched = m.Routes
		case byID[token].ID != "":
			matched = []Route{byID[token]}
		case len(byGroup[token]) > 0:
			matched = byGroup[token]
		default:
			return Force{}, fmt.Errorf("MOBILE_FORCE_PYTHON: unknown token %q", token)
		}
		for _, r := range matched {
			if r.Python == PythonFrozen {
				return Force{}, fmt.Errorf("MOBILE_FORCE_PYTHON: %q is frozen in Python and cannot be forced back", r.ID)
			}
			force.Routes[r.ID] = true
		}
	}
	return force, nil
}

// GoServed lists, in registration order, the routes Go answers after force.
func (m *Manifest) GoServed(force Force) []Route {
	var served []Route
	for _, r := range m.Routes {
		if r.Owner == OwnerGo && !force.All && !force.Routes[r.ID] {
			served = append(served, r)
		}
	}
	return served
}

// Groups returns the sorted group names, for error messages and tooling.
func (m *Manifest) Groups() []string {
	seen := map[string]bool{}
	for _, r := range m.Routes {
		seen[r.Group] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func set(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}
