"""Read a request body once, capped and timed, before anything else does.

Security fix (audit 2026-10-05, RS-07): every route used to read the whole body
-- the idempotency layer ahead of the route, the route ahead of authentication
-- with no byte cap and no deadline, so an anonymous caller could hold a
connection open with a slow body or park a large one in memory. This layer
answers the two refusals byte for byte as the Go front door does
(services/core/internal/httpapi/bodylimit) and sits where it sits: inside the
guest privacy headers, outside idempotency.
"""

from __future__ import annotations

import asyncio
import re

from app.api.idempotency import _send_problem

DEFAULT_LIMIT_BYTES = 1 << 20
#: The sanitizer's 10 MiB image plus multipart framing: an image one byte too
#: large still reaches its route and gets that route's own answer.
UPLOAD_LIMIT_BYTES = 10 * 1024 * 1024 + 64 * 1024
READ_TIMEOUT_SECONDS = 30.0

CODE_TOO_LARGE = "request_body_too_large"
CODE_TIMEOUT = "request_body_timeout"
DETAIL_TOO_LARGE = "The request body is larger than this endpoint accepts."
DETAIL_TIMEOUT = "The request body did not arrive in time."

_UPLOAD_ROUTES = (
    re.compile(r"^/contexts/[^/]+/photos$"),
    re.compile(r"^/people/[^/]+/avatar$"),
    re.compile(r"^/people/me/photos$"),
    re.compile(r"^/receipts/scan$"),
    re.compile(r"^/screenshots/scan$"),
)


def limit_for(method: str, path: str) -> int:
    if method == "POST" and any(p.match(path) for p in _UPLOAD_ROUTES):
        return UPLOAD_LIMIT_BYTES
    return DEFAULT_LIMIT_BYTES


def _declared_length(scope) -> int | None:
    for name, value in scope.get("headers") or ():
        if name == b"content-length":
            try:
                return int(value.decode("latin-1").strip())
            except ValueError:
                return None
    return None


class BodyLimitMiddleware:
    def __init__(
        self,
        app,
        *,
        read_timeout_seconds: float = READ_TIMEOUT_SECONDS,
    ) -> None:
        self.app = app
        self.read_timeout_seconds = read_timeout_seconds

    async def _refuse(self, receive, send, deadline: float) -> None:
        await _discard(receive, deadline)
        await _send_problem(send, 413, CODE_TOO_LARGE, DETAIL_TOO_LARGE)

    async def __call__(self, scope, receive, send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        limit = limit_for(scope["method"], scope["path"])
        loop = asyncio.get_running_loop()
        deadline = loop.time() + self.read_timeout_seconds
        declared = _declared_length(scope)
        if declared is not None and declared > limit:
            await self._refuse(receive, send, deadline)
            return
        chunks: list[bytes] = []
        size = 0
        more_body = True
        while more_body:
            remaining = deadline - loop.time()
            try:
                if remaining <= 0:
                    raise TimeoutError
                message = await asyncio.wait_for(receive(), remaining)
            except TimeoutError:
                await _send_problem(send, 408, CODE_TIMEOUT, DETAIL_TIMEOUT)
                return
            if message["type"] != "http.request":
                # The client went away mid-body: nobody is left to answer.
                return
            chunk = message.get("body") or b""
            size += len(chunk)
            if size > limit:
                if more_body := message.get("more_body", False):
                    await self._refuse(receive, send, deadline)
                else:
                    await _send_problem(send, 413, CODE_TOO_LARGE, DETAIL_TOO_LARGE)
                return
            if chunk:
                chunks.append(chunk)
            more_body = message.get("more_body", False)
        await self.app(scope, _replay_then_listen(b"".join(chunks), receive), send)


#: How much of a refused body is read and thrown away before answering.
DISCARD_LIMIT_BYTES = 64 * 1024 * 1024


async def _discard(receive, deadline: float) -> None:
    """Read and drop the rest of a refused body, within the same time budget
    and at most DISCARD_LIMIT_BYTES, so a proxy still sending it (the Go front
    door forwarding a Python route) reads the answer instead of a reset."""

    loop = asyncio.get_running_loop()
    dropped = 0
    while dropped <= DISCARD_LIMIT_BYTES:
        remaining = deadline - loop.time()
        if remaining <= 0:
            return
        try:
            message = await asyncio.wait_for(receive(), remaining)
        except TimeoutError:
            return
        if message["type"] != "http.request":
            return
        dropped += len(message.get("body") or b"")
        if not message.get("more_body", False):
            return


def _replay_then_listen(body: bytes, receive):
    """The buffered body once, then the real channel: a later receive waits
    for an actual disconnect instead of inventing one."""

    sent = False

    async def receive_buffered():
        nonlocal sent
        if sent:
            return await receive()
        sent = True
        return {"type": "http.request", "body": body, "more_body": False}

    return receive_buffered
