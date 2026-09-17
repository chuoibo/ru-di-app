//go:build oracle

package otp

// The full differential fuzz of app/domain/otp.py, drawn at test time inside
// the parity API image (scripts/render_domain_w9_goldens.py --live). Run from
// services/core (see internal/oracletest/live.go for the environment):
//
//	go test -tags oracle -run TestLive -v -timeout 60m ./internal/domain/otp/

import (
	"testing"

	"mobile/services/core/internal/oracletest"
)

func TestLiveOtpMatchesPython(t *testing.T) {
	file := oracletest.LiveScript(t, "scripts/render_domain_w9_goldens.py", "otp", 20000)
	checkOtp(t, []oracletest.File{file}, false)
}
