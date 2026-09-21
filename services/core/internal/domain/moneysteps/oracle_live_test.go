//go:build oracle

package moneysteps

// The full differential fuzz of the W4 service steps, drawn at test time by
// running the real ApiService methods over the recording stub inside the
// parity API image (scripts/render_domain_w4_goldens.py --live). Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/moneysteps/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveServiceStepsMatchPython(t *testing.T) {
	least := oracletest.LiveCount(t, 7900) * 9 / 10
	// person_finance and not_your_finances come from the edge cases only.
	fns := stepFunctions[:len(stepFunctions)-1]
	codes := stepCodes[:len(stepCodes)-1]
	checkSteps(t, []oracletest.File{oracletest.Live(t, "money_steps", 7900)}, "money_steps", least, fns, codes)
}
