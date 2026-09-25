"""The two-person notebook's HTTP surface against the fake (ADR-0027).

What this layer proves: one yes is never enough; a purpose above the first is
refused until the notebook is open; an offer that lapsed cannot be accepted; a
constraint belongs to whoever wrote it; the close preview counts three
different fates and the close refuses a revision that has gone stale; and
every request from outside the pair -- a stranger, a group id, an id that is
nobody -- answers the same 404 with the same sentence.

What it does not prove: any of the five things the schema decides. The
composite foreign key, the partial unique on open cycles, the primary key that
allows one couple per person and the deferred trigger behind it are proved in
tests/postgres/test_pair_notebook_postgres.py. This file drives dicts.
"""

from __future__ import annotations

import uuid

import pytest

from .pair_helpers import (
    CAP,
    HOI,
    NGUOI_KIA,
    NGUOI_LA,
    T0,
    TOI,
    TUAN_SAU,
    de_nghi,
    dong_thuan,
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


def _so(client, actor=TOI, context=CAP):
    return client.get(f"/contexts/{context}/notebook", headers=head(actor))


def test_a_fresh_pair_has_a_notebook_nobody_has_opened(client):
    read = _so(client)
    assert read.status_code == 200, read.text
    body = read.json()
    assert body["cycle_state"] is None
    assert body["open_paper_id"] is None
    assert body["nep_gui_ho"] is False, "lát 1 không có cửa bật cờ này"
    assert body["my_consents"] == [
        {"purpose": "lap_so", "granted": False},
        {"purpose": "bat_doi", "granted": False},
        {"purpose": "doc_chat", "granted": False},
        {"purpose": "chia_gu", "granted": False},
    ]
    assert body["their_consents_granted"] == {
        "lap_so": False,
        "bat_doi": False,
        "doc_chat": False,
        "chia_gu": False,
    }
    assert body["pending_proposals"] == []
    assert body["taste"] is None, "ngoài «Một đôi» không đọc gu ai"


def test_one_yes_opens_nothing_and_the_other_side_can_see_whose_turn_it_is(client):
    offered = de_nghi(client, "lap_so")
    assert offered.status_code == 201, offered.text
    assert offered.json()["my_granted"] is True, "hỏi là đã đồng ý"
    assert offered.json()["proposed_by_id"] == str(TOI)

    mine = _so(client, TOI).json()
    assert mine["cycle_state"] == "pending", "một người đồng ý thì sổ chưa mở"
    assert mine["my_consents"][0] == {"purpose": "lap_so", "granted": True}
    assert mine["their_consents_granted"]["lap_so"] is False

    theirs = _so(client, NGUOI_KIA).json()
    assert theirs["my_consents"][0] == {"purpose": "lap_so", "granted": False}
    assert theirs["their_consents_granted"]["lap_so"] is True
    assert len(theirs["pending_proposals"]) == 1
    assert theirs["pending_proposals"][0]["my_granted"] is False


def test_the_second_yes_opens_the_notebook(client):
    lap_so(client)
    body = _so(client).json()
    assert body["cycle_state"] == "active"
    assert sorted(body["participants"]) == sorted([str(TOI), str(NGUOI_KIA)])
    assert body["pending_proposals"] == [], "đề nghị đã xong thì không còn chờ"


def test_the_asker_cannot_answer_their_own_offer(client):
    offered = de_nghi(client, "lap_so")
    proposal_id = offered.json()["id"]
    refused = client.post(
        f"/contexts/{CAP}/notebook/proposals/{proposal_id}/grant", headers=head(TOI)
    )
    assert refused.status_code == 403, refused.text
    assert refused.json()["detail"] == "is_invitee"
    assert _so(client).json()["cycle_state"] == "pending"


def test_an_offer_nobody_answered_in_time_stops_being_an_offer(client, clock):
    offered = de_nghi(client, "lap_so")
    proposal_id = offered.json()["id"]
    clock(TUAN_SAU)
    refused = client.post(
        f"/contexts/{CAP}/notebook/proposals/{proposal_id}/grant",
        headers=head(NGUOI_KIA),
    )
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "consent_proposal_expired"
    assert _so(client).json()["pending_proposals"] == []


def test_a_higher_rung_is_refused_until_the_notebook_is_open(client):
    refused = de_nghi(client, "bat_doi")
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "consent_missing"


def test_tier_two_does_not_carry_tier_three(client):
    lap_so(client)
    body = _so(client).json()
    assert body["my_consents"][1] == {"purpose": "bat_doi", "granted": False}
    assert body["their_consents_granted"]["bat_doi"] is False


def test_a_couple_needs_both_and_one_person_belongs_to_one(client, repository):
    lap_so(client)
    offered = de_nghi(client, "bat_doi")
    assert repository.active_couple_members == {}, "một người đồng ý thì chưa là đôi"
    client.post(
        f"/contexts/{CAP}/notebook/proposals/{offered.json()['id']}/grant",
        headers=head(NGUOI_KIA),
    )
    assert set(repository.active_couple_members) == {TOI, NGUOI_KIA}
    assert _so(client).json()["their_consents_granted"]["bat_doi"] is True


def test_taking_back_one_half_of_a_couple_ends_it_for_both(client, repository):
    lap_so(client)
    dong_thuan(client, "bat_doi")
    gone = client.delete(
        f"/contexts/{CAP}/notebook/consents/bat_doi", headers=head(NGUOI_KIA)
    )
    assert gone.status_code == 204, gone.text
    assert repository.active_couple_members == {}, (
        "«một đôi» là chuyện của hai người; một người rút thì cả hai hàng phải đi, "
        "không thì người kia bị khoá khỏi mọi sổ khác"
    )
    assert _so(client).json()["their_consents_granted"]["bat_doi"] is False


def test_one_cannot_revoke_the_other_persons_grant(client):
    lap_so(client)
    mine = client.delete(f"/contexts/{CAP}/notebook/consents/lap_so", headers=head(TOI))
    assert mine.status_code == 204, mine.text
    # `is_self` is the row's own person, so this only ever reached my grant.
    assert _so(client, NGUOI_KIA).json()["my_consents"][0]["granted"] is True


def test_an_unknown_purpose_is_not_a_door(client):
    refused = client.delete(
        f"/contexts/{CAP}/notebook/consents/doc_het", headers=head(TOI)
    )
    assert refused.status_code == 404, refused.text
    assert refused.json()["code"] == "consent_purpose_unknown"


def test_both_may_read_the_two_lines_and_only_their_owner_writes_one(client):
    lap_so(client)
    written = client.put(
        f"/contexts/{CAP}/notebook/constraints/khong_an_duoc",
        json={"content": "Hải sản"},
        headers=head(TOI),
    )
    assert written.status_code == 200, written.text
    assert written.json() == {
        "owner_id": str(TOI),
        "kind": "khong_an_duoc",
        "content": "Hải sản",
        "version": 1,
    }
    again = client.put(
        f"/contexts/{CAP}/notebook/constraints/khong_an_duoc",
        json={"content": "Hải sản, và cay"},
        headers=head(TOI),
    )
    assert again.json()["version"] == 2, "sửa là phiên bản mới, không phải hàng mới"

    theirs = _so(client, NGUOI_KIA).json()["constraints"]
    assert theirs == [
        {
            "owner_id": str(TOI),
            "kind": "khong_an_duoc",
            "content": "Hải sản, và cay",
            "version": 2,
        }
    ], "vùng chung: người kia đọc được"

    # And writing one is always about oneself: the route carries no owner, so
    # there is no field through which one could write the other person's line.
    theirs_own = client.put(
        f"/contexts/{CAP}/notebook/constraints/dung",
        json={"content": "Đừng rủ sau 21:00."},
        headers=head(NGUOI_KIA),
    )
    assert theirs_own.json()["owner_id"] == str(NGUOI_KIA)
    gone = client.delete(
        f"/contexts/{CAP}/notebook/constraints/dung", headers=head(NGUOI_KIA)
    )
    assert gone.status_code == 204
    assert len(_so(client).json()["constraints"]) == 1


def test_an_unknown_constraint_kind_is_not_a_door(client):
    lap_so(client)
    refused = client.put(
        f"/contexts/{CAP}/notebook/constraints/thich",
        json={"content": "Cà phê"},
        headers=head(TOI),
    )
    assert refused.status_code == 404, refused.text
    assert refused.json()["code"] == "constraint_kind_unknown"


def test_a_blank_constraint_is_not_a_constraint(client):
    lap_so(client)
    refused = client.put(
        f"/contexts/{CAP}/notebook/constraints/dung",
        json={"content": "   "},
        headers=head(TOI),
    )
    assert refused.status_code == 422, refused.text


def _xem_truoc(client, actor=TOI):
    return client.post(f"/contexts/{CAP}/notebook/close/preview", headers=head(actor))


def test_the_preview_counts_three_different_fates(client):
    lap_so(client)
    # A draft only I have seen.
    nhap = client.post(f"/contexts/{CAP}/papers/draft", headers=head(TOI)).json()
    client.post(f"/papers/{nhap['id']}/skip", headers=head(TOI))
    # A sheet waiting for an answer.
    cho = client.post(f"/contexts/{CAP}/papers/draft", headers=head(TOI)).json()
    client.patch(
        f"/papers/{cho['id']}/draft", json={"content": noi_dung()}, headers=head(TOI)
    )
    client.post(f"/papers/{cho['id']}/send", json={"version": 1}, headers=head(TOI))

    body = _xem_truoc(client).json()
    assert body["so_nhap_bo"] == 0, "tờ nháp kia đã nghỉ tuần, không còn mở"
    assert body["so_to_huy"] == 1
    assert body["so_to_khoa"] == 0
    assert body["so_de_nghi_huy"] == 0
    assert len(body["revision"]) == 16

    # An offer still waiting is the fourth number, and it is counted apart.
    de_nghi(client, "doc_chat")
    assert _xem_truoc(client).json()["so_de_nghi_huy"] == 1


def test_closing_refuses_a_revision_that_has_gone_stale(client):
    lap_so(client)
    seen = _xem_truoc(client).json()["revision"]
    client.post(f"/contexts/{CAP}/papers/draft", headers=head(TOI))
    refused = client.post(
        f"/contexts/{CAP}/notebook/close",
        json={"revision": seen},
        headers=head(TOI),
    )
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "notebook_revision_stale"
    assert _so(client).json()["cycle_state"] == "active", "từ chối thì không đóng gì"


def test_closing_is_closing(client, repository):
    lap_so(client)
    dong_thuan(client, "bat_doi")
    paper = client.post(f"/contexts/{CAP}/papers/draft", headers=head(TOI)).json()
    de_nghi(client, "doc_chat")

    body = _xem_truoc(client).json()
    assert (body["so_nhap_bo"], body["so_de_nghi_huy"]) == (1, 1)
    closed = client.post(
        f"/contexts/{CAP}/notebook/close",
        json={"revision": body["revision"]},
        headers=head(TOI),
    )
    assert closed.status_code == 204, closed.text

    after = _so(client).json()
    assert after["cycle_state"] is None, "chu kỳ đã đóng; mở lại là một thoả thuận mới"
    assert after["my_consents"][0]["granted"] is False
    assert after["pending_proposals"] == []
    assert repository.active_couple_members == {}
    dropped = client.get(f"/papers/{paper['id']}", headers=head(TOI))
    assert dropped.json()["state"] == "bo", "nháp đang chờ thì bỏ, không khoá"


def test_a_closed_notebook_can_be_opened_again_as_a_new_agreement(client):
    lap_so(client)
    revision = _xem_truoc(client).json()["revision"]
    client.post(
        f"/contexts/{CAP}/notebook/close",
        json={"revision": revision},
        headers=head(TOI),
    )
    lap_so(client)
    assert _so(client).json()["cycle_state"] == "active"


def test_everybody_outside_the_pair_gets_the_same_four_oh_four(client):
    lap_so(client)
    khong_ai = uuid.UUID("ff66ff66-0f0f-4f0f-8f0f-0f0f0f0f0f66")
    for label, context, actor in (
        ("người lạ", CAP, NGUOI_LA),
        ("hội bạn, không phải cặp", HOI, TOI),
        ("không có context nào", khong_ai, TOI),
    ):
        read = _so(client, actor, context)
        assert read.status_code == 404, f"{label}: {read.text}"
        assert read.json()["code"] == "notebook_not_found", label
        assert read.json()["detail"] == "Không có sổ này.", (
            f"{label}: ba sự thật khác nhau phải ra đúng một câu"
        )
        for path, method, payload in (
            ("notebook/proposals", "post", {"purpose": "lap_so"}),
            ("notebook/close/preview", "post", None),
            ("papers", "get", None),
            ("papers/draft", "post", None),
        ):
            url = f"/contexts/{context}/{path}"
            if method == "get":
                answer = client.get(url, headers=head(actor))
            elif payload is None:
                answer = client.post(url, headers=head(actor))
            else:
                answer = client.post(url, json=payload, headers=head(actor))
            assert answer.status_code == 404, f"{label} {path}: {answer.text}"
            assert answer.json()["code"] == "notebook_not_found"


# QA 23/09 (docs/claude/2026-09-23/qa-cap-doi-minh-linh.md mục 13): the other
# person pressed «Một đôi» instead of answering, which filed a SECOND proposal;
# both phones then read «Đang là một đôi» while nothing was completed.
def test_the_other_offering_the_same_rung_is_told_to_answer_it(client, repository):
    lap_so(client)
    mine = de_nghi(client, "bat_doi")
    assert mine.status_code == 201, mine.text
    theirs = de_nghi(client, "bat_doi", actor=NGUOI_KIA)
    assert theirs.status_code == 409, theirs.text
    assert theirs.json()["code"] == "consent_proposal_pending"
    assert repository.active_couple_members == {}
    view = _so(client).json()
    assert view["granted_purposes"] == ["lap_so"]
    assert [p["purpose"] for p in view["pending_proposals"]] == ["bat_doi"]


def test_asking_twice_returns_the_offer_already_standing(client):
    lap_so(client)
    first = de_nghi(client, "bat_doi")
    again = de_nghi(client, "bat_doi")
    assert again.status_code == 201, again.text
    assert again.json()["id"] == first.json()["id"]
    assert len(_so(client).json()["pending_proposals"]) == 1


def test_granted_purposes_names_what_both_agreed_to(client):
    assert _so(client).json()["granted_purposes"] == []
    lap_so(client)
    dong_thuan(client, "bat_doi")
    assert _so(client).json()["granted_purposes"] == ["lap_so", "bat_doi"]


def test_an_offer_whose_proposer_took_their_yes_back_is_dead(client, repository):
    # Each person answers each proposal once; after taking one's own yes back,
    # asking again must file a new offer or the notebook could never open.
    first = de_nghi(client, "lap_so").json()["id"]
    assert client.delete(f"/contexts/{CAP}/notebook/consents/lap_so", headers=head(TOI)).status_code == 204
    again = de_nghi(client, "lap_so")
    assert again.status_code == 201, again.text
    assert again.json()["id"] != first
    theirs = de_nghi(client, "lap_so", actor=NGUOI_KIA)
    assert theirs.status_code == 409, theirs.text


# ADR-0034 §2.1–2.2: `chia_gu`, one person's own switch inside «Một đôi».


def _gu(repository):
    repository.person_interests[TOI] = {"cafe", "an-uong"}
    repository.person_interests[NGUOI_KIA] = {"cafe", "outdoor"}


def test_chia_gu_needs_a_couple(client, repository):
    _gu(repository)
    lap_so(client)
    refused = de_nghi(client, "chia_gu")
    assert refused.status_code == 409, refused.text
    assert refused.json()["code"] == "consent_missing"
    assert _so(client).json()["taste"] is None


def test_chia_gu_is_decided_alone_and_shows_only_the_sharers_taste(client, repository):
    _gu(repository)
    lap_so(client)
    dong_thuan(client, "bat_doi")
    assert _so(client).json()["taste"] == {"mine_shared": False, "theirs_shared": False, "theirs": [], "common": []}
    mine = de_nghi(client, "chia_gu")
    assert mine.status_code == 201, mine.text
    body = _so(client).json()
    assert body["pending_proposals"] == [], "không ai phải «đồng ý» gu của người khác"
    assert body["taste"] == {"mine_shared": True, "theirs_shared": False, "theirs": [], "common": []}
    their_view = _so(client, actor=NGUOI_KIA).json()["taste"]
    assert their_view == {"mine_shared": False, "theirs_shared": True, "theirs": ["an-uong", "cafe"], "common": []}
    assert de_nghi(client, "chia_gu").json()["id"] == mine.json()["id"], "bật lại khi đang bật trả về đúng cái cũ"
    de_nghi(client, "chia_gu", actor=NGUOI_KIA)
    assert _so(client).json()["taste"]["common"] == ["cafe"]


def test_the_other_cannot_grant_somebody_elses_chia_gu(client, repository):
    lap_so(client)
    dong_thuan(client, "bat_doi")
    mine = de_nghi(client, "chia_gu")
    answer = client.post(
        f"/contexts/{CAP}/notebook/proposals/{mine.json()['id']}/grant", headers=head(NGUOI_KIA)
    )
    assert answer.status_code in (403, 409), answer.text


def test_taking_chia_gu_back_hides_the_taste_at_the_next_read(client, repository):
    _gu(repository)
    lap_so(client)
    dong_thuan(client, "bat_doi")
    de_nghi(client, "chia_gu", actor=NGUOI_KIA)
    assert _so(client).json()["taste"]["theirs"] == ["cafe", "outdoor"]
    gone = client.delete(f"/contexts/{CAP}/notebook/consents/chia_gu", headers=head(NGUOI_KIA))
    assert gone.status_code == 204, gone.text
    assert _so(client).json()["taste"]["theirs"] == []


# ADR-0034 §2.4: «Người lo» of the week, inferred or chosen; no gender anywhere.


def test_week_role_only_in_a_couple(client):
    lap_so(client)
    assert _so(client).json()["week_role"] is None
    refused = client.put(f"/contexts/{CAP}/notebook/week-role", json={"lo": "toi"}, headers=head(TOI))
    assert refused.status_code == 409 and refused.json()["code"] == "consent_missing"


def test_week_role_defaults_to_whoever_opened_the_notebook_then_can_be_chosen(client):
    lap_so(client)  # TOI offered lap_so, so TOI opened the notebook
    dong_thuan(client, "bat_doi")
    role = _so(client).json()["week_role"]
    assert role["cach"] == "suy"
    assert role["nguoi_lo"] == [str(TOI)]
    assert [d["score"] for d in role["diem"]] == [0, 0]
    chosen = client.put(f"/contexts/{CAP}/notebook/week-role", json={"lo": "nguoi_kia"}, headers=head(TOI))
    assert chosen.status_code == 200, chosen.text
    assert chosen.json()["nguoi_lo"] == [str(NGUOI_KIA)] and chosen.json()["cach"] == "chon"
    seen = _so(client, actor=NGUOI_KIA).json()["week_role"]
    assert seen["nguoi_lo"] == [str(NGUOI_KIA)], "cả hai thấy cùng một lựa chọn"
    share = client.put(f"/contexts/{CAP}/notebook/week-role", json={"lo": "ca_hai"}, headers=head(NGUOI_KIA))
    assert set(share.json()["nguoi_lo"]) == {str(TOI), str(NGUOI_KIA)}


def test_week_role_answers_a_bad_choice_with_422(client):
    lap_so(client)
    dong_thuan(client, "bat_doi")
    bad = client.put(f"/contexts/{CAP}/notebook/week-role", json={"lo": "nam"}, headers=head(TOI))
    assert bad.status_code == 422
