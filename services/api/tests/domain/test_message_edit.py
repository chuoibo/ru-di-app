"""Reply and delete rules are pure and refuse in the right order (ADR-0021)."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest

from app.domain.message_edit import (
    DELETABLE_KINDS,
    REPLYABLE_KINDS,
    MessageEditError,
    check_deletable,
    check_reply_target,
    deleted_shape,
)

NOW = datetime(2030, 8, 27, 12, tzinfo=UTC)
ME = "4dd00000-dddd-4ddd-8ddd-0000d0000009"
OTHER = "5ee00000-eeee-4eee-8eee-0000e0000009"
GROUP = "3cc00000-cccc-4ccc-8ccc-0000c0000009"


def _message(**overrides):
    base = {"id": "m1", "context_id": GROUP, "author_id": ME, "kind": "text"}
    return {**base, **overrides}


def test_cards_are_neither_deletable_nor_quotable():
    assert "ai_card" not in DELETABLE_KINDS
    assert "ai_card" not in REPLYABLE_KINDS
    assert "deleted" not in DELETABLE_KINDS
    assert set(DELETABLE_KINDS) == {"text", "image", "sticker"} == set(REPLYABLE_KINDS)


@pytest.mark.parametrize("kind", DELETABLE_KINDS)
def test_the_author_may_delete_their_own_live_human_message(kind):
    check_deletable(_message(kind=kind), ME)  # does not raise


def test_somebody_else_may_not_delete_it_even_with_the_same_kind():
    with pytest.raises(MessageEditError) as refused:
        check_deletable(_message(), OTHER)
    assert refused.value.code == "NOT_AUTHOR"


def test_an_authorless_card_is_refused_as_not_the_callers():
    with pytest.raises(MessageEditError) as refused:
        check_deletable(_message(kind="ai_card", author_id=None), ME)
    assert refused.value.code == "NOT_AUTHOR"


def test_ones_own_poll_card_is_refused_by_kind():
    with pytest.raises(MessageEditError) as refused:
        check_deletable(_message(kind="ai_card"), ME)
    assert refused.value.code == "KIND_NOT_DELETABLE"


def test_already_deleted_is_said_before_ownership_is_examined():
    with pytest.raises(MessageEditError) as refused:
        check_deletable(_message(kind="deleted"), OTHER)
    assert refused.value.code == "ALREADY_DELETED"


def test_deleted_shape_drops_the_payload_and_keeps_the_author():
    shape = deleted_shape(
        _message(body="xin chào", image_url=None, card=None, kind="text"), NOW
    )
    assert shape["kind"] == "deleted"
    assert (
        shape["body"] is None and shape["image_url"] is None and shape["card"] is None
    )
    assert shape["deleted_at"] == NOW
    assert shape["author_id"] == ME


def test_deleted_shape_refuses_a_naive_clock():
    with pytest.raises(MessageEditError) as refused:
        deleted_shape(_message(), datetime(2030, 8, 27, 12))
    assert refused.value.code == "NAIVE_DATETIME"


def test_absent_and_cross_group_targets_are_the_same_refusal():
    with pytest.raises(MessageEditError) as absent:
        check_reply_target(None, GROUP)
    with pytest.raises(MessageEditError) as elsewhere:
        check_reply_target(_message(context_id="another-group"), GROUP)
    assert absent.value.code == elsewhere.value.code == "REPLY_NOT_FOUND"


def test_a_deleted_target_and_a_card_target_have_their_own_codes():
    with pytest.raises(MessageEditError) as deleted:
        check_reply_target(_message(kind="deleted"), GROUP)
    assert deleted.value.code == "REPLY_TO_DELETED"
    with pytest.raises(MessageEditError) as card:
        check_reply_target(_message(kind="ai_card"), GROUP)
    assert card.value.code == "REPLY_TO_CARD"


@pytest.mark.parametrize("kind", REPLYABLE_KINDS)
def test_a_live_human_message_in_the_same_group_may_be_quoted(kind):
    check_reply_target(_message(kind=kind), GROUP)  # does not raise
