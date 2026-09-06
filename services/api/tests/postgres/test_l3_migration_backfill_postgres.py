"""The L3 migration on a database that already holds pictures (review #576 S3).

`7e3c9a5f1d64` adds `uploaded_images.purpose` and then a CHECK that says a
`group` picture is exactly one with a `context_id`. On an empty schema the
order of «backfill owner-attached rows to `avatar`» and «add the CHECK» is
invisible; on a database with one avatar it is the difference between a
migration that runs and one that stops with a CheckViolation. This stages a
person with an avatar and a group with a photo in a private schema at the
revision before L3, runs the upgrade, reads the purposes back, and then runs
the downgrade and the upgrade once more -- both directions, on data.
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
from sqlalchemy.exc import IntegrityError
from sqlalchemy.schema import CreateSchema, DropSchema

from .conftest import _configured_url, _schema_url

pytestmark = pytest.mark.postgres

API_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_PREFIX = "backfill_it_"
L3 = "7e3c9a5f1d64"
BEFORE_L3 = "6d2b8f4e0c53"
NOW = datetime(2026, 9, 6, 9, 30, tzinfo=UTC)


@pytest.fixture
def scratch_schema() -> Generator[tuple[Engine, Callable[[str, str], None]]]:
    """A private schema at the revision before L3, plus a way to migrate it in
    either direction. Private because the test commits, and because moving
    the shared `alembic_version` would strand every other file here."""
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
        migrate("up", BEFORE_L3)
        engine = create_engine(scoped_url, pool_pre_ping=True, hide_parameters=True)
        yield engine, migrate
    finally:
        if engine is not None:
            engine.dispose()
        with admin_engine.begin() as connection:
            connection.execute(DropSchema(schema_name, cascade=True, if_exists=True))
        admin_engine.dispose()


def _stage(connection) -> tuple[uuid.UUID, uuid.UUID]:
    """One person with an avatar, one group with a photo -- the two shapes an
    existing database holds before `purpose` exists."""
    person = uuid.uuid4()
    group = uuid.uuid4()
    avatar = uuid.uuid4()
    photo = uuid.uuid4()
    connection.execute(
        text(
            "INSERT INTO people (id, display_name, created_at) VALUES (:id, 'Ai đó', :now)"
        ),
        {"id": person, "now": NOW},
    )
    connection.execute(
        text(
            "INSERT INTO contexts (id, display_name, created_by_id, created_at) "
            "VALUES (:id, 'Nhóm', :person, :now)"
        ),
        {"id": group, "person": person, "now": NOW},
    )
    for image_id, context_id, owner_id in (
        (avatar, None, person),
        (photo, group, None),
    ):
        connection.execute(
            text(
                "INSERT INTO uploaded_images (id, storage_key, context_id, owner_person_id, "
                "uploaded_by_id, content_type, byte_size, width, height, created_at) "
                "VALUES (:id, :key, :context, :owner, :by, 'image/png', 1, 1, 1, :now)"
            ),
            {
                "id": image_id,
                "key": uuid.uuid4().hex,
                "context": context_id,
                "owner": owner_id,
                "by": person,
                "now": NOW,
            },
        )
    return avatar, photo


def test_the_upgrade_backfills_purpose_before_the_check_and_comes_back_down(
    scratch_schema,
):
    engine, migrate = scratch_schema
    with engine.begin() as connection:
        avatar, photo = _stage(connection)

    migrate("up", L3)

    with engine.begin() as connection:
        purposes = dict(
            connection.execute(text("SELECT id, purpose FROM uploaded_images")).all()
        )
        assert purposes == {avatar: "avatar", photo: "group"}, (
            "hàng cũ có chủ là ảnh đại diện, hàng cũ của nhóm là ảnh nhóm"
        )
        # The CHECKs are live: a `personal` row that names a group is refused.
        with pytest.raises(IntegrityError) as refused:
            connection.execute(
                text(
                    "INSERT INTO uploaded_images (id, storage_key, context_id, owner_person_id, "
                    "uploaded_by_id, content_type, byte_size, width, height, created_at, purpose) "
                    "SELECT gen_random_uuid(), :key, context_id, NULL, uploaded_by_id, "
                    "content_type, byte_size, width, height, created_at, 'personal' "
                    "FROM uploaded_images WHERE id = :photo"
                ),
                {"key": uuid.uuid4().hex, "photo": photo},
            )
        assert "ck_uploaded_images_image_purpose" in str(refused.value)

    migrate("down", BEFORE_L3)
    with engine.begin() as connection:
        columns = {
            row[0]
            for row in connection.execute(
                text(
                    "SELECT column_name FROM information_schema.columns "
                    "WHERE table_name = 'uploaded_images' AND table_schema = current_schema()"
                )
            )
        }
        assert "purpose" not in columns
        assert connection.scalar(text("SELECT count(*) FROM uploaded_images")) == 2

    migrate("up", L3)
    with engine.begin() as connection:
        again = dict(
            connection.execute(text("SELECT id, purpose FROM uploaded_images")).all()
        )
        assert again == {avatar: "avatar", photo: "group"}
