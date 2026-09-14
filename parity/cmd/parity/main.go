// Command parity compares the Python reference with a candidate stack.
//
//	parity lint PATH...
//	parity run --reference URL --candidate URL [--host H] [--json FILE] PATH...
//	parity canary --reference URL --target URL [--host H] PATH...
//	parity probe --reference URL --candidate URL
//
// Exit codes: 0 every step equal, 1 at least one difference, 2 the run could
// not be completed (bad scenario, unreachable stack). A run that could not
// complete is never reported as equal.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/parity/internal/canary"
	"mobile/parity/internal/dbsnap"
	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/rawprobe"
	"mobile/parity/internal/runner"
	"mobile/parity/internal/scenario"
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

type report struct {
	Reference     string           `json:"reference"`
	DatabaseLane  bool             `json:"database_lane"`
	Candidate     string           `json:"candidate"`
	Scenarios     int              `json:"scenarios"`
	Steps         int              `json:"steps"`
	ScenariosDiff int              `json:"scenarios_diff"`
	Differences   int              `json:"differences"`
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
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reference == "" || *candidate == "" || flags.NArg() == 0 {
		fmt.Fprintln(stderr, "parity run: --reference, --candidate and at least one scenario path are required")
		return 2
	}
	scenarios, err := scenario.LoadPaths(flags.Args()...)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	refStack := runner.Stack{Name: "reference", Client: refClient, DB: refDB}
	candStack := runner.Stack{Name: "candidate", Client: candClient, DB: candDB}

	ctx := context.Background()
	rep := report{Reference: *reference, Candidate: *candidate, DatabaseLane: refDB != nil}
	for _, sc := range scenarios {
		refRun, err := runner.Execute(ctx, sc, refStack)
		if err != nil {
			fmt.Fprintf(stderr, "INFRA %v\n", err)
			return 2
		}
		candRun, err := runner.Execute(ctx, sc, candStack)
		if err != nil {
			fmt.Fprintf(stderr, "INFRA %v\n", err)
			return 2
		}
		diffs := runner.Diff(refRun, candRun)
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
	if rep.Differences > 0 {
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
			refRun, err := runner.Execute(context.Background(), sc, runner.Stack{Name: "reference", Client: refClient})
			if err != nil {
				fmt.Fprintf(stderr, "INFRA %v\n", err)
				_ = server.Close()
				return 2
			}
			candRun, err := runner.Execute(context.Background(), sc, runner.Stack{Name: "canary-" + mode.Name, Client: candClient})
			if err != nil {
				// A damaged response can break a later step's bind; that is
				// the comparator's job to notice, so count it as caught.
				differences++
				continue
			}
			for _, d := range runner.Diff(refRun, candRun) {
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
