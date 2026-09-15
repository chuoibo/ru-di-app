package guest

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/oracletest"
)

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_views.json"), "views")
	strs := func(key string) []string {
		out, err := oracletest.Strings(constants[key])
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		return out
	}
	sortedKeys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	for key, got := range map[string][]string{
		"allowed_top_level":          AllowedTopLevel,
		"allowed_block":              AllowedBlock,
		"forbidden_input_keys":       ForbiddenInputKeys,
		"allowed_not_me":             AllowedNotMe,
		"allowed_wrong_amount":       AllowedWrongAmount,
		"objection_kinds":            sortedKeys(ObjectionKinds),
		"quota_consuming_objections": sortedKeys(QuotaConsumingObjections),
	} {
		if want := strs(key); !reflect.DeepEqual(want, got) {
			t.Errorf("%s: Python %v, Go %v", key, want, got)
		}
	}
	var reasons []any
	for _, r := range ObjectionReasons {
		reasons = append(reasons, []any{r[0], r[1]})
	}
	if !reflect.DeepEqual(constants["objection_reasons"], reasons) {
		t.Errorf("objection_reasons: Python %v, Go %v", constants["objection_reasons"], reasons)
	}
	if !reflect.DeepEqual(constants["neutral_preview"], NeutralPreview()) {
		t.Errorf("neutral_preview: Python %v, Go %v", constants["neutral_preview"], NeutralPreview())
	}
	if constants["int_max_str_digits"] != int64(maxStrDigits) {
		t.Errorf("int_max_str_digits: Python %v, Go %d", constants["int_max_str_digits"], maxStrDigits)
	}
	var spaces []any
	for r := rune(0); r <= 0x10FFFF; r++ {
		if isPySpace(r) {
			spaces = append(spaces, int64(r))
		}
	}
	if !reflect.DeepEqual(constants["isspace"], spaces) {
		t.Errorf("isspace: Python %v, Go %v", constants["isspace"], spaces)
	}
}

// The embedded copies are what the distroless image serves; they must be the
// templates Python renders.
func TestEmbeddedTemplatesMatchPythonTree(t *testing.T) {
	source := filepath.Join("..", "..", "..", "..", "api", "app", "web", "templates")
	for _, name := range TemplateNames {
		want, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatalf("reading the Python template: %v", err)
		}
		got, err := templateFiles.ReadFile("templates/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != string(got) {
			t.Errorf("templates/%s differs from services/api/app/web/templates/%s; copy it again", name, name)
		}
	}
	entries, err := templateFiles.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(TemplateNames) {
		t.Errorf("%d embedded templates, %d names", len(entries), len(TemplateNames))
	}
}

func TestUnsupportedConstructsFailAtLoad(t *testing.T) {
	for name, source := range map[string]string{
		"filter":              "{{ view.x|e }}",
		"set":                 "{% set x = 'a' %}",
		"include":             "{% include 'a.html' %}",
		"macro":               "{% macro m() %}{% endmacro %}",
		"raw":                 "{% raw %}x{% endraw %}",
		"plus_modifier":       "{%+ if x %}{% endif %}",
		"plus_closer":         "{% if x +%}{% endif %}",
		"int_literal":         "{{ 1 }}",
		"call":                "{{ x() }}",
		"equals":              "{% if a == 'b' %}{% endif %}",
		"ne_name":             "{% if a != b %}{% endif %}",
		"chained_ne":          "{% if a != 'b' != 'c' %}{% endif %}",
		"and":                 "{% if a and b %}{% endif %}",
		"not":                 "{% if not a %}{% endif %}",
		"true_literal":        "{% if true %}{% endif %}",
		"list_subscript":      "{{ view.blocks[0] }}",
		"dict_method_attr":    "{{ view.items }}",
		"dict_value_not_str":  "{{ {'a': x}['a'] }}",
		"bare_dict":           "{{ {'a': 'b'} }}",
		"loop_outside":        "{{ loop.first }}",
		"loop_index":          "{% for x in y %}{{ loop.index }}{% endfor %}",
		"loop_target":         "{% for loop in y %}{% endfor %}",
		"for_if":              "{% for x in y if x %}{% endfor %}",
		"for_else":            "{% for x in y %}{% else %}{% endfor %}",
		"unclosed_if":         "{% if x %}",
		"unclosed_for":        "{% for x in y %}",
		"stray_endif":         "{% endif %}",
		"unclosed_comment":    "{# x",
		"unclosed_variable":   "{{ x",
		"backslash_string":    `{{ 'a\n' }}`,
		"else_args":           "{% if x %}{% else x %}{% endif %}",
		"endfor_closes_if":    "{% if x %}{% endfor %}",
		"arithmetic":          "{{ a + b }}",
		"attribute_of_string": "{{ 'a'.upper }}",
	} {
		_, err := Parse(name, source)
		var parseErr *ParseError
		if !errors.As(err, &parseErr) {
			t.Errorf("%s: %q parsed (err %v)", name, source, err)
		}
	}
}

// Probed in the parity image: the "-" modifiers strip every str.isspace
// character, not only ASCII ones.
func TestWhitespaceControlStripsUnicodeSpace(t *testing.T) {
	tmpl, err := Parse("ws", "x  {%- if a -%} y{% endif %}\n")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tmpl.Render(map[string]any{"a": true})
	if err != nil || got != "xy" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// ServerError is the answer package problem already writes for a crash; the
// two must not drift. net/http orders headers itself, so the set is compared.
func TestServerErrorAgreesWithProblemPackage(t *testing.T) {
	for _, path := range []string{"/g", "/g/abc", "/goals", "/"} {
		rec := httptest.NewRecorder()
		problem.WriteServerError(rec, path)
		want := ServerError(path)
		if rec.Code != want.Status || rec.Body.String() != string(want.Body) {
			t.Fatalf("%s: problem %d %q, guest %d %q", path, rec.Code, rec.Body.String(), want.Status, want.Body)
		}
		got := map[string]string{}
		for name, values := range rec.Result().Header {
			got[strings.ToLower(name)] = strings.Join(values, "\x00")
		}
		wantHeaders := map[string]string{}
		for _, pair := range want.Headers {
			wantHeaders[pair[0]] = pair[1]
		}
		if !reflect.DeepEqual(got, wantHeaders) {
			t.Fatalf("%s: problem %v, guest %v", path, got, wantHeaders)
		}
	}
}

func TestTypedEnvelopeIsTheRepositoryDict(t *testing.T) {
	e := Envelope{
		RecordedByDisplayName: "Nam", ClaimedPersonDisplayName: "Ha", LinkState: "active",
		Obligations:    []EnvelopeObligation{{ObligationID: "o1", OccasionLabel: "x", AmountVND: 82000, RecipientDisplayName: "Nam", ObjectionsAllowed: 3}},
		ReportsAllowed: 3, ObjectionsAllowed: 3,
	}
	view, err := BuildGuestView(e.Dict())
	if err != nil {
		t.Fatal(err)
	}
	block := view["blocks"].([]any)[0].(map[string]any)
	if block["amount_display"] != "82.000" || view["can_report_payment"] != true || view["can_object"] != true {
		t.Fatalf("%v", view)
	}
	obligationKeys := []string{}
	for k := range e.Dict()["obligations"].([]any)[0].(map[string]any) {
		obligationKeys = append(obligationKeys, k)
	}
	sort.Strings(obligationKeys)
	want := []string{"already_reported", "amount_vnd", "disputed", "evidence_requested", "objections_allowed", "objections_used", "obligation_id", "occasion_label", "receiver_confirmed", "recipient_display_name"}
	if !reflect.DeepEqual(obligationKeys, want) {
		t.Fatalf("obligation keys %v", obligationKeys)
	}
}
