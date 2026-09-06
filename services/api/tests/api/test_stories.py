"""24-hour stories (L4, ADR-0022 §2.3) against the fake.

What this layer proves: one's own photograph becomes a story one's friends
see and nobody else; `seen` is per reader and idempotent; only the author
deletes, and a stranger's every answer is the same 404 as «no such story»;
the personal-photo gate opens for a live story and closes at the deadline;
the rail's order is the domain's. What it does not prove: the SQL spelling
and the CHECKs -- see tests/postgres/test_stories_postgres.py.
"""

from __future__ import annotations

import uuid
from datetime import timedelta

import anyio
import pytest

from app.api.deps import get_photo_storage, get_repository
from app.api.main import create_app
from app.media.storage import PhotoStorage

from .conftest import ASGITestClient
from .helpers import actor_headers
from .test_person_photos_gate import (
    AUTHOR,
    FRIEND,
    GROUPMATE,
    STRANGER,
    T0,
    _get,
    _post,
    _seed,
    _upload,
)

DAY = timedelta(hours=24)


@pytest.fixture
def client(repository, tmp_path, monkeypatch):
    async def run_sync_inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", run_sync_inline)
    app = create_app(auth_mode="dev")
    app.dependency_overrides[get_repository] = lambda: repository
    app.dependency_overrides[get_photo_storage] = lambda: PhotoStorage(tmp_path)
    return ASGITestClient(app)


@pytest.fixture
def clock(monkeypatch):
    """A clock the test moves; the fake's rows carry whatever it said."""
    state = {"now": T0}
    monkeypatch.setattr("app.api.service._now", lambda: state["now"])

    def advance(delta):
        state["now"] = state["now"] + delta

    return advance


def _story(client, actor, image_url, caption="Story QA"):
    body = {"image_url": image_url}
    if caption is not None:
        body["caption"] = caption
    return client.post("/stories", json=body, headers=actor_headers(actor))


def _feed(client, actor):
    response = client.get("/stories", headers=actor_headers(actor))
    assert response.status_code == 200, response.text
    return response.json()["authors"]


def test_a_story_reaches_friends_and_nobody_else(client, repository, clock):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    written = _story(client, AUTHOR, photo["url"])
    assert written.status_code == 201, written.text
    story = written.json()
    assert story["author_id"] == str(AUTHOR)
    assert story["author_display_name"] == "Tác Giả"
    assert story["image_url"] == photo["url"]
    assert story["caption"] == "Story QA"
    assert story["audience"] == "friends"
    assert story["seen"] is False
    assert story["expires_at"] == (T0 + DAY).isoformat().replace("+00:00", "Z")

    theirs = _feed(client, FRIEND)
    assert [g["author"]["display_name"] for g in theirs] == ["Tác Giả"]
    assert [s["id"] for s in theirs[0]["stories"]] == [story["id"]]
    assert theirs[0]["all_seen"] is False
    assert _feed(client, GROUPMATE) == []
    assert _feed(client, STRANGER) == []
    mine = _feed(client, AUTHOR)
    assert [g["author"]["id"] for g in mine] == [str(AUTHOR)]


def test_only_ones_own_existing_photo_becomes_a_story(client, repository):
    _seed(repository)
    theirs = _upload(client, FRIEND)
    stolen = _story(client, AUTHOR, theirs["url"])
    assert stolen.status_code == 403, stolen.text
    missing = _story(client, AUTHOR, f"/people/{AUTHOR}/photos/{uuid.uuid4()}")
    assert missing.status_code == 404
    assert missing.json()["code"] == "photo_not_found"
    group_shape = _story(
        client, AUTHOR, f"/contexts/{uuid.uuid4()}/photos/{uuid.uuid4()}"
    )
    assert group_shape.status_code == 422, "story chỉ nhận ảnh cá nhân"
    assert not repository.stories


def test_the_caption_is_at_most_two_hundred_characters(client, repository):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    assert _story(client, AUTHOR, photo["url"], caption="x" * 200).status_code == 201
    too_long = _story(client, AUTHOR, photo["url"], caption="x" * 201)
    assert too_long.status_code == 422, too_long.text
    blank = _story(client, AUTHOR, photo["url"], caption="")
    assert blank.status_code == 201
    assert blank.json()["caption"] is None, "chú thích rỗng là không có chú thích"
    none = _story(client, AUTHOR, photo["url"], caption=None)
    assert none.status_code == 201 and none.json()["caption"] is None


def test_seen_is_per_reader_and_the_first_look_counts(client, repository, clock):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    story = _story(client, AUTHOR, photo["url"]).json()

    first = client.post(f"/stories/{story['id']}/seen", headers=actor_headers(FRIEND))
    assert first.status_code == 200, first.text
    assert first.json()["story_id"] == story["id"]
    clock(timedelta(minutes=5))
    again = client.post(f"/stories/{story['id']}/seen", headers=actor_headers(FRIEND))
    assert again.status_code == 200
    assert again.json()["seen_at"] == first.json()["seen_at"], "lần đầu là lần được ghi"

    theirs = _feed(client, FRIEND)
    assert theirs[0]["stories"][0]["seen"] is True and theirs[0]["all_seen"] is True
    mine = _feed(client, AUTHOR)
    assert mine[0]["stories"][0]["seen"] is False, (
        "«đã xem» là của người đọc, không lây"
    )

    refused = client.post(
        f"/stories/{story['id']}/seen", headers=actor_headers(STRANGER)
    )
    absent = client.post(
        f"/stories/{uuid.uuid4()}/seen", headers=actor_headers(STRANGER)
    )
    assert refused.status_code == 404 and absent.status_code == 404
    assert refused.json() == absent.json(), "cùng một câu với story không tồn tại"
    assert (uuid.UUID(story["id"]), STRANGER) not in repository.story_views


def test_only_the_author_takes_a_story_down(client, repository):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    story = _story(client, AUTHOR, photo["url"]).json()
    url = f"/stories/{story['id']}"

    by_friend = client.delete(url, headers=actor_headers(FRIEND))
    assert by_friend.status_code == 403, "bạn xem được nhưng không xoá được"
    by_stranger = client.delete(url, headers=actor_headers(STRANGER))
    absent = client.delete(f"/stories/{uuid.uuid4()}", headers=actor_headers(STRANGER))
    assert by_stranger.status_code == 404 and absent.status_code == 404
    assert by_stranger.json() == absent.json()
    assert uuid.UUID(story["id"]) in repository.stories

    by_author = client.delete(url, headers=actor_headers(AUTHOR))
    assert by_author.status_code == 204, by_author.text
    assert by_author.content == b""
    assert uuid.UUID(story["id"]) not in repository.stories
    assert _feed(client, FRIEND) == []
    assert client.delete(url, headers=actor_headers(AUTHOR)).status_code == 404


def test_a_live_story_opens_the_photo_and_the_deadline_closes_it(
    client, repository, clock
):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    assert _get(client, photo["url"], FRIEND).status_code == 404, (
        "chưa có gì trỏ tới ảnh"
    )
    story = _story(client, AUTHOR, photo["url"]).json()
    assert _get(client, photo["url"], FRIEND).status_code == 200
    assert _get(client, photo["url"], STRANGER).status_code == 404
    assert _get(client, photo["url"], GROUPMATE).status_code == 404

    clock(DAY - timedelta(seconds=1))
    assert len(_feed(client, FRIEND)) == 1, "một giây trước hạn vẫn còn"
    assert _get(client, photo["url"], FRIEND).status_code == 200
    clock(timedelta(seconds=1))
    assert _feed(client, FRIEND) == [], "đúng hạn là hết"
    assert _feed(client, AUTHOR) == [], "dải của chính mình cũng chỉ có story còn hạn"
    assert _get(client, photo["url"], FRIEND).status_code == 404
    assert _get(client, photo["url"], AUTHOR).status_code == 200, "chủ ảnh luôn"
    seen = client.post(f"/stories/{story['id']}/seen", headers=actor_headers(FRIEND))
    assert seen.status_code == 404, "story hết hạn không còn để xem"
    gone = client.delete(f"/stories/{story['id']}", headers=actor_headers(AUTHOR))
    assert gone.status_code == 204, "tác giả vẫn xoá được story đã hết hạn"


def test_a_post_showing_the_same_photo_keeps_it_open_after_the_story(
    client, repository, clock
):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    _story(client, AUTHOR, photo["url"])
    assert _post(client, "friends", image_url=photo["url"]).status_code == 201
    clock(DAY + timedelta(seconds=1))
    assert _get(client, photo["url"], FRIEND).status_code == 200, "bài vẫn trỏ tới ảnh"
    assert _get(client, photo["url"], STRANGER).status_code == 404


def test_the_rail_is_ordered_mine_then_unseen_then_seen(client, repository, clock):
    from .test_posts_audience import befriend

    _seed(repository)
    second = GROUPMATE
    befriend(repository, AUTHOR, second)
    befriend(repository, FRIEND, second)
    # FRIEND posts first, `second` later: newest-first would put `second`
    # ahead -- unless FRIEND's is the unseen one.
    friend_story = _story(client, FRIEND, _upload(client, FRIEND)["url"]).json()
    clock(timedelta(minutes=1))
    second_story = _story(client, second, _upload(client, second)["url"]).json()
    clock(timedelta(minutes=1))
    _story(client, AUTHOR, _upload(client, AUTHOR)["url"])

    assert [g["author"]["id"] for g in _feed(client, AUTHOR)] == [
        str(AUTHOR),
        str(second),
        str(FRIEND),
    ], "của mình trước, rồi chưa xem mới nhất trước"
    client.post(f"/stories/{second_story['id']}/seen", headers=actor_headers(AUTHOR))
    assert [g["author"]["id"] for g in _feed(client, AUTHOR)] == [
        str(AUTHOR),
        str(FRIEND),
        str(second),
    ], "đã xem hết thì lùi xuống sau người còn story chưa xem"
    client.post(f"/stories/{friend_story['id']}/seen", headers=actor_headers(AUTHOR))
    assert [g["author"]["id"] for g in _feed(client, AUTHOR)] == [
        str(AUTHOR),
        str(second),
        str(FRIEND),
    ], "cùng đã xem: mới nhất trước"
