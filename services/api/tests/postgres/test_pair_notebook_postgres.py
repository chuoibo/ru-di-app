"""Sổ hai người ở tầng PostgreSQL (ADR-0027 K1, K2, K6).

Những luật ở đây KHÔNG diễn đạt được bằng một fake dict: khoá ngoại ghép,
partial unique, và ba trigger. Mỗi cái một ca ĐỎ — một trigger không có ca đỏ
là trang trí — và mỗi ca đỏ đi kèm một đối chứng XANH, vì một ca đỏ không có
đối chứng cũng đỏ khi bảng không tồn tại.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.exc import DBAPIError, IntegrityError
from sqlalchemy.orm import Session

from app.db.models import (
    ActiveCoupleMember,
    Context,
    PairConsent,
    PairConsentProposal,
    PairCycleParticipant,
    PairNotebook,
    PairNotebookCycle,
    Person,
)

NOW = datetime(2026, 9, 13, 12, 0, tzinfo=UTC)

#: Trigger T3 và T2 là CONSTRAINT TRIGGER DEFERRED: chúng chỉ chạy khi giao
#: dịch (hoặc điểm kiểm) kết thúc, nên `flush()` một mình KHÔNG đủ để thấy
#: chúng nổ. Câu này ép kiểm ngay tại chỗ, để ca đỏ đỏ ở đúng dòng nó nói.
_KIEM_HOAN = text("SET CONSTRAINTS ALL IMMEDIATE")

#: Câu chữ của chính trigger. Ghim vào để một ca đỏ đỏ VÌ trigger, chứ không vì
#: một khoá ngoại nào đó tình cờ cũng nổ ở cùng dòng.
_T3 = "a couple needs both consents"


def _nguoi(session: Session, ten: str) -> uuid.UUID:
    person = Person(display_name=ten)
    session.add(person)
    session.flush()
    return person.id


def _cap(session: Session) -> tuple[uuid.UUID, uuid.UUID, uuid.UUID]:
    """Một context `pair` thật với hai người, như `open_direct_message` tạo."""
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
    return context_id, a, b


def _so(session: Session, context_id: uuid.UUID) -> uuid.UUID:
    notebook = PairNotebook(context_id=context_id)
    session.add(notebook)
    session.flush()
    return notebook.id


def _chu_ky(
    session: Session, notebook_id: uuid.UUID, *, state: str = "active", people=()
) -> uuid.UUID:
    cycle = PairNotebookCycle(
        notebook_id=notebook_id,
        state=state,
        opened_at=NOW if state != "pending" else None,
        closed_at=NOW if state == "closed" else None,
    )
    session.add(cycle)
    session.flush()
    for person_id in people:
        session.add(PairCycleParticipant(cycle_id=cycle.id, person_id=person_id))
    session.flush()
    return cycle.id


def _dong_y(
    session: Session,
    cycle_id: uuid.UUID,
    *,
    purpose: str,
    by: uuid.UUID,
    people,
    granted: bool = True,
) -> uuid.UUID:
    proposal = PairConsentProposal(
        cycle_id=cycle_id,
        purpose=purpose,
        proposed_by_id=by,
        expires_at=NOW + timedelta(days=3),
    )
    session.add(proposal)
    session.flush()
    for person_id in people:
        session.add(
            PairConsent(
                proposal_id=proposal.id,
                person_id=person_id,
                granted_at=NOW if granted else None,
            )
        )
    session.flush()
    return proposal.id


def test_khoa_ngoai_ghep_tu_choi_mot_nhom_deo_nhan_pair(postgres_session: Session):
    """CHECK một mình cho phép dán nhãn: hàng con nói 'pair' bên cạnh một
    `context_id` của NHÓM và không ai cãi được. Khoá ngoại ghép mới là cái
    database từ chối (ADR-0027 §2)."""
    nhom_id = uuid.uuid4()
    nguoi = _nguoi(postgres_session, "Người tạo")
    postgres_session.add(
        Context(id=nhom_id, display_name="Hội bạn", created_by_id=nguoi, kind="group")
    )
    postgres_session.flush()

    postgres_session.add(PairNotebook(context_id=nhom_id, context_kind="pair"))
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_so_cua_mot_cap_that_thi_ghi_duoc(postgres_session: Session):
    """Đối chứng xanh của ca trên: cùng câu lệnh, context đúng loại."""
    context_id, _, _ = _cap(postgres_session)
    notebook_id = _so(postgres_session, context_id)
    assert postgres_session.get(PairNotebook, notebook_id) is not None


def test_mot_so_chi_co_mot_chu_ky_chua_dong(postgres_session: Session):
    context_id, a, b = _cap(postgres_session)
    notebook_id = _so(postgres_session, context_id)
    _chu_ky(postgres_session, notebook_id, state="active", people=(a, b))

    postgres_session.add(PairNotebookCycle(notebook_id=notebook_id, state="pending"))
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_dong_chu_ky_cu_roi_mo_chu_ky_moi_thi_duoc(postgres_session: Session):
    """Mở lại là chu kỳ MỚI, không phải hồi sinh chu kỳ cũ (§7.6)."""
    context_id, a, b = _cap(postgres_session)
    notebook_id = _so(postgres_session, context_id)
    _chu_ky(postgres_session, notebook_id, state="closed", people=(a, b))
    moi = _chu_ky(postgres_session, notebook_id, state="active", people=(a, b))
    assert postgres_session.get(PairNotebookCycle, moi) is not None


def test_mot_nguoi_mot_dong_y_tren_mot_loi_de_nghi(postgres_session: Session):
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    proposal_id = _dong_y(
        postgres_session, cycle_id, purpose="lap_so", by=a, people=(a,)
    )

    postgres_session.add(
        PairConsent(proposal_id=proposal_id, person_id=a, granted_at=NOW)
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_thu_hoi_ma_chua_tung_dong_y_bi_tu_choi(postgres_session: Session):
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    proposal_id = _dong_y(
        postgres_session, cycle_id, purpose="lap_so", by=a, people=(), granted=False
    )
    postgres_session.add(
        PairConsent(
            proposal_id=proposal_id, person_id=a, granted_at=None, revoked_at=NOW
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_mot_doi_can_dong_y_cua_ca_hai_nguoi(postgres_session: Session):
    """T3, ca ĐỎ. Một đồng ý là một người tự nhận hai người là một đôi."""
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    _dong_y(postgres_session, cycle_id, purpose="bat_doi", by=a, people=(a,))

    postgres_session.add(ActiveCoupleMember(person_id=a, cycle_id=cycle_id))
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
        postgres_session.execute(_KIEM_HOAN)
    assert _T3 in str(loi.value), "phải đỏ vì T3, không vì một ràng buộc khác"
    postgres_session.rollback()


def test_mot_doi_voi_hai_dong_y_thi_ghi_duoc(postgres_session: Session):
    """Đối chứng xanh của T3."""
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    _dong_y(postgres_session, cycle_id, purpose="bat_doi", by=a, people=(a, b))

    postgres_session.add(ActiveCoupleMember(person_id=a, cycle_id=cycle_id))
    postgres_session.add(ActiveCoupleMember(person_id=b, cycle_id=cycle_id))
    postgres_session.flush()
    postgres_session.execute(_KIEM_HOAN)
    assert postgres_session.get(ActiveCoupleMember, a) is not None


def test_dong_y_bi_thu_hoi_thi_khong_con_la_mot_doi(postgres_session: Session):
    """T3 đọc `revoked_at`: rút lại là rút lại ngay, không chờ khởi động lại."""
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    proposal_id = _dong_y(
        postgres_session, cycle_id, purpose="bat_doi", by=a, people=(a, b)
    )
    thu_hoi = (
        postgres_session.query(PairConsent)
        .filter(PairConsent.proposal_id == proposal_id, PairConsent.person_id == b)
        .one()
    )
    thu_hoi.revoked_at = NOW
    postgres_session.flush()

    postgres_session.add(ActiveCoupleMember(person_id=a, cycle_id=cycle_id))
    with pytest.raises(DBAPIError) as loi:
        postgres_session.flush()
        postgres_session.execute(_KIEM_HOAN)
    assert _T3 in str(loi.value)
    postgres_session.rollback()


def test_mot_nguoi_khong_the_o_hai_so_doi(postgres_session: Session):
    """K2: khoá chính là NGƯỜI, nên «một người một sổ đôi» là điều database
    từ chối chứ không phải điều service nhớ kiểm."""
    ngu_canh_mot, a, b = _cap(postgres_session)
    chu_ky_mot = _chu_ky(
        postgres_session, _so(postgres_session, ngu_canh_mot), people=(a, b)
    )
    _dong_y(postgres_session, chu_ky_mot, purpose="bat_doi", by=a, people=(a, b))
    postgres_session.add(ActiveCoupleMember(person_id=a, cycle_id=chu_ky_mot))
    postgres_session.flush()
    postgres_session.execute(_KIEM_HOAN)

    c = _nguoi(postgres_session, "Người C")
    ngu_canh_hai = uuid.uuid4()
    postgres_session.add(
        Context(
            id=ngu_canh_hai,
            display_name="",
            created_by_id=a,
            kind="pair",
            pair_key=f"{min(str(a), str(c))}:{max(str(a), str(c))}",
        )
    )
    postgres_session.flush()
    chu_ky_hai = _chu_ky(
        postgres_session, _so(postgres_session, ngu_canh_hai), people=(a, c)
    )
    _dong_y(postgres_session, chu_ky_hai, purpose="bat_doi", by=a, people=(a, c))

    postgres_session.add(ActiveCoupleMember(person_id=a, cycle_id=chu_ky_hai))
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_han_cua_loi_de_nghi_phai_sau_luc_tao(postgres_session: Session):
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    postgres_session.add(
        PairConsentProposal(
            cycle_id=cycle_id,
            purpose="lap_so",
            proposed_by_id=a,
            created_at=NOW,
            expires_at=NOW - timedelta(seconds=1),
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()


def test_muc_dich_ngoai_tu_vung_bi_tu_choi(postgres_session: Session):
    context_id, a, b = _cap(postgres_session)
    cycle_id = _chu_ky(
        postgres_session, _so(postgres_session, context_id), people=(a, b)
    )
    postgres_session.add(
        PairConsentProposal(
            cycle_id=cycle_id,
            purpose="doc_het_moi_thu",
            proposed_by_id=a,
            expires_at=NOW + timedelta(days=1),
        )
    )
    with pytest.raises(IntegrityError):
        postgres_session.flush()
    postgres_session.rollback()
