"""Tờ giấy ở tầng PostgreSQL: bất biến, một tờ một outing, một tờ đang mở.

Bốn luật mà một fake dict không diễn đạt được, và mỗi luật một ca ĐỎ có ghim
câu chữ của chính trigger — một ca đỏ không nói được nó đỏ vì cái gì thì cũng
xanh khi cái nó gác biến mất.
"""

from __future__ import annotations

import uuid
from datetime import UTC, date, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.exc import DBAPIError, IntegrityError
from sqlalchemy.orm import Session

from app.db.models import (
    Context,
    Membership,
    MembershipState,
    Outing,
    PairPaper,
    PairPaperKeep,
    PairPaperOuting,
    PairPaperResponse,
    PairPaperVersion,
    PairPaperView,
    Person,
)

NOW = datetime(2026, 9, 13, 12, 0, tzinfo=UTC)
TUAN = date(2026, 9, 14)
KHUNG = NOW + timedelta(days=3)

_KIEM_HOAN = text("SET CONSTRAINTS ALL IMMEDIATE")
_T1 = "immutable once sent"
_T2_TRANG_THAI = "outing link needs a chot paper"
_T2_PHIEN_BAN = "must name the current version"
_T2_NGU_CANH = "belongs to another conversation"
_T4 = "responder is not in this conversation"


def _nguoi(session: Session, ten: str) -> uuid.UUID:
    person = Person(display_name=ten)
    session.add(person)
    session.flush()
    return person.id


def _cap(session: Session) -> tuple[uuid.UUID, uuid.UUID, uuid.UUID]:
    """Context `pair` với hai membership ACTIVE, như sản phẩm tạo."""
    a = _nguoi(session, "Người A")
    b = _nguoi(session, "Người B")
    context_id = uuid.uuid4()
    session.add(
        Context(
            id=context_id,
            display_name="",
            created_by_id=a,
            kind="pair",
            pair_key=f"{min(str(a), str(b))}:{max(str(a), str(b))}",
        )
    )
    session.flush()
    for person_id in (a, b):
        session.add(
            Membership(
                context_id=context_id,
                person_id=person_id,
                state=MembershipState.ACTIVE,
                joined_at=NOW,
            )
        )
    session.flush()
    return context_id, a, b


def _to(
    session: Session,
    context_id: uuid.UUID,
    owner: uuid.UUID,
    *,
    state: str = "nhap",
    version: int = 1,
) -> uuid.UUID:
    paper = PairPaper(
        context_id=context_id,
        is_temporary=True,
        draft_owner_id=owner,
        state=state,
        current_version=version,
        tuan=TUAN,
        expires_at=KHUNG,
    )
    session.add(paper)
    session.flush()
    return paper.id


def _phien_ban(
    session: Session,
    paper_id: uuid.UUID,
    version: int,
    *,
    sent_by: uuid.UUID | None = None,
) -> None:
    session.add(
        PairPaperVersion(
            paper_id=paper_id,
            version=version,
            content={"ngay": "2026-09-20", "chang": []},
            author_type="human",
            sent_at=NOW if sent_by else None,
            sent_by=sent_by,
        )
    )
    session.flush()


def _buoi(session: Session, context_id: uuid.UUID, creator: uuid.UUID) -> uuid.UUID:
    outing = Outing(
        context_id=context_id,
        created_by_id=creator,
        title="Tờ lời rủ",
        starts_on=TUAN,
        ends_on=TUAN,
        headcount=2,
        budget_per_person_vnd=0,
    )
    session.add(outing)
    session.flush()
    return outing.id


def test_phien_ban_da_gui_khong_sua_duoc(postgres_session: Session):
    """T1, ca ĐỎ. v1 là cái người kia đã đọc; UPDATE nó là viết lại điều họ
    đã đồng ý, SAU khi họ đồng ý."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)

    with pytest.raises(DBAPIError) as loi:
        postgres_session.execute(
            text(
                "UPDATE pair_paper_versions SET ly_do = 'viết lại'"
                " WHERE paper_id = :p AND version = 1"
            ),
            {"p": paper_id},
        )
    assert _T1 in str(loi.value)
    postgres_session.rollback()


def test_phien_ban_da_gui_khong_xoa_duoc(postgres_session: Session):
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)

    with pytest.raises(DBAPIError) as loi:
        postgres_session.execute(
            text("DELETE FROM pair_paper_versions WHERE paper_id = :p"), {"p": paper_id}
        )
    assert _T1 in str(loi.value)
    postgres_session.rollback()


def test_ban_nhap_chua_gui_thi_sua_duoc(postgres_session: Session):
    """Đối chứng xanh của T1: bản nháp chưa ai nhận vẫn sửa tại chỗ được
    (§3.3 luật 1 — nháp không có lịch sử)."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a)
    _phien_ban(postgres_session, paper_id, 1)

    postgres_session.execute(
        text("UPDATE pair_paper_versions SET ly_do = 'sửa nháp' WHERE paper_id = :p"),
        {"p": paper_id},
    )
    postgres_session.flush()
    con_lai = postgres_session.execute(
        text("SELECT ly_do FROM pair_paper_versions WHERE paper_id = :p"),
        {"p": paper_id},
    ).scalar_one()
    assert con_lai == "sửa nháp"


def test_mot_cuoc_tro_chuyen_chi_mot_to_dang_mo(postgres_session: Session):
    """§15.1 thành ràng buộc: luật chỉ sống trong service thì đúng lúc hai
    request tới cùng nhau là lúc nó thôi đúng."""
    context_id, a, _ = _cap(postgres_session)
    _to(postgres_session, context_id, a, state="da_gui")

    postgres_session.add(
        PairPaper(
            context_id=context_id,
            is_temporary=True,
            draft_owner_id=a,
            state="nhap",
            tuan=TUAN,
            expires_at=KHUNG,
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_to_da_khep_khong_chan_to_moi(postgres_session: Session):
    """Đối chứng xanh: partial unique chỉ gác những trạng thái ĐANG MỞ."""
    context_id, a, _ = _cap(postgres_session)
    _to(postgres_session, context_id, a, state="da_giu")
    moi = _to(postgres_session, context_id, a, state="nhap")
    assert postgres_session.get(PairPaper, moi) is not None


def test_mot_to_chi_sinh_mot_outing(postgres_session: Session):
    """K3, ca ĐỎ: khoá chính là TỜ, nên gửi lại vì mất mạng không nhân đôi."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="chot")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    postgres_session.add(
        PairPaperOuting(
            paper_id=paper_id,
            version=1,
            outing_id=_buoi(postgres_session, context_id, a),
        )
    )
    postgres_session.flush()
    postgres_session.execute(_KIEM_HOAN)

    postgres_session.add(
        PairPaperOuting(
            paper_id=paper_id,
            version=1,
            outing_id=_buoi(postgres_session, context_id, a),
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_link_outing_doi_to_da_chot(postgres_session: Session):
    """T2, ca ĐỎ thứ nhất."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)

    postgres_session.add(
        PairPaperOuting(
            paper_id=paper_id,
            version=1,
            outing_id=_buoi(postgres_session, context_id, a),
        )
    )
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
        postgres_session.execute(_KIEM_HOAN)
    assert _T2_TRANG_THAI in str(loi.value)
    postgres_session.rollback()


def test_link_outing_doi_dung_phien_ban_hien_tai(postgres_session: Session):
    """T2, ca ĐỎ thứ hai: chốt v2 mà link v1 là chốt một bản không ai đồng ý."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="chot", version=2)
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    _phien_ban(postgres_session, paper_id, 2, sent_by=a)

    postgres_session.add(
        PairPaperOuting(
            paper_id=paper_id,
            version=1,
            outing_id=_buoi(postgres_session, context_id, a),
        )
    )
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
        postgres_session.execute(_KIEM_HOAN)
    assert _T2_PHIEN_BAN in str(loi.value)
    postgres_session.rollback()


def test_link_outing_doi_cung_mot_cuoc_tro_chuyen(postgres_session: Session):
    """T2, ca ĐỎ thứ ba: một buổi của nhóm khác không phải buổi của hai người."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="chot")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    nhom_khac = uuid.uuid4()
    postgres_session.add(
        Context(id=nhom_khac, display_name="Hội bạn", created_by_id=a, kind="group")
    )
    postgres_session.flush()

    postgres_session.add(
        PairPaperOuting(
            paper_id=paper_id,
            version=1,
            outing_id=_buoi(postgres_session, nhom_khac, a),
        )
    )
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
        postgres_session.execute(_KIEM_HOAN)
    assert _T2_NGU_CANH in str(loi.value)
    postgres_session.rollback()


def test_link_outing_dung_thi_ghi_duoc(postgres_session: Session):
    """Đối chứng xanh của T2, cả ba nhánh cùng một lúc."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="chot")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    outing_id = _buoi(postgres_session, context_id, a)

    postgres_session.add(
        PairPaperOuting(paper_id=paper_id, version=1, outing_id=outing_id)
    )
    postgres_session.flush()
    postgres_session.execute(_KIEM_HOAN)
    assert postgres_session.get(PairPaperOuting, paper_id) is not None


def test_mot_nguoi_mot_dong_y_tren_mot_phien_ban(postgres_session: Session):
    """§3.3 luật 3 thành partial unique."""
    context_id, a, b = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    postgres_session.add(
        PairPaperResponse(paper_id=paper_id, version=1, person_id=b, kind="dong_y")
    )
    postgres_session.flush()

    postgres_session.add(
        PairPaperResponse(paper_id=paper_id, version=1, person_id=b, kind="dong_y")
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_de_nghi_sua_hai_lan_thi_duoc(postgres_session: Session):
    """Đối chứng xanh: partial unique chỉ gác «ừ». Đổi ý về việc muốn sửa gì
    là chuyện thường."""
    context_id, a, b = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    for _ in range(2):
        postgres_session.add(
            PairPaperResponse(
                paper_id=paper_id, version=1, person_id=b, kind="de_nghi_sua"
            )
        )
    postgres_session.flush()
    so = postgres_session.execute(
        text("SELECT count(*) FROM pair_paper_responses WHERE paper_id = :p"),
        {"p": paper_id},
    ).scalar_one()
    assert so == 2


def test_nguoi_ngoai_cuoc_tro_chuyen_khong_tra_loi_duoc(postgres_session: Session):
    """T4, ca ĐỎ."""
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    nguoi_la = _nguoi(postgres_session, "Người lạ")

    postgres_session.add(
        PairPaperResponse(
            paper_id=paper_id, version=1, person_id=nguoi_la, kind="dong_y"
        )
    )
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
    assert _T4 in str(loi.value)
    postgres_session.rollback()


def test_nguoi_da_roi_cuoc_tro_chuyen_cung_khong_tra_loi_duoc(
    postgres_session: Session,
):
    """T4 đọc `left_at`: rời rồi thì thôi, dù hàng membership vẫn còn."""
    context_id, a, b = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    postgres_session.execute(
        text(
            "UPDATE memberships SET left_at = :now, state = 'left'"
            " WHERE context_id = :c AND person_id = :p"
        ),
        {"now": NOW, "c": context_id, "p": b},
    )
    postgres_session.flush()

    postgres_session.add(
        PairPaperResponse(paper_id=paper_id, version=1, person_id=b, kind="dong_y")
    )
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
    assert _T4 in str(loi.value)
    postgres_session.rollback()


def test_dong_giu_lai_khong_duoc_rong(postgres_session: Session):
    context_id, a, _ = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_di")

    postgres_session.add(PairPaperKeep(paper_id=paper_id, person_id=a, line="   \n  "))
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_moc_xem_la_mot_hang_moi_nguoi_moi_phien_ban(postgres_session: Session):
    """Nhìn hai lần vẫn là một lần nhìn (§7.5)."""
    context_id, a, b = _cap(postgres_session)
    paper_id = _to(postgres_session, context_id, a, state="da_gui")
    _phien_ban(postgres_session, paper_id, 1, sent_by=a)
    postgres_session.add(PairPaperView(paper_id=paper_id, version=1, person_id=b))
    postgres_session.flush()

    postgres_session.add(PairPaperView(paper_id=paper_id, version=1, person_id=b))
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_da_di_phai_co_nguoi_ghi_nhan(postgres_session: Session):
    """§3.1 thành CHECK: hai cột đi cùng nhau hoặc không cột nào."""
    context_id, a, _ = _cap(postgres_session)
    postgres_session.add(
        PairPaper(
            context_id=context_id,
            is_temporary=True,
            draft_owner_id=a,
            state="da_di",
            tuan=TUAN,
            expires_at=KHUNG,
            done_recorded_at=NOW,
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_to_tam_thoi_khong_co_chu_ky_va_nguoc_lai(postgres_session: Session):
    context_id, a, _ = _cap(postgres_session)
    postgres_session.add(
        PairPaper(
            context_id=context_id,
            is_temporary=False,
            cycle_id=None,
            draft_owner_id=a,
            state="nhap",
            tuan=TUAN,
            expires_at=KHUNG,
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()
