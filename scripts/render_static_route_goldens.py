"""Goldens for services/core/internal/routes: routes that need no database.

Builds the real `create_app()` inside the pinned API image and answers each
route over raw ASGI, recording status, headers and body bytes:

    docker run --rm -i --network none --entrypoint python <api image> - \\
      < scripts/render_static_route_goldens.py \\
      > services/core/internal/routes/testdata/python_static_routes.json

The database URL points at a closed port: these routes never query, so a
request that did would fail loudly instead of reading data.
"""

from __future__ import annotations

import asyncio
import base64
import json
import os
import sys

ROUTES = [("GET", "/interests"), ("GET", "/areas")]


def configure() -> None:
    os.environ["MOBILE_DATABASE_URL"] = (
        "postgresql+psycopg://closed:closed@127.0.0.1:9/closed"
    )
    os.environ["MOBILE_AUTH_MODE"] = "dev"
    os.environ.setdefault(
        "MOBILE_PERSON_ID_KEY", base64.urlsafe_b64encode(os.urandom(33)).decode()
    )


async def answer(app, method: str, path: str) -> dict:
    messages: list[dict] = []
    scope = {
        "type": "http",
        "asgi": {"version": "3.0"},
        "http_version": "1.1",
        "method": method,
        "scheme": "http",
        "path": path,
        "raw_path": path.encode("ascii"),
        "query_string": b"",
        "root_path": "",
        "headers": [(b"host", b"parity.test")],
        "client": ("127.0.0.1", 50000),
        "server": ("parity.test", 80),
    }

    async def receive() -> dict:
        return {"type": "http.request", "body": b"", "more_body": False}

    async def send(message: dict) -> None:
        messages.append(message)

    await app(scope, receive, send)
    start = next(m for m in messages if m["type"] == "http.response.start")
    body = b"".join(
        m.get("body", b"") for m in messages if m["type"] == "http.response.body"
    )
    return {
        "method": method,
        "path": path,
        "status": start["status"],
        "headers": [
            [name.decode("latin-1"), value.decode("latin-1")]
            for name, value in start["headers"]
        ],
        "body": body.decode("utf-8"),
    }


def main() -> None:
    configure()
    from app.api.main import create_app

    app = create_app()
    responses = [asyncio.run(answer(app, method, path)) for method, path in ROUTES]
    json.dump({"responses": responses}, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
