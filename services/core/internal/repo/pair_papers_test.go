package repo

import (
	"errors"
	"fmt"
	"testing"
)

// The Python answers below are what `dict(json.loads(raw) or {})` and
// `json.loads(a) == json.loads(b)` give in CPython 3.12; the postgres oracle
// checks the same shapes against the real repository.

func TestPythonDictOrEmpty(t *testing.T) {
	for _, c := range []struct {
		raw, want string
		err       error
	}{
		{`null`, `{}`, nil}, {`false`, `{}`, nil}, {`0`, `{}`, nil}, {`-0.0`, `{}`, nil}, {`1e-400`, `{}`, nil},
		{`""`, `{}`, nil}, {`[]`, `{}`, nil}, {`{}`, `{}`, nil},
		{`{"b": 1, "a": [2]}`, `{"b": 1, "a": [2]}`, nil},
		{`[["ngay", "x"], "xy", {"a": 1, "b": 2}, ["ngay", "y"]]`, `{"ngay": "y", "x": "y", "a": "b"}`, nil},
		{`["ơ🙂"]`, `{"ơ": "🙂"}`, nil},
		{`true`, ``, ErrPythonTypeError}, {`7`, ``, ErrPythonTypeError}, {`"ab"`, ``, ErrPythonValueError},
		{`[5]`, ``, ErrPythonTypeError}, {`[null]`, ``, ErrPythonTypeError}, {`["abc"]`, ``, ErrPythonValueError},
		{`[["a", 1, 2]]`, ``, ErrPythonValueError}, {`[{"a": 1}]`, ``, ErrPythonValueError},
		{`[[["k"], 1]]`, ``, ErrPythonTypeError}, {`[[1, 2]]`, ``, ErrUnrepresentable},
		{`[["a", 1], 5]`, ``, ErrPythonTypeError},
	} {
		got, err := pythonDictOrEmpty([]byte(c.raw))
		if !errors.Is(err, c.err) || (c.err == nil && !pythonJSONEqual(got, []byte(c.want))) {
			t.Errorf("dict(%s or {}) = %s, %v; want %s, %v", c.raw, got, err, c.want, c.err)
		}
		if c.err == nil && string(got) != c.want && c.raw != `-0.0` && c.raw != `1e-400` && c.raw != `0` {
			// Key order is part of the answer, not only the value.
			if keys, _ := jsonObjectKeys(got); len(keys) > 0 {
				wantKeys, _ := jsonObjectKeys([]byte(c.want))
				for i := range keys {
					if i >= len(wantKeys) || keys[i] != wantKeys[i] {
						t.Errorf("dict(%s or {}) keys %v, want %v", c.raw, keys, wantKeys)
						break
					}
				}
			}
		}
	}
}

func TestPythonJSONEqual(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{`{"a": 1, "b": [true, null]}`, `{"b": [1, null], "a": 1.0}`, true},
		{`{"a": false}`, `{"a": 0}`, true},
		{`{"a": 1}`, `{"a": "1"}`, false},
		{fmt.Sprintf(`{"a": %d}`, 1<<53+1), fmt.Sprintf(`{"a": %d.0}`, 1<<53), false},
		{`{"a": 0.1}`, `{"a": 0.1}`, true},
		{`[1, 2]`, `[2, 1]`, false},
		{`{"a": 1}`, `{"a": 1, "b": null}`, false},
		{`null`, `{}`, false},
	} {
		if got := pythonJSONEqual([]byte(c.a), []byte(c.b)); got != c.want {
			t.Errorf("%s == %s: %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
