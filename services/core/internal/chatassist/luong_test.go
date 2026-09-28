package chatassist

import (
	"crypto/sha256"
	"testing"
)

func TestLichTrinhCuaThe(t *testing.T) {
	for name, c := range map[string]struct {
		card  string
		stops int
		ok    bool
	}{
		"itinerary":            {`{"kind":"itinerary","payload":{"stops":[{"time_text":"19:00","place":{"id":"p","name":"P"}}]}}`, 1, true},
		"reply with a plan":    {`{"kind":"tra_loi","payload":{"phan":[{"kind":"text","payload":{"text":"a"}},{"kind":"itinerary","payload":{"stops":[{"time_text":"19:00","place":{"id":"p","name":"P"}},{"time_text":"21:00","place":{"id":"q","name":"Q"}}]}}]}}`, 2, true},
		"reply without a plan": {`{"kind":"tra_loi","payload":{"phan":[{"kind":"text","payload":{"text":"a"}}]}}`, 0, false},
		"empty itinerary":      {`{"kind":"itinerary","payload":{"stops":[]}}`, 0, false},
		"poll":                 {`{"kind":"poll","payload":{"stops":[{"time_text":"19:00"}]}}`, 0, false},
		"not json":             {`{`, 0, false},
	} {
		stops, ok := lichTrinhCuaThe([]byte(c.card))
		if ok != c.ok || len(stops) != c.stops {
			t.Errorf("%s: ok=%v stops=%d", name, ok, len(stops))
		}
	}
}

// The digest is what separates a replay from a conflict. Without a trigger it
// must be byte for byte what it was before triggers existed, or a retry that
// straddles the deploy would 409 instead of replaying.
func TestInputDigestGiuNguyenKhiKhongCoTinTag(t *testing.T) {
	old := sha256.Sum256(append([]byte("plan\x00đi đâu\x00"), []byte(`{"ban":1}`)...))
	if inputDigest("plan", "đi đâu", []byte(`{"ban":1}`), "") != old {
		t.Fatal("digest without a trigger changed")
	}
	a := inputDigest("plan", "đi đâu", nil, "0b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d")
	b := inputDigest("plan", "đi đâu", nil, "1b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d")
	if a == b || a == inputDigest("plan", "đi đâu", nil, "") {
		t.Fatal("the trigger is not part of the digest")
	}
}
