package dong

import (
	"errors"
	"testing"
)

type mau string

func TestParseTuChoiGiaTriLa(t *testing.T) {
	tap := Moi[mau]("mau", "a", "b_c")
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"a", true}, {"b_c", true},
		{"", false}, {"A", false}, {" a", false}, {"a ", false}, {"b-c", false}, {"d", false},
	} {
		v, err := tap.Parse(tc.in)
		if tc.ok && (err != nil || string(v) != tc.in) {
			t.Errorf("Parse(%q) = %q, %v", tc.in, v, err)
		}
		if !tc.ok && (err == nil || !errors.Is(err, ErrLa) || v != "") {
			t.Errorf("Parse(%q) accepted: %q, %v", tc.in, v, err)
		}
	}
	if _, err := tap.ParseAll([]string{"a", "x"}); !errors.Is(err, ErrLa) {
		t.Error("ParseAll kept a list with an unknown value")
	}
	if _, err := tap.ParseAll([]string{"a", "a"}); !errors.Is(err, ErrLa) {
		t.Error("ParseAll kept a duplicate")
	}
	if got, err := tap.ParseAll(nil); err != nil || len(got) != 0 {
		t.Errorf("ParseAll(nil) = %v, %v", got, err)
	}
	if v := tap.Values(); len(v) != 2 || v[0] != "a" || v[1] != "b_c" {
		t.Errorf("Values = %v", v)
	}
}

func TestMoiTuChoiTapHong(t *testing.T) {
	for name, f := range map[string]func(){
		"empty": func() { Moi[mau]("x") },
		"blank": func() { Moi[mau]("x", "") },
		"dup":   func() { Moi[mau]("x", "a", "a") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			f()
		}()
	}
}
