#!/usr/bin/env python3
"""Render what the API's CORS middleware answers, for the Go port to match.

ADR-0029: the Go front door applies CORS to the routes it serves, and the
bytes must be the ones Starlette writes today. Rather than restating
Starlette's rules in a second language from memory, this drives the real
`PreflightNoContentCORSMiddleware` (app/api/cors.py) over an ASGI inner app and
records status, headers and body for every case. The Go test replays the same
cases and compares.

Run inside the pinned API image so the Starlette version is the shipped one:

    docker run --rm -v "$PWD":/repo:ro -w /srv --entrypoint python \\
      <api image> /repo/scripts/render_cors_goldens.py \\
      > services/core/internal/httpapi/mw/cors/testdata/starlette_cors.json

Header values travel as latin-1 strings, which is how ASGI decodes them.
"""

from __future__ import annotations

import asyncio
import json
import sys

sys.path.insert(0, "/srv")

from app.api.cors import PreflightNoContentCORSMiddleware, cors_options  # noqa: E402

CONFIGS = {
    "loopback-default": None,
    "explicit-list": " https://rudi.example ,https://admin.rudi.example,, ",
    "wildcard": "*",
}

PREFLIGHT = [("access-control-request-method", "POST")]
CASES: list[tuple[str, str, str, list[tuple[str, str]]]] = [
    ("no-origin", "GET", "/plain", []),
    ("simple-loopback", "GET", "/plain", [("origin", "http://localhost:8081")]),
    ("simple-loopback-ip-no-port", "GET", "/plain", [("origin", "http://127.0.0.1")]),
    ("simple-https-localhost", "GET", "/plain", [("origin", "https://localhost")]),
    ("simple-empty-port", "GET", "/plain", [("origin", "http://localhost:")]),
    ("simple-uppercase-scheme", "GET", "/plain", [("origin", "HTTP://localhost")]),
    ("simple-trailing-space", "GET", "/plain", [("origin", "http://localhost ")]),
    ("simple-nbsp", "GET", "/plain", [("origin", "http://localhost\xa0")]),
    ("simple-listed", "GET", "/plain", [("origin", "https://rudi.example")]),
    ("simple-evil", "GET", "/plain", [("origin", "https://evil.example")]),
    ("simple-empty-origin", "GET", "/plain", [("origin", "")]),
    (
        "simple-two-origins",
        "GET",
        "/plain",
        [("origin", "https://evil.example"), ("origin", "http://localhost")],
    ),
    (
        "simple-existing-vary",
        "GET",
        "/with-vary",
        [("origin", "http://localhost:8081")],
    ),
    ("simple-two-vary", "GET", "/with-two-vary", [("origin", "http://localhost:8081")]),
    ("simple-stale-acao", "GET", "/with-acao", [("origin", "http://localhost:8081")]),
    (
        "simple-cookie",
        "GET",
        "/plain",
        [("origin", "https://evil.example"), ("cookie", "a=b")],
    ),
    ("simple-post", "POST", "/plain", [("origin", "http://localhost:8081")]),
    (
        "options-without-request-method",
        "OPTIONS",
        "/plain",
        [("origin", "http://localhost:8081")],
    ),
    (
        "preflight-ok",
        "OPTIONS",
        "/contexts",
        [("origin", "http://localhost:8081"), *PREFLIGHT],
    ),
    (
        "preflight-ok-headers",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            *PREFLIGHT,
            (
                "access-control-request-headers",
                "Authorization, Idempotency-Key,content-type",
            ),
        ],
    ),
    (
        "preflight-unknown-header",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            *PREFLIGHT,
            ("access-control-request-headers", "authorization,x-other"),
        ],
    ),
    (
        "preflight-empty-request-headers",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            *PREFLIGHT,
            ("access-control-request-headers", ""),
        ],
    ),
    (
        "preflight-trailing-comma",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            *PREFLIGHT,
            ("access-control-request-headers", "authorization,"),
        ],
    ),
    (
        "preflight-safelisted-only",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            *PREFLIGHT,
            ("access-control-request-headers", "Accept-Language"),
        ],
    ),
    (
        "preflight-lowercase-method",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            ("access-control-request-method", "post"),
        ],
    ),
    (
        "preflight-empty-method",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            ("access-control-request-method", ""),
        ],
    ),
    (
        "preflight-all-failures",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "https://evil.example"),
            ("access-control-request-method", "TRACE"),
            ("access-control-request-headers", "x-other"),
        ],
    ),
    (
        "preflight-listed-origin",
        "OPTIONS",
        "/contexts",
        [("origin", "https://admin.rudi.example"), *PREFLIGHT],
    ),
    (
        "preflight-cookie",
        "OPTIONS",
        "/contexts",
        [
            ("origin", "http://localhost:8081"),
            *PREFLIGHT,
            ("cookie", "a=b"),
        ],
    ),
]


async def inner(scope, receive, send):
    headers = [(b"content-type", b"application/json"), (b"content-length", b"2")]
    if scope["path"] == "/with-vary":
        headers.append((b"vary", b"Accept-Encoding"))
    if scope["path"] == "/with-two-vary":
        headers.append((b"vary", b"Accept-Encoding"))
        headers.append((b"vary", b"Cookie"))
    if scope["path"] == "/with-acao":
        headers.append((b"access-control-allow-origin", b"https://stale.example"))
    await send({"type": "http.response.start", "status": 200, "headers": headers})
    await send({"type": "http.response.body", "body": b"{}"})


async def drive(
    middleware, method: str, path: str, headers: list[tuple[str, str]]
) -> dict:
    scope = {
        "type": "http",
        "method": method,
        "path": path,
        "raw_path": path.encode(),
        "query_string": b"",
        "headers": [(k.encode("latin-1"), v.encode("latin-1")) for k, v in headers],
    }
    messages: list[dict] = []

    async def receive():
        return {"type": "http.request", "body": b"", "more_body": False}

    async def send(message):
        messages.append(message)

    await middleware(scope, receive, send)
    start = next(m for m in messages if m["type"] == "http.response.start")
    body = b"".join(
        m.get("body", b"") for m in messages if m["type"] == "http.response.body"
    )
    return {
        "status": start["status"],
        "headers": [
            [k.decode("latin-1"), v.decode("latin-1")]
            for k, v in start.get("headers", [])
        ],
        "body": body.decode("latin-1"),
    }


async def main() -> None:
    out = []
    for config_name, raw in CONFIGS.items():
        middleware = PreflightNoContentCORSMiddleware(inner, **cors_options(raw))
        for case_name, method, path, headers in CASES:
            result = await drive(middleware, method, path, headers)
            out.append(
                {
                    "config": config_name,
                    "raw_origins": raw,
                    "case": case_name,
                    "method": method,
                    "path": path,
                    "request_headers": [list(pair) for pair in headers],
                    **result,
                }
            )
    json.dump(out, sys.stdout, ensure_ascii=True, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    asyncio.run(main())
