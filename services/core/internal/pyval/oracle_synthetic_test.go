//go:build oracle

package pyval

// Measures the synthetic route (see synthetic_test.go) in the parity image.
// Run from services/core:
//
//	go test -tags oracle -run TestOracleSynthetic -v ./internal/pyval/
//
// The driver adds the route to create_app(), renders its IR with the
// committed scripts/render_contract_ir.py (mounted read-only), drives every
// case over raw ASGI and prints the IR and the answers. Go binds that IR and
// must answer the same; the committed testdata must equal what was measured.
// PYVAL_ORACLE_RECORD=1 rewrites the testdata instead.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const syntheticDriver = `
import asyncio, json, os, sys
from typing import Annotated
os.environ["MOBILE_AUTH_MODE"] = "dev"
os.environ["MOBILE_DATABASE_URL"] = "postgresql+psycopg://nobody:nothing@127.0.0.1:9/nothing"
sys.path.insert(0, "/srv")
sys.path.insert(0, "/pyval")
from fastapi import Header, Query
from starlette.responses import Response
from app.api.main import create_app
import render_contract_ir

def tree(v):
    if v is None: return ["n"]
    if type(v) is int: return ["i", str(v)]
    if type(v) is str: return ["s", v.encode("utf-8", "surrogatepass").decode("latin-1")]
    if type(v) is list: return ["l", [tree(x) for x in v]]
    return ["?", type(v).__name__]

def order(
    item: int,
    q: Annotated[int, Query(ge=1)],
    x_count: Annotated[int, Header(ge=1)],
    x_tag: Annotated[list[int] | None, Header()] = None,
    x_name: Annotated[str, Header(min_length=3)] = "abc",
):
    values = [["item", item], ["q", q], ["x_count", x_count], ["x_tag", x_tag], ["x_name", x_name]]
    body = json.dumps({"accepted": [[k, tree(v)] for k, v in values]})
    return Response(body, status_code=299, media_type="application/json")

order.__module__ = "pyval_synthetic"
app = create_app()
app.add_api_route("/pyval-synthetic/order/{item}", order, methods=["GET"])
groups = render_contract_ir.route_entries(app)
doc = render_contract_ir.group_document("pyval_synthetic", groups["pyval_synthetic"], render_contract_ir.stack_versions())
ir_text = json.dumps(doc, indent=1, ensure_ascii=True) + "\n"

async def drive(req):
    path = req["path"]
    scope = {
        "type": "http", "asgi": {"version": "3.0"}, "http_version": "1.1",
        "method": req["method"], "scheme": "http", "path": path,
        "raw_path": path.encode("utf-8"), "root_path": "",
        "query_string": req["query"].encode("latin-1"),
        "headers": [(b"host", b"parity.test")] + [(k.encode("latin-1").lower(), v.encode("latin-1")) for k, v in req["headers"]],
        "client": ("127.0.0.1", 40000), "server": ("parity.test", 80), "state": {},
    }
    messages = []
    sent = False
    async def receive():
        nonlocal sent
        if sent:
            await asyncio.sleep(3600)
        sent = True
        return {"type": "http.request", "body": b"", "more_body": False}
    async def send(message):
        messages.append(message)
    try:
        await app(scope, receive, send)
    except Exception:
        pass
    start = next(m for m in messages if m["type"] == "http.response.start")
    payload = b"".join(m.get("body", b"") for m in messages if m["type"] == "http.response.body")
    if start["status"] == 299:
        return {"status": 299, "accepted": json.loads(payload)["accepted"]}
    headers = {k.decode("latin-1"): v.decode("latin-1") for k, v in start.get("headers", [])}
    return {"status": start["status"], "content_type": headers.get("content-type"), "body": payload.decode("latin-1")}

async def main():
    cases = json.loads(sys.stdin.read())["cases"]
    answers = [await drive(c) for c in cases]
    sys.stdout.write(json.dumps({"ir": ir_text, "answers": answers}))

asyncio.run(main())
`

func TestOracleSynthetic(t *testing.T) {
	image := envOr("PYVAL_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "scripts", "render_contract_ir.py"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatal(err)
	}
	cases := syntheticCases()
	wire := make([]map[string]any, len(cases))
	for i, sc := range cases {
		wire[i] = map[string]any{"method": "GET", "path": syntheticWirePath(sc), "query": sc.Query, "headers": sc.Headers, "body": ""}
	}
	stdin, err := json.Marshal(map[string]any{"cases": wire})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none",
		"-v", script+":/pyval/render_contract_ir.py:ro", "--entrypoint", "python", image, "-c", syntheticDriver)
	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(stdin), &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("synthetic driver failed: %v\n%s", err, clipText(stderr.String(), 3000))
	}
	var out struct {
		IR      string            `json:"ir"`
		Answers []json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("driver output: %v\n%s", err, clipText(stderr.String(), 3000))
	}
	if len(out.Answers) != len(cases) {
		t.Fatalf("driver answered %d of %d cases", len(out.Answers), len(cases))
	}

	route := bindSynthetic(t, []byte(out.IR))
	mismatches := 0
	for i, sc := range cases {
		want := normalizeJSON(t, out.Answers[i])
		got := normalizeJSON(t, syntheticOutcome(t, route, sc))
		if got != want {
			mismatches++
			t.Errorf("%s\n  python: %s\n  go:     %s", sc.Name, want, got)
		}
	}
	t.Logf("synthetic oracle %s: %d cases, %d mismatches", image, len(cases), mismatches)

	var rec syntheticRecord
	rec.Image, rec.Route = image, syntheticRouteID
	for i, sc := range cases {
		rec.Cases = append(rec.Cases, struct {
			syntheticCase
			Want json.RawMessage `json:"want"`
		}{sc, json.RawMessage(normalizeJSON(t, out.Answers[i]))})
	}
	var casesJSON bytes.Buffer
	enc := json.NewEncoder(&casesJSON)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(rec); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PYVAL_ORACLE_RECORD") == "1" {
		if err := os.WriteFile("testdata/synthetic_ir.json", []byte(out.IR), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("testdata/synthetic_cases.json", casesJSON.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("recorded testdata/synthetic_ir.json and testdata/synthetic_cases.json")
		return
	}
	if !bytes.Equal(readFile(t, "testdata/synthetic_ir.json"), []byte(out.IR)) {
		t.Errorf("testdata/synthetic_ir.json differs from the image: rerun with PYVAL_ORACLE_RECORD=1")
	}
	if !bytes.Equal(readFile(t, "testdata/synthetic_cases.json"), casesJSON.Bytes()) {
		t.Errorf("testdata/synthetic_cases.json differs from the image: rerun with PYVAL_ORACLE_RECORD=1")
	}
}
