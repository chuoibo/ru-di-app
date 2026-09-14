package pyval

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/pyjson"
)

// W1Routes are the pilot wave: their validation must bind with no validator
// function left to port.
var W1Routes = []string{
	"PUT /people/me/interests",
	"POST /contexts/{context_id}/meet",
	"POST /reports",
	"GET /contexts/{context_id}/preference-profile",
	"GET /contexts/{context_id}/map",
	"GET /contexts/{context_id}/heatmap",
	"GET /contexts/{context_id}/recap",
}

func loadContract(t *testing.T) *Contract {
	t.Helper()
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestW1RoutesHaveNoUnregisteredValidators(t *testing.T) {
	c := loadContract(t)
	reg := NewRegistry()
	for _, id := range W1Routes {
		rep, err := c.Inspect(id, reg)
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Unregistered) != 0 || len(rep.Unsupported) != 0 {
			t.Errorf("%s: unregistered %v, unsupported %v", id, rep.Unregistered, rep.Unsupported)
		}
		if _, err := c.Bind(id, reg); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
}

func TestBindRefusesAnUnportedValidator(t *testing.T) {
	c := loadContract(t)
	const id = "PATCH /contexts/{context_id}"
	rep, err := c.Inspect(id, NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	want := "app.api.schemas.ContextUpdateRequest._something_to_change"
	if len(rep.Unregistered) != 1 || rep.Unregistered[0] != want {
		t.Fatalf("unregistered = %v, want [%s]", rep.Unregistered, want)
	}
	if _, err := c.Bind(id, NewRegistry()); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Bind error = %v, want one naming %s", err, want)
	}
	reg := NewRegistry()
	reg.Register(want, func(_ *Call, v Value) (Value, error) { return v, nil })
	if _, err := c.Bind(id, reg); err != nil {
		t.Fatalf("Bind after registering: %v", err)
	}
}

func TestBindRefusesUnsupportedFeatures(t *testing.T) {
	c := loadContract(t)
	rep, err := c.Inspect("POST /contexts/{context_id}/photos", NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Unsupported) == 0 {
		t.Fatal("a multipart route bound as if pyval could parse multipart")
	}
}

const (
	actor   = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	context = "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa"
)

var devHeaders = [][2]string{{"X-Actor-ID", actor}, {"X-Actor-Roles", "member"}}

// devActor stands in for get_actor in dev mode, far enough for these cases.
func devActor(d Dependency) error {
	if d.Call == "app.api.deps.get_actor" && isNone(d.Values["actor_id"]) {
		return errAnonymous
	}
	return nil
}

var errAnonymous = errors.New("401 Missing X-Actor-ID")

func validationBody(t *testing.T, errs []problem.ValidationError) string {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := problem.WriteValidation(rec, errs); err != nil {
		t.Fatal(err)
	}
	return rec.Body.String()
}

// Expected bodies below were answered by create_app() in
// mobile-parity-api:7bf58e3d over raw ASGI (dev auth, closed DB).
func TestMeasuredW1Cases(t *testing.T) {
	c := loadContract(t)
	reg := NewRegistry()
	jsonCT := [2]string{"content-type", "application/json"}
	cases := []struct {
		name    string
		route   string
		path    map[string]string
		headers [][2]string
		body    string
		want    string // 422 body, "accepted", "anonymous" or "400"
	}{
		{"malformed json before auth", "PUT /people/me/interests", nil, [][2]string{jsonCT}, `{`,
			`{"detail":[{"type":"json_invalid","loc":["body",1],"msg":"JSON decode error","ctx":{"error":"Expecting property name enclosed in double quotes"}}]}`},
		{"malformed json offset", "PUT /people/me/interests", nil, append(devHeaders, jsonCT), `{"interests": `,
			`{"detail":[{"type":"json_invalid","loc":["body",14],"msg":"JSON decode error","ctx":{"error":"Expecting value"}}]}`},
		{"invalid body after auth", "PUT /people/me/interests", nil, [][2]string{jsonCT}, `{}`, "anonymous"},
		{"text plain keeps bytes", "PUT /people/me/interests", nil, append(devHeaders, [2]string{"content-type", "text/plain"}), `{"interests":["a"]}`,
			`{"detail":[{"type":"model_attributes_type","loc":["body"],"msg":"Input should be a valid dictionary or object to extract fields from"}]}`},
		{"no content type parses", "PUT /people/me/interests", nil, devHeaders, `{"interests":["a"],"budget_band":null}`, "accepted"},
		{"nbsp is stripped", "PUT /people/me/interests", nil, append(devHeaders, [2]string{"content-type", "application/json\xa0"}), `{"interests":[]}`, "accepted"},
		{"two slashes is text", "PUT /people/me/interests", nil, append(devHeaders, [2]string{"content-type", "application/json/x"}), `{"interests":[]}`,
			`{"detail":[{"type":"model_attributes_type","loc":["body"],"msg":"Input should be a valid dictionary or object to extract fields from"}]}`},
		{"first content type wins", "PUT /people/me/interests", nil, append(devHeaders, [2]string{"content-type", "text/plain"}, jsonCT), `{"interests":[]}`,
			`{"detail":[{"type":"model_attributes_type","loc":["body"],"msg":"Input should be a valid dictionary or object to extract fields from"}]}`},
		{"empty body", "PUT /people/me/interests", nil, append(devHeaders, jsonCT), ``,
			`{"detail":[{"type":"missing","loc":["body"],"msg":"Field required"}]}`},
		{"null body", "PUT /people/me/interests", nil, append(devHeaders, jsonCT), `null`,
			`{"detail":[{"type":"missing","loc":["body"],"msg":"Field required"}]}`},
		{"nan is not a list", "PUT /people/me/interests", nil, append(devHeaders, jsonCT), `{"interests": NaN, "budget_band": Infinity}`,
			`{"detail":[{"type":"list_type","loc":["body","interests"],"msg":"Input should be a valid list"},{"type":"string_type","loc":["body","budget_band"],"msg":"Input should be a valid string"}]}`},
		{"undecodable bytes", "PUT /people/me/interests", nil, append(devHeaders, jsonCT), "{\"interests\": [\"\xff\"]}", "400"},
		{"path error before body errors", "POST /contexts/{context_id}/meet", map[string]string{"context_id": "nope"}, append(devHeaders, jsonCT), `{"from_areas": 5, "x": 1}`,
			`{"detail":[{"type":"uuid_parsing","loc":["path","context_id"],"msg":"Input should be a valid UUID, invalid character: found ` + "`n`" + ` at 1","ctx":{"error":"invalid character: found ` + "`n`" + ` at 1"}},{"type":"list_type","loc":["body","from_areas"],"msg":"Input should be a valid list"},{"type":"extra_forbidden","loc":["body","x"],"msg":"Extra inputs are not permitted"}]}`},
		{"anonymous bad uuid is 401", "GET /contexts/{context_id}/recap", map[string]string{"context_id": "nope"}, nil, ``, "anonymous"},
		{"braced uuid", "GET /contexts/{context_id}/recap", map[string]string{"context_id": "{" + context + "}"}, devHeaders, ``, "accepted"},
		{"body ignored without body param", "GET /contexts/{context_id}/recap", map[string]string{"context_id": "nope"}, append(devHeaders, jsonCT), `{`,
			`{"detail":[{"type":"uuid_parsing","loc":["path","context_id"],"msg":"Input should be a valid UUID, invalid character: found ` + "`n`" + ` at 1","ctx":{"error":"invalid character: found ` + "`n`" + ` at 1"}}]}`},
		{"report literal and length", "POST /reports", nil, append(devHeaders, jsonCT), `{"target_type":"Person","target_id":5,"reason":"x","note":"` + strings.Repeat("a", 501) + `"}`,
			`{"detail":[{"type":"literal_error","loc":["body","target_type"],"msg":"Input should be 'person', 'post', 'message', 'comment' or 'story'","ctx":{"expected":"'person', 'post', 'message', 'comment' or 'story'"}},{"type":"uuid_type","loc":["body","target_id"],"msg":"UUID input should be a string, bytes or UUID object"},{"type":"literal_error","loc":["body","reason"],"msg":"Input should be 'spam', 'harassment', 'inappropriate', 'impersonation' or 'other'","ctx":{"expected":"'spam', 'harassment', 'inappropriate', 'impersonation' or 'other'"}},{"type":"string_too_long","loc":["body","note"],"msg":"String should have at most 500 characters","ctx":{"max_length":500}}]}`},
		{"report missing fields in order", "POST /reports", nil, append(devHeaders, jsonCT), `{}`,
			`{"detail":[{"type":"missing","loc":["body","target_type"],"msg":"Field required"},{"type":"missing","loc":["body","target_id"],"msg":"Field required"},{"type":"missing","loc":["body","reason"],"msg":"Field required"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			route, err := c.Bind(tc.route, reg)
			if err != nil {
				t.Fatal(err)
			}
			res, err := route.Validate(&Request{PathParams: tc.path, Headers: tc.headers, Body: []byte(tc.body)}, devActor)
			var got string
			var be *BodyError
			switch {
			case errors.Is(err, errAnonymous):
				got = "anonymous"
			case errors.As(err, &be):
				got = "400"
			case err != nil:
				t.Fatal(err)
			case len(res.Errors) == 0:
				got = "accepted"
			default:
				got = validationBody(t, res.Errors)
			}
			if got != tc.want {
				t.Errorf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestDecodedReport(t *testing.T) {
	c := loadContract(t)
	route, err := c.Bind("POST /reports", NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	body := `{"target_type":"post","target_id":"CCCCCCCCDDDD4EEE8FFFAAAAAAAAAAAA","reason":"other","note":"  hi  "}`
	res, err := route.Validate(&Request{Headers: devHeaders, Body: []byte(body)}, devActor)
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("err=%v errors=%v", err, res)
	}
	m := res.Values["request"].(*Model)
	id, _ := m.Get("target_id")
	if id.(UUID).String() != context {
		t.Errorf("target_id = %v", id)
	}
	note, _ := m.Get("note")
	if note != pyjson.String("  hi  ") {
		t.Errorf("note = %#v, want the unstripped string", note)
	}
	if strings.Join(m.FieldsSet, ",") != "target_type,target_id,reason,note" {
		t.Errorf("fields set = %v", m.FieldsSet)
	}
}

func TestUUIDSpellings(t *testing.T) {
	// Messages measured with TypeAdapter(UUID).validate_python in the image.
	cases := map[string]string{
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee":          "",
		"AAAAAAAABBBB4CCC8DDDEEEEEEEEEEEE":              "",
		"{aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee}":        "",
		"urn:uuid:aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee": "",
		"URN:UUID:aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee": "invalid character: found `U` at 1",
		"khong-phai-uuid":                               "invalid character: found `k` at 1",
		"":                                              "invalid length: expected length 32 for simple format, found 0",
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeee":           "invalid group length in group 4: expected 12, found 11",
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeeee":         "invalid group length in group 4: expected 12, found 13",
		"aaaaaaaa-bbbb-4ccc-8dddeeeeeeeeeeee":           "invalid group count: expected 5, found 4",
		"{aaaaaaaabbbb4ccc8dddeeeeeeeeeeee}":            "invalid group count: expected 5, found 1",
		"urn:uuid:":                                     "invalid group count: expected 5, found 1",
		"a-b-c-d-e":                                     "invalid group length in group 0: expected 8, found 1",
		"a-b-c-d-e-f":                                   "invalid group count: expected 5, found 6",
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeee\u00e9":     "invalid character: found `\u00e9` at 36",
	}
	for in, want := range cases {
		_, got := parseUUID(in)
		if got != want {
			t.Errorf("parseUUID(%q) = %q, want %q", in, got, want)
		}
	}
}
