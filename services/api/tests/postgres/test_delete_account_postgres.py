"""Xoá tài khoản trên PostgreSQL thật (L5, ADR-0023 §2.1).

Câu duy nhất chỉ tầng này nói được: **sổ tiền không đổi một byte**. Ca dựng
một đời sống tiền thật qua HTTP (đề xuất khoản chi, chốt, đợt thu), chụp md5
của TỪNG bảng tiền trước khi xoá, xoá tài khoản, rồi so lại từng bảng. Khác
một byte là đỏ, và câu đỏ nói rõ bảng nào.

Ngoài ra: hàng `people` còn với tên ẩn danh và `deleted_at`; mọi bảng trong
nhánh `delete` của bản đồ về 0 cho người ấy; phiên bị thu hồi; membership
thành `left`; `audit_events` có đúng một hàng `account.deleted` mà
`event_data` KHÔNG chứa tên, bio hay thành phố.
"""

from __future__ import annotations

import uuid

import pytest
from sqlalchemy import func, select, text
from sqlalchemy.orm import Session

from app.api.repository import SqlAlchemyApiRepository
from app.db.models import AccountSession, AuditEvent, Membership, Person, Post
from app.domain.account_lifecycle import ANONYMOUS_DISPLAY_NAME, ERASURE

from .test_group_recap_postgres import _app, _call, _group, _headers, _person, _split
from .test_repository_postgres import NOW

pytestmark = pytest.mark.postgres

MONEY_TABLES = (
    "expenses",
    "expense_versions",
    "expense_items",
    "expense_item_shares",
    "expense_surcharges",
    "expense_discounts",
    "confirmed_allocations",
    "collection_batches",
    "collection_batch_versions",
    "collection_obligations",
    "collection_obligation_sources",
    "collection_envelopes",
    "payment_reports",
    "receipt_confirmations",
    "bills",
    "bill_items",
    "bill_item_shares",
    "bill_surcharges",
    "bill_discounts",
)


def _fingerprints(session: Session) -> dict[str, str]:
    """One md5 per money table, over every row as text, in a stable order.

    `string_agg(t::text, ...)` renders whole rows, so a changed column, a
    deleted row and a re-ordered value all move the digest. Ordering by the
    same text keeps the digest independent of the plan the database picks.
    """
    out = {}
    for table in MONEY_TABLES:
        digest = session.execute(
            text(
                f"SELECT md5(coalesce(string_agg(t::text, '|' ORDER BY t::text), ''))"
                f" FROM {table} t"  # noqa: S608 -- names come from the tuple above
            )
        ).scalar_one()
        out[table] = digest
    return out


def test_the_ledger_does_not_move_when_an_account_ends(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    context, payer = _group(postgres_session, "Hội tiền")
    other = _person(postgres_session, "Người kia")
    postgres_session.add(
        Membership(
            context_id=context.id,
            person_id=other.id,
            role="member",
            state="active",
            origin="named",
            joined_at=NOW,
            created_at=NOW,
        )
    )
    postgres_session.flush()
    app = _app(postgres_session, monkeypatch)
    _split(app, context, payer, [other], occurred_at="2030-08-20T12:00:00Z")
    _split(app, context, other, [payer], occurred_at="2030-08-21T12:00:00Z")

    written = _call(
        app,
        "POST",
        "/posts",
        headers=_headers(payer, context),
        json={"body": "một bài của người sắp rời", "audience": "public"},
    )
    assert written.status_code == 201, written.text
    postgres_session.add(
        AccountSession(
            id=uuid.uuid4(),
            person_id=payer.id,
            token_digest=uuid.uuid4().bytes + uuid.uuid4().bytes,
            issued_via="otp",
            created_at=NOW,
            expires_at=NOW.replace(year=NOW.year + 1),
        )
    )
    postgres_session.flush()

    before = _fingerprints(postgres_session)
    assert any(digest for digest in before.values()), "phải có tiền để mà so"

    repo = SqlAlchemyApiRepository(postgres_session)
    report = repo.erase_person(payer.id, now=NOW)
    postgres_session.flush()

    after = _fingerprints(postgres_session)
    moved = [table for table in MONEY_TABLES if before[table] != after[table]]
    assert moved == [], f"sổ tiền đổi ở: {moved}"

    person = postgres_session.get(Person, payer.id)
    assert person is not None, "hàng ở lại vì khoá ngoại của sổ trỏ vào nó"
    assert person.display_name == ANONYMOUS_DISPLAY_NAME
    assert person.deleted_at == NOW
    assert person.discoverable_by_phone is False
    assert person.wall_comment_policy == "nobody"
    assert (person.bio, person.city, person.budget_band) == (None, None, None)

    assert (
        postgres_session.scalar(
            select(func.count()).select_from(Post).where(Post.author_id == payer.id)
        )
        == 0
    )
    sessions = list(
        postgres_session.scalars(
            select(AccountSession).where(AccountSession.person_id == payer.id)
        )
    )
    assert sessions and all(row.revoked_at == NOW for row in sessions)
    memberships = list(
        postgres_session.scalars(
            select(Membership).where(Membership.person_id == payer.id)
        )
    )
    assert memberships and all(
        row.state.value == "left" and row.left_at == NOW for row in memberships
    )

    events = list(
        postgres_session.scalars(
            select(AuditEvent).where(AuditEvent.event_type == "account.deleted")
        )
    )
    assert len(events) == 1
    payload = str(events[0].event_data)
    for secret in (payer.display_name, "Hội tiền"):
        assert secret not in payload, "số đếm thôi, không tên, không câu chữ"
    assert set(report.counts) <= set(ERASURE["delete"]) | {
        "account_sessions",
        "memberships",
    }


def test_the_map_names_every_table_the_schema_has(postgres_session: Session):
    """Bản đồ xoá là bản đồ ĐÓNG: một bảng mới mà không ai xếp vào nhánh nào
    là một quyết định chưa ai ra, và nó phải đỏ ở đây chứ không im lặng."""
    named = [table for group in ERASURE.values() for table in group]
    assert len(named) == len(set(named)), "một bảng hai nhánh là hai lệnh mâu thuẫn"
    real = {
        row[0]
        for row in postgres_session.execute(
            text(
                "SELECT table_name FROM information_schema.tables"
                " WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'"
            )
        )
    } - {"alembic_version"}
    assert real - set(named) == set(), "bảng chưa có trong bản đồ xoá"
    assert set(named) - real == set(), "bản đồ nhắc bảng không tồn tại"
