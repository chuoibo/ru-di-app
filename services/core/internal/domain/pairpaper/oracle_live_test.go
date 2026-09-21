//go:build oracle

package pairpaper

// The full differential fuzz of app.domain.pair_paper, drawn at test time
// inside the parity API image (scripts/render_domain_w8_goldens.py --live).
// Run from services/core (see internal/oracletest/live.go for the
// environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/pairpaper/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLivePairPaperMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w8_goldens.py", "pair_paper", 20000)
	least := oracletest.LiveCount(t, 20000)
	report := oracletest.Agree(t, []oracletest.File{file}, "pair_paper", replay, refusal)
	total := 0
	for _, tally := range report.ByFn {
		total += tally.Cases
	}
	if total < least {
		t.Fatalf("%d cases, want %d", total, least)
	}
	for _, fn := range functions {
		if report.ByFn[fn] == nil {
			t.Errorf("no Python case of %s", fn)
		}
	}
	t.Logf("refusals %v", report.Codes)
}
