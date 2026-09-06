"""The wire model is the third spelling of the payload CHECK (ADR-0021 §2.3).

A `deleted` row that still carried its text, its picture, its card, a reaction
or no timestamp must not serialise; a sticker row must name a known sticker.
This is what stands between a repository bug and a leak the database cannot
see -- the CHECK guards writes, this guards every read.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime

import pytest
from pydantic import ValidationError

from app.api.schemas import MessageResponse, ReactionSummary

NOW = datetime(2030, 8, 27, 12, tzinfo=UTC)


def _row(**overrides):
    base = {
        "id": uuid.uuid4(),
        "context_id": uuid.uuid4(),
        "author_id": uuid.uuid4(),
        "kind": "deleted",
        "body": None,
        "image_url": None,
        "card": None,
        "created_at": NOW,
        "cursor": "abc",
        "deleted_at": NOW,
    }
    return {**base, **overrides}


def test_a_well_formed_deleted_row_serialises():
    MessageResponse(**_row())


@pytest.mark.parametrize(
    "leak",
    [
        {"body": "lời cũ"},
        {"image_url": "/contexts/x/photos/y"},
        {"card": {"kind": "text", "payload": {"text": "x"}}},
        {"deleted_at": None},
        {"reactions": [ReactionSummary(kind="heart", count=1, mine=False)]},
    ],
)
def test_a_deleted_row_that_kept_anything_is_refused(leak):
    with pytest.raises(ValidationError):
        MessageResponse(**_row(**leak))


def test_a_live_row_may_not_carry_a_deletion_timestamp():
    with pytest.raises(ValidationError):
        MessageResponse(**_row(kind="text", body="còn sống", deleted_at=NOW))


def test_a_sticker_row_must_name_a_known_sticker():
    MessageResponse(**_row(kind="sticker", body="di-thoi", deleted_at=None))
    with pytest.raises(ValidationError):
        MessageResponse(**_row(kind="sticker", body="khong-co", deleted_at=None))
    with pytest.raises(ValidationError):
        MessageResponse(**_row(kind="sticker", body=None, deleted_at=None))
