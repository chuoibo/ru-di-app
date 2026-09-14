#!/usr/bin/env python3
"""Oracle for the Go port of the in-memory rate limiters (services/core/internal/limit).

ADR-0029 section 2.4: a Go-served route must refuse exactly when Python refuses,
with the bytes Python refuses with. Two classes hold every per-process window:

* `app.api.search_rate_limit.FixedWindowLimiter`: actor-keyed, window opened at
  the first admitted call, refusals leave `opened_at` alone, expired keys are
  swept (size doubling or one window of wall time since the last sweep).
* `app.api.routes.identity.FixedWindowLimit`: source-address keyed, windows are
  `int(clock() / window)` buckets of the absolute clock, the whole table is
  cleared once it holds more than `_MAX_TRACKED` entries.

Rather than restate them, this drives the real classes inside the pinned API
image on a fake clock (the same `clock=` seam `tests/api` uses) over generated
event sequences. Inputs are chosen adaptively from the limiter's own state so
that events land exactly on, just before and just after each window edge;
the recorded outputs are only public behaviour (admitted or refused) plus the
number of keys held, which is what the sweep and the clear change. Module
bounds (`_MIN_SWEEP_AT`, `_MAX_TRACKED`) are also run shrunk, by patching the
module global each class reads at call time, so eviction is exercised without
a ten thousand event sequence per case; one sequence per class keeps the real
bound.

Then it drives `create_app()` over raw ASGI (no database: the URL points at a
closed port, and every limiter is consulted before the first query) until each
shipped limiter answers 429, recording how many calls were admitted first and
the exact status, headers and body.

    docker run --rm -i --network none --entrypoint python <api image> - \\
      < scripts/render_limit_goldens.py \\
      > services/core/internal/limit/testdata/python_limits.json

Timestamps are small relative seconds with at most eight digits, so the file
passes the repository guard; float repr round-trips exactly through Go's
strconv, so no bit is lost.
"""

from __future__ import annotations

import asyncio
import json
import os
import random
import re
import sys
from uuid import UUID

os.environ["MOBILE_AUTH_MODE"] = "dev"
os.environ["MOBILE_DATABASE_URL"] = (
    "postgresql+psycopg://nobody:nothing@127.0.0.1:9/nothing"
)
sys.path.insert(0, "/srv")

import app.api.routes.identity as identity  # noqa: E402
import app.api.search_rate_limit as srl  # noqa: E402
from app.api.errors import ApiProblem  # noqa: E402
from app.api.main import create_app  # noqa: E402

SEED = 29
GUARD = re.compile(r"(?<![A-Za-z0-9])\d(?:[ .-]?\d){8,63}(?![A-Za-z0-9])")


def short(value: float) -> float:
    """Round to at most eight significant digits so the guard never fires."""

    whole = len(str(int(abs(value))))
    return round(value, max(0, min(3, 8 - whole)))


# ---------------------------------------------------------------------------
# FixedWindowLimiter (actor keyed)
# ---------------------------------------------------------------------------


def actor_sequence(rng: random.Random, name: str, cfg: dict, count: int) -> dict:
    now = [cfg["t0"]]
    saved = srl._MIN_SWEEP_AT
    srl._MIN_SWEEP_AT = cfg["min_sweep_at"]
    try:
        limiter = srl.FixedWindowLimiter(
            limit=cfg["limit"],
            window_seconds=cfg["window_seconds"],
            code="c",
            message="m",
            clock=lambda: now[0],
        )
        window = cfg["window_seconds"]
        events: list[list] = []
        allowed: list[str] = []
        tracked: list[int] = []
        t = cfg["t0"]
        plan = cfg.get("plan")
        for index in range(len(plan) if plan else count):
            key = plan[index] if plan else next_key(rng, cfg["keys"], events)
            t = next_actor_time(rng, limiter, key, t, window, cfg, index)
            now[0] = t
            try:
                limiter.check(UUID(int=key + 1))
                allowed.append("1")
            except ApiProblem as refused:
                assert (refused.status_code, refused.code, refused.detail) == (
                    429,
                    "c",
                    "m",
                )
                allowed.append("0")
            events.append([t, key])
            tracked.append(limiter.tracked())
    finally:
        srl._MIN_SWEEP_AT = saved
    return {
        "name": name,
        "limit": cfg["limit"],
        "window_seconds": cfg["window_seconds"],
        "min_sweep_at": cfg["min_sweep_at"],
        "t0": cfg["t0"],
        "events": events,
        "allowed": [int(outcome) for outcome in allowed],
        "tracked": tracked,
    }


def next_key(rng: random.Random, keys: int, events: list) -> int:
    if events and rng.random() < 0.45:
        return events[-1][1]
    return rng.randrange(keys)


def next_actor_time(rng, limiter, key, t, window, cfg, index) -> float:
    if cfg.get("plan"):
        # Distinct keys first (size sweeps at the real 1024 and 2048), then
        # two jumps aimed at exactly one window, and one window plus a
        # millisecond, after the last sweep: the time-triggered sweep.
        if index in cfg["jumps"]:
            return short(limiter._last_sweep + window + cfg["jumps"][index])
        return short(t + rng.choice([0.0, 0.0, 0.001, 0.01]))
    held = limiter._windows.get(UUID(int=key + 1))
    eps = rng.choice([-0.1, -0.001, 0.0, 0.0, 0.001, 0.1])
    roll = rng.random()
    if roll < 0.2:
        candidate = t
    elif roll < 0.5:
        candidate = t + rng.choice([0.001, 0.01, 0.125, 0.5, 1.0, 2.5])
    elif roll < 0.75 and held is not None:
        candidate = held[0] + window + eps
    elif roll < 0.85:
        candidate = limiter._last_sweep + window + eps
    elif roll < 0.95:
        candidate = t + window * rng.choice([1, 2, 5]) + eps
    else:
        candidate = t
    if cfg.get("backwards") and rng.random() < 0.05:
        candidate = t - rng.choice([0.001, 0.5, window])
    elif candidate < t and not cfg.get("backwards"):
        candidate = t
    return short(candidate)


# ---------------------------------------------------------------------------
# FixedWindowLimit (source-address keyed)
# ---------------------------------------------------------------------------


def address_sequence(rng: random.Random, name: str, cfg: dict, count: int) -> dict:
    now = [cfg["t0"]]
    saved = identity._MAX_TRACKED
    identity._MAX_TRACKED = cfg["max_tracked"]
    try:
        limiter = identity.FixedWindowLimit(
            limit=cfg["limit"],
            window_seconds=cfg["window_seconds"],
            clock=lambda: now[0],
        )
        window = cfg["window_seconds"]
        events: list[list] = []
        allowed: list[str] = []
        tracked: list[int] = []
        t = cfg["t0"]
        plan = cfg.get("plan")
        for index in range(len(plan) if plan else count):
            key = plan[index] if plan else next_key(rng, cfg["keys"], events)
            t = next_address_time(rng, t, window, cfg)
            now[0] = t
            allowed.append("1" if limiter.allow(f"c{key}") else "0")
            events.append([t, key])
            tracked.append(len(limiter._seen))
    finally:
        identity._MAX_TRACKED = saved
    return {
        "name": name,
        "limit": cfg["limit"],
        "window_seconds": cfg["window_seconds"],
        "max_tracked": cfg["max_tracked"],
        "events": events,
        "allowed": [int(outcome) for outcome in allowed],
        "tracked": tracked,
    }


def next_address_time(rng, t, window, cfg) -> float:
    if cfg.get("plan"):
        return short(t + rng.choice([0.0, 0.0, 0.0, 0.001]))
    bucket = int(t / window)
    eps = rng.choice([-0.1, -0.001, 0.0, 0.0, 0.001, 0.1])
    roll = rng.random()
    if roll < 0.25:
        candidate = t
    elif roll < 0.55:
        candidate = t + rng.choice([0.001, 0.01, 0.125, 0.5, 1.0])
    elif roll < 0.85:
        candidate = (bucket + 1) * window + eps
    else:
        candidate = t + window * rng.choice([1, 2, 3]) + eps
    if cfg.get("backwards") and rng.random() < 0.05:
        candidate = t - rng.choice([0.001, 0.5, window, 2 * t + 1])
    elif candidate < t and not cfg.get("backwards"):
        candidate = t
    return short(candidate)


def sequences() -> dict:
    rng = random.Random(SEED)
    actor = []
    for index in range(48):
        cfg = {
            "limit": rng.choice([0, 1, 1, 2, 2, 3, 3, 5, 12, 30]),
            "window_seconds": rng.choice([1, 2, 60, 60]),
            "min_sweep_at": rng.choice([1024, 1024, 2, 3, 4, 8, 16]),
            "t0": rng.choice([0.0, 3.5, 61.25, 1000.125, 1234567.5]),
            "keys": rng.choice([1, 2, 3, 8, 40]),
            "backwards": index % 6 == 5,
        }
        actor.append(actor_sequence(rng, f"actor-{index:02d}", cfg, 240))
    distinct = list(range(2200))
    actor.append(
        actor_sequence(
            rng,
            "actor-flood-real-bound",
            {
                "limit": 3,
                "window_seconds": 60,
                "min_sweep_at": 1024,
                "t0": 0.0,
                "plan": distinct + [rng.randrange(2200) for _ in range(400)],
                "jumps": {2200: 0.0, 2400: 0.001},
            },
            0,
        )
    )
    address = []
    for index in range(48):
        cfg = {
            "limit": rng.choice([0, 1, 1, 2, 3, 3, 10, 20, 30]),
            "window_seconds": rng.choice([0.5, 1.0, 7.5, 60.0, 60.0]),
            "max_tracked": rng.choice([10_000, 10_000, 2, 3, 5, 8]),
            "t0": rng.choice([0.0, 0.25, 59.999, 1000.125, 1234567.5]),
            "keys": rng.choice([1, 2, 3, 8, 40]),
            "backwards": index % 6 == 5,
        }
        address.append(address_sequence(rng, f"address-{index:02d}", cfg, 240))
    # Refusals first, then one more distinct caller than the real bound holds
    # (the clear), then callers the clear forgot.
    address.append(
        address_sequence(
            rng,
            "address-flood-real-bound",
            {
                "limit": 1,
                "window_seconds": 60.0,
                "max_tracked": 10_000,
                "t0": 0.0,
                "plan": [0, 0, 1, 1, 2, 2]
                + list(range(3, 10_004))
                + [rng.randrange(10_004) for _ in range(30)],
            },
            0,
        )
    )
    return {"actor": actor, "address": address}


# ---------------------------------------------------------------------------
# The shipped refusals, over create_app()
# ---------------------------------------------------------------------------

ACTOR = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
CONTEXT = "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa"
OTHER = "dddddddd-eeee-4fff-8aaa-bbbbbbbbbbbb"
DEV = [("x-actor-id", ACTOR), ("x-actor-roles", "member")]
JSON = [("content-type", "application/json")]
BOUNDARY = "limitboundary"
MULTIPART = [("content-type", f"multipart/form-data; boundary={BOUNDARY}")]
UPLOAD = (
    (
        f'--{BOUNDARY}\r\nContent-Disposition: form-data; name="image"; '
        'filename="a.png"\r\nContent-Type: image/png\r\n\r\n'
    ).encode()
    + b"\x89PNG\r\n\x1a\n"
    + f"\r\n--{BOUNDARY}--\r\n".encode()
)

# (case, state name, method, path, headers, body)
ROUTES = [
    ("otp-request", "otp_request_limit", "POST", "/auth/otp/request", JSON, b"{}"),
    ("otp-verify", "otp_verify_limit", "POST", "/auth/otp/verify", JSON, b"{}"),
    ("google-login", "google_login_limit", "POST", "/auth/google", JSON, b"{}"),
    ("person-id", "person_id_limit", "POST", "/identity/person-id", JSON, b"{}"),
    (
        "friend-lookup",
        "friend_lookup_limit",
        "POST",
        "/friends/lookup",
        DEV + JSON,
        b"{}",
    ),
    (
        "search",
        "search_limiter",
        "POST",
        "/places/search",
        DEV + JSON,
        b'{"query":"x"}',
    ),
    (
        "itinerary",
        "itinerary_limiter",
        "POST",
        f"/outings/{OTHER}/itinerary/preview",
        DEV + JSON,
        # Valid for pydantic, so the handler body (and its check) is reached.
        b'{"expected_revision":0,"stops":[],"days":[],"day":"2026-09-01"}',
    ),
    (
        "receipt-scan",
        "receipt_scan_limiter",
        "POST",
        "/receipts/scan",
        DEV + MULTIPART,
        UPLOAD,
    ),
    (
        "screenshot-scan",
        "screenshot_scan_limiter",
        "POST",
        "/screenshots/scan",
        DEV + MULTIPART,
        UPLOAD,
    ),
    (
        "chat-expense",
        "chat_expense_limiter",
        "POST",
        f"/contexts/{CONTEXT}/messages/{OTHER}/expense-draft",
        DEV,
        b"",
    ),
    (
        "companion-turn",
        "companion_turn_limiter",
        "POST",
        f"/contexts/{CONTEXT}/ai-turn",
        DEV,
        b"",
    ),
    (
        "suggestion",
        "suggestion_limiter",
        "GET",
        f"/contexts/{CONTEXT}/suggestion",
        DEV,
        b"",
    ),
    (
        "contextual-suggestion",
        "contextual_suggestion_limiter",
        "GET",
        f"/contexts/{CONTEXT}/contextual-suggestion",
        DEV,
        b"",
    ),
    (
        "reel",
        "reel_limiter",
        "GET",
        f"/contexts/{CONTEXT}/albums/{OTHER}/reel",
        DEV,
        b"",
    ),
    (
        "face-detection",
        "face_detection_limiter",
        "POST",
        f"/contexts/{CONTEXT}/photos/{OTHER}/face-boxes",
        DEV,
        b"",
    ),
]


async def drive(app, method: str, path: str, headers, body: bytes) -> dict:
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

    try:
        await app(scope, receive, send)
    except Exception:  # ServerErrorMiddleware re-raises after answering
        pass
    start = next(m for m in messages if m["type"] == "http.response.start")
    payload = b"".join(
        m.get("body", b"") for m in messages if m["type"] == "http.response.body"
    )
    return {
        "status": start["status"],
        "headers": [
            [k.decode("latin-1"), v.decode("latin-1")]
            for k, v in start.get("headers", [])
        ],
        "body": payload.decode("utf-8"),
    }


async def responses() -> list[dict]:
    out = []
    for case, state_name, method, path, headers, body in ROUTES:
        # A fresh app per route, so no window is shared by accident and the
        # admitted count is exactly the ceiling of this route.
        app = create_app()
        before: list[int] = []
        refused = None
        for _ in range(64):
            answer = await drive(app, method, path, headers, body)
            if answer["status"] == 429:
                refused = answer
                break
            before.append(answer["status"])
        if refused is None:
            raise SystemExit(f"{case}: no 429 after {len(before)} calls {before}")
        out.append(
            {
                "case": case,
                "state": state_name,
                "method": method,
                "path": path,
                "admitted": len(before),
                "admitted_statuses": sorted(set(before)),
                "status": refused["status"],
                "headers": refused["headers"],
                "body": refused["body"],
            }
        )
    return out


def main() -> None:
    out = sequences()
    out["responses"] = asyncio.run(responses())
    lines = ["{"]
    for group in ("actor", "address"):
        lines.append(f' "{group}": [')
        rows = [json.dumps(row, separators=(",", ":")) for row in out[group]]
        lines.append(",\n".join(rows))
        lines.append(" ],")
    lines.append(' "responses": [')
    rows = [
        json.dumps(row, ensure_ascii=True, separators=(",", ":"))
        for row in out["responses"]
    ]
    lines.append(",\n".join(rows))
    lines.append(" ]")
    lines.append("}")
    text = "\n".join(lines) + "\n"
    for number, line in enumerate(text.splitlines(), 1):
        match = GUARD.search(line)
        if match:
            raise SystemExit(f"repo guard would refuse line {number}: {match[0]}")
    sys.stdout.write(text)


if __name__ == "__main__":
    main()
