//go:build oracle

package capability

// The full differential fuzz of capability_scope, drawn at test time inside
// the parity API image by scripts/render_domain_w4_goldens.py --live. Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/capability/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveCapabilityScopeMatchesPython(t *testing.T) {
	checkScope(t, []oracletest.File{oracletest.Live(t, "capability", 1500)}, "capability", oracletest.LiveCount(t, 1500))
}
