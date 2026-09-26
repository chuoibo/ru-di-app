package runner

import (
	"strings"
	"testing"

	"mobile/parity/internal/scenario"
)

// step is one scenario step and its outcome, paired so a case reads as
// "this request, answered there".
type step struct {
	id     string
	method string
	path   string
	python int // 0: answered in core · >0: reached Python · -1: no tap
}

func fixture(routes []string, steps ...step) (*scenario.Scenario, *Run) {
	sc := &scenario.Scenario{ID: "t/served", Routes: routes}
	run := &Run{}
	for _, s := range steps {
		sc.Steps = append(sc.Steps, scenario.Step{
			ID:      s.id,
			Request: scenario.Request{Method: s.method, Path: s.path},
		})
		run.Steps = append(run.Steps, StepResult{StepID: s.id, PythonRequests: s.python})
	}
	return sc, run
}

func TestServedRoutesNeedAStepAnsweredWithoutPython(t *testing.T) {
	served := map[string]bool{"GET /areas": true}

	sc, run := fixture([]string{"GET /areas"},
		step{"other", "GET", "/interests", 1},
		step{"areas", "GET", "/areas", 0},
	)
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 0 {
		t.Errorf("a step on the route was answered in core, yet: %v", got)
	}

	sc, run = fixture([]string{"GET /areas"},
		step{"areas", "GET", "/areas", 1},
	)
	if got := RoutesNotServedInCore(sc, run, served); strings.Join(got, ",") != "GET /areas" {
		t.Errorf("the route's only step reached Python, yet: %v", got)
	}

	sc, run = fixture([]string{"GET /areas"}, step{"areas", "GET", "/areas", 0})
	if got := RoutesNotServedInCore(sc, run, map[string]bool{}); len(got) != 0 {
		t.Errorf("nothing is served, yet: %v", got)
	}

	sc, run = fixture([]string{"GET /areas"}, step{"areas", "GET", "/areas", -1})
	if got := RoutesNotServedInCore(sc, run, served); strings.Join(got, ",") != "GET /areas" {
		t.Errorf("a run without a tap proved nothing, yet: %v", got)
	}
}

// THE HOLE THIS CHECK EXISTS TO CLOSE, and the one the previous version could
// not express: a scenario that names two served routes, exercises both, and has
// only ONE of them answered in core. The old code asked "was any step in this
// file answered in core?" once for the whole file, so the route still falling
// through to Python was credited by the other route's step.
//
// That is not hypothetical. `/static` and the guest pages live in one scenario:
// the pages are Go's, the stylesheet is not, and moving `/static` to Go would
// have been credited by the pages' own steps while every stylesheet fetch kept
// reaching Python.
func TestOneRouteInCoreDoesNotCreditAnotherStillOnPython(t *testing.T) {
	served := map[string]bool{"GET /g/{token}": true, "GET /static/{path}": true}
	sc, run := fixture([]string{"GET /g/{token}", "GET /static/{path}"},
		step{"page", "GET", "/g/{{token}}", 0},
		step{"style", "GET", "/static/guest.css", 1},
	)
	if got := strings.Join(RoutesNotServedInCore(sc, run, served), ","); got != "GET /static/{path}" {
		t.Errorf("the stylesheet still reached Python; want it named, got %q", got)
	}
}

// A mount answers any method under its prefix, and no step carries the literal
// method "MOUNT". Matching it like a route would call a mounted prefix
// unserved as soon as it moved to Go -- the exact shape of red that teaches
// people to stop reading the gate.
func TestAMountMatchesAnyMethodBeneathItsPrefix(t *testing.T) {
	served := map[string]bool{"MOUNT /static": true}
	routes := []string{"MOUNT /static"}

	for _, s := range []step{
		{"missing", "GET", "/static/khong-co.css", 0},
		{"real", "GET", "/static/guest.css", 0},
		{"deep", "GET", "/static/a/b/c.js", 0},
		{"head", "HEAD", "/static/guest.css", 0},
		{"bare", "GET", "/static", 0},
	} {
		sc, run := fixture(routes, s)
		if got := RoutesNotServedInCore(sc, run, served); len(got) != 0 {
			t.Errorf("%s: a step beneath the mount should count, yet: %v", s.id, got)
		}
	}

	// A neighbour that merely starts with the same letters is not beneath it.
	sc, run := fixture(routes, step{"neighbour", "GET", "/staticky/x.css", 0})
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 1 {
		t.Errorf("/staticky is not under /static; want it named, got %v", got)
	}

	// And a step beneath the mount that reached Python still counts for nothing.
	sc, run = fixture(routes, step{"real", "GET", "/static/guest.css", 1})
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 1 {
		t.Errorf("the only step reached Python; want it named, got %v", got)
	}
}

func TestRoutePatternTreatsBothPlaceholdersAsOneSegment(t *testing.T) {
	served := map[string]bool{"GET /contexts/{context_id}/albums/{outing_id}": true}
	routes := []string{"GET /contexts/{context_id}/albums/{outing_id}"}

	sc, run := fixture(routes, step{"read", "GET", "/contexts/{{ctx}}/albums/{{trip}}", 0})
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 0 {
		t.Errorf("a bound path should match the route, yet: %v", got)
	}

	// A query string is not part of the path.
	sc, run = fixture(routes, step{"read", "GET", "/contexts/{{ctx}}/albums/{{trip}}?limit=1", 0})
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 0 {
		t.Errorf("a query string should not stop the match, yet: %v", got)
	}

	// One segment, not many: the trailing path must not be swallowed.
	sc, run = fixture(routes, step{"reel", "GET", "/contexts/{{ctx}}/albums/{{trip}}/reel", 0})
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 1 {
		t.Errorf("a longer path is a different route; want it named, got %v", got)
	}

	// The method is part of the identity.
	sc, run = fixture(routes, step{"post", "POST", "/contexts/{{ctx}}/albums/{{trip}}", 0})
	if got := RoutesNotServedInCore(sc, run, served); len(got) != 1 {
		t.Errorf("another method is another route; want it named, got %v", got)
	}
}
