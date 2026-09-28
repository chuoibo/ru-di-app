"""The backfill in f4b8d1c6e2a7, pinned to a scenario it can fail.

Before that revision a check-in copied `places.lat/lng` whatever the point
was, so a group that checked in at a place known only by its ward or province
centroid has that centroid on its wall, and the «đã tới» layer and the heatmap
read it back as where the group stood. The migration strips those points with
one `UPDATE` that runs once, on real data, and nothing reads it afterwards --
so this file runs the migration itself, the way
`test_membership_origin_backfill_postgres.py` does.

Each seeded row pins one clause: break the precision list, the NULL-precision
exemption or the `kind` filter and exactly one assertion flips. The downgrade
is driven too, because it is the half that has to put points back.

Owns a private schema and commits, like the file it copies.
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
SCHEMA_PREFIX = "checkin_point_"

BEFORE = "e8c4d2a7f913"
UNDER_TEST = "f4b8d1c6e2a7"

NOW = datetime(2026, 9, 28, 9, 0, tzinfo=UTC)

# (place id, precision, has a point). Precision NULL with a point cannot be
# seeded -- `place_point_states_its_precision` refuses it since the fed
# catalogue arrived -- so the legacy exemption is pinned by the unit test of
# `mappable_point` instead.
PLACES = [
    ("bf-rooftop", "rooftop", True),
    ("bf-street", "street", True),
    ("bf-ward", "ward_centroid", True),
    ("bf-tinh", "province_centroid", True),
    ("bf-doan", "suy_luan", True),
]
KEEPS = {"bf-rooftop", "bf-street"}


@pytest.fixture
def scratch_schema() -> Generator[
    tuple[Engine, Callable[[str], None], Callable[[str], None]]
]:
    database_url = _configured_url()
    schema_name = SCHEMA_PREFIX + uuid.uuid4().hex
    admin_engine = create_engine(database_url, pool_pre_ping=True, hide_parameters=True)
    scoped_url = _schema_url(database_url, schema_name)
    engine: Engine | None = None

    def migrate(step: Callable[..., None], revision: str) -> None:
        previous = os.environ.get("MOBILE_DATABASE_URL")
        os.environ["MOBILE_DATABASE_URL"] = scoped_url.render_as_string(
            hide_password=False
        )
        try:
            step(Config(str(API_ROOT / "alembic.ini")), revision)
        finally:
            if previous is None:
                os.environ.pop("MOBILE_DATABASE_URL", None)
            else:
                os.environ["MOBILE_DATABASE_URL"] = previous

    try:
        with admin_engine.begin() as connection:
            connection.execute(CreateSchema(schema_name))
        migrate(command.upgrade, BEFORE)
        engine = create_engine(scoped_url, pool_pre_ping=True, hide_parameters=True)
        with engine.connect() as connection:
            assert connection.scalar(text("select current_schema()")) == schema_name
        yield (
            engine,
            lambda revision: migrate(command.upgrade, revision),
            lambda revision: migrate(command.downgrade, revision),
        )
    finally:
        if engine is not None:
            engine.dispose()
        with admin_engine.begin() as connection:
            connection.execute(DropSchema(schema_name, cascade=True, if_exists=True))
        admin_engine.dispose()


def _seed(connection) -> uuid.UUID:
    """A group with one check-in at every place, and one photo row."""
    person_id, context_id = uuid.uuid4(), uuid.uuid4()
    connection.execute(
        text(
            "insert into destinations (id, name, province, lat, lng, bbox_south, "
            "bbox_west, bbox_north, bbox_east, sort_order) values ('d-tphcm', "
            "'TP. Hồ Chí Minh', 'TP. Hồ Chí Minh', 10.77, 106.7, 10.68, 106.6, "
            "10.88, 106.82, 20)"
        )
    )
    for index, (place_id, precision, _) in enumerate(PLACES):
        connection.execute(
            text(
                "insert into places (id, destination_id, name, category, kinds, "
                "lat, lng, geo_precision, traits, source) values (:id, 'd-tphcm', "
                ":name, 'cafe', '[]', :lat, :lng, :precision, '[]', 'seed')"
            ),
            {
                "id": place_id,
                "name": f"Quán {place_id}",
                "lat": 10.77 + index / 1000,
                "lng": 106.70 + index / 1000,
                "precision": precision,
            },
        )
    connection.execute(
        text(
            "insert into people (id, display_name, created_at) values (:id, 'Minh Anh', :now)"
        ),
        {"id": person_id, "now": NOW},
    )
    connection.execute(
        text(
            "insert into contexts (id, display_name, created_by_id, created_at) "
            "values (:id, 'Team Sài Gòn', :by, :now)"
        ),
        {"id": context_id, "by": person_id, "now": NOW},
    )
    for index, (place_id, _, _) in enumerate(PLACES):
        # Written the way the old route wrote them: the place's point, whatever it was.
        connection.execute(
            text(
                "insert into memories (id, context_id, author_id, kind, place_id, "
                "place_name, lat, lng, created_at) values (:id, :ctx, :author, "
                "'checkin', :place, :name, :lat, :lng, :now)"
            ),
            {
                "id": uuid.uuid4(),
                "ctx": context_id,
                "author": person_id,
                "place": place_id,
                "name": f"Quán {place_id}",
                "lat": 10.77 + index / 1000,
                "lng": 106.70 + index / 1000,
                "now": NOW,
            },
        )
    # A photo tagged with a centroid place: not a check-in, never touched.
    connection.execute(
        text(
            "insert into memories (id, context_id, author_id, kind, image_url, "
            "place_id, place_name, created_at) values (:id, :ctx, :author, 'photo', "
            ":url, 'bf-tinh', 'Quán bf-tinh', :now)"
        ),
        {
            "id": uuid.uuid4(),
            "ctx": context_id,
            "author": person_id,
            "url": f"/contexts/{context_id}/photos/{uuid.uuid4()}",
            "now": NOW,
        },
    )
    return context_id


def _points(connection) -> dict[str, tuple[float | None, float | None]]:
    rows = connection.execute(
        text("select place_id, lat, lng from memories where kind = 'checkin'")
    )
    return {place_id: (lat, lng) for place_id, lat, lng in rows}


def test_the_upgrade_strips_centroid_points_and_the_downgrade_restores_them(
    scratch_schema,
):
    engine, upgrade, downgrade = scratch_schema
    with engine.begin() as connection:
        context_id = _seed(connection)

    upgrade(UNDER_TEST)
    with engine.connect() as connection:
        points = _points(connection)
        for place_id, _, _ in PLACES:
            lat, lng = points[place_id]
            if place_id in KEEPS:
                assert lat is not None and lng is not None, place_id
            else:
                assert lat is None and lng is None, place_id
        photo = connection.execute(
            text("select lat, place_id from memories where kind = 'photo'")
        ).one()
        assert photo == (None, "bf-tinh")

    # After the upgrade a check-in may carry no point, but never half of one.
    with engine.begin() as connection:
        person_id = connection.scalar(text("select author_id from memories limit 1"))
        connection.execute(
            text(
                "insert into memories (id, context_id, author_id, kind, place_id, "
                "place_name, created_at) values (:id, :ctx, :author, 'checkin', "
                "'bf-tinh', 'Quán bf-tinh', :now)"
            ),
            {"id": uuid.uuid4(), "ctx": context_id, "author": person_id, "now": NOW},
        )
    with pytest.raises(IntegrityError, match="payload_matches_kind"):
        with engine.begin() as connection:
            connection.execute(
                text(
                    "insert into memories (id, context_id, author_id, kind, place_id, "
                    "place_name, lat, created_at) values (:id, :ctx, :author, "
                    "'checkin', 'bf-tinh', 'Quán bf-tinh', 10.77, :now)"
                ),
                {
                    "id": uuid.uuid4(),
                    "ctx": context_id,
                    "author": person_id,
                    "now": NOW,
                },
            )

    downgrade(BEFORE)
    with engine.connect() as connection:
        assert all(
            lat is not None and lng is not None
            for lat, lng in _points(connection).values()
        )
