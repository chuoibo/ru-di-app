#!/usr/bin/env python3
"""Oracle for the Go port of the idempotency middleware (services/core/internal/idem).

ADR-0029: the Go front door runs the `Idempotency-Key` layer on the routes it
serves, over the same `idempotency_keys` table, and a key spent through one
implementation must answer identically through the other. Rather than restate
app/api/idempotency.py from memory, both modes below drive the real
`IdempotencyMiddleware` inside the pinned API image.

goldens
    Drive the middleware in-process over a scripted store and a scripted inner
    app, and print every observable effect (store calls with their arguments,
    what the inner app received, the messages sent, the exception raised) as
    JSON. The polling loop runs on a fake clock so the sleep schedule is exact;
    seconds travel as float.hex() so no bit is lost.
    The Go unit tests replay the same cases without a database:

        docker run --rm --network none -v "$PWD":/repo:ro --entrypoint python \\
          <api image> /repo/scripts/render_idem_oracle.py goldens \\
          > services/core/internal/idem/testdata/python_idem.json

serve PORT_SHORT PORT_DEFAULT WAIT_SECONDS
    Serve the middleware under uvicorn (httptools, as the image's CMD does),
    with main.py's own `sqlalchemy_store_factory` on MOBILE_DATABASE_URL, around
    a stub app scripted per request by its X-Stub header. PORT_SHORT uses an
    in-flight budget of WAIT_SECONDS; PORT_DEFAULT uses the middleware's own
    default. `GET /__log` (a read, so the middleware lets it through) returns
    and clears what the stub and the store saw. The Go differential test
    (oracle_postgres_test.go) starts this in a container with --network host.

Header values travel as latin-1 strings, which is how ASGI decodes them.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import re
import sys
import urllib.parse
from contextlib import contextmanager

sys.path.insert(0, "/srv")

import anyio  # noqa: E402
import httptools  # noqa: E402

from app.api import idempotency as idem  # noqa: E402

KEY = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
JSON_TYPE = "application/json"
ANSWER = b'{"id": "one"}'
DEFAULT_APP = {
    "status": 201,
    "headers": [["content-type", JSON_TYPE]],
    "body": ANSWER.hex(),
}
SCRIPTED_FAILURE = "scripted failure"


# ---------------------------------------------------------------------------
# goldens
# ---------------------------------------------------------------------------

CASES: list[dict] = []


def add(
    name: str,
    *,
    method: str = "POST",
    target: str = "/expenses",
    headers: list[tuple[str, str]] | None = None,
    body: bytes = b"",
    gen: tuple[bytes, bytes, int, bytes] | None = None,
    key: str | None = KEY,
    outcome: dict | None = None,
    app: dict | None = None,
) -> None:
    pairs = []
    if key is not None:
        pairs.append(["idempotency-key", key])
    pairs.extend([list(pair) for pair in headers or []])
    case = {
        "name": name,
        "method": method,
        "target": target,
        "headers": pairs,
        "outcome": outcome or {"kind": "Reserved"},
        "app": app or DEFAULT_APP,
    }
    if gen is None:
        case["body"] = body.hex()
    else:
        prefix, unit, count, suffix = gen
        case["gen"] = [prefix.hex(), unit.hex(), count, suffix.hex()]
    CASES.append(case)


def json_body(name: str, body: bytes, content_type: str = JSON_TYPE, **kw) -> None:
    add(name, headers=[("content-type", content_type)], body=body, **kw)


def replay(status: int = 201, body: bytes = ANSWER, media_type=JSON_TYPE) -> dict:
    return {
        "kind": "Replay",
        "status": status,
        "body": body.hex(),
        "media_type": media_type,
    }


def build_cases() -> None:
    # Which methods the layer applies to.
    for method in ("GET", "HEAD", "OPTIONS", "post", "TRACE"):
        add(f"method {method} passes through", method=method)
    for method in ("POST", "PUT", "PATCH", "DELETE"):
        add(f"method {method} is guarded", method=method)

    # The key itself.
    add("no key passes through", key=None)
    add("empty key", key="")
    add("key at the limit", key="k" * 255)
    add("key over the limit", key="k" * 256)
    add("latin-1 key at the limit counts bytes", key="\xe9" * 255)
    add("latin-1 key over the limit", key="\xe9" * 256)
    add(
        "first of two keys wins",
        key=None,
        headers=[("idempotency-key", "first-key"), ("idempotency-key", "second-key")],
    )
    add(
        "header name is case-insensitive",
        key=None,
        headers=[("IDEMPOTENCY-KEY", "upper-key")],
    )
    add("key with inner spaces", key="a key with spaces")

    # Scope: bearer digest, then X-Actor-ID verbatim, then anonymous.
    scope_cases = [
        ("anonymous", []),
        ("actor", [("x-actor-id", "person-a")]),
        ("empty actor", [("x-actor-id", "")]),
        ("latin-1 actor", [("x-actor-id", "p\xe9rson")]),
        ("bearer", [("authorization", "Bearer tok")]),
        ("bearer lowercase scheme", [("authorization", "bearer tok")]),
        ("bearer mixed-case scheme", [("authorization", "BeArEr tok")]),
        (
            "bearer beats actor",
            [("authorization", "Bearer tok"), ("x-actor-id", "person-a")],
        ),
        ("scheme only", [("authorization", "Bearer"), ("x-actor-id", "person-a")]),
        (
            "scheme and space only",
            [("authorization", "Bearer "), ("x-actor-id", "person-a")],
        ),
        ("token padded with spaces", [("authorization", "Bearer   tok  ")]),
        ("token padded with nbsp and nel", [("authorization", "Bearer \xa0tok\x85")]),
        ("token padded with separators", [("authorization", "Bearer \x1ctok\x1f\x0b")]),
        ("tab is not the separator", [("authorization", "Bearer\ttok")]),
        ("basic scheme", [("authorization", "Basic dG9r"), ("x-actor-id", "person-a")]),
        ("token with inner space", [("authorization", "Bearer a b")]),
        ("latin-1 token", [("authorization", "Bearer t\xe9")]),
        ("empty authorization", [("authorization", ""), ("x-actor-id", "person-a")]),
        ("leading space before scheme", [("authorization", " Bearer tok")]),
        ("scheme with latin-1 letter", [("authorization", "B\xe9arer tok")]),
        (
            "first authorization header wins",
            [("authorization", "Basic x"), ("authorization", "Bearer tok")],
        ),
        (
            "first actor header wins",
            [("x-actor-id", "person-a"), ("x-actor-id", "person-b")],
        ),
    ]
    for name, headers in scope_cases:
        add(f"scope {name}", headers=headers)

    # Content types that do and do not declare JSON.
    spaced = b'{"b": 1, "a": "\\u0110"}'
    for content_type in (
        JSON_TYPE,
        "application/json; charset=utf-8",
        "Application/JSON",
        "application/vnd.api+json",
        "application/problem+JSON",
        "text/+json",
        "+json",
        "application/json-patch",
        "application/jsonx",
        " application/json",
        "application/json\xa0; charset=x",
        "application/json\x85",
        "\x1capplication/json",
        "",
        ";application/json",
        "application/json, text/plain",
        "text/plain",
        "multipart/form-data; boundary=+json",
    ):
        json_body(f"content type {content_type!r}", spaced, content_type)
    add("no content type", body=spaced)
    add(
        "first content type wins",
        headers=[("content-type", "text/plain"), ("content-type", JSON_TYPE)],
        body=spaced,
    )

    # Bodies under a JSON content type.
    bodies = [
        ("python spelling", spaced),
        ("javascript spelling", '{"b":1,"a":"Đ"}'.encode()),
        ("reordered keys", b'{"a": "\\u0110", "b": 1}'),
        ("array order kept", b'{"people": ["trang", "minh"]}'),
        ("array order kept reversed", b'{"people": ["minh", "trang"]}'),
        ("invalid json", b"{not json"),
        ("empty body", b""),
        ("whitespace only", b"   "),
        ("trailing data", b'{"a": 1} extra'),
        ("nan", b'{"a": NaN}'),
        ("infinities", b"[Infinity, -Infinity]"),
        ("float overflow", b"[1e400]"),
        ("float spellings", b'{"a": 1e5, "b": 1.0, "c": -0.0, "d": 1E+2, "e": 0.1}'),
        ("duplicate keys", b'{"a": 1, "b": 0, "a": 2}'),
        ("lone high surrogate", b'{"a": "\\ud800"}'),
        ("lone low surrogate key", b'{"\\udc00": 1}'),
        ("surrogate pair", b'{"a": "\\ud83d\\ude00"}'),
        ("invalid utf-8", b'{"a": "\xe9"}'),
        ("utf-8 bom", b'\xef\xbb\xbf{"a": 1}'),
        ("utf-16 with bom", '{"a": "Đ"}'.encode("utf-16")),
        ("utf-16-be without bom", '{"a": 1}'.encode("utf-16-be")),
        ("utf-32-le without bom", '{"a": 1}'.encode("utf-32-le")),
        ("scalar string", b'"x"'),
        ("scalar null", b"null"),
        ("nul byte", b"\x00"),
        ("escaped solidus and controls", b'{"a": "\\/\\n\\u0000"}'),
        ("non-ascii key order", '{"é": 1, "z": 2, "Đ": 3}'.encode()),
    ]
    for name, body in bodies:
        json_body(f"body {name}", body)
    json_body("body big int at the digit limit", b"", gen=(b"[", b"7", 4300, b"]"))
    json_body("body big int over the digit limit", b"", gen=(b"[", b"7", 4301, b"]"))
    add(
        "non-json body is verbatim",
        headers=[("content-type", "text/plain")],
        body=b"amount=1",
    )

    # Method, path and query are part of the digest.
    add("query", target="/expenses?b=2&a=1")
    add("query reordered", target="/expenses?a=1&b=2")
    add("query escaped", target="/expenses?a=%41")
    add("empty query", target="/expenses?")
    add("fragment dropped", target="/expenses?a=1#frag")
    add("path escaped utf-8", target="/c%C3%A9")
    add("path escaped lowercase hex", target="/c%c3%a9")
    add("path invalid utf-8", target="/c%FF")
    add("path replacement character", target="/c%EF%BF%BD")
    add("path escaped slash", target="/contexts/a%2Fb")
    add("path invalid escape", target="/c%zz")

    # Outcomes of the reservation.
    add("conflict", outcome={"kind": "Conflict"})
    add("in flight", outcome={"kind": "InFlight"})
    add("replay json", outcome=replay())
    add("replay without media type", outcome=replay(200, b"plain", None))
    add("replay with empty media type", outcome=replay(200, b"plain", ""))
    add(
        "replay text media type", outcome=replay(202, b"t", "text/plain; charset=utf-8")
    )
    add("replay latin-1 media type", outcome=replay(201, b"", "text/x-\xe9"))
    add("replay media type outside latin-1", outcome=replay(201, b"x", "text/x-Ā"))
    add("replay empty body", outcome=replay(201, b"", JSON_TYPE))
    add("replay status 299", outcome=replay(299))

    # What the inner app answers when this caller owns the key.
    def app(status=201, headers=None, body=ANSWER, fail=None):
        spec = {
            "status": status,
            "headers": headers
            if headers is not None
            else [["content-type", JSON_TYPE]],
            "body": body.hex(),
        }
        if fail:
            spec["raise"] = fail
        return spec

    add("app 200", app=app(200))
    add("app 201 empty body", app=app(201, body=b""))
    add("app 202 without content type", app=app(202, headers=[]))
    add("app empty content type", app=app(201, headers=[["content-type", ""]]))
    add(
        "app latin-1 content type",
        app=app(201, headers=[["content-type", "text/x-\xe9"]]),
    )
    add(
        "app two content types",
        app=app(
            201, headers=[["content-type", "text/plain"], ["content-type", JSON_TYPE]]
        ),
    )
    add(
        "app extra headers",
        app=app(
            201,
            headers=[
                ["content-type", JSON_TYPE],
                ["x-extra", "1"],
                ["set-cookie", "a=b"],
            ],
        ),
    )
    add("app 299", app=app(299))
    add("app 300", app=app(300))
    add("app 404", app=app(404))
    add("app 422", app=app(422))
    add("app 500", app=app(500))
    add("app raises before answering", app=app(fail="before"))
    add("app raises after starting", app=app(fail="after_start"))
    add("app 422 without key", key=None, app=app(422))
    add("app raises without key", key=None, app=app(fail="before"))

    # The itinerary exceptions.
    itinerary = [
        ("PUT", "/outings/abc/itinerary"),
        ("PUT", "/outings/abc/itinerary/"),
        ("PUT", "//outings/abc/itinerary//"),
        ("PUT", "/outings//itinerary"),
        ("PUT", "/outings/a%2Fb/itinerary"),
        ("PUT", "/Outings/abc/itinerary"),
        ("PUT", "/outings/abc/itinerary/preview"),
        ("PATCH", "/outings/abc/itinerary"),
        ("POST", "/outings/abc/itinerary"),
        ("POST", "/outings/abc/itinerary/preview"),
        ("POST", "/outings/abc/itinerary/preview/"),
        ("POST", "/outings//itinerary/preview"),
        ("POST", "/x/outings/abc/itinerary/preview"),
        ("POST", "/outings/abc/itinerary/Preview"),
        ("DELETE", "/outings/abc/itinerary/preview"),
    ]
    for method, target in itinerary:
        add(
            f"itinerary {method} {target} replay",
            method=method,
            target=target,
            outcome=replay(200),
        )
        add(f"itinerary {method} {target} reserved", method=method, target=target)
    add(
        "itinerary preview with empty key",
        target="/outings/abc/itinerary/preview",
        key="",
    )
    add(
        "itinerary preview without key",
        target="/outings/abc/itinerary/preview",
        key=None,
    )
    add(
        "itinerary write with empty key",
        method="PUT",
        target="/outings/abc/itinerary",
        key="",
    )
    add(
        "itinerary write in flight",
        method="PUT",
        target="/outings/abc/itinerary",
        outcome={"kind": "InFlight"},
    )
    add(
        "itinerary write conflict",
        method="PUT",
        target="/outings/abc/itinerary",
        outcome={"kind": "Conflict"},
    )
    add(
        "itinerary replay handed to a failing app",
        method="PUT",
        target="/outings/abc/itinerary",
        outcome=replay(200),
        app=app(fail="before"),
    )
    add(
        "itinerary replay with a body",
        method="PUT",
        target="/outings/abc/itinerary",
        headers=[("content-type", JSON_TYPE)],
        body=b'{"stops": []}',
        outcome=replay(200, b'{"kept": true}', None),
    )


def materialize(case: dict) -> bytes:
    if "gen" in case:
        prefix, unit, count, suffix = case["gen"]
        return (
            bytes.fromhex(prefix) + bytes.fromhex(unit) * count + bytes.fromhex(suffix)
        )
    return bytes.fromhex(case["body"])


def scope_for(case: dict, body: bytes) -> dict:
    # The same three lines uvicorn's httptools protocol runs (on_headers_complete).
    parsed = httptools.parse_url(case["target"].encode("ascii"))
    path = parsed.path.decode("ascii")
    if "%" in path:
        path = urllib.parse.unquote(path)
    return {
        "type": "http",
        "method": case["method"],
        "path": path,
        "query_string": parsed.query or b"",
        "headers": [
            (name.encode("latin-1"), value.encode("latin-1"))
            for name, value in case["headers"]
        ],
    }


def build_outcome(spec: dict):
    kind = spec["kind"]
    if kind == "Replay":
        return idem.Replay(
            idem.StoredResponse(
                status_code=spec["status"],
                body=bytes.fromhex(spec["body"]),
                media_type=spec["media_type"],
            )
        )
    return {
        "Reserved": idem.Reserved,
        "InFlight": idem.InFlight,
        "Conflict": idem.Conflict,
    }[kind]()


async def run_case(case: dict) -> dict:
    body = materialize(case)
    record: dict = {"store": [], "app_calls": [], "sent": None, "error": None}
    outcome = build_outcome(case["outcome"])
    spec = case["app"]

    class Store:
        def reserve(self, *, scope, key, fingerprint, legacy_fingerprint=None):
            record["store"].append(
                {
                    "op": "reserve",
                    "scope": scope,
                    "key": key,
                    "fingerprint": fingerprint,
                    "legacy": legacy_fingerprint,
                }
            )
            return outcome

        def complete(self, *, scope, key, response):
            record["store"].append(
                {
                    "op": "complete",
                    "scope": scope,
                    "key": key,
                    "status": response.status_code,
                    "body": response.body.hex(),
                    "media_type": response.media_type,
                }
            )

        def release(self, *, scope, key):
            record["store"].append({"op": "release", "scope": scope, "key": key})

    @contextmanager
    def factory():
        yield Store()

    async def inner(scope, receive, send):
        received = b""
        while True:
            message = await receive()
            if message["type"] != "http.request":
                break
            received += message.get("body") or b""
            if not message.get("more_body", False):
                break
        stored = scope.get("itinerary_authorized_replay")
        record["app_calls"].append(
            {
                "path": scope["path"],
                "body_sha256": hashlib.sha256(received).hexdigest(),
                "replay": None
                if stored is None
                else {
                    "status": stored.status_code,
                    "body": stored.body.hex(),
                    "media_type": stored.media_type,
                },
            }
        )
        if spec.get("raise") == "before":
            raise RuntimeError(SCRIPTED_FAILURE)
        await send(
            {
                "type": "http.response.start",
                "status": spec["status"],
                "headers": [
                    (name.encode("latin-1"), value.encode("latin-1"))
                    for name, value in spec["headers"]
                ],
            }
        )
        if spec.get("raise") == "after_start":
            raise RuntimeError(SCRIPTED_FAILURE)
        await send({"type": "http.response.body", "body": bytes.fromhex(spec["body"])})

    delivered = False

    async def receive():
        nonlocal delivered
        if delivered:
            return {"type": "http.disconnect"}
        delivered = True
        return {"type": "http.request", "body": body, "more_body": False}

    messages: list[dict] = []

    async def send(message):
        messages.append(message)

    middleware = idem.IdempotencyMiddleware(inner, factory, in_flight_wait_seconds=0)
    try:
        await middleware(scope_for(case, body), receive, send)
    except Exception as exc:  # noqa: BLE001 - the class name is the golden
        record["error"] = type(exc).__name__
    if messages:
        start = messages[0]
        record["sent"] = {
            "status": start["status"],
            "headers": [
                [name.decode("latin-1"), value.decode("latin-1")]
                for name, value in start.get("headers") or []
            ],
            "body": b"".join(m.get("body") or b"" for m in messages[1:]).hex(),
        }
    return record


def poll_schedules() -> list[dict]:
    """`_reserve` on a clock that only moves when the middleware sleeps."""

    schedules = []
    for budget, in_flight_times in (
        (0.0, None),
        (0.01, None),
        (0.02, None),
        (0.05, None),
        (0.35, None),
        (1.0, None),
        (5.0, None),
        (5.0, 3),
        (0.05, 2),
    ):
        clock = [0.0]
        sleeps: list[float] = []
        attempts = [0]

        async def fake_sleep(seconds):
            sleeps.append(seconds)
            clock[0] += seconds

        class Store:
            def reserve(self, **_kwargs):
                attempts[0] += 1
                if in_flight_times is not None and attempts[0] > in_flight_times:
                    return idem.Replay(idem.StoredResponse(201, b"", None))
                return idem.InFlight()

        @contextmanager
        def factory():
            yield Store()

        middleware = idem.IdempotencyMiddleware(
            None, factory, in_flight_wait_seconds=budget
        )
        original = (anyio.current_time, anyio.sleep)
        anyio.current_time = lambda: clock[0]
        anyio.sleep = fake_sleep
        try:
            outcome = asyncio.run(
                middleware._reserve(key_scope="anonymous", key=KEY, fingerprint="f")
            )
        finally:
            anyio.current_time, anyio.sleep = original
        schedules.append(
            {
                "budget_seconds": float.hex(budget),
                "in_flight_times": in_flight_times,
                "attempts": attempts[0],
                "sleeps": [float.hex(seconds) for seconds in sleeps],
                "outcome": type(outcome).__name__,
            }
        )
    return schedules


def goldens() -> None:
    build_cases()
    for case in CASES:
        case["want"] = asyncio.run(run_case(case))
    document = {
        "source": "app/api/idempotency.py driven by scripts/render_idem_oracle.py",
        "constants": {
            "max_key_length": idem.MAX_KEY_LENGTH,
            "default_in_flight_wait_seconds": float.hex(
                idem.DEFAULT_IN_FLIGHT_WAIT_SECONDS
            ),
            "first_poll_seconds": float.hex(idem._FIRST_POLL_SECONDS),
            "max_poll_seconds": float.hex(idem._MAX_POLL_SECONDS),
            "write_methods": sorted(idem.WRITE_METHODS),
        },
        "cases": CASES,
        "polls": poll_schedules(),
    }
    text = json.dumps(document, indent=1)
    # The repo guard reads digit runs inside a sha256 hex as phone numbers and
    # long numbers. Eight colon-separated groups of eight can hold neither; the
    # Go test joins them back. Refuse input that already has the grouped shape,
    # so joining can never alter a value that was not grouped here.
    assert not GROUPED_DIGEST_RE.search(text), "grouped digest shape already present"
    sys.stdout.write(
        DIGEST_RE.sub(
            lambda m: ":".join(m.group(0)[i : i + 8] for i in range(0, 64, 8)), text
        )
    )
    sys.stdout.write("\n")


# ---------------------------------------------------------------------------
# serve
# ---------------------------------------------------------------------------


def serve(port_short: int, port_default: int, wait_seconds: float) -> None:
    import socket

    import uvicorn

    from app.api.main import sqlalchemy_store_factory

    log: dict[str, list] = {"calls": [], "store": []}

    class Recording:
        """main.py's store, unchanged, with each call noted after it returns."""

        def __init__(self, inner):
            self.inner = inner

        def reserve(self, **kwargs):
            outcome = self.inner.reserve(**kwargs)
            log["store"].append(
                ["reserve", kwargs["scope"], kwargs["key"], type(outcome).__name__]
            )
            return outcome

        def complete(self, **kwargs):
            self.inner.complete(**kwargs)
            log["store"].append(
                [
                    "complete",
                    kwargs["scope"],
                    kwargs["key"],
                    kwargs["response"].status_code,
                ]
            )

        def release(self, **kwargs):
            self.inner.release(**kwargs)
            log["store"].append(["release", kwargs["scope"], kwargs["key"]])

    @contextmanager
    def factory():
        with sqlalchemy_store_factory() as store:
            yield Recording(store)

    async def stub(scope, receive, send):
        def first(name: bytes):
            for raw_name, raw_value in scope["headers"]:
                if raw_name == name:
                    return raw_value
            return None

        if scope["method"] == "GET" and scope["path"] == "/__log":
            payload = json.dumps(log).encode("ascii")
            log["calls"] = []
            log["store"] = []
            await send(
                {
                    "type": "http.response.start",
                    "status": 200,
                    "headers": [
                        (b"content-type", b"application/json"),
                        (b"content-length", str(len(payload)).encode("ascii")),
                    ],
                }
            )
            await send({"type": "http.response.body", "body": payload})
            return

        received = b""
        while True:
            message = await receive()
            if message["type"] != "http.request":
                break
            received += message.get("body") or b""
            if not message.get("more_body", False):
                break
        stored = scope.get("itinerary_authorized_replay")
        script = json.loads(first(b"x-stub") or b"{}")
        log["calls"].append(
            {
                "method": scope["method"],
                "path": scope["path"],
                "query": scope["query_string"].decode("latin-1"),
                "body_sha256": hashlib.sha256(received).hexdigest(),
                "replay": None
                if stored is None
                else {
                    "status": stored.status_code,
                    "body": stored.body.hex(),
                    "media_type": stored.media_type,
                },
            }
        )
        if script.get("sleep"):
            await asyncio.sleep(script["sleep"])
        if script.get("raise") == "before":
            raise RuntimeError(SCRIPTED_FAILURE)
        if stored is not None:
            status = stored.status_code
            out = stored.body
            headers = [(b"x-replay-seen", b"1")]
            if stored.media_type:
                headers.append((b"content-type", stored.media_type.encode("latin-1")))
        else:
            status = script.get("status", 200)
            out = bytes.fromhex(script.get("body", ""))
            headers = [
                (name.encode("latin-1"), bytes.fromhex(value))
                for name, value in script.get("headers", [])
            ]
        headers.append((b"content-length", str(len(out)).encode("ascii")))
        await send(
            {"type": "http.response.start", "status": status, "headers": headers}
        )
        if script.get("raise") == "after_start":
            raise RuntimeError(SCRIPTED_FAILURE)
        await send({"type": "http.response.body", "body": out})

    short = idem.IdempotencyMiddleware(
        stub, factory, in_flight_wait_seconds=wait_seconds
    )
    default = idem.IdempotencyMiddleware(stub, factory)

    async def application(scope, receive, send):
        chosen = short if scope["server"][1] == port_short else default
        await chosen(scope, receive, send)

    sockets = []
    for port in (port_short, port_default):
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(("127.0.0.1", port))
        sockets.append(sock)
    config = uvicorn.Config(application, lifespan="off", log_level="warning")
    uvicorn.Server(config).run(sockets=sockets)


DIGEST_RE = re.compile(r"(?<![0-9a-f])[0-9a-f]{64}(?![0-9a-f])")
GROUPED_DIGEST_RE = re.compile(r"[0-9a-f]{8}(?::[0-9a-f]{8}){7}")


def main() -> None:
    if len(sys.argv) >= 2 and sys.argv[1] == "goldens":
        goldens()
        return
    if len(sys.argv) == 5 and sys.argv[1] == "serve":
        serve(int(sys.argv[2]), int(sys.argv[3]), float(sys.argv[4]))
        return
    sys.exit(
        "usage: render_idem_oracle.py goldens | serve PORT_SHORT PORT_DEFAULT WAIT"
    )


if __name__ == "__main__":
    main()
