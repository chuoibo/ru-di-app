package promptsafety

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

// The oracle goldens were rendered by Python 3.12, whose unicodedata tables are
// Unicode 15.0.0. NFD folding here must use the same edition, or a character
// added in a later edition decomposes differently in the two stacks and the
// parity comparison drifts for a reason nobody changed on purpose. A dependency
// bump (the ADK graph pulls a newer x/text) must not move this silently.
func TestNormTablesMatchPythonOracle(t *testing.T) {
	if norm.Version != "15.0.0" {
		t.Fatalf("x/text norm tables are Unicode %s; the Python oracle is 15.0.0", norm.Version)
	}
}
