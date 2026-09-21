"""Cached chat replies must still pass current conversation authorization."""

from contextlib import contextmanager

import anyio
import pytest

from app.api.deps import get_repository
from app.api.main import create_app

from .conftest import ASGITestClient
from .helpers import actor_headers
from .test_chat_intents import CONTEXT_ID, MEMBER_ID
from .test_chat_sticker_reply_delete import ChatRepositoryWithEdits
from .test_idempotency import InMemoryIdempotencyStore


def test_chat_replay_rechecks_membership_and_never_writes_twice(monkeypatch):
    async def inline(function, *args, **kwargs):
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", inline)
    repository = ChatRepositoryWithEdits()
    store = InMemoryIdempotencyStore()

    @contextmanager
    def factory():
        yield store

    app = create_app(idempotency_store_factory=factory)
    app.dependency_overrides[get_repository] = lambda: repository
    client = ASGITestClient(app)
    path = f"/contexts/{CONTEXT_ID}/messages"
    headers = actor_headers(actor_id=MEMBER_ID) | {"Idempotency-Key": "chat-replay"}
    payload = {"kind": "text", "body": "Tin thử riêng tư (dữ liệu mẫu)"}
    first = client.post(path, json=payload, headers=headers)
    assert first.status_code == 201, first.text
    replay = client.post(path, json=payload, headers=headers)
    assert replay.status_code == 201 and replay.content == first.content
    assert replay.headers["Idempotency-Replayed"] == "true"
    monkeypatch.setattr(repository, "is_member", lambda *_: False)
    denied = client.post(path, json=payload, headers=headers)
    assert denied.status_code == 403, denied.text
    assert "Idempotency-Replayed" not in denied.headers
    assert payload["body"] not in denied.text
    assert len(repository.messages) == 1


def test_chat_replay_rejects_deleted_target_without_running_handler():
    from app.api.deps import Actor
    from app.api.errors import ApiProblem
    from app.api.schemas import MessageCreateRequest
    from app.api.service import ApiService

    repository = ChatRepositoryWithEdits()
    actor = Actor(id=MEMBER_ID, roles=frozenset({"member"}), context_ids=frozenset())
    service = ApiService(repository)
    message = service.post_context_message(
        CONTEXT_ID, MessageCreateRequest(kind="text", body="Tin thử"), actor
    )
    service.delete_own_message(CONTEXT_ID, message.id, actor)
    with pytest.raises(ApiProblem) as refused:
        service.authorize_chat_replay(CONTEXT_ID, actor, message_id=message.id)
    assert refused.value.code == "message_deleted"
    service.authorize_chat_replay(
        CONTEXT_ID, actor, message_id=message.id, deleting=True
    )
