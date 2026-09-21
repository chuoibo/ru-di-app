#!/usr/bin/env python3
"""Gate: a Python change must not silently reach a route Go already serves.

ADR-0029 §2.9. Once a route is served by the Go front door, its Python handler
is still in the tree -- it is the rollback and the parity reference -- but it no
longer answers clients. A lane that changes the Python code behind that route
changes nothing a user sees, while believing it shipped a fix; and the next
parity run compares Go against a reference that moved.

So this gate builds a call graph of services/api/app from the syntax tree
(route handler -> service -> repository -> domain, by function name), finds the
functions a diff touches, and fails when any of them is reachable from a route
whose manifest row says `owner: go` and `python: live`.

A touch is allowed when the same diff also changes that route's Go side
(services/core/) AND its evidence file, or flips the route back to Python in the
manifest. Frozen routes (`python: frozen`) are not checked here: their Python is
dead code awaiting deletion.

    python3 scripts/check_go_owned_python_touch.py [--base origin/main]
    python3 scripts/check_go_owned_python_touch.py --selftest

Exit codes: 0 pass, 1 violation, 2 cannot run.

What this does not prove: resolution is by bare function name, so it
over-approximates (two functions named alike are both treated as reachable) and
it cannot see dynamic dispatch through getattr or dictionaries of callables.
Over-approximation errs towards red, which is the safe direction for a guard.
"""

from __future__ import annotations

import argparse
import ast
import json
import os
import re
import subprocess
import sys
from collections import defaultdict, deque
from dataclasses import dataclass
from pathlib import Path

os.environ.setdefault("MOBILE_INTERNAL_TOKEN", "test-brain-token")

ROOT = Path(__file__).resolve().parents[1]
API_ROOT = ROOT / "services" / "api"
APP_DIR = API_ROOT / "app"
MANIFEST = ROOT / "services" / "core" / "ownership" / "routes.json"
HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")


@dataclass(frozen=True)
class Func:
    module: str
    qualname: str
    name: str
    start: int
    end: int
    refs: frozenset[str]

    @property
    def key(self) -> tuple[str, str]:
        return (self.module, self.qualname)


def index_source(module: str, source: str) -> list[Func]:
    """Every function and method in one module, with the names it references."""
    tree = ast.parse(source)
    found: list[Func] = []

    def visit(node: ast.AST, prefix: str) -> None:
        for child in ast.iter_child_nodes(node):
            if isinstance(child, ast.FunctionDef | ast.AsyncFunctionDef):
                refs: set[str] = set()
                for sub in ast.walk(child):
                    if isinstance(sub, ast.Attribute):
                        refs.add(sub.attr)
                    elif isinstance(sub, ast.Name):
                        refs.add(sub.id)
                qualname = prefix + child.name
                found.append(
                    Func(
                        module,
                        qualname,
                        child.name,
                        child.lineno,
                        child.end_lineno or child.lineno,
                        frozenset(refs),
                    )
                )
                visit(child, qualname + ".")
            elif isinstance(child, ast.ClassDef):
                visit(child, prefix + child.name + ".")

    visit(tree, "")
    return found


def build_index(sources: dict[str, str]) -> list[Func]:
    index: list[Func] = []
    for module in sorted(sources):
        index.extend(index_source(module, sources[module]))
    return index


def reachable(index: list[Func], root: tuple[str, str]) -> set[tuple[str, str]]:
    """Functions reachable from root by following referenced names."""
    by_key = {func.key: func for func in index}
    by_name: dict[str, list[Func]] = defaultdict(list)
    for func in index:
        by_name[func.name].append(func)
    if root not in by_key:
        return set()
    seen = {root}
    queue = deque([by_key[root]])
    while queue:
        func = queue.popleft()
        for name in func.refs:
            for target in by_name.get(name, []):
                if target.key not in seen:
                    seen.add(target.key)
                    queue.append(target)
    return seen


def touched(index: list[Func], module: str, lines: set[int]) -> set[tuple[str, str]]:
    """Innermost functions of module whose body contains any of lines."""
    hits: set[tuple[str, str]] = set()
    for line in lines:
        containing = [
            func
            for func in index
            if func.module == module and func.start <= line <= func.end
        ]
        if containing:
            innermost = min(containing, key=lambda func: func.end - func.start)
            hits.add(innermost.key)
    return hits


def find_violations(
    reach_by_route: dict[str, set[tuple[str, str]]],
    changed: set[tuple[str, str]],
    allowed_routes: set[str],
) -> list[str]:
    errors = []
    for route_id in sorted(reach_by_route):
        if route_id in allowed_routes:
            continue
        hits = sorted(reach_by_route[route_id] & changed)
        if hits:
            names = ", ".join(f"{module}:{qualname}" for module, qualname in hits)
            errors.append(
                f"{route_id} is served by Go, but this diff changes Python it reaches: {names}. "
                "Change the Go side and its evidence in the same diff, or flip the route back to "
                "Python in the manifest (ADR-0029 §2.9)."
            )
    return errors


# --- the tree -----------------------------------------------------------------


def _git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], cwd=ROOT, capture_output=True, text=True, check=True
    ).stdout


def _module_of(path: str) -> str:
    return path.removeprefix("services/api/").removesuffix(".py").replace("/", ".")


def _sources_at(ref: str | None) -> dict[str, str]:
    sources: dict[str, str] = {}
    if ref is None:
        for path in APP_DIR.rglob("*.py"):
            rel = path.relative_to(ROOT).as_posix()
            sources[_module_of(rel)] = path.read_text(encoding="utf-8")
        return sources
    for rel in _git(
        "ls-tree", "-r", "--name-only", ref, "--", "services/api/app"
    ).split():
        if rel.endswith(".py"):
            sources[_module_of(rel)] = _git("show", f"{ref}:{rel}")
    return sources


def _changed_lines(base: str) -> dict[str, tuple[set[int], set[int]]]:
    """path -> (lines changed in the base version, lines changed in the tree)."""
    out: dict[str, tuple[set[int], set[int]]] = {}
    current = None
    for line in _git("diff", "-U0", base, "--", "services/api/app").splitlines():
        if line.startswith("+++ "):
            target = line[4:]
            current = target[2:] if target.startswith("b/") else None
            if current is not None and current.endswith(".py"):
                out.setdefault(current, (set(), set()))
            continue
        if line.startswith("--- "):
            source = line[4:]
            if source.startswith("a/") and source.endswith(".py"):
                current = source[2:]
                out.setdefault(current, (set(), set()))
            continue
        match = HUNK.match(line)
        if match and current in out:
            old_start, old_len = int(match.group(1)), int(match.group(2) or "1")
            new_start, new_len = int(match.group(3)), int(match.group(4) or "1")
            out[current][0].update(range(old_start, old_start + max(old_len, 1)))
            out[current][1].update(range(new_start, new_start + max(new_len, 1)))
    return out


def _route_roots() -> dict[str, tuple[str, str]]:
    sys.path.insert(0, str(API_ROOT))
    from fastapi.routing import APIRoute

    from app.api.main import create_app

    roots = {}
    for route in create_app().router.routes:
        if isinstance(route, APIRoute):
            method = sorted(route.methods)[0]
            endpoint = route.endpoint
            roots[f"{method} {route.path}"] = (
                endpoint.__module__,
                endpoint.__qualname__,
            )
    return roots


def gate(base: str) -> int:
    try:
        _git("rev-parse", "--verify", base)
    except subprocess.CalledProcessError:
        print(f"::error::base {base!r} does not resolve")
        return 2
    rows = json.loads(MANIFEST.read_text(encoding="utf-8"))["routes"]
    live_go = {
        row["id"] for row in rows if row["owner"] == "go" and row["python"] == "live"
    }
    if not live_go:
        print("go-owned python touch OK: no route is served by Go with live Python")
        return 0

    try:
        roots = _route_roots()
    except Exception as error:  # noqa: BLE001 -- any import failure means "cannot run"
        print(f"::error::cannot import the Python app: {error}")
        return 2
    head_index = build_index(_sources_at(None))
    base_index = build_index(_sources_at(base))
    changed: set[tuple[str, str]] = set()
    for path, (old_lines, new_lines) in _changed_lines(base).items():
        module = _module_of(path)
        changed |= touched(head_index, module, new_lines)
        changed |= touched(base_index, module, old_lines)

    diff_paths = set(_git("diff", "--name-only", base).split())
    base_rows = {}
    try:
        base_rows = {
            row["id"]: row
            for row in json.loads(
                _git("show", f"{base}:services/core/ownership/routes.json")
            )["routes"]
        }
    except subprocess.CalledProcessError:
        pass
    core_changed = any(path.startswith("services/core/") for path in diff_paths)
    allowed = set()
    by_id = {row["id"]: row for row in rows}
    for route_id in live_go:
        evidence = by_id[route_id].get("evidence", "")
        if core_changed and evidence in diff_paths:
            allowed.add(route_id)
    # A route flipped back to Python in this diff is not in live_go at all.

    reach = {}
    for route_id in live_go:
        root = roots.get(route_id)
        if root is None:
            print(f"::error::manifest route {route_id!r} is not registered by the app")
            return 2
        reach[route_id] = reachable(head_index, root) | reachable(base_index, root)
    errors = find_violations(reach, changed, allowed)
    if errors:
        for error in errors:
            print(f"::error::{error}")
        return 1
    flipped = sorted(
        route_id
        for route_id, row in base_rows.items()
        if row.get("owner") == "go" and by_id.get(route_id, {}).get("owner") == "python"
    )
    print(
        f"go-owned python touch OK: {len(live_go)} Go-served route(s), "
        f"{len(changed)} changed function(s), {len(allowed)} allowed by Go+evidence, "
        f"{len(flipped)} flipped back"
    )
    return 0


def selftest() -> int:
    sources = {
        "app.api.routes.things": (
            "def read_thing(service):\n"
            "    return service.load_thing()\n"
            "\n"
            "def read_other(service):\n"
            "    return service.load_other()\n"
        ),
        "app.api.service": (
            "class ApiService:\n"
            "    def load_thing(self):\n"
            "        return shape(self.repository.fetch_thing())\n"
            "\n"
            "    def load_other(self):\n"
            "        return 1\n"
            "\n"
            "def shape(row):\n"
            "    return row\n"
        ),
        "app.api.repository": (
            "class Repo:\n    def fetch_thing(self):\n        return {}\n"
        ),
    }
    index = build_index(sources)
    reach = {
        "GET /things/{id}": reachable(index, ("app.api.routes.things", "read_thing"))
    }
    failures = []

    def expect(name: str, errors: list[str], red: bool) -> None:
        if bool(errors) != red:
            failures.append(
                f"{name}: expected {'red' if red else 'green'}, got {errors}"
            )

    deep = touched(index, "app.api.repository", {3})
    expect("change three calls deep", find_violations(reach, deep, set()), red=True)
    helper = touched(index, "app.api.service", {9})
    expect(
        "change a module-level helper", find_violations(reach, helper, set()), red=True
    )
    unrelated = touched(index, "app.api.service", {6})
    expect(
        "change a function the route never reaches",
        find_violations(reach, unrelated, set()),
        red=False,
    )
    expect(
        "change allowed by Go side and evidence",
        find_violations(reach, deep, {"GET /things/{id}"}),
        red=False,
    )
    innermost = touched(index, "app.api.service", {3})
    if innermost != {("app.api.service", "ApiService.load_thing")}:
        failures.append(f"innermost function resolution: {innermost}")

    if failures:
        for failure in failures:
            print(f"::error::selftest {failure}")
        return 1
    print("selftest OK: reached changes go red, unreached and allowed ones stay green")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--base", default="origin/main")
    parser.add_argument("--selftest", action="store_true")
    args = parser.parse_args()
    return selftest() if args.selftest else gate(args.base)


if __name__ == "__main__":
    raise SystemExit(main())
