"""Reactions and comments under a post on real PostgreSQL (L3, ADR-0022 §2.2).

What only a database proves: `uq_post_reactions_one_per_kind` under the ORM,
the CASCADE from `posts`, the CHECK on `people.wall_comment_policy`, and that
the counts on a post come from two GROUP BYs -- «3 hearts × 2 comments» reads
3 and 2, not 6 and 6 -- through the real HTTP read.
"""

from __future__ import annotations

from datetime import timedelta

import pytest
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.db.models import Person, Post, PostComment, PostReaction

from .test_group_recap_postgres import _call
from .test_posts_postgres import _befriend, _headers, _http, _person, _write
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres


def app_post(app, path, *, headers, json=None):
    return _call(app, "POST", path, headers=headers, json=json)


def app_get(app, path, *, headers):
    return _call(app, "GET", path, headers=headers)


def app_patch(app, path, *, headers, json=None):
    return _call(app, "PATCH", path, headers=headers, json=json)


def test_one_person_reacts_once_per_kind(postgres_session: Session):
    author = _person(postgres_session, "Tác giả")
    reader = _person(postgres_session, "Bạn")
    post = _write(postgres_session, author, "public")
    postgres_session.add(
        PostReaction(post_id=post.id, person_id=reader.id, kind="heart", created_at=NOW)
    )
    postgres_session.flush()

    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            postgres_session.add(
                PostReaction(
                    post_id=post.id, person_id=reader.id, kind="heart", created_at=NOW
                )
            )
            postgres_session.flush()
    assert "uq_post_reactions_one_per_kind" in str(refused.value)

    # A different kind by the same person is a second row, not a conflict.
    postgres_session.add(
        PostReaction(post_id=post.id, person_id=reader.id, kind="fire", created_at=NOW)
    )
    postgres_session.flush()


def test_an_unknown_kind_and_an_unknown_policy_are_refused_by_the_database(
    postgres_session: Session,
):
    author = _person(postgres_session, "Tác giả")
    post = _write(postgres_session, author, "public")
    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            postgres_session.add(
                PostReaction(
                    post_id=post.id, person_id=author.id, kind="clap", created_at=NOW
                )
            )
            postgres_session.flush()
    assert "ck_post_reactions_post_reaction_kind_known" in str(refused.value)

    with pytest.raises(IntegrityError) as refused_policy:
        with postgres_session.begin_nested():
            author.wall_comment_policy = "everyone"
            postgres_session.flush()
    assert "ck_people_wall_comment_policy_known" in str(refused_policy.value)


def test_reactions_and_comments_die_with_their_post(postgres_session: Session):
    author = _person(postgres_session, "Tác giả")
    reader = _person(postgres_session, "Bạn")
    post = _write(postgres_session, author, "public")
    postgres_session.add_all(
        [
            PostReaction(
                post_id=post.id, person_id=reader.id, kind="heart", created_at=NOW
            ),
            PostComment(
                post_id=post.id, author_id=reader.id, body="hay", created_at=NOW
            ),
        ]
    )
    postgres_session.flush()

    postgres_session.delete(postgres_session.get(Post, post.id))
    postgres_session.flush()

    assert (
        postgres_session.scalars(
            select(PostReaction).where(PostReaction.post_id == post.id)
        ).all()
        == []
    )
    assert (
        postgres_session.scalars(
            select(PostComment).where(PostComment.post_id == post.id)
        ).all()
        == []
    )


def test_three_hearts_and_two_comments_read_three_and_two_over_http(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    author = _person(postgres_session, "Tác giả")
    friends = [_person(postgres_session, f"Bạn {i}") for i in range(3)]
    for friend in friends:
        _befriend(postgres_session, author, friend)
    post = _write(postgres_session, author, "friends")
    app = _http(postgres_session, monkeypatch)

    for friend in friends:
        reacted = app_post(
            app,
            f"/posts/{post.id}/reactions",
            headers=_headers(friend.id),
            json={"kind": "heart"},
        )
        assert reacted.status_code == 200, reacted.text
    for i in range(2):
        # `_http` pins `_now`; two comments born in one instant would tie on
        # `created_at`, and the list's tiebreak (the id) is random.
        monkeypatch.setattr(
            "app.api.service._now", lambda i=i: NOW + timedelta(seconds=i + 1)
        )
        commented = app_post(
            app,
            f"/posts/{post.id}/comments",
            headers=_headers(friends[i].id),
            json={"body": f"bình luận {i}"},
        )
        assert commented.status_code == 201, commented.text

    read = app_get(app, f"/posts/{post.id}", headers=_headers(friends[0].id))
    assert read.status_code == 200, read.text
    body = read.json()
    assert body["reactions"] == [{"kind": "heart", "count": 3}]
    assert body["comment_count"] == 2, "hai GROUP BY, không nhân bản"
    assert body["my_reactions"] == ["heart"]
    assert body["can_comment"] is True
    assert body["author_display_name"] == "Tác giả"

    listed = app_get(app, f"/posts/{post.id}/comments", headers=_headers(friends[2].id))
    assert [c["body"] for c in listed.json()["comments"]] == [
        "bình luận 0",
        "bình luận 1",
    ]

    # The wall owner closes the wall: the same reader now may not comment.
    closed = app_patch(
        app,
        "/people/me",
        headers=_headers(author.id),
        json={"wall_comment_policy": "nobody"},
    )
    assert closed.status_code == 200, closed.text
    assert postgres_session.get(Person, author.id).wall_comment_policy == "nobody"
    refused = app_post(
        app,
        f"/posts/{post.id}/comments",
        headers=_headers(friends[0].id),
        json={"body": "nữa"},
    )
    assert refused.status_code == 403 and refused.json()["code"] == "comments_closed"
    assert (
        app_get(app, f"/posts/{post.id}", headers=_headers(friends[0].id)).json()[
            "can_comment"
        ]
        is False
    )
