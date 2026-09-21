//go:build oracle

package authsteps

// The full differential fuzz of the W9 service methods, drawn at test time by
// running the real ApiService session and OTP methods over the recording stub
// inside the parity API image (scripts/render_domain_w9_goldens.py --live).
// Run from services/core (see internal/oracletest/live.go for the
// environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/authsteps/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveAuthStepsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_auth_steps.json"), "auth_steps")
	h := newHarness(t, constants)
	file := oracletest.LiveScript(t, "scripts/render_domain_w9_goldens.py", "auth_steps", 6000)
	checkSteps(t, h, []oracletest.File{file}, false)
}
