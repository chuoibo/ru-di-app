//go:build oracle

package expense

// The full differential fuzz of component_rollups, drawn at test time inside
// the parity API image by scripts/render_domain_w4_goldens.py --live. Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/expense/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveComponentRollupsMatchPython(t *testing.T) {
	checkRollups(t, []oracletest.File{oracletest.Live(t, "expense", 1500)}, "expense", oracletest.LiveCount(t, 1500))
}
