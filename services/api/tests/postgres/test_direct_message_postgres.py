"""L2 on real PostgreSQL (ADR-0021 §2.5): what a dict-backed fake cannot prove.

- `uq_contexts_pair_key` refuses a second row for the same two people;
- the CHECKs refuse a pair without its key, a group with one, an unknown kind;
- `create_pair_context` writes the context and two ACTIVE memberships in one
  savepoint, and a lost race surfaces as `PAIR_EXISTS` with the session still
  usable, so the loser can read the winner's row;
- over HTTP: two friends share one pair whichever of them opens it, the list
  names it after the other person, a message travels in it, a roster door
  answers 409, and somebody outside the friendship gets the one 404.
"""

from __future__ import annotations

import types
import uuid

import pytest
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.errors import RepositoryConflict
from app.api.repository import SqlAlchemyApiRepository
from app.db.models import (
    Context,
    FriendRequest,
    FriendRequestState,
    Membership,
    MembershipState,
)
from app.domain.direct import pair_key

from .test_group_recap_postgres import _app, _call, _group, _headers, _person
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres


def _befriend(session: Session, a, b) -> FriendRequest:
    edge = FriendRequest(
        id=uuid.uuid4(),
        requester_id=a.id,
        addressee_id=b.id,
        state=FriendRequestState.ACCEPTED,
        decided_by_id=b.id,
        created_at=NOW,
        decided_at=NOW,
    )
    session.add(edge)
    session.flush()
    return edge


def _pair_row(session: Session, owner, **fields) -> Context:
    row = Context(
        id=uuid.uuid4(),
        display_name="",
        created_by_id=owner.id,
        **fields,
    )
    session.add(row)
    session.flush()
    return row


def test_the_database_keeps_one_pair_per_two_people(postgres_session: Session):
    _, a = _group(postgres_session)
    b = _person(postgres_session, "Bình")
    key = pair_key(str(a.id), str(b.id))
    _pair_row(postgres_session, a, kind="pair", pair_key=key)

    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            _pair_row(postgres_session, b, kind="pair", pair_key=key)
    assert "uq_contexts_pair_key" in str(refused.value)


@pytest.mark.parametrize(
    ("kind", "key"),
    [("pair", None), ("group", "a:b"), ("dm", None)],
    ids=["pair-without-key", "group-with-key", "unknown-kind"],
)
def test_the_checks_refuse_what_the_domain_refuses(
    postgres_session: Session, kind, key
):
    _, a = _group(postgres_session)

    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            _pair_row(postgres_session, a, kind=kind, pair_key=key)
    assert "ck_contexts_context_" in str(refused.value)


def test_create_pair_context_writes_two_active_memberships_in_one_savepoint(
    postgres_session: Session,
):
    _, a = _group(postgres_session)
    b = _person(postgres_session, "Bình")
    repo = SqlAlchemyApiRepository(postgres_session)
    key = pair_key(str(a.id), str(b.id))

    record = repo.create_pair_context(
        pair_key=key, member_ids=(a.id, b.id), created_by_id=a.id, now=NOW
    )

    assert record.kind == "pair" and record.pair_key == key
    assert record.display_name == "", "cặp không có tên riêng để lưu"
    rows = postgres_session.scalars(
        select(Membership).where(Membership.context_id == record.id)
    ).all()
    assert {row.person_id for row in rows} == {a.id, b.id}
    assert all(row.state is MembershipState.ACTIVE for row in rows)
    assert all(row.joined_at == NOW for row in rows), "không có bước «được mời»"

    # The other person presses at the same moment: the unique key decides, and
    # the session is still usable afterwards -- the loser reads the winner.
    with pytest.raises(RepositoryConflict) as lost:
        repo.create_pair_context(
            pair_key=key, member_ids=(b.id, a.id), created_by_id=b.id, now=NOW
        )
    assert lost.value.code == "PAIR_EXISTS"
    found = repo.get_pair_context(key)
    assert found is not None and found.id == record.id
    assert (
        postgres_session.scalar(
            select(Context.id).where(Context.pair_key == key, Context.id != record.id)
        )
        is None
    ), "hàng thua cuộc không được nằm lại trong DB"


def test_two_friends_share_one_pair_over_http_and_outsiders_get_the_one_404(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    group, a = _group(postgres_session)
    b = _person(postgres_session, "Bình")
    c = _person(postgres_session, "Cường")
    _befriend(postgres_session, a, b)
    app = _app(postgres_session, monkeypatch)

    opened = _call(app, "POST", f"/people/{b.id}/dm", headers=_headers(a, group))
    assert opened.status_code == 201, opened.text
    pair = opened.json()
    assert pair["kind"] == "pair"
    assert pair["display_name"] == "Bình"
    assert pair["counterpart"] == {"id": str(b.id), "display_name": "Bình"}
    assert pair["member_count"] == 2 and pair["my_state"] == "active"
    pair_ctx = types.SimpleNamespace(id=uuid.UUID(pair["id"]))

    again = _call(app, "POST", f"/people/{a.id}/dm", headers=_headers(b, group))
    assert again.status_code == 200, again.text
    assert again.json()["id"] == pair["id"]
    assert again.json()["display_name"] == "Minh Anh", "B thấy tên A"

    listed = _call(
        app, "GET", "/people/me/contexts", headers=_headers(b, group)
    ).json()["contexts"]
    row = next(r for r in listed if r["id"] == pair["id"])
    assert row["kind"] == "pair" and row["counterpart"]["id"] == str(a.id)
    assert row["display_name"] == "Minh Anh"

    # The whole of chat works in it: a message goes in and comes back out.
    sent = _call(
        app,
        "POST",
        f"/contexts/{pair['id']}/messages",
        headers=_headers(b, pair_ctx),
        json={"kind": "text", "body": "Chào riêng nhé"},
    )
    assert sent.status_code == 201, sent.text
    read = _call(
        app,
        "GET",
        f"/contexts/{pair['id']}/messages?limit=10",
        headers=_headers(a, pair_ctx),
    )
    assert read.status_code == 200, read.text
    assert [m["body"] for m in read.json()["messages"]] == ["Chào riêng nhé"]
    relisted = _call(
        app, "GET", "/people/me/contexts", headers=_headers(a, group)
    ).json()["contexts"]
    top = relisted[0]
    assert top["id"] == pair["id"], "cuộc trò chuyện có tin mới nhất đứng đầu"
    assert top["last_message"]["preview"] == "Chào riêng nhé"
    assert top["last_message"]["author_display_name"] == "Bình"
    assert top["unread_count"] == 1

    # Roster doors are closed in a pair; the theme is not.
    invited = _call(
        app,
        "POST",
        f"/contexts/{pair['id']}/members",
        headers=_headers(a, pair_ctx),
        json={"person_id": str(c.id)},
    )
    assert invited.status_code == 409 and invited.json()["code"] == "not_a_group"
    left = _call(
        app,
        "DELETE",
        f"/contexts/{pair['id']}/members/{a.id}",
        headers=_headers(a, pair_ctx),
    )
    assert left.status_code == 409 and left.json()["code"] == "not_a_group"
    renamed = _call(
        app,
        "PATCH",
        f"/contexts/{pair['id']}",
        headers=_headers(a, pair_ctx),
        json={"display_name": "Hai đứa"},
    )
    assert renamed.status_code == 409 and renamed.json()["code"] == "not_a_group"
    themed = _call(
        app,
        "PATCH",
        f"/contexts/{pair['id']}",
        headers=_headers(a, pair_ctx),
        json={"theme": "rung-thong"},
    )
    assert themed.status_code == 200, themed.text
    assert themed.json()["theme"] == "rung-thong"
    assert postgres_session.get(Context, pair_ctx.id).display_name == "", (
        "tên hiển thị là dẫn xuất; hàng vẫn rỗng"
    )
    members = postgres_session.scalars(
        select(Membership).where(Membership.context_id == pair_ctx.id)
    ).all()
    assert len(members) == 2 and all(m.state is MembershipState.ACTIVE for m in members)

    # Outside the friendship every answer is the same 404.
    refusals = [
        _call(app, "POST", f"/people/{a.id}/dm", headers=_headers(c, group)),
        _call(app, "POST", f"/people/{c.id}/dm", headers=_headers(a, group)),
        _call(app, "POST", f"/people/{uuid.uuid4()}/dm", headers=_headers(a, group)),
    ]
    assert [r.status_code for r in refusals] == [404, 404, 404]
    assert len({r.text for r in refusals}) == 1, "một câu cho mọi lý do"
    assert refusals[0].json()["code"] == "person_not_found"
    assert (
        postgres_session.scalar(
            select(Context.id).where(Context.kind == "pair", Context.id != pair_ctx.id)
        )
        is None
    ), "không có cặp nào được tạo cho người ngoài"


def test_a_pair_does_not_count_as_a_group_on_the_profile(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    group, a = _group(postgres_session)
    b = _person(postgres_session, "Bình")
    _befriend(postgres_session, a, b)
    app = _app(postgres_session, monkeypatch)

    before = _call(app, "GET", "/people/me", headers=_headers(a, group)).json()[
        "counts"
    ]
    opened = _call(app, "POST", f"/people/{b.id}/dm", headers=_headers(a, group))
    assert opened.status_code == 201, opened.text
    after = _call(app, "GET", "/people/me", headers=_headers(a, group)).json()["counts"]

    assert before["contexts"] == 1 and after["contexts"] == 1
    assert (
        _call(app, "GET", "/people/me", headers=_headers(b, group)).json()["counts"][
            "contexts"
        ]
        == 0
    )
