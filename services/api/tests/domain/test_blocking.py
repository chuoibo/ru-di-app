"""`blocking` (L5, ADR-0023 §2.3): what a block stops, and what it does not."""

from __future__ import annotations

import pytest

from app.domain import blocking

BLOCKER = "aaa"
BLOCKED = "bbb"
EDGE = {
    "requester_id": BLOCKER,
    "addressee_id": BLOCKED,
    "state": "blocked",
    "decided_by_id": BLOCKER,
}


@pytest.mark.parametrize(
    ("state", "expected"),
    [("blocked", True), ("accepted", False), ("pending", False), ("declined", False)],
)
def test_only_the_blocked_state_is_a_block(state, expected):
    assert blocking.is_blocked({**EDGE, "state": state}) is expected


def test_no_edge_is_not_a_block():
    assert blocking.is_blocked(None) is False
    assert blocking.blocker_of(None) is None
    assert blocking.hidden_between(None) is False


def test_the_blocker_is_whoever_decided_not_whoever_asked():
    # A friendship ended by the person who was originally asked: the blocker
    # is the decider, and only they may lift it.
    ended_by_addressee = {**EDGE, "decided_by_id": BLOCKED}
    assert blocking.blocker_of(ended_by_addressee) == BLOCKED
    assert blocking.blocker_of(EDGE) == BLOCKER
    assert blocking.blocker_of({**EDGE, "state": "accepted"}) is None


def test_hiding_is_symmetric():
    """A block is not a mute: neither side reads the other's wall."""
    assert blocking.hidden_between(EDGE) is True
    assert blocking.hidden_between({**EDGE, "decided_by_id": BLOCKED}) is True


@pytest.mark.parametrize(
    ("edge", "other_deleted", "allowed"),
    [
        (None, False, True),
        ({**EDGE, "state": "accepted"}, False, True),
        (EDGE, False, False),
        (None, True, False),
        (EDGE, True, False),
    ],
    ids=["strangers", "friends", "blocked", "other-deleted", "both"],
)
def test_a_pair_stops_accepting_messages_for_either_reason(
    edge, other_deleted, allowed
):
    assert blocking.dm_allowed(edge, other_deleted=other_deleted) is allowed


def test_both_refusals_share_one_code():
    """The weak oracle ADR-0023 §2.3.2 accepts on purpose: one sentence for
    «blocked» and «they deleted their account»."""
    assert blocking.DIRECT_MESSAGE_UNAVAILABLE == "direct_message_unavailable"
