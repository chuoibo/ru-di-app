// Command parity compares the Python reference with a candidate stack.
//
//	parity lint PATH...
//	parity run --reference URL --candidate URL [--host H] [--json FILE] PATH...
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
	"os"

	"mobile/parity/internal/httpclient"
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
}

type scenarioReport struct {
	ID    string       `json:"id"`
	File  string       `json:"file"`
	Equal bool         `json:"equal"`
	Steps []stepReport `json:"steps"`
}

type report struct {
	Reference     string           `json:"reference"`
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
	refStack := runner.Stack{Name: "reference", Client: refClient}
	candStack := runner.Stack{Name: "candidate", Client: candClient}

	ctx := context.Background()
	rep := report{Reference: *reference, Candidate: *candidate}
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
		for _, d := range diffs {
			for _, difference := range d.Differences {
				byStep[d.StepID] = append(byStep[d.StepID], difference.String())
				rep.Differences++
			}
		}
		for _, step := range sc.Steps {
			result.Steps = append(result.Steps, stepReport{ID: step.ID, Differences: byStep[step.ID]})
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
			}
		}
		rep.Results = append(rep.Results, result)
	}
	fmt.Fprintf(stdout, "parity: scenarios=%d steps=%d scenarios_diff=%d differences=%d\n",
		rep.Scenarios, rep.Steps, rep.ScenariosDiff, rep.Differences)
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
