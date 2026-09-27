"""`pair_notebook` (ADR-0027 K1, K2, K6; spec «Nếp truyền giấy» §6, §7): consent
belongs to both people and to one cycle, and closing says what it costs."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest

from app.domain import pair_notebook

A = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
B = "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
C = "a3a3a3a3-b3b3-4c3c-8d3d-e3e3e3e3e3e3"
NOW = datetime(2026, 9, 13, 12, 0, tzinfo=UTC)
HAI_NGUOI = (A, B)


def cho_phep(person: str, purpose: str, **over) -> dict:
    return {
        "person_id": person,
        "purpose": purpose,
        "granted_at": NOW - timedelta(hours=1),
        "revoked_at": None,
        "proposal_expires_at": NOW + timedelta(days=3),
        **over,
    }


def test_tu_vung_dong():
    assert pair_notebook.CYCLE_STATES == ("pending", "active", "closed")
    assert pair_notebook.CONSENT_PURPOSES == ("lap_so", "bat_doi", "doc_chat", "chia_gu")
    assert pair_notebook.PER_PERSON_PURPOSES == ("chia_gu",)
    assert pair_notebook.CONSTRAINT_KINDS == ("khong_an_duoc", "dung")


def test_mot_nguoi_dong_y_khong_mo_duoc_gi():
    mot_ben = [cho_phep(A, "lap_so")]
    assert pair_notebook.granted_purposes(mot_ben, HAI_NGUOI, now=NOW) == frozenset()


def test_ca_hai_dong_y_thi_muc_dich_do_mo():
    ca_hai = [cho_phep(A, "lap_so"), cho_phep(B, "lap_so")]
    assert pair_notebook.granted_purposes(ca_hai, HAI_NGUOI, now=NOW) == frozenset(
        {"lap_so"}
    )


def test_bac_duoi_khong_keo_theo_bac_tren():
    """§6.1: lập sổ không tự thành «Một đôi», và không tự cho Nếp đọc chat."""
    lap_so = [cho_phep(A, "lap_so"), cho_phep(B, "lap_so")]
    assert pair_notebook.can_bat_doi(lap_so, HAI_NGUOI, now=NOW) is False
    assert pair_notebook.chat_consent_active(lap_so, HAI_NGUOI, now=NOW) is False


def test_bat_doi_can_dung_hai_nguoi_cua_so_nay():
    lech = [cho_phep(A, "bat_doi"), cho_phep(C, "bat_doi")]
    assert pair_notebook.can_bat_doi(lech, HAI_NGUOI, now=NOW) is False
    dung = [cho_phep(A, "bat_doi"), cho_phep(B, "bat_doi")]
    assert pair_notebook.can_bat_doi(dung, HAI_NGUOI, now=NOW) is True


def test_thu_hoi_mot_ben_la_dong_lai_ngay():
    ca_hai = [cho_phep(A, "doc_chat"), cho_phep(B, "doc_chat")]
    assert pair_notebook.chat_consent_active(ca_hai, HAI_NGUOI, now=NOW) is True
    thu_hoi = [cho_phep(A, "doc_chat", revoked_at=NOW), cho_phep(B, "doc_chat")]
    assert pair_notebook.chat_consent_active(thu_hoi, HAI_NGUOI, now=NOW) is False


def test_chua_bam_dong_y_thi_khong_tinh():
    chua = [cho_phep(A, "lap_so", granted_at=None), cho_phep(B, "lap_so")]
    assert pair_notebook.granted_purposes(chua, HAI_NGUOI, now=NOW) == frozenset()


def test_loi_de_nghi_het_han_khong_con_la_dong_y():
    het = [
        cho_phep(A, "lap_so", proposal_expires_at=NOW - timedelta(seconds=1)),
        cho_phep(B, "lap_so"),
    ]
    assert pair_notebook.granted_purposes(het, HAI_NGUOI, now=NOW) == frozenset()


def test_muc_dich_la_khong_biet_thi_bo_qua():
    la = [cho_phep(A, "doc_het"), cho_phep(B, "doc_het")]
    assert pair_notebook.granted_purposes(la, HAI_NGUOI, now=NOW) == frozenset()


def test_mot_nguoi_thi_khong_co_gi_mo():
    ca_hai = [cho_phep(A, "lap_so"), cho_phep(A, "lap_so")]
    assert pair_notebook.granted_purposes(ca_hai, (A,), now=NOW) == frozenset()


def to(state: str, paper_id: str, **over) -> dict:
    return {
        "id": paper_id,
        "state": state,
        "current_version": 1,
        "expires_at": None,
        **over,
    }


def test_xem_truoc_dong_so_dem_ba_so_phan_khac_nhau():
    """Nháp BỎ, tờ đang chờ HUỶ, buổi đã chốt KHOÁ. Gộp là nói sai (Phase 2)."""
    papers = [
        to("nhap", "p1"),
        to("da_gui", "p2"),
        to("da_xem", "p3"),
        to("chot", "p4"),
        to("da_di", "p5"),
        to("da_giu", "p6"),
        to("het_han", "p7"),
    ]
    ra = pair_notebook.xem_truoc_dong_so(papers, [], now=NOW)
    assert ra["so_nhap_bo"] == 1
    assert ra["so_to_huy"] == 2
    assert ra["so_to_khoa"] == 2
    assert ra["so_de_nghi_huy"] == 0


def test_xem_truoc_dem_to_qua_khung_theo_hieu_luc_khong_theo_cot():
    qua_khung = to("da_gui", "p1", expires_at=NOW - timedelta(seconds=1))
    ra = pair_notebook.xem_truoc_dong_so([qua_khung], [], now=NOW)
    assert ra["so_to_huy"] == 0, "hết khung rồi thì không có gì để huỷ"


def test_xem_truoc_dem_loi_de_nghi_con_cho():
    proposals = [
        {"id": "d1", "completed_at": None, "expires_at": NOW + timedelta(days=1)},
        {"id": "d2", "completed_at": NOW, "expires_at": NOW + timedelta(days=1)},
        {"id": "d3", "completed_at": None, "expires_at": NOW - timedelta(seconds=1)},
    ]
    ra = pair_notebook.xem_truoc_dong_so([], proposals, now=NOW)
    assert ra["so_de_nghi_huy"] == 1


def test_revision_doi_khi_co_gi_do_doi_va_giu_nguyen_khi_khong():
    papers = [to("da_gui", "p1"), to("chot", "p2")]
    mot = pair_notebook.xem_truoc_dong_so(papers, [], now=NOW)
    hai = pair_notebook.xem_truoc_dong_so(list(reversed(papers)), [], now=NOW)
    assert mot["revision"] == hai["revision"], "thứ tự đọc không phải là thay đổi"
    doi = pair_notebook.xem_truoc_dong_so(
        [to("da_xem", "p1"), to("chot", "p2")], [], now=NOW
    )
    assert doi["revision"] != mot["revision"]
    them = pair_notebook.xem_truoc_dong_so(
        papers, [{"id": "d1", "completed_at": None, "expires_at": None}], now=NOW
    )
    assert them["revision"] != mot["revision"], "một lời đề nghị mới cũng là thay đổi"


def test_so_rong_dem_ra_khong():
    ra = pair_notebook.xem_truoc_dong_so([], [], now=NOW)
    assert (
        ra["so_nhap_bo"],
        ra["so_to_huy"],
        ra["so_to_khoa"],
        ra["so_de_nghi_huy"],
    ) == (0, 0, 0, 0)
    assert len(ra["revision"]) == 16


def test_loi_cua_so_mang_ma_wire():
    loi = pair_notebook.NotebookError("notebook_revision_stale")
    assert loi.code == "notebook_revision_stale"
    with pytest.raises(pair_notebook.NotebookError):
        raise loi


# QA 23/09 (docs/claude/2026-09-23/qa-cap-doi-minh-linh.md mục 13): each person
# filed their own «Một đôi» proposal and each agreed only with themselves. Read
# per purpose that was «both»; ADR-0027 says both accept THE SAME proposal.
def test_hai_loi_de_nghi_rieng_khong_thanh_dong_y():
    rieng = [
        cho_phep(A, "bat_doi", proposal_id="PR-A"),
        cho_phep(B, "bat_doi", proposal_id="PR-B"),
    ]
    assert pair_notebook.granted_purposes(rieng, HAI_NGUOI, now=NOW) == frozenset()
    assert pair_notebook.can_bat_doi(rieng, HAI_NGUOI, now=NOW) is False
    # And the same count gates Nếp reading the chat.
    doc = [
        cho_phep(A, "doc_chat", proposal_id="PR-A"),
        cho_phep(B, "doc_chat", proposal_id="PR-B"),
    ]
    assert pair_notebook.chat_consent_active(doc, HAI_NGUOI, now=NOW) is False


def test_ca_hai_tren_cung_de_nghi_thi_dong_y():
    chung = [
        cho_phep(A, "bat_doi", proposal_id="PR-1"),
        cho_phep(B, "bat_doi", proposal_id="PR-1"),
    ]
    assert pair_notebook.granted_purposes(chung, HAI_NGUOI, now=NOW) == {"bat_doi"}


def test_de_nghi_da_hoan_tat_khong_het_han():
    qua_han = NOW - timedelta(days=1)
    xong = [
        cho_phep(p, "bat_doi", proposal_id="PR-1", proposal_expires_at=qua_han, proposal_completed_at=NOW - timedelta(days=8))
        for p in HAI_NGUOI
    ]
    chua_xong = [cho_phep(p, "bat_doi", proposal_id="PR-1", proposal_expires_at=qua_han) for p in HAI_NGUOI]
    assert pair_notebook.granted_purposes(xong, HAI_NGUOI, now=NOW) == {"bat_doi"}
    assert pair_notebook.granted_purposes(chua_xong, HAI_NGUOI, now=NOW) == frozenset()


# ADR-0034 §2.1–2.2: taste in a couple's notebook, per person.

DOI = [cho_phep(A, "bat_doi", proposal_id="p-doi"), cho_phep(B, "bat_doi", proposal_id="p-doi")]
GU = {A: ["cafe", "an-uong"], B: ["cafe", "outdoor", "khong-co-trong-tu-vung"]}


def gu(consents, toi=A, gu_theo_nguoi=GU):
    return pair_notebook.gu_hai_nguoi(consents, HAI_NGUOI, toi, gu_theo_nguoi, now=NOW)


def test_gu_ngoai_mot_doi_la_none():
    ban_be = [cho_phep(A, "chia_gu", proposal_id="pa"), cho_phep(B, "chia_gu", proposal_id="pb")]
    assert gu(ban_be) is None


def test_chua_ai_bat_thi_khong_thay_gu_ai():
    assert gu(DOI) == {"mine_shared": False, "theirs_shared": False, "theirs": [], "common": []}


def test_chi_minh_bat_thi_van_khong_thay_gu_nguoi_kia():
    r = gu(DOI + [cho_phep(A, "chia_gu", proposal_id="pa")])
    assert r == {"mine_shared": True, "theirs_shared": False, "theirs": [], "common": []}


def test_nguoi_kia_bat_thi_thay_gu_ho_nhung_chua_co_gu_chung():
    r = gu(DOI + [cho_phep(B, "chia_gu", proposal_id="pb")])
    assert r["theirs_shared"] is True
    assert r["theirs"] == ["cafe", "outdoor"], "thứ tự từ vựng, bỏ tag không còn trong từ vựng"
    assert r["common"] == [], "gu chung cần cả hai bật"


def test_ca_hai_bat_thi_co_gu_chung():
    r = gu(DOI + [cho_phep(A, "chia_gu", proposal_id="pa"), cho_phep(B, "chia_gu", proposal_id="pb")])
    assert r["common"] == ["cafe"]


def test_thu_hoi_chia_gu_la_thoi_ngay():
    thu_hoi = cho_phep(B, "chia_gu", proposal_id="pb", revoked_at=NOW - timedelta(minutes=1))
    assert gu(DOI + [cho_phep(A, "chia_gu", proposal_id="pa"), thu_hoi])["theirs"] == []


def test_chia_gu_khong_bao_gio_la_dong_y_cua_ca_hai():
    ca_hai = [cho_phep(A, "chia_gu", proposal_id="pa"), cho_phep(B, "chia_gu", proposal_id="pb")]
    assert "chia_gu" not in pair_notebook.granted_purposes(ca_hai, HAI_NGUOI, now=NOW)


# ADR-0034 §2.3–2.4: «Người lo», from what the two did in this cycle only.


def _to(cycle="CY", sent_by=None, author="human", responses=()):
    return {
        "cycle_id": cycle,
        "versions": [{"version": 1, "author_type": author, "sent_by": sent_by}],
        "responses": [{"person_id": p, "kind": k} for p, k in responses],
    }


def test_chua_co_gi_thi_nguoi_lap_so_lo():
    r = pair_notebook.nguoi_lo_suy([A, B], [], cycle_id="CY", nguoi_lap_so=B)
    assert r == {"nguoi_lo": [B], "diem": [[A, 0], [B, 0]]}


def test_ai_hay_gui_truoc_thi_lo():
    to = [_to(sent_by=A), _to(sent_by=A), _to(sent_by=B, responses=[(A, "de_nghi_sua")])]
    r = pair_notebook.nguoi_lo_suy([A, B], to, cycle_id="CY", nguoi_lap_so=B)
    assert r["nguoi_lo"] == [A]
    assert r["diem"] == [[A, 5], [B, 2]]


def test_chu_ky_khac_va_to_cua_nep_khong_tinh():
    to = [_to(cycle="CU", sent_by=A), _to(sent_by=None, author="nep"), _to(author="nep", sent_by=A)]
    assert pair_notebook.nguoi_lo_suy([A, B], to, cycle_id="CY", nguoi_lap_so=B)["nguoi_lo"] == [B]


def test_hoa_ma_khong_co_nguoi_lap_so_thi_nguoi_dau():
    assert pair_notebook.nguoi_lo_suy([A, B], [_to(sent_by=A), _to(sent_by=B)], cycle_id="CY", nguoi_lap_so=None)["nguoi_lo"] == [A]


def test_tuan_da_chon_thang_suy_luan():
    suy = {"nguoi_lo": [A], "diem": [[A, 2], [B, 0]]}
    assert pair_notebook.vai_tuan(suy, None, [A, B])["cach"] == "suy"
    assert pair_notebook.vai_tuan(suy, {"nguoi_lo_id": B}, [A, B]) == {"nguoi_lo": [B], "cach": "chon", "diem": [[A, 2], [B, 0]]}
    assert pair_notebook.vai_tuan(suy, {"nguoi_lo_id": None}, [A, B])["nguoi_lo"] == [A, B], "hôm nay mình share"


def _gui(tuan, ai, luc, cycle="CY"):
    return {"cycle_id": cycle, "tuan": tuan, "versions": [{"version": 1, "author_type": "human", "sent_by": ai, "sent_at": luc}], "responses": []}


def test_nguoi_mo_loi_la_nguoi_gui_to_dau_tien_cua_tuan():
    to = [_gui("2026-09-14", B, NOW - timedelta(days=5)), _gui("2026-09-14", A, NOW - timedelta(days=6)), _gui("2026-09-07", B, NOW - timedelta(days=12))]
    assert pair_notebook.nguoi_mo_loi(to, cycle_id="CY", tuan="2026-09-14") == A
    assert pair_notebook.nguoi_mo_loi(to, cycle_id="CY", tuan="2026-08-31") is None
    assert pair_notebook.nguoi_mo_loi(to, cycle_id="CU", tuan="2026-09-14") is None


def test_gay_sang_nguoi_kia_khi_nguoi_lo_da_mo_loi_hai_tuan_lien():
    suy = {"nguoi_lo": [A], "diem": [[A, 4], [B, 0]]}
    assert pair_notebook.vai_tuan(suy, None, [A, B], mo_loi_truoc=[A, A]) == {"nguoi_lo": [B], "cach": "luot", "diem": suy["diem"]}
    assert pair_notebook.vai_tuan(suy, None, [A, B], mo_loi_truoc=[A, B])["cach"] == "suy"
    assert pair_notebook.vai_tuan(suy, None, [A, B], mo_loi_truoc=[A, None])["cach"] == "suy"
    assert pair_notebook.vai_tuan(suy, {"nguoi_lo_id": A}, [A, B], mo_loi_truoc=[A, A])["cach"] == "chon", "đã chọn thì thắng gậy"
