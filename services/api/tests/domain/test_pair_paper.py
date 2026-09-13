"""`pair_paper` (ADR-0027, spec «Nếp truyền giấy» §3): the thirteen states and
the three rules a service is not allowed to decide for itself."""

from __future__ import annotations

from datetime import UTC, date, datetime, timedelta

import pytest

from app.domain import pair_paper

A = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
B = "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
NOW = datetime(2026, 9, 13, 12, 0, tzinfo=UTC)
KHUNG = NOW + timedelta(days=2)


def to(state: str, **over) -> dict:
    return {
        "id": "paper-1",
        "state": state,
        "current_version": 1,
        "expires_at": KHUNG,
        **over,
    }


def phien_ban(version: int = 1, sent_by: str | None = A, author: str = "human") -> dict:
    return {"version": version, "sent_by": sent_by, "author_type": author}


def test_tu_vung_dong_va_khong_chong_len_nhau():
    assert len(pair_paper.PAPER_STATES) == 13
    assert len(set(pair_paper.PAPER_STATES)) == 13
    nhom = (pair_paper.OPEN_STATES, pair_paper.PLAN_STATES, pair_paper.TERMINAL)
    assert sum(len(n) for n in nhom) == 13
    assert set().union(*(set(n) for n in nhom)) == set(pair_paper.PAPER_STATES)
    for i, mot in enumerate(nhom):
        for hai in nhom[i + 1 :]:
            assert not set(mot) & set(hai)


@pytest.mark.parametrize("state", pair_paper.OPEN_STATES)
def test_qua_khung_thi_mot_to_chua_quyet_doc_la_het_han(state):
    assert pair_paper.hieu_luc(to(state), now=KHUNG) == "het_han"
    assert pair_paper.hieu_luc(to(state), now=NOW) == state


@pytest.mark.parametrize("state", (*pair_paper.PLAN_STATES, *pair_paper.TERMINAL))
def test_qua_khung_khong_viet_lai_ke_hoach_hay_tuan_da_khep(state):
    assert pair_paper.hieu_luc(to(state), now=KHUNG + timedelta(days=400)) == state


def test_im_lang_khong_bao_gio_thanh_chot():
    """Luật §3.3 số 2. Một phía đã ừ rồi hết khung vẫn là hết hạn."""
    assert pair_paper.hieu_luc(to("dong_y"), now=KHUNG) == "het_han"
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.chuyen(to("dong_y"), "dong_y", now=KHUNG, du_dong_y=True)
    assert loi.value.code == "paper_expired"


def test_khong_co_khung_thi_khong_het_han():
    assert pair_paper.hieu_luc(to("da_gui", expires_at=None), now=KHUNG) == "da_gui"


def test_dong_y_can_hai_nguoi_khac_nhau_tren_cung_phien_ban():
    assert (
        pair_paper.da_du_dong_y([{"person_id": A, "kind": "dong_y", "version": 1}], 1)
        is False
    )
    hai_lan_mot_nguoi = [
        {"person_id": A, "kind": "dong_y", "version": 1},
        {"person_id": A, "kind": "dong_y", "version": 1},
    ]
    assert pair_paper.da_du_dong_y(hai_lan_mot_nguoi, 1) is False
    du = [
        {"person_id": A, "kind": "dong_y", "version": 1},
        {"person_id": B, "kind": "dong_y", "version": 1},
    ]
    assert pair_paper.da_du_dong_y(du, 1) is True
    assert pair_paper.da_du_dong_y(du, 2) is False, "đồng ý không sang phiên bản sau"
    de_nghi = [
        {"person_id": A, "kind": "dong_y", "version": 1},
        {"person_id": B, "kind": "de_nghi_sua", "version": 1},
    ]
    assert pair_paper.da_du_dong_y(de_nghi, 1) is False


def test_to_nep_gui_khong_co_ai_dong_y_san():
    """ADR-0027: tờ tác giả `nep` cần cả hai người trả lời."""
    nep = [phien_ban(sent_by=None, author="nep")]
    assert pair_paper.da_du_dong_y([], 1) is False
    assert pair_paper.co_the_rut(to("da_gui"), nep, [], [], actor_id=A) is False, (
        "không ai rút được tờ của Nếp"
    )


def test_rut_chi_khi_nguoi_gui_va_chua_ai_xem_chua_ai_tra_loi():
    versions = [phien_ban()]
    gui = to("da_gui")
    cua_toi = [{"version": 1, "person_id": A, "kind": "dong_y"}]
    assert pair_paper.co_the_rut(gui, versions, [], cua_toi, actor_id=A) is True
    assert pair_paper.co_the_rut(gui, versions, [], cua_toi, actor_id=B) is False
    da_xem = [{"version": 1, "person_id": B}]
    assert pair_paper.co_the_rut(gui, versions, da_xem, cua_toi, actor_id=A) is False
    da_tra_loi = [*cua_toi, {"version": 1, "person_id": B, "kind": "de_nghi_sua"}]
    assert pair_paper.co_the_rut(gui, versions, [], da_tra_loi, actor_id=A) is False
    assert (
        pair_paper.co_the_rut(to("da_xem"), versions, [], cua_toi, actor_id=A) is False
    )
    xem_phien_khac = [{"version": 2, "person_id": B}]
    assert (
        pair_paper.co_the_rut(gui, versions, xem_phien_khac, cua_toi, actor_id=A)
        is True
    )


def test_vong_mot_tuan_di_het_duong():
    paper = to("nhap")
    paper = pair_paper.chuyen(paper, "gui", now=NOW)
    assert paper["state"] == "da_gui"
    paper = pair_paper.chuyen(paper, "xem", now=NOW)
    assert paper["state"] == "da_xem"
    mot_phia = pair_paper.chuyen(paper, "dong_y", now=NOW, du_dong_y=False)
    assert mot_phia["state"] == "dong_y"
    ca_hai = pair_paper.chuyen(mot_phia, "dong_y", now=NOW, du_dong_y=True)
    assert ca_hai["state"] == "chot"
    da_di = pair_paper.chuyen(ca_hai, "da_di", now=NOW, nguoi_ghi=A)
    assert da_di["state"] == "da_di"
    assert pair_paper.chuyen(da_di, "giu", now=NOW)["state"] == "da_giu"


def test_da_di_can_nguoi_ghi_nhan():
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.chuyen(to("chot"), "da_di", now=NOW, nguoi_ghi=None)
    assert loi.value.code == "paper_needs_recorder"


def test_de_nghi_sua_tao_phien_ban_moi_va_quay_ve_da_gui():
    moi = pair_paper.chuyen(to("da_xem"), "de_nghi_sua", now=NOW)
    assert moi["state"] == "da_gui"
    assert moi["current_version"] == 2


@pytest.mark.parametrize("state", pair_paper.PLAN_STATES)
def test_dong_bang_sau_chot(state):
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.chuyen(to(state), "de_nghi_sua", now=NOW)
    assert loi.value.code == "paper_frozen"


def test_nghi_tuan_bo_nhap_va_huy_to_da_gui():
    assert pair_paper.chuyen(to("nhap"), "nghi_tuan", now=NOW)["state"] == "nghi_tuan"
    for state in ("da_gui", "da_xem", "de_nghi_sua", "dong_y"):
        assert pair_paper.chuyen(to(state), "nghi_tuan", now=NOW)["state"] == "huy"
    for state in ("chot", "da_di"):
        with pytest.raises(pair_paper.PaperError) as loi:
            pair_paper.chuyen(to(state), "nghi_tuan", now=NOW)
        assert loi.value.code == "paper_wrong_state"


def test_rut_khong_co_su_that_thi_tu_choi():
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.chuyen(to("da_gui"), "rut", now=NOW, co_the_rut=False)
    assert loi.value.code == "paper_not_withdrawable"
    assert (
        pair_paper.chuyen(to("da_gui"), "rut", now=NOW, co_the_rut=True)["state"]
        == "rut"
    )


@pytest.mark.parametrize("state", pair_paper.TERMINAL)
@pytest.mark.parametrize(
    "su_kien", ("gui", "xem", "dong_y", "de_nghi_sua", "rut", "da_di", "huy")
)
def test_trang_thai_cuoi_khong_di_tiep_duoc(state, su_kien):
    if state == "da_giu" and su_kien == "giu":
        return
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.chuyen(
            to(state), su_kien, now=NOW, du_dong_y=True, co_the_rut=True, nguoi_ghi=A
        )
    assert loi.value.code in {"paper_wrong_state", "paper_frozen"}


def test_su_kien_khong_biet_thi_tu_choi():
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.chuyen(to("nhap"), "xoa_het", now=NOW)
    assert loi.value.code == "paper_event_unknown"


def test_phac_to_giay_la_mau_xac_dinh_va_noi_chua_biet():
    routine = {
        "ngay": date(2026, 9, 20),
        "gio": "18:30",
        "viec": "Ăn tối, một quán chưa đi",
        "ly_do": "Ba tuần liền hai bạn ăn ở cùng một khu.",
    }
    mot = pair_paper.phac_to_giay(routine, (), now=NOW)
    hai = pair_paper.phac_to_giay(routine, (), now=NOW)
    assert mot == hai, "cùng đầu vào, cùng đầu ra: không gọi mô hình"
    assert mot["content"]["ngay"] == "2026-09-20"
    assert len(mot["content"]["chang"]) == 1
    chang = mot["content"]["chang"][0]
    assert chang["place_id"] is None and chang["can_kiem"] is True
    assert mot["nguon"]["scope"] == "chung"
    assert "rang_buoc" not in mot["nguon"]["dung"]


def test_phac_to_giay_them_chang_di_tiep_va_ghi_nguon_rang_buoc():
    routine = {
        "ngay": date(2026, 9, 20),
        "gio": "18:30",
        "viec": "Ăn tối",
        "di_tiep": {"gio": "20:00", "viec": "Đi bộ, rồi chè"},
        "ly_do": "",
    }
    ra = pair_paper.phac_to_giay(
        routine, ({"kind": "khong_an_duoc", "content": "Hải sản"},), now=NOW
    )
    assert [c["gio"] for c in ra["content"]["chang"]] == ["18:30", "20:00"]
    assert all(c["can_kiem"] for c in ra["content"]["chang"])
    assert "rang_buoc" in ra["nguon"]["dung"]


def test_phac_to_giay_doi_ngay_that():
    with pytest.raises(pair_paper.PaperError) as loi:
        pair_paper.phac_to_giay(
            {"ngay": "2026-09-20", "gio": "18:30", "viec": "x"}, (), now=NOW
        )
    assert loi.value.code == "paper_draft_needs_date"
