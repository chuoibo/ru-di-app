package aigate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The core never installs an OpenTelemetry provider (ADR-0037 §2.8). ADK
// traces every model call and tool call through the global otel API, and its
// span attributes carry whole tool results; with the global providers left as
// they are, those spans go nowhere. One call to otel.SetTracerProvider, or
// one import of the SDK or of ADK's telemetry setup, and questions and
// answers start leaving the process.
//
// What this gate checks, and what it does not. The core binary DOES link
// OpenTelemetry: the otel API and its global tracer, ADK's internal
// telemetry (adk/internal/telemetry), otelhttp, and go.opentelemetry.io/
// auto/sdk -- which the global tracer itself imports
// (otel/internal/global/trace.go) so that an eBPF auto-instrumentation agent
// can attach to the process and flip it on at run time, exporting ADK's spans
// with no code change and no provider installed. The gate forbids, on the
// source, importing the SDK, an exporter, ADK's telemetry setup or
// go.opentelemetry.io/auto, and calling a Set…Provider; on the linked graph,
// the SDK and the exporters, and auto/sdk reached from anywhere but the otel
// API. It cannot see a process being instrumented from outside, so that half
// is an operations rule (ADR-0037 §4): no Go eBPF auto-instrumentation agent
// (go.opentelemetry.io/auto or any tool built on it) on a host that runs
// `core serve` or `core work`.

// Packages non-test core code may not import.
var otelForbidden = []string{
	"go.opentelemetry.io/otel/sdk",
	"go.opentelemetry.io/otel/exporters",
	"google.golang.org/adk/telemetry",
	"go.opentelemetry.io/contrib/exporters",
}

// Packages our own code may not import, although the otel API links them.
const otelAutoForbidden = "go.opentelemetry.io/auto"

// otelAutoVia is the only package allowed to bring auto/sdk into the graph.
const otelAutoVia = "go.opentelemetry.io/otel/internal/global"

func forbiddenImport(path string) bool {
	for _, f := range otelForbidden {
		if path == f || strings.HasPrefix(path, f+"/") {
			return true
		}
	}
	return false
}

func forbiddenSourceImport(path string) bool {
	return forbiddenImport(path) || path == otelAutoForbidden || strings.HasPrefix(path, otelAutoForbidden+"/")
}

// Packages whose Set…Provider functions install a global.
var otelGlobals = map[string]bool{
	"go.opentelemetry.io/otel":               true,
	"go.opentelemetry.io/otel/log/global":    true,
	"go.opentelemetry.io/otel/metric/global": true,
}

var setGlobal = regexp.MustCompile(`^Set\w*(Provider|Propagator)$`)

// otelViolations parses one Go file and reports forbidden imports and calls.
func otelViolations(fset *token.FileSet, name string, src any) ([]string, error) {
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out []string
	local := map[string]string{} // local name -> import path, for otel globals
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if forbiddenSourceImport(path) {
			out = append(out, name+" imports "+path)
		}
		if otelGlobals[path] {
			n := filepath.Base(path)
			if imp.Name != nil {
				n = imp.Name.Name
			}
			local[n] = path
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && local[id.Name] != "" && setGlobal.MatchString(sel.Sel.Name) {
			out = append(out, name+" uses "+local[id.Name]+"."+sel.Sel.Name)
		}
		return true
	})
	return out, nil
}

func TestCoreNeverInstallsAnOtelProvider(t *testing.T) {
	fset := token.NewFileSet()
	root := "../.."
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files++
		v, err := otelViolations(fset, path, nil)
		if err != nil {
			return err
		}
		for _, x := range v {
			t.Error(x)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 200 {
		t.Fatalf("only %d source files scanned; the walk is looking at the wrong place", files)
	}
}

// The linked binary: the dependency graph of `core` holds no SDK, no exporter
// and no ADK telemetry setup, whoever imports them; and auto/sdk, which it
// does hold, is imported by the otel API's global tracer and by nothing else.
func TestCoreBinaryDoesNotLinkOtelSDK(t *testing.T) {
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps, Dir: "../.."}, module+"/cmd/core")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	importers := map[string][]string{}
	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		if seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		for _, imp := range p.Imports {
			importers[imp.PkgPath] = append(importers[imp.PkgPath], p.PkgPath)
			walk(imp)
		}
	}
	for _, p := range pkgs {
		walk(p)
	}
	for _, must := range []string{"google.golang.org/adk/agent/llmagent", "go.opentelemetry.io/otel"} {
		if !seen[must] {
			t.Fatalf("%s is not in core's graph; the walk is broken (%d packages)", must, len(seen))
		}
	}
	for path := range seen {
		if forbiddenImport(path) {
			t.Errorf("core links %s", path)
		}
		if path == otelAutoForbidden || strings.HasPrefix(path, otelAutoForbidden+"/") {
			for _, by := range importers[path] {
				if by != otelAutoVia && !strings.HasPrefix(by, otelAutoForbidden+"/") && by != otelAutoForbidden {
					t.Errorf("%s is imported by %s: only %s may bring it in", path, by, otelAutoVia)
				}
			}
		}
	}
	t.Logf("auto/sdk in the graph: %v, imported by %v", seen[otelAutoForbidden+"/sdk"], importers[otelAutoForbidden+"/sdk"])
}

// Canary: the source check is red on each thing it forbids.
func TestOtelGateCanRed(t *testing.T) {
	fset := token.NewFileSet()
	for name, src := range map[string]string{
		"sdk.go":    "package x\nimport _ \"go.opentelemetry.io/otel/sdk/trace\"\n",
		"adk.go":    "package x\nimport _ \"google.golang.org/adk/telemetry\"\n",
		"auto.go":   "package x\nimport _ \"go.opentelemetry.io/auto/sdk\"\n",
		"set.go":    "package x\nimport \"go.opentelemetry.io/otel\"\nfunc f() { otel.SetTracerProvider(nil) }\n",
		"alias.go":  "package x\nimport o \"go.opentelemetry.io/otel\"\nfunc f() { o.SetTextMapPropagator(nil) }\n",
		"global.go": "package x\nimport \"go.opentelemetry.io/otel/log/global\"\nfunc f() { global.SetLoggerProvider(nil) }\n",
	} {
		v, err := otelViolations(fset, name, src)
		if err != nil {
			t.Fatal(err)
		}
		if len(v) == 0 {
			t.Errorf("%s: not caught", name)
		}
	}
	v, _ := otelViolations(fset, "ok.go", "package x\nimport \"go.opentelemetry.io/otel\"\nfunc f() { _ = otel.GetTracerProvider() }\n")
	if len(v) != 0 {
		t.Fatalf("reading the global provider was taken for installing one: %v", v)
	}
}
