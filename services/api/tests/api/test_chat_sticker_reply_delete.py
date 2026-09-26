"""Sticker, reply and delete on `POST/DELETE /contexts/{id}/messages` (L1, ADR-0021).

What this layer proves: a sticker is stored by id and an unknown id is a 422
with a sentence; a reply carries a server-built quote and quoting the wrong
thing is refused with the same 404 for absent and cross-group; deleting one's
own message leaves a `deleted` row with no payload and no reactions and the
list says so; and other people's messages and cards cannot be deleted. (The
case «the companion is never handed a deleted message» left with the automatic
companion, ADR-0036 §2.1: no server path reads the message table for AI.)

Built on the chat fake of `test_chat_intents.py`, widened with the three
repository methods this slice added.
"""

from __future__ import annotations

import uuid

import pytest

from app.api.errors import RepositoryConflict

from .helpers import actor_headers
from .test_chat_intents import (
    CONTEXT_ID,
    MEMBER_ID,
    ChatRepository,
    CountingCompanion,
    _client,
)

OTHER_MEMBER_ID = uuid.UUID("5ee00000-eeee-4eee-8eee-0000e0000009")
OTHER_CONTEXT_ID = uuid.UUID("6ff00000-ffff-4fff-8fff-0000f0000009")


class ChatRepositoryWithEdits(ChatRepository):
    """The intent fake plus what replies and deletion need."""

    def create_message(self, **fields):
        record = super().create_message(**fields)
        if fields.get("reply_to_id") is not None:
            from dataclasses import replace

            record = replace(record, reply_to_id=fields["reply_to_id"])
            self.messages[-1] = record
        return record

    def get_messages_by_ids(self, message_ids):
        wanted = set(message_ids)
        return {m.id: m for m in self.messages if m.id in wanted}

    def soft_delete_message(self, message_id, *, now):
        from dataclasses import replace

        for index, message in enumerate(self.messages):
            if message.id == message_id:
                updated = replace(
                    message,
                    kind="deleted",
                    body=None,
                    image_url=None,
                    card=None,
                    deleted_at=now,
                )
                self.messages[index] = updated
                for key in [k for k in self.reactions if k[0] == message_id]:
                    del self.reactions[key]
                return updated
        return None


@pytest.fixture
def repository():
    return ChatRepositoryWithEdits()


@pytest.fixture
def companion():
    return CountingCompanion()


@pytest.fixture
def client(repository, companion, monkeypatch):
    return _client(repository, companion, monkeypatch)


def _post(client, payload, *, actor=MEMBER_ID, context=CONTEXT_ID):
    return client.post(
        f"/contexts/{context}/messages",
        json=payload,
        headers=actor_headers(actor_id=actor),
    )


def _text(client, body, **extra):
    return _post(client, {"kind": "text", "body": body, **extra})


def _list(client, *, actor=MEMBER_ID):
    response = client.get(
        f"/contexts/{CONTEXT_ID}/messages", headers=actor_headers(actor_id=actor)
    )
    assert response.status_code == 200, response.text
    return response.json()["messages"]


# --- sticker -----------------------------------------------------------------


def test_a_sticker_is_stored_by_id_and_listed_with_no_other_payload(client, repository):
    response = _post(client, {"kind": "sticker", "body": "di-thoi"})
    assert response.status_code == 201, response.text
    body = response.json()
    assert body["kind"] == "sticker" and body["body"] == "di-thoi"
    assert body["image_url"] is None and body["card"] is None
    assert body["intent"] is None, "sticker không phải lệnh"
    listed = _list(client)
    assert [(m["kind"], m["body"]) for m in listed] == [("sticker", "di-thoi")]


def test_an_unknown_sticker_id_is_a_sentence_not_a_row(client, repository):
    response = _post(client, {"kind": "sticker", "body": "khong-co-trong-bo"})
    assert response.status_code == 422, response.text
    assert response.json()["code"] == "sticker_unknown"
    assert repository.messages == [], "không có hàng nào được ghi"


def test_a_sticker_carrying_a_picture_or_a_card_is_a_payload_mismatch(client):
    for extra in (
        {"image_url": f"/contexts/{CONTEXT_ID}/photos/{uuid.uuid4()}"},
        {"card": {}},
    ):
        response = _post(client, {"kind": "sticker", "body": "di-thoi", **extra})
        assert response.status_code == 422, response.text
        assert response.json()["code"] == "message_payload_invalid"


def test_the_client_cannot_post_a_deleted_message(client):
    response = _post(client, {"kind": "deleted", "body": None})
    assert response.status_code == 422, response.text


# --- reply -------------------------------------------------------------------


def test_a_reply_carries_a_server_built_quote_on_write_and_on_read(client):
    parent = _text(client, "Đẹp quá").json()
    response = _text(client, "Ừ, đi thôi", reply_to_id=parent["id"])
    assert response.status_code == 201, response.text
    quote = response.json()["reply_to"]
    assert quote == {
        "id": parent["id"],
        "kind": "text",
        "author_id": str(MEMBER_ID),
        "preview": "Đẹp quá",
    }
    listed = _list(client)
    reply = next(m for m in listed if m["body"] == "Ừ, đi thôi")
    assert reply["reply_to"]["preview"] == "Đẹp quá"
    assert next(m for m in listed if m["id"] == parent["id"])["reply_to"] is None


def test_quoting_an_absent_message_and_a_message_of_another_group_look_the_same(
    client, repository
):
    elsewhere = repository.create_message(
        context_id=OTHER_CONTEXT_ID,
        author_id=OTHER_MEMBER_ID,
        kind="text",
        body="bí mật của nhóm khác",
        image_url=None,
        card=None,
        now=repository.clock,
    )
    absent = _text(client, "trả lời ai?", reply_to_id=str(uuid.uuid4()))
    cross = _text(client, "trả lời ai?", reply_to_id=str(elsewhere.id))
    assert absent.status_code == cross.status_code == 404
    assert absent.json() == cross.json(), "cùng một câu cho hai trường hợp"
    assert "bí mật" not in cross.text


def test_quoting_a_deleted_message_is_refused_with_its_own_code(client):
    parent = _text(client, "sẽ xoá").json()
    deleted = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{parent['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert deleted.status_code == 204, deleted.text
    response = _text(client, "trả lời tin đã xoá", reply_to_id=parent["id"])
    assert response.status_code == 409, response.text
    assert response.json()["code"] == "reply_target_deleted"


def test_quoting_a_poll_card_is_refused(client, repository):
    posted = _text(client, "/vote Ăn gì? Bún | Phở")
    assert posted.status_code == 201, posted.text
    card = repository.messages[-1]
    assert card.kind == "ai_card"
    response = _text(client, "trả lời thẻ", reply_to_id=str(card.id))
    assert response.status_code == 422, response.text
    assert response.json()["code"] == "reply_target_not_quotable"


def test_a_card_cannot_itself_be_a_reply(client):
    parent = _text(client, "gốc").json()
    response = _post(
        client,
        {
            "kind": "ai_card",
            "card": {"kind": "text", "payload": {"text": "x"}},
            "reply_to_id": parent["id"],
        },
    )
    assert response.status_code == 422, response.text
    assert response.json()["code"] == "message_payload_invalid"


# --- delete ------------------------------------------------------------------


def test_deleting_ones_own_message_leaves_a_payloadless_row_without_reactions(
    client, repository
):
    posted = _text(client, "gửi nhầm").json()
    reacted = client.post(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}/reactions",
        json={"kind": "heart"},
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert reacted.status_code == 201, reacted.text

    response = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert response.status_code == 204, response.text
    assert response.content == b""

    (row,) = _list(client)
    assert row["kind"] == "deleted"
    assert row["body"] is None and row["image_url"] is None and row["card"] is None
    assert row["deleted_at"] is not None
    assert row["reactions"] == []
    assert row["author_id"] == str(MEMBER_ID), "tác giả vẫn được giữ"
    assert repository.reactions == {}


def test_a_reply_to_a_deleted_parent_quotes_the_deletion_not_the_words(client):
    parent = _text(client, "lời gốc").json()
    reply = _text(client, "trả lời", reply_to_id=parent["id"]).json()
    assert reply["reply_to"]["preview"] == "lời gốc"
    client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{parent['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    listed = _list(client)
    quoted = next(m for m in listed if m["id"] == reply["id"])["reply_to"]
    assert quoted["kind"] == "deleted"
    assert quoted["preview"] == "Tin nhắn đã bị xoá"
    assert "lời gốc" not in str(listed)


def test_deleting_somebody_elses_message_is_forbidden(client):
    posted = _text(client, "của tôi").json()
    response = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=OTHER_MEMBER_ID),
    )
    assert response.status_code == 403, response.text
    assert response.json()["code"] == "permission_denied"
    assert _list(client)[0]["body"] == "của tôi"


def test_deleting_twice_is_a_conflict_not_a_second_deletion(client):
    posted = _text(client, "một lần").json()
    first = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    second = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert (first.status_code, second.status_code) == (204, 409)
    assert second.json()["code"] == "message_already_deleted"


def test_a_poll_card_in_ones_own_name_cannot_be_deleted(client, repository):
    _text(client, "/vote Đi đâu? Biển | Núi")
    card = repository.messages[-1]
    response = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{card.id}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert response.status_code == 409, response.text
    assert response.json()["code"] == "message_kind_not_deletable"


def test_a_stranger_to_the_group_is_refused_before_the_row_is_read(client):
    posted = _text(client, "trong nhóm").json()
    response = client.delete(
        f"/contexts/{OTHER_CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert response.status_code == 403, response.text


def test_an_expense_draft_cannot_be_read_from_a_deleted_message(client):
    posted = _text(client, "bún bò 50k").json()
    client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    response = client.post(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}/expense-draft",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert response.status_code == 409, response.text
    assert response.json()["code"] == "message_deleted"


def test_a_conversation_list_row_reads_sticker_and_deletion_as_labels():
    """The preview a list row shows never carries the words of a deleted row."""
    from app.api.repository import _message_preview

    assert _message_preview("sticker", "di-thoi", None) == "[Sticker]"
    assert _message_preview("deleted", None, None) == "Tin nhắn đã bị xoá"
    assert _message_preview("text", "Đẹp quá", None) == "Đẹp quá"


# --- reactions and deletion do not cross (review PR #574, blocker B1) --------


def test_a_reaction_on_a_deleted_message_is_refused_and_the_thread_still_lists(
    client, repository
):
    posted = _text(client, "sẽ xoá").json()
    deleted = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert deleted.status_code == 204, deleted.text

    reacted = client.post(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}/reactions",
        json={"kind": "heart"},
        headers=actor_headers(actor_id=OTHER_MEMBER_ID),
    )
    assert reacted.status_code == 409, reacted.text
    assert reacted.json()["code"] == "message_deleted"
    assert repository.reactions == {}, "không có hàng phản ứng lạc"

    (row,) = _list(client, actor=OTHER_MEMBER_ID)
    assert row["kind"] == "deleted" and row["reactions"] == []


def test_a_tap_that_loses_the_race_to_a_deletion_is_the_same_409(client, repository):
    """The repository re-reads the kind under a lock and answers with a
    conflict; the service turns it into the same 409 as the plain case."""
    posted = _text(client, "đua với xoá").json()

    def lost_the_race(**kwargs):
        raise RepositoryConflict("MESSAGE_DELETED")

    repository.add_reaction = lost_the_race
    reacted = client.post(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}/reactions",
        json={"kind": "heart"},
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert reacted.status_code == 409, reacted.text
    assert reacted.json()["code"] == "message_deleted"


def test_a_stray_reaction_on_a_deleted_row_never_fails_the_feed(client, repository):
    """Should a reaction row ever survive next to a deleted message, the feed
    still lists (the wire shows none) -- one bad row must not 500 the group."""
    posted = _text(client, "hàng lạc").json()
    client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    repository.reactions[(uuid.UUID(posted["id"]), OTHER_MEMBER_ID, "heart")] = (
        repository.clock
    )

    (row,) = _list(client)
    assert row["kind"] == "deleted" and row["reactions"] == []


def test_the_second_of_two_simultaneous_deletions_is_a_conflict(client, repository):
    posted = _text(client, "xoá hai tay").json()

    def already_flipped(message_id, *, now):
        raise RepositoryConflict("MESSAGE_ALREADY_DELETED")

    repository.soft_delete_message = already_flipped
    response = client.delete(
        f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
        headers=actor_headers(actor_id=MEMBER_ID),
    )
    assert response.status_code == 409, response.text
    assert response.json()["code"] == "message_already_deleted"
