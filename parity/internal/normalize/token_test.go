package normalize

import (
	"strings"
	"testing"
)

// tokenOf builds a 43-character base64url token from a seed letter, at run
// time so no token-shaped literal sits in the repository.
func tokenOf(seed string) string {
	return (strings.Repeat(seed+"b3_-", 11))[:43]
}

// A guest link token is random per stack; where it is reused and its length
// must still show.
func TestTokensAreBoundByFirstAppearanceNotByValue(t *testing.T) {
	apply := func(texts ...string) []string {
		b := NewBinder()
		for _, text := range texts {
			if err := b.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
		out := make([]string, len(texts))
		for i, text := range texts {
			out[i] = b.Apply(text)
		}
		return out
	}
	links := func(first, second string) []string {
		return []string{
			`{"guest_links":[{"path":"/g/` + first + `"},{"path":"/g/` + second + `"}]}`,
			`{"opened":"/g/` + first + `"}`,
		}
	}
	ref := apply(links(tokenOf("A"), tokenOf("C"))...)
	cand := apply(links(tokenOf("D"), tokenOf("E"))...)
	if strings.Join(ref, "\n") != strings.Join(cand, "\n") {
		t.Fatalf("same token pattern differs:\n%v\n%v", ref, cand)
	}
	if ref[1] != `{"opened":"/g/<token43#1>"}` {
		t.Fatalf("not bound: %v", ref)
	}
	if reused := apply(links(tokenOf("D"), tokenOf("D"))...); reused[0] == ref[0] {
		t.Fatalf("token reuse hidden: %v", reused)
	}
	for _, literal := range []string{tokenOf("A")[:42], tokenOf("A") + "x", tokenOf("A") + "="} {
		if got := apply(`{"path":"/g/` + literal + `"}`)[0]; strings.Contains(got, "<token43#") && !strings.HasSuffix(literal, "=") {
			t.Fatalf("%d-character run bound: %s", len(literal), got)
		}
	}
	if got := apply(`{"path":"/g/` + tokenOf("A") + `="}`)[0]; got != `{"path":"/g/<token43#1>="}` {
		t.Fatalf("padded token: %s", got)
	}
	if got := Mask(`{"path":"/g/` + tokenOf("A") + `"}`); got != `{"path":"/g/<token43>"}` {
		t.Fatalf("Mask = %s", got)
	}
}
