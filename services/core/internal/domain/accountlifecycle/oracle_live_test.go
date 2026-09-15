//go:build oracle

package accountlifecycle

// The full differential fuzz of app.domain.account_lifecycle, drawn at test
// time inside the parity API image (scripts/render_domain_w10_goldens.py
// --live). Run from services/core (see internal/oracletest/live.go for the
// environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/accountlifecycle/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveAccountLifecycleMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w10_goldens.py", "account_lifecycle", 20000)
	check(t, []oracletest.File{file}, oracletest.LiveCount(t, 20000))
}
