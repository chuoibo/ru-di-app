"""Reactions and comments under a post (L3, ADR-0022 §2.2) against the fake.

What this layer proves: who may read may react, twice is once, the counts on
the post are recounted from the rows; commenting obeys the wall owner's policy
(readers / friends / nobody) and the author is exempt; the author or the
commenter may delete, nobody else; every route with `{post_id}` answers 404
for a post the actor may not read -- never 403; and the public profile does
not carry the policy. What it does not prove: the unique key under a race, the
CASCADE, and the two GROUP BYs -- tests/postgres/test_post_social_postgres.py.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime

from app.api.repository import ContextRecord, PersonRecord

from .helpers import ADVANCER_ID, CONTEXT_ID, OTHER_ID, SENDER_ID, actor_headers
from .test_posts_audience import befriend, post_body

AUTHOR = ADVANCER_ID
GROUPMATE = SENDER_ID
STRANGER = OTHER_ID
FRIEND = uuid.UUID("5ee00000-eeee-4eee-8eee-0000e0000003")
T0 = datetime(2030, 8, 27, 12, tzinfo=UTC)


def _seed(repository):
    for pid, name in (
        (AUTHOR, "Tác Giả"),
        (FRIEND, "Bạn Thân"),
        (GROUPMATE, "Cùng Nhóm"),
        (STRANGER, "Người Lạ"),
    ):
        repository.people[pid] = PersonRecord(id=pid, display_name=name, created_at=T0)
    repository.contexts[CONTEXT_ID] = ContextRecord(
        id=CONTEXT_ID, display_name="Hội", created_by_id=AUTHOR, created_at=T0
    )
    repository.active_memberships |= {(CONTEXT_ID, AUTHOR), (CONTEXT_ID, GROUPMATE)}
    befriend(repository, AUTHOR, FRIEND)


def _write(client, audience, *, author=AUTHOR, context_id=None, body="Chào"):
    response = client.post(
        "/posts",
        json=post_body(audience, context_id=context_id, body=body),
        headers=actor_headers(author),
    )
    assert response.status_code == 201, response.text
    return response.json()


def _read(client, post_id, actor):
    return client.get(f"/posts/{post_id}", headers=actor_headers(actor))


def _react(client, post_id, actor, kind="heart"):
    return client.post(
        f"/posts/{post_id}/reactions", json={"kind": kind}, headers=actor_headers(actor)
    )


def _comment(client, post_id, actor, body="Hay quá"):
    return client.post(
        f"/posts/{post_id}/comments", json={"body": body}, headers=actor_headers(actor)
    )


def _policy(client, actor, policy):
    response = client.patch(
        "/people/me",
        json={"wall_comment_policy": policy},
        headers=actor_headers(actor),
    )
    assert response.status_code == 200, response.text
    return response.json()


# --- reactions ---------------------------------------------------------------


def test_a_reader_reacts_once_per_kind_and_the_post_recounts_from_the_rows(
    client, repository
):
    _seed(repository)
    post = _write(client, "friends")

    first = _react(client, post["id"], FRIEND)
    assert first.status_code == 200, first.text
    assert first.json() == {
        "post_id": post["id"],
        "reactions": [{"kind": "heart", "count": 1}],
        "my_reactions": ["heart"],
    }
    again = _react(client, post["id"], FRIEND)
    assert again.status_code == 200
    assert again.json()["reactions"] == [{"kind": "heart", "count": 1}], (
        "hai lần là một"
    )
    _react(client, post["id"], AUTHOR, "fire")
    _react(client, post["id"], AUTHOR, "heart")

    read = _read(client, post["id"], FRIEND).json()
    assert read["reactions"] == [
        {"kind": "fire", "count": 1},
        {"kind": "heart", "count": 2},
    ]
    assert read["my_reactions"] == ["heart"]
    assert read["comment_count"] == 0

    gone = client.delete(
        f"/posts/{post['id']}/reactions/heart", headers=actor_headers(FRIEND)
    )
    assert gone.status_code == 200, gone.text
    assert gone.json()["my_reactions"] == []
    assert gone.json()["reactions"] == [
        {"kind": "fire", "count": 1},
        {"kind": "heart", "count": 1},
    ]
    assert _read(client, post["id"], FRIEND).json()["my_reactions"] == []


def test_an_unknown_kind_is_refused_at_the_wire(client, repository):
    _seed(repository)
    post = _write(client, "public")
    assert _react(client, post["id"], FRIEND, "clap").status_code == 422
    assert (
        client.delete(
            f"/posts/{post['id']}/reactions/clap", headers=actor_headers(FRIEND)
        ).status_code
        == 422
    )


# --- comments ----------------------------------------------------------------


def test_a_reader_comments_and_the_list_runs_oldest_first(client, repository):
    _seed(repository)
    post = _write(client, "friends")

    first = _comment(client, post["id"], FRIEND, "Hay quá")
    assert first.status_code == 201, first.text
    assert first.json()["author_display_name"] == "Bạn Thân"
    assert first.json()["author_id"] == str(FRIEND)
    assert first.json()["post_id"] == post["id"]
    second = _comment(client, post["id"], AUTHOR, "Cảm ơn nha")
    assert second.status_code == 201

    listed = client.get(f"/posts/{post['id']}/comments", headers=actor_headers(FRIEND))
    assert listed.status_code == 200, listed.text
    assert [c["body"] for c in listed.json()["comments"]] == ["Hay quá", "Cảm ơn nha"]
    assert listed.json()["has_more"] is False and listed.json()["next_cursor"] is None

    page = client.get(
        f"/posts/{post['id']}/comments?limit=1", headers=actor_headers(FRIEND)
    ).json()
    assert [c["body"] for c in page["comments"]] == ["Hay quá"]
    assert page["has_more"] is True and page["next_cursor"]
    rest = client.get(
        f"/posts/{post['id']}/comments?limit=1&after={page['next_cursor']}",
        headers=actor_headers(FRIEND),
    ).json()
    assert [c["body"] for c in rest["comments"]] == ["Cảm ơn nha"]
    assert rest["has_more"] is False

    assert _read(client, post["id"], FRIEND).json()["comment_count"] == 2
    bad = client.get(
        f"/posts/{post['id']}/comments?after=not-a-cursor",
        headers=actor_headers(FRIEND),
    )
    assert bad.status_code == 422


def test_the_wall_owner_decides_who_may_comment_and_the_post_says_so(
    client, repository
):
    _seed(repository)
    post = _write(client, "group", context_id=CONTEXT_ID)
    befriend(repository, AUTHOR, GROUPMATE)  # groupmate is now ALSO a friend
    outsider_post = _write(client, "public")

    # readers (default): anyone who may read may comment.
    assert _read(client, post["id"], GROUPMATE).json()["can_comment"] is True
    assert _comment(client, post["id"], GROUPMATE).status_code == 201

    # friends: FRIEND may on the public post, a groupmate-who-is-not-a-friend may not.
    _policy(client, AUTHOR, "friends")
    repository.friend_edges.clear()
    befriend(repository, AUTHOR, FRIEND)
    assert _read(client, outsider_post["id"], FRIEND).json()["can_comment"] is True
    assert _read(client, post["id"], GROUPMATE).json()["can_comment"] is False
    closed = _comment(client, post["id"], GROUPMATE)
    assert closed.status_code == 403, closed.text
    assert closed.json()["code"] == "comments_closed"
    assert _comment(client, outsider_post["id"], FRIEND).status_code == 201

    # nobody: the wall is closed to everyone but its owner.
    _policy(client, AUTHOR, "nobody")
    assert _read(client, outsider_post["id"], FRIEND).json()["can_comment"] is False
    assert _comment(client, outsider_post["id"], FRIEND).status_code == 403
    assert _read(client, post["id"], AUTHOR).json()["can_comment"] is True
    assert _comment(client, post["id"], AUTHOR, "Tôi vẫn viết được").status_code == 201

    # The setting is the person's own: readable on their profile, absent on the public one.
    me = client.get("/people/me", headers=actor_headers(AUTHOR)).json()
    assert me["wall_comment_policy"] == "nobody"
    public = client.get(f"/people/{AUTHOR}", headers=actor_headers(FRIEND)).json()
    assert "wall_comment_policy" not in public
    assert (
        client.patch(
            "/people/me",
            json={"wall_comment_policy": "everyone"},
            headers=actor_headers(AUTHOR),
        ).status_code
        == 422
    )


def test_a_comment_is_deleted_by_its_author_or_the_posts_author_and_nobody_else(
    client, repository
):
    _seed(repository)
    post = _write(client, "public")
    mine = _comment(client, post["id"], FRIEND, "của bạn").json()
    theirs = _comment(client, post["id"], GROUPMATE, "của người cùng nhóm").json()

    forbidden = client.delete(
        f"/posts/{post['id']}/comments/{theirs['id']}", headers=actor_headers(FRIEND)
    )
    assert forbidden.status_code == 403, forbidden.text
    assert (
        client.delete(
            f"/posts/{post['id']}/comments/{mine['id']}", headers=actor_headers(FRIEND)
        ).status_code
        == 204
    )
    assert (
        client.delete(
            f"/posts/{post['id']}/comments/{theirs['id']}",
            headers=actor_headers(AUTHOR),
        ).status_code
        == 204
    ), "chủ bài quét được tường mình"
    assert _read(client, post["id"], FRIEND).json()["comment_count"] == 0

    other_post = _write(client, "public", body="bài khác")
    stray = _comment(client, other_post["id"], FRIEND, "ở bài khác").json()
    misfiled = client.delete(
        f"/posts/{post['id']}/comments/{stray['id']}", headers=actor_headers(FRIEND)
    )
    assert misfiled.status_code == 404, "bình luận của bài khác không «ở đây»"
    assert repository.get_post_comment(uuid.UUID(stray["id"])) is not None


# --- the 404 that is never a 403 --------------------------------------------


def test_every_post_route_answers_404_for_a_post_the_actor_may_not_read(
    client, repository
):
    _seed(repository)
    post = _write(client, "friends")
    comment = _comment(client, post["id"], FRIEND).json()
    nobody = uuid.uuid4()

    attempts = {
        "read": lambda pid: _read(client, pid, STRANGER),
        "react": lambda pid: _react(client, pid, STRANGER),
        "unreact": lambda pid: client.delete(
            f"/posts/{pid}/reactions/heart", headers=actor_headers(STRANGER)
        ),
        "comments": lambda pid: client.get(
            f"/posts/{pid}/comments", headers=actor_headers(STRANGER)
        ),
        "comment": lambda pid: _comment(client, pid, STRANGER),
        "delete": lambda pid: client.delete(
            f"/posts/{pid}/comments/{comment['id']}", headers=actor_headers(STRANGER)
        ),
    }
    for door, call in attempts.items():
        real = call(post["id"])
        fake = call(nobody)
        assert real.status_code == 404, (door, real.text)
        assert real.json() == fake.json(), (
            door,
            "bài thật và bài không có phải cùng một câu",
        )
    assert len(repository.post_reactions) == 0
    assert len(repository.post_comments) == 1
