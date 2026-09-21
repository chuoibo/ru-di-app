package ownership

import (
	"fmt"
	"strings"
)

// EnvCandidateRoutes names routes Go serves although the manifest still gives
// them to Python.
const EnvCandidateRoutes = "MOBILE_CORE_CANDIDATE_ROUTES"

// CandidatesPorted selects every route whose Go code is merged but not live.
const CandidatesPorted = "ported"

// candidateStates are the states between merging a route's Go code and giving
// the route to Go (ADR-0029 §2.3): the code exists, the evidence does not yet.
var candidateStates = set("PORTED-UNPROVEN", "PORTED", "PARITY-LOCAL", "AGY-PASS", "RERUN-PASS")

// ParseCandidates reads MOBILE_CORE_CANDIDATE_ROUTES: route ids, group names or
// "ported", comma-separated. A candidate is served by Go while Python still
// owns it, which is how a parity run compares Go code with the Python it is to
// replace before anyone flips the owner. The rules keep that from becoming a
// back door:
//
//   - only a route in a candidate state can be one; naming any other refuses
//     to start, and a group or "ported" selects only its candidate-state rows;
//   - MOBILE_FORCE_PYTHON still wins;
//   - a candidate may not split in-memory state (a limiter, a cache) with a
//     route Python would keep serving, or each process would count alone.
//
// Routes the manifest already gives Go are not returned: GoServed has them.
func (m *Manifest) ParseCandidates(raw string, force Force) ([]Route, error) {
	byID := map[string]Route{}
	byGroup := map[string][]Route{}
	for _, r := range m.Routes {
		byID[r.ID] = r
		byGroup[r.Group] = append(byGroup[r.Group], r)
	}
	chosen := map[string]bool{}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		switch {
		case token == "":
		case token == CandidatesPorted:
			for _, r := range m.Routes {
				if candidateStates[r.State] {
					chosen[r.ID] = true
				}
			}
		case byID[token].ID != "":
			r := byID[token]
			if !candidateStates[r.State] {
				return nil, fmt.Errorf("%s: %q is %s; only a route whose Go code is merged (PORTED-UNPROVEN to RERUN-PASS) can be a candidate",
					EnvCandidateRoutes, r.ID, r.State)
			}
			chosen[r.ID] = true
		case len(byGroup[token]) > 0:
			found := false
			for _, r := range byGroup[token] {
				if candidateStates[r.State] {
					chosen[r.ID] = true
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("%s: group %q has no ported route", EnvCandidateRoutes, token)
			}
		default:
			return nil, fmt.Errorf("%s: unknown token %q", EnvCandidateRoutes, token)
		}
	}

	var candidates []Route
	inGo := map[string]bool{}
	for _, r := range m.GoServed(force) {
		inGo[r.ID] = true
	}
	for _, r := range m.Routes {
		if chosen[r.ID] && r.Owner != OwnerGo && !force.All && !force.Routes[r.ID] {
			candidates = append(candidates, r)
			inGo[r.ID] = true
		}
	}
	holders := map[string][]Route{}
	for _, r := range m.Routes {
		for _, name := range r.InMemory {
			holders[name] = append(holders[name], r)
		}
	}
	for _, r := range candidates {
		for _, name := range r.InMemory {
			for _, other := range holders[name] {
				if !inGo[other.ID] {
					return nil, fmt.Errorf("%s: %q shares in-memory %q with %q, which Python would still serve",
						EnvCandidateRoutes, r.ID, name, other.ID)
				}
			}
		}
	}
	return candidates, nil
}
