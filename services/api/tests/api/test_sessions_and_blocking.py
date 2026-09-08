"""Phiên, chặn và báo cáo trên fake (L5, ADR-0023 §2.3–2.5).

Tầng này chứng minh orchestration: ai được hỏi gì, câu từ chối nào giống câu
nào, và cái gì KHÔNG rời máy chủ. Nó không chứng minh SQL — `_readable_by`
với nhánh chặn và bản đồ xoá tài khoản có ca riêng ở tests/postgres.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

import pytest

from app.api.repository import AccountSessionRecord, PersonRecord

from .helpers import ADVANCER_ID, OTHER_ID, SENDER_ID, actor_headers

ME = ADVANCER_ID
OTHER = SENDER_ID
THIRD = OTHER_ID
NOW = datetime(2030, 8, 27, 12, tzinfo=UTC)


def _seed(repository):
    for pid, name in ((ME, "Tôi"), (OTHER, "Người kia"), (THIRD, "Người thứ ba")):
        repository.people[pid] = PersonRecord(id=pid, display_name=name, created_at=NOW)


def _session(repository, person_id, *, issued_via="otp", offset=0, revoked=False):
    record = AccountSessionRecord(
        id=uuid.uuid4(),
        person_id=person_id,
        issued_from_invite_id=None,
        issued_via=issued_via,
        created_at=NOW + timedelta(minutes=offset),
        expires_at=NOW + timedelta(days=30),
        revoked_at=NOW if revoked else None,
    )
    repository.account_sessions[record.id] = record
    return record


def test_the_session_list_is_only_ones_own_and_hides_dead_rows(client, repository):
    _seed(repository)
    live = _session(repository, ME, offset=1)
    older = _session(repository, ME, issued_via="google", offset=0)
    _session(repository, ME, offset=2, revoked=True)
    _session(repository, OTHER)

    listed = client.get("/sessions", headers=actor_headers(ME))
    assert listed.status_code == 200, listed.text
    rows = listed.json()["sessions"]
    assert [row["id"] for row in rows] == [str(live.id), str(older.id)], (
        "chỉ phiên còn sống của chính mình, mới nhất trước"
    )
    assert [row["issued_via"] for row in rows] == ["otp", "google"]
    assert all(row["current"] is False for row in rows), (
        "chế độ dev không có bearer nên không phiên nào là «phiên này»"
    )
    assert not any("device" in row or "ip" in row for row in rows), (
        "bảng không lưu nhãn thiết bị nên wire không được bịa ra"
    )


def test_revoking_somebody_elses_session_is_the_same_404_as_a_made_up_id(
    client, repository
):
    _seed(repository)
    theirs = _session(repository, OTHER)
    mine = _session(repository, ME)

    refused = client.delete(f"/sessions/{theirs.id}", headers=actor_headers(ME))
    absent = client.delete(f"/sessions/{uuid.uuid4()}", headers=actor_headers(ME))
    assert refused.status_code == 404 and absent.status_code == 404
    assert refused.json() == absent.json(), "403 sẽ xác nhận id ấy là một phiên thật"
    assert repository.account_sessions[theirs.id].revoked_at is None

    gone = client.delete(f"/sessions/{mine.id}", headers=actor_headers(ME))
    assert gone.status_code == 204, gone.text
    assert repository.account_sessions[mine.id].revoked_at is not None
    assert client.get("/sessions", headers=actor_headers(ME)).json()["sessions"] == []


def test_blocking_is_idempotent_and_only_the_blocker_lifts_it(client, repository):
    _seed(repository)
    first = client.post(f"/people/{OTHER}/block", headers=actor_headers(ME))
    assert first.status_code == 200, first.text
    assert first.json() == {"person_id": str(OTHER), "state": "blocked"}
    again = client.post(f"/people/{OTHER}/block", headers=actor_headers(ME))
    assert again.status_code == 200 and again.json() == first.json(), (
        "chặn lần hai là cùng bức tường, không phải lỗi"
    )

    listed = client.get("/people/me/blocked", headers=actor_headers(ME)).json()
    assert [row["person_id"] for row in listed["blocked"]] == [str(OTHER)]
    assert listed["blocked"][0]["display_name"] == "Người kia"
    theirs = client.get("/people/me/blocked", headers=actor_headers(OTHER)).json()
    assert theirs["blocked"] == [], "người bị chặn không được biết mình bị chặn"

    wrong_way = client.delete(f"/people/{ME}/block", headers=actor_headers(OTHER))
    assert wrong_way.status_code == 403, wrong_way.text

    lifted = client.delete(f"/people/{OTHER}/block", headers=actor_headers(ME))
    assert lifted.status_code == 200
    assert lifted.json() == {"person_id": str(OTHER), "state": "declined"}, (
        "gỡ chặn không nối lại tình bạn"
    )
    assert (
        client.get("/people/me/blocked", headers=actor_headers(ME)).json()["blocked"]
        == []
    )


def test_blocking_yourself_and_a_stranger_are_refused_differently(client, repository):
    _seed(repository)
    myself = client.post(f"/people/{ME}/block", headers=actor_headers(ME))
    assert myself.status_code == 403, myself.text
    nobody = client.post(f"/people/{uuid.uuid4()}/block", headers=actor_headers(ME))
    assert nobody.status_code == 404


def test_a_block_hides_the_wall_both_ways_but_leaves_the_shared_group(
    client, repository
):
    from .helpers import CONTEXT_ID, join_group
    from .test_posts_audience import post_body

    _seed(repository)
    join_group(repository, ME, OTHER)
    for audience, body in (("public", "công khai"), ("group", "trong nhóm")):
        written = client.post(
            "/posts",
            json=post_body(
                audience, context_id=CONTEXT_ID if audience == "group" else None
            )
            | {"body": body},
            headers=actor_headers(ME),
        )
        assert written.status_code == 201, written.text

    before = client.get("/posts", headers=actor_headers(OTHER)).json()["posts"]
    assert {row["body"] for row in before} == {"công khai", "trong nhóm"}

    client.post(f"/people/{OTHER}/block", headers=actor_headers(ME))
    after = client.get("/posts", headers=actor_headers(OTHER)).json()["posts"]
    assert {row["body"] for row in after} == {"trong nhóm"}, (
        "bài công khai ẩn đi; bài của nhóm chung ở lại vì nhóm là của nhóm"
    )
    mine = client.get("/posts", headers=actor_headers(ME)).json()["posts"]
    assert len(mine) == 2, "tác giả vẫn đọc được bài của mình"


def test_a_report_keeps_the_note_off_the_answer(client, repository):
    _seed(repository)
    filed = client.post(
        "/reports",
        json={
            "target_type": "post",
            "target_id": str(uuid.uuid4()),
            "reason": "harassment",
            "note": "  câu chữ của người gửi  ",
        },
        headers=actor_headers(ME),
    )
    assert filed.status_code == 201, filed.text
    assert set(filed.json()) == {"id", "created_at"}, "không echo lại ghi chú"
    stored = repository.reports_filed[-1]
    assert stored["reporter_id"] == ME
    assert stored["reason"] == "harassment"
    assert stored["note"] == "câu chữ của người gửi", "ghi chú được cắt khoảng trắng"


@pytest.mark.parametrize(
    ("field", "value"),
    [("target_type", "trip"), ("reason", "vi phạm"), ("note", "x" * 501)],
)
def test_a_report_outside_the_vocabulary_is_refused_at_the_wire(
    client, repository, field, value
):
    _seed(repository)
    body = {
        "target_type": "post",
        "target_id": str(uuid.uuid4()),
        "reason": "spam",
        field: value,
    }
    refused = client.post("/reports", json=body, headers=actor_headers(ME))
    assert refused.status_code == 422, refused.text
    assert not repository.reports_filed
