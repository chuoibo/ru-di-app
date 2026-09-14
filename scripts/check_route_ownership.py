#!/usr/bin/env python3
"""Gate: the route manifest, the Python app and the Go binary agree (ADR-0029).

`services/core/ownership/routes.json` is the one answer to "which process
serves this route". Four things can make that answer wrong, and each is checked:

1. The manifest is stale: the Python app registers a route the manifest does
   not list, or in a different order. Another lane adding a route turns this
   red until a row is generated for it.
2. The Go binary and the manifest disagree. A handler Go has but the manifest
   gives to Python is dead code that looks migrated; a Go-owned row without a
   handler makes `core serve` refuse to boot on the host.
3. Routes sharing an in-memory limiter or cache have different owners. Two
   processes would each count, and the limit would silently double.
4. A Go-owned row points at evidence that is not in the tree.

    python3 scripts/check_route_ownership.py             # the gate
    python3 scripts/check_route_ownership.py --selftest  # prove it can go red

Exit codes: 0 pass, 1 fail, 2 cannot run (missing python deps or go toolchain).
A gate that cannot run is not a gate that passed.
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "services" / "core" / "ownership" / "routes.json"
CORE = ROOT / "services" / "core"


def go_owned_mismatch(rows: list[dict], go_ids: list[str]) -> list[str]:
    """Rows the manifest gives Go versus routes the binary has handlers for."""
    manifest_go = [row["id"] for row in rows if row["owner"] == "go"]
    errors = []
    for route_id in sorted(set(go_ids) - set(manifest_go)):
        errors.append(
            f"Go has a handler for {route_id!r} but the manifest owner is python"
        )
    for route_id in sorted(set(manifest_go) - set(go_ids)):
        errors.append(
            f"manifest gives {route_id!r} to Go but the binary has no handler"
        )
    return errors


def shared_state_split(rows: list[dict]) -> list[str]:
    owners: dict[str, set[str]] = defaultdict(set)
    holders: dict[str, list[str]] = defaultdict(list)
    for row in rows:
        for name in row.get("in_memory", []):
            owners[name].add(row["owner"])
            holders[name].append(row["id"])
    return [
        f"in-memory {name!r} is shared by {holders[name]} but owned by {sorted(owners[name])}"
        for name in sorted(owners)
        if len(owners[name]) > 1
    ]


def missing_evidence(rows: list[dict], root: Path) -> list[str]:
    errors = []
    for row in rows:
        if row["owner"] != "go":
            continue
        evidence = row.get("evidence", "")
        if not evidence:
            errors.append(f"{row['id']!r} is Go-owned without evidence")
        elif not (root / evidence).is_file():
            errors.append(f"{row['id']!r} evidence {evidence!r} does not exist")
    return errors


def _manifest_is_current() -> tuple[int, str]:
    result = subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "render_route_manifest.py"), "--check"],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode not in (0, 1):
        return 2, result.stderr.strip() or result.stdout.strip()
    return result.returncode, (result.stderr or result.stdout).strip()


def _go_route_ids() -> tuple[int, list[str], str]:
    if shutil.which("go") is None:
        return 2, [], "go toolchain not found on PATH"
    result = subprocess.run(
        ["go", "run", "./cmd/core", "routes", "--json"],
        cwd=CORE,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        return 2, [], result.stderr.strip()
    return 0, [view["id"] for view in json.loads(result.stdout)], ""


def gate() -> int:
    code, message = _manifest_is_current()
    if code == 2:
        print(f"::error::cannot render the manifest from the Python app: {message}")
        return 2
    errors = [] if code == 0 else [message]

    rows = json.loads(MANIFEST.read_text(encoding="utf-8"))["routes"]
    go_code, go_ids, go_message = _go_route_ids()
    if go_code == 2:
        print(f"::error::cannot ask the Go binary which routes it serves: {go_message}")
        return 2
    errors += go_owned_mismatch(rows, go_ids)
    errors += shared_state_split(rows)
    errors += missing_evidence(rows, ROOT)

    if errors:
        for error in errors:
            print(f"::error::{error}")
        return 1
    go_count = sum(1 for row in rows if row["owner"] == "go")
    print(f"route ownership OK: {len(rows)} rows, {go_count} served by Go")
    return 0


def selftest() -> int:
    """Each check must go red on the failure it exists for, and stay green otherwise."""

    def row(route_id: str, owner: str = "python", **extra) -> dict:
        return {"id": route_id, "owner": owner, **extra}

    good = [row("GET /a"), row("GET /b", "go", evidence="services/core/go.mod")]
    failures = []

    def expect(name: str, errors: list[str], red: bool) -> None:
        if bool(errors) != red:
            failures.append(
                f"{name}: expected {'red' if red else 'green'}, got {errors}"
            )

    expect("go mismatch green", go_owned_mismatch(good, ["GET /b"]), red=False)
    expect(
        "handler without manifest row",
        go_owned_mismatch(good, ["GET /a", "GET /b"]),
        red=True,
    )
    expect("manifest row without handler", go_owned_mismatch(good, []), red=True)

    shared = [
        row("GET /p", in_memory=["reason_writer"]),
        row("GET /q", in_memory=["reason_writer"]),
    ]
    expect("shared state same owner", shared_state_split(shared), red=False)
    shared[1]["owner"] = "go"
    expect("shared state split", shared_state_split(shared), red=True)

    expect("evidence present", missing_evidence(good, ROOT), red=False)
    expect("evidence absent", missing_evidence([row("GET /c", "go")], ROOT), red=True)
    expect(
        "evidence path missing",
        missing_evidence(
            [row("GET /c", "go", evidence="docs/migration/nope.json")], ROOT
        ),
        red=True,
    )

    if failures:
        for failure in failures:
            print(f"::error::selftest {failure}")
        return 1
    print("selftest OK: every check went red on its failure and green otherwise")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--selftest", action="store_true")
    args = parser.parse_args()
    return selftest() if args.selftest else gate()


if __name__ == "__main__":
    raise SystemExit(main())
