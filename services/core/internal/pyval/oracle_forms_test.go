//go:build oracle

package pyval

// Form bodies measured in the parity image. Run from services/core:
//
//	go test -tags oracle -run 'TestOracleForms|TestOracleCodecTables' -v ./internal/pyval/
//
// TestOracleForms runs oracleDriver with two synthetic form routes added to
// create_app(), their IR rendered by the committed
// scripts/render_contract_ir.py (mounted read-only) before any endpoint is
// stubbed. It first asks for that IR alone, binds it together with the
// contract's form routes, sends every case of formCases
// (form_cases_test.go) and compares Go with Python. The committed
// testdata/synthetic_form_ir.json and testdata/form_cases.json must equal
// what was measured; PYVAL_ORACLE_RECORD=1 rewrites them instead.
//
// The random form bodies of TestOracle (formRandom below) cover the same
// routes plus the File routes an auth dependency guards.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const formSyntheticRoutes = `
sys.path.insert(0, "/pyval")
from typing import Annotated
from fastapi import File, Form
from fastapi import UploadFile as FastAPIUploadFile
import render_contract_ir

def syn_fields(
    a: Annotated[str, Form()],
    n: Annotated[int, Form()],
    b: Annotated[str, Form()] = "dflt",
    c: Annotated[int | None, Form()] = None,
    d: Annotated[str, Form(alias="d-key")] = "x",
):
    return None

def syn_file(
    upload: Annotated[FastAPIUploadFile, File()],
    note: Annotated[str | None, Form()] = None,
):
    return None

for _fn in (syn_fields, syn_file):
    _fn.__module__ = "pyval_synthetic_form"
app.add_api_route("/pyval-synthetic/form/fields", syn_fields, methods=["POST"])
app.add_api_route("/pyval-synthetic/form/file", syn_file, methods=["POST"])
_groups = render_contract_ir.route_entries(app)
_doc = render_contract_ir.group_document("pyval_synthetic_form", _groups["pyval_synthetic_form"], render_contract_ir.stack_versions())
sys.stdout.write(json.dumps({"ir": json.dumps(_doc, indent=1, ensure_ascii=True) + "\n"}) + "\n")
sys.stdout.flush()
`

func TestOracleForms(t *testing.T) {
	image := envOr("PYVAL_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "scripts", "render_contract_ir.py"))
	if err != nil {
		t.Fatal(err)
	}
	scriptText, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	driver := strings.Replace(oracleDriver, oracleRoutesMarker, formSyntheticRoutes, 1)
	if driver == oracleDriver {
		t.Fatalf("oracleDriver has no %q line", oracleRoutesMarker)
	}
	scriptSum := sha256.Sum256(scriptText)
	oracleDriverOverride = driver
	oracleDockerArgs = []string{"-v", script + ":/pyval/render_contract_ir.py:ro"}
	oracleCacheSalt = letterDigest(scriptSum[:])
	defer func() {
		oracleDriverOverride, oracleDockerArgs, oracleCacheSalt, oracleCacheSuffix = "", nil, "", ""
	}()

	oracleCacheSuffix = ".forms-ir"
	head := pythonAnswers(t, image, nil, 1)
	var measured struct {
		IR string `json:"ir"`
	}
	if err := json.Unmarshal(head[0], &measured); err != nil {
		t.Fatalf("driver IR line: %v", err)
	}
	routes := formCaseRoutes(t, []byte(measured.IR))
	cases := formCases(routes)

	var stdin bytes.Buffer
	for _, fc := range cases {
		r := routes[fc.Route]
		oc := oracleCase{Method: r.Method, Path: fc.path(r), Headers: fc.Headers, Body: fc.Body}
		line, err := json.Marshal(oc.wire())
		if err != nil {
			t.Fatal(err)
		}
		stdin.Write(line)
		stdin.WriteByte('\n')
	}
	oracleCacheSuffix = ".forms"
	lines := pythonAnswers(t, image, stdin.Bytes(), len(cases)+1)[1:]

	type tally struct{ total, accepted, refused, bad int }
	byRoute := map[string]*tally{}
	rec := formRecord{Image: image}
	mismatches := 0
	for i, fc := range cases {
		want := normalizeJSON(t, lines[i])
		got := normalizeJSON(t, formOutcome(t, routes[fc.Route], fc))
		tl := byRoute[fc.Route]
		if tl == nil {
			tl = &tally{}
			byRoute[fc.Route] = tl
		}
		tl.total++
		if strings.Contains(want, `"status":299`) {
			tl.accepted++
		} else {
			tl.refused++
		}
		rec.Cases = append(rec.Cases, newFormRecordCase(fc, want))
		if got == want {
			continue
		}
		tl.bad++
		mismatches++
		if mismatches <= 25 {
			t.Errorf("mismatch %s [%s]\n  request: %s\n  python:  %s\n  go:      %s",
				fc.Route, fc.Name, clip(strconv.Quote(fc.Body), 400), clip(want, 900), clip(got, 900))
		}
	}
	ids := make([]string, 0, len(byRoute))
	for id := range byRoute {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		tl := byRoute[id]
		t.Logf("%-40s cases=%4d accepted=%4d refused=%4d mismatches=%d", id, tl.total, tl.accepted, tl.refused, tl.bad)
	}
	t.Logf("form oracle %s: %d routes, %d cases, %d mismatches", image, len(ids), len(cases), mismatches)

	encoded, err := encodeFormRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PYVAL_ORACLE_RECORD") == "1" {
		if err := os.WriteFile("testdata/synthetic_form_ir.json", []byte(measured.IR), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("testdata/form_cases.json", encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("recorded testdata/synthetic_form_ir.json and testdata/form_cases.json (%d bytes)", len(encoded))
		return
	}
	if !bytes.Equal(readFile(t, "testdata/synthetic_form_ir.json"), []byte(measured.IR)) {
		t.Errorf("testdata/synthetic_form_ir.json differs from the image: rerun with PYVAL_ORACLE_RECORD=1")
	}
	if !bytes.Equal(readFile(t, "testdata/form_cases.json"), encoded) {
		t.Errorf("testdata/form_cases.json differs from the image: rerun with PYVAL_ORACLE_RECORD=1")
	}
}

const codecTablesDriver = `
import codecs, encodings, encodings.aliases, json, pkgutil, sys
mods = sorted(m.name for m in pkgutil.iter_modules(encodings.__path__))
nontext = []
for m in mods:
    try:
        if not getattr(codecs.lookup(m), "_is_text_encoding", True):
            nontext.append(m)
    except LookupError:
        pass
json.dump({"modules": mods, "aliases": encodings.aliases.aliases, "nontext": nontext}, sys.stdout)
`

// TestOracleCodecTables re-measures the encodings tables codecs.go carries.
func TestOracleCodecTables(t *testing.T) {
	image := envOr("PYVAL_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	out, err := exec.Command("docker", "run", "--rm", "--network", "none", "--entrypoint", "python", image, "-c", codecTablesDriver).Output()
	if err != nil {
		t.Fatalf("codec tables: %v", err)
	}
	var measured struct {
		Modules []string          `json:"modules"`
		Aliases map[string]string `json:"aliases"`
		Nontext []string          `json:"nontext"`
	}
	if err := json.Unmarshal(out, &measured); err != nil {
		t.Fatal(err)
	}
	same := func(name string, got map[string]bool, want []string) {
		if len(got) != len(want) {
			t.Errorf("%s: Go has %d, the image %d", name, len(got), len(want))
		}
		for _, w := range want {
			if !got[w] {
				t.Errorf("%s: %s is missing in Go", name, w)
			}
		}
	}
	same("modules", codecModules, measured.Modules)
	same("non-text codecs", codecNotText, measured.Nontext)
	if len(codecAliases) != len(measured.Aliases) {
		t.Errorf("aliases: Go has %d, the image %d", len(codecAliases), len(measured.Aliases))
	}
	for k, v := range measured.Aliases {
		if codecAliases[k] != v {
			t.Errorf("alias %s: Go %q, the image %q", k, codecAliases[k], v)
		}
	}
	t.Logf("codec tables %s: %d modules, %d aliases, %d non-text", image, len(measured.Modules), len(measured.Aliases), len(measured.Nontext))
}

// ---- random form bodies for TestOracle ----

func (g *gen) formRandom() {
	slots := formSlotsOf(g.route)
	hasFile := false
	for _, s := range slots {
		hasFile = hasFile || s.kind == "file"
	}
	oddTypes := []string{
		"", "text/plain", "application/json", "APPLICATION/X-WWW-FORM-URLENCODED",
		formURLEncoded + "; charset=latin-1", "Application/X-WWW-Form-Urlencoded; charset=utf-8",
		"multipart/form-data", formMultipart + "; charset=latin-1", "Multipart/Form-Data; boundary=" + formBoundary,
	}
	for len(g.cases) < g.budget {
		rq := request{params: map[string]string{}}
		for _, p := range g.routeParams("path") {
			rq.params[p.name] = g.pathValue(p.v)
			if g.chance(0.08) {
				rq.params[p.name] = pick(g, g.pathWrongs(p.v)...)
			}
		}
		multipart := hasFile && g.chance(0.85) || g.chance(0.35)
		var pairs [][2]string
		var parts []string
		for _, s := range slots {
			n := 1
			switch g.r.IntN(10) {
			case 0:
				n = 0
			case 1:
				n = 2
			}
			for j := 0; j < n; j++ {
				value := g.formValue(s)
				switch {
				case !multipart:
					pairs = append(pairs, [2]string{g.formEscape(s.alias), g.formEscape(value)})
				case (s.kind == "file") != g.chance(0.1):
					parts = append(parts, mpFile(s.alias, g.text(0, 6), value))
				default:
					parts = append(parts, mpText(s.alias, value))
				}
			}
		}
		if g.chance(0.2) {
			name := pick(g, "x", "", "note", "é", strings.ToUpper(slots[0].alias))
			value := g.text(0, 5)
			if multipart {
				parts = append(parts, mpText(name, value))
			} else {
				pairs = append(pairs, [2]string{g.formEscape(name), g.formEscape(value)})
			}
		}
		if g.chance(0.3) {
			g.r.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
			g.r.Shuffle(len(parts), func(i, j int) { parts[i], parts[j] = parts[j], parts[i] })
		}
		if multipart {
			rq.ct, rq.body = formMultipart, mpJoin(formBoundary, parts...)
		} else {
			rq.ct, rq.body = formURLEncoded, formEncode(pairs)
		}
		if g.chance(0.1) {
			rq.ct = pick(g, oddTypes...)
		}
		rq.anon = g.needsActor() && g.chance(0.05)
		g.emit("form-random", rq)
	}
}

func (g *gen) formValue(s formSlot) string {
	switch g.r.IntN(10) {
	case 0:
		return ""
	case 1:
		return pick(g, "%ZZ", "\xe9", "a+b", "%C3%A9", "a&b", "a;b", "a=b", " ", "\x00", "%ED%A0%80")
	}
	switch s.kind {
	case "uuid":
		if g.chance(0.6) {
			return g.uuidCanonical()
		}
		return g.uuidSpelling(g.chance(0.5))
	case "int":
		return pick(g, strconv.Itoa(g.r.IntN(100)), "-3", "1_0", "7.0", "x", " 7")
	}
	return g.text(0, 12)
}

func (g *gen) formEscape(s string) string {
	if g.chance(0.7) {
		return url.QueryEscape(s)
	}
	return s
}
