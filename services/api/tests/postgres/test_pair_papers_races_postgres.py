"""Hai người bấm cùng lúc trên một tờ giấy (ADR-0027 K3).

`test_pair_papers_postgres.py` chứng minh trigger và ràng buộc bằng những lời
gọi tuần tự trong MỘT session. Điều đó chứng minh chúng tồn tại và bắn. Nó
không chạm được câu hỏi khác: hai yêu cầu HTTP riêng biệt, mỗi cái một
transaction, cùng đọc một tờ giấy rồi cùng ghi lên nó.

Bốn ca dưới đây viết ở dạng **tất định, không cần luồng** — cùng cách
`test_friend_consent_races_postgres.py` chọn. Mỗi ca đặt tay đúng dãy service
thực hiện: lượt một đọc, lượt hai đọc (cùng thấy trạng thái cũ), lượt một ghi
và commit, rồi lượt hai mới đi tiếp. Khoá `with_for_update` là thứ xếp hàng hai
lượt ghi trong sản phẩm; cái được chứng minh ở đây là điều xảy ra SAU khi xếp
hàng: service đọc lại dưới khoá, nên lượt thua **từ chối** thay vì ghi đè.

Mọi khẳng định là **về hàng trong database**, không phải về số lần gọi hàm.
Đếm lời gọi chỉ chứng minh mã đi qua một dòng; đếm hàng chứng minh hai người
không cùng có một buổi tối khác nhau.
"""

from __future__ import annotations

import uuid
from collections.abc import Iterator
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import delete, select, text
from sqlalchemy.engine import Engine
from sqlalchemy.orm import Session

from app.api.deps import Actor
from app.api.errors import ApiProblem
from app.api.repository import SqlAlchemyApiRepository
from app.api.schemas import (
    CloseNotebookRequest,
    PaperAgreeRequest,
    PaperReviseRequest,
    PaperSendRequest,
)
from app.api.service import ApiService
from app.db.models import (
    Context,
    Membership,
    Outing,
    PairNotebook,
    PairPaper,
    PairPaperOuting,
    Person,
)

pytestmark = pytest.mark.postgres

#: Thứ Tư, giữa trưa giờ Việt Nam. Thứ Bảy của tuần ấy còn ở phía trước, nên
#: ngày tờ giấy đề xuất là một ngày người ta còn đi được.
NOW = datetime(2030, 9, 18, 5, tzinfo=UTC)

#: Id mọi hàng ca này tạo, để dọn đúng chúng. Tầng postgres dùng chung một
#: schema và những ca ở đây COMMIT thật.
_CREATED: list[uuid.UUID] = []


@pytest.fixture
def two_connections(postgres_engine: Engine) -> Iterator[tuple[Session, Session]]:
    """Hai session độc lập — hai transaction, đúng như hai yêu cầu HTTP.

    Một session không diễn tả được những lỗi này: trong cùng transaction, lượt
    đọc thứ hai đã thấy lượt ghi thứ nhất, nên TOCTOU biến mất và ca xanh trong
    khi sản phẩm hỏng.
    """
    first = Session(postgres_engine, expire_on_commit=False)
    second = Session(postgres_engine, expire_on_commit=False)
    try:
        yield first, second
    finally:
        first.rollback()
        second.rollback()
        with Session(postgres_engine) as cleanup:
            # Dọn ĐÚNG những hàng ca này tạo, theo context, chứ không
            # `DELETE FROM` cả bảng: tầng postgres dùng chung một schema, và
            # một lệnh xoá rộng ở đây làm đỏ một ca đếm hàng ở file khác.
            papers = [
                row[0]
                for row in cleanup.execute(
                    select(PairPaper.id).where(PairPaper.context_id.in_(_CREATED))
                )
            ]
            # T1 từ chối xoá một phiên bản ĐÃ GỬI, không có cửa sau nào —
            # đúng như nó nên thế, và ca đỏ của nó nằm ở
            # `test_pair_papers_postgres.py`. Ở đây ta đang tháo một fixture
            # chứ không viết lại lịch sử của ai, nên trigger được tắt đúng
            # trong khối dọn rồi bật lại ngay.
            cleanup.execute(
                text(
                    "ALTER TABLE pair_paper_versions"
                    " DISABLE TRIGGER pair_paper_versions_immutable"
                )
            )
            for table, column, keys in (
                ("pair_paper_outings", "paper_id", papers),
                ("pair_paper_keeps", "paper_id", papers),
                ("pair_paper_responses", "paper_id", papers),
                ("pair_paper_views", "paper_id", papers),
                ("pair_paper_versions", "paper_id", papers),
                ("pair_papers", "id", papers),
            ):
                if keys:
                    cleanup.execute(
                        text(f"DELETE FROM {table} WHERE {column} = ANY(:ids)"),  # noqa: S608
                        {"ids": keys},
                    )
            cycles = [
                row[0]
                for row in cleanup.execute(
                    text(
                        "SELECT c.id FROM pair_notebook_cycles c"
                        " JOIN pair_notebooks n ON n.id = c.notebook_id"
                        " WHERE n.context_id = ANY(:ids)"
                    ),
                    {"ids": _CREATED},
                )
            ]
            if cycles:
                cleanup.execute(
                    text(
                        "DELETE FROM pair_consents WHERE proposal_id IN ("
                        " SELECT id FROM pair_consent_proposals"
                        " WHERE cycle_id = ANY(:ids))"
                    ),
                    {"ids": cycles},
                )
                for table in (
                    "pair_shared_constraints",
                    "active_couple_members",
                    "pair_consent_proposals",
                    "pair_cycle_participants",
                ):
                    cleanup.execute(
                        text(f"DELETE FROM {table} WHERE cycle_id = ANY(:ids)"),  # noqa: S608
                        {"ids": cycles},
                    )
                # Bảng chu kỳ tự gọi khoá chính của nó là `id`, không phải
                # `cycle_id` — dòng cuối của vòng lặp trên từng đứng ở đây và
                # nổ vì thế.
                cleanup.execute(
                    text("DELETE FROM pair_notebook_cycles WHERE id = ANY(:ids)"),
                    {"ids": cycles},
                )
            cleanup.execute(
                text(
                    "ALTER TABLE pair_paper_versions"
                    " ENABLE TRIGGER pair_paper_versions_immutable"
                )
            )
            cleanup.execute(
                delete(PairNotebook).where(PairNotebook.context_id.in_(_CREATED))
            )
            cleanup.execute(
                text("DELETE FROM outings WHERE context_id = ANY(:ids)"),
                {"ids": _CREATED},
            )
            cleanup.execute(
                delete(Membership).where(Membership.person_id.in_(_CREATED))
            )
            cleanup.execute(delete(Context).where(Context.id.in_(_CREATED)))
            cleanup.execute(delete(Person).where(Person.id.in_(_CREATED)))
            cleanup.commit()
        _CREATED.clear()
        first.close()
        second.close()


def _actor(person_id: uuid.UUID, context_id: uuid.UUID) -> Actor:
    return Actor(
        id=person_id, roles=frozenset({"member"}), context_ids=frozenset({context_id})
    )


def _cap(session: Session) -> tuple[uuid.UUID, uuid.UUID, uuid.UUID]:
    """Một cặp thật: hai người, một context `pair`, hai tư cách thành viên."""
    a = Person(id=uuid.uuid4(), display_name="Người A")
    b = Person(id=uuid.uuid4(), display_name="Người B")
    session.add_all([a, b])
    session.flush()
    context = Context(
        id=uuid.uuid4(),
        display_name="",
        created_by_id=a.id,
        kind="pair",
        pair_key=f"{min(str(a.id), str(b.id))}:{max(str(a.id), str(b.id))}",
    )
    session.add(context)
    session.flush()
    for person_id in (a.id, b.id):
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
    session.commit()
    _CREATED.extend([a.id, b.id, context.id])
    return context.id, a.id, b.id


def _mo_so(service: ApiService, context_id: uuid.UUID, a: uuid.UUID, b: uuid.UUID):
    """Cả hai đồng ý lập sổ, qua đúng hai lệnh của wire."""
    from app.api.schemas import PairProposalCreateRequest

    offered = service.propose_pair_consent(
        context_id, PairProposalCreateRequest(purpose="lap_so"), _actor(a, context_id)
    )
    service.grant_pair_consent(context_id, offered.id, _actor(b, context_id))


def _to_da_gui(
    service: ApiService, context_id: uuid.UUID, nguoi_gui: uuid.UUID
) -> uuid.UUID:
    """Một tờ đã gửi, với «ừ» của người gửi đã nằm trong bảng."""
    drafted = service.draft_pair_paper(context_id, _actor(nguoi_gui, context_id))
    service.send_pair_paper(
        drafted.id, PaperSendRequest(version=1), _actor(nguoi_gui, context_id)
    )
    return drafted.id


def _dem_keo(session: Session, paper_id: uuid.UUID) -> int:
    return len(
        session.execute(
            select(PairPaperOuting).where(PairPaperOuting.paper_id == paper_id)
        )
        .scalars()
        .all()
    )


def _monkeypatched_now(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr("app.api.service._now", lambda: NOW)


def test_hai_lan_dong_y_cung_luc_chi_sinh_mot_keo(
    two_connections: tuple[Session, Session], monkeypatch: pytest.MonkeyPatch
):
    """Người B bấm «ừ» hai lần từ hai màn — một buổi tối, một kèo.

    Đây là hình dạng của một lần gửi lại sau khi mất mạng: lượt đầu đã ghi và
    commit, lượt sau tới với cùng ý định. Nó phải nhận lại **cùng `outing_id`**,
    không phải một kèo thứ hai và không phải một lời từ chối.
    """
    _monkeypatched_now(monkeypatch)
    first, second = two_connections
    context_id, a, b = _cap(first)
    service_one = ApiService(SqlAlchemyApiRepository(first))
    _mo_so(service_one, context_id, a, b)
    paper_id = _to_da_gui(service_one, context_id, a)
    first.commit()

    service_two = ApiService(SqlAlchemyApiRepository(second))
    # Cả hai lượt đọc tờ giấy khi nó còn `da_gui`.
    assert service_one.pair_paper(paper_id, _actor(b, context_id)).state == "da_gui"
    assert service_two.pair_paper(paper_id, _actor(b, context_id)).state == "da_gui"

    lan_dau = service_one.respond_pair_paper(
        paper_id, 1, PaperAgreeRequest(kind="dong_y"), _actor(b, context_id)
    )
    first.commit()
    assert lan_dau.state == "chot"
    assert lan_dau.outing_id is not None

    lan_hai = service_two.respond_pair_paper(
        paper_id, 1, PaperAgreeRequest(kind="dong_y"), _actor(b, context_id)
    )
    second.commit()
    assert lan_hai.outing_id == lan_dau.outing_id, "phát lại đúng kèo cũ"
    assert lan_hai.state == "chot"
    assert _dem_keo(first, paper_id) == 1, "K3: một tờ, một kèo"


def test_de_nghi_sua_thang_thi_lan_dong_y_cu_nhan_stale(
    two_connections: tuple[Session, Session], monkeypatch: pytest.MonkeyPatch
):
    """B đề nghị sửa, A đồng ý phiên bản cũ. Bên thua phải biết mình thua.

    Nếu «ừ» của A ghi được lên v1 sau khi v2 ra đời, hai người sẽ cùng nhìn một
    tờ giấy mà mỗi người đọc một buổi tối khác nhau — và không ai trong hai
    người có cách biết điều đó.
    """
    _monkeypatched_now(monkeypatch)
    first, second = two_connections
    context_id, a, b = _cap(first)
    service_one = ApiService(SqlAlchemyApiRepository(first))
    _mo_so(service_one, context_id, a, b)
    paper_id = _to_da_gui(service_one, context_id, a)
    first.commit()

    service_two = ApiService(SqlAlchemyApiRepository(second))
    assert service_two.pair_paper(paper_id, _actor(a, context_id)).version == 1

    service_one.respond_pair_paper(
        paper_id,
        1,
        PaperReviseRequest(
            kind="de_nghi_sua",
            content={"ngay": "2030-09-21", "chang": [{"gio": "20:00", "viec": "Chè"}]},
            ly_do="Muộn hơn được không?",
        ),
        _actor(b, context_id),
    )
    first.commit()

    with pytest.raises(ApiProblem) as refused:
        service_two.respond_pair_paper(
            paper_id, 1, PaperAgreeRequest(kind="dong_y"), _actor(a, context_id)
        )
    second.rollback()
    assert refused.value.code == "paper_version_stale"
    assert refused.value.status_code == 409
    assert _dem_keo(first, paper_id) == 0, "không có kèo nào sinh ra từ phiên bản cũ"
    assert first.get(PairPaper, paper_id).state == "da_gui"
    assert first.get(PairPaper, paper_id).current_version == 2


def test_dong_so_thang_thi_lan_chot_bi_tu_choi(
    two_connections: tuple[Session, Session], monkeypatch: pytest.MonkeyPatch
):
    """Một người đóng sổ trong khi người kia đang bấm «ừ».

    «Đóng sổ là đóng» (§7.6). Một lượt chốt lọt qua sau đó sẽ sinh ra một kèo
    trong một cuốn sổ vừa được đóng, và người đóng sổ không có cách nào biết.
    """
    _monkeypatched_now(monkeypatch)
    first, second = two_connections
    context_id, a, b = _cap(first)
    service_one = ApiService(SqlAlchemyApiRepository(first))
    _mo_so(service_one, context_id, a, b)
    paper_id = _to_da_gui(service_one, context_id, a)
    first.commit()

    service_two = ApiService(SqlAlchemyApiRepository(second))
    assert service_two.pair_paper(paper_id, _actor(b, context_id)).state == "da_gui"

    xem_truoc = service_one.preview_close_pair_notebook(
        context_id, _actor(a, context_id)
    )
    assert xem_truoc.so_to_huy == 1
    service_one.close_pair_notebook(
        context_id,
        CloseNotebookRequest(revision=xem_truoc.revision),
        _actor(a, context_id),
    )
    first.commit()

    with pytest.raises(ApiProblem) as refused:
        service_two.respond_pair_paper(
            paper_id, 1, PaperAgreeRequest(kind="dong_y"), _actor(b, context_id)
        )
    second.rollback()
    assert refused.value.code == "paper_wrong_state"
    assert _dem_keo(first, paper_id) == 0
    assert first.get(PairPaper, paper_id).state == "huy"


def test_xem_truoc_cu_khong_dong_duoc_cuon_so_vua_doi(
    two_connections: tuple[Session, Session], monkeypatch: pytest.MonkeyPatch
):
    """Người đọc «1 tờ sẽ huỷ» rồi bấm đóng, trong lúc người kia gửi thêm.

    `revision` tồn tại đúng vì lúc này: con số người ta đồng ý và những hàng
    sắp bị đóng phải là cùng những hàng ấy.
    """
    _monkeypatched_now(monkeypatch)
    first, second = two_connections
    context_id, a, b = _cap(first)
    service_one = ApiService(SqlAlchemyApiRepository(first))
    _mo_so(service_one, context_id, a, b)
    first.commit()

    da_doc = service_one.preview_close_pair_notebook(context_id, _actor(a, context_id))
    assert (da_doc.so_nhap_bo, da_doc.so_to_huy) == (0, 0)

    service_two = ApiService(SqlAlchemyApiRepository(second))
    _to_da_gui(service_two, context_id, b)
    second.commit()

    with pytest.raises(ApiProblem) as refused:
        service_one.close_pair_notebook(
            context_id,
            CloseNotebookRequest(revision=da_doc.revision),
            _actor(a, context_id),
        )
    first.rollback()
    assert refused.value.code == "notebook_revision_stale"
    con_song = (
        first.execute(select(PairPaper).where(PairPaper.context_id == context_id))
        .scalars()
        .all()
    )
    assert [row.state for row in con_song] == ["da_gui"], "từ chối thì không đóng gì"


def test_het_han_khong_bao_gio_thanh_chot_du_hai_nguoi_cung_bam(
    two_connections: tuple[Session, Session], monkeypatch: pytest.MonkeyPatch
):
    """Tuần trôi qua giữa lúc đọc và lúc bấm.

    Im lặng không phải đồng ý, và ở đây «im lặng» là cả một tuần. Ca này ở tầng
    hai kết nối vì nó là chỗ duy nhất `hieu_luc` được áp dụng DƯỚI KHOÁ, sau
    khi một transaction khác đã đi qua.
    """
    state = {"now": NOW}
    monkeypatch.setattr("app.api.service._now", lambda: state["now"])
    first, second = two_connections
    context_id, a, b = _cap(first)
    service_one = ApiService(SqlAlchemyApiRepository(first))
    _mo_so(service_one, context_id, a, b)
    paper_id = _to_da_gui(service_one, context_id, a)
    first.commit()

    service_two = ApiService(SqlAlchemyApiRepository(second))
    assert service_two.pair_paper(paper_id, _actor(b, context_id)).state == "da_gui"

    state["now"] = NOW + timedelta(days=8)
    with pytest.raises(ApiProblem) as refused:
        service_two.respond_pair_paper(
            paper_id, 1, PaperAgreeRequest(kind="dong_y"), _actor(b, context_id)
        )
    second.rollback()
    assert refused.value.code == "paper_expired"
    assert _dem_keo(first, paper_id) == 0
    assert first.get(PairPaper, paper_id).state == "da_gui", (
        "hết hạn là luật lúc đọc, không phải một hàng ai đó ghi"
    )
    assert service_one.pair_paper(paper_id, _actor(a, context_id)).state == "het_han"


def test_chot_hai_lan_tren_cung_mot_to_khong_sinh_keo_thu_hai(
    two_connections: tuple[Session, Session], monkeypatch: pytest.MonkeyPatch
):
    """Lớp thứ hai của K3, gọi thẳng vào nơi nó sống.

    Một phép đo độc lập xoá hai dòng đầu của `_chot` — chỗ dùng lại kèo đã có —
    và **năm ca đua ở trên vẫn xanh**. Đúng như thế: qua HTTP thì nhánh ấy
    không tới được. «Ừ» lần hai bị partial unique trên `pair_paper_responses`
    chặn trước, service phát lại thân cũ, và `_chot` không bao giờ chạy lần thứ
    hai. Khoá `with_for_update` là cái xếp hàng hai lượt ghi để chuyện đó đúng.

    Nhưng «không tới được hôm nay» không phải «không cần». Nếu `_chot` vào lần
    thứ hai mà thiếu hai dòng ấy, nó **tạo một kèo mới** rồi mới phát hiện
    `UNIQUE(paper_id)` từ chối liên kết — và cái kèo vừa tạo nằm lại trong danh
    sách của hai người như một buổi đi không ai hẹn. Lớp dưới (`link_paper_outing`
    ném xung đột) chặn được liên kết, không chặn được hàng thừa.

    Nên ca này gọi thẳng `_chot` hai lần, vì đó là hướng mà đường vòng đi tới.
    Nó là ca hộp trắng có chủ ý, và lý do nó tồn tại nằm ở đây chứ không ở tên
    hàm.
    """
    _monkeypatched_now(monkeypatch)
    first, _second = two_connections
    context_id, a, b = _cap(first)
    service = ApiService(SqlAlchemyApiRepository(first))
    _mo_so(service, context_id, a, b)
    paper_id = _to_da_gui(service, context_id, a)
    chot = service.respond_pair_paper(
        paper_id, 1, PaperAgreeRequest(kind="dong_y"), _actor(b, context_id)
    )
    first.commit()
    assert chot.state == "chot" and chot.outing_id is not None

    lai = service._chot(
        SqlAlchemyApiRepository(first).get_pair_paper(paper_id),
        1,
        _actor(b, context_id),
        now=NOW,
    )
    first.commit()
    assert lai == chot.outing_id, "vào lần hai phải nhận lại đúng kèo cũ"
    assert _dem_keo(first, paper_id) == 1
    keo = (
        first.execute(select(Outing).where(Outing.context_id == context_id))
        .scalars()
        .all()
    )
    assert len(keo) == 1, (
        "một hàng kèo thừa ở đây là một buổi đi không ai hẹn, nằm trong danh "
        "sách của hai người"
    )
