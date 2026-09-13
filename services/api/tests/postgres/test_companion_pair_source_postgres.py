"""Khoản bổ sung ADR-0019 trên PostgreSQL thật.

Tầng `tests/api` chứng minh thứ tự: từ chối trước khi đọc tin nhắn, và mã lỗi
riêng. Nó không chứng minh được cái này: phép cộng gu đọc consent bằng **những
hàng thật** — bốn bảng nối nhau (`pair_notebooks` → `pair_notebook_cycles` →
`pair_consent_proposals` → `pair_consents`) cộng roster — và một fake dict có
thể nói dối về hình dạng ấy theo cách không ai thấy.

Ca cuối cùng là ca hồi quy của hội bạn, so **từng trường** trên cùng hai con
người: nếu khoản bổ sung rò ra ngoài phạm vi `pair`, nó đỏ.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy.orm import Session

from app.api.repository import SqlAlchemyApiRepository
from app.api.service import ApiService
from app.db.models import (
    Context,
    Membership,
    PairConsent,
    PairConsentProposal,
    Person,
    PersonInterest,
)
from app.places.taste import UNKNOWN

pytestmark = pytest.mark.postgres

NOW = datetime(2030, 9, 18, 5, tzinfo=UTC)


def _nguoi(session: Session, ten: str, tags: tuple[str, ...]) -> uuid.UUID:
    person = Person(display_name=ten, budget_band="vua-phai")
    session.add(person)
    session.flush()
    for tag in tags:
        session.add(PersonInterest(person_id=person.id, tag=tag))
    session.flush()
    return person.id


def _context(session: Session, kind: str, people: tuple[uuid.UUID, ...]) -> uuid.UUID:
    context = Context(
        id=uuid.uuid4(),
        display_name="" if kind == "pair" else "Hội bạn",
        created_by_id=people[0],
        kind=kind,
        pair_key=(
            f"{min(str(people[0]), str(people[1]))}:"
            f"{max(str(people[0]), str(people[1]))}"
            if kind == "pair"
            else None
        ),
    )
    session.add(context)
    session.flush()
    for person_id in people:
        session.add(
            Membership(
                context_id=context.id,
                person_id=person_id,
                role="member",
                state="active",
                origin="named",
                joined_at=NOW,
                created_at=NOW,
            )
        )
    session.flush()
    return context.id


def _cho_doc_chat(
    session: Session, cycle_id: uuid.UUID, by: uuid.UUID, people: tuple[uuid.UUID, ...]
) -> None:
    proposal = PairConsentProposal(
        cycle_id=cycle_id,
        purpose="doc_chat",
        proposed_by_id=by,
        expires_at=NOW + timedelta(days=7),
    )
    session.add(proposal)
    session.flush()
    for person_id in people:
        session.add(
            PairConsent(proposal_id=proposal.id, person_id=person_id, granted_at=NOW)
        )
    session.flush()


def _service(session: Session, monkeypatch: pytest.MonkeyPatch) -> ApiService:
    monkeypatch.setattr("app.api.service._now", lambda: NOW)
    return ApiService(SqlAlchemyApiRepository(session))


def test_gu_cua_cap_la_chua_biet_cho_toi_khi_ca_hai_dong_y_tren_hang_that(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    a = _nguoi(postgres_session, "Người A", ("cafe", "outdoor"))
    b = _nguoi(postgres_session, "Người B", ("cafe", "an-uong"))
    context_id = _context(postgres_session, "pair", (a, b))
    service = _service(postgres_session, monkeypatch)
    repository = SqlAlchemyApiRepository(postgres_session)

    # Chưa có sổ: chưa ai đồng ý gì cả.
    assert service.group_taste(context_id) is UNKNOWN

    notebook = repository.create_pair_notebook(context_id, now=NOW)
    cycle_id = repository.open_pair_cycle(
        notebook.id, participants=(a, b), terms_version=1, now=NOW
    )
    repository.activate_pair_cycle(cycle_id, now=NOW)
    postgres_session.flush()
    assert service.group_taste(context_id) is UNKNOWN, "lập sổ không kéo theo bậc 4"

    # Một người đồng ý.
    _cho_doc_chat(postgres_session, cycle_id, by=a, people=(a,))
    assert service.group_taste(context_id) is UNKNOWN, "im lặng không phải đồng ý"

    # Người kia đồng ý nốt, trên cùng lời đề nghị.
    proposal_id = postgres_session.scalars(
        PairConsentProposal.__table__.select().with_only_columns(PairConsentProposal.id)
    ).first()
    postgres_session.add(
        PairConsent(proposal_id=proposal_id, person_id=b, granted_at=NOW)
    )
    postgres_session.flush()

    du = service.group_taste(context_id)
    assert du is not UNKNOWN
    assert du.people_answered == 2
    assert "cafe" in du.interests


def test_thu_hoi_lam_gu_ve_lai_chua_biet_ngay_lan_doc_sau(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    """Consent được hỏi ở mọi biên đọc. Không cache, nên không có cửa sổ nào
    giữa lúc thu hồi và lúc nó có tác dụng."""
    a = _nguoi(postgres_session, "Người A", ("cafe",))
    b = _nguoi(postgres_session, "Người B", ("an-uong",))
    context_id = _context(postgres_session, "pair", (a, b))
    service = _service(postgres_session, monkeypatch)
    repository = SqlAlchemyApiRepository(postgres_session)
    notebook = repository.create_pair_notebook(context_id, now=NOW)
    cycle_id = repository.open_pair_cycle(
        notebook.id, participants=(a, b), terms_version=1, now=NOW
    )
    repository.activate_pair_cycle(cycle_id, now=NOW)
    _cho_doc_chat(postgres_session, cycle_id, by=a, people=(a, b))
    assert service.group_taste(context_id) is not UNKNOWN

    repository.revoke_consents(cycle_id, "doc_chat", b, now=NOW)
    postgres_session.flush()
    assert service.group_taste(context_id) is UNKNOWN


def test_mot_loi_de_nghi_het_han_khong_con_la_mot_loi_dong_y(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    """Hạn thuộc về LỜI ĐỀ NGHỊ, và phép cộng đọc nó lúc đọc.

    Chỉ tầng này nói được câu ấy: hạn nằm ở hàng `pair_consent_proposals`, còn
    cái được đọc là hàng `pair_consents` — hai bảng, một luật.
    """
    a = _nguoi(postgres_session, "Người A", ("cafe",))
    b = _nguoi(postgres_session, "Người B", ("an-uong",))
    context_id = _context(postgres_session, "pair", (a, b))
    repository = SqlAlchemyApiRepository(postgres_session)
    notebook = repository.create_pair_notebook(context_id, now=NOW)
    cycle_id = repository.open_pair_cycle(
        notebook.id, participants=(a, b), terms_version=1, now=NOW
    )
    repository.activate_pair_cycle(cycle_id, now=NOW)
    _cho_doc_chat(postgres_session, cycle_id, by=a, people=(a, b))

    monkeypatch.setattr("app.api.service._now", lambda: NOW)
    service = ApiService(repository)
    assert service.group_taste(context_id) is not UNKNOWN

    monkeypatch.setattr("app.api.service._now", lambda: NOW + timedelta(days=8))
    assert service.group_taste(context_id) is UNKNOWN


def test_duong_cua_hoi_ban_khong_doi_mot_truong(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    """Ca hồi quy: CÙNG hai con người, một context `group`, không consent nào.

    Nếu khoản bổ sung rò ra ngoài phạm vi `pair`, hội bạn sẽ về «chưa biết» và
    mọi tấm thẻ trong sản phẩm mất phần trăm của nó.
    """
    a = _nguoi(postgres_session, "Người A", ("cafe", "outdoor"))
    b = _nguoi(postgres_session, "Người B", ("cafe", "an-uong"))
    hoi = _context(postgres_session, "group", (a, b))
    service = _service(postgres_session, monkeypatch)

    gu = service.group_taste(hoi)
    assert gu is not UNKNOWN
    assert gu.basis == "nhom"
    assert gu.people == 2 and gu.people_answered == 2
    assert gu.interests == ("an-uong", "cafe", "outdoor")
    assert gu.budget_per_person_vnd is not None

    # Và mở một cuốn sổ ở một cặp khác không đụng tới hội bạn.
    cap = _context(postgres_session, "pair", (a, b))
    repository = SqlAlchemyApiRepository(postgres_session)
    repository.create_pair_notebook(cap, now=NOW)
    postgres_session.flush()
    assert service.group_taste(cap) is UNKNOWN
    assert service.group_taste(hoi) == gu, "hội bạn phải giống hệt, từng trường"
