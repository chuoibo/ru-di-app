package diary

import (
	"testing"
	"time"
)

func TestOutingStartedUsesVietnamCalendar(t *testing.T) {
	for _, tc := range []struct {
		instant, start string
		want           bool
	}{
		{"2026-10-01T16:59:59Z", "2026-10-02", false},
		{"2026-10-01T17:00:00Z", "2026-10-02", true},
		{"2026-10-02T16:59:59Z", "2026-10-03", false},
		{"2026-10-02T17:00:00Z", "2026-10-03", true},
		{"2026-10-02T00:00:00Z", "2026-10-01", true},
	} {
		now, err := time.Parse(time.RFC3339, tc.instant)
		if err != nil {
			t.Fatal(err)
		}
		if got := outingStarted(tc.start, now); got != tc.want {
			t.Fatalf("%s at %s: got %t, want %t", tc.start, tc.instant, got, tc.want)
		}
	}
}
