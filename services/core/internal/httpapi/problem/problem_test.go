package problem

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/pyjson"
)

type golden struct {
	Case    string      `json:"case"`
	Path    string      `json:"path"`
	Status  int         `json:"status"`
	Headers [][2]string `json:"headers"`
	Body    string      `json:"body"`
}

func headerMap(pairs [][2]string) map[string]string {
	out := map[string]string{}
	for _, pair := range pairs {
		name := strings.ToLower(pair[0])
		if prev, ok := out[name]; ok {
			out[name] = prev + "\x00" + pair[1]
			continue
		}
		out[name] = pair[1]
	}
	return out
}

// parseValidation reads a golden 422 body back into ValidationErrors, keeping
// pydantic's key order and integer locations through pyjson.
func parseValidation(t *testing.T, body string) []ValidationError {
	t.Helper()
	value, err := pyjson.Loads([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	root := value.(*pyjson.OrderedMap)
	detail, _ := root.Get("detail")
	var errs []ValidationError
	for _, item := range detail.(pyjson.List) {
		entry := item.(*pyjson.OrderedMap)
		keys := strings.Join(entry.Keys(), ",")
		if keys != "type,loc,msg" && keys != "type,loc,msg,ctx" {
			t.Fatalf("unexpected error keys %q", keys)
		}
		typ, _ := entry.Get("type")
		loc, _ := entry.Get("loc")
		msg, _ := entry.Get("msg")
		e := ValidationError{Type: string(typ.(pyjson.String)), Loc: loc.(pyjson.List), Msg: string(msg.(pyjson.String))}
		if ctx, ok := entry.Get("ctx"); ok {
			e.Ctx = ctx.(*pyjson.OrderedMap)
		}
		errs = append(errs, e)
	}
	return errs
}

func TestMatchesPythonHandlers(t *testing.T) {
	data, err := os.ReadFile("testdata/python_problems.json")
	if err != nil {
		t.Fatalf("goldens missing — render with scripts/render_problem_goldens.py: %v", err)
	}
	var cases []golden
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 11 {
		t.Fatalf("only %d golden cases", len(cases))
	}
	for _, c := range cases {
		t.Run(c.Case, func(t *testing.T) {
			rec := httptest.NewRecorder()
			switch {
			case c.Status == 500:
				WriteServerError(rec, c.Path)
			case strings.HasPrefix(c.Body, `{"detail":[`):
				if err := WriteValidation(rec, parseValidation(t, c.Body)); err != nil {
					t.Fatal(err)
				}
			default:
				var p struct{ Code, Detail string }
				if err := json.Unmarshal([]byte(c.Body), &p); err != nil {
					t.Fatal(err)
				}
				if err := WriteJSON(rec, Problem{Status: c.Status, Code: p.Code, Detail: p.Detail}); err != nil {
					t.Fatal(err)
				}
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if rec.Code != c.Status {
				t.Errorf("status = %d, want %d", rec.Code, c.Status)
			}
			if string(body) != c.Body {
				t.Errorf("body:\n got  %s\n want %s", body, c.Body)
			}
			got := map[string]string{}
			for name, values := range rec.Result().Header {
				got[strings.ToLower(name)] = strings.Join(values, "\x00")
			}
			want := headerMap(c.Headers)
			names := map[string]bool{}
			for n := range got {
				names[n] = true
			}
			for n := range want {
				names[n] = true
			}
			sorted := make([]string, 0, len(names))
			for n := range names {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)
			for _, n := range sorted {
				if got[n] != want[n] {
					t.Errorf("header %s = %q, want %q", n, got[n], want[n])
				}
			}
		})
	}
}

func TestIsGuestPath(t *testing.T) {
	for path, want := range map[string]bool{"/g": true, "/g/": true, "/g/abc": true, "/goals": false, "/": false, "/gg/x": false} {
		if IsGuestPath(path) != want {
			t.Errorf("IsGuestPath(%q) != %v", path, want)
		}
	}
}
