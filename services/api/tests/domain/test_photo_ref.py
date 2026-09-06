"""`image_url` parsing (ADR-0022 §2.1): two shapes, one refusal."""

from __future__ import annotations

import uuid

import pytest

from app.domain.photo_ref import (
    OWNER_CONTEXT,
    OWNER_PERSON,
    PhotoUrlError,
    context_photo_url,
    parse_photo_url,
    person_photo_url,
)

CTX = "1aa00000-aaaa-4aaa-8aaa-0000a0000001"
PID = "2bb00000-bbbb-4bbb-8bbb-0000b0000001"
PHOTO = "3cc00000-cccc-4ccc-8ccc-0000c0000001"


def test_a_group_photo_and_a_personal_photo_parse_to_their_owner():
    group = parse_photo_url(f"/contexts/{CTX}/photos/{PHOTO}")
    assert (group.owner_kind, group.owner_id, group.photo_id) == (
        OWNER_CONTEXT,
        CTX,
        PHOTO,
    )
    person = parse_photo_url(f"/people/{PID}/photos/{PHOTO}")
    assert (person.owner_kind, person.owner_id, person.photo_id) == (
        OWNER_PERSON,
        PID,
        PHOTO,
    )


def test_the_canonical_url_is_what_gets_compared_later():
    shouted = parse_photo_url(f"/people/{PID.upper()}/photos/{PHOTO.upper()}")
    assert shouted.url == f"/people/{PID}/photos/{PHOTO}"
    assert person_photo_url(PID, PHOTO) == shouted.url
    assert context_photo_url(CTX, PHOTO) == f"/contexts/{CTX}/photos/{PHOTO}"


@pytest.mark.parametrize(
    "bad",
    [
        None,
        "",
        "/",
        "/contexts",
        "/contexts/not-a-uuid/photos/also-not",
        "/contexts//photos/",
        f"/contexts/{CTX}/photos/{PHOTO}/",
        f"/contexts/{CTX}/photos/{PHOTO}?x=1",
        f"/groups/{CTX}/photos/{PHOTO}",
        f"/people/{PID}/avatar",
        "https://tracker.example/pixel.png",
        "javascript:alert(1)",
        "/contexts/../../etc/passwd",
        f"contexts/{CTX}/photos/{PHOTO}",
    ],
)
def test_anything_else_is_one_refusal(bad):
    with pytest.raises(PhotoUrlError) as refused:
        parse_photo_url(bad)
    assert refused.value.code == "MALFORMED"


def test_ids_are_real_uuids_not_just_uuid_shaped():
    assert uuid.UUID(parse_photo_url(f"/people/{PID}/photos/{PHOTO}").photo_id)
