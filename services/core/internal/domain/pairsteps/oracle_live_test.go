//go:build oracle

package pairsteps

// The full differential fuzz of the W8 service methods, drawn at test time by
// running the real ApiService pair methods over the recording stub inside the
// parity API image (scripts/render_domain_w8_goldens.py --live). Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/pairsteps/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLivePairStepsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_pair_steps.json"), "pair_steps")
	h := newHarness(t, constants)
	file := oracletest.LiveScript(t, "scripts/render_domain_w8_goldens.py", "pair_steps", 6000)
	checkSteps(t, h, []oracletest.File{file}, oracletest.LiveCount(t, 6000), false)
}
