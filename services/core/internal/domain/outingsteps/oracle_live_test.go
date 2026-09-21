//go:build oracle

package outingsteps

// The full differential fuzz of the W7 service methods, drawn at test time by
// running the real ApiService outing methods over the recording stub inside
// the parity API image (scripts/render_domain_w7_goldens.py --live). Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/outingsteps/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveOutingStepsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_outing_steps.json"), "outing_steps")
	h := newHarness(t, constants)
	file := oracletest.LiveScript(t, "scripts/render_domain_w7_goldens.py", "outing_steps", 6000)
	checkSteps(t, h, []oracletest.File{file}, false)
}
