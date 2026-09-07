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
from app.db.models import AccountSession, AuditEvent, Context, Membership, Person, Post
from app.domain.account_lifecycle import ANONYMOUS_DISPLAY_NAME, ERASURE

from .test_group_recap_postgres import _app, _call, _group, _headers, _person, _split
from .test_posts_postgres import _befriend
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
    # Không mang số tiền nào, nhưng nó là cái CỬA vào một nghĩa vụ: mất một
    # hàng ở đây là một envelope khách không mở được nữa, và đó cũng là sổ
    # tiền hỏng theo nghĩa người dùng. `ERASURE` xếp nó ở nhánh «giữ», nên
    # phép so md5 phải bao nó (ADR-0023 §6 đếm 20 bảng, không phải 19).
    "guest_links",
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


def test_a_private_conversation_stops_taking_messages_when_one_side_ends(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    """ADR-0023 §2.3.2 qua con đường của §2.1: tài khoản kết thúc thì cặp đóng.

    Chỉ tầng này bắt được nó. Kết thúc một tài khoản đặt mọi tư cách thành viên
    thành «đã rời», và roster của một cặp là nơi phép gác đi tìm «người kia».
    Bản đầu đọc «không thấy người kia» thành «không có gì để gác» và cho tin đi
    qua — đúng cánh cửa nó sinh ra để đóng thì nó mở, và mở im lặng. Fake
    repository không dựng lại được hình dạng ấy vì nó không có roster thật.
    """
    context, mot = _group(postgres_session, "Hội có cặp")
    hai = _person(postgres_session, "Người sẽ rời")
    postgres_session.add(
        Membership(
            context_id=context.id,
            person_id=hai.id,
            role="member",
            state="active",
            origin="named",
            joined_at=NOW,
            created_at=NOW,
        )
    )
    _befriend(postgres_session, mot, hai)
    postgres_session.flush()
    app = _app(postgres_session, monkeypatch)

    mo = _call(
        app, "POST", f"/people/{hai.id}/dm", headers=_headers(mot, context), json=None
    )
    assert mo.status_code in (200, 201), mo.text
    pair_id = mo.json()["id"]
    pair_headers = {
        "X-Actor-ID": str(mot.id),
        "X-Actor-Roles": "member",
        "X-Actor-Contexts": pair_id,
    }

    song = _call(
        app,
        "POST",
        f"/contexts/{pair_id}/messages",
        headers=pair_headers,
        json={"kind": "text", "body": "Còn sống", "image_url": None, "card": None},
    )
    assert song.status_code == 201, song.text

    SqlAlchemyApiRepository(postgres_session).erase_person(hai.id, now=NOW)
    postgres_session.flush()

    chet = _call(
        app,
        "POST",
        f"/contexts/{pair_id}/messages",
        headers=pair_headers,
        json={"kind": "text", "body": "Còn ai không", "image_url": None, "card": None},
    )
    assert chet.status_code == 409, chet.text
    assert chet.json()["code"] == "direct_message_unavailable"
    assert chet.json()["detail"] == "Cuộc trò chuyện này không còn nhận tin."


def test_an_ended_id_cannot_be_claimed_or_renamed_by_anybody(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    """ADR-0023 §2.2.3: `PUT /people/{id}` không hồi sinh một hàng đã xoá.

    Trước bản sửa, đường này trả 403 «rename_person_identity» — vừa nói cho
    người hỏi biết id ấy TỪNG là ai đó, vừa đóng vĩnh viễn đường mời-bằng-số
    cho số ấy. Câu đúng là 404, và đúng CÂU của «chưa có ai dùng số này».
    """
    app = _app(postgres_session, monkeypatch)
    repo = SqlAlchemyApiRepository(postgres_session)
    di = _person(postgres_session, "Người sẽ đi")
    o_lai = _person(postgres_session, "Người ở lại")
    postgres_session.flush()
    repo.erase_person(di.id, now=NOW)
    postgres_session.flush()

    dat_ten = _call(
        app,
        "PUT",
        f"/people/{di.id}",
        headers={"X-Actor-ID": str(o_lai.id), "X-Actor-Roles": "member"},
        json={"display_name": "Tên mới toanh"},
    )
    assert dat_ten.status_code == 404, dat_ten.text
    assert dat_ten.json()["code"] == "person_not_found"
    assert dat_ten.json()["detail"] == "Chưa có ai dùng số này trong Rủ Đi."
    con = postgres_session.get(Person, di.id)
    assert con is not None and con.display_name == ANONYMOUS_DISPLAY_NAME


def _mo_cap(app, mot: Person, hai: Person, context: Context) -> dict[str, str]:
    """Mở cặp giữa hai người bạn và trả về header để gửi tin vào cặp ấy."""
    mo = _call(
        app, "POST", f"/people/{hai.id}/dm", headers=_headers(mot, context), json=None
    )
    assert mo.status_code in (200, 201), mo.text
    return {
        "X-Actor-ID": str(mot.id),
        "X-Actor-Roles": "member",
        "X-Actor-Contexts": mo.json()["id"],
    }


def _gui(app, headers: dict[str, str], body: str):
    return _call(
        app,
        "POST",
        f"/contexts/{headers['X-Actor-Contexts']}/messages",
        headers=headers,
        json={"kind": "text", "body": body, "image_url": None, "card": None},
    )


def test_both_reasons_a_pair_dies_answer_with_the_very_same_bytes(
    postgres_session: Session, monkeypatch: pytest.MonkeyPatch
):
    """ADR-0023 §2.3.2: MỘT mã và MỘT câu cho cả hai nguyên nhân.

    «Người kia chặn tôi» và «người kia đã xoá tài khoản» là hai chuyện khác
    nhau, và ADR chọn nói cùng một câu cho cả hai — một oracle yếu, cố ý: người
    đang gõ vào một cuộc trò chuyện đã chết cần biết là nó chết, nhưng không ai
    được dùng đường này để hỏi «người kia có chặn tôi không».

    Trước ca này, «một mã một câu» là hai chuỗi ký tự trùng nhau ở hai chỗ
    trong `service.py` và KHÔNG có gì gác. Tách chúng ra thành hai mã khác
    nhau không làm ca nào đỏ ở bất kỳ tầng nào — reviewer của PR #581 đã chạy
    đúng phép ấy và cả bộ test giữ nguyên số. Ca này là cái gác còn thiếu: nó
    so THÂN TRẢ VỀ của hai nguyên nhân với nhau, từng byte.
    """
    context, mot = _group(postgres_session, "Hội hai lý do")
    bi_chan = _person(postgres_session, "Người sẽ chặn")
    da_xoa = _person(postgres_session, "Người sẽ xoá")
    for ai in (bi_chan, da_xoa):
        postgres_session.add(
            Membership(
                context_id=context.id,
                person_id=ai.id,
                role="member",
                state="active",
                origin="named",
                joined_at=NOW,
                created_at=NOW,
            )
        )
        _befriend(postgres_session, mot, ai)
    postgres_session.flush()
    app = _app(postgres_session, monkeypatch)
    repo = SqlAlchemyApiRepository(postgres_session)

    cap_chan = _mo_cap(app, mot, bi_chan, context)
    cap_xoa = _mo_cap(app, mot, da_xoa, context)
    # Đối chứng dương: khi cả hai còn sống thì cả hai cửa đều mở.
    assert _gui(app, cap_chan, "Còn sống 1").status_code == 201
    assert _gui(app, cap_xoa, "Còn sống 2").status_code == 201

    # Nguyên nhân một: người kia chặn.
    repo.open_block_edge(blocker_id=bi_chan.id, addressee_id=mot.id, now=NOW)
    postgres_session.flush()
    # Nguyên nhân hai: người kia kết thúc tài khoản.
    repo.erase_person(da_xoa.id, now=NOW)
    postgres_session.flush()

    vi_chan = _gui(app, cap_chan, "Còn ai không 1")
    vi_xoa = _gui(app, cap_xoa, "Còn ai không 2")

    assert vi_chan.status_code == 409, vi_chan.text
    assert vi_chan.status_code == vi_xoa.status_code, vi_xoa.text
    assert vi_chan.json() == vi_xoa.json(), (
        "hai nguyên nhân trả hai câu khác nhau — mã trả về đang là một cách hỏi "
        "«người kia có chặn tôi không», đúng thứ ADR-0023 §2.3.2 bỏ công giấu"
    )
    assert vi_chan.json()["code"] == "direct_message_unavailable"
    assert vi_chan.json()["detail"] == "Cuộc trò chuyện này không còn nhận tin."
