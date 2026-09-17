"""The brain is behind a door, not a public route (ADR-0029 §2.7)."""

from __future__ import annotations

import anyio
import httpx
import pytest
from fastapi.routing import APIRoute

from app.api.cors import PreflightNoContentCORSMiddleware
from app.api.internal_token import (
    INTERNAL_TOKEN_ENV_VAR,
    INTERNAL_TOKEN_HEADER,
    InternalTokenMissing,
    TEST_TOKEN,
)
from app.api.main import create_app
from app.api.routes.brain import BrainDoor


@pytest.fixture
def brain_client(monkeypatch):
    async def run_sync_inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", run_sync_inline)
    app = create_app()

    class Client:
        def request(self, method, path, **kwargs):
            async def send():
                transport = httpx.ASGITransport(app=app)
                async with httpx.AsyncClient(
                    transport=transport, base_url="http://testserver"
                ) as http:
                    return await http.request(method, path, **kwargs)

            return anyio.run(send)

        def get(self, path, **kwargs):
            return self.request("GET", path, **kwargs)

        def post(self, path, **kwargs):
            return self.request("POST", path, **kwargs)

    return Client(), app


def test_create_app_refuses_an_empty_token(monkeypatch):
    monkeypatch.setenv(INTERNAL_TOKEN_ENV_VAR, "")
    with pytest.raises(InternalTokenMissing):
        create_app()


def test_cors_stays_outermost_with_the_brain_door():
    app = create_app()
    stack = [entry.cls for entry in app.user_middleware]
    assert stack[0] is PreflightNoContentCORSMiddleware
    assert BrainDoor in stack
    assert stack.index(BrainDoor) > stack.index(PreflightNoContentCORSMiddleware)


def test_brain_is_not_in_the_public_route_table(brain_client):
    _, app = brain_client
    public = []
    for route in app.router.routes:
        path = getattr(route, "path", "") or ""
        public.append(path)
        assert not str(path).startswith("/internal")
    assert any(isinstance(route, APIRoute) and route.path == "/healthz" for route in app.router.routes)
    assert "/internal/brain/v1/ready" not in public


def test_openapi_does_not_list_the_brain(brain_client):
    client, _app = brain_client
    response = client.get("/openapi.json")
    assert response.status_code == 200
    paths = response.json()["paths"]
    assert not any(path.startswith("/internal") for path in paths)


def test_ready_requires_the_token(brain_client):
    client, _app = brain_client
    missing = client.get("/internal/brain/v1/ready")
    assert missing.status_code == 401
    assert missing.json() == {"code": "internal_token_invalid"}

    wrong = client.get(
        "/internal/brain/v1/ready",
        headers={INTERNAL_TOKEN_HEADER: "other-brain-token"},
    )
    assert wrong.status_code == 401
    assert wrong.json() == {"code": "internal_token_invalid"}


def test_ready_answers_with_the_token(brain_client):
    client, _app = brain_client
    response = client.get(
        "/internal/brain/v1/ready",
        headers={INTERNAL_TOKEN_HEADER: TEST_TOKEN},
    )
    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_companion_plan_refuses_a_malformed_body(brain_client):
    client, _app = brain_client
    response = client.post(
        "/internal/brain/v1/companion-plan",
        headers={INTERNAL_TOKEN_HEADER: TEST_TOKEN},
        json={"conversation": "not-a-dict"},
    )
    assert response.status_code == 422
    assert response.json() == {"code": "brain_request_invalid"}


def test_idempotency_key_does_not_reserve_a_brain_call(brain_client):
    """A write key on /internal must not open an idempotency transaction."""

    client, _app = brain_client
    response = client.post(
        "/internal/brain/v1/companion-plan",
        headers={
            INTERNAL_TOKEN_HEADER: TEST_TOKEN,
            "Idempotency-Key": "brain-must-not-touch-the-store",
        },
        json={"conversation": {}},
    )
    assert response.status_code == 422
    assert response.json() == {"code": "brain_request_invalid"}
