//go:build postgres

package chatassist

import (
	"testing"

	"mobile/services/core/internal/aiharness/tools"
)

// TestNhomGoKhongHoiBrain: on the Go engine a group invocation is taken
// without asking the brain; a host with no usable brain (the vnlocal stack's
// api has no Gemini key) used to refuse every one provider_unavailable while
// chat-capabilities said the bot was on. On a brain host the probe still
// decides.
func TestNhomGoKhongHoiBrain(t *testing.T) {
	n := setupNhomGo(t, tools.NguonDuLieu{})
	n.f.handler.brain = nil // no brain at all
	before := n.f.capabilityCalls.Load()
	if _, code, e := n.goiCap(t, "plan"); code != 202 {
		t.Fatalf("Go engine, no brain: %d %s", code, e)
	}
	if n.f.capabilityCalls.Load() != before {
		t.Fatal("the Go engine asked the brain")
	}
	n.f.handler.nhomGo = false
	if _, code, e := n.goiCap(t, "plan"); code != 503 || e != "provider_unavailable" {
		t.Fatalf("brain host without a brain: %d %s", code, e)
	}
}
