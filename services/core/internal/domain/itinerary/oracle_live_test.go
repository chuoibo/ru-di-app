//go:build oracle

package itinerary

// The full differential fuzz of app/journey/preview.py, drawn at test time
// inside the parity API image (scripts/render_domain_w7_goldens.py --live).
// Run from services/core:
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/itinerary/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveItineraryMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w7_goldens.py", "itinerary", 6000)
	checkItinerary(t, []oracletest.File{file}, false)
}
