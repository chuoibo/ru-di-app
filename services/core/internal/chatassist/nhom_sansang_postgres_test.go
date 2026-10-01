//go:build postgres

package chatassist

import (
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/tools"
)

// TestKhongMayThiTuChoiRo: a host with a model takes a group invocation; a
// host with none (a keyless stack) refuses it provider_unavailable at the
// route, and chat-capabilities says so for every command, as a keyless
// stack always has (ADR-0052: no brain to ask any more).
func TestKhongMayThiTuChoiRo(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{})
	if _, code, e := n.goiCap(t, "plan"); code != 202 {
		t.Fatalf("host with a model: %d %s", code, e)
	}
	n.f.handler.coMay, n.f.handler.nhomEngine, n.f.handler.nepEngine = false, nil, nil
	if _, code, e := n.goiCap(t, "plan"); code != 503 || e != "provider_unavailable" {
		t.Fatalf("keyless host: %d %s", code, e)
	}
	w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", n.f.token, nil)
	requireCode(t, w, 200)
	for _, lenh := range []string{"plan", "chia_bill", "hoi"} {
		if !strings.Contains(w.Body.String(), `"`+lenh+`":{"available":false,"reason":"provider_unavailable"}`) {
			t.Fatalf("keyless capabilities for %s: %s", lenh, w.Body.String())
		}
	}
}
