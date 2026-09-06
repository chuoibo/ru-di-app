"""The L1 migration comes back down with a read mark on a deleted message.

Review of PR #574 (S1) ran `alembic downgrade` on a database where somebody's
read mark had stopped on a message that was later deleted: the downgrade
deleted the row and `fk_context_read_marks_message` refused. The fix turns
the deleted row into a text placeholder instead. This test stages exactly
that database in a private schema, runs the downgrade, and then the upgrade
again -- both directions must pass, on data, not only on an empty schema.
"""

from __future__ import annotations

import os
import uuid
from collections.abc import Callable, Generator
from datetime import UTC, datetime
from pathlib import Path

import pytest
from alembic import command
from alembic.config import Config
from sqlalchemy import create_engine, text
from sqlalchemy.engine import Engine
from sqlalchemy.schema import CreateSchema, DropSchema

from .conftest import _configured_url, _schema_url

pytestmark = pytest.mark.postgres

API_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_PREFIX = "downgrade_it_"
L1 = "5c1a7e3d9b42"
BEFORE_L1 = "e1f2a3b4c5d6"
NOW = datetime(2026, 9, 6, 3, 15, tzinfo=UTC)


@pytest.fixture
def scratch_schema() -> Generator[tuple[Engine, Callable[[str, str], None]]]:
    """A private schema at L1, plus a way to migrate it in either direction.
    Private because the test commits, and because moving the shared
    `alembic_version` would strand every other file in the directory."""
    database_url = _configured_url()
    schema_name = SCHEMA_PREFIX + uuid.uuid4().hex
    admin_engine = create_engine(database_url, pool_pre_ping=True, hide_parameters=True)
    scoped_url = _schema_url(database_url, schema_name)
    engine: Engine | None = None

    def migrate(direction: str, revision: str) -> None:
        previous = os.environ.get("MOBILE_DATABASE_URL")
        os.environ["MOBILE_DATABASE_URL"] = scoped_url.render_as_string(
            hide_password=False
        )
        try:
            config = Config(str(API_ROOT / "alembic.ini"))
            if direction == "up":
                command.upgrade(config, revision)
            else:
                command.downgrade(config, revision)
        finally:
            if previous is None:
                os.environ.pop("MOBILE_DATABASE_URL", None)
            else:
                os.environ["MOBILE_DATABASE_URL"] = previous

    try:
        with admin_engine.begin() as connection:
            connection.execute(CreateSchema(schema_name))
        migrate("up", L1)
        engine = create_engine(scoped_url, pool_pre_ping=True, hide_parameters=True)
        yield engine, migrate
    finally:
        if engine is not None:
            engine.dispose()
        with admin_engine.begin() as connection:
            connection.execute(DropSchema(schema_name, cascade=True, if_exists=True))
        admin_engine.dispose()


def _stage(connection) -> tuple[uuid.UUID, uuid.UUID]:
    person = uuid.uuid4()
    context = uuid.uuid4()
    deleted = uuid.uuid4()
    sticker = uuid.uuid4()
    connection.execute(
        text(
            "insert into people (id, display_name, created_at) "
            "values (:id, 'Minh Anh', :now)"
        ),
        {"id": person, "now": NOW},
    )
    connection.execute(
        text(
            "insert into contexts (id, display_name, created_by_id, created_at) "
            "values (:id, 'Nhóm', :person, :now)"
        ),
        {"id": context, "person": person, "now": NOW},
    )
    connection.execute(
        text(
            "insert into messages (id, context_id, author_id, kind, body, created_at, "
            "deleted_at) values (:id, :ctx, :person, 'deleted', NULL, :now, :now)"
        ),
        {"id": deleted, "ctx": context, "person": person, "now": NOW},
    )
    connection.execute(
        text(
            "insert into messages (id, context_id, author_id, kind, body, created_at) "
            "values (:id, :ctx, :person, 'sticker', 'di-thoi', :now)"
        ),
        {"id": sticker, "ctx": context, "person": person, "now": NOW},
    )
    # The reader's mark stopped on the message that was later deleted.
    connection.execute(
        text(
            "insert into context_read_marks (context_id, person_id, "
            "last_read_message_id, last_read_at, updated_at) "
            "values (:ctx, :person, :msg, :now, :now)"
        ),
        {"ctx": context, "person": person, "msg": deleted, "now": NOW},
    )
    return deleted, sticker


def test_downgrade_keeps_the_row_a_read_mark_points_at_and_upgrade_returns(
    scratch_schema,
):
    engine, migrate = scratch_schema
    with engine.begin() as connection:
        deleted, sticker = _stage(connection)

    migrate("down", BEFORE_L1)

    with engine.connect() as connection:
        kind, body = connection.execute(
            text("select kind, body from messages where id = :id"), {"id": deleted}
        ).one()
        assert (kind, body) == ("text", "Tin nhắn đã bị xoá")
        kind, body = connection.execute(
            text("select kind, body from messages where id = :id"), {"id": sticker}
        ).one()
        assert (kind, body) == ("text", "di-thoi")
        assert (
            connection.scalar(
                text(
                    "select count(*) from context_read_marks "
                    "where last_read_message_id = :id"
                ),
                {"id": deleted},
            )
            == 1
        ), "dấu đọc vẫn trỏ được vào hàng"
        columns = {
            row[0]
            for row in connection.execute(
                text(
                    "select column_name from information_schema.columns "
                    "where table_schema = current_schema() and table_name = 'messages'"
                )
            )
        }
        assert "deleted_at" not in columns and "reply_to_id" not in columns

    migrate("up", L1)
    with engine.connect() as connection:
        assert connection.scalar(text("select count(*) from messages")) == 2
