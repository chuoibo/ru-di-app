package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"mobile/parity/internal/compare"
	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/scenario"
)

const burstScript = `
id: fake/burst
routes: ["POST /things"]
auth_mode: dev
personas: {owner: {}}
steps:
  - id: together
    as: owner
    concurrent: 4
    request: {method: POST, path: "/things?copy={{burst}}", body_raw: '{}'}
  - id: alone
    as: owner
    request: {method: GET, path: /things}
`

func parseBurst(t *testing.T) *scenario.Scenario {
	t.Helper()
	sc, err := scenario.Parse([]byte(burstScript))
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// barrierServer answers a POST only once want POSTs are inside the handler at
// the same time, so copies sent one after another time out instead of passing.
func barrierServer(t *testing.T, want int) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	inside := 0
	full := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mu.Lock()
			inside++
			if inside == want {
				close(full)
			}
			mu.Unlock()
			select {
			case <-full:
			case <-time.After(2 * time.Second):
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"copy":%q}`, r.URL.Query().Get("copy"))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestABurstSendsEveryCopyAtOnce(t *testing.T) {
	run, err := Execute(context.Background(), parseBurst(t), stack(t, "candidate", barrierServer(t, 4)), "t1")
	if err != nil {
		t.Fatal(err)
	}
	burst := run.Steps[0].BurstNorm
	if len(burst) != 4 {
		t.Fatalf("burst has %d responses, want 4", len(burst))
	}
	for i, exchange := range burst {
		want := fmt.Sprintf(`{"copy":"%d"}`, i+1)
		if exchange.Status != http.StatusOK || exchange.Body != want {
			t.Fatalf("response %d = %d %s, want 200 %s", i, exchange.Status, exchange.Body, want)
		}
	}
}

// raceServer lets the first POST to finish win: it answers 201 and every later
// copy 200. Copy n waits n*40ms, or (5-n)*40ms with reverse set, so copy 1 wins
// on one stack and copy 4 on the other. Every answer carries a fresh id and the
// instant it finished.
func raceServer(t *testing.T, reverse bool) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	won := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			_, _ = fmt.Fprintf(w, `{"id":%q}`, uuid4())
			return
		}
		var n int
		_, _ = fmt.Sscan(r.URL.Query().Get("copy"), &n)
		if reverse {
			n = 5 - n
		}
		time.Sleep(time.Duration(n) * 40 * time.Millisecond)
		mu.Lock()
		first := !won
		won = true
		stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
		mu.Unlock()
		if first {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = fmt.Fprintf(w, `{"id":%q,"first":%t,"finished_at":%q}`, uuid4(), first, stamp)
	}))
	t.Cleanup(server.Close)
	return server
}

// Which copy wins a race differs between stacks, so a burst is not compared in
// copy order; and the instants a burst carries share one rank, since which copy
// finished first is scheduling, not behaviour.
func TestBurstsCompareByOutcomeNotByWhichCopyWon(t *testing.T) {
	sc := parseBurst(t)
	ref, err := Execute(context.Background(), sc, stack(t, "reference", raceServer(t, false)), "t1")
	if err != nil {
		t.Fatal(err)
	}
	cand, err := Execute(context.Background(), sc, stack(t, "candidate", raceServer(t, true)), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if diffs := Diff(ref, cand); len(diffs) != 0 {
		t.Fatalf("differences: %+v", diffs)
	}
	statuses := []int{}
	for _, exchange := range cand.Steps[0].BurstNorm {
		statuses = append(statuses, exchange.Status)
	}
	if want := []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusCreated}; !slices.Equal(statuses, want) {
		t.Fatalf("statuses in order %v, want the three losers then the winner", statuses)
	}
}

func burstRun(statuses ...int) *Run {
	step := StepResult{StepID: "together", Burst: make([]httpclient.Response, len(statuses))}
	for _, status := range statuses {
		step.BurstNorm = append(step.BurstNorm, compare.Exchange{Status: status, Header: http.Header{}})
	}
	return &Run{ScenarioID: "fake/burst", Steps: []StepResult{step}}
}

func TestACandidateMayMatchAnyReferenceRunOfARacyBurst(t *testing.T) {
	references := []*Run{burstRun(200, 500), burstRun(200, 200)}
	if racy := RacySteps(references); len(racy) != 1 || racy[0] != "together" {
		t.Fatalf("racy = %v", racy)
	}
	if racy := RacySteps(references[:1]); racy != nil {
		t.Fatalf("one run cannot race with itself: %v", racy)
	}
	if diffs, matched := Closest(references, burstRun(200, 200)); len(diffs) != 0 || matched != 1 {
		t.Fatalf("matched %d with %v", matched, diffs)
	}
	if diffs, matched := Closest(references, burstRun(500, 500)); matched != -1 || len(diffs) != 1 {
		t.Fatalf("an outcome Python never produced matched %d with %v", matched, diffs)
	}
	if diffs := Diff(burstRun(200), burstRun(200, 200)); len(diffs) != 1 || diffs[0].Differences[0].Part != "burst size" {
		t.Fatalf("burst size difference: %+v", diffs)
	}
}
