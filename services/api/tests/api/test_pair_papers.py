"""The sheet of paper's HTTP surface against the fake (ADR-0027).

What this layer proves: the whole loop, from asking the notebook for a sheet to
keeping a line after the evening; that pressing send is agreeing; that
agreement never travels to a later version; that a sheet both agreed to becomes
one outing inside the same request; that a draft is invisible to the person it
was not sent to; that «đã đi rồi» needs the day to have come and the name of
whoever says so; and one refusal for each of the eleven paper doors.

What it does not prove: the partial unique behind «one open sheet», the
`UNIQUE(paper_id)` behind «one outing per sheet», the immutability trigger on a
sent version, or any race. Those are
tests/postgres/test_pair_papers_postgres.py and
tests/postgres/test_pair_papers_races_postgres.py. This file drives dicts.
"""

from __future__ import annotations

from datetime import timedelta

import pytest

from .pair_helpers import (
    CAP,
    HAN_TUAN,
    NGUOI_KIA,
    T0,
    THU_BAY,
    TOI,
    TUAN,
    TUAN_SAU,
    head,
    lap_so,
    noi_dung,
    seed_pair,
)


@pytest.fixture
def clock(monkeypatch):
    state = {"now": T0}
    monkeypatch.setattr("app.api.service._now", lambda: state["now"])

    def advance(delta):
        state["now"] = state["now"] + delta

    return advance


@pytest.fixture(autouse=True)
def seeded(repository, clock):
    del clock
    seed_pair(repository)


def _draft(client, actor=TOI):
    written = client.post(f"/contexts/{CAP}/papers/draft", headers=head(actor))
    assert written.status_code == 201, written.text
    return written.json()["id"]


def _patch(client, paper_id, *, actor=TOI, ly_do=None, **kwargs):
    body = {"content": noi_dung(**kwargs)}
    if ly_do is not None:
        body["ly_do"] = ly_do
    return client.patch(f"/papers/{paper_id}/draft", json=body, headers=head(actor))


def _send(client, paper_id, version=1, *, actor=TOI):
    return client.post(
        f"/papers/{paper_id}/send", json={"version": version}, headers=head(actor)
    )


def _read(client, paper_id, *, actor=TOI):
    return client.get(f"/papers/{paper_id}", headers=head(actor))


def _agree(client, paper_id, version=1, *, actor=NGUOI_KIA):
    return client.post(
        f"/papers/{paper_id}/versions/{version}/responses",
        json={"kind": "dong_y"},
        headers=head(actor),
    )


def _da_gui(client, actor=TOI):
    """A sheet on its way, which most cases below start from."""
    paper_id = _draft(client, actor)
    assert _patch(client, paper_id, actor=actor).status_code == 200
    assert _send(client, paper_id, actor=actor).status_code == 200
    return paper_id


def test_a_fresh_sheet_arrives_pre_filled_and_unpromised(client):
    lap_so(client)
    paper_id = _draft(client)
    body = _read(client, paper_id).json()
    assert body["state"] == "nhap"
    assert body["version"] == 1
    assert body["author_type"] == "human", (
        "người sẽ bấm gửi là tác giả; `nep` dành cho tờ Nếp tự gửi, lát 1 không có cửa"
    )
    assert body["tuan"] == TUAN
    assert body["expires_at"] == HAN_TUAN
    assert body["outing_id"] is None
    assert body["co_the_ghi_da_di"] is False
    assert body["keeps"] == []
    first = body["versions"][0]
    assert first["content"]["ngay"] == THU_BAY, (
        "ngày đề xuất là Thứ Bảy còn ở phía trước"
    )
    assert first["sent_at"] is None and first["sent_by"] is None
    assert first["my_response"] is None and first["their_agreed"] is False
    assert all(stop["place_id"] is None for stop in first["content"]["chang"]), (
        "bản phác nói «chưa biết», không nói «chắc hợp» (§8)"
    )
    assert all(stop["can_kiem"] for stop in first["content"]["chang"])


def test_a_draft_is_not_a_sent_sheet(client):
    lap_so(client)
    paper_id = _draft(client, TOI)
    theirs = _read(client, paper_id, actor=NGUOI_KIA)
    assert theirs.status_code == 404, theirs.text
    assert theirs.json()["code"] == "paper_not_found", (
        "403 ở đây tự nó là rò rỉ: người kia không được biết bản nháp tồn tại"
    )
    listed = client.get(f"/contexts/{CAP}/papers", headers=head(NGUOI_KIA)).json()
    assert listed["papers"] == []
    mine = client.get(f"/contexts/{CAP}/papers", headers=head(TOI)).json()
    assert [row["state"] for row in mine["papers"]] == ["nhap"]
    assert mine["papers"][0]["ngay"] == THU_BAY


def test_only_the_owner_edits_the_draft(client):
    lap_so(client)
    paper_id = _draft(client, TOI)
    refused = _patch(client, paper_id, actor=NGUOI_KIA)
    assert refused.status_code == 404, refused.text
    assert refused.json()["code"] == "paper_not_found"


def test_sending_is_agreeing(client):
    lap_so(client)
    paper_id = _draft(client)
    _patch(client, paper_id, gio="19:30", viec="Ăn tối, quán mới", ly_do="Thử chỗ mới.")
    sent = _send(client, paper_id)
    assert sent.status_code == 200, sent.text
    assert sent.json() == {
        "id": paper_id,
        "state": "da_gui",
        "version": 1,
        "outing_id": None,
    }, "lệnh trả đúng bốn trường, không mang nội dung"

    mine = _read(client, paper_id).json()["versions"][0]
    assert mine["sent_by"] == str(TOI) and mine["sent_at"] is not None
    assert mine["my_response"] == "dong_y", "bấm gửi là đồng ý phiên bản ấy (§3.4)"
    assert mine["their_agreed"] is False, "im lặng không phải đồng ý"
    assert mine["ly_do"] == "Thử chỗ mới."

    theirs = _read(client, paper_id, actor=NGUOI_KIA).json()["versions"][0]
    assert theirs["my_response"] is None
    assert theirs["their_agreed"] is True, "phía người nhận: người gửi đã đồng ý"


def test_the_view_mark_is_the_senders_to_see(client):
    lap_so(client)
    paper_id = _da_gui(client)
    assert (
        _read(client, paper_id).json()["versions"][0]["viewed_by_recipient_at"] is None
    )

    seen = client.post(f"/papers/{paper_id}/versions/1/viewed", headers=head(NGUOI_KIA))
    assert seen.status_code == 204, seen.text
    assert _read(client, paper_id).json()["state"] == "da_xem"

    sender_view = _read(client, paper_id).json()["versions"][0]
    assert sender_view["viewed_by_recipient_at"] is not None
    reader_view = _read(client, paper_id, actor=NGUOI_KIA).json()["versions"][0]
    assert reader_view["viewed_by_recipient_at"] is None, (
        "người nhận không cần biết mình bị theo dõi đã mở lúc nào (§7.5)"
    )

    again = client.post(
        f"/papers/{paper_id}/versions/1/viewed", headers=head(NGUOI_KIA)
    )
    assert again.status_code == 204, "mốc xem là idempotent"
    assert (
        sender_view["viewed_by_recipient_at"]
        == (_read(client, paper_id).json()["versions"][0]["viewed_by_recipient_at"])
    ), "lần mở đầu tiên là lần được ghi"


def test_the_sender_cannot_mark_their_own_sheet_seen(client):
    lap_so(client)
    paper_id = _da_gui(client)
    refused = client.post(f"/papers/{paper_id}/versions/1/viewed", headers=head(TOI))
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_self_response"


def test_the_sender_cannot_answer_their_own_version(client):
    lap_so(client)
    paper_id = _da_gui(client)
    refused = _agree(client, paper_id, actor=TOI)
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_self_response"


def test_two_yeses_make_one_outing_in_the_same_request(client, repository):
    lap_so(client)
    paper_id = _da_gui(client)
    agreed = _agree(client, paper_id)
    assert agreed.status_code == 200, agreed.text
    body = agreed.json()
    assert body["state"] == "chot"
    assert body["outing_id"] is not None
    outing = repository.get_outing(__import__("uuid").UUID(body["outing_id"]))
    assert outing is not None
    assert outing.context_id == CAP
    assert outing.title == f"Tờ lời rủ {THU_BAY[8:]}/{THU_BAY[5:7]}"
    assert outing.starts_on.isoformat() == THU_BAY == outing.ends_on.isoformat()
    assert outing.headcount == 2
    assert outing.budget_per_person_vnd == 0, "không cột tiền nào nhận số không ai gõ"
    assert outing.created_by_id == NGUOI_KIA, "người xác nhận cuối là người tạo kèo"
    assert _read(client, paper_id).json()["outing_id"] == body["outing_id"]


def test_a_second_yes_is_the_same_yes_and_never_a_second_outing(client, repository):
    """Một lần gửi lại vì mất mạng, và một cú bấm đúp, trông giống hệt nhau ở
    đây. Cả hai đều là MỘT lời đồng ý, nên câu trả lời là câu cũ.

    Bảo người ta «đồng ý thất bại» trong khi nó đã thành công là câu tệ nhất
    trong ba câu có thể nói. Partial unique là cái làm chuyện này an toàn: chỉ
    có đúng một hàng để tìm lại.
    """
    lap_so(client)
    paper_id = _da_gui(client)
    lan_dau = _agree(client, paper_id)
    lan_hai = _agree(client, paper_id)
    assert lan_hai.status_code == 200, lan_hai.text
    assert lan_hai.json() == lan_dau.json(), "phát lại đúng thân cũ"
    assert len(repository.pair_paper_outings) == 1, "không có kèo thứ hai"


def test_a_counter_proposal_is_a_new_version_its_author_has_agreed_to(client):
    lap_so(client)
    paper_id = _da_gui(client)
    revised = client.post(
        f"/papers/{paper_id}/versions/1/responses",
        json={
            "kind": "de_nghi_sua",
            "content": noi_dung(gio="20:00"),
            "ly_do": "Tối muộn hơn được không?",
        },
        headers=head(NGUOI_KIA),
    )
    assert revised.status_code == 200, revised.text
    assert revised.json()["state"] == "da_gui"
    assert revised.json()["version"] == 2

    body = _read(client, paper_id, actor=NGUOI_KIA).json()
    assert [row["version"] for row in body["versions"]] == [1, 2]
    v1, v2 = body["versions"]
    assert v1["my_response"] == "de_nghi_sua"
    assert v2["sent_by"] == str(NGUOI_KIA) and v2["my_response"] == "dong_y"
    assert v2["ly_do"] == "Tối muộn hơn được không?"
    assert v2["content"]["chang"][0]["gio"] == "20:00"

    mine = _read(client, paper_id).json()["versions"]
    assert mine[0]["my_response"] == "dong_y", "cái tôi đồng ý là v1, và nó ở lại v1"
    assert mine[1]["my_response"] is None, "đồng ý không đi theo sang phiên bản sau"
    assert mine[1]["their_agreed"] is True


def test_a_yes_aimed_at_a_version_that_has_been_replaced_is_refused(client):
    """Three versions, so that the stale one was sent by the OTHER person.

    Both refusals are true of «tôi đồng ý v1» after v2 exists -- it is my own
    version and it is stale -- and the door reports staleness because that is
    the one the person can act on. To see it at all the stale version has to be
    one the actor did not send, which is what the third version arranges.
    """
    lap_so(client)
    paper_id = _da_gui(client)
    client.post(
        f"/papers/{paper_id}/versions/1/responses",
        json={"kind": "de_nghi_sua", "content": noi_dung(gio="20:00")},
        headers=head(NGUOI_KIA),
    )
    client.post(
        f"/papers/{paper_id}/versions/2/responses",
        json={"kind": "de_nghi_sua", "content": noi_dung(gio="19:45")},
        headers=head(TOI),
    )
    assert _read(client, paper_id).json()["version"] == 3
    stale = _agree(client, paper_id, 2, actor=NGUOI_KIA)
    assert stale.status_code == 409, stale.text
    assert stale.json()["code"] == "paper_version_stale"
    assert _read(client, paper_id).json()["state"] == "da_gui", "chưa chốt"

    now_agreed = _agree(client, paper_id, 3, actor=NGUOI_KIA)
    assert now_agreed.status_code == 200, now_agreed.text
    assert now_agreed.json()["state"] == "chot"


def test_a_frozen_plan_takes_no_counter_proposal(client):
    lap_so(client)
    paper_id = _da_gui(client)
    _agree(client, paper_id)
    refused = client.post(
        f"/papers/{paper_id}/versions/1/responses",
        json={"kind": "de_nghi_sua", "content": noi_dung(gio="21:00")},
        headers=head(NGUOI_KIA),
    )
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] in ("paper_frozen", "paper_version_stale")


def test_sending_a_version_that_has_moved_on_is_refused(client):
    lap_so(client)
    paper_id = _draft(client)
    _patch(client, paper_id)
    refused = _send(client, paper_id, version=2)
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_version_stale"


def test_a_sent_sheet_is_no_longer_a_draft(client):
    lap_so(client)
    paper_id = _da_gui(client)
    refused = _patch(client, paper_id, gio="21:00")
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_wrong_state"
    refused_again = _send(client, paper_id)
    assert refused_again.status_code == 409, refused_again.text


def test_withdrawal_works_until_they_have_looked(client):
    lap_so(client)
    paper_id = _da_gui(client)
    taken = client.post(
        f"/papers/{paper_id}/withdraw", json={"version": 1}, headers=head(TOI)
    )
    assert taken.status_code == 200, taken.text
    assert taken.json()["state"] == "rut"


def test_withdrawal_stops_the_moment_they_have_looked(client):
    lap_so(client)
    paper_id = _da_gui(client)
    client.post(f"/papers/{paper_id}/versions/1/viewed", headers=head(NGUOI_KIA))
    refused = client.post(
        f"/papers/{paper_id}/withdraw", json={"version": 1}, headers=head(TOI)
    )
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_not_withdrawable", (
        "sau một cái nhìn thì đường ra là một phiên bản mới, không phải một sự biến mất"
    )


def test_only_the_sender_withdraws(client):
    lap_so(client)
    paper_id = _da_gui(client)
    refused = client.post(
        f"/papers/{paper_id}/withdraw", json={"version": 1}, headers=head(NGUOI_KIA)
    )
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_not_withdrawable"


def test_a_week_that_ran_out_never_reads_as_agreed(client, clock):
    lap_so(client)
    paper_id = _da_gui(client)
    clock(TUAN_SAU)
    assert _read(client, paper_id).json()["state"] == "het_han"
    refused = _agree(client, paper_id)
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_expired"
    assert _read(client, paper_id).json()["state"] == "het_han", (
        "im lặng không phải đồng ý: hết hạn không bao giờ thành chốt"
    )


def test_a_plan_that_stands_outlives_the_week(client, clock):
    lap_so(client)
    paper_id = _da_gui(client)
    _agree(client, paper_id)
    clock(TUAN_SAU)
    assert _read(client, paper_id).json()["state"] == "chot", (
        "hạn chỉ xoá được một tờ còn chưa quyết"
    )


def test_either_of_them_may_call_the_week_off(client):
    lap_so(client)
    nhap = _draft(client)
    dropped = client.post(f"/papers/{nhap}/skip", headers=head(TOI))
    assert dropped.json()["state"] == "nghi_tuan", (
        "nháp chưa ai nhận thì bỏ cho tuần này"
    )

    paper_id = _da_gui(client)
    cancelled = client.post(f"/papers/{paper_id}/skip", headers=head(NGUOI_KIA))
    assert cancelled.status_code == 200, cancelled.text
    assert cancelled.json()["state"] == "huy", (
        "tờ đã gửi thì huỷ, vì người kia đã thấy một thứ và đáng được một cái kết"
    )


def test_one_open_sheet_at_a_time(client):
    lap_so(client)
    _draft(client)
    refused = client.post(f"/contexts/{CAP}/papers/draft", headers=head(NGUOI_KIA))
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_wrong_state"


def test_the_very_first_invitation_happens_before_any_notebook(client, repository):
    """§14.1: two friends can pass one sheet without opening a notebook first.

    Asking somebody out is how the notebook gets proposed in the first place, so
    the door that would require a cycle would have nothing to stand on. The
    sheet is temporary -- it belongs to no cycle -- and that is what
    `cycle_active_or_temporary` names.
    """
    paper_id = _draft(client)
    assert _read(client, paper_id).json()["state"] == "nhap"
    stored = repository.pair_papers[__import__("uuid").UUID(paper_id)]
    assert stored["cycle_id"] is None, "tờ tạm: chưa thuộc chu kỳ nào"


def test_a_notebook_half_open_takes_no_sheet(client):
    """One person has asked and the other has not answered: not «temporary»,
    and not open either. A sheet here would be a notebook opened by one."""
    from .pair_helpers import de_nghi

    assert de_nghi(client, "lap_so").status_code == 201
    refused = client.post(f"/contexts/{CAP}/papers/draft", headers=head(TOI))
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "cycle_not_active"


def test_the_day_has_to_have_come_before_anybody_says_they_went(client, clock):
    lap_so(client)
    paper_id = _da_gui(client)
    _agree(client, paper_id)
    assert _read(client, paper_id).json()["co_the_ghi_da_di"] is False
    too_early = client.post(f"/papers/{paper_id}/done", headers=head(TOI))
    assert too_early.status_code == 409, too_early.text
    assert too_early.json()["code"] == "paper_wrong_state"

    clock(timedelta(days=3))
    body = _read(client, paper_id).json()
    assert body["co_the_ghi_da_di"] is True, (
        "máy chủ tính, không phải client: client không đọc đồng hồ (§3.3 luật 6)"
    )
    went = client.post(f"/papers/{paper_id}/done", headers=head(TOI))
    assert went.status_code == 200, went.text
    assert went.json()["state"] == "da_di"


def test_a_line_kept_closes_the_sheet(client, clock, repository):
    lap_so(client)
    paper_id = _da_gui(client)
    _agree(client, paper_id)
    clock(timedelta(days=3))
    client.post(f"/papers/{paper_id}/done", headers=head(TOI))

    kept = client.post(
        f"/papers/{paper_id}/keeps",
        json={"line": "Cái đèn ở góc bàn."},
        headers=head(NGUOI_KIA),
    )
    assert kept.status_code == 201, kept.text
    assert kept.json()["line"] == "Cái đèn ở góc bàn."
    body = _read(client, paper_id).json()
    assert body["state"] == "da_giu"
    assert [row["line"] for row in body["keeps"]] == ["Cái đèn ở góc bàn."]
    assert (
        repository.pair_paper_keeps[__import__("uuid").UUID(paper_id)][0].person_id
        == NGUOI_KIA
    )

    second = client.post(
        f"/papers/{paper_id}/keeps", json={"line": "Và bài hát."}, headers=head(TOI)
    )
    assert second.status_code == 201, "giữ thêm một dòng nữa vẫn được"
    assert len(_read(client, paper_id).json()["keeps"]) == 2


def test_a_line_of_spaces_is_not_a_line(client, clock):
    lap_so(client)
    paper_id = _da_gui(client)
    _agree(client, paper_id)
    clock(timedelta(days=3))
    client.post(f"/papers/{paper_id}/done", headers=head(TOI))
    refused = client.post(
        f"/papers/{paper_id}/keeps", json={"line": "   "}, headers=head(TOI)
    )
    assert refused.status_code == 422, refused.text


def test_keeping_a_line_needs_an_evening_that_happened(client):
    lap_so(client)
    paper_id = _da_gui(client)
    refused = client.post(
        f"/papers/{paper_id}/keeps", json={"line": "Sớm quá."}, headers=head(TOI)
    )
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "paper_wrong_state"


def test_a_command_answer_never_carries_content(client):
    lap_so(client)
    paper_id = _draft(client)
    _patch(client, paper_id)
    for answer in (
        _send(client, paper_id),
        _agree(client, paper_id),
    ):
        assert set(answer.json()) == {"id", "state", "version", "outing_id"}, (
            "một lần gửi lại vì mất mạng phát lại đúng thân này (ADR-0027 §3)"
        )


def test_the_hour_has_to_be_an_hour(client):
    lap_so(client)
    paper_id = _draft(client)
    for bad in ("7 giờ tối", "25:00", "19:60", "19h30"):
        refused = client.patch(
            f"/papers/{paper_id}/draft",
            json={"content": {"ngay": THU_BAY, "chang": [{"gio": bad, "viec": "Ăn"}]}},
            headers=head(TOI),
        )
        assert refused.status_code == 422, f"{bad}: {refused.text}"


def test_three_stops_are_not_a_sheet(client):
    lap_so(client)
    paper_id = _draft(client)
    refused = client.patch(
        f"/papers/{paper_id}/draft",
        json={
            "content": {
                "ngay": THU_BAY,
                "chang": [
                    {"gio": "18:00", "viec": "Ăn"},
                    {"gio": "20:00", "viec": "Đi bộ"},
                    {"gio": "21:30", "viec": "Chè"},
                ],
            }
        },
        headers=head(TOI),
    )
    assert refused.status_code == 422, (
        "ba chặng là để tờ giấy gấp ba quyết định buổi tối đi thế nào (§5.2)"
    )


def test_a_request_cannot_set_what_the_server_decides(client):
    lap_so(client)
    paper_id = _draft(client)
    refused = client.patch(
        f"/papers/{paper_id}/draft",
        json={"content": noi_dung(), "state": "chot", "author_type": "nep"},
        headers=head(TOI),
    )
    assert refused.status_code == 422, refused.text
    sent = client.post(
        f"/papers/{paper_id}/send",
        json={"version": 1, "sent_by": str(NGUOI_KIA)},
        headers=head(TOI),
    )
    assert sent.status_code == 422, sent.text


def test_a_yes_carrying_a_counter_proposal_is_refused(client):
    lap_so(client)
    paper_id = _da_gui(client)
    refused = client.post(
        f"/papers/{paper_id}/versions/1/responses",
        json={"kind": "dong_y", "content": noi_dung(gio="20:00")},
        headers=head(NGUOI_KIA),
    )
    assert refused.status_code == 422, refused.text


def test_a_version_that_does_not_exist_is_not_a_version(client):
    lap_so(client)
    paper_id = _da_gui(client)
    for path in ("viewed", "responses"):
        answer = (
            client.post(
                f"/papers/{paper_id}/versions/9/{path}", headers=head(NGUOI_KIA)
            )
            if path == "viewed"
            else client.post(
                f"/papers/{paper_id}/versions/9/{path}",
                json={"kind": "dong_y"},
                headers=head(NGUOI_KIA),
            )
        )
        assert answer.status_code == 404, f"{path}: {answer.text}"
        assert answer.json()["code"] == "paper_not_found"


def test_a_closed_notebook_refuses_the_whole_sheet(client):
    lap_so(client)
    paper_id = _da_gui(client)
    revision = client.post(
        f"/contexts/{CAP}/notebook/close/preview", headers=head(TOI)
    ).json()["revision"]
    client.post(
        f"/contexts/{CAP}/notebook/close",
        json={"revision": revision},
        headers=head(TOI),
    )
    assert _read(client, paper_id).json()["state"] == "huy"
    for answer in (
        _agree(client, paper_id),
        client.post(f"/papers/{paper_id}/skip", headers=head(TOI)),
        client.post(f"/papers/{paper_id}/done", headers=head(TOI)),
        client.post(
            f"/papers/{paper_id}/keeps", json={"line": "Một dòng."}, headers=head(TOI)
        ),
    ):
        assert answer.status_code == 409, answer.text


def test_only_the_owner_sends_their_own_draft(client):
    lap_so(client)
    paper_id = _draft(client, TOI)
    _patch(client, paper_id)
    refused = _send(client, paper_id, actor=NGUOI_KIA)
    assert refused.status_code == 404, refused.text
    assert refused.json()["code"] == "paper_not_found"


#: Every pair route, with a body where one is required. Keyed by the route's own
#: path so that the sweep below reads the app's table rather than a list
#: somebody maintains: a twentieth route added without a line here fails the
#: sweep, which is the only way a list of doors can notice it is short.
_THAN = {
    ("POST", "/contexts/{context_id}/notebook/proposals"): {"purpose": "lap_so"},
    ("PUT", "/contexts/{context_id}/notebook/constraints/{kind}"): {"content": "Cay"},
    ("POST", "/contexts/{context_id}/notebook/close"): {"revision": "khong-quan-trong"},
    # Valid bodies throughout: pydantic runs before the door, so a malformed
    # one would answer 422 and prove nothing about who may pass.
    ("PATCH", "/papers/{paper_id}/draft"): {"content": noi_dung()},
    ("POST", "/papers/{paper_id}/send"): {"version": 1},
    ("POST", "/papers/{paper_id}/withdraw"): {"version": 1},
    ("POST", "/papers/{paper_id}/versions/{version}/responses"): {"kind": "dong_y"},
    ("POST", "/papers/{paper_id}/keeps"): {"line": "Một dòng."},
}


def test_every_pair_route_refuses_a_stranger(client):
    """The sweep walks the app's own route table, not a list written by hand.

    A door added later is covered the moment it is registered. Eighteen doors
    were checked one at a time while this feature was written, and one of them
    would have been forgotten: a hand-written list does not know it is short.
    """
    from uuid import UUID

    from .pair_helpers import NGUOI_LA

    lap_so(client)
    paper_id = _da_gui(client)
    known = {
        "context_id": str(CAP),
        "paper_id": paper_id,
        "version": "1",
        "purpose": "lap_so",
        "kind": "dung",
        "proposal_id": str(UUID("aaaacccc-0a0a-4a0a-8a0a-0a0a0a0a0acc")),
    }
    swept = 0
    for route in client.app.routes:
        path = getattr(route, "path", "")
        # `in`, not `startswith`: two of the paper doors hang off a context id
        # («/contexts/{id}/papers»), and a filter anchored at the front swept
        # seventeen doors while reporting nothing wrong. The count below is what
        # caught that.
        if "/notebook" not in path and "/papers" not in path:
            continue
        for method in sorted(getattr(route, "methods", set()) - {"HEAD", "OPTIONS"}):
            url = path
            for name, value in known.items():
                url = url.replace("{" + name + "}", value)
            assert "{" not in url, f"chưa có id cho {path}"
            body = _THAN.get((method, path))
            kwargs = {"headers": head(NGUOI_LA)}
            if body is not None:
                kwargs["json"] = body
            answer = client.request(method, url, **kwargs)
            assert answer.status_code == 404, f"{method} {path}: {answer.text}"
            assert answer.json()["code"] in (
                "notebook_not_found",
                "paper_not_found",
            ), f"{method} {path}: {answer.text}"
            swept += 1
    assert swept == 19, f"quét được {swept} cửa, phải là 19"


def test_the_list_carries_the_one_line_a_closed_row_shows(client, clock):
    """Hai mẩu cho dòng phụ của một tờ đã khép, và KHÔNG phải một câu soạn sẵn.

    Màn «Tờ đã khép» hiện giờ-việc của chặng đầu, hoặc dòng đã giữ nếu có. Đọc
    trọn từng tờ để lấy một dòng ấy là một yêu cầu mỗi hàng, trên mỗi nhịp
    poll. Nên danh sách mang sẵn hai mẩu — mẩu, không phải câu: màn tự viết
    «Giữ lại: …» bằng chữ của nó.
    """
    lap_so(client)
    # Sửa TRƯỚC khi gửi: một tờ đã gửi không còn là bản nháp, và bản đầu của ca
    # này sửa sau khi gửi rồi đọc lại đúng nội dung mặc định.
    paper_id = _draft(client)
    assert (
        _patch(client, paper_id, gio="19:30", viec="Ăn tối, quán mới").status_code
        == 200
    )
    assert _send(client, paper_id).status_code == 200
    rows = client.get(f"/contexts/{CAP}/papers", headers=head(TOI)).json()["papers"]
    assert rows[0]["chang_dau"] == {"gio": "19:30", "viec": "Ăn tối, quán mới"}
    assert rows[0]["dong_giu_dau"] is None

    _agree(client, paper_id)
    clock(timedelta(days=3))
    client.post(f"/papers/{paper_id}/done", headers=head(TOI))
    client.post(
        f"/papers/{paper_id}/keeps",
        json={"line": "Cái đèn ở góc bàn."},
        headers=head(NGUOI_KIA),
    )
    rows = client.get(f"/contexts/{CAP}/papers", headers=head(TOI)).json()["papers"]
    assert rows[0]["dong_giu_dau"] == "Cái đèn ở góc bàn."
    assert rows[0]["chang_dau"]["viec"] == "Ăn tối, quán mới", "vẫn còn cả hai mẩu"


def test_a_sheet_the_list_cannot_read_does_not_take_the_list_down(client, repository):
    """Một tờ hỏng không được làm hai mươi tuần khác không hiện ra."""
    import uuid as _uuid

    lap_so(client)
    paper_id = _draft(client)
    repository.pair_paper_versions[(_uuid.UUID(paper_id), 1)]["content"] = {"ngay": "x"}
    rows = client.get(f"/contexts/{CAP}/papers", headers=head(TOI)).json()["papers"]
    assert len(rows) == 1
    assert rows[0]["chang_dau"] is None
    assert rows[0]["ngay"] is None
