package chatlegacychange

import (
	"net/http/httptest"
	"testing"
)

func TestQueryRejectsURLTokensAndCursorAmbiguity(t *testing.T) {
	// repo-guard: allow=long-number reason=synthetic-cursor-overflow
	for _, q := range []string{"token=secret", "access_token=secret", "after=-1", "after=1&after=2", "limit=101", "limit=0", "after=90000000000000000000", "after=%XX"} {
		// repo-guard: allow=long-number reason=synthetic-context-uuid
		r := httptest.NewRequest("GET", "/contexts/11111111-1111-1111-1111-111111111111/changes?"+q, nil)
		// repo-guard: allow=long-number reason=synthetic-context-uuid
		r.SetPathValue("room", "11111111-1111-1111-1111-111111111111")
		if _, _, err := query(r); err == nil {
			t.Fatalf("accepted %q", q)
		}
	}
}
func TestWebsocketOriginsAreExplicit(t *testing.T) {
	h := &Handler{Origins: []string{"https://chat.example"}}
	for _, tc := range []struct {
		origin string
		want   bool
	}{{"", true}, {"https://chat.example", true}, {"https://evil.example", false}, {"https://chat.example.evil.example", false}, {"https://chat.example/path", false}, {"null", false}} {
		r := httptest.NewRequest("GET", "https://api.example/", nil)
		r.Header.Set("Origin", tc.origin)
		if got := h.originAllowed(r); got != tc.want {
			t.Fatalf("%s: %v", tc.origin, got)
		}
	}
}
