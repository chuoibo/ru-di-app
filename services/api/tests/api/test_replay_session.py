"""A stored answer goes back to a bearer only while its session is live.

Audit 2026-10-05, RS-02: replaying a generic write's key after the bearer was
revoked used to return the first answer's bytes. In prod the replay now answers
exactly what get_actor answers a dead session; the Go front door does the same
(idem.SessionGate). Chat and itinerary replays already run the whole endpoint
and have their own tests.
"""

from __future__ import annotations

from contextlib import contextmanager
from datetime import UTC, datetime

import anyio

from app.api.deps import get_repository
from app.api.idempotency import IDEMPOTENCY_HEADER
from app.api.main import create_app
from app.api.service import token_digest

from .conftest import ASGITestClient
from .helpers import ADVANCER_ID, expense_payload
from .test_idempotency import InMemoryIdempotencyStore
from .test_prod_session_auth import _bearer, _grant_session, _seed_person


def _prod_client(repository, store, monkeypatch):
    async def inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", inline)

    @contextmanager
    def factory():
        yield store

    app = create_app(auth_mode="prod", idempotency_store_factory=factory)
    app.dependency_overrides[get_repository] = lambda: repository
    return ASGITestClient(app)


def test_a_replay_to_a_revoked_session_gets_get_actors_answer(repository, monkeypatch):
    store = InMemoryIdempotencyStore()
    client = _prod_client(repository, store, monkeypatch)
    _seed_person(repository, ADVANCER_ID)
    token = _grant_session(repository, ADVANCER_ID)
    headers = _bearer(token) | {IDEMPOTENCY_HEADER: "replay-session"}
    payload = expense_payload()

    first = client.post("/expenses", json=payload, headers=headers)
    assert first.status_code == 201, first.text
    again = client.post("/expenses", json=payload, headers=headers)
    assert again.status_code == 201 and again.content == first.content
    assert again.headers["Idempotency-Replayed"] == "true"

    record = repository.get_account_session_by_digest(token_digest(token))
    repository.revoke_account_session(session_id=record.id, now=datetime.now(UTC))

    denied = client.post("/expenses", json=payload, headers=headers)
    assert denied.status_code == 401
    assert (
        denied.content
        == b'{"code":"authentication_required","detail":"Session is not valid"}'
    )
    assert "Idempotency-Replayed" not in denied.headers
    assert len(repository.expenses) == 1
