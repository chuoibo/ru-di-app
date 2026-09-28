package otelchan

import (
	"os"
	"os/exec"
	"testing"
)

const envCon = "OTELCHAN_CON"

// The init has run before any test: a child test binary started with the
// variable set sees it cleared.
func TestInitClearsContentCapture(t *testing.T) {
	if os.Getenv(envCon) == "1" {
		if v, ok := os.LookupEnv(BienNoiDung); ok {
			t.Fatalf("%s still set to %q after init", BienNoiDung, v)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInitClearsContentCapture$", "-test.count=1")
	cmd.Env = append(os.Environ(), envCon+"=1", BienNoiDung+"=SPAN_AND_EVENT")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
}

// Canary: without Chan the variable would survive.
func TestChanClears(t *testing.T) {
	t.Setenv(BienNoiDung, "SPAN_AND_EVENT")
	if os.Getenv(BienNoiDung) == "" {
		t.Fatal("t.Setenv did not set the variable; the check below proves nothing")
	}
	Chan()
	if _, ok := os.LookupEnv(BienNoiDung); ok {
		t.Fatal("Chan left the variable set")
	}
}
