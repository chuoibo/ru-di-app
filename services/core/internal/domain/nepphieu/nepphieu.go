// Package nepphieu holds the one rule about Nếp's context slip that both the
// request handler (chatassist) and the Go engine (aiharness) must apply the
// same way: on a money screen Nếp steps back and answers nothing (ADR-0033
// §2.2, ADR-0036 §2.9). One list, so the two cannot drift; chatassist's
// nep_test.go holds it to MAN_NEP_LUI in the app's nep/phieu.ts.
package nepphieu

import "strings"

// ManLui is MAN_NEP_LUI of phieu.ts: the first route segment of every money
// screen.
var ManLui = []string{"finance", "settlements", "batches", "smart-split"}

// PhaiLui is `nepPhaiLui` of phieu.ts: whole first segment, never a prefix,
// so `financial-report` is not a money screen.
func PhaiLui(man string) bool {
	dau := strings.Split(strings.TrimLeft(man, "/"), "/")[0]
	for _, m := range ManLui {
		if dau == m {
			return true
		}
	}
	return false
}
