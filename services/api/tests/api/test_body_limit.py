"""The request body is read once, capped and timed (audit 2026-10-05, RS-07).

These answers are byte for byte the Go front door's
(services/core/internal/httpapi/bodylimit): the parity harness sends the
oversize cases to both stacks.
"""

from __future__ import annotations

import asyncio
import json

from app.api.body_limit import (
    DEFAULT_LIMIT_BYTES,
    UPLOAD_LIMIT_BYTES,
    BodyLimitMiddleware,
    limit_for,
)
from app.api.guest_privacy import GuestPrivacyHeadersMiddleware
from app.api.idempotency import IdempotencyMiddleware
from app.api.main import create_app

TOO_LARGE = (
    b'{"code": "request_body_too_large", '
    b'"detail": "The request body is larger than this endpoint accepts."}'
)


def _run(app, *, path="/expenses", method="POST", chunks=(b"{}",), headers=(), stall=False):
    seen = {}
    sent = []

    async def inner(scope, receive, send):
        body = b""
        while True:
            message = await receive()
            body += message.get("body", b"")
            if not message.get("more_body"):
                break
        seen["body"] = body
        await send({"type": "http.response.start", "status": 204, "headers": []})
        await send({"type": "http.response.body", "body": b""})

    messages = [
        {"type": "http.request", "body": c, "more_body": stall or i < len(chunks) - 1}
        for i, c in enumerate(chunks)
    ]

    async def receive():
        if messages:
            return messages.pop(0)
        if stall:
            await asyncio.sleep(3600)
        return {"type": "http.disconnect"}

    async def send(message):
        sent.append(message)

    scope = {"type": "http", "method": method, "path": path, "headers": list(headers)}
    asyncio.run(app(inner)(scope, receive, send))
    return seen, sent


def test_a_declared_oversize_body_is_refused_unread():
    seen, sent = _run(
        BodyLimitMiddleware,
        headers=[(b"content-length", str(DEFAULT_LIMIT_BYTES + 1).encode())],
    )
    assert "body" not in seen
    assert sent[0]["status"] == 413
    assert dict(sent[0]["headers"]) == {
        b"content-type": b"application/json",
        b"content-length": str(len(TOO_LARGE)).encode(),
    }
    assert sent[1]["body"] == TOO_LARGE


def test_an_undeclared_oversize_body_is_refused_at_the_cap():
    half = b"x" * (DEFAULT_LIMIT_BYTES // 2 + 1)
    seen, sent = _run(BodyLimitMiddleware, chunks=(half, half))
    assert "body" not in seen and sent[0]["status"] == 413


def test_a_body_at_the_cap_reaches_the_route_whole():
    body = b"x" * DEFAULT_LIMIT_BYTES
    seen, sent = _run(BodyLimitMiddleware, chunks=(body[:10], body[10:]))
    assert seen["body"] == body and sent[0]["status"] == 204


def test_upload_routes_take_an_image_one_byte_over_the_sanitizer_cap():
    assert limit_for("POST", "/people/me/photos") == UPLOAD_LIMIT_BYTES
    assert limit_for("POST", "/contexts/abc/photos") == UPLOAD_LIMIT_BYTES
    assert limit_for("POST", "/receipts/scan") == UPLOAD_LIMIT_BYTES
    assert UPLOAD_LIMIT_BYTES > 10 * 1024 * 1024 + 1
    assert limit_for("POST", "/expenses") == DEFAULT_LIMIT_BYTES
    assert limit_for("GET", "/people/me/photos") == DEFAULT_LIMIT_BYTES


def test_a_slow_body_runs_out_of_time():
    seen, sent = _run(
        lambda app: BodyLimitMiddleware(app, read_timeout_seconds=0.05),
        chunks=(b'{"half":',),
        stall=True,
    )
    # A stalled client: the only message was marked as having more to come.
    assert "body" not in seen
    assert sent[0]["status"] == 408
    assert json.loads(sent[1]["body"])["code"] == "request_body_timeout"


def test_the_layer_sits_inside_guest_headers_and_outside_idempotency():
    app = create_app()
    order = [m.cls for m in app.user_middleware]
    guest = order.index(GuestPrivacyHeadersMiddleware)
    limit = order.index(BodyLimitMiddleware)
    idem = order.index(IdempotencyMiddleware)
    # user_middleware lists the outermost first.
    assert guest < limit < idem
