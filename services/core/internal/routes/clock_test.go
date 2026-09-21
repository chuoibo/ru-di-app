package routes

import "testing"

// Expected strings are Python's f"{m // 60:02d}:{m % 60:02d}" for each m.
func TestClockFormatsMinutesAsPythonDoes(t *testing.T) {
	for _, tc := range []struct {
		minute int64
		want   string
	}{{0, "00:00"}, {1, "00:01"}, {59, "00:59"}, {60, "01:00"}, {61, "01:01"}, {599, "09:59"}, {1439, "23:59"}, {1440, "24:00"}, {-1, "-1:59"}, {-59, "-1:01"}, {-60, "-1:00"}, {-61, "-2:59"}, {100000, "1666:40"}} {
		if got := clock(tc.minute); got != tc.want {
			t.Fatalf("clock(%d) = %q, Python %q", tc.minute, got, tc.want)
		}
	}
}
