package limit

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/httpapi/problem"
)

// testdata/python_limits.json is rendered by scripts/render_limit_goldens.py
// from the real Python classes and create_app() in the pinned API image.

type oracleFile struct {
	Actor     []actorSequence   `json:"actor"`
	Address   []addressSequence `json:"address"`
	Responses []responseCase    `json:"responses"`
}

type actorSequence struct {
	Name          string       `json:"name"`
	Limit         int          `json:"limit"`
	WindowSeconds int          `json:"window_seconds"`
	MinSweepAt    int          `json:"min_sweep_at"`
	T0            float64      `json:"t0"`
	Events        [][2]float64 `json:"events"`
	Allowed       []int        `json:"allowed"`
	Tracked       []int        `json:"tracked"`
}

type addressSequence struct {
	Name          string       `json:"name"`
	Limit         int          `json:"limit"`
	WindowSeconds float64      `json:"window_seconds"`
	MaxTracked    int          `json:"max_tracked"`
	Events        [][2]float64 `json:"events"`
	Allowed       []int        `json:"allowed"`
	Tracked       []int        `json:"tracked"`
}

type responseCase struct {
	Case     string      `json:"case"`
	State    string      `json:"state"`
	Method   string      `json:"method"`
	Path     string      `json:"path"`
	Admitted int         `json:"admitted"`
	Status   int         `json:"status"`
	Headers  [][2]string `json:"headers"`
	Body     string      `json:"body"`
}

func loadOracle(t *testing.T) oracleFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/python_limits.json")
	if err != nil {
		t.Fatal(err)
	}
	var out oracleFile
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, seq := range out.Actor {
		if len(seq.Events) != len(seq.Allowed) || len(seq.Events) != len(seq.Tracked) {
			t.Fatalf("%s: ragged sequence", seq.Name)
		}
	}
	for _, seq := range out.Address {
		if len(seq.Events) != len(seq.Allowed) || len(seq.Events) != len(seq.Tracked) {
			t.Fatalf("%s: ragged sequence", seq.Name)
		}
	}
	return out
}

// requireAtLeast keeps a regenerated corpus from quietly losing the cases
// that make this replay worth anything.
func requireAtLeast(t *testing.T, what string, got, floor int) {
	t.Helper()
	if got < floor {
		t.Errorf("corpus exercises %s %d times, want at least %d", what, got, floor)
	}
}

func TestOracleActorSequencesMatchPython(t *testing.T) {
	oracle := loadOracle(t)
	refusal := problem.Problem{Status: 429, Code: "c", Detail: "m"}
	var events, mismatches, refusals, sweeps, atEdge, nearEdge, backwards, realBoundSweeps int
	for _, seq := range oracle.Actor {
		now := seq.T0
		l := newActorWindow[int](
			ActorConfig{Limit: seq.Limit, WindowSeconds: seq.WindowSeconds, Code: "c", Message: "m"},
			func() float64 { return now },
			seq.MinSweepAt,
		)
		previous, previousTracked := seq.T0, 0
		for i, event := range seq.Events {
			now = event[0]
			key := int(event[1])
			if entry, ok := l.windows[key]; ok {
				switch gap := now - entry.openedAt; {
				case gap == l.window:
					atEdge++
				case math.Abs(gap-l.window) <= 0.0011:
					nearEdge++
				}
			}
			if now < previous {
				backwards++
			}
			sweepAtBefore := l.sweepAt
			got := l.Check(key)
			events++
			want := seq.Allowed[i] == 1
			tracked := l.Tracked()
			if got.Allowed != want || tracked != seq.Tracked[i] || (!got.Allowed && got.Problem != refusal) {
				mismatches++
				if mismatches <= 5 {
					t.Errorf("%s event %d (t=%v key=%d): go allowed=%v tracked=%d, python allowed=%v tracked=%d",
						seq.Name, i, now, key, got.Allowed, tracked, want, seq.Tracked[i])
				}
			}
			if !want {
				refusals++
			}
			if seq.Tracked[i] < previousTracked {
				sweeps++
			}
			if seq.MinSweepAt == defaultMinSweepAt && l.sweepAt != sweepAtBefore {
				realBoundSweeps++
			}
			previous, previousTracked = now, seq.Tracked[i]
		}
	}
	t.Logf("actor: %d sequences, %d events, %d mismatches; refusals=%d sweeps=%d at-edge=%d near-edge=%d backwards=%d real-bound-sweeps=%d",
		len(oracle.Actor), events, mismatches, refusals, sweeps, atEdge, nearEdge, backwards, realBoundSweeps)
	if mismatches != 0 {
		t.Fatalf("%d of %d events differ from Python", mismatches, events)
	}
	requireAtLeast(t, "events", events, 10_000)
	requireAtLeast(t, "refusals", refusals, 1_000)
	requireAtLeast(t, "sweeps that dropped keys", sweeps, 300)
	requireAtLeast(t, "calls exactly one window after opening", atEdge, 150)
	requireAtLeast(t, "calls within a millisecond of the edge", nearEdge, 300)
	requireAtLeast(t, "backwards clock steps", backwards, 50)
	requireAtLeast(t, "size sweeps at the shipped bound", realBoundSweeps, 2)
}

func TestOracleAddressSequencesMatchPython(t *testing.T) {
	oracle := loadOracle(t)
	var events, mismatches, refusals, clears, atEdge, nearEdge, negative, realBoundClears int
	for _, seq := range oracle.Address {
		now := 0.0
		l := newAddressWindow(
			AddressConfig{Limit: seq.Limit, WindowSeconds: seq.WindowSeconds, Code: "rate_limited", Detail: "d"},
			func() float64 { return now },
			seq.MaxTracked,
		)
		previousTracked := 0
		for i, event := range seq.Events {
			now = event[0]
			caller := "c" + strconv.Itoa(int(event[1]))
			if now != 0 {
				remainder := math.Mod(math.Abs(now), seq.WindowSeconds)
				switch {
				case remainder == 0:
					atEdge++
				case remainder <= 0.0011 || seq.WindowSeconds-remainder <= 0.0011:
					nearEdge++
				}
			}
			if now < 0 {
				negative++
			}
			got := l.Check(caller)
			events++
			want := seq.Allowed[i] == 1
			tracked := l.Tracked()
			if got.Allowed != want || tracked != seq.Tracked[i] {
				mismatches++
				if mismatches <= 5 {
					t.Errorf("%s event %d (t=%v caller=%s): go allowed=%v tracked=%d, python allowed=%v tracked=%d",
						seq.Name, i, now, caller, got.Allowed, tracked, want, seq.Tracked[i])
				}
			}
			if !want {
				refusals++
			}
			if seq.Tracked[i] < previousTracked {
				clears++
				if seq.MaxTracked == defaultMaxTracked && previousTracked > defaultMaxTracked {
					realBoundClears++
				}
			}
			previousTracked = seq.Tracked[i]
		}
	}
	t.Logf("address: %d sequences, %d events, %d mismatches; refusals=%d clears=%d at-edge=%d near-edge=%d negative=%d real-bound-clears=%d",
		len(oracle.Address), events, mismatches, refusals, clears, atEdge, nearEdge, negative, realBoundClears)
	if mismatches != 0 {
		t.Fatalf("%d of %d events differ from Python", mismatches, events)
	}
	requireAtLeast(t, "events", events, 20_000)
	requireAtLeast(t, "refusals", refusals, 1_000)
	requireAtLeast(t, "clears", clears, 300)
	requireAtLeast(t, "calls exactly on a bucket edge", atEdge, 500)
	requireAtLeast(t, "calls within a millisecond of an edge", nearEdge, 500)
	requireAtLeast(t, "negative clock readings", negative, 50)
	requireAtLeast(t, "clears at the shipped bound", realBoundClears, 1)
}

// windowsByState names every window of a Set by its app.state attribute.
func windowsByState(set *Set) (map[string]*ActorWindow[string], map[string]*AddressWindow) {
	return map[string]*ActorWindow[string]{
			"search_limiter":                set.SearchLimiter,
			"itinerary_limiter":             set.ItineraryLimiter,
			"receipt_scan_limiter":          set.ReceiptScanLimiter,
			"chat_expense_limiter":          set.ChatExpenseLimiter,
			"screenshot_scan_limiter":       set.ScreenshotScanLimiter,
			"suggestion_limiter":            set.SuggestionLimiter,
			"contextual_suggestion_limiter": set.ContextualSuggestionLimiter,
			"face_detection_limiter":        set.FaceDetectionLimiter,
			"reel_limiter":                  set.ReelLimiter,
		}, map[string]*AddressWindow{
			"person_id_limit":     set.PersonIDLimit,
			"friend_lookup_limit": set.FriendLookupLimit,
			"otp_request_limit":   set.OTPRequestLimit,
			"otp_verify_limit":    set.OTPVerifyLimit,
			"google_login_limit":  set.GoogleLoginLimit,
		}
}

func headerMap(pairs [][2]string) map[string]string {
	out := map[string]string{}
	for _, pair := range pairs {
		out[strings.ToLower(pair[0])] = pair[1]
	}
	return out
}

func TestOracleShippedRefusalsAreByteIdentical(t *testing.T) {
	oracle := loadOracle(t)
	set := NewSet(func() float64 { return 0 })
	actor, address := windowsByState(set)
	seen := map[string]bool{}
	for _, c := range oracle.Responses {
		seen[c.State] = true
		var check func() Decision
		if w, ok := actor[c.State]; ok {
			check = func() Decision { return w.Check("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee") }
		} else if w, ok := address[c.State]; ok {
			// The raw ASGI scope's client host.
			check = func() Decision { return w.Check("127.0.0.1") }
		} else {
			t.Fatalf("%s: no Go window for app.state.%s", c.Case, c.State)
		}
		for i := 0; i < c.Admitted; i++ {
			if !check().Allowed {
				t.Fatalf("%s: Go refused call %d, Python admitted %d", c.Case, i+1, c.Admitted)
			}
		}
		decision := check()
		if decision.Allowed {
			t.Fatalf("%s: Go admitted call %d, Python refused it", c.Case, c.Admitted+1)
		}
		recorder := httptest.NewRecorder()
		if err := WriteRefusal(recorder, decision); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != c.Status {
			t.Errorf("%s: status %d, python %d", c.Case, recorder.Code, c.Status)
		}
		if got := recorder.Body.String(); got != c.Body {
			t.Errorf("%s: body\n go     %q\n python %q", c.Case, got, c.Body)
		}
		goHeaders := map[string]string{}
		for name, values := range recorder.Header() {
			goHeaders[strings.ToLower(name)] = strings.Join(values, "\x00")
		}
		want := headerMap(c.Headers)
		if len(goHeaders) != len(want) {
			t.Errorf("%s: headers %v, python %v", c.Case, goHeaders, want)
		}
		for name, value := range want {
			if goHeaders[name] != value {
				t.Errorf("%s: header %s=%q, python %q", c.Case, name, goHeaders[name], value)
			}
		}
	}
	// Every window has a captured 429.
	for state := range actor {
		if !seen[state] {
			t.Errorf("no Python 429 captured for %s", state)
		}
	}
	for state := range address {
		if !seen[state] {
			t.Errorf("no Python 429 captured for %s", state)
		}
	}
}

type manifestRow struct {
	ID       string   `json:"id"`
	InMemory []string `json:"in_memory"`
}

// TestManifestLimitersAllHaveAGoWindow ties this package to routes.json: a
// limiter added to Python shows up in `in_memory` and must show up here.
func TestManifestLimitersAllHaveAGoWindow(t *testing.T) {
	raw, err := os.ReadFile("../../ownership/routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Routes []manifestRow `json:"routes"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	rows := manifest.Routes
	if len(rows) < 100 {
		t.Fatalf("manifest has %d routes; the shape changed", len(rows))
	}
	actor, address := windowsByState(NewSet(func() float64 { return 0 }))
	responses := map[string]responseCase{}
	for _, c := range loadOracle(t).Responses {
		responses[c.State] = c
	}
	var named []string
	for _, row := range rows {
		for _, state := range row.InMemory {
			if state == "reason_writer" {
				continue // an AI answer cache that stays in Python
			}
			named = append(named, state)
			_, isActor := actor[state]
			_, isAddress := address[state]
			if !isActor && !isAddress {
				t.Errorf("%s uses app.state.%s, which has no Go window", row.ID, state)
				continue
			}
			c, ok := responses[state]
			if !ok {
				continue
			}
			method, template, _ := strings.Cut(row.ID, " ")
			pattern := "^" + regexp.MustCompile(`\\\{[^}]*\\\}`).ReplaceAllString(regexp.QuoteMeta(template), `[^/]+`) + "$"
			if c.Method != method || !regexp.MustCompile(pattern).MatchString(c.Path) {
				t.Errorf("golden %s drove %s %s, but the manifest puts %s on %s", c.Case, c.Method, c.Path, state, row.ID)
			}
		}
	}
	sort.Strings(named)
	if len(named) != len(actor)+len(address) {
		t.Errorf("manifest names %d limiters %v, Go has %d", len(named), named, len(actor)+len(address))
	}
}
