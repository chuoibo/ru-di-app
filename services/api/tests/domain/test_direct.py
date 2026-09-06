"""A direct message is a two-person context (ADR-0021 §2.5): the pure rules."""

from __future__ import annotations

import pytest

from app.domain import direct
from app.domain.friendship import FriendshipError

A = "aa00aa00-0a0a-4a0a-8a0a-0a0a0a0a0aa0"
B = "bb00bb00-0b0b-4b0b-8b0b-0b0b0b0b0bb0"
C = "cc00cc00-0c0c-4c0c-8c0c-0c0c0c0c0cc0"


def test_the_pair_key_is_the_same_whoever_opens_the_conversation():
    assert direct.pair_key(A, B) == direct.pair_key(B, A) == f"{A}:{B}"


def test_the_pair_key_refuses_one_person_twice_and_an_empty_id():
    with pytest.raises(FriendshipError) as refused:
        direct.pair_key(A, A)
    assert "SELF_EDGE" in str(refused.value)
    with pytest.raises(FriendshipError):
        direct.pair_key(A, "")


def test_kinds_are_exactly_group_and_pair():
    assert direct.KINDS == ("group", "pair")
    assert direct.is_kind("group") and direct.is_kind("pair")
    assert not direct.is_kind("dm")
    assert not direct.is_kind(None)
    assert direct.is_pair("pair")
    assert not direct.is_pair("group")
    assert not direct.is_pair(None)


@pytest.mark.parametrize(
    ("is_friend", "other_exists", "other_deleted", "expected"),
    [
        (True, True, False, True),
        (False, True, False, False),
        (True, False, False, False),
        (True, True, True, False),
        (False, False, True, False),
    ],
)
def test_can_open_needs_a_living_friend(
    is_friend, other_exists, other_deleted, expected
):
    assert (
        direct.can_open(
            is_friend=is_friend, other_exists=other_exists, other_deleted=other_deleted
        )
        is expected
    )


def test_counterpart_is_the_one_other_member_or_nothing():
    assert direct.counterpart_of([A, B], A) == B
    assert direct.counterpart_of([A, B], B) == A
    assert direct.counterpart_of([A], A) is None, "một người không phải một cặp"
    assert direct.counterpart_of([A, B, C], A) is None, "ba người không phải một cặp"
    assert direct.counterpart_of([], A) is None


def test_a_group_keeps_its_name_and_a_pair_takes_the_other_persons():
    assert direct.display_name_for("group", "Hội đi Đà Lạt", None) == "Hội đi Đà Lạt"
    assert direct.display_name_for("group", "Hội đi Đà Lạt", "Bình") == "Hội đi Đà Lạt"
    assert direct.display_name_for("pair", "", "Bình") == "Bình"


def test_a_pair_with_no_name_to_show_gets_a_word_never_an_id():
    for missing in (None, ""):
        shown = direct.display_name_for("pair", "", missing)
        assert shown == direct.ANONYMOUS_COUNTERPART
        assert "-" not in shown, "một id không bao giờ là tên hiển thị"


def test_the_roster_only_doors_are_a_closed_list_of_distinct_names():
    doors = direct.ROSTER_ONLY_DOORS
    assert isinstance(doors, tuple)
    assert len(doors) == len(set(doors))
    assert {"invite_context_member", "leave_context", "rename_context"} <= set(doors)
    # Money and outings stay open in a pair (§2.5.5): splitting a bill between
    # two friends is a real thing to do.
    assert not any(
        "expense" in door or "bill" in door or "batch" in door for door in doors
    )
    assert "create_outing" not in doors
