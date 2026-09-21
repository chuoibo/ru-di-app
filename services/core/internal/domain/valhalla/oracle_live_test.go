//go:build oracle

package valhalla

// The full differential fuzz of the pure half of app/journey/routing.py, drawn
// at test time inside the parity API image
// (scripts/render_domain_w7_goldens.py --live). Run from services/core:
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/valhalla/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveValhallaMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w7_goldens.py", "valhalla", 20000)
	checkValhalla(t, []oracletest.File{file}, false)
}
