#!/usr/bin/env python3
"""Generate services/core/ownership/routes.json from the Python app.

The manifest is the single answer to which process serves each route
(ADR-0029 §2.1). Rows follow the app's registration order because Starlette
matches routes in that order, and the Go router has to match the same way.

Migration state (owner, python, state, evidence) is carried over by route id,
so regenerating after another lane adds a route never resets a migrated one.
A route that disappeared from the app is an error unless --prune is given:
silently dropping a Go-owned row would make the front door forget it.

In-memory state (rate limiters, the place-reason cache) is attributed to the
routes that touch it by reading the router modules' syntax tree, not from a
hand-kept table. Routes sharing one of those objects must move together,
because splitting them across two processes doubles the limit.

    python3 scripts/render_route_manifest.py           # write
    python3 scripts/render_route_manifest.py --check   # exit 1 on drift
"""

from __future__ import annotations

import argparse
import ast
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
API_ROOT = ROOT / "services" / "api"
MANIFEST = ROOT / "services" / "core" / "ownership" / "routes.json"

# Routes whose primary purpose is a model call (ADR-0029 §2.1).
AI_ROUTES = {
    "POST /contexts/{context_id}/messages/{message_id}/expense-draft",
    "POST /contexts/{context_id}/ai-turn",
    "POST /places/search",
    "POST /receipts/scan",
    "POST /screenshots/scan",
    "GET /contexts/{context_id}/suggestion",
    "GET /contexts/{context_id}/contextual-suggestion",
    "GET /contexts/{context_id}/albums/{outing_id}/reel",
    "POST /contexts/{context_id}/photos/{photo_id}/face-boxes",
}
# Core routes with one model-backed step inside their flow.
MIXED_ROUTES = {
    "POST /contexts/{context_id}/messages",
    "GET /places",
    "GET /places/{place_id}",
}
IN_MEMORY_SUFFIXES = ("_limit", "_limiter", "reason_writer")
CARRIED_FIELDS = ("owner", "python", "state", "evidence")


def _state_names(path: Path) -> dict[str, set[str]]:
    """Map each function in a module to the in-memory objects it reaches."""
    tree = ast.parse(path.read_text(encoding="utf-8"))
    own: dict[str, set[str]] = {}
    calls: dict[str, set[str]] = {}
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef | ast.AsyncFunctionDef):
            continue
        names: set[str] = set()
        called: set[str] = set()
        for sub in ast.walk(node):
            if (
                isinstance(sub, ast.Attribute)
                and isinstance(sub.value, ast.Attribute)
                and sub.value.attr == "state"
            ):
                names.add(sub.attr)
            elif isinstance(sub, ast.Constant) and isinstance(sub.value, str):
                # getattr(app.state, "otp_request_limit") or a helper taking
                # the attribute name as an argument.
                names.add(sub.value)
            elif isinstance(sub, ast.Name):
                # Every referenced name, not only direct calls: limiters are
                # usually reached through Depends(_some_limiter), where the
                # helper is an argument rather than the thing being called.
                called.add(sub.id)
        own[node.name] = {n for n in names if n.endswith(IN_MEMORY_SUFFIXES)}
        calls[node.name] = called
    total = {name: set(values) for name, values in own.items()}
    changed = True
    while changed:
        changed = False
        for name, callees in calls.items():
            for callee in callees:
                extra = total.get(callee, set()) - total[name]
                if extra:
                    total[name] |= extra
                    changed = True
    return total


def _app_rows() -> list[dict]:
    sys.path.insert(0, str(API_ROOT))
    from fastapi.routing import APIRoute
    from starlette.routing import Mount, Route

    from app.api.main import create_app

    app = create_app()
    rows: list[dict] = []
    module_states: dict[str, dict[str, set[str]]] = {}
    for order, route in enumerate(app.router.routes):
        row: dict = {"order": order}
        if isinstance(route, Mount):
            row.update(
                id=f"MOUNT {route.path}",
                kind="mount",
                method="",
                path=route.path,
                group="static",
                **{"class": "framework"},
            )
        elif isinstance(route, APIRoute):
            methods = sorted(route.methods)
            if len(methods) != 1:
                raise SystemExit(f"{route.path}: expected one method, got {methods}")
            route_id = f"{methods[0]} {route.path}"
            module = route.endpoint.__module__
            group = (
                module.rsplit(".", 1)[-1]
                if module.startswith("app.api.routes.")
                else "main"
            )
            if route_id in AI_ROUTES:
                kind_class = "ai"
            elif route_id in MIXED_ROUTES:
                kind_class = "mixed"
            else:
                kind_class = "core"
            row.update(
                id=route_id,
                kind="route",
                method=methods[0],
                path=route.path,
                group=group,
                **{"class": kind_class},
            )
            source = Path(route.endpoint.__code__.co_filename)
            states = module_states.setdefault(str(source), _state_names(source))
            in_memory = sorted(states.get(route.endpoint.__name__, set()))
            if in_memory:
                row["in_memory"] = in_memory
        elif isinstance(route, Route):
            methods = sorted(route.methods - {"HEAD"})
            if len(methods) != 1:
                raise SystemExit(f"{route.path}: expected one method, got {methods}")
            row.update(
                id=f"{methods[0]} {route.path}",
                kind="route",
                method=methods[0],
                path=route.path,
                group="framework",
                **{"class": "framework"},
            )
        else:
            raise SystemExit(f"unknown route type {type(route).__name__}")
        rows.append(row)

    ids = {row["id"] for row in rows}
    missing = (AI_ROUTES | MIXED_ROUTES) - ids
    if missing:
        raise SystemExit(f"classified routes not served by the app: {sorted(missing)}")
    return rows


def _ordered(row: dict) -> dict:
    keys = (
        "id",
        "order",
        "kind",
        "method",
        "path",
        "group",
        "class",
        "owner",
        "python",
        "in_memory",
        "state",
        "evidence",
    )
    optional = {"in_memory", "evidence"}
    return {
        key: row[key]
        for key in keys
        if key in row and not (key in optional and not row[key])
    }


def build(previous: dict | None, prune: bool) -> dict:
    rows = _app_rows()
    old = {r["id"]: r for r in (previous or {}).get("routes", [])}
    gone = sorted(set(old) - {row["id"] for row in rows})
    if gone and not prune:
        raise SystemExit(f"routes left the app, rerun with --prune if intended: {gone}")
    for row in rows:
        row.update(owner="python", python="live", state="PY")
        carried = old.get(row["id"], {})
        for field in CARRIED_FIELDS:
            if field in carried:
                row[field] = carried[field]
    return {"schema": 1, "routes": [_ordered(row) for row in rows]}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--check", action="store_true", help="exit 1 if the file is stale"
    )
    parser.add_argument(
        "--prune", action="store_true", help="drop rows for removed routes"
    )
    args = parser.parse_args()

    previous = (
        json.loads(MANIFEST.read_text(encoding="utf-8")) if MANIFEST.exists() else None
    )
    rendered = (
        json.dumps(build(previous, args.prune), ensure_ascii=False, indent=2) + "\n"
    )
    if args.check:
        current = MANIFEST.read_text(encoding="utf-8") if MANIFEST.exists() else ""
        if current != rendered:
            print(
                f"{MANIFEST.relative_to(ROOT)} is stale; run scripts/render_route_manifest.py",
                file=sys.stderr,
            )
            return 1
        print("route manifest is current")
        return 0
    MANIFEST.parent.mkdir(parents=True, exist_ok=True)
    MANIFEST.write_text(rendered, encoding="utf-8")
    print(f"wrote {MANIFEST.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
