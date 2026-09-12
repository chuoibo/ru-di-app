"""Itinerary identity, CAS, private pins and HTTP replay on real PostgreSQL."""

from contextlib import contextmanager
from uuid import uuid4

import anyio
import pytest
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.deps import get_repository
from app.api.idempotency import SqlAlchemyIdempotencyStore
from app.api.main import create_app
from app.api.repository import SqlAlchemyApiRepository
from app.db.models import Membership, MembershipState, OutingStop

from .test_outings_postgres import _client, _group, _headers, _make_outing

pytestmark = pytest.mark.postgres


@pytest.fixture
def itinerary_http(postgres_session, postgres_engine, monkeypatch):
    async def inline(function, *args, **kwargs):
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", inline)

    @contextmanager
    def store():
        with Session(postgres_engine) as session:
            yield SqlAlchemyIdempotencyStore(session)
            session.commit()

    app = create_app(idempotency_store_factory=store)
    app.dependency_overrides[get_repository] = lambda: SqlAlchemyApiRepository(
        postgres_session
    )
    context, owner, outsider = _group(postgres_session)
    outing = _make_outing(postgres_session, app, owner, context)
    return app, owner, outsider, outing


def draft(outing):
    day = outing["starts_on"]
    return {
        "expected_revision": outing["timeline_revision"],
        "stops": [
            {
                "id": f"tmp-{i}",
                "at": at,
                "label": f"Điểm thử {i}",
                "place_name": None,
                "place_id": None,
                "day": day,
                "duration_minutes": 30,
                "time_locked": False,
                "meeting_point": {
                    "lat": 11.94 + i / 1000,
                    "lng": 108.43,
                    "label": "Điểm hẹn thử",
                },
            }
            for i, at in enumerate(("08:00", "10:00", "12:00"))
        ],
        "days": [
            {
                "day": day,
                "transport_mode": "walk",
                "start_at": "08:00",
                "start_stop_id": "tmp-0",
                "end_stop_id": None,
                "return_to_start": False,
            }
        ],
    }


def saved_draft(outing):
    return {
        "expected_revision": outing["timeline_revision"],
        "days": outing["days"],
        "stops": [
            {k: v for k, v in s.items() if k != "position"} for s in outing["stops"]
        ],
    }


def write_headers(owner, key=None):
    return {**_headers(owner.id), "Idempotency-Key": key or str(uuid4())}


def test_save_reorder_identity_undo_conflict_and_real_replay(
    itinerary_http, postgres_session
):
    app, owner, _, outing = itinerary_http
    url = f"/outings/{outing['id']}/itinerary"

    async def exchange():
        async with _client(app) as client:
            body = draft(outing)
            headers = write_headers(owner)
            first = await client.put(url, json=body, headers=headers)
            assert first.status_code == 200, first.text
            saved = first.json()
            assert saved["itinerary_version"] == 2
            assert saved["days"][0]["start_stop_id"] == saved["stops"][0]["id"]
            replay = await client.put(url, json=body, headers=headers)
            assert replay.json() == saved
            assert replay.headers["Idempotency-Replayed"] == "true"
            stop_id = saved["stops"][1]["id"]
            check = await client.post(
                f"/outing-stops/{stop_id}/checkins", headers=_headers(owner.id)
            )
            assert check.status_code == 201, check.text
            # Check-in changes scheduling constraints and invalidates stale previews.
            stale = await client.put(
                url, json=saved_draft(saved), headers=write_headers(owner)
            )
            assert stale.status_code == 409
            assert stale.json()["code"] == "timeline_conflict"
            current = SqlAlchemyApiRepository(postgres_session).get_outing(outing_id)
            edit = saved_draft(saved)
            edit["expected_revision"] = current.timeline_revision
            edit["stops"][1], edit["stops"][2] = edit["stops"][2], edit["stops"][1]
            edit["stops"][2]["at"] = "13:00"
            second = await client.put(url, json=edit, headers=write_headers(owner))
            assert second.status_code == 200, second.text
            assert second.json()["stops"][2]["id"] == stop_id
            checkins = await client.get(
                f"/outings/{outing['id']}/checkins", headers=_headers(owner.id)
            )
            assert checkins.json()["checkins"][0]["stop_id"] == stop_id
            undo = saved_draft(saved)
            undo["expected_revision"] = second.json()["timeline_revision"]
            restored = await client.put(url, json=undo, headers=write_headers(owner))
            assert restored.status_code == 200, restored.text
            assert [s["id"] for s in restored.json()["stops"]] == [
                s["id"] for s in saved["stops"]
            ]
            legacy = await client.put(
                url.replace("itinerary", "timeline"),
                json={"stops": []},
                headers=_headers(owner.id),
            )
            assert legacy.status_code == 409
            assert legacy.json()["code"] == "itinerary_upgrade_required"

    from uuid import UUID

    outing_id = UUID(outing["id"])
    anyio.run(exchange)


def test_preview_is_private_read_only_and_resolves_catalogue(
    itinerary_http, postgres_session, monkeypatch
):
    app, owner, outsider, outing = itinerary_http
    captured = []

    def preview(body):
        captured.append(body)
        return {"status": "unavailable", "revision": body["expected_revision"]}

    monkeypatch.setattr("app.journey.preview.preview_itinerary", preview)

    async def exchange():
        body = {**draft(outing), "day": outing["starts_on"], "include_suggestion": True}
        async with _client(app) as client:
            url = f"/outings/{outing['id']}/itinerary/preview"
            refused = await client.post(url, json=body, headers=_headers(outsider.id))
            assert refused.status_code == 403
            assert not captured
            allowed = await client.post(url, json=body, headers=_headers(owner.id))
            assert allowed.status_code == 200, allowed.text
            assert captured[0]["stops"][0]["lat"] == 11.94
            assert captured[0]["stops"][0]["checked_in"] is False
            assert not list(
                postgres_session.scalars(
                    select(OutingStop).where(OutingStop.outing_id == outing["id"])
                )
            )

    anyio.run(exchange)


def test_saved_pin_response_cannot_replay_after_leaving_group(
    itinerary_http, postgres_session
):
    from datetime import timedelta

    app, owner, _, outing = itinerary_http

    async def exchange():
        async with _client(app) as client:
            url = f"/outings/{outing['id']}/itinerary"
            headers = write_headers(owner)
            body = draft(outing)
            first = await client.put(url, json=body, headers=headers)
            assert first.status_code == 200, first.text
            membership = postgres_session.scalar(
                select(Membership).where(Membership.person_id == owner.id)
            )
            membership.left_at = membership.joined_at + timedelta(minutes=1)
            membership.state = MembershipState.LEFT
            postgres_session.flush()
            replay = await client.put(url, json=body, headers=headers)
            assert replay.status_code == 403, replay.text
            assert "meeting_point" not in replay.text

    anyio.run(exchange)


@pytest.mark.parametrize(
    "mutation",
    [
        "foreign_id",
        "duplicate_id",
        "unknown_place",
        "outside_day",
        "both_locations",
        "nan_pin",
        "foreign_anchor",
    ],
)
def test_invalid_drafts_cannot_write(itinerary_http, mutation):
    app, owner, _, outing = itinerary_http
    body = draft(outing)
    if mutation == "foreign_id":
        body["stops"][1]["id"] = str(uuid4())
    elif mutation == "duplicate_id":
        body["stops"][1]["id"] = body["stops"][0]["id"]
    elif mutation == "unknown_place":
        body["stops"][1].update(place_id="unknown-test-place", meeting_point=None)
    elif mutation == "outside_day":
        body["stops"][1]["day"] = "1900-01-01"
    elif mutation == "both_locations":
        body["stops"][1]["place_id"] = "somewhere"
    elif mutation == "nan_pin":
        body["stops"][1]["meeting_point"]["lat"] = "NaN"
    elif mutation == "foreign_anchor":
        body["days"][0]["end_stop_id"] = str(uuid4())

    async def exchange():
        async with _client(app) as client:
            response = await client.put(
                f"/outings/{outing['id']}/itinerary",
                json=body,
                headers=write_headers(owner),
            )
            assert response.status_code == 422, response.text

    anyio.run(exchange)


@pytest.mark.parametrize(
    "values",
    [
        {"meeting_lat": 91, "meeting_lng": 108, "meeting_label": "Test"},
        {"meeting_lat": 11, "meeting_lng": None, "meeting_label": "Test"},
        {"meeting_lat": 11, "meeting_lng": 108, "meeting_label": " "},
        {
            "meeting_lat": 11,
            "meeting_lng": 108,
            "meeting_label": "Test",
            "place_id": "x",
        },
        {"duration_minutes": -1},
    ],
)
def test_database_rejects_invalid_schedule_or_pin(
    itinerary_http, postgres_session, values
):
    _, _, _, outing = itinerary_http
    with pytest.raises(IntegrityError), postgres_session.begin_nested():
        postgres_session.add(
            OutingStop(
                outing_id=outing["id"],
                position=0,
                minute_of_day=480,
                label="Test",
                **values,
            )
        )
        postgres_session.flush()
