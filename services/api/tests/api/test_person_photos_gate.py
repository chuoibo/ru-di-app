"""Personal photographs (L3, ADR-0022 §2.1) against the fake.

What this layer proves: the owner uploads and reads back; nobody else may read
until a post that shows the photo is readable by them, and then exactly they
may; every refusal is the same 404 as «no such photo»; a post may show one's
own personal photo (any audience) or a group photo only when addressed to that
group; and memories/chat still refuse the personal shape. What it does not
prove: the gate's SQL against `_readable_by` and the CASCADE -- see
tests/postgres/test_person_photo_gate_postgres.py.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime

import anyio
import pytest

from app.api.deps import get_photo_storage, get_repository
from app.api.main import create_app
from app.api.repository import ContextRecord, PersonRecord
from app.media.storage import PhotoStorage

from .conftest import ASGITestClient
from .helpers import (
    ADVANCER_ID,
    CONTEXT_ID,
    OTHER_ID,
    SENDER_ID,
    actor_headers,
    png_bytes,
)
from .test_posts_audience import befriend, post_body

AUTHOR = ADVANCER_ID
GROUPMATE = SENDER_ID
STRANGER = OTHER_ID
FRIEND = uuid.UUID("5ee00000-eeee-4eee-8eee-0000e0000004")
OTHER_CONTEXT_ID = uuid.UUID("6ff00000-ffff-4fff-8fff-0000f0000004")
T0 = datetime(2030, 8, 27, 12, tzinfo=UTC)


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


def _upload(client, actor):
    response = client.post(
        "/people/me/photos",
        files={"file": ("anh.png", png_bytes(), "image/png")},
        headers=actor_headers(actor),
    )
    assert response.status_code == 201, response.text
    return response.json()


def _upload_group(client, actor):
    response = client.post(
        f"/contexts/{CONTEXT_ID}/photos",
        files={"file": ("anh.png", png_bytes(), "image/png")},
        headers=actor_headers(actor),
    )
    assert response.status_code == 201, response.text
    return response.json()


def _get(client, url, actor):
    return client.get(url, headers=actor_headers(actor))


def _post(client, audience, *, image_url, context_id=None, author=AUTHOR):
    return client.post(
        "/posts",
        json={**post_body(audience, context_id=context_id), "image_url": image_url},
        headers=actor_headers(author),
    )


def test_the_owner_reads_their_own_photo_and_nobody_else_does_until_a_post_shows_it(
    client, repository
):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    assert photo["url"] == f"/people/{AUTHOR}/photos/{photo['id']}"
    assert photo["context_id"] is None
    stored = repository.uploaded_images[uuid.UUID(photo["id"])]
    assert stored.purpose == "personal" and stored.owner_person_id == AUTHOR

    mine = _get(client, photo["url"], AUTHOR)
    assert mine.status_code == 200, mine.text
    # The sanitizer re-encodes what it stores; the type it answers is its own.
    assert mine.headers["content-type"].startswith("image/")
    for who in (FRIEND, GROUPMATE, STRANGER):
        refused = _get(client, photo["url"], who)
        absent = _get(client, f"/people/{AUTHOR}/photos/{uuid.uuid4()}", who)
        assert refused.status_code == 404, (who, refused.text)
        assert refused.json() == absent.json(), "cùng một câu với ảnh không tồn tại"

    written = _post(client, "friends", image_url=photo["url"])
    assert written.status_code == 201, written.text
    assert written.json()["image_url"] == photo["url"]
    assert _get(client, photo["url"], FRIEND).status_code == 200
    assert _get(client, photo["url"], GROUPMATE).status_code == 404
    assert _get(client, photo["url"], STRANGER).status_code == 404


def test_a_personal_photo_is_not_an_avatar(client, repository):
    _seed(repository)
    _upload(client, AUTHOR)
    assert _get(client, f"/people/{AUTHOR}/avatar", AUTHOR).status_code == 404, (
        "ảnh cá nhân mới nhất không được thành ảnh đại diện"
    )
    avatar = client.post(
        f"/people/{AUTHOR}/avatar",
        files={"file": ("mat.png", png_bytes(), "image/png")},
        headers=actor_headers(AUTHOR),
    )
    assert avatar.status_code == 201, avatar.text
    assert (
        repository.uploaded_images[uuid.UUID(avatar.json()["id"])].purpose == "avatar"
    )
    _upload(client, AUTHOR)  # a newer personal photo must not shadow the avatar
    assert _get(client, f"/people/{AUTHOR}/avatar", GROUPMATE).status_code == 200


def test_a_post_may_show_ones_own_photo_but_not_somebody_elses(client, repository):
    _seed(repository)
    theirs = _upload(client, FRIEND)
    stolen = _post(client, "public", image_url=theirs["url"])
    assert stolen.status_code == 403, stolen.text
    missing = _post(
        client, "public", image_url=f"/people/{AUTHOR}/photos/{uuid.uuid4()}"
    )
    assert missing.status_code == 404, missing.text
    assert missing.json()["code"] == "photo_not_found"
    assert not repository.posts


def test_a_group_photo_illustrates_only_a_post_addressed_to_that_group(
    client, repository
):
    _seed(repository)
    group_photo = _upload_group(client, AUTHOR)
    elsewhere = _post(client, "public", image_url=group_photo["url"])
    assert elsewhere.status_code == 422, elsewhere.text
    assert elsewhere.json()["code"] == "photo_not_addressable"
    to_friends = _post(client, "friends", image_url=group_photo["url"])
    assert to_friends.status_code == 422
    # A second group the author IS in: membership passes, and only then does
    # the photo's group get compared with the post's.
    repository.contexts[OTHER_CONTEXT_ID] = ContextRecord(
        id=OTHER_CONTEXT_ID,
        display_name="Hội khác",
        created_by_id=AUTHOR,
        created_at=T0,
    )
    repository.active_memberships.add((OTHER_CONTEXT_ID, AUTHOR))
    other_group = _post(
        client, "group", image_url=group_photo["url"], context_id=OTHER_CONTEXT_ID
    )
    assert other_group.status_code == 422, other_group.text
    assert other_group.json()["code"] == "photo_not_addressable"
    stranger_group = _post(
        client, "group", image_url=group_photo["url"], context_id=uuid.uuid4()
    )
    assert stranger_group.status_code == 403, (
        "không phải thành viên nhóm được nêu: 403 như cũ"
    )
    home = _post(client, "group", image_url=group_photo["url"], context_id=CONTEXT_ID)
    assert home.status_code == 201, home.text
    outsider = _post(
        client,
        "group",
        image_url=group_photo["url"],
        context_id=CONTEXT_ID,
        author=STRANGER,
    )
    assert outsider.status_code == 403, outsider.text


def test_the_url_is_stored_in_canonical_form(client, repository):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    shouted = (
        photo["url"]
        .upper()
        .replace("/PEOPLE/", "/people/")
        .replace("/PHOTOS/", "/photos/")
    )
    written = _post(client, "friends", image_url=shouted)
    assert written.status_code == 201, written.text
    assert written.json()["image_url"] == photo["url"], "id viết hoa vẫn là một ảnh"
    assert _get(client, photo["url"], FRIEND).status_code == 200


def test_memories_and_chat_still_refuse_the_personal_shape(client, repository):
    _seed(repository)
    photo = _upload(client, AUTHOR)
    memory = client.post(
        f"/contexts/{CONTEXT_ID}/memories",
        json={"image_url": photo["url"], "caption": "x"},
        headers=actor_headers(AUTHOR),
    )
    assert memory.status_code == 422, memory.text
    message = client.post(
        f"/contexts/{CONTEXT_ID}/messages",
        json={"kind": "image", "image_url": photo["url"]},
        headers=actor_headers(AUTHOR),
    )
    assert message.status_code == 422, message.text
