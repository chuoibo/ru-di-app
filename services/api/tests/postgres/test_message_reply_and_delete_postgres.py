"""L1 on real PostgreSQL (ADR-0021): the four things a fake cannot prove.

- the composite foreign key refuses a reply that quotes a message of another
  group, whatever the service believed;
- the payload CHECK refuses a `deleted` row that kept its text, and the
  timestamp CHECK refuses a `deleted` row with no `deleted_at`;
- `soft_delete_message` flips the row and drops its reactions in one
  transaction, and the HTTP read shows exactly that;
- `contexts.theme` is CHECKed against the five slugs, and `PATCH` persists it.
"""

from __future__ import annotations

import uuid
from datetime import timedelta

import pytest
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.db.models import (
    Context,
    Membership,
    MembershipRole,
    MembershipState,
    Message,
    MessageKind,
    MessageReaction,
)

from .test_group_recap_postgres import _app, _call, _group, _headers, _person
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres


def _join(session: Session, context: Context, name: str):
    person = _person(session, name)
    session.add(
        Membership(
            id=uuid.uuid4(),
            context_id=context.id,
            person_id=person.id,
            state=MembershipState.ACTIVE,
            role=MembershipRole.MEMBER,
            joined_at=NOW,
        )
    )
    session.flush()
    return person


def _message(session: Session, context: Context, author, body: str, **extra) -> Message:
    row = Message(
        id=uuid.uuid4(),
        context_id=context.id,
        author_id=author.id,
        kind=MessageKind.TEXT,
        body=body,
        created_at=NOW,
        **extra,
    )
    session.add(row)
    session.flush()
    return row


def test_the_database_refuses_a_reply_that_quotes_another_groups_message(
    postgres_session: Session,
):
    group_a, owner_a = _group(postgres_session, "Nhóm A")
    group_b, owner_b = _group(postgres_session, "Nhóm B")
    secret = _message(postgres_session, group_b, owner_b, "chuyện riêng của B")

    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            _message(
                postgres_session,
                group_a,
                owner_a,
                "trả lời chéo",
                reply_to_id=secret.id,
            )
    assert "fk_messages_reply_to" in str(refused.value)


def test_the_database_refuses_a_deleted_row_that_kept_its_words(postgres_session):
    group, owner = _group(postgres_session)
    with pytest.raises(IntegrityError) as kept_body:
        with postgres_session.begin_nested():
            postgres_session.add(
                Message(
                    id=uuid.uuid4(),
                    context_id=group.id,
                    author_id=owner.id,
                    kind=MessageKind.DELETED,
                    body="vẫn còn chữ",
                    deleted_at=NOW,
                    created_at=NOW,
                )
            )
            postgres_session.flush()
    assert "ck_messages_payload_matches_kind" in str(kept_body.value)

    with pytest.raises(IntegrityError) as no_stamp:
        with postgres_session.begin_nested():
            postgres_session.add(
                Message(
                    id=uuid.uuid4(),
                    context_id=group.id,
                    author_id=owner.id,
                    kind=MessageKind.DELETED,
                    created_at=NOW,
                )
            )
            postgres_session.flush()
    assert "ck_messages_deleted_state_matches_timestamp" in str(no_stamp.value)


def test_the_database_refuses_a_sticker_body_that_is_not_a_slug(postgres_session):
    group, owner = _group(postgres_session)
    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            postgres_session.add(
                Message(
                    id=uuid.uuid4(),
                    context_id=group.id,
                    author_id=owner.id,
                    kind=MessageKind.STICKER,
                    body="Không Phải Slug",
                    created_at=NOW,
                )
            )
            postgres_session.flush()
    assert "ck_messages_payload_matches_kind" in str(refused.value)


def test_deleting_over_http_flips_the_row_and_drops_its_reactions(
    postgres_session, monkeypatch
):
    group, owner = _group(postgres_session)
    friend = _join(postgres_session, group, "Thu Trang")
    app = _app(postgres_session, monkeypatch)

    posted = _call(
        app,
        "POST",
        f"/contexts/{group.id}/messages",
        headers=_headers(owner, group),
        json={"kind": "text", "body": "gửi nhầm rồi"},
    )
    assert posted.status_code == 201, posted.text
    message_id = posted.json()["id"]
    reacted = _call(
        app,
        "POST",
        f"/contexts/{group.id}/messages/{message_id}/reactions",
        headers=_headers(friend, group),
        json={"kind": "heart"},
    )
    assert reacted.status_code == 201, reacted.text

    stranger_delete = _call(
        app,
        "DELETE",
        f"/contexts/{group.id}/messages/{message_id}",
        headers=_headers(friend, group),
    )
    assert stranger_delete.status_code == 403

    deleted = _call(
        app,
        "DELETE",
        f"/contexts/{group.id}/messages/{message_id}",
        headers=_headers(owner, group),
    )
    assert deleted.status_code == 204, deleted.text

    row = postgres_session.get(Message, uuid.UUID(message_id))
    assert row.kind == MessageKind.DELETED
    assert row.body is None and row.image_url is None and row.card is None
    assert row.deleted_at is not None
    reactions = postgres_session.scalars(
        select(MessageReaction).where(MessageReaction.message_id == row.id)
    ).all()
    assert reactions == []

    listed = _call(
        app, "GET", f"/contexts/{group.id}/messages", headers=_headers(friend, group)
    )
    assert listed.status_code == 200
    (wire,) = listed.json()["messages"]
    assert wire["kind"] == "deleted" and wire["reactions"] == []
    assert wire["body"] is None
    assert "gửi nhầm" not in listed.text


def test_a_sticker_and_a_reply_round_trip_over_http(postgres_session, monkeypatch):
    group, owner = _group(postgres_session)
    app = _app(postgres_session, monkeypatch)
    headers = _headers(owner, group)

    sticker = _call(
        app,
        "POST",
        f"/contexts/{group.id}/messages",
        headers=headers,
        json={"kind": "sticker", "body": "di-thoi"},
    )
    assert sticker.status_code == 201, sticker.text
    # `_app` pins `_now`; two rows born in the same instant would tie on
    # `created_at` and the feed's tiebreak (the id) is random. One second
    # later is what a real second message is.
    monkeypatch.setattr("app.api.service._now", lambda: NOW + timedelta(seconds=1))
    reply = _call(
        app,
        "POST",
        f"/contexts/{group.id}/messages",
        headers=headers,
        json={"kind": "text", "body": "đi liền", "reply_to_id": sticker.json()["id"]},
    )
    assert reply.status_code == 201, reply.text
    assert reply.json()["reply_to"]["preview"] == "[Sticker]"

    listed = _call(app, "GET", f"/contexts/{group.id}/messages", headers=headers)
    rows = listed.json()["messages"]
    assert [(m["kind"], m["body"]) for m in rows] == [
        ("text", "đi liền"),
        ("sticker", "di-thoi"),
    ], "tin mới nhất đứng đầu"
    assert rows[0]["reply_to"]["kind"] == "sticker"

    summary = _call(app, "GET", "/people/me/contexts", headers=headers)
    assert summary.status_code == 200, summary.text
    (row,) = summary.json()["contexts"]
    assert row["last_message"]["preview"] == "đi liền"
    assert row["theme"] == "mac-dinh"


def test_theme_is_checked_by_the_database_and_persisted_over_http(
    postgres_session, monkeypatch
):
    group, owner = _group(postgres_session)
    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            group.theme = "hong"
            postgres_session.flush()
    assert "ck_contexts_context_theme_known" in str(refused.value)
    postgres_session.refresh(group)

    app = _app(postgres_session, monkeypatch)
    patched = _call(
        app,
        "PATCH",
        f"/contexts/{group.id}",
        headers=_headers(owner, group),
        json={"theme": "bien-dem", "display_name": "Nhóm biển đêm"},
    )
    assert patched.status_code == 200, patched.text
    assert patched.json()["theme"] == "bien-dem"

    read = _call(app, "GET", f"/contexts/{group.id}", headers=_headers(owner, group))
    assert read.json()["theme"] == "bien-dem"
    assert read.json()["display_name"] == "Nhóm biển đêm"
    assert postgres_session.get(Context, group.id).theme == "bien-dem"
