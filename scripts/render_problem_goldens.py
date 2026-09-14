#!/usr/bin/env python3
"""Render how the API answers with an error, for the Go port to match.

ADR-0029: a Go-served route must fail with the bytes Python fails with. Three
shapes come from services/api/app/api/main.py and guest_privacy.py: an
`ApiProblem` as `{"code","detail"}`, a 422 whose FastAPI error list has `input`
removed, and Starlette's plain 500 (with the guest privacy headers under /g).

This drives the real `create_app()` over raw ASGI calls inside the pinned API
image, so the handlers, pydantic's messages and the middleware stack are the
shipped ones. No database is needed: the two 500 cases point
MOBILE_DATABASE_URL at a closed port, so the first query fails.

    docker run --rm -v "$PWD":/repo:ro -w /srv --network none --entrypoint python \\
      <api image> /repo/scripts/render_problem_goldens.py \\
      > services/core/internal/httpapi/problem/testdata/python_problems.json
"""

from __future__ import annotations

import asyncio
import json
import os
import sys

os.environ["MOBILE_AUTH_MODE"] = "dev"
os.environ["MOBILE_DATABASE_URL"] = (
    "postgresql+psycopg://nobody:nothing@127.0.0.1:9/nothing"
)
sys.path.insert(0, "/srv")

from app.api.main import create_app  # noqa: E402

ACTOR = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
CONTEXT = "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa"
TOKEN = "t" * 43
DEV = [("x-actor-id", ACTOR), ("x-actor-roles", "member")]
JSON = [("content-type", "application/json")]

CASES = [
    ("problem-missing-actor", "GET", f"/contexts/{CONTEXT}", [], b""),
    (
        "problem-invalid-actor",
        "GET",
        f"/contexts/{CONTEXT}",
        [("x-actor-id", "nope")],
        b"",
    ),
    ("validation-path-uuid", "GET", "/contexts/khong-phai-uuid", DEV, b""),
    ("validation-missing-field", "POST", "/contexts", DEV + JSON, b"{}"),
    (
        "validation-extra-field",
        "POST",
        "/contexts",
        DEV + JSON,
        b'{"display_name":"x","extra":1}',
    ),
    ("validation-wrong-type", "POST", "/contexts", DEV + JSON, b'{"display_name":5}'),
    (
        "validation-too-long",
        "POST",
        "/contexts",
        DEV + JSON,
        ('{"display_name":"' + "x" * 201 + '"}').encode(),
    ),
    ("validation-json-invalid", "POST", "/contexts", DEV + JSON, b'{"display_name": '),
    ("server-error-plain", "GET", f"/contexts/{CONTEXT}", DEV, b""),
    ("server-error-guest", "GET", f"/g/{TOKEN}", [], b""),
]


async def drive(app, name: str, method: str, path: str, headers, body: bytes) -> dict:
    scope = {
        "type": "http",
        "asgi": {"version": "3.0"},
        "http_version": "1.1",
        "method": method,
        "scheme": "http",
        "path": path,
        "raw_path": path.encode(),
        "root_path": "",
        "query_string": b"",
        "headers": [(b"host", b"parity.test")]
        + [(k.encode("latin-1"), v.encode("latin-1")) for k, v in headers],
        "client": ("127.0.0.1", 40000),
        "server": ("parity.test", 80),
        "state": {},
    }
    messages: list[dict] = []
    sent = False

    async def receive():
        nonlocal sent
        if sent:
            await asyncio.sleep(3600)
        sent = True
        return {"type": "http.request", "body": body, "more_body": False}

    async def send(message):
        messages.append(message)

    raised = None
    try:
        await app(scope, receive, send)
    except Exception as exc:  # ServerErrorMiddleware re-raises after answering
        raised = type(exc).__name__
    start = next(m for m in messages if m["type"] == "http.response.start")
    payload = b"".join(
        m.get("body", b"") for m in messages if m["type"] == "http.response.body"
    )
    return {
        "case": name,
        "method": method,
        "path": path,
        "status": start["status"],
        "headers": [
            [k.decode("latin-1"), v.decode("latin-1")]
            for k, v in start.get("headers", [])
        ],
        "body": payload.decode("utf-8"),
        "raised": raised,
    }


async def rate_limited(app) -> dict:
    """The per-IP OTP limit answers a Vietnamese detail before parsing the body."""
    result = None
    for _ in range(11):
        result = await drive(
            app, "problem-vietnamese-detail", "POST", "/auth/otp/request", JSON, b"{}"
        )
    return result


async def main() -> None:
    app = create_app()
    out = [await drive(app, *case) for case in CASES]
    out.append(await rate_limited(app))
    json.dump(out, sys.stdout, ensure_ascii=True, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    asyncio.run(main())
