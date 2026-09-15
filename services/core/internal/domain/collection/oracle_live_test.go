//go:build oracle

package collection

// The full differential fuzz of the collection state machine, drawn at test
// time inside the parity API image by scripts/render_domain_w4_goldens.py
// --live. Run from services/core (see internal/oracletest/live.go for the
// environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/collection/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveCollectionMatchesPython(t *testing.T) {
	// Parts of a multi-part fuzz are floored, so a run may draw a few cases
	// fewer than asked.
	least := oracletest.LiveCount(t, 3800) * 9 / 10
	checkCollection(t, []oracletest.File{oracletest.Live(t, "collection", 3800)}, "collection", least, collectionFunctions[:4])
}
