package aieval

import (
	"strings"
	"testing"
)

// A run through agy-proxy says so on its scoreboard, in words, not as the
// bare label: the numbers came from the door production uses.
func TestBangDiemGoiTenNguonAgy(t *testing.T) {
	if got := moTaNguon(Manifest{Nguon: NguonAgy}); !strings.Contains(got, "agy-proxy") || got == NguonAgy {
		t.Fatalf("nguồn agy: %q", got)
	}
}
