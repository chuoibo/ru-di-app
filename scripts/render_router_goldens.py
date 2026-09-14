#!/usr/bin/env python3
"""Render Starlette's routing decisions as goldens for the Go router.

The Go front door serves a request only when it is a FULL match on a Go-owned
route; everything else goes to Python. That is only safe if Go resolves a
request to the same route, in the same registration order, as Starlette does.
This script records what Starlette actually decides for several thousand
requests, and `services/core/internal/httpapi/router` asserts it agrees with
every one of them.

Nothing about matching is reimplemented here. Each case is written as raw
HTTP/1.1 bytes into uvicorn's real `HttpToolsProtocol` (the protocol the API
image runs, since httptools is installed), through uvicorn's
`ProxyHeadersMiddleware` trusting every peer (docker-compose sets
`FORWARDED_ALLOW_IPS: "*"` on `api`), into the application's real router. The
only change to the application is that every route's inner ASGI app is
replaced by a stub that reports which route it is and its path params, so a
FULL match never reaches a handler or a database. 405, 307 and 404 are the
router's own responses, read back from the bytes uvicorn wrote.

Run it inside the pinned API image. The host interpreter has other starlette
and fastapi versions, and routing details differ between them:

    docker run --rm -v "$PWD":/repo:ro -w /srv --entrypoint python \\
        mobile-parity-api:7bf58e3d /repo/scripts/render_router_goldens.py \\
        --out - > services/core/internal/httpapi/router/testdata/starlette_decisions.json

    ... render_router_goldens.py --probe-hash-seeds   # Allow ordering per seed

`Allow` for a route with more than one method is `", ".join(set)`, so its
order follows str hashing, i.e. PYTHONHASHSEED. Goldens are rendered under
PYTHONHASHSEED=0 (the script re-executes itself to guarantee that) and rows
whose order can vary carry `"allow_order_varies": true`.
"""

from __future__ import annotations

import argparse
import asyncio
import importlib.metadata
import json
import os
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "services" / "core" / "ownership" / "routes.json"
PINNED = {
    "fastapi": "0.115.6",
    "starlette": "0.41.3",
    "uvicorn": "0.34.0",
    "httptools": "0.8.0",
}
GOLDEN_HASH_SEED = "0"

METHODS = ("GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS")
# Every method llhttp accepts on an HTTP/1.1 request line, plus some it
# rejects, so the Go side's table of 400s is checked against the real parser.
PARSER_METHODS = (
    "GET HEAD POST PUT DELETE OPTIONS TRACE PATCH COPY LOCK MKCOL MOVE PROPFIND "
    "PROPPATCH SEARCH UNLOCK BIND REBIND UNBIND ACL REPORT MKACTIVITY CHECKOUT "
    "MERGE M-SEARCH NOTIFY SUBSCRIBE UNSUBSCRIBE PURGE MKCALENDAR LINK UNLINK "
    "SOURCE QUERY CONNECT PRI DESCRIBE SETUP get Get FOO GETX"
).split()
# Letter-rich samples: the repo guard blocks long digit runs in tracked files.
UUID_SAMPLE = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
NOT_UUID = "khong-phai-uuid"
HOST = "api.test:8099"
PARAM = re.compile(r"^\{([a-zA-Z_][a-zA-Z0-9_]*)(:[a-zA-Z_][a-zA-Z0-9_]*)?\}$")
# Values substituted into the first path param of every template.
PARAM_VARIANTS = (
    "aaaa%2Fbbbb",  # decodes to a slash: never matches a [^/]+ segment
    "aa%25bb",  # decodes once, to "aa%bb"
    "%C3%A9t%E2%82",  # valid UTF-8, then a truncated sequence -> U+FFFD
    "a%20b",
    "%zz",  # not an escape; kept literally
    "a%0A",  # newline inside a param
    'a"b<c>{d}|e\\f^g`h',  # printable bytes llhttp accepts unescaped
)


def _check_versions() -> None:
    found = {name: importlib.metadata.version(name) for name in PINNED}
    if found != PINNED:
        raise SystemExit(
            f"routing goldens must come from the pinned API image; found {found}, "
            f"want {PINNED}"
        )


def _import_app():
    cwd = Path.cwd()
    api_root = (
        cwd
        if (cwd / "app" / "api" / "main.py").is_file()
        else ROOT / "services" / "api"
    )
    sys.path.insert(0, str(api_root))
    from app.api.main import create_app

    return create_app()


def _route_ids(app) -> list[str]:
    """Manifest ids in registration order, after checking the app agrees."""
    from starlette.routing import Mount, Route

    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))["routes"]
    routes = app.router.routes
    if len(routes) != len(manifest):
        raise SystemExit(
            f"app has {len(routes)} routes, manifest has {len(manifest)}; "
            "rerun scripts/render_route_manifest.py"
        )
    for index, (route, row) in enumerate(zip(routes, manifest)):
        if isinstance(route, Mount):
            ok = row["kind"] == "mount" and route.path == row["path"]
        elif isinstance(route, Route):
            ok = (
                row["kind"] == "route"
                and route.path == row["path"]
                and row["method"] in (route.methods or ())
            )
        else:
            ok = False
        if not ok:
            raise SystemExit(
                f"route {index} {route!r} does not match manifest {row['id']!r}"
            )
    return [row["id"] for row in manifest]


def _stub(route_id: str):
    async def endpoint(scope, receive, send):
        params = json.dumps(
            scope.get("path_params", {}), ensure_ascii=True, sort_keys=True
        )
        await send(
            {
                "type": "http.response.start",
                "status": 200,
                "headers": [
                    (b"x-route", route_id.encode("ascii")),
                    (b"x-params", params.encode("ascii")),
                    (b"content-length", b"0"),
                ],
            }
        )
        await send({"type": "http.response.body", "body": b""})

    return endpoint


class _Recorder:
    """Pass-through ASGI app that remembers the scope path uvicorn built."""

    def __init__(self, router) -> None:
        self.router = router
        self.path: str | None = None

    async def __call__(self, scope, receive, send) -> None:
        self.path = scope["path"]
        await self.router(scope, receive, send)


class _Transport(asyncio.Transport):
    def __init__(self) -> None:
        super().__init__()
        self.written = bytearray()
        self.closed = False

    def get_extra_info(self, name, default=None):
        # No "socket"/"sockname": scope["server"] stays None, so a Host-less
        # redirect gets a relative Location (see the Go package docs).
        return {"peername": ("10.0.0.1", 40000)}.get(name, default)

    def write(self, data) -> None:
        self.written += data

    def close(self) -> None:
        self.closed = True

    def is_closing(self) -> bool:
        return self.closed

    def pause_reading(self) -> None:
        pass

    def resume_reading(self) -> None:
        pass


def _request(method: str, target: str, host: str, scheme: str) -> bytes:
    lines = [f"{method} ".encode("latin-1") + target.encode("utf-8") + b" HTTP/1.1"]
    if host:
        lines.append(b"Host: " + host.encode("utf-8"))
    if scheme:
        lines.append(b"X-Forwarded-Proto: " + scheme.encode("ascii"))
    lines.append(b"Connection: close")
    return b"\r\n".join(lines) + b"\r\n\r\n"


def _parse_response(raw: bytes) -> tuple[int, dict[str, str]]:
    head = raw.split(b"\r\n\r\n", 1)[0].decode("latin-1")
    status_line, *header_lines = head.split("\r\n")
    headers: dict[str, str] = {}
    for line in header_lines:
        name, _, value = line.partition(":")
        headers.setdefault(name.strip().lower(), value.strip())
    return int(status_line.split(" ")[1]), headers


async def _decide_all(cases: list[dict], app, route_ids: list[str]) -> None:
    from uvicorn.config import Config
    from uvicorn.protocols.http.httptools_impl import HttpToolsProtocol
    from uvicorn.server import ServerState

    by_id = {}
    for route, route_id in zip(app.router.routes, route_ids):
        route.app = _stub(route_id)
        by_id[route_id] = route
    recorder = _Recorder(app.router)
    config = Config(
        app=recorder,
        http="httptools",
        proxy_headers=True,
        forwarded_allow_ips="*",
        lifespan="off",
        log_level="critical",
        access_log=False,
    )
    config.load()

    for case in cases:
        transport = _Transport()
        protocol = HttpToolsProtocol(
            config=config, server_state=ServerState(), app_state={}
        )
        protocol.connection_made(transport)
        recorder.path = None
        protocol.data_received(
            _request(case["method"], case["target"], case["host"], case["scheme"])
        )
        for _ in range(1000):
            if transport.closed:
                break
            await asyncio.sleep(0)
        else:
            raise SystemExit(f"no response for {case}")
        protocol.connection_lost(None)

        status, headers = _parse_response(bytes(transport.written))
        if recorder.path is not None:
            case["path"] = recorder.path
        if status == 200 and "x-route" in headers:
            case["kind"] = "full"
            case["route"] = headers["x-route"]
            params = json.loads(headers["x-params"])
            if params:
                case["params"] = params
        elif status == 405:
            case["kind"] = "method_not_allowed"
            case["allow"] = headers["allow"]
            if (
                headers["allow"].count(",")
                and len(set(headers["allow"].split(", "))) > 1
            ):
                case["allow_order_varies"] = True
        elif status == 307:
            case["kind"] = "redirect"
            case["location"] = headers["location"]
        elif status == 404:
            case["kind"] = "not_found"
        elif status == 400 and recorder.path is None:
            case["kind"] = "bad_request"
        else:
            raise SystemExit(f"unexpected {status} {headers} for {case}")


def _segments(template: str) -> list[str]:
    return template.split("/")[1:]


def _is_param(segment: str) -> bool:
    return PARAM.match(segment) is not None


def _instantiate(segments: list[str], values: dict[int, str]) -> str:
    out = []
    for index, segment in enumerate(segments):
        out.append(values.get(index, UUID_SAMPLE) if _is_param(segment) else segment)
    return "/" + "/".join(out)


def _collisions(
    template: list[str], templates: list[list[str]]
) -> list[dict[int, str]]:
    """Literal segments another template has where this one has a param."""
    found: list[dict[int, str]] = []
    for index, segment in enumerate(template):
        if not _is_param(segment):
            continue
        literals = []
        for other in templates:
            if len(other) <= index or _is_param(other[index]):
                continue
            prefix_ok = all(
                other[j] == template[j] or _is_param(other[j]) or _is_param(template[j])
                for j in range(index)
            )
            if prefix_ok and other[index] not in literals:
                literals.append(other[index])
        found.extend({index: literal} for literal in literals)
    return found


def build_cases(app) -> list[dict]:
    from starlette.routing import Mount

    cases: list[dict] = []
    seen: set[tuple[str, str, str, str]] = set()

    def add(method: str, target: str, host: str = HOST, scheme: str = "http") -> None:
        key = (method, target, host, scheme)
        if key not in seen:
            seen.add(key)
            cases.append(
                {"method": method, "target": target, "host": host, "scheme": scheme}
            )

    own_methods: dict[str, set[str]] = {}
    for route in app.router.routes:
        if not isinstance(route, Mount):
            own_methods.setdefault(route.path, set()).update(route.methods or ())
    templates = [_segments(path) for path in own_methods]

    for path, methods in own_methods.items():
        segments = _segments(path)
        for segment in segments:
            if "{" in segment and not _is_param(segment):
                raise SystemExit(f"{path}: params inside a segment are not enumerated")
        params = [i for i, s in enumerate(segments) if _is_param(s)]
        base = _instantiate(segments, {})
        focus = sorted(methods | {"GET"})

        for method in METHODS:
            add(method, base)
            add(method, base + "/")
        if params:
            for method in METHODS:
                add(method, _instantiate(segments, {i: NOT_UUID for i in params}))
                for values in _collisions(segments, templates):
                    add(method, _instantiate(segments, values))
            for method in focus:
                for variant in PARAM_VARIANTS:
                    add(method, _instantiate(segments, {params[0]: variant}))
        for method in focus:
            add(method, base + "%0A")
            add(method, base + "%0A%0A")
            add(method, base + "//")
            add(method, "/" + base)
            if len(segments) > 1:
                add(method, "/" + segments[0] + "/" + base[len(segments[0]) + 1 :])
            add(method, base + "?x=1#frag")
            add(method, base + "#frag/")
            add(method, base + "/?x=1&y=%2F&z=a%20b")
            add(method, base + '/?q="<{|}>^`\\')
        add("GET", base + "/", host="")
        add("GET", base + "/", scheme="https")

    for method in METHODS:
        for target in (
            "/static/x.css",
            "/static",
            "/static/",
            "/static/a/b.css",
            "/static/x.css/",
            "/staticx",
            "/static%2Fx.css",
            "/static/..%2F..%2Fsecret",
            "/static/x.css%0A",
            "/static/a%0Ab",
            "/",
            "//",
            "///",
            "/?x=1",
            "/%2F",
            "/khong-co",
            "/khong-co/",
            "/people",
            "/people/",
            "/contexts",
            "/g",
            "/g/",
            "/HEALTHZ",
            "/healthz%0A",
            "/people/me%0A/",
        ):
            add(method, target)
    for method in ("OPTIONS", "GET"):
        add(method, "*")
    for target in ("/posts", "/docs", "/posts/"):
        for method in PARSER_METHODS:
            add(method, target)
    for target in ("/posts/é", "/posts\x7f", "/posts\tx", "/posts?x=é"):
        add("GET", target)
    # CPython replaces invalid UTF-8 per maximal subpart; Go's decoder does
    # not, so every shape of broken sequence is pinned, in a param and in a
    # redirect Location.
    for escaped in (
        "%ED%A0%80",
        "%F4%90%80%80",
        "%F0%9F%98",
        "%F0%9F%98%80",
        "%C0%AF",
        "%E0%80%80",
        "%E0%A0",
        "%F0%80",
        "%80",
        "%FF%FE",
        "%E2%82%AC%E2",
        "%e2%82%ac",
        "%%41",
        "%4",
        "%a",
        "%2f",
        "a%",
        "%C3%A9%0A",
    ):
        add("GET", "/posts/" + escaped)
        add("GET", "/posts/" + escaped + "/")
    for host in ("", "hést.test", "[::1]:8099", "api.test"):
        add("GET", "/posts/", host=host)
        add("GET", "/people/me/?a=b", host=host)
    for scheme in ("https", "ws", "wss", "bogus", ""):
        add("GET", "/posts/", scheme=scheme)
        add("GET", "/static?x=1", scheme=scheme)
    return cases


def _probe_hash_seeds() -> int:
    orders: dict[str, dict[str, list[str]]] = {}
    for seed in range(8):
        out = subprocess.run(
            [sys.executable, __file__, "--allow-orders"],
            env={**os.environ, "PYTHONHASHSEED": str(seed)},
            check=True,
            capture_output=True,
            text=True,
        ).stdout
        for route_id, allow in json.loads(out).items():
            orders.setdefault(route_id, {}).setdefault(allow, []).append(str(seed))
    for route_id, seen in orders.items():
        described = "; ".join(
            f"{allow!r} under seeds {','.join(s)}" for allow, s in seen.items()
        )
        print(f"{route_id}: {described}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    parser.add_argument("--out", help="write goldens here, '-' for stdout")
    parser.add_argument("--check", help="exit 1 if this goldens file is stale")
    parser.add_argument("--probe-hash-seeds", action="store_true")
    parser.add_argument("--allow-orders", action="store_true", help=argparse.SUPPRESS)
    args = parser.parse_args()
    _check_versions()

    if args.probe_hash_seeds:
        return _probe_hash_seeds()
    if args.allow_orders:
        app = _import_app()
        multi = {
            route_id: ", ".join(route.methods)
            for route, route_id in zip(app.router.routes, _route_ids(app))
            if len(getattr(route, "methods", None) or ()) > 1
        }
        print(json.dumps(multi))
        return 0
    if bool(args.out) == bool(args.check):
        parser.error("give exactly one of --out or --check")
    if os.environ.get("PYTHONHASHSEED") != GOLDEN_HASH_SEED:
        env = {**os.environ, "PYTHONHASHSEED": GOLDEN_HASH_SEED}
        os.execve(sys.executable, [sys.executable, *sys.argv], env)

    app = _import_app()
    route_ids = _route_ids(app)
    cases = build_cases(app)
    asyncio.run(_decide_all(cases, app, route_ids))
    rendered = (
        "[\n"
        + ",\n".join(
            json.dumps(case, ensure_ascii=True, separators=(",", ":")) for case in cases
        )
        + "\n]\n"
    )
    if args.check:
        if Path(args.check).read_text(encoding="utf-8") != rendered:
            print(f"{args.check} is stale; rerun with --out", file=sys.stderr)
            return 1
        return 0
    if args.out == "-":
        sys.stdout.write(rendered)
    else:
        Path(args.out).write_text(rendered, encoding="utf-8")
    print(f"{len(cases)} cases", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
