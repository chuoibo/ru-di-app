"""Who may comment under a post, and who may take a comment back (ADR-0022 §2.2)."""

from __future__ import annotations

import itertools

import pytest

from app.domain import post_audience
from app.domain.post_audience import (
    COMMENT_POLICIES,
    DEFAULT_COMMENT_POLICY,
    can_comment,
    can_delete_comment,
    can_read,
    is_comment_policy,
)

AUTHOR = "aa00aa00-0a0a-4a0a-8a0a-0a0a0a0a0aa0"
READER = "bb00bb00-0b0b-4b0b-8b0b-0b0b0b0b0bb0"
GROUP = "c3c3c3c3-0c0c-4c0c-8c0c-0c0c0c0c0c03"


def _post(audience: str) -> dict:
    return {
        "author_id": AUTHOR,
        "audience": audience,
        "context_id": GROUP if audience == "group" else None,
    }


def test_the_default_policy_is_one_of_the_three():
    assert DEFAULT_COMMENT_POLICY in COMMENT_POLICIES
    assert COMMENT_POLICIES == ("readers", "friends", "nobody")
    assert all(is_comment_policy(p) for p in COMMENT_POLICIES)
    assert not is_comment_policy("everyone")
    assert not is_comment_policy(None)


@pytest.mark.parametrize(
    ("policy", "audience", "is_friend", "is_group_member"),
    list(
        itertools.product(
            COMMENT_POLICIES, post_audience.AUDIENCES, (False, True), (False, True)
        )
    ),
)
def test_commenting_is_reading_plus_the_wall_owners_policy(
    policy, audience, is_friend, is_group_member
):
    """The whole truth table: 3 policies × 4 audiences × 2 facts × 2 facts."""
    post = _post(audience)
    readable = can_read(
        post, reader_id=READER, is_friend=is_friend, is_group_member=is_group_member
    )
    expected = readable and (policy == "readers" or (policy == "friends" and is_friend))
    assert (
        can_comment(
            post,
            policy=policy,
            reader_id=READER,
            is_friend=is_friend,
            is_group_member=is_group_member,
        )
        is expected
    )


def test_the_author_may_always_comment_on_their_own_post():
    for policy in COMMENT_POLICIES:
        for audience in post_audience.AUDIENCES:
            assert can_comment(
                _post(audience),
                policy=policy,
                reader_id=AUTHOR,
                is_friend=False,
                is_group_member=False,
            )


def test_an_unknown_policy_closes_the_composer():
    assert not can_comment(
        _post("public"),
        policy="everyone",
        reader_id=READER,
        is_friend=True,
        is_group_member=True,
    )


def test_a_comment_is_deleted_by_its_author_or_by_the_posts_author_only():
    post = _post("public")
    comment = {"author_id": READER}
    assert can_delete_comment(comment, post, READER)
    assert can_delete_comment(comment, post, AUTHOR)
    assert not can_delete_comment(comment, post, "cc00cc00-0c0c-4c0c-8c0c-0c0c0c0c0cc0")
