"""The personal-photo gate on real PostgreSQL (L3, ADR-0022 §2.1).

The fake tier re-implements «is there a readable post showing this photo» by
the same hand; here the real `_readable_by` decides, through the real HTTP
route: the owner always, a friend once a `friends` post shows the photo, a
stranger never, and everybody but the owner again once the post is gone.
Plus what `purpose` was added for: a personal photo is never an avatar, and
the CHECKs refuse the combinations the domain refuses.
"""

from __future__ import annotations

import io
import uuid

import anyio
import httpx
import pytest
from PIL import Image
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.deps import get_photo_storage
from app.api.repository import SqlAlchemyApiRepository
from app.db.models import Post, UploadedImage
from app.media.storage import PhotoStorage

from .test_group_recap_postgres import _call
from .test_posts_postgres import _befriend, _headers, _http, _person
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres


def _png() -> bytes:
    buffer = io.BytesIO()
    Image.new("RGB", (48, 32), (201, 57, 0)).save(buffer, format="PNG")
    return buffer.getvalue()


def _app_with_storage(session, monkeypatch, tmp_path):
    app = _http(session, monkeypatch)
    app.dependency_overrides[get_photo_storage] = lambda: PhotoStorage(tmp_path)
    return app


def _upload(app, path: str, headers: dict, png: bytes):
    async def run():
        transport = httpx.ASGITransport(app=app)
        async with httpx.AsyncClient(
            transport=transport, base_url="http://testserver"
        ) as client:
            return await client.post(
                path, headers=headers, files={"file": ("anh.png", png, "image/png")}
            )

    return anyio.run(run)


def app_get(app, path, *, headers):
    return _call(app, "GET", path, headers=headers)


def app_post(app, path, *, headers, json=None):
    return _call(app, "POST", path, headers=headers, json=json)


def test_the_gate_follows_the_post_and_closes_when_the_post_is_gone(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path
):
    owner = _person(postgres_session, "Chủ ảnh")
    friend = _person(postgres_session, "Bạn")
    stranger = _person(postgres_session, "Người lạ")
    _befriend(postgres_session, owner, friend)
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)

    uploaded = _upload(app, "/people/me/photos", _headers(owner.id), _png())
    assert uploaded.status_code == 201, uploaded.text
    url = uploaded.json()["url"]
    assert url == f"/people/{owner.id}/photos/{uploaded.json()['id']}"
    row = postgres_session.get(UploadedImage, uuid.UUID(uploaded.json()["id"]))
    assert row.purpose == "personal" and row.owner_person_id == owner.id

    assert app_get(app, url, headers=_headers(owner.id)).status_code == 200
    absent = app_get(
        app, f"/people/{owner.id}/photos/{uuid.uuid4()}", headers=_headers(friend.id)
    )
    for who in (friend, stranger):
        refused = app_get(app, url, headers=_headers(who.id))
        assert refused.status_code == 404, (who.display_name, refused.text)
        assert refused.json() == absent.json(), "cùng một câu với ảnh không tồn tại"

    written = app_post(
        app,
        "/posts",
        headers=_headers(owner.id),
        json={"body": "có ảnh", "audience": "friends", "image_url": url},
    )
    assert written.status_code == 201, written.text
    assert app_get(app, url, headers=_headers(friend.id)).status_code == 200
    assert app_get(app, url, headers=_headers(stranger.id)).status_code == 404

    postgres_session.delete(postgres_session.get(Post, uuid.UUID(written.json()["id"])))
    postgres_session.flush()
    assert app_get(app, url, headers=_headers(friend.id)).status_code == 404, (
        "bài bị xoá thì ảnh của nó đóng lại với mọi người trừ chủ"
    )
    assert app_get(app, url, headers=_headers(owner.id)).status_code == 200


def test_the_newest_personal_photo_is_not_the_avatar(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch, tmp_path
):
    owner = _person(postgres_session, "Chủ ảnh")
    mate = _person(postgres_session, "Cùng nhóm")
    app = _app_with_storage(postgres_session, monkeypatch, tmp_path)
    repo = SqlAlchemyApiRepository(postgres_session)

    avatar = _upload(app, f"/people/{owner.id}/avatar", _headers(owner.id), _png())
    assert avatar.status_code == 201, avatar.text
    later = _upload(app, "/people/me/photos", _headers(owner.id), _png())
    assert later.status_code == 201, later.text

    latest = repo.get_latest_avatar(owner.id)
    assert latest is not None and str(latest.id) == avatar.json()["id"]
    assert repo.get_person_image(owner.id, uuid.UUID(avatar.json()["id"])) is None, (
        "ảnh đại diện không phải ảnh cá nhân để đăng bài"
    )
    assert mate.id != owner.id


@pytest.mark.parametrize(
    ("purpose", "context", "owner"),
    [
        ("group", None, True),
        ("avatar", True, False),
        ("personal", True, False),
        ("selfie", None, True),
    ],
    ids=[
        "group-without-context",
        "avatar-with-context",
        "personal-with-context",
        "unknown-purpose",
    ],
)
def test_the_checks_refuse_a_purpose_that_does_not_match_its_owner(
    postgres_session: Session, purpose, context, owner
):
    from app.db.models import Context

    person = _person(postgres_session, "Ai đó")
    group = Context(id=uuid.uuid4(), display_name="Nhóm", created_by_id=person.id)
    postgres_session.add(group)
    postgres_session.flush()
    with pytest.raises(IntegrityError) as refused:
        with postgres_session.begin_nested():
            postgres_session.add(
                UploadedImage(
                    storage_key=uuid.uuid4().hex,
                    context_id=group.id if context else None,
                    owner_person_id=person.id if owner else None,
                    uploaded_by_id=person.id,
                    content_type="image/png",
                    byte_size=1,
                    width=1,
                    height=1,
                    created_at=NOW,
                    purpose=purpose,
                )
            )
            postgres_session.flush()
    assert "ck_uploaded_images_image_purpose" in str(refused.value)
