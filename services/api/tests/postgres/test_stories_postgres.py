"""24-hour stories on real PostgreSQL (L4, ADR-0022 §2.3).

The fake tier re-implements «may this reader see this story» by hand; here
the real `_story_readable_by` decides, through the real HTTP routes, with the
service clock moved by hand: a friend sees the story and reads its photo, a
stranger sees nothing and the row never leaves the database, the deadline is
strict and applies to the author's own rail, `story_views` is one row per
look and goes with the story, the three CHECKs refuse what the domain refuses,
the domain and the SQL agree on every combination, and the sweep removes only
what nothing points at any more.
"""

from __future__ import annotations

import uuid
from datetime import timedelta

import pytest
from sqlalchemy import insert, select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.repository import SqlAlchemyApiRepository
from app.db.models import Story, StoryView, UploadedImage
from app.db.story_purge import purge_expired_stories, remove_purged_files
from app.domain import story_visibility
from app.media.storage import PhotoStorage

from .test_group_recap_postgres import _call
from .test_person_photo_gate_postgres import _app_with_storage, _png, _upload
from .test_posts_postgres import _befriend, _context, _headers, _join, _person
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres

DAY = timedelta(hours=24)


def _clock(monkeypatch, when):
    monkeypatch.setattr("app.api.service._now", lambda: when)


def _feed(app, person):
    response = _call(app, "GET", "/stories", headers=_headers(person.id))
    assert response.status_code == 200, response.text
    return response.json()["authors"]


def _own_photo(app, person) -> str:
    uploaded = _upload(app, "/people/me/photos", _headers(person.id), _png())
    assert uploaded.status_code == 201, uploaded.text
    return uploaded.json()["url"]


def _story(app, person, url, caption="Story QA"):
    return _call(
        app,
        "POST",
        "/stories",
        headers=_headers(person.id),
        json={"image_url": url, "caption": caption},
    )


def test_a_story_reaches_a_friend_and_never_leaves_the_database_for_a_stranger(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path
):
    author = _person(postgres_session, "Chủ story")
    friend = _person(postgres_session, "Bạn")
    stranger = _person(postgres_session, "Người lạ")
    _befriend(postgres_session, author, friend)
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)
    repo = SqlAlchemyApiRepository(postgres_session)

    url = _own_photo(app, author)
    written = _story(app, author, url)
    assert written.status_code == 201, written.text
    story_id = uuid.UUID(written.json()["id"])
    row = postgres_session.get(Story, story_id)
    assert row.expires_at == NOW + DAY and row.audience == "friends"

    theirs = _feed(app, friend)
    assert [g["author"]["display_name"] for g in theirs] == ["Chủ story"]
    assert theirs[0]["stories"][0]["seen"] is False
    assert _feed(app, stranger) == []
    # «Không rời DB»: the SQL spelling keeps the row out of the result set,
    # it does not merely hide it afterwards.
    assert (
        postgres_session.scalar(
            select(Story.id).where(repo._story_readable_by(stranger.id, NOW))
        )
        is None
    )
    assert (
        postgres_session.scalar(
            select(Story.id).where(repo._story_readable_by(friend.id, NOW))
        )
        == story_id
    )

    # The personal-photo gate opens through the story, for the friend only.
    assert _call(app, "GET", url, headers=_headers(friend.id)).status_code == 200
    assert _call(app, "GET", url, headers=_headers(stranger.id)).status_code == 404


def test_seen_is_one_row_per_look_and_goes_with_the_story(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path
):
    author = _person(postgres_session, "Chủ story")
    friend = _person(postgres_session, "Bạn")
    _befriend(postgres_session, author, friend)
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)
    repo = SqlAlchemyApiRepository(postgres_session)
    story_id = uuid.UUID(_story(app, author, _own_photo(app, author)).json()["id"])

    first = _call(app, "POST", f"/stories/{story_id}/seen", headers=_headers(friend.id))
    assert first.status_code == 200, first.text
    _clock(monkeypatch, NOW + timedelta(minutes=3))
    again = _call(app, "POST", f"/stories/{story_id}/seen", headers=_headers(friend.id))
    assert again.status_code == 200
    assert again.json()["seen_at"] == first.json()["seen_at"]
    views = list(
        postgres_session.scalars(
            select(StoryView).where(StoryView.story_id == story_id)
        )
    )
    assert len(views) == 1 and views[0].viewer_id == friend.id
    assert repo.mark_story_seen(story_id, friend.id, now=NOW + DAY) == views[0].seen_at

    # A second row for the same look, written past the repository: the key
    # refuses it. Core insert rather than `session.add`, because the ORM
    # already holds the persistent row and would only warn about the clash.
    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            postgres_session.execute(
                insert(StoryView).values(
                    story_id=story_id, viewer_id=friend.id, seen_at=NOW
                )
            )
    assert "pk_story_views" in str(refused.value)

    assert _feed(app, friend)[0]["all_seen"] is True
    assert _feed(app, author)[0]["stories"][0]["seen"] is False

    gone = _call(app, "DELETE", f"/stories/{story_id}", headers=_headers(author.id))
    assert gone.status_code == 204, gone.text
    assert postgres_session.get(Story, story_id) is None
    assert (
        postgres_session.scalar(select(StoryView).where(StoryView.story_id == story_id))
        is None
    ), "CASCADE: dấu đã xem đi cùng story"


def test_the_deadline_is_strict_and_closes_the_photo(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path
):
    author = _person(postgres_session, "Chủ story")
    friend = _person(postgres_session, "Bạn")
    _befriend(postgres_session, author, friend)
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)
    url = _own_photo(app, author)
    story_id = uuid.UUID(_story(app, author, url).json()["id"])

    _clock(monkeypatch, NOW + DAY - timedelta(seconds=1))
    assert len(_feed(app, friend)) == 1
    assert _call(app, "GET", url, headers=_headers(friend.id)).status_code == 200

    _clock(monkeypatch, NOW + DAY)
    assert _feed(app, friend) == [], "đúng hạn là hết"
    assert _feed(app, author) == [], "dải của chính mình cũng chỉ còn story còn hạn"
    assert _call(app, "GET", url, headers=_headers(friend.id)).status_code == 404
    assert _call(app, "GET", url, headers=_headers(author.id)).status_code == 200
    seen = _call(app, "POST", f"/stories/{story_id}/seen", headers=_headers(friend.id))
    assert seen.status_code == 404
    gone = _call(app, "DELETE", f"/stories/{story_id}", headers=_headers(author.id))
    assert gone.status_code == 204, "tác giả vẫn xoá được story đã hết hạn"


@pytest.mark.parametrize("offset", [timedelta(0), DAY, DAY + timedelta(seconds=1)])
def test_the_domain_and_the_sql_agree_on_every_reader(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path, offset
):
    """Two spellings of one rule (7.8): for every reader the SQL predicate
    must select the row exactly when `can_view` says yes."""
    author = _person(postgres_session, "Chủ story")
    friend = _person(postgres_session, "Bạn")
    pending = _person(postgres_session, "Mới xin kết bạn")
    stranger = _person(postgres_session, "Người lạ")
    _befriend(postgres_session, author, friend)
    from app.db.models import FriendRequestState

    _befriend(postgres_session, pending, author, state=FriendRequestState.PENDING)
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)
    repo = SqlAlchemyApiRepository(postgres_session)
    story_id = uuid.UUID(_story(app, author, _own_photo(app, author)).json()["id"])
    row = postgres_session.get(Story, story_id)
    story = {
        "author_id": str(row.author_id),
        "audience": row.audience,
        "expires_at": row.expires_at,
    }
    now = NOW + offset

    for reader, is_friend in (
        (author, False),
        (friend, True),
        (pending, False),
        (stranger, False),
    ):
        by_sql = (
            postgres_session.scalar(
                select(Story.id).where(
                    Story.id == story_id, repo._story_readable_by(reader.id, now)
                )
            )
            is not None
        )
        by_domain = story_visibility.can_view(
            story,
            reader_id=str(reader.id),
            is_friend=is_friend,
            is_blocked=False,
            now=now,
        )
        assert by_sql is by_domain, (reader.display_name, offset, by_sql, by_domain)


@pytest.mark.parametrize(
    ("audience", "expires_offset", "caption", "constraint"),
    [
        ("public", DAY, "x", "ck_stories_story_audience_known"),
        ("friends", timedelta(0), "x", "ck_stories_story_expires_after_created"),
        ("friends", DAY, "x" * 201, "ck_stories_story_caption_length"),
    ],
    ids=["audience", "deadline-not-after-created", "caption-too-long"],
)
def test_the_checks_refuse_what_the_domain_refuses(
    postgres_session: Session, audience, expires_offset, caption, constraint
):
    author = _person(postgres_session, "Ai đó")
    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            postgres_session.add(
                Story(
                    id=uuid.uuid4(),
                    author_id=author.id,
                    image_url=f"/people/{author.id}/photos/{uuid.uuid4()}",
                    caption=caption,
                    audience=audience,
                    created_at=NOW,
                    expires_at=NOW + expires_offset,
                )
            )
            postgres_session.flush()
    assert constraint in str(refused.value)


def test_the_sweep_removes_only_what_nothing_points_at(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path
):
    """Rows the sweep may take: stories expired past the grace period, and
    personal photographs past the grace period that no post or story names.
    Rows it must leave (review #577 S1/S2): a photo a post still shows, a
    story inside the grace period, a photo uploaded just now with no story
    yet, an avatar, and a group photograph. Files go only through
    `remove_purged_files`, after the rows (S3)."""
    author = _person(postgres_session, "Chủ story")
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)
    storage = PhotoStorage(tmp_path)
    repo = SqlAlchemyApiRepository(postgres_session)

    stale_url = _own_photo(app, author)
    shared_url = _own_photo(app, author)
    orphan_url = _own_photo(app, author)
    fresh_url = _own_photo(app, author)
    just_uploaded_url = _own_photo(app, author)
    avatar = _upload(app, f"/people/{author.id}/avatar", _headers(author.id), _png())
    assert avatar.status_code == 201, avatar.text
    group = _context(postgres_session, author, "Nhóm")
    _join(postgres_session, group, author)
    group_photo = _upload(
        app,
        f"/contexts/{group.id}/photos",
        _headers(author.id, contexts=str(group.id)),
        _png(),
    )
    assert group_photo.status_code == 201, group_photo.text

    stale = uuid.UUID(_story(app, author, stale_url).json()["id"])
    shared = uuid.UUID(_story(app, author, shared_url).json()["id"])
    fresh = uuid.UUID(_story(app, author, fresh_url).json()["id"])
    posted = _call(
        app,
        "POST",
        "/posts",
        headers=_headers(author.id),
        json={"body": "có ảnh", "audience": "friends", "image_url": shared_url},
    )
    assert posted.status_code == 201, posted.text

    # Two stories expired eight days ago; one expired a minute ago (in grace).
    for story_id in (stale, shared):
        row = postgres_session.get(Story, story_id)
        row.created_at = NOW - timedelta(days=9)
        row.expires_at = NOW - timedelta(days=8)
    fresh_row = postgres_session.get(Story, fresh)
    fresh_row.created_at = NOW - DAY - timedelta(minutes=1)
    fresh_row.expires_at = NOW - timedelta(minutes=1)
    # Every photograph but the one «just uploaded» is old enough to sweep.
    old_ids = [
        uuid.UUID(url.rsplit("/", 1)[1])
        for url in (stale_url, shared_url, orphan_url, fresh_url)
    ]
    old_ids += [uuid.UUID(avatar.json()["id"]), uuid.UUID(group_photo.json()["id"])]
    for image_id in old_ids:
        postgres_session.get(UploadedImage, image_id).created_at = NOW - timedelta(
            days=9
        )
    postgres_session.flush()

    def key_of(url: str) -> str:
        image_id = uuid.UUID(url.rsplit("/", 1)[1])
        return postgres_session.get(UploadedImage, image_id).storage_key

    keys = {
        name: key_of(url)
        for name, url in (
            ("stale", stale_url),
            ("shared", shared_url),
            ("orphan", orphan_url),
            ("fresh", fresh_url),
            ("just_uploaded", just_uploaded_url),
        )
    }
    keys["avatar"] = postgres_session.get(
        UploadedImage, uuid.UUID(avatar.json()["id"])
    ).storage_key
    keys["group"] = postgres_session.get(
        UploadedImage, uuid.UUID(group_photo.json()["id"])
    ).storage_key

    rehearsal = purge_expired_stories(
        postgres_session, now=NOW, older_than=timedelta(days=7), dry_run=True
    )
    assert (rehearsal.stories, rehearsal.images, rehearsal.dry_run) == (2, 2, True)
    assert set(rehearsal.storage_keys) == {keys["stale"], keys["orphan"]}
    assert postgres_session.get(Story, stale) is not None, "dry-run không xoá gì"
    assert all(storage.read(k) for k in keys.values())

    report = purge_expired_stories(
        postgres_session, now=NOW, older_than=timedelta(days=7)
    )
    assert (report.stories, report.images) == (2, 2)
    assert postgres_session.get(Story, stale) is None
    assert postgres_session.get(Story, shared) is None
    assert postgres_session.get(Story, fresh) is not None, "còn trong thời gian ân hạn"
    remaining = {
        row.storage_key
        for row in postgres_session.scalars(
            select(UploadedImage).where(UploadedImage.uploaded_by_id == author.id)
        )
    }
    assert remaining == {
        keys["shared"],
        keys["fresh"],
        keys["just_uploaded"],
        keys["avatar"],
        keys["group"],
    }, (
        "ở lại: ảnh chung với bài, ảnh của story còn hạn, ảnh vừa tải chưa có story, "
        "ảnh đại diện, ảnh nhóm; đi: ảnh mồ côi cũ và ảnh của story cũ"
    )
    # Rows are gone, files are still there: the caller commits first.
    assert all(storage.read(k) for k in keys.values())
    assert remove_purged_files(storage, report.storage_keys) == 2
    assert storage.read(keys["shared"]) and storage.read(keys["fresh"])
    for gone in ("stale", "orphan"):
        with pytest.raises(FileNotFoundError):
            storage.read(keys[gone])
    assert remove_purged_files(storage, report.storage_keys) == 0, (
        "gỡ lần hai là không có gì"
    )
    assert repo.get_story(fresh) is not None

    with pytest.raises(ValueError):
        purge_expired_stories(postgres_session, now=NOW.replace(tzinfo=None))
