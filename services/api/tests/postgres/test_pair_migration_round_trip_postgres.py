"""`c4f27a90d1e3` lên, xuống, rồi lên lại — trên một schema của riêng ca này.

Một migration chỉ đi được một chiều là một migration không quay lui được khi
có sự cố, và cách duy nhất biết điều đó là chạy nó. Cả suite còn lại chạy trên
schema đã ở head trước khi ca đầu tiên chạy, nên không ca nào ở đó nói được
câu này.

Ca này cũng là chỗ duy nhất kiểm rằng `downgrade` bỏ CẢ trigger lẫn hàm: một
hàm còn sót lại làm lần `upgrade` sau đó đỏ ở `CREATE FUNCTION`, và nó đỏ ở
lần deploy chứ không ở đây nếu không ai chạy vòng này.
"""

from __future__ import annotations

import os
import uuid
from collections.abc import Generator

import pytest
from alembic import command
from alembic.config import Config
from sqlalchemy import create_engine, text
from sqlalchemy.engine import Engine
from sqlalchemy.schema import CreateSchema, DropSchema

from tests.postgres.conftest import API_ROOT, _configured_url, _schema_url

TRUOC = "9a5e1c7b3f86"
DUOI_THU = "c4f27a90d1e3"

BANG = (
    "pair_notebooks",
    "pair_notebook_cycles",
    "pair_cycle_participants",
    "pair_consent_proposals",
    "pair_consents",
    "active_couple_members",
    "pair_papers",
    "pair_paper_versions",
    "pair_paper_views",
    "pair_paper_responses",
    "pair_paper_outings",
    "pair_paper_keeps",
    "pair_shared_constraints",
)
HAM = (
    "pair_reject_sent_version_change",
    "pair_check_outing_link",
    "pair_check_couple_consent",
    "pair_check_response_participant",
)


def _alembic(schema_url: str, revision: str) -> None:
    truoc = os.environ.get("MOBILE_DATABASE_URL")
    os.environ["MOBILE_DATABASE_URL"] = schema_url
    try:
        command.upgrade(Config(str(API_ROOT / "alembic.ini")), revision)
    finally:
        if truoc is None:
            os.environ.pop("MOBILE_DATABASE_URL", None)
        else:
            os.environ["MOBILE_DATABASE_URL"] = truoc


def _alembic_xuong(schema_url: str, revision: str) -> None:
    truoc = os.environ.get("MOBILE_DATABASE_URL")
    os.environ["MOBILE_DATABASE_URL"] = schema_url
    try:
        command.downgrade(Config(str(API_ROOT / "alembic.ini")), revision)
    finally:
        if truoc is None:
            os.environ.pop("MOBILE_DATABASE_URL", None)
        else:
            os.environ["MOBILE_DATABASE_URL"] = truoc


@pytest.fixture
def schema_rieng() -> Generator[tuple[Engine, str]]:
    database_url = _configured_url()
    schema_name = "pair_round_trip_it_" + uuid.uuid4().hex
    admin = create_engine(database_url, pool_pre_ping=True, hide_parameters=True)
    engine: Engine | None = None
    created = False
    try:
        with admin.begin() as connection:
            connection.execute(CreateSchema(schema_name))
        created = True
        scoped = _schema_url(database_url, schema_name).render_as_string(
            hide_password=False
        )
        _alembic(scoped, DUOI_THU)
        engine = create_engine(
            _schema_url(database_url, schema_name),
            pool_pre_ping=True,
            hide_parameters=True,
        )
        yield engine, scoped
    finally:
        if engine is not None:
            engine.dispose()
        if created:
            with admin.begin() as connection:
                connection.execute(DropSchema(schema_name, cascade=True))
        admin.dispose()


def _co_bang(engine: Engine) -> set[str]:
    with engine.begin() as connection:
        return {
            row[0]
            for row in connection.execute(
                text(
                    "SELECT table_name FROM information_schema.tables"
                    " WHERE table_schema = current_schema()"
                )
            )
        }


def _co_ham(engine: Engine) -> set[str]:
    with engine.begin() as connection:
        return {
            row[0]
            for row in connection.execute(
                text(
                    "SELECT p.proname FROM pg_proc p"
                    " JOIN pg_namespace n ON n.oid = p.pronamespace"
                    " WHERE n.nspname = current_schema()"
                )
            )
        }


def test_len_xuong_len_lai(schema_rieng: tuple[Engine, str]):
    engine, scoped = schema_rieng
    assert set(BANG) <= _co_bang(engine), "lên: mười ba bảng phải có mặt"
    assert set(HAM) <= _co_ham(engine), "lên: bốn hàm trigger phải có mặt"

    _alembic_xuong(scoped, TRUOC)
    con_lai = _co_bang(engine)
    assert not (set(BANG) & con_lai), (
        f"xuống: còn sót bảng {sorted(set(BANG) & con_lai)}"
    )
    con_ham = _co_ham(engine)
    assert not (set(HAM) & con_ham), f"xuống: còn sót hàm {sorted(set(HAM) & con_ham)}"
    with engine.begin() as connection:
        # Lọc theo schema. Không có mệnh đề ấy, câu này đếm cả ràng buộc cùng
        # tên trong schema của ca khác đang chạy cùng phiên, và ca này đỏ khi
        # chạy chung còn xanh khi chạy một mình — đúng kiểu đỏ nói dối.
        con = connection.execute(
            text(
                "SELECT count(*) FROM pg_constraint c"
                " JOIN pg_class t ON t.oid = c.conrelid"
                " JOIN pg_namespace n ON n.oid = t.relnamespace"
                " WHERE c.conname = 'uq_contexts_id_kind'"
                "   AND n.nspname = current_schema()"
            )
        ).scalar_one()
    assert con == 0, "xuống: unique trên contexts phải rời đi cùng"

    _alembic(scoped, DUOI_THU)
    assert set(BANG) <= _co_bang(engine), "lên lại: mười ba bảng phải trở lại"
    assert set(HAM) <= _co_ham(engine), "lên lại: bốn hàm phải trở lại"


def test_xuong_khong_dong_cham_bang_cua_hoi_ban(schema_rieng: tuple[Engine, str]):
    """§12: gỡ sổ hai người ra không được lấy theo cái gì của hội bạn."""
    engine, scoped = schema_rieng
    truoc = _co_bang(engine) - set(BANG)
    _alembic_xuong(scoped, TRUOC)
    sau = _co_bang(engine)
    assert truoc <= sau, f"xuống làm mất bảng {sorted(truoc - sau)}"
