// Command parity compares the Python reference with a candidate stack.
//
//	parity lint PATH...
//	parity run --auth MODE --reference URL --candidate URL [--reference-dsn DSN --candidate-dsn DSN] [--host H] [--json FILE] PATH...
//	parity canary --auth MODE --reference URL --target URL [--reference-dsn DSN --target-dsn DSN] [--host H] PATH...
//	parity probe --reference URL --candidate URL
//	parity tap --listen ADDR --control ADDR --upstream URL
//
// Exit codes: 0 every step equal, 1 at least one difference, 2 the run could
// not be completed (bad scenario, unreachable stack). A run that could not
// complete is never reported as equal.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/parity/internal/canary"
	"mobile/parity/internal/compare"
	"mobile/parity/internal/dbsnap"
	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/rawprobe"
	"mobile/parity/internal/runner"
	"mobile/parity/internal/scenario"
	"mobile/parity/internal/tap"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: parity lint PATH... | parity run --reference URL --candidate URL PATH...")
		return 2
	}
	switch args[0] {
	case "lint":
		return lint(args[1:], stdout, stderr)
	case "run":
		return compareStacks(args[1:], stdout, stderr)
	case "canary":
		return canaryRun(args[1:], stdout, stderr)
	case "probe":
		return probeRun(args[1:], stdout, stderr)
	case "tap":
		return tapRun(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func lint(paths []string, stdout, stderr io.Writer) int {
	scenarios, err := scenario.LoadPaths(paths...)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "lint: %d scenario(s) valid\n", len(scenarios))
	return 0
}

type stepReport struct {
	ID          string   `json:"id"`
	Differences []string `json:"differences,omitempty"`
	Database    []string `json:"database,omitempty"`
}

type scenarioReport struct {
	ID    string       `json:"id"`
	File  string       `json:"file"`
	Equal bool         `json:"equal"`
	Steps []stepReport `json:"steps"`
}

type tapReport struct {
	Steps        int      `json:"steps"`
	InCore       int      `json:"answered_in_core"`
	ServedRoutes int      `json:"served_routes"`
	Unserved     []string `json:"unserved,omitempty"`
}

type report struct {
	Racy          []string         `json:"racy,omitempty"`
	Reference     string           `json:"reference"`
	DatabaseLane  bool             `json:"database_lane"`
	Candidate     string           `json:"candidate"`
	Scenarios     int              `json:"scenarios"`
	Steps         int              `json:"steps"`
	ScenariosDiff int              `json:"scenarios_diff"`
	Differences   int              `json:"differences"`
	Accepted      map[string]int   `json:"accepted,omitempty"`
	Tap           *tapReport       `json:"tap,omitempty"`
	Results       []scenarioReport `json:"results"`
}

func compareStacks(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	reference := flags.String("reference", "", "base URL of the Python reference")
	candidate := flags.String("candidate", "", "base URL of the candidate front door")
	host := flags.String("host", "parity.test", "Host header sent to both stacks")
	jsonOut := flags.String("json", "", "write a JSON report here")
	refDSN := flags.String("reference-dsn", "", "reference database URL; with --candidate-dsn, snapshot both after every step")
	candDSN := flags.String("candidate-dsn", "", "candidate database URL; with --reference-dsn, snapshot both after every step")
	authMode := flags.String("auth", "", "auth mode both stacks were started in (dev or prod); only scenarios written for it run")
	candidateTap := flags.String("candidate-tap", "", "control URL of the tap between the candidate front door and its Python")
	servedRoutes := flags.String("served-routes", "", "JSON of `core routes --json`: routes the candidate serves in Go; needs --candidate-tap")
	candidatePython := flags.String("candidate-python", "", "base URL of the candidate's Python without core (through the tap), for steps with via: python")
	burstRepeats := flags.Int("burst-repeats", 3, "reference runs of a scenario with a concurrent step; the candidate must match one of them")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reference == "" || *candidate == "" || flags.NArg() == 0 {
		fmt.Fprintln(stderr, "parity run: --reference, --candidate and at least one scenario path are required")
		return 2
	}
	if *burstRepeats < 1 {
		fmt.Fprintln(stderr, "parity run: --burst-repeats must be at least 1")
		return 2
	}
	scenarios, err := scenario.LoadPaths(flags.Args()...)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if scenarios, err = filterAuth(scenarios, *authMode, stdout); err != nil {
		fmt.Fprintln(stderr, "parity run:", err)
		return 2
	}
	refClient, err := httpclient.New(*reference, *host)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	candClient, err := httpclient.New(*candidate, *host)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var candPython *httpclient.Client
	if *candidatePython != "" {
		if candPython, err = httpclient.New(*candidatePython, *host); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if (*refDSN == "") != (*candDSN == "") {
		fmt.Fprintln(stderr, "parity run: --reference-dsn and --candidate-dsn go together; one database alone compares nothing")
		return 2
	}
	// Interfaces stay nil when no DSN is given: a nil *pgxpool.Pool stored in
	// dbsnap.Conn would look like a database lane that is switched on.
	var refDB, candDB dbsnap.Conn
	if *refDSN != "" {
		refPool, err := pgxpool.New(context.Background(), *refDSN)
		if err != nil {
			fmt.Fprintln(stderr, "parity run: reference database:", err)
			return 2
		}
		defer refPool.Close()
		candPool, err := pgxpool.New(context.Background(), *candDSN)
		if err != nil {
			fmt.Fprintln(stderr, "parity run: candidate database:", err)
			return 2
		}
		defer candPool.Close()
		refDB, candDB = refPool, candPool
	}
	for _, side := range []struct {
		name   string
		client *httpclient.Client
	}{{"reference", refClient}, {"candidate", candClient}} {
		if err := checkAuthMode(context.Background(), side.name, side.client, *authMode); err != nil {
			fmt.Fprintln(stderr, "INFRA", err)
			return 2
		}
	}
	if *servedRoutes != "" && *candidateTap == "" {
		fmt.Fprintln(stderr, "parity run: --served-routes needs --candidate-tap; without a tap nothing shows who answered")
		return 2
	}
	var candTap *tap.Client
	served := map[string]bool{}
	if *candidateTap != "" {
		if candTap, err = tap.NewClient(*candidateTap); err != nil {
			fmt.Fprintln(stderr, "parity run:", err)
			return 2
		}
	}
	if *servedRoutes != "" {
		data, err := os.ReadFile(*servedRoutes)
		if err != nil {
			fmt.Fprintln(stderr, "parity run:", err)
			return 2
		}
		var views []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &views); err != nil {
			fmt.Fprintln(stderr, "parity run: --served-routes:", err)
			return 2
		}
		for _, view := range views {
			served[view.ID] = true
		}
	}
	refStack := runner.Stack{Name: "reference", Client: refClient, DB: refDB, Python: refClient}
	candStack := runner.Stack{Name: "candidate", Client: candClient, DB: candDB, Tap: candTap, Python: candPython}

	ctx := context.Background()
	rep := report{Reference: *reference, Candidate: *candidate, DatabaseLane: refDB != nil, Accepted: map[string]int{}}
	if candTap != nil {
		rep.Tap = &tapReport{ServedRoutes: len(served)}
	}
	ran := map[string]bool{}
	for _, sc := range scenarios {
		nonce := runner.NewNonce()
		refRun, err := runner.Execute(ctx, sc, refStack, nonce)
		if err != nil {
			fmt.Fprintf(stderr, "INFRA %v\n", err)
			return 2
		}
		if runner.PersonasRefused(sc, refRun) {
			fmt.Fprintf(stderr, "INFRA %s: the reference answered 401 to every persona step; its sessions or actor headers were not accepted\n", sc.ID)
			return 2
		}
		refRuns := []*runner.Run{refRun}
		for sc.HasBursts() && len(refRuns) < *burstRepeats {
			again, err := runner.Execute(ctx, sc, refStack, runner.NewNonce())
			if err != nil {
				fmt.Fprintf(stderr, "INFRA %v\n", err)
				return 2
			}
			refRuns = append(refRuns, again)
		}
		if racy := runner.RacySteps(refRuns); len(racy) > 0 {
			rep.Racy = append(rep.Racy, sc.ID+": "+strings.Join(racy, ", "))
			fmt.Fprintf(stdout, "RACY  %s: the reference answered %s differently across %d runs; the candidate must match one of them\n",
				sc.ID, strings.Join(racy, ", "), len(refRuns))
		}
		candRun, err := runner.Execute(ctx, sc, candStack, nonce)
		if err != nil {
			fmt.Fprintf(stderr, "INFRA %v\n", err)
			return 2
		}
		diffs, matched := runner.Closest(refRuns, candRun)
		if matched > 0 {
			refRun = refRuns[matched]
		}
		ran[sc.ID] = true
		if rep.Tap != nil {
			for _, step := range candRun.Steps {
				rep.Tap.Steps++
				if step.PythonRequests == 0 {
					rep.Tap.InCore++
				}
			}
			for _, id := range runner.RoutesNotServedInCore(sc, candRun, served) {
				rep.Tap.Unserved = append(rep.Tap.Unserved, sc.ID+": "+id)
				fmt.Fprintf(stdout, "NOT-IN-CORE %s: Go serves %s, but every step reached Python\n", sc.ID, id)
			}
		}
		for name, n := range runner.AcceptedCounts(refRun, candRun) {
			rep.Accepted[name] += n
		}
		result := scenarioReport{ID: sc.ID, File: sc.File, Equal: len(diffs) == 0}
		byStep := map[string][]string{}
		dbByStep := map[string][]string{}
		for _, d := range diffs {
			for _, difference := range d.Differences {
				byStep[d.StepID] = append(byStep[d.StepID], difference.String())
				rep.Differences++
			}
			for _, difference := range d.Database {
				dbByStep[d.StepID] = append(dbByStep[d.StepID], difference.String())
				rep.Differences++
			}
		}
		for _, step := range sc.Steps {
			result.Steps = append(result.Steps, stepReport{ID: step.ID, Differences: byStep[step.ID], Database: dbByStep[step.ID]})
			rep.Steps++
		}
		rep.Scenarios++
		if result.Equal {
			fmt.Fprintf(stdout, "EQUAL %s (%d steps)\n", sc.ID, len(sc.Steps))
		} else {
			rep.ScenariosDiff++
			fmt.Fprintf(stdout, "DIFF  %s\n", sc.ID)
			for _, d := range diffs {
				for _, difference := range d.Differences {
					fmt.Fprintf(stdout, "  step %s — %s\n", d.StepID, difference)
				}
				for _, difference := range d.Database {
					fmt.Fprintf(stdout, "  step %s — database: %s\n", d.StepID, difference)
				}
			}
		}
		rep.Results = append(rep.Results, result)
	}
	// An accepted divergence is checked where its scenario ran: it must still
	// show, or the exception outlived its cause (ADR-0029 §2.4).
	stale := 0
	names := make([]string, 0, len(compare.AcceptedDivergence))
	for name := range compare.AcceptedDivergence {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		scenarioID := compare.AcceptedDivergence[name]
		if !ran[scenarioID] {
			continue
		}
		fmt.Fprintf(stdout, "accepted: %s=%d (ADR-0029 §2.4, shown by %s)\n", name, rep.Accepted[name], scenarioID)
		if rep.Accepted[name] == 0 {
			stale++
			fmt.Fprintf(stdout, "STALE %s: %s ran and the two sides no longer differ this way; remove the exception from ADR-0029 §2.4\n", name, scenarioID)
		}
	}
	lane := "off"
	if rep.DatabaseLane {
		lane = "on"
	}
	fmt.Fprintf(stdout, "parity: scenarios=%d steps=%d scenarios_diff=%d differences=%d database_lane=%s\n",
		rep.Scenarios, rep.Steps, rep.ScenariosDiff, rep.Differences, lane)
	if *jsonOut != "" {
		data, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(*jsonOut, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if rep.Tap != nil {
		fmt.Fprintf(stdout, "tap: steps=%d answered_in_core=%d served_routes=%d unserved=%d\n",
			rep.Tap.Steps, rep.Tap.InCore, rep.Tap.ServedRoutes, len(rep.Tap.Unserved))
	}
	if rep.Differences > 0 || stale > 0 || (rep.Tap != nil && len(rep.Tap.Unserved) > 0) {
		return 1
	}
	return 0
}

// canaryRun puts a damaging proxy in front of target (a second, isolated
// Python) and runs the scenarios reference-vs-proxy once per mode. Identity
// must be equal; every mode that damaged at least one response must produce
// at least one difference. A mode the corpus never exercises is reported, not
// counted as caught.
func canaryRun(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("canary", flag.ContinueOnError)
	flags.SetOutput(stderr)
	reference := flags.String("reference", "", "base URL of the Python reference")
	target := flags.String("target", "", "base URL of the isolated Python the canary proxy fronts")
	host := flags.String("host", "parity.test", "Host header sent to both stacks")
	authMode := flags.String("auth", "", "auth mode both stacks were started in (dev or prod); only scenarios written for it run")
	refDSN := flags.String("reference-dsn", "", "reference database URL, used only to seed prod-mode sessions")
	targetDSN := flags.String("target-dsn", "", "target database URL, used only to seed prod-mode sessions")
	burstRepeats := flags.Int("burst-repeats", 3, "reference runs of a scenario with a concurrent step; the damaged target must match none of them")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reference == "" || *target == "" || flags.NArg() == 0 {
		fmt.Fprintln(stderr, "parity canary: --reference, --target and at least one scenario path are required")
		return 2
	}
	scenarios, err := scenario.LoadPaths(flags.Args()...)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if scenarios, err = filterAuth(scenarios, *authMode, stdout); err != nil {
		fmt.Fprintln(stderr, "parity canary:", err)
		return 2
	}
	targetURL, err := url.Parse(*target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	refClient, err := httpclient.New(*reference, *host)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if (*refDSN == "") != (*targetDSN == "") {
		fmt.Fprintln(stderr, "parity canary: --reference-dsn and --target-dsn go together")
		return 2
	}
	targetClient, err := httpclient.New(*target, *host)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, side := range []struct {
		name   string
		client *httpclient.Client
	}{{"reference", refClient}, {"target", targetClient}} {
		if err := checkAuthMode(context.Background(), side.name, side.client, *authMode); err != nil {
			fmt.Fprintln(stderr, "INFRA", err)
			return 2
		}
	}
	// Sessions only: the canary compares the wire and never snapshots.
	var refSessions, targetSessions dbsnap.Conn
	if *refDSN != "" {
		refPool, err := pgxpool.New(context.Background(), *refDSN)
		if err != nil {
			fmt.Fprintln(stderr, "parity canary: reference database:", err)
			return 2
		}
		defer refPool.Close()
		targetPool, err := pgxpool.New(context.Background(), *targetDSN)
		if err != nil {
			fmt.Fprintln(stderr, "parity canary: target database:", err)
			return 2
		}
		defer targetPool.Close()
		refSessions, targetSessions = refPool, targetPool
	}

	failed := 0
	fmt.Fprintf(stdout, "%-24s %-10s %-12s %s\n", "mode", "applied", "differences", "verdict")
	for _, mode := range canary.Modes() {
		var applied atomic.Int64
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		server := &http.Server{Handler: canary.Proxy(targetURL, mode, &applied)}
		go func() { _ = server.Serve(listener) }()
		candClient, err := httpclient.New("http://"+listener.Addr().String(), *host)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		differences := 0
		for _, sc := range scenarios {
			nonce := runner.NewNonce()
			refRun, err := runner.Execute(context.Background(), sc, runner.Stack{Name: "reference", Client: refClient, Sessions: refSessions, Python: refClient}, nonce)
			if err != nil {
				fmt.Fprintf(stderr, "INFRA %v\n", err)
				_ = server.Close()
				return 2
			}
			if runner.PersonasRefused(sc, refRun) {
				fmt.Fprintf(stderr, "INFRA %s: the reference answered 401 to every persona step; its sessions or actor headers were not accepted\n", sc.ID)
				_ = server.Close()
				return 2
			}
			refRuns := []*runner.Run{refRun}
			for sc.HasBursts() && len(refRuns) < *burstRepeats {
				again, err := runner.Execute(context.Background(), sc, runner.Stack{Name: "reference", Client: refClient, Sessions: refSessions, Python: refClient}, runner.NewNonce())
				if err != nil {
					fmt.Fprintf(stderr, "INFRA %v\n", err)
					_ = server.Close()
					return 2
				}
				refRuns = append(refRuns, again)
			}
			candRun, err := runner.Execute(context.Background(), sc, runner.Stack{Name: "canary-" + mode.Name, Client: candClient, Sessions: targetSessions, Python: candClient}, nonce)
			if errors.Is(err, runner.ErrSetup) {
				fmt.Fprintf(stderr, "INFRA %v\n", err)
				_ = server.Close()
				return 2
			}
			if err != nil {
				// A damaged response can break a later step's bind; that is
				// the comparator's job to notice, so count it as caught.
				differences++
				continue
			}
			closest, _ := runner.Closest(refRuns, candRun)
			for _, d := range closest {
				differences += len(d.Differences)
				if mode.Name == "identity" {
					// Identity must never differ; show why it did.
					for _, difference := range d.Differences {
						fmt.Fprintf(stdout, "  identity %s step %s — %s\n", sc.ID, d.StepID, difference)
					}
				}
			}
		}
		_ = server.Close()
		verdict := "ok"
		switch {
		case mode.Name == "identity" && differences != 0:
			verdict = "FAIL: identity must be equal"
			failed++
		case mode.Name != "identity" && applied.Load() == 0:
			verdict = "not exercised by these scenarios"
		case mode.Name != "identity" && differences == 0:
			verdict = "FAIL: damage went unnoticed"
			failed++
		}
		fmt.Fprintf(stdout, "%-24s %-10d %-12d %s\n", mode.Name, applied.Load(), differences, verdict)
	}
	if failed > 0 {
		fmt.Fprintf(stdout, "canary: %d mode(s) failed\n", failed)
		return 1
	}
	fmt.Fprintln(stdout, "canary: identity equal, every exercised damage caught")
	return 0
}

// probeRun sends malformed request lines straight to both stacks. Only the
// ADR-0029 §2.4 MALFORMED-REQUEST-LINE cases may differ, and every one of them
// must still differ: a stale list is as wrong as an incomplete one.
func probeRun(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	reference := flags.String("reference", "", "base URL of the Python reference")
	candidate := flags.String("candidate", "", "base URL of the candidate front door")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reference == "" || *candidate == "" {
		fmt.Fprintln(stderr, "parity probe: --reference and --candidate are required")
		return 2
	}
	results, err := rawprobe.Run(context.Background(), *reference, *candidate)
	if err != nil {
		fmt.Fprintf(stderr, "INFRA %v\n", err)
		return 2
	}
	differ := 0
	for _, r := range results {
		_, accepted := rawprobe.ExpectedDivergence[r.Case.Name]
		switch {
		case r.Equal && !accepted:
			fmt.Fprintf(stdout, "EQUAL    %-20s %s\n", r.Case.Name, r.Reference.Code)
		case r.Equal && accepted:
			fmt.Fprintf(stdout, "STALE    %-20s both %s, but ADR-0029 lists it as differing\n", r.Case.Name, r.Reference.Code)
		case !r.Equal && accepted:
			differ++
			fmt.Fprintf(stdout, "ACCEPTED %-20s reference %s, candidate %s\n", r.Case.Name, r.Reference.Code, r.Candidate.Code)
		default:
			differ++
			fmt.Fprintf(stdout, "DIFF     %-20s reference %s %q, candidate %s %q\n", r.Case.Name,
				r.Reference.Code, r.Reference.Body, r.Candidate.Code, r.Candidate.Body)
		}
	}
	unexpected, stale := rawprobe.Verdict(results, rawprobe.ExpectedDivergence)
	fmt.Fprintf(stdout, "probe: cases=%d differ=%d accepted=%d unexpected=%d stale=%d\n",
		len(results), differ, len(rawprobe.ExpectedDivergence), len(unexpected), len(stale))
	if len(unexpected) > 0 || len(stale) > 0 {
		return 1
	}
	return 0
}

// filterAuth keeps the scenarios written for the mode the stacks were started
// in. A scenario run against the other mode answers 401 or ignores its
// headers on both sides alike, which reads as equal and proves nothing, so the
// mode is required and the number left out is printed.
func filterAuth(scenarios []*scenario.Scenario, mode string, stdout io.Writer) ([]*scenario.Scenario, error) {
	if mode != "dev" && mode != "prod" {
		return nil, fmt.Errorf("--auth %q: say which mode the stacks run in, dev or prod", mode)
	}
	kept := make([]*scenario.Scenario, 0, len(scenarios))
	for _, sc := range scenarios {
		if sc.AuthMode == mode {
			kept = append(kept, sc)
		}
	}
	fmt.Fprintf(stdout, "auth=%s: %d scenario(s) for this mode, %d left for the other\n", mode, len(kept), len(scenarios)-len(kept))
	if len(kept) == 0 {
		return nil, fmt.Errorf("--auth %s selects no scenario", mode)
	}
	return kept, nil
}

// checkAuthMode asks a stack the one question whose answer depends only on its
// auth mode (ADR-0014): GET /people/me with a well-formed X-Actor-ID and no
// bearer. Dev trusts the header and does not answer 401; prod ignores it and
// does. Statuses alone cannot catch the mix-up: a prod scenario run against
// dev stacks came out equal on every step, because the session routes read
// the bearer in both modes while every other step was 401 on both sides.
func checkAuthMode(ctx context.Context, name string, client *httpclient.Client, mode string) error {
	header := http.Header{}
	header.Set("X-Actor-ID", runner.PersonaID("parity/auth-mode", "sentinel"))
	resp, err := client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: "/people/me", Header: header})
	if err != nil {
		return fmt.Errorf("%s: auth mode check: %w", name, err)
	}
	if (resp.Status == http.StatusUnauthorized) != (mode == "prod") {
		return fmt.Errorf("%s answered GET /people/me with only X-Actor-ID as %d, so it is not running auth=%s", name, resp.Status, mode)
	}
	return nil
}

// tapRun runs the tap between a candidate front door and its Python until the
// process is stopped.
func tapRun(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	listen := flags.String("listen", "", "address the front door sends Python-bound requests to")
	control := flags.String("control", "", "address the harness reads the record from")
	upstream := flags.String("upstream", "", "base URL of the Python API")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	target, err := url.Parse(*upstream)
	if *listen == "" || *control == "" || err != nil || target.Scheme == "" || target.Host == "" {
		fmt.Fprintln(stderr, "parity tap: --listen, --control and an --upstream URL are required")
		return 2
	}
	rec := &tap.Recorder{}
	errs := make(chan error, 2)
	for _, srv := range []*http.Server{
		{Addr: *listen, Handler: rec.Proxy(target), ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 1 << 20},
		{Addr: *control, Handler: rec.Control(), ReadHeaderTimeout: 5 * time.Second},
	} {
		srv := srv
		go func() { errs <- srv.ListenAndServe() }()
	}
	fmt.Fprintf(stdout, "tap: %s -> %s, record on %s\n", *listen, *upstream, *control)
	fmt.Fprintln(stderr, "parity tap:", <-errs)
	return 2
}
