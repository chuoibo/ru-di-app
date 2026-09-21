package pyval

// Form bodies as the parity image answers them. formCases declares requests
// for every Form or File route of the contract that no auth dependency
// guards, and for the two synthetic form routes of
// testdata/synthetic_form_ir.json (TestOracleForms adds them to create_app()
// in the image and renders their IR there). testdata/form_cases.json holds
// what the image answered to each case; TestFormCases replays those answers
// against Go without Docker. Record them again with
//
//	PYVAL_ORACLE_RECORD=1 go test -tags oracle -run TestOracleForms ./internal/pyval/
//
// A recorded case names its request by digest and, when Python's answer is
// long (a megabyte field echoed back), keeps the answer's digest too, so the
// file stays small. Digests are written with the letters a-p for the nibbles
// 0-f: a hex digest can hold a run of digits the repository guard reads as
// a phone number.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/pyjson"
)

const (
	syntheticFormFieldsRoute = "POST /pyval-synthetic/form/fields"
	syntheticFormFileRoute   = "POST /pyval-synthetic/form/file"

	formBoundary        = "pyvalFormBoundary"
	formURLEncoded      = "application/x-www-form-urlencoded"
	formMultipart       = "multipart/form-data; boundary=" + formBoundary
	formWantInlineLimit = 1500
	formUUID            = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	formUUIDOther       = "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff"
)

var formToken = strings.Repeat("Tk", 16)

type formCase struct {
	Name    string
	Route   string
	Params  map[string]string
	Headers [][2]string
	Body    string
}

func (fc formCase) path(r *Route) string {
	path := r.Path
	for k, v := range fc.Params {
		path = strings.Replace(path, "{"+k+"}", v, 1)
	}
	return path
}

// digest names the request bytes: route, path parameters, headers, body.
func (fc formCase) digest() string {
	h := sha256.New()
	write := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		h.Write(n[:])
		h.Write([]byte(s))
	}
	write(fc.Route)
	keys := make([]string, 0, len(fc.Params))
	for k := range fc.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		write(k)
		write(fc.Params[k])
	}
	for _, hd := range fc.Headers {
		write(hd[0])
		write(hd[1])
	}
	write(fc.Body)
	return letterDigest(h.Sum(nil))
}

func letterDigest(sum []byte) string {
	out := make([]byte, 0, 2*len(sum))
	for _, b := range sum {
		out = append(out, 'a'+b>>4, 'a'+b&0x0F)
	}
	return string(out)
}

type formRecord struct {
	Image string           `json:"image"`
	Cases []formRecordCase `json:"cases"`
}

type formRecordCase struct {
	Name       string          `json:"name"`
	Route      string          `json:"route"`
	Request    string          `json:"request"`
	Want       json.RawMessage `json:"want,omitempty"`
	WantDigest string          `json:"want_digest,omitempty"`
	WantBytes  int             `json:"want_bytes,omitempty"`
}

func newFormRecordCase(fc formCase, want string) formRecordCase {
	rc := formRecordCase{Name: fc.Name, Route: fc.Route, Request: fc.digest()}
	if len(want) <= formWantInlineLimit {
		rc.Want = json.RawMessage(want)
	} else {
		sum := sha256.Sum256([]byte(want))
		rc.WantDigest, rc.WantBytes = letterDigest(sum[:]), len(want)
	}
	return rc
}

// matches compares a normalized Go outcome with the recording.
func (rc formRecordCase) matches(t *testing.T, got string) bool {
	if rc.WantDigest == "" {
		return normalizeJSON(t, rc.Want) == got
	}
	sum := sha256.Sum256([]byte(got))
	return letterDigest(sum[:]) == rc.WantDigest && len(got) == rc.WantBytes
}

// encodeFormRecord writes one case per line, so a re-recording diffs by case.
func encodeFormRecord(rec formRecord) ([]byte, error) {
	var b strings.Builder
	image, err := json.Marshal(rec.Image)
	if err != nil {
		return nil, err
	}
	b.WriteString("{\n \"image\": " + string(image) + ",\n \"cases\": [\n")
	for i, c := range rec.Cases {
		var line strings.Builder
		enc := json.NewEncoder(&line)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(c); err != nil {
			return nil, err
		}
		b.WriteString("  " + strings.TrimSuffix(line.String(), "\n"))
		if i < len(rec.Cases)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(" ]\n}\n")
	return []byte(b.String()), nil
}

func TestFormCases(t *testing.T) {
	routes := formCaseRoutes(t, readFile(t, "testdata/synthetic_form_ir.json"))
	cases := formCases(routes)
	var rec formRecord
	if err := json.Unmarshal(readFile(t, "testdata/form_cases.json"), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Cases) != len(cases) {
		t.Fatalf("recorded %d form cases, the test declares %d: rerun TestOracleForms with PYVAL_ORACLE_RECORD=1", len(rec.Cases), len(cases))
	}
	mismatches := 0
	for i, fc := range cases {
		rc := rec.Cases[i]
		if rc.Name != fc.Name || rc.Route != fc.Route || rc.Request != fc.digest() {
			t.Fatalf("case %d (%s %s) differs from the recording: rerun TestOracleForms with PYVAL_ORACLE_RECORD=1", i, fc.Route, fc.Name)
		}
		got := normalizeJSON(t, formOutcome(t, routes[fc.Route], fc))
		if rc.matches(t, got) {
			continue
		}
		mismatches++
		if mismatches <= 10 {
			want := string(rc.Want)
			if rc.WantDigest != "" {
				want = "(digest " + rc.WantDigest + ", " + strconv.Itoa(rc.WantBytes) + " bytes)"
			}
			t.Errorf("%s [%s]\n  python: %s\n  go:     %s", fc.Route, fc.Name, clipText(want, 600), clipText(got, 600))
		}
	}
	t.Logf("form cases from %s: %d routes, %d cases, %d mismatches", rec.Image, len(routes), len(cases), mismatches)
}

// TestBindRefusesUnsupportedFeatures edits a form route's IR in memory into
// shapes pyval does not implement and expects Bind to name them.
func TestBindRefusesUnsupportedFeatures(t *testing.T) {
	const id = "POST /g/{token}/doi-so-tien"
	for _, tc := range []struct {
		name, want string
		edit       func(route *pyjson.OrderedMap)
	}{
		{"sequence form field", "form parameter obligation_id is a sequence", func(route *pyjson.OrderedMap) {
			body := mustGet(mustGet(route, "dependant").(*pyjson.OrderedMap), "body").(pyjson.List)
			body[0].(*pyjson.OrderedMap).Set("sequence", pyjson.Bool(true))
		}},
		{"list default", "form parameter reason with a default other than None or a scalar", func(route *pyjson.OrderedMap) {
			body := mustGet(mustGet(route, "dependant").(*pyjson.OrderedMap), "body").(pyjson.List)
			reason := body[1].(*pyjson.OrderedMap)
			reason.Set("required", pyjson.Bool(false))
			reason.Set("default", pyjson.List{pyjson.String("x")})
		}},
		{"unknown body kind", "octet-stream request body", func(route *pyjson.OrderedMap) {
			mustGet(route, "body").(*pyjson.OrderedMap).Set("kind", pyjson.String("octet-stream"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := loadContract(t)
			if _, err := c.Bind(id, NewRegistry()); err != nil {
				t.Fatalf("unedited: %v", err)
			}
			tc.edit(c.routes[id].node)
			if _, err := c.Bind(id, NewRegistry()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Bind error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}

// formCaseRoutes binds the contract's form routes that no auth dependency
// guards, and the synthetic form routes of ir. A form route of the contract
// that does not bind fails the test.
func formCaseRoutes(t *testing.T, ir []byte) map[string]*Route {
	t.Helper()
	reg := NewRegistry()
	routes := map[string]*Route{}
	c := loadContract(t)
	for _, id := range c.RouteIDs() {
		r, rep, err := c.compile(id, reg)
		if err != nil {
			t.Fatal(err)
		}
		if !r.form || callsActor(r.root) {
			continue
		}
		if len(rep.Unsupported)+len(rep.Unregistered) > 0 {
			t.Fatalf("form route %s does not bind: %v %v", id, rep.Unsupported, rep.Unregistered)
		}
		routes[id] = r
	}
	sc, err := Parse(map[string][]byte{"synthetic_form_ir.json": ir})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range sc.RouteIDs() {
		r, err := sc.Bind(id, reg)
		if err != nil {
			t.Fatal(err)
		}
		routes[id] = r
	}
	return routes
}

func callsActor(d *dependant) bool {
	for _, s := range d.deps {
		if s.call == "app.api.deps.get_actor" || s.call == "app.api.deps.get_actor_optional" || callsActor(s) {
			return true
		}
	}
	return false
}

// formOutcome answers in the oracle drivers' shape.
func formOutcome(t *testing.T, r *Route, fc formCase) []byte {
	t.Helper()
	res, err := r.Validate(&Request{PathParams: fc.Params, Headers: fc.Headers, Body: []byte(fc.Body)}, nil)
	return renderOutcome(t, r, res, err)
}

// renderOutcome renders a Validate result: 299 with the value tree of every
// endpoint parameter, or the status, content type and body of a refusal.
func renderOutcome(t *testing.T, r *Route, res *Result, err error) []byte {
	t.Helper()
	rec := httptest.NewRecorder()
	var be *BodyError
	switch {
	case errors.As(err, &be):
		if werr := be.Respond(rec); werr != nil {
			t.Fatal(werr)
		}
	case err != nil:
		t.Fatalf("%s: %v", r.ID, err)
	case len(res.Errors) > 0:
		if werr := problem.WriteValidation(rec, res.Errors); werr != nil {
			t.Fatal(werr)
		}
	default:
		accepted := []any{}
		for _, in := range []string{"path", "query", "header", "cookie", "body"} {
			for _, p := range r.root.params[in] {
				if v, ok := res.Values[p.name]; ok {
					accepted = append(accepted, []any{p.name, goTree(v)})
				}
			}
		}
		out, _ := json.Marshal(map[string]any{"status": 299, "accepted": accepted})
		return out
	}
	out, _ := json.Marshal(map[string]any{
		"status":       rec.Code,
		"content_type": rec.Header().Get("Content-Type"),
		"body":         latin1(rec.Body.String()),
	})
	return out
}

// ---- the fields a form route reads ----

type formSlot struct {
	alias    string
	kind     string // uuid, str, int, file or other
	required bool
}

func formSlotsOf(r *Route) []formSlot {
	d := r.root
	body := d.params["body"]
	var out []formSlot
	if len(body) == 1 && !r.embed {
		for i, f := range formModel(body[0].v).fields {
			out = append(out, formSlot{alias: f.name, kind: slotKind(f.v), required: d.formFields[i].required})
		}
		return out
	}
	for i, p := range body {
		out = append(out, formSlot{alias: p.alias, kind: slotKind(p.v), required: d.formFields[i].required})
	}
	return out
}

func slotKind(v validator) string {
	for {
		switch x := v.(type) {
		case *nullableValidator:
			v = x.inner
		case *defaultValidator:
			v = x.inner
		case *refValidator:
			v = x.inner
		case *funcValidator:
			if x.name_ == "fastapi.datastructures.UploadFile._validate" {
				return "file"
			}
			if x.inner == nil {
				return "other"
			}
			v = x.inner
		case *uuidValidator:
			return "uuid"
		case *strValidator:
			return "str"
		case *intValidator:
			return "int"
		default:
			return "other"
		}
	}
}

func goodFormValue(s formSlot, n int) string {
	switch s.kind {
	case "uuid":
		if n == 0 {
			return formUUID
		}
		return formUUIDOther
	case "str":
		return []string{"ok", "ok-two"}[n%2]
	case "int":
		return []string{"7", "8"}[n%2]
	case "file":
		return "file content"
	}
	return "x"
}

type formWrong struct{ label, raw string }

// formWrongs are raw urlencoded values: escapes are left for the parser.
func formWrongs(s formSlot) []formWrong {
	switch s.kind {
	case "uuid":
		return []formWrong{
			{"word", "nope"}, {"invalid escape", "%ZZ"}, {"escaped braces", "%7B" + formUUID + "%7D"},
			{"upper case", strings.ToUpper(formUUID)}, {"plus for hyphen", strings.Replace(formUUID, "-", "+", 1)},
			{"hex only", strings.ReplaceAll(formUUID, "-", "")}, {"escaped urn", "urn%3Auuid%3A" + formUUID},
			{"escaped letter", "%61" + formUUID[1:]}, {"raw utf-8", "\xc3\xa9"}, {"escaped surrogate", "%ED%A0%80"},
			{"leading plus", "+" + formUUID},
		}
	case "str":
		return []formWrong{
			{"plus", "a+b"}, {"escaped plus", "a%2Bb"}, {"invalid escape", "%ZZ"}, {"lone percent", "%"},
			{"escaped utf-8", "%C3%A9"}, {"truncated utf-8", "%C3"}, {"escaped surrogate", "%ED%A0%80"},
			{"raw utf-8", "\xc3\xa9"}, {"raw latin-1", "\xe9"}, {"escaped nul", "%00"}, {"only plus", "+"},
			{"equals", "a=b"}, {"semicolon", "a;b"}, {"escaped newline", "%0D%0A"},
		}
	case "int":
		return []formWrong{
			{"underscore", "1_0"}, {"escaped sign", "%2B7"}, {"padded", "+7+"}, {"float", "7.0"},
			{"word", "x"}, {"escaped digit", "%37"}, {"fullwidth", "%EF%BC%97"},
		}
	}
	return []formWrong{{"text", "x"}}
}

// ---- request bodies ----

func formHeaders(contentType string) [][2]string {
	return [][2]string{{"Content-Type", contentType}}
}

func formEncode(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p[0] + "=" + p[1]
	}
	return strings.Join(parts, "&")
}

func mpText(name, data string) string {
	return "Content-Disposition: form-data; name=\"" + name + "\"\r\n\r\n" + data
}

func mpFile(name, filename, data string) string {
	return "Content-Disposition: form-data; name=\"" + name + "\"; filename=\"" + filename + "\"\r\nContent-Type: application/octet-stream\r\n\r\n" + data
}

// mpJoin frames parts (headers, blank line, data) with boundary.
func mpJoin(boundary string, parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString("--" + boundary + "\r\n" + p + "\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.String()
}

func withItems[T any](list []T, extra ...T) []T {
	return append(append([]T(nil), list...), extra...)
}

type formBuilder struct {
	id     string
	route  *Route
	slots  []formSlot
	params map[string]string
	cases  []formCase
}

func newFormBuilder(id string, r *Route) *formBuilder {
	b := &formBuilder{id: id, route: r, slots: formSlotsOf(r), params: map[string]string{}}
	for _, p := range r.root.params["path"] {
		b.params[p.name] = formToken
	}
	return b
}

func (b *formBuilder) add(name string, headers [][2]string, body string) {
	b.addWith(name, b.params, headers, body)
}

func (b *formBuilder) addWith(name string, params map[string]string, headers [][2]string, body string) {
	b.cases = append(b.cases, formCase{Name: name, Route: b.id, Params: params, Headers: headers, Body: body})
}

func (b *formBuilder) hasFile() bool {
	for _, s := range b.slots {
		if s.kind == "file" {
			return true
		}
	}
	return false
}

// pairs are the valid urlencoded fields, skipping index skip and, unless
// optional, the fields that are not required.
func (b *formBuilder) pairs(skip int, optional bool) [][2]string {
	var out [][2]string
	for i, s := range b.slots {
		if i == skip || s.kind == "file" || !optional && !s.required {
			continue
		}
		out = append(out, [2]string{url.QueryEscape(s.alias), url.QueryEscape(goodFormValue(s, 0))})
	}
	return out
}

func (b *formBuilder) parts(skip int) []string {
	var out []string
	for i, s := range b.slots {
		if i == skip {
			continue
		}
		if s.kind == "file" {
			out = append(out, mpFile(s.alias, "photo.png", goodFormValue(s, 0)))
		} else {
			out = append(out, mpText(s.alias, goodFormValue(s, 0)))
		}
	}
	return out
}

// textSlot is the first str field, else the first field.
func (b *formBuilder) textSlot() int {
	for i, s := range b.slots {
		if s.kind == "str" {
			return i
		}
	}
	return 0
}

func formCases(routes map[string]*Route) []formCase {
	ids := make([]string, 0, len(routes))
	for id := range routes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []formCase
	for _, id := range ids {
		b := newFormBuilder(id, routes[id])
		b.valid()
		b.perSlot()
		b.extras()
		b.pathOrder()
		switch id {
		case "POST /g/{token}/doi-so-tien", "POST /g/{token}/da-chuyen":
			b.contentTypes()
			b.jsonBodies()
			b.urlencodedEdges()
			b.multipartEdges()
			b.charsets()
			b.limits()
		case syntheticFormFileRoute:
			b.fileCases()
			b.limits()
		}
		out = append(out, b.cases...)
	}
	return out
}

func (b *formBuilder) valid() {
	all := b.pairs(-1, true)
	if !b.hasFile() {
		reversed := withItems(all)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		b.add("valid urlencoded", formHeaders(formURLEncoded), formEncode(all))
		b.add("valid urlencoded reversed", formHeaders(formURLEncoded), formEncode(reversed))
		b.add("valid urlencoded required only", formHeaders(formURLEncoded), formEncode(b.pairs(-1, false)))
		b.add("valid urlencoded accepting html", withItems(formHeaders(formURLEncoded), [2]string{"Accept", "text/html"}), formEncode(all))
	}
	parts := b.parts(-1)
	b.add("valid multipart", formHeaders(formMultipart), mpJoin(formBoundary, parts...))
	b.add("empty body urlencoded", formHeaders(formURLEncoded), "")
	b.add("empty body multipart", formHeaders(formMultipart), "")
	b.add("empty body without content type", nil, "")
}

func (b *formBuilder) perSlot() {
	for i, s := range b.slots {
		a := url.QueryEscape(s.alias)
		others := b.pairs(i, true)
		ct := formHeaders(formURLEncoded)
		if s.kind != "file" {
			good := url.QueryEscape(goodFormValue(s, 0))
			bad := formWrongs(s)[0].raw
			b.add(s.alias+": missing", ct, formEncode(others))
			b.add(s.alias+": empty", ct, formEncode(withItems(others, [2]string{a, ""})))
			b.add(s.alias+": bare name last", ct, strings.TrimPrefix(formEncode(others)+"&"+a, "&"))
			b.add(s.alias+": bare name first", ct, a+"&"+formEncode(others))
			b.add(s.alias+": repeated good then bad", ct, formEncode(withItems(others, [2]string{a, good}, [2]string{a, bad})))
			b.add(s.alias+": repeated bad then good", ct, formEncode(withItems(others, [2]string{a, bad}, [2]string{a, good})))
			b.add(s.alias+": repeated empty then good", ct, formEncode(withItems(others, [2]string{a, ""}, [2]string{a, good})))
			b.add(s.alias+": repeated good then empty", ct, formEncode(withItems(others, [2]string{a, good}, [2]string{a, ""})))
			for _, w := range formWrongs(s) {
				b.add(s.alias+": value "+w.label, ct, formEncode(withItems(others, [2]string{a, w.raw})))
			}
			b.add(s.alias+": huge value", ct, formEncode(withItems(others, [2]string{a, strings.Repeat("a", multipartMaxPartSize+1)})))
		} else {
			b.add(s.alias+": urlencoded text", ct, formEncode(withItems(others, [2]string{a, "x"})))
			b.add(s.alias+": urlencoded empty", ct, formEncode(withItems(others, [2]string{a, ""})))
		}
		mp := formHeaders(formMultipart)
		rest := b.parts(i)
		b.add(s.alias+": multipart missing", mp, mpJoin(formBoundary, rest...))
		b.add(s.alias+": multipart empty", mp, mpJoin(formBoundary, withItems(rest, mpText(s.alias, ""))...))
		b.add(s.alias+": multipart with filename", mp, mpJoin(formBoundary, withItems(rest, mpFile(s.alias, "a.txt", goodFormValue(s, 0)))...))
		b.add(s.alias+": multipart without filename", mp, mpJoin(formBoundary, withItems(rest, mpText(s.alias, goodFormValue(s, 0)))...))
		b.add(s.alias+": multipart repeated", mp, mpJoin(formBoundary, withItems(rest, mpText(s.alias, "bad"), mpText(s.alias, goodFormValue(s, 1)))...))
	}
}

func (b *formBuilder) extras() {
	ct := formHeaders(formURLEncoded)
	if !b.hasFile() {
		all := b.pairs(-1, true)
		b.add("extra field", ct, formEncode(withItems(all, [2]string{"x", "1"})))
		b.add("extra field first", ct, formEncode(withItems([][2]string{{"x", "1"}}, all...)))
		b.add("extra empty name", ct, formEncode(withItems(all, [2]string{"", "v"})))
		b.add("extra escaped name", ct, formEncode(withItems(all, [2]string{"%C3%A9", "1"})))
		b.add("extra bare name", ct, "x&"+formEncode(all))
		b.add("extra repeated", ct, formEncode(withItems(all, [2]string{"x", "1"}, [2]string{"x", "2"})))
		b.add("extra upper case name", ct, formEncode(withItems(all, [2]string{strings.ToUpper(url.QueryEscape(b.slots[0].alias)), "v"})))
		b.add("extra only", ct, "x=1")
	}
	mp := formHeaders(formMultipart)
	b.add("extra multipart part", mp, mpJoin(formBoundary, withItems(b.parts(-1), mpText("x", "1"))...))
	b.add("extra multipart file", mp, mpJoin(formBoundary, withItems(b.parts(-1), mpFile("x", "x.bin", "1"))...))
}

func (b *formBuilder) pathOrder() {
	if len(b.params) == 0 {
		return
	}
	bad := map[string]string{}
	for k := range b.params {
		bad[k] = "short"
	}
	b.addWith("path fault with empty form", bad, formHeaders(formURLEncoded), "")
	b.addWith("path fault with multipart without boundary", bad, formHeaders("multipart/form-data"), "x")
	b.addWith("path fault with malformed multipart", bad, formHeaders(formMultipart), "x")
	b.addWith("path fault with valid multipart", bad, formHeaders(formMultipart), mpJoin(formBoundary, b.parts(-1)...))
}

func (b *formBuilder) contentTypes() {
	body := formEncode(b.pairs(-1, true))
	for _, v := range []struct{ label, value string }{
		{"text/plain", "text/plain"},
		{"application/json", "application/json"},
		{"upper case", "APPLICATION/X-WWW-FORM-URLENCODED"},
		{"padded", " application/x-www-form-urlencoded "},
		{"no-break space", "\xa0application/x-www-form-urlencoded"},
		{"unit separator", "application/x-www-form-urlencoded\x1f"},
		{"next line", "\x85application/x-www-form-urlencoded"},
		{"trailing semicolon", "application/x-www-form-urlencoded;"},
		{"charset", "application/x-www-form-urlencoded; charset=utf-8"},
		{"charset latin-1", "application/x-www-form-urlencoded; charset=latin-1"},
		{"spaced semicolon", "application/x-www-form-urlencoded ; charset=utf-8"},
		{"mixed case with parameter", "Application/X-WWW-Form-Urlencoded; charset=utf-8"},
		{"equals in type", "application/x-www-form-urlencoded=; x"},
		{"upper case type with equals", "APPLICATION/X-WWW-FORM-URLENCODED=1; x"},
		{"quoted type", "\"application/x-www-form-urlencoded\"; a=b"},
		{"longer type", "application/x-www-form-urlencodedx"},
		{"nul", "application/x-www-form-urlencoded\x00"},
		{"multipart", "multipart/form-data"},
	} {
		b.add("content type "+v.label, formHeaders(v.value), body)
	}
	b.add("content type absent", nil, body)
	b.add("content type empty", formHeaders(""), body)
	b.add("content type urlencoded then text", [][2]string{{"Content-Type", formURLEncoded}, {"content-type", "text/plain"}}, body)
	b.add("content type text then urlencoded", [][2]string{{"Content-Type", "text/plain"}, {"content-type", formURLEncoded}}, body)

	parts := b.parts(-1)
	standard := mpJoin(formBoundary, parts...)
	for _, v := range []struct{ label, value, boundary string }{
		{"no boundary", "multipart/form-data", formBoundary},
		{"upper case without parameters", "MULTIPART/FORM-DATA", formBoundary},
		{"mixed case with boundary", "Multipart/Form-Data; boundary=" + formBoundary, formBoundary},
		{"empty boundary", "multipart/form-data; boundary=", ""},
		{"quoted boundary", "multipart/form-data; boundary=\"" + formBoundary + "\"", formBoundary},
		{"angle bracket boundary", "multipart/form-data; boundary=<" + formBoundary + ">", formBoundary},
		{"upper case key", "multipart/form-data; BOUNDARY=" + formBoundary, formBoundary},
		{"no space", "multipart/form-data;boundary=" + formBoundary, formBoundary},
		{"quoted semicolon", "multipart/form-data; boundary=\"pyval;Form\"", "pyval;Form"},
		{"quoted escaped quote", "multipart/form-data; boundary=\"pyval\\\"Form\"", "pyval\"Form"},
		{"rfc 2231 extended", "multipart/form-data; boundary*=utf-8''" + formBoundary, formBoundary},
		{"rfc 2231 extended escaped", "multipart/form-data; boundary*=utf-8''pyval%46ormBoundary", formBoundary},
		{"rfc 2231 continuations", "multipart/form-data; boundary*0=pyvalForm; boundary*1=Boundary", formBoundary},
		{"rfc 2231 continuations reversed", "multipart/form-data; boundary*1=Boundary; boundary*0=pyvalForm", formBoundary},
		{"rfc 2231 mixed continuations", "multipart/form-data; boundary*=pyval; boundary*0=FormBoundary", formBoundary},
		{"rfc 2231 long number", "multipart/form-data; boundary*" + strings.Repeat("1", maxIntStrDigits+1) + "=" + formBoundary, formBoundary},
		{"two boundaries", "multipart/form-data; boundary=" + formBoundary + "; boundary=other", formBoundary},
		{"two boundaries last used", "multipart/form-data; boundary=other; boundary=" + formBoundary, formBoundary},
		{"unterminated quote", "multipart/form-data; boundary=\"" + formBoundary + "; x=1", formBoundary},
	} {
		body := standard
		if v.boundary != formBoundary {
			body = mpJoin(v.boundary, parts...)
		}
		b.add("multipart type "+v.label, formHeaders(v.value), body)
	}
}

func (b *formBuilder) jsonBodies() {
	doc := pyjson.NewOrderedMap()
	for _, s := range b.slots {
		doc.Set(s.alias, pyjson.String(goodFormValue(s, 0)))
	}
	text, err := pyjson.Dumps(doc)
	if err != nil {
		panic(err)
	}
	b.add("json body", formHeaders("application/json"), string(text))
	b.add("json body without content type", nil, string(text))
	b.add("json body as urlencoded", formHeaders(formURLEncoded), string(text))
	b.add("json body as multipart", formHeaders(formMultipart), string(text))
}

func (b *formBuilder) urlencodedEdges() {
	ct := formHeaders(formURLEncoded)
	all := b.pairs(-1, true)
	body := formEncode(all)
	first := all[0]
	rest := formEncode(all[1:])
	joinRest := func(head string) string { return strings.TrimSuffix(head+"&"+rest, "&") }
	semis := make([]string, len(all))
	for i, p := range all {
		semis[i] = p[0] + "=" + p[1]
	}
	b.add("semicolon separators", ct, strings.Join(semis, ";"))
	b.add("semicolon before ampersand", ct, first[0]+"="+first[1]+";x=1&"+rest+"&y=2")
	b.add("separators around", ct, "&&"+body+"&&")
	b.add("semicolons around", ct, ";"+body+";")
	b.add("only equals", ct, "=")
	b.add("only ampersand", ct, "&")
	b.add("only semicolon", ct, ";")
	b.add("lone percent", ct, "%")
	b.add("double equals", ct, joinRest(first[0]+"=="+first[1]))
	b.add("escaped equals in name", ct, joinRest(first[0]+"%3D="+first[1]))
	b.add("plus in name", ct, joinRest(strings.ReplaceAll(first[0], "_", "+")+"="+first[1]))
	b.add("escaped name", ct, joinRest("%"+strings.ToUpper(strconv.FormatInt(int64(first[0][0]), 16))+first[0][1:]+"="+first[1]))
	b.add("byte order mark", ct, "\xef\xbb\xbf"+body)
	b.add("trailing crlf", ct, body+"\r\n")
	b.add("many extra fields", ct, strings.Repeat("x=1&", 20000)+body)
	b.add("many repeats", ct, strings.Repeat(first[0]+"=bad&", 5000)+body)
}

func (b *formBuilder) multipartEdges() {
	ct := formHeaders(formMultipart)
	parts := b.parts(-1)
	full := mpJoin(formBoundary, parts...)
	closing := "--" + formBoundary + "--\r\n"
	firstName := b.slots[0].alias
	firstValue := goodFormValue(b.slots[0], 0)
	replaceFirst := func(part string) string {
		return mpJoin(formBoundary, withItems([]string{part}, parts[1:]...)...)
	}
	b.add("truncated before closing boundary", ct, strings.TrimSuffix(full, closing))
	b.add("truncated inside last part", ct, full[:len(full)-len(closing)-4])
	b.add("leading newlines", ct, "\r\n\r\n"+full)
	b.add("preamble", ct, "preamble\r\n"+full)
	b.add("epilogue", ct, full+"epilogue text")
	b.add("crlf after closing boundary", ct, full+"\r\n\r\n")
	b.add("lf line breaks", ct, strings.ReplaceAll(full, "\r\n", "\n"))
	b.add("header without space", ct, strings.ReplaceAll(full, "Content-Disposition: ", "Content-Disposition:"))
	b.add("header value with two spaces", ct, strings.ReplaceAll(full, "Content-Disposition: ", "Content-Disposition:  "))
	b.add("header name with space", ct, strings.Replace(full, "Content-Disposition:", "Content Disposition:", 1))
	b.add("empty header name", ct, replaceFirst(": x\r\n"+parts[0]))
	b.add("part without disposition", ct, replaceFirst("Content-Type: text/plain\r\n\r\n"+firstValue))
	b.add("disposition without name", ct, replaceFirst("Content-Disposition: form-data; filename=\"a.txt\"\r\n\r\n"+firstValue))
	b.add("empty disposition", ct, replaceFirst("Content-Disposition: \r\n\r\n"+firstValue))
	b.add("unquoted name", ct, replaceFirst("Content-Disposition: form-data; name="+firstName+"\r\n\r\n"+firstValue))
	b.add("name without parameters", ct, replaceFirst("Content-Disposition: name\r\n\r\n"+firstValue))
	b.add("escaped quote in name", ct, replaceFirst("Content-Disposition: form-data; name=\"a\\\"b\"\r\n\r\n"+firstValue))
	b.add("rfc 2231 name", ct, replaceFirst("Content-Disposition: form-data; name*=utf-8''"+firstName+"\r\n\r\n"+firstValue))
	b.add("rfc 2231 escaped name", ct, replaceFirst("Content-Disposition: form-data; name*=utf-8''%C3%A9\r\n\r\n"+firstValue))
	b.add("upper case disposition", ct, replaceFirst("CONTENT-DISPOSITION: FORM-DATA; NAME=\""+firstName+"\"\r\n\r\n"+firstValue))
	b.add("empty filename", ct, replaceFirst(mpFile(firstName, "", firstValue)))
	b.add("rfc 2231 filename", ct, replaceFirst("Content-Disposition: form-data; name=\""+firstName+"\"; filename*=UTF-8''%C3%A9.txt\r\n\r\n"+firstValue))
	b.add("windows path filename", ct, replaceFirst(mpFile(firstName, `C:\dir\a.txt`, firstValue)))
	b.add("part charset header", ct, replaceFirst("Content-Disposition: form-data; name=\""+firstName+"\"\r\nContent-Type: text/plain; charset=latin-1\r\n\r\n"+firstValue+"\xe9"))
	b.add("partial boundary in data", ct, replaceFirst(mpText(firstName, firstValue+"\r\n--"+formBoundary[:6])))
	b.add("boundary prefix in data", ct, replaceFirst(mpText(firstName, firstValue+"\r\n--"+formBoundary+"X")))
	b.add("boundary with dash in data", ct, replaceFirst(mpText(firstName, firstValue+"\r\n--"+formBoundary+"-x")))
	b.add("duplicate parts", ct, mpJoin(formBoundary, withItems(parts, parts...)...))
	b.add("folded header", ct, replaceFirst("Content-Disposition: form-data;\r\n name=\""+firstName+"\"\r\n\r\n"+firstValue))
	b.add("closing boundary only", ct, closing)
	b.add("text before boundary on same line", ct, "x--"+formBoundary+"\r\n"+parts[0]+"\r\n"+closing)
}

func (b *formBuilder) charsets() {
	latin, utf8Text := "\xe9t\xe9", "\xc3\xa9t\xc3\xa9"
	bom := "\xef\xbb\xbf" + utf8Text
	i := b.textSlot()
	show := b.slots[i].kind == "str"
	for _, v := range []struct{ charset, data string }{
		{"utf-8", utf8Text}, {"utf-8", latin}, {"UTF8", utf8Text}, {"utf 8", utf8Text}, {"u8", utf8Text},
		{"cp65001", latin}, {"utf-8-sig", bom}, {"UTF_8_SIG", latin}, {"latin-1", utf8Text},
		{"ISO-8859-1", utf8Text}, {"l1", utf8Text}, {"ascii", latin}, {"us-ascii", utf8Text}, {"646", utf8Text},
		{"nope", utf8Text}, {"", utf8Text}, {"hex", utf8Text}, {"base64", utf8Text}, {"rot13", utf8Text},
		{"aliases", utf8Text}, {"mbcs", utf8Text}, {"charmap", utf8Text}, {"undefined", utf8Text},
		{"utf.8", utf8Text}, {"utf\x008", utf8Text}, {"\"utf-8\"", latin}, {"latin\xe91", utf8Text},
		{"utf-8x", utf8Text}, {"iso_8859_1_1987", utf8Text},
	} {
		var parts []string
		if show {
			parts = withItems(b.parts(i), mpText(b.slots[i].alias, v.data))
		} else {
			// No text field to echo the value: decode a forbidden name instead.
			parts = withItems(b.parts(-1), mpText("n"+v.data, "1"))
		}
		b.add("charset "+strconv.Quote(v.charset)+" with "+strconv.Quote(v.data), formHeaders(formMultipart+"; charset="+v.charset), mpJoin(formBoundary, parts...))
	}
}

func (b *formBuilder) limits() {
	ct := formHeaders(formMultipart)
	i := b.textSlot()
	name := b.slots[i].alias
	rest := b.parts(i)
	b.add("part at size limit", ct, mpJoin(formBoundary, withItems(rest, mpText(name, strings.Repeat("a", multipartMaxPartSize)))...))
	b.add("part over size limit", ct, mpJoin(formBoundary, withItems(rest, mpText(name, strings.Repeat("a", multipartMaxPartSize+1)))...))
	b.add("file part over size limit", ct, mpJoin(formBoundary, withItems(rest, mpFile(name, "big.bin", strings.Repeat("a", multipartMaxPartSize+1)))...))
	fields, files := 0, 0
	for _, s := range b.slots {
		if s.kind == "file" {
			files++
		} else {
			fields++
		}
	}
	all := b.parts(-1)
	many := func(n int, part string) []string {
		out := withItems(all)
		for k := 0; k < n; k++ {
			out = append(out, part)
		}
		return out
	}
	b.add("fields at count limit", ct, mpJoin(formBoundary, many(multipartMaxFields-fields, mpText("x", "1"))...))
	b.add("fields over count limit", ct, mpJoin(formBoundary, many(multipartMaxFields-fields+1, mpText("x", "1"))...))
	b.add("files at count limit", ct, mpJoin(formBoundary, many(multipartMaxFiles-files, mpFile("f", "f.bin", "1"))...))
	b.add("files over count limit", ct, mpJoin(formBoundary, many(multipartMaxFiles-files+1, mpFile("f", "f.bin", "1"))...))
}

func (b *formBuilder) fileCases() {
	ct := formHeaders(formMultipart)
	file := func(disposition, headers, data string) string {
		return "Content-Disposition: form-data; name=\"upload\"" + disposition + "\r\n" + headers + "\r\n" + data
	}
	note := mpText("note", "hi")
	b.add("file with note", ct, mpJoin(formBoundary, file("; filename=\"photo.png\"", "Content-Type: image/png\r\n", "\x89PNG\r\n\x1a\n data"), note))
	b.add("file without content type", ct, mpJoin(formBoundary, file("; filename=\"a.bin\"", "", "abc")))
	b.add("file with extra headers", ct, mpJoin(formBoundary, file("; filename=\"a.bin\"", "X-Extra: one\r\nContent-Type: text/plain\r\ncontent-type: image/png\r\n", "abc")))
	b.add("file empty content", ct, mpJoin(formBoundary, file("; filename=\"empty.bin\"", "", "")))
	b.add("file empty filename", ct, mpJoin(formBoundary, file("; filename=\"\"", "", "abc")))
	b.add("file rfc 2231 filename", ct, mpJoin(formBoundary, file("; filename*=UTF-8''%C3%A9.png", "", "abc")))
	b.add("file latin-1 filename", ct, mpJoin(formBoundary, file("; filename=\"\xe9.png\"", "", "abc")))
	b.add("file utf-8 filename", ct, mpJoin(formBoundary, file("; filename=\"\xc3\xa9.png\"", "", "abc")))
	b.add("file unc path filename", ct, mpJoin(formBoundary, file(`; filename="\\server\share\a.png"`, "", "abc")))
	b.add("file data with crlf", ct, mpJoin(formBoundary, file("; filename=\"a.txt\"", "", "line one\r\nline two\r\n")))
	b.add("file repeated", ct, mpJoin(formBoundary, file("; filename=\"first.bin\"", "", "1"), file("; filename=\"second.bin\"", "", "2")))
	b.add("file then text of the same name", ct, mpJoin(formBoundary, file("; filename=\"first.bin\"", "", "1"), mpText("upload", "2")))
	b.add("note as file", ct, mpJoin(formBoundary, file("; filename=\"a.bin\"", "", "abc"), mpFile("note", "n.txt", "hi")))
	b.add("file huge", ct, mpJoin(formBoundary, file("; filename=\"big.bin\"", "", strings.Repeat("z", multipartMaxPartSize+1))))
	b.add("file charset latin-1 filename", formHeaders(formMultipart+"; charset=latin-1"), mpJoin(formBoundary, file("; filename=\"\xc3\xa9.png\"", "", "abc")))
}
