//go:build oracle

package budget

// The full differential fuzz of build_group_budget, drawn at test time inside
// the parity API image by scripts/render_domain_w4_goldens.py --live. Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/budget/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveBuildGroupBudgetMatchesPython(t *testing.T) {
	checkBudget(t, []oracletest.File{oracletest.Live(t, "budget", 2500)}, "budget", oracletest.LiveCount(t, 2500))
}
