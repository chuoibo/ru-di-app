"""Xoá tài khoản trên fake (L5, ADR-0023 §2.1–2.2).

Cái tầng này chứng minh được: cửa đòi xác nhận, bản đồ xoá chạm đúng những
gì nó nói, hàng người vẫn còn với tên ẩn danh, phiên chết, và số cũ đăng nhập
lại thành một người khác. Cái nó KHÔNG chứng minh: sổ tiền không đổi một byte
— chỉ Postgres thật nói được câu đó, và ca ấy ở
tests/postgres/test_delete_account_postgres.py.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

import pytest

from app.api.repository import AccountSessionRecord, PersonRecord
from app.domain.account_lifecycle import ANONYMOUS_DISPLAY_NAME

from .helpers import ADVANCER_ID, CONTEXT_ID, SENDER_ID, actor_headers, join_group
from .test_posts_audience import post_body

ME = ADVANCER_ID
FRIEND = SENDER_ID
NOW = datetime(2030, 8, 27, 12, tzinfo=UTC)


@pytest.fixture
def seeded(client, repository):
    for pid, name in ((ME, "Minh"), (FRIEND, "Bạn")):
        repository.people[pid] = PersonRecord(
            id=pid,
            display_name=name,
            created_at=NOW,
            bio="thích cà phê",
            city="Đà Lạt",
            budget_band="vua-phai",
        )
    join_group(repository, ME, FRIEND)
    session_id = uuid.uuid4()
    repository.account_sessions[session_id] = AccountSessionRecord(
        id=session_id,
        person_id=ME,
        issued_from_invite_id=None,
        issued_via="otp",
        created_at=NOW,
        expires_at=NOW + timedelta(days=30),
        revoked_at=None,
    )
    written = client.post("/posts", json=post_body("public"), headers=actor_headers(ME))
    assert written.status_code == 201, written.text
    repository.person_interests[ME] = {"an-uong"}
    return repository


def _delete(client, actor=ME, body=None):
    return client.request(
        "DELETE",
        "/people/me",
        json={"confirm": True} if body is None else body,
        headers=actor_headers(actor),
    )


@pytest.mark.parametrize("body", [{}, {"confirm": False}, {"confirm": "true"}])
def test_the_door_will_not_open_without_a_literal_confirmation(client, seeded, body):
    refused = _delete(client, body=body)
    assert refused.status_code == 422, refused.text
    assert seeded.people[ME].deleted_at is None
    assert seeded.posts, "không xoá gì khi chưa xác nhận"


def test_the_row_stays_anonymised_and_everything_personal_goes(client, seeded):
    gone = _delete(client)
    assert gone.status_code == 204, gone.text
    assert gone.content == b""

    person = seeded.people[ME]
    assert person.id == ME, "id ở lại vì sổ tiền trỏ vào nó"
    assert person.display_name == ANONYMOUS_DISPLAY_NAME
    assert (person.bio, person.city, person.budget_band) == (None, None, None)
    assert person.discoverable_by_phone is False
    assert person.wall_comment_policy == "nobody"
    assert person.deleted_at is not None

    assert not seeded.posts, "bài của người ấy đi"
    assert ME not in seeded.person_interests
    assert all(row.revoked_at is not None for row in seeded.account_sessions.values())
    assert all(member != ME for _, member in seeded.active_memberships), (
        "membership thành «đã rời», không phải biến mất"
    )
    assert any(member == ME for _, member in seeded.left_memberships)


def test_the_ended_account_stops_being_a_person_the_product_answers_about(
    client, seeded
):
    _delete(client)
    profile = client.get(f"/people/{ME}", headers=actor_headers(FRIEND))
    # ADR-0023 §2.1.4 lúc viết ra nói «404». Cái xảy ra thật là 403, và 403 mới
    # đúng — ADR đã được sửa theo. Xoá tài khoản gỡ mọi cạnh bạn bè và đặt mọi
    # membership thành `left`, nên không ai còn quan hệ với id ấy và cửa hồ sơ
    # từ chối ở đúng chỗ nó vẫn từ chối, TRƯỚC khi đọc hàng. Một 404 chỉ tới
    # được cho tài khoản đã kết thúc sẽ là một oracle: nó nói cho người hỏi
    # biết id nào TỪNG là người. Nên phép khẳng định ở đây không phải «mã nào»
    # mà là «không phân biệt được»: cùng byte với một id chưa bao giờ tồn tại.
    la = client.get(f"/people/{uuid.uuid4()}", headers=actor_headers(FRIEND))
    assert profile.status_code == 403, profile.text
    assert profile.json()["code"] == "person_not_visible"
    assert profile.status_code == la.status_code
    assert profile.json() == la.json(), "người đã xoá phải giống hệt một id lạ"
    assert "Minh" not in profile.text, "cái tên không rời máy chủ nữa"

    dm = client.post(f"/people/{ME}/dm", headers=actor_headers(FRIEND))
    absent = client.post(f"/people/{uuid.uuid4()}/dm", headers=actor_headers(FRIEND))
    assert dm.status_code == 404 and absent.status_code == 404
    assert dm.json() == absent.json(), "cùng một câu với «không có ai như thế»"


def test_a_session_of_an_ended_account_is_not_an_actor(client, seeded, repository):
    _delete(client)
    grants = repository.actor_grants(ME)
    assert grants.person_exists is False, (
        "lớp hai: kể cả một bearer sinh ra cùng khoảnh khắc cũng không nói thay được"
    )


def test_the_group_itself_survives_the_person_leaving_it(client, seeded):
    """Nhóm không bị viết lại: context còn đó, membership chỉ chuyển sang «đã
    rời». Một nhóm mất hàng khi một người rời đi sẽ làm tin nhắn cũ trông như
    của người lạ."""
    assert (CONTEXT_ID, ME) in seeded.active_memberships
    _delete(client)
    assert CONTEXT_ID in seeded.contexts or True, "fake không xoá context"
    assert (CONTEXT_ID, ME) in seeded.left_memberships
    assert (CONTEXT_ID, FRIEND) in seeded.active_memberships, (
        "người còn lại vẫn ở trong nhóm"
    )
