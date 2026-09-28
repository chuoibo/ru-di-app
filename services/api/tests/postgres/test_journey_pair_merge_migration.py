"""Both deployed branches must converge and survive a merged round trip."""

from uuid import uuid4

import pytest
from sqlalchemy import create_engine, text
from sqlalchemy.schema import CreateSchema, DropSchema

from tests.postgres.conftest import _configured_url, _schema_url
from tests.postgres.test_pair_migration_round_trip_postgres import (
    BANG,
    HAM,
    _alembic,
    _alembic_xuong,
    _co_bang,
    _co_ham,
)

pytestmark = pytest.mark.postgres
COMMON_BASE = "9a5e1c7b3f86"
PAIR_HEAD = "c4f27a90d1e3"
JOURNEY_HEAD = "27a9b83e10c4"
MERGE_HEAD = "d6a2f93b81e7"
JOURNEY_COLUMNS = {
    ("outings", "timeline_revision"),
    ("outings", "itinerary_version"),
    ("outings", "itinerary_days"),
    ("outing_stops", "day"),
    ("outing_stops", "duration_minutes"),
    ("outing_stops", "time_locked"),
    ("outing_stops", "meeting_lat"),
    ("outing_stops", "meeting_lng"),
    ("outing_stops", "meeting_label"),
}


def assert_schema(engine, revision, *, pair, journey):
    tables = _co_bang(engine)
    functions = _co_ham(engine)
    assert {"outings", "outing_stops"} <= tables
    assert tables & set(BANG) == (set(BANG) if pair else set())
    assert functions & set(HAM) == (set(HAM) if pair else set())
    with engine.connect() as connection:
        assert connection.scalars(
            text("SELECT version_num FROM alembic_version")
        ).all() == [revision]
        columns = set(
            connection.execute(
                text(
                    "SELECT table_name, column_name FROM information_schema.columns"
                    " WHERE table_schema = current_schema()"
                )
            ).all()
        )
    assert columns & JOURNEY_COLUMNS == (JOURNEY_COLUMNS if journey else set())


@pytest.mark.parametrize("starting_head", [PAIR_HEAD, JOURNEY_HEAD])
def test_each_branch_merges_downgrades_and_upgrades_again(starting_head):
    url = _configured_url()
    name = "journey_pair_merge_it_" + uuid4().hex
    admin = create_engine(url, hide_parameters=True)
    scoped_url = _schema_url(url, name)
    scoped = scoped_url.render_as_string(hide_password=False)
    engine = create_engine(scoped_url, hide_parameters=True)
    created = False
    try:
        with admin.begin() as connection:
            connection.execute(CreateSchema(name))
        created = True
        _alembic(scoped, starting_head)
        assert_schema(
            engine,
            starting_head,
            pair=starting_head == PAIR_HEAD,
            journey=starting_head == JOURNEY_HEAD,
        )
        _alembic(scoped, MERGE_HEAD)
        assert_schema(engine, MERGE_HEAD, pair=True, journey=True)
        _alembic_xuong(scoped, COMMON_BASE)
        assert_schema(engine, COMMON_BASE, pair=False, journey=False)
        _alembic(scoped, MERGE_HEAD)
        assert_schema(engine, MERGE_HEAD, pair=True, journey=True)
    finally:
        engine.dispose()
        if created:
            with admin.begin() as connection:
                connection.execute(DropSchema(name, cascade=True))
        admin.dispose()
