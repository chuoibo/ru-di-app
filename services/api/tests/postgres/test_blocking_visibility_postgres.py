"""Chặn trên PostgreSQL thật (L5, ADR-0023 §2.3).

Tầng fake nói về orchestration; ở đây `_readable_by` và `_story_readable_by`
thật quyết định. Ba câu được đo: bài `public`/`friends` và story ẩn HAI CHIỀU
và **không rời database**; bài `group` trong nhóm chung vẫn thấy; và lời mời
kết bạn từ người bị chặn nhận CÙNG MỘT BYTE với «đã có lời mời» — nếu hai câu
ấy khác nhau, người bị chặn đọc được điều mà `BLOCKED_IS_SILENT` giấu.
"""

from __future__ import annotations

import pytest
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.api.repository import SqlAlchemyApiRepository
from app.db.models import Post, Story

from .test_group_recap_postgres import _call
from .test_posts_postgres import _befriend, _context, _headers, _http, _join, _person
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres


def _post(app, author, audience, body, *, context=None):
    payload = {"body": body, "audience": audience}
    if context is not None:
        payload["context_id"] = str(context.id)
    headers = _headers(author.id, contexts=None if context is None else str(context.id))
    written = _call(app, "POST", "/posts", headers=headers, json=payload)
    assert written.status_code == 201, written.text
    return written.json()


def _feed(app, reader):
    answer = _call(app, "GET", "/posts?limit=50", headers=_headers(reader.id))
    assert answer.status_code == 200, answer.text
    return {row["body"] for row in answer.json()["posts"]}


def test_a_block_hides_the_wall_both_ways_and_the_rows_never_leave_the_database(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    a = _person(postgres_session, "A")
    b = _person(postgres_session, "B")
    _befriend(postgres_session, a, b)
    group = _context(postgres_session, a, "Nhóm chung")
    _join(postgres_session, group, a)
    _join(postgres_session, group, b)
    app = _http(postgres_session, monkeypatch)
    repo = SqlAlchemyApiRepository(postgres_session)

    _post(app, a, "public", "A công khai")
    _post(app, a, "friends", "A cho bạn bè")
    _post(app, a, "group", "A trong nhóm", context=group)
    _post(app, b, "public", "B công khai")

    assert _feed(app, b) == {
        "A công khai",
        "A cho bạn bè",
        "A trong nhóm",
        "B công khai",
    }

    repo.open_block_edge(blocker_id=a.id, addressee_id=b.id, now=NOW)
    postgres_session.flush()

    assert _feed(app, b) == {"A trong nhóm", "B công khai"}, (
        "công khai và bạn bè ẩn đi; bài của nhóm chung ở lại"
    )
    assert _feed(app, a) == {"A công khai", "A cho bạn bè", "A trong nhóm"}, (
        "chặn không phải mute: người chặn cũng thôi đọc tường người kia"
    )

    # «Không rời DB»: câu SQL không chọn hàng, chứ không phải lọc sau khi chọn.
    hidden = postgres_session.scalars(
        select(Post.body).where(repo._readable_by(b.id))
    ).all()
    assert "A công khai" not in hidden and "A cho bạn bè" not in hidden
    assert "A trong nhóm" in hidden


def test_a_block_hides_a_story_both_ways(postgres_session: Session):
    # No HTTP app here on purpose: the claim is about the SQL that decides who
    # a story leaves the database for, so the test asks that predicate directly.
    a = _person(postgres_session, "A")
    b = _person(postgres_session, "B")
    _befriend(postgres_session, a, b)
    repo = SqlAlchemyApiRepository(postgres_session)
    story = repo.create_story(
        author_id=a.id,
        image_url=f"/people/{a.id}/photos/{a.id}",
        caption="A kể",
        audience="friends",
        now=NOW,
        expires_at=NOW.replace(hour=NOW.hour + 1),
    )
    postgres_session.flush()

    seen = postgres_session.scalars(
        select(Story.id).where(repo._story_readable_by(b.id, NOW))
    ).all()
    assert story.id in seen

    repo.open_block_edge(blocker_id=b.id, addressee_id=a.id, now=NOW)
    postgres_session.flush()
    after = postgres_session.scalars(
        select(Story.id).where(repo._story_readable_by(b.id, NOW))
    ).all()
    assert story.id not in after, "người chặn cũng không xem story người bị chặn"
    still_mine = postgres_session.scalars(
        select(Story.id).where(repo._story_readable_by(a.id, NOW))
    ).all()
    assert story.id in still_mine, "tác giả vẫn thấy story của mình"


def test_a_friend_request_from_a_blocked_person_reads_like_a_duplicate(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    a = _person(postgres_session, "A")
    b = _person(postgres_session, "B")
    c = _person(postgres_session, "C")
    d = _person(postgres_session, "D")
    app = _http(postgres_session, monkeypatch)
    repo = SqlAlchemyApiRepository(postgres_session)

    repo.open_block_edge(blocker_id=a.id, addressee_id=b.id, now=NOW)
    # A pair with a request already open: the control this comparison needs.
    opened = _call(
        app,
        "POST",
        "/friends/requests",
        headers=_headers(c.id),
        json={"addressee_id": str(d.id)},
    )
    assert opened.status_code == 201, opened.text
    postgres_session.flush()

    blocked = _call(
        app,
        "POST",
        "/friends/requests",
        headers=_headers(b.id),
        json={"addressee_id": str(a.id)},
    )
    duplicate = _call(
        app,
        "POST",
        "/friends/requests",
        headers=_headers(c.id),
        json={"addressee_id": str(d.id)},
    )
    assert blocked.status_code == duplicate.status_code == 409
    assert blocked.json() == duplicate.json(), (
        "bị chặn và đã gửi rồi phải là CÙNG MỘT câu, hoặc bức tường tự khai ra"
    )


def test_lifting_a_block_gives_the_wall_back_but_not_the_friendship(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    a = _person(postgres_session, "A")
    b = _person(postgres_session, "B")
    _befriend(postgres_session, a, b)
    app = _http(postgres_session, monkeypatch)
    repo = SqlAlchemyApiRepository(postgres_session)
    _post(app, a, "public", "A công khai")

    repo.open_block_edge(blocker_id=a.id, addressee_id=b.id, now=NOW)
    postgres_session.flush()
    assert _feed(app, b) == set()

    repo.lift_block_edge(blocker_id=a.id, addressee_id=b.id, now=NOW)
    postgres_session.flush()
    assert _feed(app, b) == {"A công khai"}, "công khai đọc lại được"
    assert repo.are_friends(a.id, b.id) is False, "gỡ chặn không nối lại tình bạn"
