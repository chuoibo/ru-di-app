//go:build oracle

package ledger

// The full differential fuzz of confirmed_total, obligation_status and
// settlement_suggestions, drawn at test time inside the parity API image by
// scripts/render_domain_w4_goldens.py --live. Run from services/core (see
// internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/ledger/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveObligationStatusMatchesPython(t *testing.T) {
	least := oracletest.LiveCount(t, 2900) * 9 / 10
	// A nil balance is an edge case only; the fuzz draws none.
	codes := []string{CodeNegativeAmount, CodeNonPositiveConfirmation, CodeNonPositiveObligation, CodeBalancesDoNotNetToZero}
	checkStatus(t, []oracletest.File{oracletest.Live(t, "ledger_status", 2900)}, "ledger_status", least, codes)
}
