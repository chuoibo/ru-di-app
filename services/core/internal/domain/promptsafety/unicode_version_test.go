package promptsafety

import (
	"testing"
	"unicode"

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

// The standard library's unicode tables (unicode.IsLetter and friends, used
// across the core) move with the toolchain, not with go.mod's require lines.
// Go 1.23 through 1.26 ship 15.0.0; a toolchain bump that moves them must be
// red here, not a parity drift later.
func TestStdlibUnicodeTablesMatchPythonOracle(t *testing.T) {
	if unicode.Version != "15.0.0" {
		t.Fatalf("the toolchain's unicode tables are Unicode %s; the Python oracle is 15.0.0", unicode.Version)
	}
}
