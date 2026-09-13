"""Data migration round-trip and competing itinerary writers in private schemas."""

import os
from concurrent.futures import ThreadPoolExecutor
from datetime import date
from pathlib import Path
from threading import Barrier
from uuid import uuid4

import pytest
from alembic import command
from alembic.config import Config
from sqlalchemy import create_engine, text
from sqlalchemy.orm import Session
from sqlalchemy.schema import CreateSchema, DropSchema

from app.api.errors import RepositoryConflict
from app.api.repository import SqlAlchemyApiRepository

from .conftest import _configured_url, _schema_url

pytestmark = pytest.mark.postgres
BEFORE = "9a5e1c7b3f86"
AFTER = "27a9b83e10c4"


@pytest.fixture
def itinerary_schema():
    url = _configured_url()
    name = "journey_it_" + uuid4().hex
    admin = create_engine(url, hide_parameters=True)
    scoped = _schema_url(url, name)
    engine = create_engine(scoped, hide_parameters=True)

    def migrate(direction, revision):
        previous = os.environ.get("MOBILE_DATABASE_URL")
        os.environ["MOBILE_DATABASE_URL"] = scoped.render_as_string(hide_password=False)
        try:
            config = Config(str(Path(__file__).resolve().parents[2] / "alembic.ini"))
            getattr(command, direction)(config, revision)
        finally:
            if previous is None:
                os.environ.pop("MOBILE_DATABASE_URL", None)
            else:
                os.environ["MOBILE_DATABASE_URL"] = previous

    try:
        with admin.begin() as connection:
            connection.execute(CreateSchema(name))
        migrate("upgrade", BEFORE)
        yield engine, migrate
    finally:
        engine.dispose()
        with admin.begin() as connection:
            connection.execute(DropSchema(name, cascade=True, if_exists=True))
        admin.dispose()


def stage(engine):
    owner, context, one, many, stop_one, stop_many = [uuid4() for _ in range(6)]
    with engine.begin() as connection:
        connection.execute(
            text("INSERT INTO people (id,display_name) VALUES (:id,'Test')"),
            {"id": owner},
        )
        connection.execute(
            text(
                "INSERT INTO contexts (id,display_name,created_by_id) VALUES (:id,'Test',:owner)"
            ),
            {"id": context, "owner": owner},
        )
        for outing, ends, stop in (
            (one, "2030-01-01", stop_one),
            (many, "2030-01-02", stop_many),
        ):
            connection.execute(
                text(
                    "INSERT INTO outings (id,context_id,created_by_id,title,starts_on,ends_on,headcount,budget_per_person_vnd) VALUES (:id,:context,:owner,'Test','2030-01-01',:ends,2,0)"
                ),
                {"id": outing, "context": context, "owner": owner, "ends": ends},
            )
            connection.execute(
                text(
                    "INSERT INTO outing_stops (id,outing_id,position,minute_of_day,label) VALUES (:id,:outing,0,480,'Test')"
                ),
                {"id": stop, "outing": outing},
            )
        connection.execute(
            text(
                "INSERT INTO outing_stop_checkins (id,stop_id,person_id) VALUES (:id,:stop,:owner)"
            ),
            {"id": uuid4(), "stop": stop_one, "owner": owner},
        )
    return one, stop_one, stop_many


def test_migrate_existing_days_and_keep_checkins_roundtrip(itinerary_schema):
    engine, migrate = itinerary_schema
    _, one, many = stage(engine)
    for step in range(2):
        migrate("upgrade", AFTER)
        with engine.connect() as connection:
            rows = dict(
                connection.execute(text("SELECT id,day FROM outing_stops")).all()
            )
            assert rows[one] == date(2030, 1, 1)
            assert rows[many] is None
            assert (
                connection.scalar(text("SELECT count(*) FROM outing_stop_checkins"))
                == 1
            )
            assert connection.scalar(
                text("SELECT bool_and(time_locked) FROM outing_stops")
            )
        if step == 0:
            migrate("downgrade", BEFORE)


@pytest.mark.parametrize("legacy_second", [False, True])
def test_two_connections_cannot_overwrite_same_revision(
    itinerary_schema, legacy_second
):
    engine, migrate = itinerary_schema
    outing_id, stop_id, _ = stage(engine)
    migrate("upgrade", AFTER)
    barrier = Barrier(2)

    def write(number):
        with Session(engine) as session:
            repository = SqlAlchemyApiRepository(session)
            loaded = repository.get_outing(outing_id)
            assert loaded.timeline_revision == 0
            barrier.wait(timeout=5)
            try:
                if number == 2 and legacy_second:
                    repository.replace_outing_stops(
                        outing_id=outing_id,
                        stops=[
                            {"minute_of_day": 480, "label": "Test", "place_name": None}
                        ],
                        expected_revision=0,
                    )
                else:
                    repository.replace_outing_itinerary(
                        outing_id=outing_id,
                        expected_revision=0,
                        itinerary_days=[],
                        stops=[
                            {
                                "id": str(stop_id),
                                "minute_of_day": 480 + number,
                                "label": "Test",
                                "day": date(2030, 1, 1),
                                "place_name": None,
                            }
                        ],
                    )
                session.commit()
                return "saved"
            except RepositoryConflict as exc:
                session.rollback()
                return exc.code

    with ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(write, (1, 2)))
    assert sorted(results) == ["TIMELINE_REVISION_CONFLICT", "saved"]
    with engine.connect() as connection:
        assert (
            connection.scalar(
                text("SELECT timeline_revision FROM outings WHERE id=:id"),
                {"id": outing_id},
            )
            == 1
        )
