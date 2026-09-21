//go:build oracle

package allocator

// The full differential fuzz of the allocator, drawn at test time inside the
// parity API image by scripts/render_domain_w4_goldens.py --live and replayed
// here case by case. Run from services/core (see internal/oracletest/live.go
// for W4_ORACLE_IMAGE, W4_ORACLE_SEED, W4_ORACLE_COUNT and W4_ORACLE_CACHE):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/allocator/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveAllocateMatchesPython(t *testing.T) {
	count := oracletest.LiveCount(t, 21000)
	file := oracletest.Live(t, "allocator", 21000)
	checkAllocate(t, []oracletest.File{file}, "allocator", count)
}

func TestLiveApportionMatchesPython(t *testing.T) {
	count := oracletest.LiveCount(t, 3000)
	file := oracletest.Live(t, "apportion", 3000)
	checkApportion(t, []oracletest.File{file}, "apportion", count)
}
