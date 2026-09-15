//go:build oracle

package peoplesteps

// The full differential fuzz of the W10 people methods, drawn at test time by
// running the real ApiService methods over the recording stubs inside the
// parity API image (scripts/render_domain_w10_goldens.py --live). Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/peoplesteps/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLivePeopleStepsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_people_steps.json"), "people_steps")
	h := newHarness(t, constants)
	file := oracletest.LiveScript(t, "scripts/render_domain_w10_goldens.py", "people_steps", 6000)
	checkSteps(t, h, []oracletest.File{file}, oracletest.LiveCount(t, 6000), false)
}
