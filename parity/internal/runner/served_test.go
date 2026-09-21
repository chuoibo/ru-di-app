package runner

import (
	"strings"
	"testing"

	"mobile/parity/internal/scenario"
)

func TestServedRoutesNeedAStepAnsweredWithoutPython(t *testing.T) {
	sc := &scenario.Scenario{ID: "t/served", Routes: []string{"GET /areas", "GET /contexts/{context_id}"}}
	run := func(counts ...int) *Run {
		r := &Run{}
		for _, n := range counts {
			r.Steps = append(r.Steps, StepResult{PythonRequests: n})
		}
		return r
	}
	served := map[string]bool{"GET /areas": true}
	if got := RoutesNotServedInCore(sc, run(1, 0, 1), served); len(got) != 0 {
		t.Fatalf("a step answered in core was not counted: %v", got)
	}
	if got := RoutesNotServedInCore(sc, run(1, 1), served); strings.Join(got, ",") != "GET /areas" {
		t.Fatalf("every step reached Python, yet: %v", got)
	}
	if got := RoutesNotServedInCore(sc, run(1, 1), map[string]bool{}); len(got) != 0 {
		t.Fatalf("no served route, yet: %v", got)
	}
	if got := RoutesNotServedInCore(sc, run(-1, -1), served); strings.Join(got, ",") != "GET /areas" {
		t.Fatalf("a run without a tap proved nothing, yet: %v", got)
	}
}
