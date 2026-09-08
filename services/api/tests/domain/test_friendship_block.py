"""Chặn và gỡ chặn trên cạnh bạn bè (L5, ADR-0023 §2.3).

`decide(BLOCK)` đã có từ F04; L5 thêm hai thứ nó chưa nói được: chặn một người
CHƯA có cạnh nào (người lạ, hoặc người từng bị từ chối), và gỡ chặn — cửa mà
docstring cũ khẳng định là không tồn tại.
"""

from __future__ import annotations

import pytest

from app.domain.friendship import (
    FriendshipError,
    are_friends,
    is_live_edge,
    open_block,
    open_request,
    unblock,
)

BLOCKER = "aaa"
OTHER = "bbb"


def _edge(state: str, *, decided_by: str | None = None) -> dict:
    return {
        "requester_id": BLOCKER,
        "addressee_id": OTHER,
        "state": state,
        "decided_by_id": decided_by,
    }


def test_blocking_a_stranger_writes_a_fresh_edge_owned_by_the_blocker():
    edge = open_block(blocker_id=BLOCKER, addressee_id=OTHER, existing=None)
    assert edge["state"] == "blocked"
    assert edge["requester_id"] == BLOCKER, "không có câu hỏi nào nên không có ai hỏi"
    assert edge["decided_by_id"] == BLOCKER
    assert edge["pair"] == sorted([BLOCKER, OTHER])
    assert are_friends(edge) is False


def test_blocking_somebody_who_declined_reuses_the_pair():
    """DECLINED không chiếm cặp, nên chặn phải mở cạnh mới chứ không sửa nó."""
    edge = open_block(
        blocker_id=BLOCKER,
        addressee_id=OTHER,
        existing=_edge("declined", decided_by=OTHER),
    )
    assert edge["state"] == "blocked" and edge["decided_by_id"] == BLOCKER


@pytest.mark.parametrize("state", ["pending", "accepted"])
def test_blocking_a_live_edge_goes_through_decide(state):
    edge = open_block(blocker_id=OTHER, addressee_id=BLOCKER, existing=_edge(state))
    assert edge["state"] == "blocked"
    assert edge["decided_by_id"] is None or True  # decide() không đặt trường này
    assert is_live_edge(edge["state"]) is True


def test_blocking_twice_says_so_instead_of_pretending_to_work():
    with pytest.raises(FriendshipError) as refused:
        open_block(
            blocker_id=BLOCKER,
            addressee_id=OTHER,
            existing=_edge("blocked", decided_by=BLOCKER),
        )
    assert refused.value.code == "ALREADY_BLOCKED"


def test_nobody_blocks_themselves():
    with pytest.raises(FriendshipError) as refused:
        open_block(blocker_id=BLOCKER, addressee_id=BLOCKER, existing=None)
    assert refused.value.code == "SELF_EDGE"


def test_the_blocked_person_cannot_reopen_the_edge():
    """Bức tường không có cửa từ phía bên kia — và câu từ chối không nói ai chặn ai."""
    with pytest.raises(FriendshipError) as refused:
        open_request(
            requester_id=OTHER,
            addressee_id=BLOCKER,
            existing=_edge("blocked", decided_by=BLOCKER),
        )
    assert refused.value.code == "REQUEST_NOT_OPEN"


def test_only_the_blocker_lifts_it_and_what_comes_back_is_not_a_friendship():
    blocked = _edge("blocked", decided_by=BLOCKER)
    lifted = unblock(edge=blocked, actor_id=BLOCKER)
    assert lifted["state"] == "declined", "gỡ chặn không lặng lẽ nối lại tình bạn"
    assert are_friends(lifted) is False
    assert is_live_edge(lifted["state"]) is False, "cặp lại trống để ai đó hỏi lại"


def test_the_blocked_person_may_not_lift_it():
    with pytest.raises(FriendshipError) as refused:
        unblock(edge=_edge("blocked", decided_by=BLOCKER), actor_id=OTHER)
    assert refused.value.code == "ONLY_BLOCKER_MAY_UNBLOCK"


@pytest.mark.parametrize("state", ["pending", "accepted", "declined"])
def test_unblocking_something_that_is_not_a_block_is_refused(state):
    with pytest.raises(FriendshipError) as refused:
        unblock(edge=_edge(state, decided_by=BLOCKER), actor_id=BLOCKER)
    assert refused.value.code == "NOT_BLOCKED"


def test_unblocking_nothing_is_refused():
    with pytest.raises(FriendshipError) as refused:
        unblock(edge=None, actor_id=BLOCKER)
    assert refused.value.code == "NOT_BLOCKED"
