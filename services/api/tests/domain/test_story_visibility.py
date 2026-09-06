"""`story_visibility` (L4, ADR-0022 §2.3): lifetime, audience, who may view,
and the rail's order. Pure; the boundary cases are the point."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest

from app.domain import story_visibility as sv

T0 = datetime(2030, 8, 27, 12, tzinfo=UTC)
STORY = {
    "author_id": "author",
    "audience": "friends",
    "expires_at": T0 + timedelta(hours=24),
}


def test_the_deadline_is_twenty_four_hours_after_writing():
    assert sv.expires_at_for(T0) == T0 + timedelta(hours=24)
    assert sv.STORY_TTL == timedelta(hours=24)


def test_a_naive_datetime_is_refused_everywhere():
    naive = datetime(2030, 8, 27, 12)
    with pytest.raises(sv.StoryError) as refused:
        sv.expires_at_for(naive)
    assert refused.value.code == "NAIVE_DATETIME"
    with pytest.raises(sv.StoryError):
        sv.is_live(STORY, naive)
    with pytest.raises(sv.StoryError):
        sv.is_live({**STORY, "expires_at": naive}, T0)
    with pytest.raises(sv.StoryError):
        sv.can_view(
            STORY, reader_id="friend", is_friend=True, is_blocked=False, now=naive
        )


@pytest.mark.parametrize(
    ("offset", "live"),
    [
        (timedelta(hours=24) - timedelta(seconds=1), True),
        (timedelta(hours=24), False),
        (timedelta(hours=24) + timedelta(seconds=1), False),
        (timedelta(0), True),
    ],
    ids=["one-second-before", "at-the-deadline", "one-second-after", "just-written"],
)
def test_liveness_is_strict_at_the_deadline(offset, live):
    assert sv.is_live(STORY, T0 + offset) is live


@pytest.mark.parametrize(
    ("reader", "is_friend", "is_blocked", "offset", "expected"),
    [
        ("author", False, False, timedelta(0), True),
        ("author", False, False, timedelta(hours=48), True),
        ("friend", True, False, timedelta(0), True),
        ("friend", True, False, timedelta(hours=24), False),
        ("friend", True, True, timedelta(0), False),
        ("stranger", False, False, timedelta(0), False),
    ],
    ids=[
        "author-live",
        "author-expired-still-theirs",
        "friend-live",
        "friend-expired",
        "friend-blocked",
        "stranger",
    ],
)
def test_who_may_view(reader, is_friend, is_blocked, offset, expected):
    assert (
        sv.can_view(
            STORY,
            reader_id=reader,
            is_friend=is_friend,
            is_blocked=is_blocked,
            now=T0 + offset,
        )
        is expected
    )


def test_an_unknown_audience_fails_closed_even_for_the_author():
    odd = {**STORY, "audience": "public"}
    assert not sv.can_view(
        odd, reader_id="author", is_friend=True, is_blocked=False, now=T0
    )
    assert not sv.can_view(
        odd, reader_id="friend", is_friend=True, is_blocked=False, now=T0
    )


def test_the_audience_vocabulary_has_one_word():
    assert sv.STORY_AUDIENCES == ("friends",)
    assert sv.DEFAULT_STORY_AUDIENCE in sv.STORY_AUDIENCES


def test_caption_length():
    sv.check_caption(None)
    sv.check_caption("x" * sv.MAX_CAPTION_LENGTH)
    with pytest.raises(sv.StoryError) as refused:
        sv.check_caption("x" * (sv.MAX_CAPTION_LENGTH + 1))
    assert refused.value.code == "CAPTION_TOO_LONG"
    with pytest.raises(sv.StoryError) as not_text:
        sv.check_caption(42)
    assert not_text.value.code == "CAPTION_NOT_TEXT"


def test_the_rail_orders_mine_then_unseen_then_seen_newest_first():
    groups = [
        {"name": "seen-old", "mine": False, "all_seen": True, "latest_at": T0},
        {"name": "unseen-old", "mine": False, "all_seen": False, "latest_at": T0},
        {
            "name": "mine",
            "mine": True,
            "all_seen": True,
            "latest_at": T0 - timedelta(hours=5),
        },
        {
            "name": "unseen-new",
            "mine": False,
            "all_seen": False,
            "latest_at": T0 + timedelta(hours=1),
        },
        {
            "name": "seen-new",
            "mine": False,
            "all_seen": True,
            "latest_at": T0 + timedelta(hours=1),
        },
    ]
    assert [g["name"] for g in sv.order_authors(groups)] == [
        "mine",
        "unseen-new",
        "unseen-old",
        "seen-new",
        "seen-old",
    ]


def test_ordering_is_stable_for_ties():
    a = {"name": "a", "mine": False, "all_seen": False, "latest_at": T0}
    b = {"name": "b", "mine": False, "all_seen": False, "latest_at": T0}
    assert [g["name"] for g in sv.order_authors([a, b])] == ["a", "b"]
    assert [g["name"] for g in sv.order_authors([b, a])] == ["b", "a"]
