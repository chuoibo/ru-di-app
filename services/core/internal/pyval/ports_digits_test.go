package pyval

import (
	"strings"
	"testing"

	"mobile/services/core/internal/pyjson"
)

// The candidate query goes through int(), which refuses a decimal string of
// more than 4300 digits, leading zeros included. Measured on the pinned image:
// 422 value_error with this message for 4301 nines, 4301 zeros and 5000 nines.
func TestCandidateMoneyRefusesWhatIntRefuses(t *testing.T) {
	for _, s := range []string{strings.Repeat("9", 4301), strings.Repeat("0", 4301), "1" + strings.Repeat("0", 4300), strings.Repeat("9", 5000)} {
		_, err := parseCandidateMoney(nil, pyjson.String(s))
		refusal, ok := err.(*Error)
		if !ok {
			t.Fatalf("%d digits: got %v, want a ValueError", len(s), err)
		}
		want := "Value error, Exceeds the limit (4300 digits) for integer string conversion: value has " +
			map[int]string{4301: "4301", 5000: "5000"}[len(s)] + " digits; use sys.set_int_max_str_digits() to increase the limit"
		if refusal.Type != "value_error" || refusal.Msg != want {
			t.Errorf("%d digits: got %s %q", len(s), refusal.Type, refusal.Msg)
		}
	}
	got, err := parseCandidateMoney(nil, pyjson.String(strings.Repeat("9", 4300)))
	if err != nil {
		t.Fatalf("4300 digits: %v", err)
	}
	if n, ok := got.(pyjson.Int); !ok || len(n.Big().String()) != 4300 {
		t.Errorf("4300 digits: got %T", got)
	}
}
