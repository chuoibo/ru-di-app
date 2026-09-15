package chattheme

import (
	"slices"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_*.json is rendered by scripts/render_domain_w3_goldens.py
// from the real app.domain.chat_theme in the parity API image.

func TestIsThemeMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.CheckShards(t, files, "chat_theme")
	total, mismatches, accepted := 0, 0, 0
	for _, file := range files {
		for _, c := range file.Cases {
			if c.Fn != "is_theme" {
				t.Fatalf("%s %s: unknown function %q", file.Mode, c.Name, c.Fn)
			}
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatal(err)
			}
			value, ok := args["value"].(string)
			if !ok {
				t.Fatalf("%s %s: value %v is not a str", file.Mode, c.Name, args["value"])
			}
			want, raised, err := c.Outcome()
			if err != nil || raised != nil {
				t.Fatalf("%s %s: Python raised %v (%v)", file.Mode, c.Name, raised, err)
			}
			got := IsTheme(value)
			total++
			if got {
				accepted++
			}
			if want != got {
				mismatches++
				t.Errorf("%s %s is_theme(%q): Python %v, Go %v", file.Mode, c.Name, value, want, got)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	if total < 450 || accepted < 5 {
		t.Errorf("%d cases, %d accepted: the corpus lost its spread", total, accepted)
	}
	t.Logf("is_theme: %d Python cases agree, 0 mismatches; %d accepted", total, accepted)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "chat_theme")
	themes, err := oracletest.Strings(constants["themes"])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(themes, Themes()) {
		t.Errorf("THEMES: Python %v, Go %v", themes, Themes())
	}
	if constants["default_theme"] != DefaultTheme {
		t.Errorf("DEFAULT_THEME: Python %v, Go %q", constants["default_theme"], DefaultTheme)
	}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"DEFAULT_THEME", "THEMES", "is_theme"}; !slices.Equal(names, want) {
		t.Errorf("Python public names %v; the port maps %v", names, want)
	}
}
