package guest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"mobile/services/core/internal/domain/ledger"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_*.json is rendered by scripts/render_guest_web_goldens.py
// inside the parity API image: the real view builders, the real ApiService
// methods and route handlers over a stub repository, and the real
// Jinja2Templates object of app/api/routes/guests.py. Every case is replayed
// here; a body is compared byte for byte after joining its line table pieces.

type replayFn func(args map[string]any) (any, error)

// pyValue turns a decoded oracle value into this package's Python values.
func pyValue(v any) any {
	switch x := v.(type) {
	case oracletest.BigInt:
		n, _ := new(big.Int).SetString(string(x), 10)
		return n
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = pyValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = pyValue(item)
		}
		return out
	}
	return v
}

// oracleValue turns a Go answer into the shape oracletest decodes.
func oracleValue(v any) any {
	switch x := v.(type) {
	case *big.Int:
		if x.IsInt64() {
			return x.Int64()
		}
		return oracletest.BigInt(x.String())
	case int:
		return int64(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = oracleValue(item)
		}
		return out
	case map[string]any:
		if x == nil {
			return nil
		}
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = oracleValue(item)
		}
		return out
	}
	return v
}

func errorClass(err error) (class string, code any, message string, ok bool) {
	var view *ViewError
	var objection *ObjectionError
	var refused *ledger.LedgerError
	var py *PyError
	switch {
	case errors.As(err, &view):
		return "GuestViewError", view.Code, view.Code, true
	case errors.As(err, &objection):
		return "ObjectionError", objection.Code, objection.Code, true
	case errors.As(err, &refused):
		return "LedgerError", refused.Code, refused.Code, true
	case errors.As(err, &py):
		return py.Class, nil, py.Message, true
	}
	return "", nil, "", false
}

func lineTable(t *testing.T, file oracletest.File) []string {
	t.Helper()
	if file.Module != "pages" {
		return nil
	}
	var raw struct {
		Lines []string `json:"lines"`
	}
	if err := json.Unmarshal(file.Constants, &raw); err != nil {
		t.Fatalf("%s: line table: %v", file.Path, err)
	}
	out := make([]string, len(raw.Lines))
	for i, piece := range raw.Lines {
		text, err := oracletest.Text(piece)
		if err != nil {
			t.Fatalf("%s: line %d: %v", file.Path, i, err)
		}
		out[i] = text
	}
	return out
}

// expandBodies replaces every "body" list of line indices with its text.
func expandBodies(t *testing.T, v any, lines []string) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			if indices, isList := item.([]any); k == "body" && isList {
				var b strings.Builder
				for _, index := range indices {
					i, ok := index.(int64)
					if !ok || i < 0 || int(i) >= len(lines) {
						t.Fatalf("bad line index %v", index)
					}
					b.WriteString(lines[i])
				}
				out[k] = b.String()
				continue
			}
			out[k] = expandBodies(t, item, lines)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = expandBodies(t, item, lines)
		}
		return out
	}
	return v
}

func firstDiff(want, got any) string {
	w, _ := json.Marshal(want)
	g, _ := json.Marshal(got)
	a, b := string(w), string(g)
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(i-80, 0)
	return fmt.Sprintf("at byte %d:\n    Python ...%s\n    Go     ...%s", i, a[lo:min(i+80, len(a))], b[lo:min(i+80, len(b))])
}

func safely(replay replayFn, args map[string]any) (got any, err error) {
	defer func() {
		if p := recover(); p != nil {
			got, err = nil, fmt.Errorf("panic: %v", p)
		}
	}()
	return replay(args)
}

// agree replays every case and fails on any disagreement, after logging each
// function's case and mismatch counts.
func agree(t *testing.T, files []oracletest.File, replays map[string]replayFn) map[string]int {
	t.Helper()
	cases, mismatches := map[string]int{}, map[string]int{}
	total, failed := 0, 0
	for _, file := range files {
		lines := lineTable(t, file)
		for _, c := range file.Cases {
			want, raised, err := c.Outcome()
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			args, err := c.PlainArgs()
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			replay := replays[c.Fn]
			if replay == nil {
				t.Fatalf("%s %s: no replay for %s", file.Mode, c.Name, c.Fn)
			}
			got, goErr := safely(replay, pyValue(args).(map[string]any))
			ok := false
			if raised != nil {
				class, code, message, known := errorClass(goErr)
				ok = known && class == raised.Type && reflect.DeepEqual(code, raised.Code) &&
					(class != "KeyError" || message == raised.Message)
			} else {
				want = expandBodies(t, want, lines)
				ok = goErr == nil && reflect.DeepEqual(want, got)
			}
			cases[c.Fn]++
			total++
			if !ok {
				mismatches[c.Fn]++
				failed++
				if failed <= 8 {
					if raised != nil || goErr != nil {
						t.Errorf("%s %s %s: Python raised %+v, Go %v (%T), Go answer %.300v", file.Mode, c.Name, c.Fn, raised, goErr, goErr, got)
					} else {
						t.Errorf("%s %s %s: %s", file.Mode, c.Name, c.Fn, firstDiff(want, got))
					}
				}
			}
		}
	}
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("%s: %d Python cases, %d mismatches", name, cases[name], mismatches[name])
	}
	if failed > 0 {
		t.Fatalf("%d mismatches over %d Python cases", failed, total)
	}
	return cases
}

func requireFns(t *testing.T, got map[string]int, fns ...string) {
	t.Helper()
	for _, fn := range fns {
		if got[fn] == 0 {
			t.Fatalf("no Python cases for %s", fn)
		}
	}
}

func envelopeArg(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func optionalString(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func problemOf(r *Refusal) any {
	if r == nil {
		return nil
	}
	return map[string]any{"status": int64(r.Status), "code": r.Code, "detail": r.Detail}
}

func viewOrNone(view map[string]any) any {
	if view == nil {
		return nil
	}
	return oracleValue(view)
}

func shape(r Response) map[string]any {
	headers := make([]any, len(r.Headers))
	for i, pair := range r.Headers {
		headers[i] = []any{pair[0], pair[1]}
	}
	return map[string]any{"status": int64(r.Status), "headers": headers, "body": string(r.Body)}
}

var viewReplays = map[string]replayFn{
	"format_vnd": func(args map[string]any) (any, error) {
		return FormatVND(args["amount_vnd"])
	},
	"format_vnd_digits": func(args map[string]any) (any, error) {
		n, ok := new(big.Int).SetString(strings.Repeat(args["digit"].(string), int(args["count"].(int64))), 10)
		if !ok {
			return nil, fmt.Errorf("bad digits")
		}
		text, err := FormatVND(n)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(text))
		return map[string]any{"len": int64(utf8.RuneCountInString(text)), "sha256": hex.EncodeToString(sum[:])}, nil
	},
	"build_guest_view": func(args map[string]any) (any, error) {
		view, err := BuildGuestView(args["envelope"].(map[string]any))
		return viewOrNone(view), err
	},
	"build_not_me_view": func(args map[string]any) (any, error) {
		view, err := BuildNotMeView(args["envelope"].(map[string]any))
		return viewOrNone(view), err
	},
	"build_wrong_amount_view": func(args map[string]any) (any, error) {
		view, err := BuildWrongAmountView(args["envelope"].(map[string]any), args["obligation_id"].(string))
		return viewOrNone(view), err
	},
}

func viewStep(step func(token string, envelope map[string]any, found bool) (map[string]any, *Refusal, error)) replayFn {
	return func(args map[string]any) (any, error) {
		env, found := envelopeArg(args["envelope"])
		view, refusal, err := step(args["token"].(string), env, found)
		if err != nil {
			return nil, err
		}
		return map[string]any{"calls": []any{"get_guest_envelope"}, "problem": problemOf(refusal), "view": viewOrNone(view)}, nil
	}
}

var stepReplays = map[string]replayFn{
	"guest_view":  viewStep(GuestView),
	"not_me_view": viewStep(NotMeView),
	"wrong_amount_view": func(args map[string]any) (any, error) {
		return viewStep(func(token string, envelope map[string]any, found bool) (map[string]any, *Refusal, error) {
			return WrongAmountView(token, envelope, found, args["obligation_id"].(string))
		})(args)
	},
	"record_objection": func(args map[string]any) (any, error) {
		calls := []any{}
		kind := args["kind"].(string)
		load := func() (map[string]any, bool, error) {
			calls = append(calls, "get_guest_envelope")
			env, found := envelopeArg(args["envelope"])
			return env, found, nil
		}
		refusal, err := RecordObjection(args["token"].(string), kind, optionalString(args["obligation_id"]), optionalString(args["reason"]), load)
		if err != nil {
			return nil, err
		}
		if refusal == nil {
			calls = append(calls, []any{"save_guest_objection", kind, args["obligation_id"], args["reason"]})
		}
		return map[string]any{"calls": calls, "problem": problemOf(refusal)}, nil
	},
	"report_payment": func(args map[string]any) (any, error) {
		status, calls, refusal, err := reportPayment(args)
		if err != nil {
			return nil, err
		}
		var answer any
		if refusal == nil {
			answer = status
		}
		return map[string]any{"calls": calls, "problem": problemOf(refusal), "status": answer}, nil
	},
}

// reportPayment composes ApiService.report_payment from its pure steps.
func reportPayment(args map[string]any) (string, []any, *Refusal, error) {
	calls := []any{"get_payment_report_target"}
	var target *PaymentReportTarget
	var amount money.VND
	if m, ok := args["target"].(map[string]any); ok {
		target = &PaymentReportTarget{ActiveCapability: m["active_capability"].(bool), ReportsUsed: m["reports_used"].(int64)}
		amount = money.VND(m["amount_vnd"].(int64))
	}
	refusal, err := PaymentReportGate(args["token"].(string), target)
	if refusal != nil || err != nil {
		return "", calls, refusal, err
	}
	calls = append(calls, "save_payment_report")
	if code, ok := args["conflict"].(string); ok {
		return "", calls, PaymentReportConflict(code), nil
	}
	var receipts []money.VND
	if list, ok := args["receipts"].([]any); ok {
		for _, r := range list {
			receipts = append(receipts, money.VND(r.(int64)))
		}
	}
	status, err := PaymentReportStatus(amount, receipts)
	return status, calls, nil, err
}

// canonicalUUID is str(uuid.UUID(s)) for the spellings the route cases use.
func canonicalUUID(s string) string {
	h := strings.ReplaceAll(strings.ReplaceAll(s, "urn:", ""), "uuid:", "")
	h = strings.ToLower(strings.ReplaceAll(strings.Trim(h, "{}"), "-", ""))
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

var pageReplays = map[string]replayFn{
	"template_response": func(args map[string]any) (any, error) {
		r, err := TemplateResponse(args["name"].(string), args["context"].(map[string]any), int(args["status_code"].(int64)))
		if err != nil {
			return nil, err
		}
		return shape(r), nil
	},
	"link_broken_page": func(map[string]any) (any, error) {
		r, err := LinkBrokenPage()
		if err != nil {
			return nil, err
		}
		return shape(r), nil
	},
	"see_other":    func(args map[string]any) (any, error) { return shape(SeeOther(args["url"].(string))), nil },
	"server_error": func(args map[string]any) (any, error) { return shape(ServerError(args["path"].(string))), nil },
	"accepts_html": func(args map[string]any) (any, error) {
		var values []string
		for _, v := range args["values"].([]any) {
			values = append(values, v.(string))
		}
		return AcceptsHTML(values), nil
	},
	"link_broken_applies": func(args map[string]any) (any, error) {
		return LinkBrokenApplies(args["code"].(string), args["url_path"].(string)), nil
	},
	"route": replayRoute,
}

// replayRoute composes a guests.py handler from the package's pieces, making
// the repository calls the handler makes, in its order.
func replayRoute(args map[string]any) (any, error) {
	token := args["token"].(string)
	calls := []any{}
	read := func() (map[string]any, bool) {
		calls = append(calls, "get_guest_envelope")
		return envelopeArg(args["envelope"])
	}
	load := func() (map[string]any, bool, error) {
		env, found := read()
		return env, found, nil
	}
	done := func(refusal *Refusal, response any) (any, error) {
		return map[string]any{"calls": calls, "problem": problemOf(refusal), "response": response}, nil
	}
	page := func(view map[string]any, render func(map[string]any, string) (Response, error)) (any, error) {
		r, err := render(view, token)
		if err != nil {
			return nil, err
		}
		return done(nil, shape(r))
	}
	switch args["route"].(string) {
	case "guest_page", "not_me_page":
		step, render := GuestView, GuestPage
		if args["route"] == "not_me_page" {
			step, render = NotMeView, NotMePage
		}
		env, found := read()
		view, refusal, err := step(token, env, found)
		if refusal != nil || err != nil {
			return errOr(done, refusal, err)
		}
		return page(view, render)
	case "not_me_submit":
		env, found := read()
		seen, refusal, err := NotMeView(token, env, found)
		if refusal != nil || err != nil {
			return errOr(done, refusal, err)
		}
		if refusal, err = RecordObjection(token, "not_me", nil, nil, load); refusal != nil || err != nil {
			return errOr(done, refusal, err)
		}
		calls = append(calls, []any{"save_guest_objection", "not_me", nil, nil})
		view, err := NotMeSubmittedView(seen)
		if err != nil {
			return nil, err
		}
		return page(view, NotMePage)
	case "wrong_amount_page":
		id, given := args["obligation_id"].(string)
		if !given {
			env, found := read()
			view, refusal, err := GuestView(token, env, found)
			if refusal != nil || err != nil {
				return errOr(done, refusal, err)
			}
			if id, refusal, err = DefaultObligationID(view); refusal != nil || err != nil {
				return errOr(done, refusal, err)
			}
		}
		env, found := read()
		view, refusal, err := WrongAmountView(token, env, found, id)
		if refusal != nil || err != nil {
			return errOr(done, refusal, err)
		}
		return page(view, WrongAmountPage)
	case "wrong_amount_submit", "request_evidence":
		raw := args["obligation_id"].(string)
		canonical := canonicalUUID(raw)
		kind, reason, location := "wrong_amount", optionalString(args["reason"]), GuestPageURL(token)
		if args["route"] == "request_evidence" {
			kind, reason, location = "evidence_request", nil, EvidenceRequestedURL(token, raw)
		}
		refusal, err := RecordObjection(token, kind, &canonical, reason, load)
		if refusal != nil || err != nil {
			return errOr(done, refusal, err)
		}
		var recorded any
		if reason != nil {
			recorded = *reason
		}
		calls = append(calls, []any{"save_guest_objection", kind, canonical, recorded})
		return done(nil, shape(SeeOther(location)))
	case "report_payment":
		status, stepCalls, refusal, err := reportPayment(args)
		calls = stepCalls
		if refusal != nil || err != nil {
			return errOr(done, refusal, err)
		}
		var accept []string
		if value, ok := args["accept"].(string); ok {
			accept = []string{value}
		}
		if AcceptsHTML(accept) {
			return done(nil, shape(SeeOther(GuestPageURL(token))))
		}
		return done(nil, map[string]any{"model_status": status})
	}
	return nil, fmt.Errorf("unknown route %v", args["route"])
}

func errOr(done func(*Refusal, any) (any, error), refusal *Refusal, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return done(refusal, nil)
}

func TestViewsMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_views*.json")
	oracletest.CheckShards(t, files, "views")
	got := agree(t, files, viewReplays)
	requireFns(t, got, "format_vnd", "format_vnd_digits", "build_guest_view", "build_not_me_view", "build_wrong_amount_view")
}

func TestStepsMatchPython(t *testing.T) {
	got := agree(t, oracletest.Load(t, "testdata/python_steps.json"), stepReplays)
	requireFns(t, got, "guest_view", "not_me_view", "wrong_amount_view", "record_objection", "report_payment")
}

func TestPagesMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_pages*.json")
	oracletest.CheckShards(t, files, "pages")
	got := agree(t, files, pageReplays)
	requireFns(t, got, "template_response", "link_broken_page", "see_other", "server_error", "accepts_html", "link_broken_applies", "route")
}
