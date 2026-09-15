package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// urlPathGolden is one row of testdata/starlette_url_paths.json, rendered in
// the pinned API image by scripts/render_url_path_goldens.py. Host is nil for
// a request without one; exactly one of URLPath and Raises is set.
type urlPathGolden struct {
	Host    *string `json:"host"`
	Path    string  `json:"path"`
	Query   string  `json:"query"`
	URLPath *string `json:"url_path"`
	Raises  string  `json:"raises"`
}

func TestURLPathIsStarlettesRequestURLPath(t *testing.T) {
	data, err := os.ReadFile("testdata/starlette_url_paths.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var rows []urlPathGolden
	if err := decoder.Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 3000 {
		t.Fatalf("only %d goldens", len(rows))
	}
	raised, elsewhere := 0, 0
	for i, row := range rows {
		host := ""
		if row.Host != nil {
			host = latin1Bytes(t, *row.Host)
		}
		got, err := URLPath(host, row.Path, latin1Bytes(t, row.Query))
		switch {
		case (row.URLPath == nil) == (row.Raises == ""):
			t.Fatalf("row %d is neither a path nor a raise", i)
		case row.Raises != "":
			raised++
			if !errors.Is(err, ErrURLPath) {
				t.Fatalf("row %d host %q path %q query %q: got %q, Python raises %s", i, host, row.Path, row.Query, got, row.Raises)
			}
		case err != nil || got != *row.URLPath:
			t.Fatalf("row %d host %q path %q query %q: got %q %v, Python %q", i, host, row.Path, row.Query, got, err, *row.URLPath)
		case got != row.Path:
			elsewhere++
		}
	}
	if raised == 0 || raised == len(rows) || elsewhere == 0 {
		t.Fatalf("%d raise and %d move the path, of %d: the goldens cannot tell the branches apart", raised, elsewhere, len(rows))
	}
}

// latin1Bytes turns text the render script wrote as latin-1 code points back
// into the header bytes they stand for.
func latin1Bytes(t *testing.T, s string) string {
	t.Helper()
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xff {
			t.Fatalf("%q is not latin-1", s)
		}
		out = append(out, byte(r))
	}
	return string(out)
}

// The two measurements the broken-link branch depends on.
func TestURLPathHostSpellings(t *testing.T) {
	path := "/g/" + strings.Repeat("t", 43)
	if got, err := URLPath("x/y", path, ""); err != nil || got != "/y"+path {
		t.Fatalf("Host x/y: %q %v", got, err)
	}
	if _, err := URLPath("[", path, ""); !errors.Is(err, ErrURLPath) {
		t.Fatalf("Host [: %v", err)
	}
	if got, err := URLPath("[::1%eth0]:8000", path, "a=/g"); err != nil || got != path {
		t.Fatalf("scoped IPv6 host: %q %v", got, err)
	}
}
