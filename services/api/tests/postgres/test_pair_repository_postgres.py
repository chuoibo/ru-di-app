"""`SqlAlchemyApiRepository` lái trọn một tuần của sổ hai người, trên DB thật.

Ba mươi hai ca ở `test_pair_notebook_postgres` và `test_pair_papers_postgres`
chứng minh **schema** từ chối đúng thứ nó phải từ chối. Ca này chứng minh thứ
khác: mã thật đọc và ghi đúng những hàng ấy. Một fake dict có thể làm cả hai
việc trông giống nhau, và đó là lý do tầng này tồn tại.
"""

from __future__ import annotations

import uuid
from datetime import UTC, date, datetime, timedelta

import pytest
from sqlalchemy.orm import Session

from app.api.errors import RepositoryConflict
from app.api.repository import SqlAlchemyApiRepository
from app.db.models import Context, Membership, MembershipState, Person
from app.domain import pair_notebook, pair_paper

NOW = datetime(2026, 9, 13, 12, 0, tzinfo=UTC)
TUAN = date(2026, 9, 14)
KHUNG = NOW + timedelta(days=3)
NOI_DUNG = {
    "ngay": "2026-09-20",
    "chang": [
        {
            "gio": "18:30",
            "viec": "Ăn tối, một quán chưa đi",
            "place_id": None,
            "can_kiem": True,
        }
    ],
}


@pytest.fixture
def kho(postgres_session: Session) -> SqlAlchemyApiRepository:
    return SqlAlchemyApiRepository(postgres_session)


def _cap(session: Session) -> tuple[uuid.UUID, uuid.UUID, uuid.UUID]:
    a = Person(display_name="Người A")
    b = Person(display_name="Người B")
    session.add_all([a, b])
    session.flush()
    context_id = uuid.uuid4()
    session.add(
        Context(
            id=context_id,
            display_name="",
            created_by_id=a.id,
            kind="pair",
            pair_key=f"{min(str(a.id), str(b.id))}:{max(str(a.id), str(b.id))}",
        )
    )
    session.flush()
    for person in (a, b):
        session.add(
            Membership(
                context_id=context_id,
                person_id=person.id,
                state=MembershipState.ACTIVE,
                joined_at=NOW,
            )
        )
    session.flush()
    return context_id, a.id, b.id


def _so_mo(kho: SqlAlchemyApiRepository, session: Session):
    """Sổ với một chu kỳ đang mở và cả hai đã đồng ý lập sổ."""
    context_id, a, b = _cap(session)
    notebook = kho.create_pair_notebook(context_id, now=NOW)
    cycle_id = kho.open_pair_cycle(
        notebook.id, participants=(a, b), terms_version=1, now=NOW
    )
    proposal = kho.create_consent_proposal(
        cycle_id=cycle_id,
        purpose="lap_so",
        proposed_by_id=a,
        terms_version=1,
        expires_at=NOW + timedelta(days=3),
        now=NOW,
    )
    kho.grant_consent(proposal.id, a, now=NOW)
    kho.grant_consent(proposal.id, b, now=NOW)
    kho.complete_consent_proposal(proposal.id, now=NOW)
    kho.activate_pair_cycle(cycle_id, now=NOW)
    return context_id, a, b, cycle_id


def test_mot_tuan_di_het_duong_qua_kho_that(kho, postgres_session):
    """Phác → gửi → xem → ừ → chốt → đã đi → giữ, đọc lại sau mỗi bước."""
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)

    paper = kho.create_pair_paper(
        context_id=context_id,
        cycle_id=cycle_id,
        draft_owner_id=a,
        tuan=TUAN,
        expires_at=KHUNG,
        content=NOI_DUNG,
        ly_do="Ba tuần liền hai bạn ăn ở cùng một khu.",
        nguon={"scope": "chung"},
        author_type="human",
        now=NOW,
    )
    assert paper.state == "nhap"
    assert paper.versions[0].sent_at is None
    assert paper.is_temporary is False

    kho.mark_version_sent(paper.id, 1, sent_by=a, now=NOW)
    kho.add_paper_response(
        paper_id=paper.id, version=1, person_id=a, kind="dong_y", now=NOW
    )
    kho.set_paper_state(paper.id, "da_gui", now=NOW)
    doc = kho.get_pair_paper(paper.id)
    assert doc.state == "da_gui"
    assert doc.versions[0].sent_by == a
    assert [r.person_id for r in doc.responses] == [a]
    assert (
        pair_paper.da_du_dong_y(
            [
                {"person_id": str(r.person_id), "kind": r.kind, "version": r.version}
                for r in doc.responses
            ],
            1,
        )
        is False
    ), "một người gửi mới là một đồng ý, chưa đủ"

    seen_at = kho.mark_paper_viewed(paper.id, 1, b, now=NOW + timedelta(minutes=5))
    lai = kho.mark_paper_viewed(paper.id, 1, b, now=NOW + timedelta(hours=9))
    assert lai == seen_at, "nhìn lần hai vẫn là lần nhìn đầu"
    kho.set_paper_state(paper.id, "da_xem", now=NOW)

    kho.add_paper_response(
        paper_id=paper.id, version=1, person_id=b, kind="dong_y", now=NOW
    )
    doc = kho.get_pair_paper(paper.id)
    assert (
        pair_paper.da_du_dong_y(
            [
                {"person_id": str(r.person_id), "kind": r.kind, "version": r.version}
                for r in doc.responses
            ],
            1,
        )
        is True
    )

    kho.set_paper_state(paper.id, "chot", now=NOW)
    outing = kho.create_outing(
        context_id=context_id,
        created_by_id=b,
        title="Tờ lời rủ 20/09",
        starts_on=date(2026, 9, 20),
        ends_on=date(2026, 9, 20),
        headcount=2,
        budget_per_person_vnd=0,
        now=NOW,
    )
    kho.link_paper_outing(paper_id=paper.id, version=1, outing_id=outing.id, now=NOW)
    assert kho.get_paper_outing(paper.id) == outing.id

    kho.set_paper_state(paper.id, "da_di", now=NOW, recorded_by_id=b)
    doc = kho.get_pair_paper(paper.id)
    assert doc.done_recorded_by_id == b and doc.done_recorded_at is not None

    kho.add_paper_keep(
        paper_id=paper.id, person_id=b, line="Quán này ồn, lần sau ngồi sân.", now=NOW
    )
    kho.set_paper_state(paper.id, "da_giu", now=NOW)
    doc = kho.get_pair_paper(paper.id)
    assert doc.state == "da_giu"
    assert [k.line for k in doc.keeps] == ["Quán này ồn, lần sau ngồi sân."]
    assert doc.outing_id == outing.id


def test_gui_lai_lan_hai_khong_sinh_outing_thu_hai(kho, postgres_session):
    """K3 qua kho thật: lần thứ hai là một xung đột, không phải một outing nữa."""
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    paper = kho.create_pair_paper(
        context_id=context_id,
        cycle_id=cycle_id,
        draft_owner_id=a,
        tuan=TUAN,
        expires_at=KHUNG,
        content=NOI_DUNG,
        ly_do=None,
        nguon={},
        author_type="human",
        now=NOW,
    )
    kho.mark_version_sent(paper.id, 1, sent_by=a, now=NOW)
    kho.set_paper_state(paper.id, "chot", now=NOW)
    mot = kho.create_outing(
        context_id=context_id,
        created_by_id=a,
        title="Tờ lời rủ",
        starts_on=TUAN,
        ends_on=TUAN,
        headcount=2,
        budget_per_person_vnd=0,
        now=NOW,
    )
    kho.link_paper_outing(paper_id=paper.id, version=1, outing_id=mot.id, now=NOW)

    hai = kho.create_outing(
        context_id=context_id,
        created_by_id=a,
        title="Tờ lời rủ",
        starts_on=TUAN,
        ends_on=TUAN,
        headcount=2,
        budget_per_person_vnd=0,
        now=NOW,
    )
    with pytest.raises(RepositoryConflict) as loi:
        kho.link_paper_outing(paper_id=paper.id, version=1, outing_id=hai.id, now=NOW)
    assert loi.value.code == "paper_outing_exists"


def test_mot_nguoi_u_hai_lan_la_mot_xung_dot(kho, postgres_session):
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    paper = kho.create_pair_paper(
        context_id=context_id,
        cycle_id=cycle_id,
        draft_owner_id=a,
        tuan=TUAN,
        expires_at=KHUNG,
        content=NOI_DUNG,
        ly_do=None,
        nguon={},
        author_type="human",
        now=NOW,
    )
    kho.mark_version_sent(paper.id, 1, sent_by=a, now=NOW)
    kho.add_paper_response(
        paper_id=paper.id, version=1, person_id=b, kind="dong_y", now=NOW
    )
    with pytest.raises(RepositoryConflict) as loi:
        kho.add_paper_response(
            paper_id=paper.id, version=1, person_id=b, kind="dong_y", now=NOW
        )
    assert loi.value.code == "paper_already_agreed"


def test_dong_y_cua_kho_doc_duoc_bang_domain(kho, postgres_session):
    """Hàng kho ghi ra phải là đúng hình dạng `pair_notebook` đọc — nếu không,
    hai tầng nói hai thứ và cổng nào cũng xanh."""
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    so = kho.get_pair_notebook(context_id)
    consents = [
        {
            "person_id": str(c.person_id),
            "purpose": c.purpose,
            "granted_at": c.granted_at,
            "revoked_at": c.revoked_at,
            "proposal_expires_at": c.proposal_expires_at,
        }
        for c in so.consents
    ]
    nguoi = tuple(str(p) for p in so.participants)
    assert pair_notebook.granted_purposes(consents, nguoi, now=NOW) == frozenset(
        {"lap_so"}
    )
    assert pair_notebook.can_bat_doi(consents, nguoi, now=NOW) is False
    assert pair_notebook.chat_consent_active(consents, nguoi, now=NOW) is False


def test_thu_hoi_cham_moi_hang_cua_muc_dich_do(kho, postgres_session):
    """Một mục đích hỏi hai lần để lại hai lời đề nghị; thu hồi phải tới cả hai,
    không thì màn này nói «đã rút» còn màn kia nói «đang cho»."""
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    for _ in range(2):
        proposal = kho.create_consent_proposal(
            cycle_id=cycle_id,
            purpose="doc_chat",
            proposed_by_id=a,
            terms_version=1,
            expires_at=NOW + timedelta(days=3),
            now=NOW,
        )
        kho.grant_consent(proposal.id, a, now=NOW)
        kho.grant_consent(proposal.id, b, now=NOW)

    so = kho.get_pair_notebook(context_id)
    consents = [
        {
            "person_id": str(c.person_id),
            "purpose": c.purpose,
            "granted_at": c.granted_at,
            "revoked_at": c.revoked_at,
            "proposal_expires_at": c.proposal_expires_at,
        }
        for c in so.consents
    ]
    nguoi = tuple(str(p) for p in so.participants)
    assert "doc_chat" in pair_notebook.granted_purposes(consents, nguoi, now=NOW)

    assert kho.revoke_consents(cycle_id, "doc_chat", a, now=NOW) == 2
    so = kho.get_pair_notebook(context_id)
    consents = [
        {
            "person_id": str(c.person_id),
            "purpose": c.purpose,
            "granted_at": c.granted_at,
            "revoked_at": c.revoked_at,
            "proposal_expires_at": c.proposal_expires_at,
        }
        for c in so.consents
    ]
    assert "doc_chat" not in pair_notebook.granted_purposes(consents, nguoi, now=NOW)


def test_mot_nguoi_khong_o_hai_so_doi_qua_kho(kho, postgres_session):
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    proposal = kho.create_consent_proposal(
        cycle_id=cycle_id,
        purpose="bat_doi",
        proposed_by_id=a,
        terms_version=1,
        expires_at=NOW + timedelta(days=3),
        now=NOW,
    )
    kho.grant_consent(proposal.id, a, now=NOW)
    kho.grant_consent(proposal.id, b, now=NOW)
    kho.set_couple_member(a, cycle_id, now=NOW)
    kho.set_couple_member(b, cycle_id, now=NOW)
    assert kho.couple_cycle_for(a) == cycle_id

    ngu_canh_hai, c, _ = _cap(postgres_session)
    so_hai = kho.create_pair_notebook(ngu_canh_hai, now=NOW)
    chu_ky_hai = kho.open_pair_cycle(
        so_hai.id, participants=(a, c), terms_version=1, now=NOW
    )
    with pytest.raises(RepositoryConflict) as loi:
        kho.set_couple_member(a, chu_ky_hai, now=NOW)
    assert loi.value.code == "couple_slot_taken"


def test_dong_so_bo_nhap_huy_to_dang_cho_va_de_ke_hoach_yen(kho, postgres_session):
    """§7.6 qua kho thật, và ba con số khớp `xem_truoc_dong_so` của domain."""
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    nhap = kho.create_pair_paper(
        context_id=context_id,
        cycle_id=cycle_id,
        draft_owner_id=a,
        tuan=TUAN,
        expires_at=KHUNG,
        content=NOI_DUNG,
        ly_do=None,
        nguon={},
        author_type="human",
        now=NOW,
    )
    kho.set_paper_state(nhap.id, "chot", now=NOW)

    dang_cho = kho.create_pair_paper(
        context_id=context_id,
        cycle_id=cycle_id,
        draft_owner_id=a,
        tuan=TUAN + timedelta(days=7),
        expires_at=KHUNG + timedelta(days=7),
        content=NOI_DUNG,
        ly_do=None,
        nguon={},
        author_type="human",
        now=NOW,
    )
    kho.mark_version_sent(dang_cho.id, 1, sent_by=a, now=NOW)
    kho.set_paper_state(dang_cho.id, "da_gui", now=NOW)

    truoc = pair_notebook.xem_truoc_dong_so(
        [
            {
                "id": str(p.id),
                "state": p.state,
                "current_version": p.current_version,
                "expires_at": p.expires_at,
            }
            for p in kho.list_pair_papers(context_id)
        ],
        [],
        now=NOW,
    )
    assert (truoc["so_nhap_bo"], truoc["so_to_huy"], truoc["so_to_khoa"]) == (0, 1, 1)

    counts = kho.close_open_pair_papers(context_id, now=NOW)
    assert counts == {"bo": 0, "huy": 1}
    trang_thai = {p.id: p.state for p in kho.list_pair_papers(context_id)}
    assert trang_thai[nhap.id] == "chot", "kế hoạch đã đứng thì đóng sổ không đụng"
    assert trang_thai[dang_cho.id] == "huy"


def test_sua_nhap_ghi_tai_cho_va_khong_them_phien_ban(kho, postgres_session):
    context_id, a, b, cycle_id = _so_mo(kho, postgres_session)
    paper = kho.create_pair_paper(
        context_id=context_id,
        cycle_id=cycle_id,
        draft_owner_id=a,
        tuan=TUAN,
        expires_at=KHUNG,
        content=NOI_DUNG,
        ly_do=None,
        nguon={},
        author_type="human",
        now=NOW,
    )
    moi = {**NOI_DUNG, "chang": [{**NOI_DUNG["chang"][0], "gio": "19:00"}]}
    kho.update_pair_draft(paper.id, content=moi, ly_do="Đổi giờ.")
    doc = kho.get_pair_paper(paper.id)
    assert len(doc.versions) == 1, "nháp không có lịch sử"
    assert doc.versions[0].content["chang"][0]["gio"] == "19:00"
    assert doc.versions[0].ly_do == "Đổi giờ."
