//go:build oracle

package journey

// The full differential fuzz of app/domain/journey.py, drawn at test time
// inside the parity API image (scripts/render_domain_w7_goldens.py --live).
// Run from services/core (see internal/oracletest/live.go for the
// environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/journey/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveJourneyMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w7_goldens.py", "journey", 20000)
	checkJourney(t, []oracletest.File{file}, false)
}
