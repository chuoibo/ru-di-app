package repo

import (
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"
)

func TestWallClockDateTurnsAtVietnamsMidnight(t *testing.T) {
	cases := []struct{ instant, want string }{
		{"2030-08-27T16:59:59.999999Z", "2030-08-27"},
		{"2030-08-27T17:00:00Z", "2030-08-28"},
		{"2030-08-28T00:30:00+07:00", "2030-08-28"},
		{"2030-08-27T23:59:59-10:00", "2030-08-28"},
	}
	for _, c := range cases {
		instant, err := time.Parse(time.RFC3339Nano, c.instant)
		if err != nil {
			t.Fatal(err)
		}
		got := WallClockDate(instant)
		if got.Format("2006-01-02") != c.want || got.Location() != time.UTC || got.Hour() != 0 {
			t.Errorf("WallClockDate(%s) = %v, want %s at midnight UTC", c.instant, got, c.want)
		}
	}
}

func TestPythonInstantFloorsToMicroseconds(t *testing.T) {
	in := time.Date(2030, 8, 27, 12, 0, 0, 0, time.UTC).Add(123456*time.Microsecond + 789*time.Nanosecond)
	if got := pythonInstant(in); got.Nanosecond() != 123456*1000 {
		t.Fatalf("pythonInstant kept %d ns", got.Nanosecond())
	}
}

func TestPythonListOrEmptyFollowsPythonTruthiness(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
		err  error
	}{
		{`null`, []string{}, nil},
		{`[]`, []string{}, nil},
		{`["cà phê", "b"]`, []string{"cà phê", "b"}, nil},
		{`""`, []string{}, nil},
		{`"ẩm"`, []string{"ẩ", "m"}, nil},
		{`{}`, []string{}, nil},
		{`{"z": [1], "aa": {"x": 2}}`, []string{"z", "aa"}, nil},
		{`0`, []string{}, nil},
		{`-0.0`, []string{}, nil},
		{`1e-400`, []string{}, nil},
		{`false`, []string{}, nil},
		{`true`, nil, ErrPythonTypeError},
		{`7`, nil, ErrPythonTypeError},
		{`[1]`, nil, ErrUnrepresentable},
	}
	for _, c := range cases {
		got, err := pythonListOrEmpty([]byte(c.raw))
		if !errors.Is(err, c.err) || (c.err == nil && !reflect.DeepEqual(got, c.want)) {
			t.Errorf("%s: got %q, %v; want %q, %v", c.raw, got, err, c.want, c.err)
		}
	}
	if got, err := pythonListOrEmpty(nil); err != nil || got == nil || len(got) != 0 {
		t.Errorf("SQL NULL: %q %v", got, err)
	}
}

func TestJSONOrNoneMergesSQLNullAndJSONNull(t *testing.T) {
	if jsonOrNone(nil) != nil || jsonOrNone([]byte("null")) != nil {
		t.Fatal("NULL and null must both be None")
	}
	if string(jsonOrNone([]byte(`{"a": 1}`))) != `{"a": 1}` {
		t.Fatal("a value must keep its text")
	}
}

func TestPythonSliceEnd(t *testing.T) {
	for _, c := range []struct{ length, limit, want int }{
		{5, 3, 3}, {2, 3, 2}, {0, 0, 0}, {4, -1, 3}, {0, -1, 0}, {1, -5, 0},
	} {
		if got := pythonSliceEnd(c.length, c.limit); got != c.want {
			t.Errorf("rows[:%d] of %d = %d, want %d", c.limit, c.length, got, c.want)
		}
	}
}

func TestNewUUIDIsVersionFour(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id, err := newUUID()
		if err != nil || !pattern.MatchString(id) || seen[id] {
			t.Fatalf("newUUID() = %q, %v", id, err)
		}
		seen[id] = true
	}
}

func TestJSONArrayElementsKeepsElementText(t *testing.T) {
	got, err := jsonArrayElements([]byte(`[{"b": 1, "a": 2.0}, 3]`))
	if err != nil || len(got) != 2 || string(got[0]) != `{"b": 1, "a": 2.0}` || string(got[1]) != `3` {
		t.Fatalf("got %q %v", got, err)
	}
	if got, err := jsonArrayElements([]byte(`[]`)); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty array: %q %v", got, err)
	}
}

func TestPlaceholders(t *testing.T) {
	if got := uuidPlaceholders(4, 3); got != "$4::UUID, $5::UUID, $6::UUID" {
		t.Fatal(got)
	}
	if got := wallClockDate("$2", "m.created_at"); got != "CAST(timezone($2::VARCHAR, m.created_at) AS DATE)" {
		t.Fatal(got)
	}
}
