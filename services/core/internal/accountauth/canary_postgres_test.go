//go:build postgres

package accountauth

import (
	"os"
	"testing"
)

func TestPostgresAuthHarnessRejectsWrongPassword(t *testing.T) {
	h := authWorld(t)
	signup(t, h, "synthetic_canary")
	status, _ := call(t, h, "/auth/login", "POST", "", map[string]string{
		"username": "synthetic_canary", "password": "wrong isolated synthetic credential",
	})
	want := 401
	if os.Getenv("RUDI_AUTH_QA_CANARY") == "1" {
		// Deliberately wrong expectation proves the same harness can turn red.
		want = 201
	}
	if status != want {
		t.Fatalf("wrong-password control: HTTP %d, want %d", status, want)
	}
}
