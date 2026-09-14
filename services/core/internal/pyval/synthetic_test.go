package pyval

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"mobile/services/core/internal/httpapi/problem"
)

// The synthetic route exists only inside the parity image, added to
// create_app() by TestOracleSynthetic, because no route of the API declares
// a query and a header parameter on the same dependant, and none declares a
// header parameter that validation reaches with repeated values. It pins:
//
//   - errors in the order path, query, header (solve_dependencies);
//   - the first value of a repeated header name, in any letter case
//     (Headers.get), and every value of a sequence header (Headers.getlist);
//   - the last value of a repeated query key (ImmutableMultiDict).
//
// testdata/synthetic_ir.json is that route's IR as scripts/render_contract_ir.py
// renders it in the image, and testdata/synthetic_cases.json the answers the
// image gave; TestOracleSynthetic re-measures both and fails when they drift.
//
//	def order(item: int, q: Annotated[int, Query(ge=1)],
//	          x_count: Annotated[int, Header(ge=1)],
//	          x_tag: Annotated[list[int] | None, Header()] = None,
//	          x_name: Annotated[str, Header(min_length=3)] = "abc")
const (
	syntheticRouteID = "GET /pyval-synthetic/order/{item}"
	syntheticPath    = "/pyval-synthetic/order/{item}"
)

type syntheticCase struct {
	Name    string            `json:"name"`
	Path    map[string]string `json:"path"`
	Query   string            `json:"query"`
	Headers [][2]string       `json:"headers"`
}

func syntheticCases() []syntheticCase {
	h := func(kv ...string) [][2]string {
		out := [][2]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			out = append(out, [2]string{kv[i], kv[i+1]})
		}
		return out
	}
	item := func(v string) map[string]string { return map[string]string{"item": v} }
	return []syntheticCase{
		{"valid", item("1"), "q=5", h("X-Count", "2", "X-Tag", "1", "X-Tag", "2", "X-Name", "abcd")},
		{"valid defaults", item("1"), "q=5", h("X-Count", "2")},
		{"query and header below bound", item("1"), "q=0", h("X-Count", "0")},
		{"query and header unparsable", item("1"), "q=x", h("X-Count", "y")},
		{"path query header all invalid", item("z"), "q=0", h("X-Count", "0")},
		{"query and header missing", item("1"), "", h()},
		{"header name lower case", item("1"), "q=-1", h("x-count", "0")},
		{"every parameter faulty", item("z"), "q=x", h("X-Count", "0", "X-Tag", "a", "X-Name", "ab")},
		{"repeated query and header", item("1"), "q=x&q=0", h("X-Count", "0", "X-Count", "1")},
		{"repeated header valid first", item("1"), "q=5", h("X-Count", "2", "X-Count", "0")},
		{"repeated header invalid first", item("1"), "q=5", h("X-Count", "0", "X-Count", "2")},
		{"repeated header mixed case valid first", item("1"), "q=5", h("x-count", "2", "X-COUNT", "0")},
		{"repeated header mixed case invalid first", item("1"), "q=5", h("X-COUNT", "abc", "x-count", "3")},
		{"repeated string header short first", item("1"), "q=5", h("X-Count", "2", "X-Name", "ab", "X-Name", "abcd")},
		{"repeated string header long first", item("1"), "q=5", h("X-Count", "2", "x-name", "abcd", "X-NAME", "ab")},
		{"repeated query last valid", item("1"), "q=0&q=5", h("X-Count", "2")},
		{"repeated query last invalid", item("1"), "q=5&q=0", h("X-Count", "2")},
		{"sequence header keeps order", item("1"), "q=5", h("X-Count", "2", "X-Tag", "1", "X-Tag", "x", "X-Tag", "3")},
		{"sequence header mixed case", item("1"), "q=5", h("X-Count", "2", "x-tag", "2", "X-TAG", "y")},
		{"sequence header all valid mixed case", item("1"), "q=5", h("X-Count", "2", "X-Tag", "3", "x-tag", "4")},
	}
}

type syntheticRecord struct {
	Image string `json:"image"`
	Route string `json:"route"`
	Cases []struct {
		syntheticCase
		Want json.RawMessage `json:"want"`
	} `json:"cases"`
}

func TestSyntheticParamOrder(t *testing.T) {
	route := bindSynthetic(t, readFile(t, "testdata/synthetic_ir.json"))
	var rec syntheticRecord
	if err := json.Unmarshal(readFile(t, "testdata/synthetic_cases.json"), &rec); err != nil {
		t.Fatal(err)
	}
	cases := syntheticCases()
	if len(rec.Cases) != len(cases) {
		t.Fatalf("recorded %d cases, the test declares %d: rerun TestOracleSynthetic with PYVAL_ORACLE_RECORD=1", len(rec.Cases), len(cases))
	}
	for i, c := range rec.Cases {
		declared, _ := json.Marshal(cases[i])
		recorded, _ := json.Marshal(c.syntheticCase)
		if string(declared) != string(recorded) {
			t.Fatalf("case %d differs from the recording: rerun TestOracleSynthetic with PYVAL_ORACLE_RECORD=1", i)
		}
		want := normalizeJSON(t, c.Want)
		got := normalizeJSON(t, syntheticOutcome(t, route, c.syntheticCase))
		if got != want {
			t.Errorf("%s\n  python: %s\n  go:     %s", c.Name, want, got)
		}
	}
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func bindSynthetic(t *testing.T, ir []byte) *Route {
	t.Helper()
	c, err := Parse(map[string][]byte{"synthetic_ir.json": ir})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Bind(syntheticRouteID, NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// syntheticOutcome answers in the oracle drivers' shape: 299 with the value
// tree of every endpoint parameter, or the status, content-type and body of
// a refusal.
func syntheticOutcome(t *testing.T, r *Route, sc syntheticCase) []byte {
	t.Helper()
	res, err := r.Validate(&Request{PathParams: sc.Path, RawQuery: sc.Query, Headers: sc.Headers}, nil)
	if err != nil {
		t.Fatalf("%s: %v", sc.Name, err)
	}
	if len(res.Errors) > 0 {
		rec := httptest.NewRecorder()
		if err := problem.WriteValidation(rec, res.Errors); err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]any{
			"status": rec.Code, "content_type": rec.Header().Get("Content-Type"), "body": latin1(rec.Body.String()),
		})
		return out
	}
	accepted := []any{}
	for _, in := range []string{"path", "query", "header", "cookie", "body"} {
		for _, p := range r.root.params[in] {
			accepted = append(accepted, []any{p.name, goTree(res.Values[p.name])})
		}
	}
	out, _ := json.Marshal(map[string]any{"status": 299, "accepted": accepted})
	return out
}

func syntheticWirePath(sc syntheticCase) string {
	return strings.Replace(syntheticPath, "{item}", sc.Path["item"], 1)
}
