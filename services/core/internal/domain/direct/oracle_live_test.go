//go:build oracle

package direct

// The full differential fuzz of the app.domain.direct functions the W10
// direct-message route reaches, drawn at test time inside the parity API image
// (scripts/render_domain_w10_goldens.py --live). Run from services/core (see
// internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/direct/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveDirectPeopleMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w10_goldens.py", "direct_people", 20000)
	checkPeople(t, []oracletest.File{file}, oracletest.LiveCount(t, 20000))
}
