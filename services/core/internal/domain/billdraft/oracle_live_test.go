//go:build oracle

package billdraft

// The full differential fuzz of allocator_input_from_bill, drawn at test time
// inside the parity API image by scripts/render_domain_w4_goldens.py --live.
// Run from services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/billdraft/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveAllocatorInputFromBillMatchesPython(t *testing.T) {
	checkBill(t, []oracletest.File{oracletest.Live(t, "billdraft", 2500)}, "billdraft", oracletest.LiveCount(t, 2500))
}
